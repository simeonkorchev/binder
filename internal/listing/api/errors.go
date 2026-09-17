package api

import (
	"context"
	"net/http"

	"github.com/simeonkorchev/binder/internal/listing/service"
	"github.com/simeonkorchev/binder/pkg/humaerr"
)

// serviceErrorMappings is the listing domain's only place where an error becomes
// a status. errors_internal_test.go reads the service package's sentinels out of
// its source and fails if one of them is missing here, so adding a sentinel
// without deciding its status breaks the build rather than reaching a user as an
// unexplained 500.
//
// ErrBinderSlotNotFound and ErrListingNotFound are each deliberately the answer
// for both a row that is not there and one belonging to somebody else: a 403
// would confirm to a stranger that it exists. That is the precedent the binder
// domain set for its own ids.
//
//nolint:gochecknoglobals // the domain's error table: read-only, walked by handleErr.
var serviceErrorMappings = []humaerr.Mapping{
	{
		Sentinel: service.ErrBinderSlotNotFound,
		Status:   http.StatusNotFound,
		Message:  "That card is not in one of your binders.",
	},
	{
		Sentinel: service.ErrListingNotFound,
		Status:   http.StatusNotFound,
		Message:  "That listing does not exist.",
	},
	{
		Sentinel: service.ErrAlreadyListed,
		Status:   http.StatusConflict,
		Message:  "That card is already for sale.",
	},
	{
		Sentinel: service.ErrSellerNotFound,
		Status:   http.StatusNotFound,
		Message:  "That seller does not exist.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves, and every status this domain
// answers comes from the one table above.
//
// An error no row matches is a 500 whose detail stays on this side and is logged
// once, here — see pkg/humaerr.
func handleErr(ctx context.Context, err error) error {
	return humaerr.Handle(ctx, err, "listing request failed", serviceErrorMappings)
}
