package api

// White-box on purpose: these are the completeness tests for the unexported
// model -> DTO mappers. Every source field is set to a distinct non-zero value
// and every destination field is asserted, so a field added to the model and
// forgotten here fails the build instead of silently never reaching a client
// (000-principles.md section 9).

import (
	"testing"
	"time"

	"github.com/google/uuid"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

func TestToListingBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, time.April, 2, 11, 0, 0, 0, time.UTC)
	source := model.Listing{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		BinderSlotID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CreatedAt:    created,
		UpdatedAt:    updated,
	}

	got := toListingBody(source)

	if got.ID != source.ID {
		t.Errorf("ID = %v, want %v", got.ID, source.ID)
	}
	if got.BinderSlotID != source.BinderSlotID {
		t.Errorf("BinderSlotID = %v, want %v", got.BinderSlotID, source.BinderSlotID)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, updated)
	}
}

func TestToListedCardBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	objectKey := "cards/blue-eyes.jpg"
	setCode := "LOB-EN001"
	listedAt := time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)
	source := model.ListedCard{
		ListingID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		SellerID:  uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Card: cardmodel.Card{
			ID:             uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			Name:           "Blue-Eyes White Dragon",
			ImageObjectKey: &objectKey,
		},
		SetCode:  &setCode,
		ListedAt: listedAt,
	}

	got := toListedCardBody(source)

	if got.ListingID != source.ListingID {
		t.Errorf("ListingID = %v, want %v", got.ListingID, source.ListingID)
	}
	if got.SellerID != source.SellerID {
		t.Errorf("SellerID = %v, want %v", got.SellerID, source.SellerID)
	}
	if got.CardID != source.Card.ID {
		t.Errorf("CardID = %v, want %v", got.CardID, source.Card.ID)
	}
	if got.CardName != source.Card.Name {
		t.Errorf("CardName = %q, want %q", got.CardName, source.Card.Name)
	}
	if got.ImageObjectKey == nil || *got.ImageObjectKey != objectKey {
		t.Errorf("ImageObjectKey = %v, want %q", got.ImageObjectKey, objectKey)
	}
	if got.SetCode == nil || *got.SetCode != setCode {
		t.Errorf("SetCode = %v, want %q", got.SetCode, setCode)
	}
	if !got.ListedAt.Equal(listedAt) {
		t.Errorf("ListedAt = %v, want %v", got.ListedAt, listedAt)
	}
}

// A card whose set was never resolved names no set, and a card whose image has
// not been fetched yet has no object key. Both stay absent rather than becoming
// an empty string the client would have to recognise.
func TestToListedCardBodyKeepsAbsentFieldsAbsent(t *testing.T) {
	t.Parallel()

	got := toListedCardBody(model.ListedCard{
		ListingID: uuid.New(),
		SellerID:  uuid.New(),
		Card: cardmodel.Card{
			ID:             uuid.New(),
			Name:           "Kuriboh",
			ImageObjectKey: nil,
		},
		SetCode:  nil,
		ListedAt: time.Now(),
	})

	if got.SetCode != nil {
		t.Errorf("SetCode = %v, want nil for a card whose set was never resolved", got.SetCode)
	}
	if got.ImageObjectKey != nil {
		t.Errorf("ImageObjectKey = %v, want nil for a card with no image yet", got.ImageObjectKey)
	}
}

func TestToSellerContactBodyMapsEveryField(t *testing.T) {
	t.Parallel()

	email := "seller@example.test"
	phone := "+359888123456"

	got := toSellerContactBody(model.SellerContact{Email: &email, Phone: &phone})

	if got.Email == nil || *got.Email != email {
		t.Errorf("Email = %v, want %q", got.Email, email)
	}
	if got.Phone == nil || *got.Phone != phone {
		t.Errorf("Phone = %v, want %q", got.Phone, phone)
	}
}

// The whole point of D4: a seller who opted into nothing is representable, and
// what a buyer gets is two absent fields rather than an empty string that reads
// as a shared, blank address.
func TestToSellerContactBodyOfASellerWhoSharedNothingIsEmpty(t *testing.T) {
	t.Parallel()

	got := toSellerContactBody(model.SellerContact{Email: nil, Phone: nil})

	if got.Email != nil {
		t.Errorf("Email = %v, want nil", got.Email)
	}
	if got.Phone != nil {
		t.Errorf("Phone = %v, want nil", got.Phone)
	}
}
