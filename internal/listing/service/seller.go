package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

// SellerContact is how a buyer reaches a seller: the email and phone number
// that seller opted into publishing, and nothing else about them.
//
// A seller who opted into neither is a successful response with both fields
// absent, not a 404 — "this seller shared no contact details" is a normal state
// of the account, and the client's answer to it is to show no contact button
// (D4, 000-principles.md section 8b). Only an id that names no user at all is
// not found.
func (s *Service) SellerContact(ctx context.Context, sellerID uuid.UUID) (model.SellerContact, error) {
	contact, err := s.sellers.ContactFor(ctx, sellerID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return model.SellerContact{}, ErrSellerNotFound
		}
		return model.SellerContact{}, fmt.Errorf("getting the seller's contact details: %w", err)
	}
	return contact, nil
}
