package api

// This file is white-box on purpose: it reads serviceErrorMappings, which is
// unexported, and hands it to the two checks every domain's error table is held
// to. The checks themselves live in pkg/humaerr/humaerrtest — four byte-identical
// copies of them was one rule stated four times (000-principles.md section 6).

import (
	"path/filepath"
	"testing"

	"github.com/simeonkorchev/binder/pkg/humaerr/humaerrtest"
)

// serviceDir is the package whose sentinels this domain must have decided a
// status for, relative to this file.
const serviceDir = "../service"

func TestEveryServiceSentinelIsMapped(t *testing.T) {
	t.Parallel()

	humaerrtest.RequireTotalOverSentinels(t, filepath.FromSlash(serviceDir), serviceErrorMappings)
}

func TestEveryMappingHasAClientSafeStatusAndMessage(t *testing.T) {
	t.Parallel()

	humaerrtest.RequireClientSafe(t, serviceErrorMappings)
}
