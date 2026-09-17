package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	// Registers the "pgx" driver used by connect. The domain stores translate
	// driver errors into internal/dataerror types, and pgx returns
	// *pgconn.PgError where lib/pq returns *pq.Error — so connecting on any
	// other driver would silently disable every translation the stores make
	// (internal/testdb).
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	// driverName is the driver the whole backend connects with; see the import
	// comment above.
	driverName = "pgx"

	// readHeaderTimeout bounds how long a client may take to send its request
	// headers. Without it a handful of idle connections can hold the server open
	// indefinitely (gosec G112).
	readHeaderTimeout = 10 * time.Second
	// readTimeout and writeTimeout bound a whole request and its response. The
	// slowest endpoint here is a card search, which is one indexed query.
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
	// idleTimeout closes a kept-alive connection that has gone quiet.
	idleTimeout = 120 * time.Second

	// connMaxLifetime retires a pooled connection after a while, whatever its
	// state. Managed Postgres and the proxies in front of it drop long-lived
	// connections on their own schedule, and a pool that never retires one
	// hands the first request after that out a connection the other end has
	// already closed.
	connMaxLifetime = 30 * time.Minute
)

// connect opens the connection pool every store shares.
//
// The pool is bounded. database/sql's default is unlimited, which is survivable
// for a command that runs one query at a time (cmd/cardimport) and is not for a
// server: a burst of requests would open a connection each, and Postgres refuses
// the ones past max_connections — so a traffic spike becomes an outage for every
// process on the cluster rather than a queue in this one.
func connect(ctx context.Context, cfg config) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, driverName, cfg.DatabaseURL)
	if err != nil {
		// The URL carries a password, so it is not in the message: the operator
		// knows which DATABASE_URL they set, and a log line does not need it
		// (004-security.md).
		return nil, fmt.Errorf("connecting to the database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConnections)
	// Idle is held at the same number so that a pool which grew for a burst does
	// not close every connection the moment it passes, only to reopen them for
	// the next one.
	db.SetMaxIdleConns(cfg.MaxConnections)
	db.SetConnMaxLifetime(connMaxLifetime)

	return db, nil
}

// listenAndServe serves handler until ctx is cancelled, then drains.
//
// The listener is opened here rather than inside http.Server so that the address
// actually bound is known before anything is served — with PORT=0 the operating
// system chooses it, and the logged address is how the smoke test finds the
// server it started.
func listenAndServe(ctx context.Context, cfg config, handler http.Handler) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	address := net.JoinHostPort("", strconv.Itoa(cfg.Port))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", address, err)
	}

	// Buffered so the serving goroutine can always deliver its result and exit,
	// even when the shutdown path below won the race and nobody is receiving.
	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(listener) }()

	slog.InfoContext(ctx, "binderd is listening", slog.String("address", listener.Addr().String()))

	select {
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serving: %w", err)
	case <-ctx.Done():
		return drain(ctx, srv, cfg.ShutdownTimeout)
	}
}

// drain stops accepting connections and gives the requests already in flight
// their timeout to finish.
//
// The shutdown context is deliberately detached from ctx: ctx is already
// cancelled by the time we get here — it is what asked for the shutdown — so
// passing it on would make the drain expire immediately and cut every in-flight
// request, which is the opposite of graceful.
func drain(ctx context.Context, srv *http.Server, timeout time.Duration) error {
	slog.InfoContext(ctx, "binderd is shutting down", slog.Duration("drain_timeout", timeout))

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("draining in-flight requests: %w", err)
	}
	return nil
}
