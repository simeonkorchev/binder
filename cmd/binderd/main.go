// Command binderd serves the Binder HTTP API: the card, binder, listing and
// account domains behind one router, authenticated by the session token the
// sign-in endpoint issues.
//
// Every domain's operations are registered on one Huma API, so the OpenAPI
// document this same binary prints describes exactly what it serves:
//
//	DATABASE_URL=postgres://... SESSION_JWT_SECRET=... \
//	  APPLE_CLIENT_IDS=... GOOGLE_CLIENT_IDS=... go run ./cmd/binderd
//	go run ./cmd/binderd -openapi > packages/types/openapi.json
//
// Configuration is read and validated before anything is served: a missing
// database URL, signing secret or provider audience exits non-zero rather than
// starting a server that would fail every request, or — worse — accept a token
// meant for somebody else's app (004-security.md).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jmoiron/sqlx"
	binderservice "github.com/simeonkorchev/binder/internal/binder/service"
	binderstore "github.com/simeonkorchev/binder/internal/binder/store"
	cardservice "github.com/simeonkorchev/binder/internal/card/service"
	cardstore "github.com/simeonkorchev/binder/internal/card/store"
	listingservice "github.com/simeonkorchev/binder/internal/listing/service"
	listingstore "github.com/simeonkorchev/binder/internal/listing/store"
	"github.com/simeonkorchev/binder/internal/user/identity"
	userservice "github.com/simeonkorchev/binder/internal/user/service"
	"github.com/simeonkorchev/binder/internal/user/session"
	userstore "github.com/simeonkorchev/binder/internal/user/store"
)

// config is what the server needs from the environment beyond what the session
// and identity packages read for themselves.
//
// DATABASE_URL has no default on purpose: a server that silently fell back to a
// local database would look healthy while serving nobody's data.
type config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Port is where the server listens. Zero asks the operating system for a
	// free one and is what the smoke test uses; it is never a production value.
	Port int `env:"PORT" envDefault:"8080"`

	// MaxConnections bounds the database pool. It is per process, so the budget
	// to divide is the cluster's max_connections over however many instances
	// are running.
	MaxConnections int `env:"DATABASE_MAX_CONNECTIONS" envDefault:"10"`

	// ShutdownTimeout is how long a shutdown waits for in-flight requests before
	// dropping them. Longer than the slowest endpoint, shorter than the platform
	// SIGKILLs after.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

func main() {
	// SIGTERM and Ctrl-C start a graceful shutdown rather than killing the
	// process, so a deploy finishes the requests it has already accepted.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Released before the exit below rather than deferred: os.Exit runs no
	// deferred function, so a `defer stop()` would be dead on the path that
	// matters.
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()

	if err != nil {
		slog.ErrorContext(ctx, "binderd failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run is main with its exits and its process-global state passed in, so the
// smoke test can drive the same code the binary does.
func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("binderd", flag.ContinueOnError)
	flags.SetOutput(out)
	printSpec := flags.Bool("openapi", false,
		"write the OpenAPI document to stdout and exit, without configuration or a database")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("reading the command line: %w", err)
	}

	if *printSpec {
		return writeOpenAPI(out)
	}
	return serve(ctx)
}

// writeOpenAPI prints the document the registered operations describe. It reads
// no configuration and opens no connection: `make gen-spec` runs it on a machine
// that has neither.
func writeOpenAPI(out io.Writer) error {
	document, err := openAPIDocument()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, string(document)); err != nil {
		return fmt.Errorf("writing the OpenAPI document: %w", err)
	}
	return nil
}

// serve validates the configuration, opens the database, and serves until ctx is
// cancelled.
func serve(ctx context.Context) error {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		return fmt.Errorf("reading the server configuration: %w", err)
	}

	tokens, verifiers, err := loadIdentity()
	if err != nil {
		return err
	}

	db, err := connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeQuietly(ctx, "database", db)

	return listenAndServe(ctx, cfg, newHandler(newServices(db, verifiers, tokens), tokens))
}

// loadIdentity builds the two halves of the auth stack — the session signer and
// the provider token verifiers — failing rather than starting without either.
//
// There is deliberately no development fallback for the signing secret: a
// default secret is a published one, and every session it ever signed would be
// forgeable (internal/user/session, 004-security.md).
func loadIdentity() (*session.Tokens, *identity.Verifiers, error) {
	sessionCfg, err := session.LoadConfig()
	if err != nil {
		return nil, nil, err
	}
	tokens, err := session.New(sessionCfg, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building the session signer: %w", err)
	}

	identityCfg, err := identity.LoadConfig()
	if err != nil {
		return nil, nil, err
	}
	verifiers, err := identity.NewFromConfig(identityCfg, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building the provider identity verifiers: %w", err)
	}

	return tokens, verifiers, nil
}

// newServices builds each domain's service over one connection pool.
//
// The account service is built first and handed to the listing service through
// the seller-contact adapter: that is the one place the two domains meet, and
// they meet as an interface rather than as each other's concrete type
// (contact.go).
func newServices(db *sqlx.DB, verifiers *identity.Verifiers, tokens *session.Tokens) services {
	accounts := userservice.NewService(userstore.NewStore(db), verifiers, tokens, nil)

	return services{
		cards:    cardservice.NewService(cardstore.NewStore(db)),
		binders:  binderservice.NewService(binderstore.NewStore(db)),
		listings: listingservice.NewService(listingstore.NewStore(db), sellerContacts{accounts: accounts}),
		accounts: accounts,
	}
}

// closeQuietly reports a close failure rather than returning it: overwriting the
// run's outcome with a teardown error would hide it.
func closeQuietly(ctx context.Context, what string, closer interface{ Close() error }) {
	if err := closer.Close(); err != nil {
		slog.WarnContext(ctx, "closing a resource failed",
			slog.String("resource", what),
			slog.String("error", err.Error()),
		)
	}
}
