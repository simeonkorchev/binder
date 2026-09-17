package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simeonkorchev/binder/internal/user/service"
	"github.com/simeonkorchev/binder/pkg/humaerr"
)

// errNoActor is what a handler returns when ActorFunc cannot name the user. Built
// once: there is nothing request-specific in a 401.
var errNoActor = huma.Error401Unauthorized("Sign in to use your account.")

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
var serviceErrorMappings = []humaerr.Mapping{
	{
		Sentinel: service.ErrIdentityRejected,
		Status:   http.StatusUnauthorized,
		Message:  "That sign-in could not be verified. Please try again.",
	},
	{
		Sentinel: service.ErrProviderUnknown,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That is not a way to sign in to this app.",
	},
	{
		Sentinel: service.ErrUserNotFound,
		Status:   http.StatusNotFound,
		Message:  "That account does not exist.",
	},
	{
		Sentinel: service.ErrContactBlank,
		Status:   http.StatusUnprocessableEntity,
		Message:  "A contact detail you share cannot be blank. Leave it out to stop sharing it.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves, and every status this domain
// answers comes from the one table above.
//
// An error no row matches is a 500 whose detail stays on this side and is logged
// once, here — see pkg/humaerr.
func handleErr(ctx context.Context, err error) error {
	return humaerr.Handle(ctx, err, "account request failed", serviceErrorMappings)
}
