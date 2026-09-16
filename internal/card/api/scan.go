package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/card/model"
)

// ScanService resolves the OCR text of one scanned card to a card and, where
// the ladder can tell, the set it was printed in.
//
// It is faked through api.Service, which embeds it.
type ScanService interface {
	ResolveScan(ctx context.Context, scan model.ScanInput) (model.MatchResult, error)
}

// scanCard is a card as this domain returns it. It is deliberately not
// model.Card: the object key is exposed under a name the client understands to
// be a key and not a URL, and a field added to the model does not reach the wire
// until a mapper here puts it there.
type scanCard struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// ImageObjectKey is null until cmd/cardimages has fetched the image. The
	// client turns it into a URL with the bucket host it was configured with.
	ImageObjectKey *string `json:"imageObjectKey"`
}

// scanPrinting is one card as printed in one set.
type scanPrinting struct {
	ID      uuid.UUID `json:"id"`
	CardID  uuid.UUID `json:"cardId"`
	SetCode string    `json:"setCode"`
	Rarity  string    `json:"rarity"`
}

// scanCandidate is one card the ladder could not rule out. Printing is null for
// a candidate the name rung produced, which names a card and no set.
type scanCandidate struct {
	Card     scanCard      `json:"card"`
	Printing *scanPrinting `json:"printing"`
}

// setResolution is model.SetResolution on the wire. It exists as its own type
// so the OpenAPI enum is generated from model.SetResolutions() rather than
// re-typed into a struct tag that the next rung added would not update.
type setResolution model.SetResolution

// Schema declares the enum from the one list of rungs.
func (setResolution) Schema(huma.Registry) *huma.Schema {
	resolutions := model.SetResolutions()
	values := make([]any, 0, len(resolutions))
	for _, resolution := range resolutions {
		values = append(values, string(resolution))
	}

	return &huma.Schema{
		Type:        huma.TypeString,
		Enum:        values,
		Description: "How the scanned card's set was determined.",
	}
}

// scanMatch is what the ladder concluded. Card is null only when nothing
// matched at all; Printing is non-null exactly for the rungs that resolve a set
// (model.RequiresPrinting), which is the same equivalence the binder_slots CHECK
// enforces in SQL.
type scanMatch struct {
	Resolution setResolution `json:"resolution"`
	Card       *scanCard     `json:"card"`
	Printing   *scanPrinting `json:"printing"`
	// Candidates is empty, never null, so a client can iterate it without a
	// null check. It carries what an ambiguous rung could not choose between.
	Candidates []scanCandidate `json:"candidates"`
}

type resolveScanInput struct {
	Body resolveScanBody
}

// resolveScanBody is the OCR text of one card, as the camera read it.
type resolveScanBody struct {
	Code string `json:"code" required:"false" maxLength:"64" doc:"The printed code line, ideally {PREFIX}-{NUMBER}."`
	Name string `json:"name" required:"false" maxLength:"256" doc:"The card-name line."`
}

// Resolve rejects a scan carrying neither line at all. It is the cheap half of
// the guard: the service rejects the subtler case where both lines are present
// but neither survives normalisation (a code of "--" has no number), and answers
// the same 422 for it. Both halves are needed — the boundary check is not a
// licence for the layer below to assume (000-principles.md section 10).
func (b resolveScanBody) Resolve(huma.Context) []error {
	if strings.TrimSpace(b.Code) != "" || strings.TrimSpace(b.Name) != "" {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  "A scan needs a printed code or a card name.",
		Location: "body",
		Value:    b,
	}}
}

type resolveScanOutput struct {
	Body scanMatch
}

func registerScanEndpoints(api huma.API, svc ScanService) {
	huma.Register(api,
		newOp(http.MethodPost, "/scans/resolve", "resolve-scan",
			"Resolve one scanned card to a card and, where it can be told, a printing.",
			http.StatusOK),
		func(ctx context.Context, req *resolveScanInput) (*resolveScanOutput, error) {
			match, err := svc.ResolveScan(ctx, model.ScanInput{
				Code: req.Body.Code,
				Name: req.Body.Name,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("resolving a scan: %w", err))
			}

			return &resolveScanOutput{Body: toScanMatch(match)}, nil
		})
}

// toScanMatch maps a match result onto the wire, field for field.
func toScanMatch(match model.MatchResult) scanMatch {
	return scanMatch{
		Resolution: setResolution(match.Resolution),
		Card:       toScanCardPtr(match.Card),
		Printing:   toScanPrintingPtr(match.Printing),
		Candidates: toScanCandidates(match.Candidates),
	}
}

func toScanCard(card model.Card) scanCard {
	return scanCard{
		ID:             card.ID,
		Name:           card.Name,
		ImageObjectKey: card.ImageObjectKey,
	}
}

func toScanCardPtr(card *model.Card) *scanCard {
	if card == nil {
		return nil
	}
	mapped := toScanCard(*card)
	return &mapped
}

func toScanPrinting(printing model.CardPrinting) scanPrinting {
	return scanPrinting{
		ID:      printing.ID,
		CardID:  printing.CardID,
		SetCode: printing.SetCode,
		Rarity:  printing.Rarity,
	}
}

func toScanPrintingPtr(printing *model.CardPrinting) *scanPrinting {
	if printing == nil {
		return nil
	}
	mapped := toScanPrinting(*printing)
	return &mapped
}

// toScanCandidates always returns a slice, so an unambiguous match serialises
// its candidates as [] rather than null (000-principles.md section 8b).
func toScanCandidates(candidates []model.Candidate) []scanCandidate {
	mapped := make([]scanCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		mapped = append(mapped, scanCandidate{
			Card:     toScanCard(candidate.Card),
			Printing: toScanPrintingPtr(candidate.Printing),
		})
	}
	return mapped
}
