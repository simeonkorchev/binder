// Package humaerrtest holds the two checks every domain's error table is held
// to, so that the four domains state them once instead of carrying four
// byte-identical copies of the same test file (000-principles.md section 6).
//
// It is a test helper in its own package rather than a _test.go file because
// the tables it checks are unexported: each domain's own internal test passes
// its table in, and everything else about the check is the same everywhere.
package humaerrtest

import (
	"testing"

	"github.com/simeonkorchev/binder/pkg/humaerr"
	"github.com/simeonkorchev/binder/pkg/sentinelscan"
)

// errorStatusFloor and errorStatusCeiling bound the statuses a mapping may
// choose: an error table that answered 200 or 601 would be a row nobody can
// read as an error.
const (
	errorStatusFloor   = 400
	errorStatusCeiling = 599
)

// RequireTotalOverSentinels fails t for every exported sentinel declared in
// serviceDir that no row of mappings matches.
//
// The sentinels are read out of the service package's source rather than listed
// here, because a hand-written list stops being true the day somebody adds one —
// and a sentinel nobody remembered is exactly the failure this guards
// (000-principles.md section 8c, 006-testing.md "Derive lists from source").
func RequireTotalOverSentinels(t *testing.T, serviceDir string, mappings []humaerr.Mapping) {
	t.Helper()

	sentinels, err := sentinelscan.ExportedMessages(serviceDir)
	if err != nil {
		t.Fatalf("reading the service package's sentinels: %v", err)
	}
	if len(sentinels) == 0 {
		t.Fatalf("found no sentinels in %s: this guard cannot fail, which is worse than no guard", serviceDir)
	}

	mapped := map[string]bool{}
	for _, mapping := range mappings {
		mapped[mapping.Sentinel.Error()] = true
	}

	for name, message := range sentinels {
		if !mapped[message] {
			t.Errorf(
				"service.%s has no row in the domain's error table, so it would reach a client as an "+
					"unexplained 500; decide its status (000-principles.md section 8c)", name)
		}
	}
}

// RequireClientSafe fails t for a mapping a client cannot make sense of. A
// mapped status a client cannot act on is allowed — that is what a decided 500
// is — but a status outside the error range, or an empty message, is not:
// humaerr.Handle returns these strings verbatim.
func RequireClientSafe(t *testing.T, mappings []humaerr.Mapping) {
	t.Helper()

	for _, mapping := range mappings {
		if mapping.Status < errorStatusFloor || mapping.Status > errorStatusCeiling {
			t.Errorf("mapping for %q has status %d, which is not an error status", mapping.Sentinel, mapping.Status)
		}
		if mapping.Message == "" {
			t.Errorf("mapping for %q has no client message", mapping.Sentinel)
		}
	}
}
