package api

// This file is white-box on purpose: it reads serviceErrorMappings, which is
// unexported, and pins that the table is total over the service package's
// sentinels.

import (
	"path/filepath"
	"testing"

	"github.com/simeonkorchev/binder/pkg/sentinelscan"
)

// serviceDir is the package whose sentinels this domain must have decided a
// status for, relative to this file.
const serviceDir = "../service"

func TestEveryServiceSentinelIsMapped(t *testing.T) {
	t.Parallel()

	sentinels, err := sentinelscan.ExportedMessages(filepath.FromSlash(serviceDir))
	if err != nil {
		t.Fatalf("reading the service package's sentinels: %v", err)
	}
	if len(sentinels) == 0 {
		t.Fatalf("found no sentinels in %s: this guard cannot fail, which is worse than no guard", serviceDir)
	}

	mapped := map[string]bool{}
	for _, mapping := range serviceErrorMappings {
		mapped[mapping.sentinel.Error()] = true
	}

	for name, message := range sentinels {
		if !mapped[message] {
			t.Errorf(
				"service.%s has no row in serviceErrorMappings, so it would reach a client as an unexplained 500; "+
					"decide its status (000-principles.md section 8c)", name)
		}
	}
}

// A mapped status a client cannot act on is allowed, but a message that leaks
// internal detail is not: handleErr returns these strings verbatim.
func TestEveryMappingHasAClientSafeStatusAndMessage(t *testing.T) {
	t.Parallel()

	for _, mapping := range serviceErrorMappings {
		if mapping.status < 400 || mapping.status > 599 {
			t.Errorf("mapping for %q has status %d, which is not an error status", mapping.sentinel, mapping.status)
		}
		if mapping.message == "" {
			t.Errorf("mapping for %q has no client message", mapping.sentinel)
		}
	}
}
