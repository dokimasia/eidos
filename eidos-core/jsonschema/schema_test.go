// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package jsonschema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// The keywords and the type names of JSON Schema that the tests pin.
const (
	keyType          = "type"
	keyItems         = "items"
	keyProperties    = "properties"
	keyAdditional    = "additionalProperties"
	keyPropertyNames = "propertyNames"
	keyRequired      = "required"
	keyDescription   = "description"
	keyEnum          = "enum"

	typeArray   = "array"
	typeObject  = "object"
	typeString  = "string"
	typeBoolean = "boolean"
	typeInteger = "integer"
)

// The keys, the descriptions and the spellings of the fixture types, as
// their tags write them.
const (
	nameKey   = "name"
	tagsKey   = "tags"
	modeKey   = "mode"
	onKey     = "on"
	nameDoc   = "The name of the document."
	tagsDoc   = "The tags of the document."
	modeDoc   = "The mode of the document."
	onDoc     = "When true, the document is on."
	quietMode = "quiet"
	loudMode  = "loud"
)

// mode is an enumeration with a schema of its own.
type mode string

// JSONSchema returns the schema of the two spellings of a mode, a new map
// on each call.
func (mode) JSONSchema() map[string]any {
	return map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}}
}

// document has a required field with a key, a field with a schema of its
// own, a field without a key, and a field that its key leaves out.
type document struct {
	Name    string   `yaml:"name,omitempty" schema:"required" doc:"The name of the document."`
	Tags    []string `yaml:"tags"                             doc:"The tags of the document."`
	Mode    mode     `yaml:"mode"                             doc:"The mode of the document."`
	On      bool     `                                        doc:"When true, the document is on."`
	Skipped string   `yaml:"-"`
}

// sealed has an exported field and an unexported field, which a YAML
// decoder does not read.
type sealed struct {
	Name string `yaml:"name" doc:"The name of the document."`
	key  string
}

func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give reflect.Type
			want map[string]any
		}{
			{
				name: "returns the schema of the element for a pointer",
				give: reflect.TypeFor[*string](),
				want: map[string]any{keyType: typeString},
			},
			{
				name: "returns the schema that a Schemer returns",
				give: reflect.TypeFor[mode](),
				want: map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}},
			},
			{
				name: "returns a closed object with one property for each field that a YAML decoder reads",
				give: reflect.TypeFor[document](),
				want: map[string]any{
					keyType: typeObject,
					keyProperties: map[string]any{
						nameKey: map[string]any{keyType: typeString, keyDescription: nameDoc},
						tagsKey: map[string]any{
							keyType: typeArray, keyItems: map[string]any{keyType: typeString}, keyDescription: tagsDoc,
						},
						modeKey: map[string]any{
							keyType:        typeString,
							keyEnum:        []any{quietMode, loudMode},
							keyDescription: modeDoc,
						},
						onKey: map[string]any{keyType: typeBoolean, keyDescription: onDoc},
					},
					keyAdditional: false,
					keyRequired:   []string{nameKey},
				},
			},
			{
				name: "leaves the required list out of an object without a required field",
				give: reflect.TypeOf(sealed{key: nameKey}),
				want: map[string]any{
					keyType: typeObject,
					keyProperties: map[string]any{
						nameKey: map[string]any{keyType: typeString, keyDescription: nameDoc},
					},
					keyAdditional: false,
				},
			},
			{
				name: "returns an object of the element's schema for a map",
				give: reflect.TypeFor[map[string]bool](),
				want: map[string]any{keyType: typeObject, keyAdditional: map[string]any{keyType: typeBoolean}},
			},
			{
				name: "returns the schema of a Schemer key type as the names of the properties of a map",
				give: reflect.TypeFor[map[mode]bool](),
				want: map[string]any{
					keyType:          typeObject,
					keyAdditional:    map[string]any{keyType: typeBoolean},
					keyPropertyNames: map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}},
				},
			},
			{
				name: "returns the integer type for a signed integer",
				give: reflect.TypeFor[int64](),
				want: map[string]any{keyType: typeInteger},
			},
			{
				name: "returns the integer type for an unsigned integer",
				give: reflect.TypeFor[uint8](),
				want: map[string]any{keyType: typeInteger},
			},
			{
				name: "allows any value for an interface",
				give: reflect.TypeFor[any](),
				want: map[string]any{},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, jsonschema.Of(tt.give), tt.want, "the schema of the type")
			})
		}

		t.Run("returns a new map on each call", func(t *testing.T) {
			t.Parallel()

			give := reflect.TypeFor[document]()
			assert.NotEqual(t, jsonschema.Of(give), jsonschema.Of(give), "two calls share no map", assert.ByIdentity())
		})
	})
}
