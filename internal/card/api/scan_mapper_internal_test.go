package api

// White-box on purpose: these are the completeness tests for the unexported
// model -> DTO mappers. Every source field is set to a distinct non-zero value
// and every destination field is asserted, so a field added to the model and
// forgotten here fails the build instead of silently never reaching a client
// (000-principles.md section 9).

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/card/model"
)

func fullCard() model.Card {
	key := "cards/dark-magician.jpg"
	return model.Card{
		ID:             uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:           "Dark Magician",
		ImageObjectKey: &key,
	}
}

func fullPrinting() model.CardPrinting {
	return model.CardPrinting{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CardID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SetCode: "LOB-EN005",
		Rarity:  "Ultra Rare",
	}
}

func TestToScanCardMapsEveryField(t *testing.T) {
	t.Parallel()

	source := fullCard()
	got := toScanCard(source)

	if got.ID != source.ID {
		t.Errorf("ID = %v, want %v", got.ID, source.ID)
	}
	if got.Name != source.Name {
		t.Errorf("Name = %q, want %q", got.Name, source.Name)
	}
	if got.ImageObjectKey == nil || *got.ImageObjectKey != *source.ImageObjectKey {
		t.Errorf("ImageObjectKey = %v, want %q", got.ImageObjectKey, *source.ImageObjectKey)
	}
}

func TestToScanPrintingMapsEveryField(t *testing.T) {
	t.Parallel()

	source := fullPrinting()
	got := toScanPrinting(source)

	if got.ID != source.ID {
		t.Errorf("ID = %v, want %v", got.ID, source.ID)
	}
	if got.CardID != source.CardID {
		t.Errorf("CardID = %v, want %v", got.CardID, source.CardID)
	}
	if got.SetCode != source.SetCode {
		t.Errorf("SetCode = %q, want %q", got.SetCode, source.SetCode)
	}
	if got.Rarity != source.Rarity {
		t.Errorf("Rarity = %q, want %q", got.Rarity, source.Rarity)
	}
}

func TestToScanMatchMapsEveryField(t *testing.T) {
	t.Parallel()

	card := fullCard()
	printing := fullPrinting()

	got := toScanMatch(model.MatchResult{
		Outcome:    model.ScanOutcomeResolved,
		Resolution: model.SetResolutionByPrefixAndNumber,
		Card:       &card,
		Printing:   &printing,
		Candidates: []model.Candidate{{Card: card, Printing: &printing}},
	})

	if string(got.Outcome) != string(model.ScanOutcomeResolved) {
		t.Errorf("Outcome = %q, want %q", got.Outcome, model.ScanOutcomeResolved)
	}
	if string(got.Resolution) != string(model.SetResolutionByPrefixAndNumber) {
		t.Errorf("Resolution = %q, want %q", got.Resolution, model.SetResolutionByPrefixAndNumber)
	}
	if got.Card == nil || got.Card.ID != card.ID {
		t.Errorf("Card = %+v, want the card %v", got.Card, card.ID)
	}
	if got.Printing == nil || got.Printing.ID != printing.ID {
		t.Errorf("Printing = %+v, want the printing %v", got.Printing, printing.ID)
	}
	if len(got.Candidates) != 1 {
		t.Errorf("Candidates has %d entries, want 1", len(got.Candidates))
	}
}

func TestToScanCandidatesMapsEveryField(t *testing.T) {
	t.Parallel()

	card := fullCard()
	printing := fullPrinting()
	nameRungCard := model.Card{
		ID:             uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Name:           "Dark Magician Girl",
		ImageObjectKey: nil,
	}

	got := toScanCandidates([]model.Candidate{
		{Card: card, Printing: &printing},
		{Card: nameRungCard, Printing: nil},
	})

	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	if got[0].Card.ID != card.ID || got[0].Printing == nil || got[0].Printing.ID != printing.ID {
		t.Errorf("code-rung candidate = %+v, want the card with its printing", got[0])
	}
	if got[1].Card.ID != nameRungCard.ID || got[1].Printing != nil {
		t.Errorf("name-rung candidate = %+v, want the card with no printing", got[1])
	}
}

// A nil card or printing has to stay nil: the three code rungs carry a printing
// and the other two must not, which is the binder_slots CHECK.
func TestToScanMatchKeepsAnAbsentCardAndPrintingAbsent(t *testing.T) {
	t.Parallel()

	got := toScanMatch(model.MatchResult{
		Outcome:    model.ScanOutcomeNoMatch,
		Resolution: model.SetResolutionUnresolved,
		Card:       nil,
		Printing:   nil,
		Candidates: nil,
	})

	if string(got.Outcome) != string(model.ScanOutcomeNoMatch) {
		t.Errorf("Outcome = %q, want %q", got.Outcome, model.ScanOutcomeNoMatch)
	}
	if got.Card != nil {
		t.Errorf("Card = %+v, want nil", got.Card)
	}
	if got.Printing != nil {
		t.Errorf("Printing = %+v, want nil", got.Printing)
	}
	if got.Candidates == nil {
		t.Error("Candidates is nil, which serialises as null; an empty result must serialise as []")
	}
}

// The OpenAPI enum has to be generated from model.ScanResolutions(), or a rung
// added to the ladder reaches the wire without being declared.
//
// ScanResolutions and not SetResolutions: this endpoint answers rungs, and
// `manual` is a value only a person produces. Publishing it here would document
// an answer the ladder cannot give, which is the same kind of untruth the
// outcome field exists to end.
func TestSetResolutionSchemaEnumeratesEveryRung(t *testing.T) {
	t.Parallel()

	declared := enumValues(t, setResolution("").Schema(nil))

	if len(declared) != len(model.ScanResolutions()) {
		t.Fatalf("schema enum has %d values, want the %d rungs of the ladder",
			len(declared), len(model.ScanResolutions()))
	}
	for _, resolution := range model.ScanResolutions() {
		if !declared[string(resolution)] {
			t.Errorf("rung %q is missing from the OpenAPI enum", resolution)
		}
	}
	if declared[string(model.SetResolutionManual)] {
		t.Errorf("%q is in the scan enum, but no rung of the ladder can answer it",
			model.SetResolutionManual)
	}
}

// The outcome enum has to be generated from model.ScanOutcomes() for the same
// reason: an outcome added to the ladder must not reach the wire undeclared.
func TestScanOutcomeSchemaEnumeratesEveryOutcome(t *testing.T) {
	t.Parallel()

	declared := enumValues(t, scanOutcome("").Schema(nil))

	if len(declared) != len(model.ScanOutcomes()) {
		t.Fatalf("schema enum has %d values, want the %d outcomes",
			len(declared), len(model.ScanOutcomes()))
	}
	for _, outcome := range model.ScanOutcomes() {
		if !declared[string(outcome)] {
			t.Errorf("outcome %q is missing from the OpenAPI enum", outcome)
		}
	}
}

// enumValues reads a schema's enum into a set, failing the test rather than the
// caller when a value is not the string an enum is made of.
func enumValues(t *testing.T, schema *huma.Schema) map[string]bool {
	t.Helper()

	values := map[string]bool{}
	for _, value := range schema.Enum {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("enum value %v is not a string", value)
		}
		values[text] = true
	}
	return values
}
