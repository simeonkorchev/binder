package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	listingmodel "github.com/simeonkorchev/binder/internal/listing/model"
	usermodel "github.com/simeonkorchev/binder/internal/user/model"
)

// sellerContactReader is the user domain's half of the seller-contact seam: the
// one method this bootstrap needs in order to satisfy the listing domain's
// service.SellerContacts. It is declared here, at the consumer, so that neither
// domain imports the other's concrete service (002-go-conventions.md section 3a).
type sellerContactReader interface {
	SellerContact(ctx context.Context, userID uuid.UUID) (usermodel.Contact, error)
}

// sellerContacts lets the listing domain read a seller's published contact
// details out of the user domain.
//
// The two domains describe the same thing under different names — the listing
// service asks for ContactFor and a listing/model.SellerContact, the user service
// offers SellerContact and a user/model.Contact — and both are right to: a
// consumer-side interface exists so that a domain states what it needs in its own
// vocabulary. Making the two names and types agree is the wiring layer's job, and
// this is it. Nothing in either domain knows this type exists.
//
// The documented contract is carried across unchanged: a seller who has opted
// into nothing is both fields nil and a nil error, and only a userID naming no
// account at all is a dataerror.MissingEntityError — which survives the wrap
// below, because the listing service matches it with errors.As.
type sellerContacts struct {
	accounts sellerContactReader
}

// ContactFor implements internal/listing/service.SellerContacts.
func (s sellerContacts) ContactFor(ctx context.Context, userID uuid.UUID) (listingmodel.SellerContact, error) {
	contact, err := s.accounts.SellerContact(ctx, userID)
	if err != nil {
		return listingmodel.SellerContact{}, fmt.Errorf("reading the seller's contact details: %w", err)
	}

	// Both fields are mapped; the two types have no others (000-principles.md
	// section 9, pinned by the completeness spec in contact_internal_test.go).
	return listingmodel.SellerContact{
		Email: contact.Email,
		Phone: contact.Phone,
	}, nil
}
