package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simeonkorchev/binder/internal/binder/service"
)

// serviceErrorMapping is one row of the table below: a sentinel the service can
// return, and the answer a client gets for it. A sentinel a client cannot act
// on is mapped to 500 here rather than left out, so a decided 500 is
// distinguishable from a forgotten one (000-principles.md section 8c).
type serviceErrorMapping struct {
	sentinel error
	status   int
	// message is what the client is told. It never carries internal detail.
	message string
}

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
var serviceErrorMappings = []serviceErrorMapping{
	{
		sentinel: service.ErrBinderNotFound,
		status:   http.StatusNotFound,
		message:  "That binder does not exist.",
	},
	{
		sentinel: service.ErrSlotNotFound,
		status:   http.StatusNotFound,
		message:  "That card is not in this binder.",
	},
	{
		sentinel: service.ErrPositionOutOfRange,
		status:   http.StatusUnprocessableEntity,
		message:  "That position is outside the binder.",
	},
	{
		sentinel: service.ErrResolutionUnknown,
		status:   http.StatusUnprocessableEntity,
		message:  "That is not a way a card's set can be resolved.",
	},
	{
		sentinel: service.ErrPrintingRequired,
		status:   http.StatusUnprocessableEntity,
		message:  "A card matched by its set code has to name the printing it matched.",
	},
	{
		sentinel: service.ErrPrintingNotAllowed,
		status:   http.StatusUnprocessableEntity,
		message:  "A card whose set was not determined cannot name a printing.",
	},
	{
		sentinel: service.ErrCardUnknown,
		status:   http.StatusUnprocessableEntity,
		message:  "That card is not in the card database.",
	},
	{
		sentinel: service.ErrPrintingUnknown,
		status:   http.StatusUnprocessableEntity,
		message:  "That printing is not a printing of that card.",
	},
	{
		sentinel: service.ErrPositionTaken,
		status:   http.StatusConflict,
		message:  "The binder changed while this was being saved. Try again.",
	},
}

// handleErr turns a service error into the response the client sees. Handlers
// call it instead of choosing a status themselves.
//
// An error no row matches is a 500 whose detail stays on this side: SQL, file
// paths and upstream bodies never reach a client. This is the one place it is
// logged — the layers below return it, they do not also log it.
func handleErr(ctx context.Context, err error) error {
	for _, mapping := range serviceErrorMappings {
		if errors.Is(err, mapping.sentinel) {
			return huma.NewError(mapping.status, mapping.message)
		}
	}

	slog.ErrorContext(ctx, "binder request failed", slog.String("error", err.Error()))
	return huma.Error500InternalServerError("Something went wrong.")
}
