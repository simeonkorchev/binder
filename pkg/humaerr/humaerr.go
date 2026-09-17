// Package humaerr turns a domain's error into the answer its client sees.
//
// Every domain keeps its own table of sentinel-to-status rows — which errors
// exist and what each one means to a client is domain knowledge, and the tables
// have nothing in common. What they *did* have in common was the mechanism
// around them: four packages had declared the same row type, walked it with the
// same loop, and logged and answered the same way for an error no row matched
// (000-principles.md section 6). That part lives here now, so a change to how
// an unmapped error is answered or logged is one change rather than four.
//
// The rule the table exists for is 000-principles.md section 8c: every exported
// sentinel a service can return needs a decided status, including a decided
// 500, so "decided" is distinguishable from "forgotten". humaerrtest holds the
// checks that prove a table obeys it.
package humaerr

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
)

// unmappedMessage is what a client is told about an error no row matched. It
// says nothing: the detail behind it is SQL, file paths and upstream bodies,
// and none of that crosses the wire (004-security.md).
const unmappedMessage = "Something went wrong."

// Mapping is one row of a domain's error table: a sentinel the service can
// return, and the answer a client gets for it. A sentinel a client cannot act
// on is mapped to 500 here rather than left out, so a decided 500 is
// distinguishable from a forgotten one (000-principles.md section 8c).
type Mapping struct {
	Sentinel error
	Status   int
	// Message is what the client is told. It never carries internal detail.
	Message string
}

// Handle returns the response error for err: the first row it matches, or a 500
// whose detail stays on this side.
//
// Handlers call it through their domain's own handleErr instead of choosing a
// status themselves (002-go-conventions.md section 4.4). logMessage names the
// domain's work in the one log line an unmapped error produces — this is the one
// place it is logged, because the layers below return errors, they do not also
// log them.
func Handle(ctx context.Context, err error, logMessage string, mappings []Mapping) error {
	for _, mapping := range mappings {
		if errors.Is(err, mapping.Sentinel) {
			return huma.NewError(mapping.Status, mapping.Message)
		}
	}

	slog.ErrorContext(ctx, logMessage, slog.String("error", err.Error()))
	return huma.Error500InternalServerError(unmappedMessage)
}
