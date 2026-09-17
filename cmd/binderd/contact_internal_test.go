// This file is in package main because what it pins is unexported: the
// seller-contact adapter that lets the listing domain read the user domain's
// contact details. The mapping has no exported surface to reach from outside,
// and it is a hand-written field-by-field copy between two structurally
// identical types — exactly the shape that silently drops a field
// (000-principles.md section 9).
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	usermodel "github.com/simeonkorchev/binder/internal/user/model"
)

// errAccountLookup is the don't-care failure: the test using it asserts that the
// error survives, never its text.
var errAccountLookup = errors.New("reading the account failed")

// stubAccounts answers SellerContact with whatever a test set on it.
type stubAccounts struct {
	contact usermodel.Contact
	err     error
	// calledWith records the id the adapter passed through.
	calledWith uuid.UUID
}

func (s *stubAccounts) SellerContact(_ context.Context, userID uuid.UUID) (usermodel.Contact, error) {
	s.calledWith = userID
	return s.contact, s.err
}

func TestContactForMapsEveryField(t *testing.T) {
	email := "seller@example.test"
	phone := "+359 888 123 456"
	accounts := &stubAccounts{contact: usermodel.Contact{Email: &email, Phone: &phone}}
	userID := uuid.New()

	contact, err := sellerContacts{accounts: accounts}.ContactFor(context.Background(), userID)
	if err != nil {
		t.Fatalf("ContactFor returned %v", err)
	}

	// Every field of listing/model.SellerContact is asserted, with values that
	// are distinct from each other: a mapping that swapped the two would pass a
	// test that only checked they were non-nil.
	if contact.Email == nil || *contact.Email != email {
		t.Errorf("Email = %v, want %q", contact.Email, email)
	}
	if contact.Phone == nil || *contact.Phone != phone {
		t.Errorf("Phone = %v, want %q", contact.Phone, phone)
	}
	if accounts.calledWith != userID {
		t.Errorf("asked for %v, want %v", accounts.calledWith, userID)
	}
}

func TestContactForCarriesTheOptedIntoNothingCase(t *testing.T) {
	accounts := &stubAccounts{}

	contact, err := sellerContacts{accounts: accounts}.ContactFor(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ContactFor returned %v, want nil: a seller who opted into nothing is not an error", err)
	}
	if contact.Email != nil || contact.Phone != nil {
		t.Errorf("contact = %+v, want both fields nil", contact)
	}
}

func TestContactForKeepsTheMissingAccountRecognisable(t *testing.T) {
	// The listing service decides between "no such seller" and a failure by
	// matching this type, so the wrap must not hide it.
	accounts := &stubAccounts{err: dataerror.WrapMissingEntityError("user", errAccountLookup)}

	_, err := sellerContacts{accounts: accounts}.ContactFor(context.Background(), uuid.New())

	if !dataerror.IsMissingEntityError(err) {
		t.Errorf("err = %v, want a dataerror.MissingEntityError", err)
	}
}

func TestContactForReturnsTheFailureItWasGiven(t *testing.T) {
	accounts := &stubAccounts{err: errAccountLookup}

	_, err := sellerContacts{accounts: accounts}.ContactFor(context.Background(), uuid.New())

	if !errors.Is(err, errAccountLookup) {
		t.Errorf("err = %v, want it to wrap %v", err, errAccountLookup)
	}
}
