package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// entityBinder names the binder in a not-found error.
const entityBinder = "binder"

// binderRow is the binders table as it is read. Every column the queries below
// select has a field here, and toBinder carries every field across.
type binderRow struct {
	ID        uuid.UUID `db:"id"`
	OwnerID   uuid.UUID `db:"owner_id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// toBinder converts a row to the domain model. It maps every field; there is
// nothing on binderRow the model deliberately drops.
func toBinder(r binderRow) model.Binder {
	return model.Binder{
		ID:        r.ID,
		OwnerID:   r.OwnerID,
		Name:      r.Name,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

const createBinderQuery = `
INSERT INTO binders (id, owner_id, name)
VALUES ($1, $2, $3)
RETURNING id, owner_id, name, created_at, updated_at`

// CreateBinder inserts b and returns it as stored, so the caller gets the
// timestamps the database assigned rather than a guess at them.
func (s *Store) CreateBinder(ctx context.Context, b model.Binder) (model.Binder, error) {
	var row binderRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, createBinderQuery, b.ID, b.OwnerID, b.Name)
	})
	if err != nil {
		return model.Binder{}, fmt.Errorf("creating binder: %w", err)
	}

	return toBinder(row), nil
}

const getBinderByIDQuery = `
SELECT id, owner_id, name, created_at, updated_at
FROM binders
WHERE id = $1`

// GetBinderByID fetches one binder. A binder that is not there is a
// MissingEntityError: this is the by-id lookup where absence is exceptional,
// unlike the collection reads below (000-principles.md section 8b).
func (s *Store) GetBinderByID(ctx context.Context, id uuid.UUID) (model.Binder, error) {
	var row binderRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, getBinderByIDQuery, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Binder{}, dataerror.WrapMissingEntityError(entityBinder, err)
		}
		return model.Binder{}, fmt.Errorf("getting binder by id: %w", err)
	}

	return toBinder(row), nil
}

const listBindersByOwnerQuery = `
SELECT id, owner_id, name, created_at, updated_at
FROM binders
WHERE owner_id = $1
ORDER BY created_at, id`

// ListBindersByOwner returns the owner's binders, oldest first. An owner with
// no binders gets an empty slice and a nil error — an empty collection is not
// an error.
func (s *Store) ListBindersByOwner(ctx context.Context, ownerID uuid.UUID) ([]model.Binder, error) {
	var rows []binderRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.SelectContext(ctx, &rows, listBindersByOwnerQuery, ownerID)
	})
	if err != nil {
		return nil, fmt.Errorf("listing binders by owner: %w", err)
	}

	binders := make([]model.Binder, 0, len(rows))
	for _, row := range rows {
		binders = append(binders, toBinder(row))
	}

	return binders, nil
}

const updateBinderNameQuery = `
UPDATE binders
SET name = $2, updated_at = now()
WHERE id = $1
RETURNING id, owner_id, name, created_at, updated_at`

// UpdateBinderName renames one binder and returns it as stored.
func (s *Store) UpdateBinderName(ctx context.Context, id uuid.UUID, name string) (model.Binder, error) {
	var row binderRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, updateBinderNameQuery, id, name)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Binder{}, dataerror.WrapMissingEntityError(entityBinder, err)
		}
		return model.Binder{}, fmt.Errorf("updating binder name: %w", err)
	}

	return toBinder(row), nil
}
