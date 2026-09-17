package api

import (
	"context"
	"net/http"

	"github.com/simeonkorchev/binder/internal/card/service"
	"github.com/simeonkorchev/binder/pkg/humaerr"
)

// serviceErrorMappings is the card domain's only place where an error becomes a
// status. errors_internal_test.go reads the service package's sentinels out of
// its source and fails if one of them is missing here, so adding a sentinel
// without deciding its status breaks the build rather than reaching a user as
// an unexplained 500.
//
//nolint:gochecknoglobals // the domain's error table: read-only, walked by handleErr.
var serviceErrorMappings = []humaerr.Mapping{
	{
		Sentinel: service.ErrEmptyScan,
		Status:   http.StatusUnprocessableEntity,
		Message:  "A scan needs a printed code or a card name.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves, and every status this domain
// answers comes from the one table above.
//
// An error no row matches is a 500 whose detail stays on this side and is logged
// once, here — see pkg/humaerr.
func handleErr(ctx context.Context, err error) error {
	return humaerr.Handle(ctx, err, "card request failed", serviceErrorMappings)
}
