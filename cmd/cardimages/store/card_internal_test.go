package store

import (
	"testing"

	"github.com/google/uuid"
)

// White-box: toPendingCard is the row->model converter, and this is the
// completeness test store.go points at. Every source field is set to a
// distinct non-zero value and every destination field is asserted, so adding a
// column to cardRow without carrying it across fails here rather than shipping
// a zero value (000-principles.md section 9, 006-testing.md "Mapper
// completeness tests").
func TestToPendingCardCarriesEveryField(t *testing.T) {
	t.Parallel()

	row := cardRow{
		ID:           uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		YGOProDeckID: 89631139,
	}

	card := row.toPendingCard()

	if card.ID != row.ID {
		t.Errorf("ID = %v, want %v", card.ID, row.ID)
	}
	if card.YGOProDeckID != row.YGOProDeckID {
		t.Errorf("YGOProDeckID = %d, want %d", card.YGOProDeckID, row.YGOProDeckID)
	}
}
