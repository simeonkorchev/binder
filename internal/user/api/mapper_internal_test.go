package api

// This file is white-box on purpose: it drives toUserBody and toSessionBody,
// which are unexported, and pins that each carries every field it is meant to —
// and that the ones it deliberately drops stay dropped. A field added to
// model.User and forgotten in a mapper compiles and passes every other test in
// the package (000-principles.md section 9).

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/model"
)

// fullUser has every field set to a distinct non-zero value, so a mapper that
// drops one is visible.
func fullUser() model.User {
	email := "seller@example.com"
	phone := "+359888123456"
	return model.User{
		ID:        uuid.MustParse("6f9619ff-8b86-d011-b42d-00c04fc964ff"),
		Identity:  model.Identity{Provider: model.ProviderGoogle, Subject: "109876543210987654321"},
		Contact:   model.Contact{Email: &email, Phone: &phone},
		CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.April, 2, 11, 30, 0, 0, time.UTC),
	}
}

func TestToUserBodyMapsEveryFieldItCarries(t *testing.T) {
	t.Parallel()

	user := fullUser()
	got := toUserBody(user)

	want := userBody{
		ID:           user.ID,
		ContactEmail: user.Contact.Email,
		ContactPhone: user.Contact.Phone,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toUserBody\n got %+v\nwant %+v", got, want)
	}
}

// The omissions are deliberate and documented on userBody. Pinning them here is
// what makes "we chose not to expose this" different from "somebody forgot":
// adding Identity or a timestamp to the DTO has to come with a decision to
// change this test.
func TestToUserBodyDropsTheIdentityAndTheTimestamps(t *testing.T) {
	t.Parallel()

	const carried = 3
	if got := reflect.TypeOf(userBody{}).NumField(); got != carried {
		t.Errorf(
			"userBody has %d fields, %d are pinned by TestToUserBodyMapsEveryFieldItCarries: "+
				"a field added to the wire needs a mapping and an assertion, "+
				"and a field deliberately not carried needs the comment on userBody to say so",
			got, carried)
	}
}

func TestToUserBodyKeepsUnsharedContactNull(t *testing.T) {
	t.Parallel()

	user := fullUser()
	user.Contact = model.Contact{Email: nil, Phone: nil}

	got := toUserBody(user)
	if got.ContactEmail != nil || got.ContactPhone != nil {
		t.Errorf("toUserBody turned an unshared contact into %+v, want both nil", got)
	}
}

func TestToSessionBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	session := model.Session{
		Token:     "the-session-token",
		ExpiresAt: time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC),
		User:      fullUser(),
	}

	got := toSessionBody(session)
	want := sessionBody{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
		User:      toUserBody(session.User),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toSessionBody\n got %+v\nwant %+v", got, want)
	}

	const carried = 3
	if fields := reflect.TypeOf(sessionBody{}).NumField(); fields != carried {
		t.Errorf("sessionBody has %d fields, %d are asserted above", fields, carried)
	}
}

// setContactBody is the other direction: a request shaped into the domain's type.
// A field added to the body and not to contact() is one the user can send and
// nothing will ever store.
func TestSetContactBodyMapsEveryFieldIntoTheDomain(t *testing.T) {
	t.Parallel()

	email := "seller@example.com"
	phone := "+359888123456"
	body := setContactBody{Email: &email, Phone: &phone}

	want := model.Contact{Email: &email, Phone: &phone}
	if got := body.contact(); !reflect.DeepEqual(got, want) {
		t.Errorf("setContactBody.contact()\n got %+v\nwant %+v", got, want)
	}

	if fields := reflect.TypeOf(setContactBody{}).NumField(); fields != reflect.TypeOf(model.Contact{}).NumField() {
		t.Errorf(
			"setContactBody has %d fields and model.Contact has %d: "+
				"a contact detail on the wire with nowhere to go is one the user cannot actually publish",
			fields, reflect.TypeOf(model.Contact{}).NumField())
	}
}
