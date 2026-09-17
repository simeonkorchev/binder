// Package service holds the listing domain's business logic: who may put a card
// up for sale or take it down, what a buyer browsing sees, and how much of a
// seller a buyer is allowed to be told.
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var (
	// ErrBinderSlotNotFound is returned for a slot that does not exist and for
	// one in somebody else's binder. They are deliberately the same answer: a
	// distinct "forbidden" would confirm to a stranger that the slot exists,
	// which is the precedent the binder domain set for its own ids.
	ErrBinderSlotNotFound = errors.New("binder slot not found")
	// ErrAlreadyListed is returned when the slot is already for sale. One
	// listing per slot is a UNIQUE constraint, so this comes back from the
	// insert rather than from a read before it: a check first would let a
	// concurrent request slip between the two.
	ErrAlreadyListed = errors.New("binder slot is already listed")
	// ErrListingNotFound is returned for a listing that does not exist and for
	// one that is somebody else's, for the same reason as ErrBinderSlotNotFound.
	// A card taken out of its binder takes its listing with it (ON DELETE
	// CASCADE), so a listing that was there a moment ago is this answer too.
	ErrListingNotFound = errors.New("listing not found")
	// ErrSellerNotFound is returned when there is no such user at all. It is
	// not the answer for a seller who shared no contact details: that is a
	// successful, empty response (000-principles.md section 8b).
	ErrSellerNotFound = errors.New("seller not found")
)

// Store is the listing domain's persistence surface.
//
//counterfeiter:generate . Store
type Store interface {
	// InTx runs cb inside a transaction, joining one already on ctx.
	InTx(ctx context.Context, cb func(ctx context.Context) error) error
	// InsertListing returns a dataerror.ConflictError when the slot is already
	// listed, and a dataerror.InvalidReferenceError over
	// model.ReferenceBinderSlot when there is no such slot.
	InsertListing(ctx context.Context, listing model.Listing) (model.Listing, error)
	// DeleteListingByID returns a dataerror.MissingEntityError when there is no
	// such listing, rather than reporting an unlisting that did not happen.
	DeleteListingByID(ctx context.Context, id uuid.UUID) error
	// SlotSeller is the owner of the binder a slot is in, and a
	// dataerror.MissingEntityError when there is no such slot.
	SlotSeller(ctx context.Context, binderSlotID uuid.UUID) (uuid.UUID, error)
	// ListingSeller is the owner of the binder a listing's slot is in, and a
	// dataerror.MissingEntityError when there is no such listing.
	ListingSeller(ctx context.Context, listingID uuid.UUID) (uuid.UUID, error)
	// BrowseListings returns an empty slice when nothing matches the search.
	BrowseListings(ctx context.Context, search model.ListingSearch) ([]model.ListedCard, error)
}

// SellerContacts reads the contact details a seller has opted into sharing.
//
// The details themselves belong to the user domain; this interface is declared
// here, next to the only code that reads them, and says exactly what this domain
// asks for (002-go-conventions.md section 3a). The user domain's service
// satisfies it at wiring time in main.go, so neither domain holds the other's
// concrete service.
//
// The contract, so a fake and the real implementation cannot drift: a user who
// has opted into nothing is a SellerContact with both fields nil and a nil
// error — the default row in db/migrations/002_users.sql, and the normal state.
// Only a userID that names no user at all is a
// dataerror.MissingEntityError.
//
//counterfeiter:generate . SellerContacts
type SellerContacts interface {
	ContactFor(ctx context.Context, userID uuid.UUID) (model.SellerContact, error)
}

// Service is the listing domain's business logic.
type Service struct {
	store   Store
	sellers SellerContacts
}

// NewService returns a service over store, reading seller contact details
// through sellers.
func NewService(store Store, sellers SellerContacts) *Service {
	return &Service{store: store, sellers: sellers}
}
