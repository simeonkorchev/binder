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

// New returns a store reading through db.
func New(db *sqlx.DB) *Store {
	return &Store{db: db}
}
