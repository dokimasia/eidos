// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package jsonschema_test

import (
	"encoding"
	"reflect"
	"testing"
	"time"

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
	tagsField = "Tags"
	modeKey   = "mode"
	onKey     = "on"
	dashKey   = "-"
	nameDoc   = "The name of the document."
	tagsDoc   = "The tags of the document."
	modeDoc   = "The mode of the document."
	onDoc     = "When true, the document is on."
	dashDoc   = "The dash of the document."
	quietMode = "quiet"
	loudMode  = "loud"
)

// mode is an enumeration with a schema of its own, which a decoder reads
// from its text.
type mode string

var (
	_ jsonschema.Schemer       = mode("")
	_ encoding.TextUnmarshaler = (*mode)(nil)
)

// JSONSchema returns the schema of the two spellings of a mode, a new map
// on each call.
func (mode) JSONSchema() map[string]any {
	return map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}}
}

// UnmarshalText sets m to text. It returns no error.
func (m *mode) UnmarshalText(text []byte) error {
	*m = mode(text)
	return nil
}

// level is an integer that a decoder reads from its text.
type level int

var _ encoding.TextUnmarshaler = (*level)(nil)

// UnmarshalText sets l to the length of text. It returns no error.
func (l *level) UnmarshalText(text []byte) error {
	*l = level(len(text))
	return nil
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

// options has the json keys of a required field with a key, a field
// without a key, and a field that its key leaves out.
type options struct {
	Name    string   `json:"name,omitempty" schema:"required" doc:"The name of the document."`
	Tags    []string `                                        doc:"The tags of the document."`
	Skipped string   `json:"-"`
}

// dashed has a field whose key is -, which a tag of - and a comma states.
type dashed struct {
	Dash string `yaml:"-," doc:"The dash of the document."`
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
			name     string
			giveType reflect.Type
			giveTag  jsonschema.Tag
			want     map[string]any
		}{
			{
				name:     "returns the schema of the element for a pointer",
				giveType: reflect.TypeFor[*string](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{keyType: typeString},
			},
			{
				name:     "returns the schema of a Schemer that unmarshals its text",
				giveType: reflect.TypeFor[mode](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}},
			},
			{
				name:     "returns the string type for a type that unmarshals its text",
				giveType: reflect.TypeFor[level](),
				giveTag:  jsonschema.TagJSON,
				want:     map[string]any{keyType: typeString},
			},
			{
				name:     "returns the string and integer types for a duration",
				giveType: reflect.TypeFor[time.Duration](),
				giveTag:  jsonschema.TagJSON,
				want:     map[string]any{keyType: []string{typeString, typeInteger}},
			},
			{
				name:     "returns a closed object with one property for each field that a YAML decoder reads",
				giveType: reflect.TypeFor[document](),
				giveTag:  jsonschema.TagYAML,
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
				name:     "returns a closed object with one property for each field that a JSON decoder reads",
				giveType: reflect.TypeFor[options](),
				giveTag:  jsonschema.TagJSON,
				want: map[string]any{
					keyType: typeObject,
					keyProperties: map[string]any{
						nameKey: map[string]any{keyType: typeString, keyDescription: nameDoc},
						tagsField: map[string]any{
							keyType: typeArray, keyItems: map[string]any{keyType: typeString}, keyDescription: tagsDoc,
						},
					},
					keyAdditional: false,
					keyRequired:   []string{nameKey},
				},
			},
			{
				name:     "returns the key - for a tag of - and a comma",
				giveType: reflect.TypeFor[dashed](),
				giveTag:  jsonschema.TagYAML,
				want: map[string]any{
					keyType: typeObject,
					keyProperties: map[string]any{
						dashKey: map[string]any{keyType: typeString, keyDescription: dashDoc},
					},
					keyAdditional: false,
				},
			},
			{
				name:     "leaves the required list out of an object without a required field",
				giveType: reflect.TypeOf(sealed{key: nameKey}),
				giveTag:  jsonschema.TagYAML,
				want: map[string]any{
					keyType: typeObject,
					keyProperties: map[string]any{
						nameKey: map[string]any{keyType: typeString, keyDescription: nameDoc},
					},
					keyAdditional: false,
				},
			},
			{
				name:     "returns an object of the element's schema for a map",
				giveType: reflect.TypeFor[map[string]bool](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{keyType: typeObject, keyAdditional: map[string]any{keyType: typeBoolean}},
			},
			{
				name:     "returns the schema of a Schemer key type as the names of the properties of a map",
				giveType: reflect.TypeFor[map[mode]bool](),
				giveTag:  jsonschema.TagYAML,
				want: map[string]any{
					keyType:          typeObject,
					keyAdditional:    map[string]any{keyType: typeBoolean},
					keyPropertyNames: map[string]any{keyType: typeString, keyEnum: []any{quietMode, loudMode}},
				},
			},
			{
				name:     "returns the integer type for a signed integer",
				giveType: reflect.TypeFor[int64](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{keyType: typeInteger},
			},
			{
				name:     "returns the integer type for an unsigned integer",
				giveType: reflect.TypeFor[uint8](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{keyType: typeInteger},
			},
			{
				name:     "allows any value for an interface",
				giveType: reflect.TypeFor[any](),
				giveTag:  jsonschema.TagYAML,
				want:     map[string]any{},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, jsonschema.Of(tt.giveType, tt.giveTag), tt.want, "the schema of the type")
			})
		}

		t.Run("returns a new map on each call", func(t *testing.T) {
			t.Parallel()

			give := reflect.TypeFor[document]()
			assert.NotEqual(t, jsonschema.Of(give, jsonschema.TagYAML), jsonschema.Of(give, jsonschema.TagYAML),
				"two calls share no map", assert.ByIdentity())
		})
	})
}
