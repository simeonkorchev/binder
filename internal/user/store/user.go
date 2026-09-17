package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// entityUser names the account in a not-found error.
const entityUser = "user"

// userColumns is the select list every query below returns, so one row shape
// serves all of them and a column added to userRow is added in one place.
const userColumns = `id, auth_provider, auth_subject, contact_email, contact_phone, created_at, updated_at`

// userRow is the users table as it is read. Every column userColumns selects has
// a field here, and toUser carries every field across.
type userRow struct {
	ID           uuid.UUID `db:"id"`
	AuthProvider string    `db:"auth_provider"`
	AuthSubject  string    `db:"auth_subject"`
	// ContactEmail and ContactPhone are nullable because not having opted in to
	// being contacted is the ordinary state of an account, not a missing value.
	ContactEmail *string   `db:"contact_email"`
	ContactPhone *string   `db:"contact_phone"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// toUser converts a row to the domain model. It maps every field; there is
// nothing on userRow the model deliberately drops.
func toUser(r userRow) model.User {
	return model.User{
		ID: r.ID,
		Identity: model.Identity{
			Provider: model.AuthProvider(r.AuthProvider),
			Subject:  r.AuthSubject,
		},
		Contact: model.Contact{
			Email: r.ContactEmail,
			Phone: r.ContactPhone,
		},
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// ensureUserQuery is one statement rather than a select-then-insert: two
// concurrent first sign-ins of the same account would both find nothing and both
// insert, and one of them would fail on the unique index.
//
// The conflict branch updates updated_at to the value it already holds. That
// reads oddly and is deliberate: ON CONFLICT DO NOTHING returns no row, so there
// would be nothing to RETURNING, and a repeat sign-in genuinely changes nothing
// about the account — updated_at means "when the contact details last changed",
// not "last seen".
const ensureUserQuery = `
INSERT INTO users (id, auth_provider, auth_subject)
VALUES ($1, $2, $3)
ON CONFLICT (auth_provider, auth_subject)
DO UPDATE SET updated_at = users.updated_at
RETURNING ` + userColumns

// EnsureUserByIdentity returns the account behind a provider identity, creating
// it on the first sign-in.
//
// newID is the id the account gets if it is created and is ignored if it already
// exists, so the caller minting an id is never the caller deciding whether this
// is a new person.
func (s *Store) EnsureUserByIdentity(
	ctx context.Context, newID uuid.UUID, identity model.Identity,
) (model.User, error) {
	var row userRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, ensureUserQuery, newID, string(identity.Provider), identity.Subject)
	})
	if err != nil {
		return model.User{}, fmt.Errorf("ensuring the user behind a provider identity: %w", err)
	}

	return toUser(row), nil
}

const getUserByIDQuery = `
SELECT ` + userColumns + `
FROM users
WHERE id = $1`

// GetUserByID fetches one account. An account that is not there is a
// MissingEntityError: this is the by-id lookup where absence is exceptional.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	var row userRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, getUserByIDQuery, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, dataerror.WrapMissingEntityError(entityUser, err)
		}
		return model.User{}, fmt.Errorf("getting user by id: %w", err)
	}

	return toUser(row), nil
}

const setUserContactQuery = `
UPDATE users
SET contact_email = $2, contact_phone = $3, updated_at = now()
WHERE id = $1
RETURNING ` + userColumns

// SetUserContact replaces both contact fields and returns the account as stored.
//
// Both columns are written every time: a contact preference is one decision with
// two parts, so a caller that names only an email is saying the phone number is
// not shared. Patching one field would make "leave the other alone" and "stop
// sharing the other" the same request.
func (s *Store) SetUserContact(
	ctx context.Context, id uuid.UUID, contact model.Contact,
) (model.User, error) {
	var row userRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, setUserContactQuery, id, contact.Email, contact.Phone)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, dataerror.WrapMissingEntityError(entityUser, err)
		}
		return model.User{}, fmt.Errorf("setting user contact details: %w", err)
	}

	return toUser(row), nil
}
