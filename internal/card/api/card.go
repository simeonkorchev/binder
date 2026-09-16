package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simeonkorchev/binder/internal/card/model"
)

// cardsPerPage is how many cards one page of the search holds. It is fixed
// rather than a query parameter because nothing asks to vary it: the scanner's
// correction sheet shows a list the user scrolls, and a client that wants more
// asks for the next page.
const cardsPerPage = 50

// CardService searches the imported card database.
//
// It is faked through api.Service, which embeds it.
type CardService interface {
	SearchCards(ctx context.Context, search model.CardSearch) ([]model.Card, error)
}

type searchCardsInput struct {
	// Query matches anywhere in the card name, case-insensitively.
	Query string `query:"q" required:"false" maxLength:"256" doc:"Part of a card name."`
	// SetPrefix restricts the result to cards with a printing in that set.
	SetPrefix string `query:"set" required:"false" maxLength:"16" doc:"A set prefix, e.g. LOB."`
	Page      int    `query:"page" required:"false" default:"1" minimum:"1" doc:"1-based page number."`
}

type searchCardsOutput struct {
	Body searchCardsBody
}

// searchCardsBody wraps the cards in an object rather than returning a bare
// array, so a later addition (a total, a cursor) is not a breaking change.
type searchCardsBody struct {
	// Cards is empty, never null: a page past the end of the results is an
	// empty page, not an error (000-principles.md section 8b).
	Cards []scanCard `json:"cards"`
}

func registerCardEndpoints(api huma.API, svc CardService) {
	huma.Register(api,
		newOp(http.MethodGet, "/cards", "search-cards",
			"Search the card database by name and set.",
			http.StatusOK),
		func(ctx context.Context, req *searchCardsInput) (*searchCardsOutput, error) {
			cards, err := svc.SearchCards(ctx, model.CardSearch{
				Query:     req.Query,
				SetPrefix: req.SetPrefix,
				Page:      req.Page,
				PageSize:  cardsPerPage,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("searching cards: %w", err))
			}

			return &searchCardsOutput{Body: searchCardsBody{Cards: toScanCards(cards)}}, nil
		})
}

func toScanCards(cards []model.Card) []scanCard {
	mapped := make([]scanCard, 0, len(cards))
	for _, card := range cards {
		mapped = append(mapped, toScanCard(card))
	}
	return mapped
}
