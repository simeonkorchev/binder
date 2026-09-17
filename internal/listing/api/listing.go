package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

// listingsPerBrowse is how many listings one browse answers with, newest first.
// It is fixed rather than a query parameter because the contract has none: a
// buyer narrows the feed with q and set, and the MVP has no paging to offer.
// The summary of the operation says so, so the cap is a stated limit rather
// than a silent truncation.
const listingsPerBrowse = 50

// errNoActor is what a handler returns when ActorFunc cannot name the user.
// Built once: there is nothing request-specific in a 401.
var errNoActor = huma.Error401Unauthorized("Sign in to buy and sell cards.")

// ListingService is the half of the domain that owns listings themselves.
//
// It is faked through api.Service, which embeds it.
type ListingService interface {
	CreateListing(ctx context.Context, ownerID, binderSlotID uuid.UUID) (model.Listing, error)
	DeleteListing(ctx context.Context, ownerID, listingID uuid.UUID) error
	BrowseListings(ctx context.Context, search model.ListingSearch) ([]model.ListedCard, error)
}

// listingBody is one card offered for sale, as its seller sees it.
//
// It carries no card and no seller: the slot names the card, the actor is the
// seller, and repeating either would be one more thing that can disagree with
// the binder.
type listingBody struct {
	ID           uuid.UUID `json:"id"`
	BinderSlotID uuid.UUID `json:"binderSlotId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func toListingBody(listing model.Listing) listingBody {
	return listingBody{
		ID:           listing.ID,
		BinderSlotID: listing.BinderSlotID,
		CreatedAt:    listing.CreatedAt,
		UpdatedAt:    listing.UpdatedAt,
	}
}

type createListingInput struct {
	Body createListingBody
}

// createListingBody names the card to put up for sale. It is the slot rather
// than the card: the same card in two binders, or twice in one, is two
// different pieces of cardboard, and only one of them is being sold.
type createListingBody struct {
	BinderSlotID uuid.UUID `json:"binderSlotId" doc:"The binder slot holding the card to sell."`
}

// Resolve refuses the nil uuid, which is representable but names no slot that
// can ever exist. Rejecting it here tells the client which field is wrong
// instead of answering a well-formed request with "not found"
// (000-principles.md section 10). The service still checks the slot itself —
// this guard is not a licence for the layer below to assume.
func (b createListingBody) Resolve(huma.Context) []error {
	if b.BinderSlotID == uuid.Nil {
		return []error{&huma.ErrorDetail{
			Message:  "binderSlotId has to name a binder slot.",
			Location: "body.binderSlotId",
			Value:    b.BinderSlotID.String(),
		}}
	}
	return nil
}

type listingOutput struct {
	Body listingBody
}

type deleteListingInput struct {
	ListingID uuid.UUID `path:"listingId"`
}

type browseListingsInput struct {
	// Query matches anywhere in the card name, case-insensitively.
	Query string `query:"q" required:"false" maxLength:"256" doc:"Part of a card name."`
	// SetPrefix keeps only listings whose card was resolved to a printing in
	// that set. A listing whose set was never resolved names no set, so it is
	// not in a filtered result.
	SetPrefix string `query:"set" required:"false" maxLength:"16" doc:"A set prefix, e.g. LOB."`
}

type browseListingsOutput struct {
	Body browseListingsBody
}

// browseListingsBody wraps the listings in an object rather than returning a
// bare array, so a later addition (a total, a cursor) is not a breaking change.
type browseListingsBody struct {
	// Listings is empty, never null: a marketplace with nothing in it yet, and a
	// filter that matches nothing, are both ordinary (000-principles.md
	// section 8b).
	Listings []listedCardBody `json:"listings"`
}

// listedCardBody is one card for sale in the browse feed.
//
// SellerID is as much of the seller as a feed carries: the contact details
// behind it are a separate, deliberate request, so scrolling cannot collect
// other people's email addresses.
type listedCardBody struct {
	ListingID uuid.UUID `json:"listingId"`
	SellerID  uuid.UUID `json:"sellerId"`
	CardID    uuid.UUID `json:"cardId"`
	CardName  string    `json:"cardName"`
	// ImageObjectKey is null until cmd/cardimages has fetched the image. The
	// client turns it into a URL with the bucket host it was configured with.
	ImageObjectKey *string `json:"imageObjectKey"`
	// SetCode is null when the seller's scan never determined which set the card
	// was printed in, which is what the client shows as an unknown set.
	SetCode  *string   `json:"setCode"`
	ListedAt time.Time `json:"listedAt"`
}

func toListedCardBody(listed model.ListedCard) listedCardBody {
	return listedCardBody{
		ListingID:      listed.ListingID,
		SellerID:       listed.SellerID,
		CardID:         listed.Card.ID,
		CardName:       listed.Card.Name,
		ImageObjectKey: listed.Card.ImageObjectKey,
		SetCode:        listed.SetCode,
		ListedAt:       listed.ListedAt,
	}
}

func toListedCardBodies(listed []model.ListedCard) []listedCardBody {
	mapped := make([]listedCardBody, 0, len(listed))
	for _, one := range listed {
		mapped = append(mapped, toListedCardBody(one))
	}
	return mapped
}

func registerListingEndpoints(api huma.API, svc ListingService, actor ActorFunc) {
	registerCreateListing(api, svc, actor)
	registerDeleteListing(api, svc, actor)
	registerBrowseListings(api, svc)
}

func registerCreateListing(api huma.API, svc ListingService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPost, "/listings", "create-listing",
			"Mark one of your cards for sale.", http.StatusCreated),
		func(ctx context.Context, req *createListingInput) (*listingOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			listing, err := svc.CreateListing(ctx, ownerID, req.Body.BinderSlotID)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("marking a card for sale: %w", err))
			}
			return &listingOutput{Body: toListingBody(listing)}, nil
		})
}

func registerDeleteListing(api huma.API, svc ListingService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodDelete, "/listings/{listingId}", "delete-listing",
			"Take one of your cards off sale. The card stays in the binder.", http.StatusNoContent),
		func(ctx context.Context, req *deleteListingInput) (*struct{}, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			if err := svc.DeleteListing(ctx, ownerID, req.ListingID); err != nil {
				return nil, handleErr(ctx, fmt.Errorf("taking a card off sale: %w", err))
			}
			return nil, nil //nolint:nilnil // 204: huma sends no body for an empty output struct.
		})
}

// registerBrowseListings takes no ActorFunc: the feed is the same for everybody
// and carries nothing that belongs to one person, so there is nothing for a
// session to decide here.
func registerBrowseListings(api huma.API, svc ListingService) {
	huma.Register(api,
		newOp(http.MethodGet, "/listings", "browse-listings",
			fmt.Sprintf("Browse cards for sale: the %d most recently listed that match the filter.",
				listingsPerBrowse),
			http.StatusOK),
		func(ctx context.Context, req *browseListingsInput) (*browseListingsOutput, error) {
			listed, err := svc.BrowseListings(ctx, model.ListingSearch{
				Query:     req.Query,
				SetPrefix: req.SetPrefix,
				Limit:     listingsPerBrowse,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("browsing listings: %w", err))
			}

			return &browseListingsOutput{
				Body: browseListingsBody{Listings: toListedCardBodies(listed)},
			}, nil
		})
}
