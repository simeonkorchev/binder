package humaschema_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/pkg/humaschema"
)

// probeNested is a struct that gets a $ref, so the shapes below can be pointed
// at something other than a scalar.
type probeNested struct {
	Value string `json:"value"`
}

// probeBody carries one field per shape the rule has to decide, so the table
// below reads as the rule itself: a Go pointer means null on the wire, and
// nothing else does.
type probeBody struct {
	Plain        string            `json:"plain"`
	PlainPointer *string           `json:"plainPointer"`
	ID           uuid.UUID         `json:"id"`
	MaybeID      *uuid.UUID        `json:"maybeId"`
	Nested       probeNested       `json:"nested"`
	MaybeNested  *probeNested      `json:"maybeNested"`
	List         []probeNested     `json:"list"`
	SparseList   []*probeNested    `json:"sparseList"`
	NilableList  []probeNested     `json:"nilableList" nullable:"true"`
	NeverNil     *probeNested      `json:"neverNil" nullable:"false"`
	Lookup       map[string]string `json:"lookup"`
	Anything     any               `json:"anything"`
	Unsent       string            `json:"-"`
}

type probeOutput struct {
	Body probeBody
}

func TestConfigDocumentsEveryShapeAsNullableAsItsGoType(t *testing.T) {
	t.Parallel()

	properties := probeProperties(t)

	for _, shape := range []struct {
		property string
		want     string
	}{
		{"plain", `{"type":"string"}`},
		{"plainPointer", `{"type":["string","null"]}`},
		{"id", `{"type":"string"}`},
		// A uuid is an array of bytes underneath, which huma's registry decays
		// the pointer off before it decides nullability.
		{"maybeId", `{"type":["string","null"]}`},
		{"nested", `{"$ref":"#/components/schemas/ProbeNested"}`},
		// A $ref cannot carry a type, so null is a branch beside it.
		{"maybeNested", `{"anyOf":[{"$ref":"#/components/schemas/ProbeNested"},{"type":"null"}]}`},
		{"list", `{"items":{"$ref":"#/components/schemas/ProbeNested"},"type":"array"}`},
		{
			"sparseList",
			`{"items":{"anyOf":[{"$ref":"#/components/schemas/ProbeNested"},{"type":"null"}]},"type":"array"}`,
		},
		// The tag still wins, in both directions.
		{"nilableList", `{"items":{"$ref":"#/components/schemas/ProbeNested"},"type":["array","null"]}`},
		{"neverNil", `{"$ref":"#/components/schemas/ProbeNested"}`},
		{"lookup", `{"additionalProperties":{"type":"string"},"type":"object"}`},
		// An `any` constrains nothing, so it already admits null and is left be.
		{"anything", `{}`},
	} {
		emitted, ok := properties[shape.property]
		if !ok {
			t.Errorf("%s is missing from the generated schema", shape.property)
			continue
		}
		if string(emitted) != shape.want {
			t.Errorf("%s was documented as %s, want %s", shape.property, emitted, shape.want)
		}
	}

	if _, ok := properties["Unsent"]; ok {
		t.Error("a field tagged json:\"-\" reached the document")
	}
}

// TestInspectAgreesWithConfig pins the two halves of this package against each
// other: what Config writes is what Inspect reads back as honest.
func TestInspectAgreesWithConfig(t *testing.T) {
	t.Parallel()

	api := probeAPI(t, humaschema.Config("probe", "test"))

	report, err := humaschema.Inspect(api.OpenAPI().Components.Schemas)
	if err != nil {
		t.Fatalf("inspecting the probe schemas: %v", err)
	}
	if report.Checked == 0 {
		t.Fatal("no property was checked, so this guard cannot fail")
	}
	for _, disagreement := range report.Disagreements {
		t.Errorf("Config left a disagreement Inspect can see: %s", disagreement)
	}
	for _, missing := range report.MissingFromDoc {
		t.Errorf("%s never reached the document", missing)
	}
}

// probeProperties renders the probe body's schema and returns each property as
// it will be sent to a client, so the assertions above are made against the
// wire and not against huma's in-memory flags.
func probeProperties(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	api := probeAPI(t, humaschema.Config("probe", "test"))
	schema := api.OpenAPI().Components.Schemas.Map()["ProbeBody"]
	if schema == nil {
		t.Fatal("the probe body was not registered as a component schema")
	}

	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshalling the probe schema: %v", err)
	}

	var document struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("re-reading the probe schema: %v", err)
	}
	return document.Properties
}

func probeAPI(t *testing.T, config huma.Config) huma.API {
	t.Helper()

	api := humago.New(http.NewServeMux(), config)
	huma.Register(api, huma.Operation{
		OperationID: "probe",
		Method:      http.MethodGet,
		Path:        "/probe",
	}, func(context.Context, *struct{}) (*probeOutput, error) {
		return &probeOutput{}, nil
	})
	return api
}
