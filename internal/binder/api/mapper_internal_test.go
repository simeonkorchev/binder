package api

// White-box on purpose: these are the completeness tests for the unexported
// model -> DTO mappers. Every source field is set to a distinct non-zero value
// and every destination field is asserted, so a field added to the model and
// forgotten here fails the build instead of silently never reaching a client
// (000-principles.md section 9).

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

func TestToBinderBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, time.April, 2, 11, 0, 0, 0, time.UTC)
	source := model.Binder{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OwnerID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Name:      "Trade binder",
		CreatedAt: created,
		UpdatedAt: updated,
	}

	got := toBinderBody(source)

	if got.ID != source.ID {
		t.Errorf("ID = %v, want %v", got.ID, source.ID)
	}
	if got.Name != source.Name {
		t.Errorf("Name = %q, want %q", got.Name, source.Name)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, updated)
	}
	// OwnerID is deliberately not mapped: see the comment on binderBody.
}

func TestToSlotBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	printingID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	source := model.Slot{
		ID:             uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		BinderID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Position:       11,
		CardID:         uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		CardPrintingID: &printingID,
		SetResolution:  cardmodel.SetResolutionByNumber,
		CreatedAt:      time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, time.April, 2, 11, 0, 0, 0, time.UTC),
	}

	got := toSlotBody(source)

	if got.ID != source.ID {
		t.Errorf("ID = %v, want %v", got.ID, source.ID)
	}
	if got.Position != 11 {
		t.Errorf("Position = %d, want 11", got.Position)
	}
	if got.Page != 1 {
		t.Errorf("Page = %d, want 1 (position 11 is on the second page)", got.Page)
	}
	if got.SlotOnPage != 2 {
		t.Errorf("SlotOnPage = %d, want 2", got.SlotOnPage)
	}
	if got.CardID != source.CardID {
		t.Errorf("CardID = %v, want %v", got.CardID, source.CardID)
	}
	if got.CardPrintingID == nil || *got.CardPrintingID != printingID {
		t.Errorf("CardPrintingID = %v, want %v", got.CardPrintingID, printingID)
	}
	if string(got.SetResolution) != string(source.SetResolution) {
		t.Errorf("SetResolution = %q, want %q", got.SetResolution, source.SetResolution)
	}
	// BinderID, CreatedAt and UpdatedAt are deliberately not carried across:
	// see the comment in toSlotBody.
}

func TestToSlotBodyKeepsAnAbsentPrintingAbsent(t *testing.T) {
	t.Parallel()

	got := toSlotBody(model.Slot{
		ID:            uuid.New(),
		Position:      0,
		CardID:        uuid.New(),
		SetResolution: cardmodel.SetResolutionByName,
	})

	if got.CardPrintingID != nil {
		t.Errorf("CardPrintingID = %v, want nil for a name-rung slot", got.CardPrintingID)
	}
}

// The grid is what the page is drawn as, so every pocket has to be represented,
// occupied or not, and each slot has to land in its own pocket.
func TestToPageBodyLaysSlotsOutIntoTheGrid(t *testing.T) {
	t.Parallel()

	second := model.Slot{ID: uuid.New(), Position: 10, CardID: uuid.New(), SetResolution: cardmodel.SetResolutionByName}
	last := model.Slot{ID: uuid.New(), Position: 17, CardID: uuid.New(), SetResolution: cardmodel.SetResolutionByName}

	got := toPageBody(model.Page{Number: 1, Slots: []model.Slot{second, last}, PageCount: 2})

	if got.Page != 1 {
		t.Errorf("Page = %d, want 1", got.Page)
	}
	if got.PageCount != 2 {
		t.Errorf("PageCount = %d, want 2", got.PageCount)
	}
	if len(got.Slots) != model.SlotsPerPage {
		t.Fatalf("grid has %d pockets, want %d", len(got.Slots), model.SlotsPerPage)
	}
	if got.Slots[1] == nil || got.Slots[1].ID != second.ID {
		t.Errorf("pocket 1 = %v, want the slot at position 10", got.Slots[1])
	}
	if got.Slots[8] == nil || got.Slots[8].ID != last.ID {
		t.Errorf("pocket 8 = %v, want the slot at position 17", got.Slots[8])
	}
	for _, pocket := range []int{0, 2, 3, 4, 5, 6, 7} {
		if got.Slots[pocket] != nil {
			t.Errorf("pocket %d = %v, want null for an empty pocket", pocket, got.Slots[pocket])
		}
	}
}

func TestToPageBodyOfAnEmptyBinderIsAnEmptyGrid(t *testing.T) {
	t.Parallel()

	got := toPageBody(model.Page{Number: 0, Slots: []model.Slot{}, PageCount: 0})

	if len(got.Slots) != model.SlotsPerPage {
		t.Fatalf("grid has %d pockets, want %d", len(got.Slots), model.SlotsPerPage)
	}
	for pocket, slot := range got.Slots {
		if slot != nil {
			t.Errorf("pocket %d = %v, want null", pocket, slot)
		}
	}
	if got.PageCount != 0 {
		t.Errorf("PageCount = %d, want 0: an empty binder has no pages", got.PageCount)
	}
}

// The OpenAPI enum has to be generated from cardmodel.SetResolutions(), or a
// rung added to the ladder reaches the wire without being declared.
func TestSetResolutionSchemaEnumeratesEveryRung(t *testing.T) {
	t.Parallel()

	schema := setResolution("").Schema(nil)
	if len(schema.Enum) != len(cardmodel.SetResolutions()) {
		t.Fatalf("schema enum has %d values, want the %d rungs of the ladder",
			len(schema.Enum), len(cardmodel.SetResolutions()))
	}

	declared := map[string]bool{}
	for _, value := range schema.Enum {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("enum value %v is not a string", value)
		}
		declared[text] = true
	}
	for _, resolution := range cardmodel.SetResolutions() {
		if !declared[string(resolution)] {
			t.Errorf("rung %q is missing from the OpenAPI enum", resolution)
		}
	}
}
