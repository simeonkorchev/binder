package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/user/model"
)

// GetUser returns one account. It is what a signed-in user reads about itself,
// so the id always comes from the session rather than from a request.
func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (model.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("getting the account: %w", err)
	}
	return user, nil
}

// SetContact replaces the contact details a user publishes.
//
// It replaces rather than patches, because a contact preference is one decision:
// a caller that names only an email is saying the phone number is not shared.
// Patching would make "leave the other alone" and "stop sharing the other" the
// same request, and there would be no way to opt back out.
//
// A field that is nothing but whitespace is refused rather than stored or
// silently dropped — see ErrContactBlank.
func (s *Service) SetContact(
	ctx context.Context, userID uuid.UUID, contact model.Contact,
) (model.User, error) {
	normalised, blank := contact.Normalise()
	if len(blank) > 0 {
		// The field names, never the values: the values are the personal data
		// this domain exists to protect, and an error message is a thing that
		// gets logged.
		return model.User{}, fmt.Errorf("%w: %v", ErrContactBlank, blank)
	}

	user, err := s.store.SetUserContact(ctx, userID, normalised)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("setting the account's contact details: %w", err)
	}

	return user, nil
}

// SellerContact is the contact details one account has opted into publishing,
// and nothing else about it.
//
// It exists for the listing domain, which declares the interface this satisfies
// (internal/listing/service.SellerContacts) next to the code that reads it. The
// narrow return type is the point: the marketplace can reach a seller's published
// email and phone number and has no way to reach the provider subject, the
// timestamps or anything else on the account.
//
// A missing account is left as the dataerror.MissingEntityError the store
// produced, rather than converted to ErrUserNotFound, because that is the
// contract SellerContacts documents and the listing service branches on. A user
// who has opted into nothing is both fields nil and a nil error: not opting in is
// the normal state of an account, not a failure (000-principles.md section 8b).
func (s *Service) SellerContact(ctx context.Context, userID uuid.UUID) (model.Contact, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return model.Contact{}, fmt.Errorf("getting the seller's account: %w", err)
	}
	return user.Contact, nil
}
