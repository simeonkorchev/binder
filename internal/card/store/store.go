// Package store reads the imported card database. Every method here is a
// single read-only query: the card domain never writes and never composes two
// statements, so it holds a *sqlx.DB directly rather than a transaction
// boundary nothing would use.
package store

import "github.com/jmoiron/sqlx"

// Store answers the match ladder's lookups against Postgres.
type Store struct {
	db *sqlx.DB
}

// NewStore returns a store reading through db. It is NewStore and not New for
// the same reason the other three domains' is: one name for one thing across
// the four stores (000-principles.md section 4).
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}
