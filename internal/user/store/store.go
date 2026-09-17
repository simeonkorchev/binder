// Package store is the user domain's SQL. Every method runs inside InTx, so a
// method works on its own and equally as one step of a transaction a service
// opened around several of them.
package store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// Store reads and writes accounts.
type Store struct {
	db *sqlx.DB
}

// NewStore returns a Store over db.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// InTx joins the transaction already on ctx, or starts one. Read Committed is
// enough for everything in this package: the sign-in upsert takes its decision
// from the UNIQUE (auth_provider, auth_subject) index rather than from a row it
// read earlier, and the contact update writes one row by its primary key.
func (s *Store) InTx(ctx context.Context, cb func(ctx context.Context) error) error {
	return sqlxtx.EnsureTx(ctx, s.db, cb, sql.LevelReadCommitted)
}
