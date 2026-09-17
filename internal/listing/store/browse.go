package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// An empty query matches every listed card, because '%' || ” || '%' is '%%'.
// The set filter is skipped the same way, by an empty $2 rather than a second
// query — and when it is given, a listing whose scan resolved no printing drops
// out, because p.set_prefix is NULL for it and NULL = $2 is never true. That is
// the intended meaning: a card whose set nobody determined cannot be claimed to
// be in a particular set.
//
// The join walks the whole chain because none of it is on a listing: the slot
// carries the card and the printing, and the binder carries the seller.
const browseListingsQuery = `
SELECT l.id              AS listing_id,
       b.owner_id        AS seller_id,
       c.id              AS card_id,
       c.name            AS card_name,
       c.image_object_key,
       p.set_code        AS set_code,
       l.created_at      AS listed_at
FROM listings l
JOIN binder_slots s ON s.id = l.binder_slot_id
JOIN binders b ON b.id = s.binder_id
JOIN cards c ON c.id = s.card_id
LEFT JOIN card_printings p ON p.id = s.card_printing_id
WHERE c.name ILIKE '%' || $1 || '%'
  AND ($2 = '' OR p.set_prefix = $2)
ORDER BY l.created_at DESC, l.id
LIMIT $3`

// listedCardRow is one row of the browse feed. It is not a table: every column
// comes from the join above, which is why the names are aliased to what they
// mean here rather than to where they came from.
type listedCardRow struct {
	ListingID      uuid.UUID `db:"listing_id"`
	SellerID       uuid.UUID `db:"seller_id"`
	CardID         uuid.UUID `db:"card_id"`
	CardName       string    `db:"card_name"`
	ImageObjectKey *string   `db:"image_object_key"`
	// SetCode is NULL for a slot whose scan resolved no printing.
	SetCode  *string   `db:"set_code"`
	ListedAt time.Time `db:"listed_at"`
}

// toListedCard converts a row to the domain model, mapping every field.
func toListedCard(r listedCardRow) model.ListedCard {
	return model.ListedCard{
		ListingID: r.ListingID,
		SellerID:  r.SellerID,
		Card: cardmodel.Card{
			ID:             r.CardID,
			Name:           r.CardName,
			ImageObjectKey: r.ImageObjectKey,
		},
		SetCode:  r.SetCode,
		ListedAt: r.ListedAt,
	}
}

// BrowseListings returns the cards for sale that match search, newest first, at
// most search.Limit of them. Nothing matching is an empty slice and a nil error:
// a feed with no listings in it is the ordinary state of a young marketplace,
// not a failure (000-principles.md section 8b).
func (s *Store) BrowseListings(ctx context.Context, search model.ListingSearch) ([]model.ListedCard, error) {
	var rows []listedCardRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.SelectContext(ctx, &rows, browseListingsQuery,
			escapeLikePattern(search.Query), search.SetPrefix, search.Limit)
	})
	if err != nil {
		return nil, fmt.Errorf("browsing listings: %w", err)
	}

	listed := make([]model.ListedCard, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, toListedCard(row))
	}
	return listed, nil
}
