// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package jsonschema

import (
	"reflect"
	"strings"
)

// The struct tags that [Of] reads.
const (
	// yamlTag contains the key of a field, before the first comma.
	yamlTag = "yaml"
	// docTag contains the description of a field.
	docTag = "doc"
	// schemaTag contains required for a field that every value has.
	schemaTag = "schema"
	// requiredTag is the value of schemaTag that makes a field required.
	requiredTag = "required"
	// skipKey is the yaml key that leaves a field out.
	skipKey = "-"
	// keySeparator ends the key in a yaml tag.
	keySeparator = ","
)

// The keywords and the type names of JSON Schema that [Of] writes.
const (
	keywordType          = "type"
	keywordItems         = "items"
	keywordProperties    = "properties"
	keywordAdditional    = "additionalProperties"
	keywordPropertyNames = "propertyNames"
	keywordRequired      = "required"
	keywordDescription   = "description"

	typeArray   = "array"
	typeObject  = "object"
	typeString  = "string"
	typeBoolean = "boolean"
	typeInteger = "integer"
)

// Schemer is a type with a JSON Schema of its own, such as an enumeration
// that a decoder reads from a fixed list of spellings.
type Schemer interface {
	// JSONSchema returns the schema of the values of the type. [Of] calls
	// it on the zero value of the type, and a call returns a new map.
	JSONSchema() map[string]any
}

// Of returns the JSON Schema, draft 2020-12, of the values of t.
//
//   - A pointer has the schema of its element.
//   - A type whose zero value implements [Schemer] has the schema that
//     its JSONSchema method returns.
//   - A struct is a closed object with one property for each field that
//     a YAML decoder reads. The property has the field's yaml key and its
//     doc tag as the description, and a field whose schema tag is
//     required is a required property.
//   - A slice is an array, and a map is an object whose properties have
//     the schema of the map's elements. Where the zero value of the map's
//     key type implements [Schemer], the names of the properties have the
//     schema that its JSONSchema method returns.
//   - A string and a bool have the JSON types string and boolean, and
//     every signed and unsigned integer has the JSON type integer.
//   - Any other type, such as an interface, allows any value.
//
// Of returns a new map on each call. It cannot fail.
//
// # Allocation contract
//
// Of allocates the maps and the lists of the schema that it returns, and
// what a [Schemer] allocates.
func Of(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		return Of(t.Elem())
	}
	if s, ok := reflect.TypeAssert[Schemer](reflect.Zero(t)); ok {
		return s.JSONSchema()
	}
	switch t.Kind() {
	case reflect.Struct:
		return object(t)
	case reflect.Slice:
		return map[string]any{keywordType: typeArray, keywordItems: Of(t.Elem())}
	case reflect.Map:
		out := map[string]any{keywordType: typeObject, keywordAdditional: Of(t.Elem())}
		if s, ok := reflect.TypeAssert[Schemer](reflect.Zero(t.Key())); ok {
			out[keywordPropertyNames] = s.JSONSchema()
		}
		return out
	case reflect.String:
		return map[string]any{keywordType: typeString}
	case reflect.Bool:
		return map[string]any{keywordType: typeBoolean}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{keywordType: typeInteger}
	default:
		return map[string]any{}
	}
}

// object returns the JSON Schema of a struct type. The schema is a closed
// object with one property for each field that a YAML decoder reads,
// because the decoder refuses any other key. The decoder reads neither an
// unexported field that is not embedded nor a field with the yaml key -,
// so the object has no property for either. The required list keeps the
// order of the fields, and an object without a required field has no
// required list.
func object(t reflect.Type) map[string]any {
	properties := map[string]any{}
	var required []string
	for f := range t.Fields() {
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		key, _, _ := strings.Cut(f.Tag.Get(yamlTag), keySeparator)
		if key == skipKey {
			continue
		}
		if key == "" {
			key = strings.ToLower(f.Name)
		}
		property := Of(f.Type)
		property[keywordDescription] = f.Tag.Get(docTag)
		properties[key] = property
		if f.Tag.Get(schemaTag) == requiredTag {
			required = append(required, key)
		}
	}
	out := map[string]any{keywordType: typeObject, keywordProperties: properties, keywordAdditional: false}
	if len(required) > 0 {
		out[keywordRequired] = required
	}
	return out
}
