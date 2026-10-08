// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"reflect"
	"strings"
)

// [Schema] reads these struct tags. The yaml tag contains the key of a
// field, and the doc tag contains its description. The schema tag contains
// required for a key that every file of the type has.
const (
	yamlTag     = "yaml"
	docTag      = "doc"
	schemaTag   = "schema"
	requiredTag = "required"
)

// [Schema] returns a schema of this dialect with this title.
const (
	dialect = "https://json-schema.org/draft/2020-12/schema"
	title   = "The config file of a binary built with eidos"
)

// schemer is a type with its own JSON Schema, such as [Bytes].
type schemer interface {
	schema() map[string]any
}

// Schema returns the JSON Schema of the config file format, draft 2020-12,
// as indented JSON that ends with a newline. A file is valid when it is a
// valid [Document] or a valid [List]. Each options section is an object
// with any keys, because each plugin defines its own options and Build
// validates them.
//
// Schema builds the schema from the yaml, doc and schema tags of the
// fields, and returns the same bytes on each call.
func Schema() []byte {
	root := map[string]any{
		"$schema": dialect,
		"title":   title,
		"oneOf":   []any{of(reflect.TypeFor[Document]()), of(reflect.TypeFor[List]())},
	}
	// MarshalIndent cannot fail on a tree of maps, slices, strings, numbers
	// and booleans.
	out, _ := json.MarshalIndent(root, "", "  ")
	return append(out, '\n')
}

// of returns the JSON Schema of the values of t.
//
//   - A pointer has the schema of its element.
//   - A type with a schema method has the schema that the method returns.
//   - A struct is an object with one property for each field.
//   - A slice is an array, and a map with string keys is an object.
//   - A string and a bool have the JSON types string and boolean.
//   - Any other type, such as an interface, allows any value.
func of(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		return of(t.Elem())
	}
	if s, ok := reflect.TypeAssert[schemer](reflect.Zero(t)); ok {
		return s.schema()
	}
	switch t.Kind() {
	case reflect.Struct:
		return object(t)
	case reflect.Slice:
		return map[string]any{"type": "array", "items": of(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": of(t.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	default:
		return map[string]any{}
	}
}

// object returns the JSON Schema of a struct type. Each field is a property
// under its yaml key, with the doc tag as its description. A field with the
// schema tag required adds its key to the required keys. The object allows
// no other keys, because the decoder rejects them.
func object(t reflect.Type) map[string]any {
	properties := map[string]any{}
	var required []string
	for f := range t.Fields() {
		key, _, _ := strings.Cut(f.Tag.Get(yamlTag), ",")
		property := of(f.Type)
		property["description"] = f.Tag.Get(docTag)
		properties[key] = property
		if f.Tag.Get(schemaTag) == requiredTag {
			required = append(required, key)
		}
	}
	out := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
