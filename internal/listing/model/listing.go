// Package model holds the listing domain's types.
//
// A listing is a flag on a binder slot, not a copy of it: the card, its
// printing and the seller are all reachable from the slot it points at, so
// nothing here repeats them (db/migrations/004_listings.sql).
package model

import (
	"time"

	"github.com/google/uuid"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

// Listing is a binder slot its owner has marked for sale.
//
// There is deliberately no seller field: the seller is the owner of the binder
// the slot is in, and carrying it here would be a second place to get it wrong.
// There is no price either — price data is out of scope for the MVP.
type Listing struct {
	ID           uuid.UUID
	BinderSlotID uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListedCard is one listing as a buyer browsing them sees it: what is for sale,
// and who to ask about it.
//
// It carries no contact details on purpose. Revealing those is a separate,
// deliberate request for one seller, so scrolling a feed cannot collect other
// people's email addresses.
type ListedCard struct {
	ListingID uuid.UUID
	// SellerID is the owner of the binder the listed slot is in — the id a
	// buyer then asks for contact details with.
	SellerID uuid.UUID
	Card     cardmodel.Card
	// SetCode is the printed code of the printing the seller's scan resolved
	// ("LOB-EN005"), and nil when it resolved none: the name rung of the match
	// ladder identifies a card but not the set it was printed in.
	SetCode  *string
	ListedAt time.Time
}

// ListingSearch is the filter behind the browse feed.
type ListingSearch struct {
	// Query matches anywhere in the card's name, case-insensitively. Empty
	// matches every listed card.
	Query string
	// SetPrefix restricts the result to listings whose slot resolved to a
	// printing in that set ("LOB"), matched exactly as it is printed. Empty
	// does not restrict; a non-empty value excludes listings whose set was
	// never resolved, because those name no set to compare it with.
	SetPrefix string
	// Limit is how many listings to return, newest first.
	Limit int
}

// SellerContact is the contact details a seller has opted into sharing.
//
// Both fields nil is the normal state and not an error: a seller who opted into
// neither simply has no contact button (000-principles.md section 8b). Which
// fields exist at all is the user domain's decision — see 002_users.sql, where
// both columns are nullable and blank strings are refused, so "shared" stays
// distinguishable from "not shared".
type SellerContact struct {
	Email *string
	Phone *string
}

// ReferenceBinderSlot names the row a listing points at, in the vocabulary a
// dataerror.InvalidReferenceError carries. The store produces it when the
// foreign key rejects a write and the service matches on it to decide which
// sentinel the caller gets, so it is one fact here rather than two string
// literals in two layers that can drift apart.
const ReferenceBinderSlot = "binder slot"
