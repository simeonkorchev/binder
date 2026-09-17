// This file is in package main because what it pins is unexported: the route
// registration every domain's operations are mounted by, and the title and
// version the document is rendered under. It is the one place all four domains
// meet, so it is where the whole contract can be checked at once.
package main

import (
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/simeonkorchev/binder/pkg/humaschema"
)

// TestTheDocumentIsAsNullableAsTheGoCode is the drift guard: every property of
// every registered schema admits null exactly when the Go field behind it can
// marshal to one.
//
// It enumerates the fields from the types themselves rather than from a list
// kept by hand, so a DTO added tomorrow is covered without this file being
// touched — the same reason TestEveryServiceSentinelIsMapped parses the service
// package (000-principles.md section 8c).
func TestTheDocumentIsAsNullableAsTheGoCode(t *testing.T) {
	t.Parallel()

	report := inspectRegisteredRoutes(t, humaschema.Config(apiTitle, apiVersion))

	for _, disagreement := range report.Disagreements {
		t.Errorf("the OpenAPI document contradicts the Go code: %s", disagreement)
	}
	for _, missing := range report.MissingFromDoc {
		t.Errorf("%s is a field of a registered DTO but no property of its schema, "+
			"so it never reaches a client (000-principles.md section 9)", missing)
	}
}

// TestTheNullabilityGuardCatchesADishonestDocument is the negative control.
//
// It registers the same routes on huma's own defaults — the configuration this
// API was built on when the bug was found — and asserts that every property
// that was wrong then is named. Without it, a green drift guard would prove
// only that it looked at nothing.
func TestTheNullabilityGuardCatchesADishonestDocument(t *testing.T) {
	t.Parallel()

	// knownLies are the properties huma's own defaults get wrong, named so the
	// guard below is provably able to fail. Each is one of the three shapes
	// pkg/humaschema corrects:
	//
	//   - a pointer to a struct, documented as always present;
	//   - a slice every mapper makes, documented as possibly null;
	//   - a pointer to uuid.UUID, which huma decays to a plain string because a
	//     uuid is an array underneath.
	//
	// If huma ever fixes one of these upstream, this list stops matching and the
	// negative control fails — which is the signal to drop that half of the pass,
	// not to edit the list.
	knownLies := []string{
		"ScanMatch.card",
		"ScanMatch.printing",
		"ScanMatch.candidates",
		"ScanCandidate.printing",
		"PageBody.slots",
		"PageBody.slots[]",
		"SlotBody.cardPrintingId",
		"AddSlotBody.cardPrintingId",
		"SearchCardsBody.cards",
		"ListBindersOutputBody.binders",
		"BrowseListingsBody.listings",
	}

	report := inspectRegisteredRoutes(t, huma.DefaultConfig(apiTitle, apiVersion))

	caught := make([]string, 0, len(report.Disagreements))
	for _, disagreement := range report.Disagreements {
		caught = append(caught, disagreement.Schema+"."+disagreement.Property)
	}

	for _, lie := range knownLies {
		if !slices.Contains(caught, lie) {
			t.Errorf("%s is documented dishonestly by huma's defaults, but the guard did not catch it; "+
				"it caught %v", lie, caught)
		}
	}
}

// inspectRegisteredRoutes mounts every operation the server serves on config and
// reads the resulting schemas back.
func inspectRegisteredRoutes(t *testing.T, config huma.Config) humaschema.Report {
	t.Helper()

	api := humago.New(http.NewServeMux(), config)
	register(api, services{})

	report, err := humaschema.Inspect(api.OpenAPI().Components.Schemas)
	if err != nil {
		t.Fatalf("reading the registered schemas: %v", err)
	}
	if report.Checked == 0 {
		t.Fatal("no property was checked, so this guard cannot fail, which is worse than no guard")
	}
	return report
}
