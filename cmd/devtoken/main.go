// Command devtoken prints a signed session token for a user id, so the API can
// be exercised locally with curl without a real Apple or Google sign-in.
//
// It is a development tool and grants nothing on its own: it signs with
// SESSION_JWT_SECRET, so it can only mint a token a server already trusts
// whoever holds that secret to mint. Never point it at a deployment's secret.
//
//	SESSION_JWT_SECRET=… go run ./cmd/devtoken 99999999-9999-9999-9999-999999999999
//
// docs/running-locally.md uses it for the whole scan → binder → sell loop.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/session"
)

// argCount is the program name plus the one user id it takes.
const argCount = 2

var errUsage = errors.New("usage: devtoken <user-uuid>")

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != argCount {
		return errUsage
	}
	userID, err := uuid.Parse(os.Args[1])
	if err != nil {
		return fmt.Errorf("parsing the user id: %w", err)
	}

	// LoadConfig's own error already names the variable and what it is for, so
	// wrapping it here would only say the same thing twice.
	cfg, err := session.LoadConfig()
	if err != nil {
		return err
	}
	tokens, err := session.New(cfg, time.Now)
	if err != nil {
		return fmt.Errorf("building the signer: %w", err)
	}
	token, err := tokens.Issue(userID)
	if err != nil {
		return fmt.Errorf("issuing the token: %w", err)
	}

	fmt.Println(token.Value)
	return nil
}
