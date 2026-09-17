package humaschema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Disagreement is one property whose emitted schema does not say what the Go
// field it was generated from guarantees.
type Disagreement struct {
	// Schema is the component schema's name, Property the dotted path to the
	// property inside it — an item of a list is named `slots[]`.
	Schema   string
	Property string
	// GoType is the field's Go type, Document what the emitted schema says.
	GoType   string
	Document string
	// WantsNull is what the Go type guarantees; the document says the opposite.
	WantsNull bool
}

func (d Disagreement) String() string {
	if d.WantsNull {
		return fmt.Sprintf("%s.%s is %s in Go, which marshals to null, but the document says %s "+
			"— a client that trusts it dereferences null", d.Schema, d.Property, d.GoType, d.Document)
	}
	return fmt.Sprintf("%s.%s is %s in Go and is never sent as null, but the document says %s "+
		"— every client pays for a null check that can never fire", d.Schema, d.Property, d.GoType, d.Document)
}

// Report is what Inspect found.
//
// Checked is carried because a guard that cannot fail is worse than no guard
// (006-testing.md): a walk that matched no property at all would report no
// disagreements and prove nothing.
type Report struct {
	Checked        int
	Disagreements  []Disagreement
	MissingFromDoc []string
}

// Inspect reads every registered schema back as it will be emitted and reports
// each property whose nullability disagrees with the Go field behind it.
//
// It deliberately re-derives the truth from reflection and reads the claim from
// the marshalled JSON rather than from huma's in-memory flags, so it is an
// independent check on Config's pass rather than a restatement of it.
func Inspect(registry huma.Registry) (Report, error) {
	report := Report{}
	for _, name := range sortedNames(registry) {
		schema := registry.Map()[name]
		goType := registry.TypeFromRef(schemaRefPrefix + name)
		if goType == nil || schema == nil {
			continue
		}

		for jsonName, field := range wireFields(goType) {
			property := schema.Properties[jsonName]
			if property == nil {
				report.MissingFromDoc = append(report.MissingFromDoc, name+"."+jsonName)
				continue
			}
			emitted, err := emittedSchema(property)
			if err != nil {
				return Report{}, fmt.Errorf("reading the emitted schema of %s.%s: %w", name, jsonName, err)
			}
			report.inspect(name, jsonName, field.Type, nullableTag(field), emitted)
		}
	}

	sort.Strings(report.MissingFromDoc)
	return report, nil
}

// inspect compares one property, then the items of a list property, against the
// Go type behind them, recording what it finds.
func (r *Report) inspect(schemaName, path string, goType reflect.Type, override *bool, emitted wireSchema) {
	wantsNull := goType.Kind() == reflect.Pointer || goType.Kind() == reflect.Interface
	if override != nil {
		wantsNull = *override
	}

	r.Checked++
	if emitted.admitsNull() != wantsNull {
		r.Disagreements = append(r.Disagreements, Disagreement{
			Schema:    schemaName,
			Property:  path,
			GoType:    goType.String(),
			Document:  emitted.describe(),
			WantsNull: wantsNull,
		})
	}

	element := deref(goType)
	if isList(element) && emitted.Items != nil && emitted.hasType(huma.TypeArray) {
		r.inspect(schemaName, path+"[]", element.Elem(), nil, *emitted.Items)
	}
}

// sortedNames keeps the report stable, so a failure names the same property in
// the same order on every run.
func sortedNames(registry huma.Registry) []string {
	schemas := registry.Map()
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// wireSchema is the part of a marshalled property schema this package reads
// back. Everything else about the property is irrelevant to nullability.
type wireSchema struct {
	Type  any          `json:"type"`
	Ref   string       `json:"$ref"`
	AnyOf []wireSchema `json:"anyOf"`
	OneOf []wireSchema `json:"oneOf"`
	AllOf []wireSchema `json:"allOf"`
	Items *wireSchema  `json:"items"`
}

func emittedSchema(s *huma.Schema) (wireSchema, error) {
	encoded, err := json.Marshal(s)
	if err != nil {
		return wireSchema{}, fmt.Errorf("marshalling the schema: %w", err)
	}
	var emitted wireSchema
	if err := json.Unmarshal(encoded, &emitted); err != nil {
		return wireSchema{}, fmt.Errorf("re-reading the marshalled schema: %w", err)
	}
	return emitted, nil
}

func (w wireSchema) admitsNull() bool {
	if w.hasType(typeNull) {
		return true
	}
	for _, branch := range w.AnyOf {
		if branch.hasType(typeNull) {
			return true
		}
	}
	return unconstrained(w.single(), w.Ref, len(w.AnyOf)+len(w.OneOf)+len(w.AllOf))
}

// hasType reports whether the schema's `type` includes want. OpenAPI 3.1 allows
// either a string or a list of them, and huma emits both shapes.
func (w wireSchema) hasType(want string) bool {
	switch typed := w.Type.(type) {
	case string:
		return typed == want
	case []any:
		for _, one := range typed {
			if name, ok := one.(string); ok && name == want {
				return true
			}
		}
	}
	return false
}

// single is the schema's type when it has exactly one, and "" otherwise.
func (w wireSchema) single() string {
	name, _ := w.Type.(string)
	return name
}

// describe is how the emitted schema reads in a failure message.
func (w wireSchema) describe() string {
	switch {
	case w.Ref != "":
		return "$ref " + w.Ref
	case len(w.AnyOf) > 0:
		branches := make([]string, 0, len(w.AnyOf))
		for _, branch := range w.AnyOf {
			branches = append(branches, branch.describe())
		}
		return "anyOf [" + strings.Join(branches, ", ") + "]"
	case w.Type != nil:
		return fmt.Sprintf("type %v", w.Type)
	default:
		return "nothing at all"
	}
}
