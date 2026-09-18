package api

// White-box on purpose: this reads a struct tag on an unexported request type.
// It pins the batch bound the OpenAPI document publishes to the one the service
// enforces, which a struct tag cannot do for itself.

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

// A struct tag cannot read a Go constant, so addSlotsBody.Cards states
// model.MaxSlotsPerBatch as a literal. Without this test, moving the constant
// would leave the contract advertising a limit the service does not have — and
// clients batching to the published number would start getting 422s.
func TestBatchSchemaMaxItemsMatchesTheServicesBound(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeOf(addSlotsBody{}).FieldByName("Cards")
	if !ok {
		t.Fatal("addSlotsBody has no Cards field, so the bound below guards nothing")
	}

	declared := field.Tag.Get("maxItems")
	if declared == "" {
		t.Fatal("addSlotsBody.Cards has no maxItems, so the contract does not say how long a batch may be")
	}
	if declared != strconv.Itoa(model.MaxSlotsPerBatch) {
		t.Errorf("the schema allows %s cards per batch but the service allows %d; "+
			"they are one bound and must be one number", declared, model.MaxSlotsPerBatch)
	}

	// The prose beside the tag is read by every client developer and generated
	// into the document, so it is the same number or it is a lie
	// (007-clean-code-checklist.md, "code matches its own OpenAPI description").
	if doc := field.Tag.Get("doc"); !strings.Contains(doc, strconv.Itoa(model.MaxSlotsPerBatch)) {
		t.Errorf("the doc string %q does not name the bound %d it describes",
			doc, model.MaxSlotsPerBatch)
	}
}

// The request -> model mapper both writes share. Every source field is set to a
// distinct non-zero value and every destination field is asserted, so a field
// added to slotCardBody and forgotten here fails the build instead of silently
// never reaching the database (000-principles.md section 9).
func TestToSlotCardMapsEveryField(t *testing.T) {
	t.Parallel()

	printingID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	source := slotCardBody{
		CardID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CardPrintingID: &printingID,
		SetResolution:  setResolution(cardmodel.SetResolutionManual),
	}

	got := source.toSlotCard()

	if got.CardID != source.CardID {
		t.Errorf("CardID = %v, want %v", got.CardID, source.CardID)
	}
	if got.CardPrintingID == nil || *got.CardPrintingID != printingID {
		t.Errorf("CardPrintingID = %v, want %v", got.CardPrintingID, printingID)
	}
	if got.SetResolution != cardmodel.SetResolutionManual {
		t.Errorf("SetResolution = %q, want %q", got.SetResolution, cardmodel.SetResolutionManual)
	}
}

// A card with no printing must arrive with none: the name rung identifies a
// card and no set, and a nil turned into uuid.Nil would be a reference to a
// printing that does not exist.
func TestToSlotCardKeepsAnAbsentPrintingAbsent(t *testing.T) {
	t.Parallel()

	got := slotCardBody{
		CardID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CardPrintingID: nil,
		SetResolution:  setResolution(cardmodel.SetResolutionByName),
	}.toSlotCard()

	if got.CardPrintingID != nil {
		t.Errorf("CardPrintingID = %v, want nil", got.CardPrintingID)
	}
}

// addSlotBody repeats slotCardBody's fields because huma does not flatten an
// embedded struct into the parent schema. This is what keeps the repetition
// honest: the two must carry the same three values across.
func TestAddSlotBodyCarriesEveryCardField(t *testing.T) {
	t.Parallel()

	printingID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	position := 3
	body := addSlotBody{
		CardID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CardPrintingID: &printingID,
		SetResolution:  setResolution(cardmodel.SetResolutionExact),
		Position:       &position,
	}

	got := body.card()

	if got.CardID != body.CardID {
		t.Errorf("CardID = %v, want %v", got.CardID, body.CardID)
	}
	if got.CardPrintingID != body.CardPrintingID {
		t.Errorf("CardPrintingID = %v, want %v", got.CardPrintingID, body.CardPrintingID)
	}
	if got.SetResolution != body.SetResolution {
		t.Errorf("SetResolution = %q, want %q", got.SetResolution, body.SetResolution)
	}
}

// An empty batch's slots must serialise as [] and not null, on both sides of
// the request (000-principles.md section 8b).
func TestEmptyBatchMapsToEmptySlices(t *testing.T) {
	t.Parallel()

	if cards := toSlotCards(nil); cards == nil {
		t.Error("toSlotCards(nil) is nil, want an empty slice")
	}
	if slots := toSlotBodies(nil); slots == nil {
		t.Error("toSlotBodies(nil) is nil, which serialises as null")
	}
}

// Order is the payload: it is what decides the position each card lands at.
func TestToSlotCardsKeepsTheOrderItWasGiven(t *testing.T) {
	t.Parallel()

	first := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	second := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	got := toSlotCards([]slotCardBody{
		{CardID: first, CardPrintingID: nil, SetResolution: setResolution(cardmodel.SetResolutionByName)},
		{CardID: second, CardPrintingID: nil, SetResolution: setResolution(cardmodel.SetResolutionByName)},
	})

	if len(got) != 2 || got[0].CardID != first || got[1].CardID != second {
		t.Errorf("toSlotCards reordered or dropped its input: %+v", got)
	}
}

// model.SlotCard is what the mapper above fills. exhaustruct does not guard it
// — a printing is genuinely optional per card — so this is what proves the
// mapper is total over it: every field of the destination is accounted for
// above, and this fails if one is added.
func TestSlotCardHasNoFieldTheMapperIgnores(t *testing.T) {
	t.Parallel()

	const mapped = 3 // CardID, CardPrintingID, SetResolution

	if got := reflect.TypeOf(model.SlotCard{}).NumField(); got != mapped {
		t.Errorf("model.SlotCard has %d fields and toSlotCard maps %d; "+
			"map the new one and assert it in TestToSlotCardMapsEveryField", got, mapped)
	}
}
