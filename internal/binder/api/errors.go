package api

import (
	"context"
	"net/http"

	"github.com/simeonkorchev/binder/internal/binder/service"
	"github.com/simeonkorchev/binder/pkg/humaerr"
)

// serviceErrorMappings is the binder domain's only place where an error becomes
// a status. errors_internal_test.go reads the service package's sentinels out of
// its source and fails if one of them is missing here, so adding a sentinel
// without deciding its status breaks the build rather than reaching a user as an
// unexplained 500.
//
// ErrBinderNotFound is deliberately the answer for both a binder that is not
// there and one belonging to somebody else: a 403 would confirm to a stranger
// that the binder exists.
//
//nolint:gochecknoglobals // the domain's error table: read-only, walked by handleErr.
var serviceErrorMappings = []humaerr.Mapping{
	{
		Sentinel: service.ErrBinderNotFound,
		Status:   http.StatusNotFound,
		Message:  "That binder does not exist.",
	},
	{
		Sentinel: service.ErrSlotNotFound,
		Status:   http.StatusNotFound,
		Message:  "That card is not in this binder.",
	},
	{
		Sentinel: service.ErrPositionOutOfRange,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That position is outside the binder.",
	},
	{
		Sentinel: service.ErrResolutionUnknown,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That is not a way a card's set can be resolved.",
	},
	{
		Sentinel: service.ErrPrintingRequired,
		Status:   http.StatusUnprocessableEntity,
		Message:  "A card matched by its set code has to name the printing it matched.",
	},
	{
		Sentinel: service.ErrPrintingNotAllowed,
		Status:   http.StatusUnprocessableEntity,
		Message:  "A card whose set was not determined cannot name a printing.",
	},
	{
		Sentinel: service.ErrCardUnknown,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That card is not in the card database.",
	},
	{
		Sentinel: service.ErrPrintingUnknown,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That printing is not a printing of that card.",
	},
	{
		Sentinel: service.ErrBatchTooLarge,
		Status:   http.StatusUnprocessableEntity,
		Message:  "That is more cards than one commit can hold. Send them in smaller batches.",
	},
	{
		Sentinel: service.ErrPositionTaken,
		Status:   http.StatusConflict,
		Message:  "The binder changed while this was being saved. Try again.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves, and every status this domain
// answers comes from the one table above.
//
// An error no row matches is a 500 whose detail stays on this side and is logged
// once, here — see pkg/humaerr.
func handleErr(ctx context.Context, err error) error {
	return humaerr.Handle(ctx, err, "binder request failed", serviceErrorMappings)
}
