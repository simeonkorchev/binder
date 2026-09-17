// Package humaschema makes the generated OpenAPI document tell the truth about
// which fields can be null.
//
// Huma derives a property's nullability from the Go type it generates the
// schema from, and gets three cases wrong for the shapes this API uses:
//
//   - a pointer to a struct decays to a bare `$ref`, so `*scanCard` is
//     documented as a card that is always there;
//   - a pointer to a named array type that unmarshals from text — `*uuid.UUID`
//     is one — decays to a plain `string`, because the registry treats "pointer
//     to array" as "array" before it ever looks at the pointer;
//   - every slice is nullable by default (huma.DefaultArrayNullable), while
//     every mapper in this codebase returns a made slice, so `null` is a value
//     the client is told to expect and can never receive
//     (000-principles.md section 8b).
//
// The first two lie in the direction that crashes a client: it dereferences
// what the contract promised is there. The third lies the other way and costs
// every client a null check that can never fire.
//
// Config installs one rule over all of them — a Go pointer means null on the
// wire and nothing else does — applied to every registered schema, so a DTO
// added tomorrow inherits it without being touched. Disagreements is the same
// rule read back off the emitted document, for the test that fails if the two
// ever drift apart again.
package humaschema

import (
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

const (
	// schemaRefPrefix is where huma.DefaultConfig puts component schemas. It is
	// needed to ask the registry which Go type a schema came from, and is
	// mirrored here rather than read back because huma keeps it unexported.
	// TestSchemasResolveBackToTheirGoTypes pins that the two still agree.
	schemaRefPrefix = "#/components/schemas/"

	// typeNull is JSON Schema's null type. huma has no constant for it: its own
	// nullability is a bool that turns `"type": "x"` into `["x", "null"]`, which
	// cannot express a nullable `$ref`.
	typeNull = "null"
)

// Config is huma.DefaultConfig with the nullability of every generated schema
// corrected to match the Go types it came from.
//
// Every API in this repo is built from it — the server, the OpenAPI generator
// behind `make gen-spec`, and the handler suites — so a spec that drives the
// real stack validates requests against the same document the app is generated
// from.
func Config(title, version string) huma.Config {
	config := huma.DefaultConfig(title, version)
	config.OnAddOperation = append(config.OnAddOperation, alignOperation)
	return config
}

// alignOperation re-aligns the whole registry after each operation is added.
//
// It hooks registration rather than schema generation because huma's registry
// generates a struct's fields by calling itself, not the caller's wrapper, so a
// decorating Registry only ever sees the outermost type. OnAddOperation is the
// first point at which the schemas an operation pulled in are all present. The
// pass is a function of the Go types alone, so running it once per operation
// re-does settled work and changes nothing.
func alignOperation(oapi *huma.OpenAPI, _ *huma.Operation) {
	if oapi == nil || oapi.Components == nil || oapi.Components.Schemas == nil {
		return
	}
	alignRegistry(oapi.Components.Schemas)
}

// alignRegistry corrects every property of every registered schema.
func alignRegistry(registry huma.Registry) {
	for name, schema := range registry.Map() {
		goType := registry.TypeFromRef(schemaRefPrefix + name)
		if goType == nil || schema == nil {
			continue
		}
		for jsonName, field := range wireFields(goType) {
			property := schema.Properties[jsonName]
			if property == nil {
				continue
			}
			schema.Properties[jsonName] = alignSchema(property, field.Type, nullableTag(field))
		}
	}
}

// alignSchema makes s admit null exactly when a value of goType can marshal to
// it, recursing into the items of an array so `[]*slotBody` documents the empty
// pocket the grid is laid out with.
//
// override is the field's `nullable` tag, which still wins: it is how a slice
// that genuinely can be nil, or a pointer that is never sent as null, says so.
func alignSchema(s *huma.Schema, goType reflect.Type, override *bool) *huma.Schema {
	wantsNull := goType.Kind() == reflect.Pointer || goType.Kind() == reflect.Interface
	if override != nil {
		wantsNull = *override
	}

	if element := deref(goType); isList(element) && s.Type == huma.TypeArray && s.Items != nil {
		// The element's own tag cannot be read here: a tag belongs to a field,
		// and the items of a slice have none.
		s.Items = alignSchema(s.Items, element.Elem(), nil)
	}

	return setNullable(s, wantsNull)
}

// setNullable returns a schema equivalent to s that admits null iff wantsNull.
//
// A `$ref` cannot carry huma's nullability flag — the flag rewrites `type`,
// which a reference does not have, and huma panics outright if a field tries —
// so a nullable reference is expressed the way JSON Schema 2020-12 expresses
// it, as a choice between the reference and null.
func setNullable(s *huma.Schema, wantsNull bool) *huma.Schema {
	if admitsNull(s) == wantsNull {
		return s
	}
	if !wantsNull {
		// Nothing but huma's own flag can have added the null: this pass only
		// ever builds the anyOf below for a field it decided wants one, and that
		// decision is a function of the Go type, which has not changed.
		s.Nullable = false
		return s
	}
	if s.Ref == "" && s.Type != huma.TypeObject {
		s.Nullable = true
		return s
	}
	return &huma.Schema{AnyOf: []*huma.Schema{s, {Type: typeNull}}}
}

// admitsNull reports whether s accepts a null.
//
// A schema with no type at all constrains nothing — it is what huma generates
// for an `any` field — so it already admits null and is left alone rather than
// being given a null branch that would say nothing new.
func admitsNull(s *huma.Schema) bool {
	if s.Nullable || s.Type == typeNull {
		return true
	}
	for _, branch := range s.AnyOf {
		if branch.Type == typeNull {
			return true
		}
	}
	return unconstrained(s.Type, s.Ref, len(s.AnyOf)+len(s.OneOf)+len(s.AllOf))
}

// unconstrained reports whether a schema with this type, this $ref and this many
// composition branches says anything at all about the value's shape.
func unconstrained(schemaType, ref string, branches int) bool {
	return schemaType == "" && ref == "" && branches == 0
}

// wireFields returns the fields of goType that huma turns into properties,
// keyed by the name they are sent under.
//
// It is empty for anything but a struct, which is what makes a non-struct
// registered schema (a named slice, an enum with its own SchemaProvider) fall
// through the caller's loop untouched.
func wireFields(goType reflect.Type) map[string]reflect.StructField {
	if goType.Kind() != reflect.Struct {
		return nil
	}

	fields := map[string]reflect.StructField{}
	for _, field := range reflect.VisibleFields(goType) {
		if !field.IsExported() || (field.Anonymous && deref(field.Type).Kind() == reflect.Struct) {
			// An embedded struct is flattened into its own fields, which
			// VisibleFields already returned alongside it.
			continue
		}
		if name, ok := wireName(field); ok {
			fields[name] = field
		}
	}
	return fields
}

// wireName is the JSON name of a field, and whether it is sent at all.
func wireName(field reflect.StructField) (string, bool) {
	name := field.Name
	if tag, ok := field.Tag.Lookup("json"); ok {
		if tagged, _, _ := strings.Cut(tag, ","); tagged != "" {
			name = tagged
		}
	}
	if name == "-" || name == "_" {
		return "", false
	}
	return name, true
}

// nullableTag reads a field's `nullable` tag, or nil when it has none.
func nullableTag(field reflect.StructField) *bool {
	tag, ok := field.Tag.Lookup("nullable")
	if !ok {
		return nil
	}
	value := tag == "true"
	return &value
}

func isList(goType reflect.Type) bool {
	return goType.Kind() == reflect.Slice || goType.Kind() == reflect.Array
}

func deref(goType reflect.Type) reflect.Type {
	if goType.Kind() == reflect.Pointer {
		return goType.Elem()
	}
	return goType
}
