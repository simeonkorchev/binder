package store

// This file is white-box on purpose: it builds a userRow, which is unexported,
// and pins that toUser carries every one of its fields into the model. A field
// added to the row and forgotten here is a column that saves and never reads
// back, which compiles and passes every other test in the package
// (000-principles.md section 9).

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/model"
)

func TestToUserMapsEveryField(t *testing.T) {
	t.Parallel()

	email := "seller@example.com"
	phone := "+359888123456"
	row := userRow{
		ID:           uuid.MustParse("6f9619ff-8b86-d011-b42d-00c04fc964ff"),
		AuthProvider: string(model.ProviderGoogle),
		AuthSubject:  "109876543210987654321",
		ContactEmail: &email,
		ContactPhone: &phone,
		CreatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.April, 2, 11, 30, 0, 0, time.UTC),
	}

	want := model.User{
		ID: row.ID,
		Identity: model.Identity{
			Provider: model.ProviderGoogle,
			Subject:  row.AuthSubject,
		},
		Contact: model.Contact{
			Email: &email,
			Phone: &phone,
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}

	if got := toUser(row); !reflect.DeepEqual(got, want) {
		t.Errorf("toUser(row)\n got %+v\nwant %+v", got, want)
	}
}

// A row where neither contact field is shared is the ordinary account, so the
// mapper has to produce nil rather than a pointer to an empty string.
func TestToUserKeepsUnsharedContactNil(t *testing.T) {
	t.Parallel()

	user := toUser(userRow{
		ID:           uuid.MustParse("6f9619ff-8b86-d011-b42d-00c04fc964ff"),
		AuthProvider: string(model.ProviderApple),
		AuthSubject:  "001234.apple.subject.5678",
		ContactEmail: nil,
		ContactPhone: nil,
		CreatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
	})

	if user.Contact.Email != nil || user.Contact.Phone != nil {
		t.Errorf("toUser turned an unshared contact into %+v, want both nil", user.Contact)
	}
}

// userRowFieldCount fails when a column is added to the row without the
// completeness test above being extended to cover it. reflect.DeepEqual over a
// hand-built want value cannot notice a *new* field both sides leave at zero.
func TestUserRowFieldCountIsPinned(t *testing.T) {
	t.Parallel()

	const covered = 7
	if got := reflect.TypeOf(userRow{}).NumField(); got != covered {
		t.Errorf(
			"userRow has %d fields, TestToUserMapsEveryField covers %d: "+
				"set the new one to a distinct value there and assert it on the model, then update this count",
			got, covered)
	}
}
