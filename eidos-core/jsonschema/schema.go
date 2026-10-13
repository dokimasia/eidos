// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package jsonschema

import (
	"encoding"
	"reflect"
	"strings"
	"time"
)

// The struct tags that [Of] reads beside the tag of the keys.
const (
	// docTag contains the description of a field.
	docTag = "doc"
	// schemaTag contains required for a field that every value has.
	schemaTag = "schema"
	// requiredTag is the value of schemaTag that makes a field required.
	requiredTag = "required"
	// skipKey is the tag that leaves a field out.
	skipKey = "-"
	// keySeparator ends the key in the tag of the keys.
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

// The types that [Of] maps to JSON types of their own.
var (
	durationType    = reflect.TypeFor[time.Duration]()
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// Tag is the name of the struct tag that contains the key of a field. A
// decoder reads the value of the field under the key.
type Tag string

const (
	// TagYAML is the tag of a YAML decoder. A field without a key has the
	// name of the field in lower case.
	TagYAML Tag = "yaml"
	// TagJSON is the tag of encoding/json. A field without a key has the
	// name of the field.
	TagJSON Tag = "json"
)

// Schemer is a type with a JSON Schema of its own, such as an enumeration
// that a decoder reads from a fixed list of spellings.
type Schemer interface {
	// JSONSchema returns the schema of the values of the type. [Of] calls
	// it on the zero value of the type, and a call returns a new map.
	JSONSchema() map[string]any
}

// Of returns the JSON Schema, draft 2020-12, of the values of t. tag is the
// struct tag that contains the key of each field of a struct below t.
//
//   - A pointer has the schema of its element.
//   - A type whose zero value implements [Schemer] has the schema that
//     its JSONSchema method returns.
//   - A [time.Duration] has the JSON types string and integer: the text of
//     a duration, such as 1m30s, or a number of nanoseconds.
//   - Any other type whose pointer implements [encoding.TextUnmarshaler]
//     has the JSON type string.
//   - A struct is a closed object with one property for each exported field
//     and each embedded field. The property has the key of the field in tag
//     and the doc tag of the field as its description. A field without a
//     key has the key that the decoder of tag gives it, and a field whose
//     tag is - has no property. A field whose schema tag is required is a
//     required property.
//   - A slice is an array, and a map is an object whose properties have
//     the schema of the map's elements. Where the zero value of the map's
//     key type implements [Schemer], the names of the properties have the
//     schema that its JSONSchema method returns.
//   - A string and a bool have the JSON types string and boolean, and
//     every signed and unsigned integer has the JSON type integer.
//   - Any other type, such as an interface, allows any value.
//
// Of reads the keys of tag as [TagJSON] does for any tag other than
// [TagYAML]. Of returns a new map on each call. It cannot fail.
//
// # Allocation contract
//
// Of allocates the maps and the lists of the schema that it returns, and
// what a [Schemer] allocates.
func Of(t reflect.Type, tag Tag) map[string]any {
	if t.Kind() == reflect.Pointer {
		return Of(t.Elem(), tag)
	}
	if s, ok := reflect.TypeAssert[Schemer](reflect.Zero(t)); ok {
		return s.JSONSchema()
	}
	if t == durationType {
		return map[string]any{keywordType: []string{typeString, typeInteger}}
	}
	if reflect.PointerTo(t).Implements(textUnmarshaler) {
		return map[string]any{keywordType: typeString}
	}
	switch t.Kind() {
	case reflect.Struct:
		return object(t, tag)
	case reflect.Slice:
		return map[string]any{keywordType: typeArray, keywordItems: Of(t.Elem(), tag)}
	case reflect.Map:
		out := map[string]any{keywordType: typeObject, keywordAdditional: Of(t.Elem(), tag)}
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

// object returns the JSON Schema of a struct type whose keys tag contains.
// The schema is a closed object with one property for each field that the
// decoder of tag reads, because the decoder refuses any other key. Neither
// decoder reads an unexported field that is not embedded, nor a field whose
// tag is -, so the object has no property for either. A tag of - and a
// comma gives the field the key -. The required list keeps the order of the
// fields, and an object without a required field has no required list.
func object(t reflect.Type, tag Tag) map[string]any {
	properties := map[string]any{}
	var required []string
	for f := range t.Fields() {
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		spelled := f.Tag.Get(string(tag))
		if spelled == skipKey {
			continue
		}
		key, _, _ := strings.Cut(spelled, keySeparator)
		if key == "" {
			key = f.Name
			if tag == TagYAML {
				key = strings.ToLower(key)
			}
		}
		property := Of(f.Type, tag)
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
