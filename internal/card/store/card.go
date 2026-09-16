package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/simeonkorchev/binder/internal/card/model"
)

// The `%` operator is what uses cards_name_trgm_idx (001_cards.sql); the
// explicit similarity() floor pins the threshold the ladder relies on rather
// than inheriting whatever pg_trgm.similarity_threshold the cluster is set to.
const findCardsByNameQuery = `
	SELECT c.id,
	       c.name,
	       c.image_object_key,
	       similarity(c.name, $1) AS similarity
	  FROM cards c
	 WHERE c.name % $1
	   AND similarity(c.name, $1) >= $2
	 ORDER BY similarity(c.name, $1) DESC, c.name, c.id
	 LIMIT $3`

// An empty query matches every card, because '%' || ” || '%' is '%%'. The set
// filter is skipped the same way, by an empty $2 rather than a second query.
const searchCardsQuery = `
	SELECT c.id,
	       c.name,
	       c.image_object_key
	  FROM cards c
	 WHERE c.name ILIKE '%' || $1 || '%'
	   AND ($2 = '' OR EXISTS (
	         SELECT 1 FROM card_printings p
	          WHERE p.card_id = c.id AND p.set_prefix = $2))
	 ORDER BY c.name, c.id
	 LIMIT $3 OFFSET $4`

type cardRow struct {
	ID             uuid.UUID `db:"id"`
	Name           string    `db:"name"`
	ImageObjectKey *string   `db:"image_object_key"`
}

func (r cardRow) toModel() model.Card {
	return model.Card{
		ID:             r.ID,
		Name:           r.Name,
		ImageObjectKey: r.ImageObjectKey,
	}
}

type nameMatchRow struct {
	cardRow
	Similarity float64 `db:"similarity"`
}

func (r nameMatchRow) toModel() model.NameMatch {
	return model.NameMatch{
		Card:       r.cardRow.toModel(),
		Similarity: r.Similarity,
	}
}

// FindCardsByName returns the cards whose name is trigram-similar to name, best
// first, at most limit of them. No match is an empty slice, not an error.
func (s *Store) FindCardsByName(
	ctx context.Context,
	name string,
	threshold float64,
	limit int,
) ([]model.NameMatch, error) {
	var rows []nameMatchRow
	if err := s.db.SelectContext(ctx, &rows, findCardsByNameQuery, name, threshold, limit); err != nil {
		return nil, fmt.Errorf("finding cards by name: %w", err)
	}

	matches := make([]model.NameMatch, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, row.toModel())
	}
	return matches, nil
}

// SearchCards returns one page of cards matching search, by name. A page past
// the end is an empty slice, not an error.
func (s *Store) SearchCards(ctx context.Context, search model.CardSearch) ([]model.Card, error) {
	offset := (search.Page - 1) * search.PageSize

	var rows []cardRow
	err := s.db.SelectContext(
		ctx,
		&rows,
		searchCardsQuery,
		escapeLikePattern(search.Query),
		search.SetPrefix,
		search.PageSize,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("searching cards: %w", err)
	}

	cards := make([]model.Card, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, row.toModel())
	}
	return cards, nil
}

// escapeLikePattern neutralises the LIKE metacharacters in a user-supplied
// search term. The term travels as a bound parameter, so this is not about
// injection — it is that a user typing "100%" means the three characters, not
// "anything after 100". Backslash first: it is LIKE's own escape character.
func escapeLikePattern(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}
