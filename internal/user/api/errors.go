package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simeonkorchev/binder/internal/user/service"
)

// errNoActor is what a handler returns when ActorFunc cannot name the user. Built
// once: there is nothing request-specific in a 401.
var errNoActor = huma.Error401Unauthorized("Sign in to use your account.")

// serviceErrorMapping is one row of the table below: a sentinel the service can
// return, and the answer a client gets for it. A sentinel a client cannot act on
// is mapped to 500 here rather than left out, so a decided 500 is distinguishable
// from a forgotten one (000-principles.md section 8c).
type serviceErrorMapping struct {
	sentinel error
	status   int
	// message is what the client is told. It never carries internal detail.
	message string
}

// serviceErrorMappings is the user domain's only place where an error becomes a
// status. errors_internal_test.go reads the service package's sentinels out of
// its source and fails if one of them is missing here, so adding a sentinel
// without deciding its status breaks the build rather than reaching a user as an
// unexplained 500.
//
// ErrIdentityRejected is one 401 for every way a provider token can fail —
// forged, expired, addressed to another app, signed by an unpublished key, bound
// to a different nonce. Telling the client which check failed would hand it an
// oracle for the next attempt, and there is nothing it could do differently
// anyway: the answer to all of them is "sign in again".
//
//nolint:gochecknoglobals // the domain's error table: read-only, walked by handleErr.
var serviceErrorMappings = []serviceErrorMapping{
	{
		sentinel: service.ErrIdentityRejected,
		status:   http.StatusUnauthorized,
		message:  "That sign-in could not be verified. Please try again.",
	},
	{
		sentinel: service.ErrProviderUnknown,
		status:   http.StatusUnprocessableEntity,
		message:  "That is not a way to sign in to this app.",
	},
	{
		sentinel: service.ErrUserNotFound,
		status:   http.StatusNotFound,
		message:  "That account does not exist.",
	},
	{
		sentinel: service.ErrContactBlank,
		status:   http.StatusUnprocessableEntity,
		message:  "A contact detail you share cannot be blank. Leave it out to stop sharing it.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves.
//
// An error no row matches is a 500 whose detail stays on this side. This is the
// one place it is logged, and the layers below never put a token, an email
// address or a phone number into an error message — so what is logged here is an
// operation name and a driver's complaint, not somebody's personal data
// (004-security.md).
func handleErr(ctx context.Context, err error) error {
	for _, mapping := range serviceErrorMappings {
		if errors.Is(err, mapping.sentinel) {
			return huma.NewError(mapping.status, mapping.message)
		}
	}

	slog.ErrorContext(ctx, "account request failed", slog.String("error", err.Error()))
	return huma.Error500InternalServerError("Something went wrong.")
}
