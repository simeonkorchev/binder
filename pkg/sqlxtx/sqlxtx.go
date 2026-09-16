// Package sqlxtx carries a database transaction on a context.Context so that a
// store method works both on its own and as one step of a larger transaction.
//
// A store method never takes a *sqlx.Tx parameter and a service never holds
// one: the service opens a transaction by calling the store's InTx with a
// closure, and every store call inside that closure joins the transaction
// already on the context. See .claude/rules/002-go-conventions.md section 7.
package sqlxtx

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// txKey is the context key the transaction is stored under. An unexported
// zero-sized type cannot collide with a key from another package and needs no
// package-level variable.
type txKey struct{}

// WithTx returns a context carrying tx. Production code reaches this through
// EnsureTx; it is exported so a store test can run its specs inside a
// transaction it rolls back afterwards.
func WithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// GetTx returns the transaction on ctx, or nil when there is none. It is for
// the rare caller that legitimately runs either way; everything inside an
// EnsureTx callback should use MustGetTx.
func GetTx(ctx context.Context) *sqlx.Tx {
	tx, ok := ctx.Value(txKey{}).(*sqlx.Tx)
	if !ok {
		return nil
	}
	return tx
}

// MustGetTx returns the transaction on ctx and panics when there is none.
//
// The panic is a programmer-error assertion, not error handling: every caller
// is inside an EnsureTx callback, which always puts a transaction on the
// context, so a missing one means the call was wired up wrong rather than that
// anything failed at runtime (002-go-conventions.md section 4.5).
func MustGetTx(ctx context.Context) *sqlx.Tx {
	tx := GetTx(ctx)
	if tx == nil {
		panic("sqlxtx: no transaction on the context; call this inside EnsureTx")
	}
	return tx
}

// EnsureTx joins the transaction already on ctx, or starts one at the given
// isolation level and commits it when cb returns nil. Joining is what lets a
// service compose several store calls into one atomic unit without any of them
// knowing whether they are the outermost caller.
//
// When cb returns an error the transaction is rolled back and cb's error is
// returned unchanged, so a caller can still match a sentinel with errors.Is.
func EnsureTx(ctx context.Context, db *sqlx.DB, cb func(ctx context.Context) error, level sql.IsolationLevel) error {
	if tx := GetTx(ctx); tx != nil {
		return cb(ctx)
	}

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{Isolation: level, ReadOnly: false})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// A panic inside cb must not leave the connection holding an open
	// transaction; the rollback runs before the panic continues upwards.
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := cb(WithTx(ctx, tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	committed = true

	return nil
}
