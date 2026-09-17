// Package store is the listing domain's SQL. Every method runs inside InTx, so
// a method works on its own and equally as one step of a transaction a service
// opened around several of them.
//
// The reads reach across into binder_slots, binders and cards. A listing is a
// flag on a slot and carries neither the card nor the seller of its own
// (db/migrations/004_listings.sql), so "who owns this slot" and "what is for
// sale" are joins — one query each, rather than a second copy of the columns
// here or a round trip per row into another domain.
package store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// Store reads and writes listings.
type Store struct {
	db *sqlx.DB
}

// NewStore returns a Store over db.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// InTx joins the transaction already on ctx, or starts one. Read Committed is
// enough here: the only compound operation is "check who owns the slot, then
// list it", and what makes that safe is not isolation but the UNIQUE constraint
// on binder_slot_id and the foreign key to binder_slots, which reject the racing
// writer whatever this transaction read.
func (s *Store) InTx(ctx context.Context, cb func(ctx context.Context) error) error {
	return sqlxtx.EnsureTx(ctx, s.db, cb, sql.LevelReadCommitted)
}
