// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package jsonschema builds the JSON Schema, draft 2020-12, of a Go type
// from the tags of its fields.
//
// [Of] maps a type to the schema of its values. A struct is a closed
// object with one property for each field that a YAML decoder reads,
// under the field's yaml key, described by its doc tag, and required
// where its schema tag is required. For a type that implements
// [Schemer], Of returns the schema that the type's JSONSchema method
// returns, such as an enum of the spellings of an enumeration. A map
// whose key type implements Schemer has that schema for the names of its
// properties. The schema and a YAML decoder with known fields read one
// declaration, so the schema refuses every key that the decoder refuses
// as unknown.
//
// # Tags
//
//   - yaml: the key of the field, before the first comma. A field without
//     a key takes its name in lower case, and the key - leaves the field
//     out, as the YAML decoder does.
//   - doc: the description of the property.
//   - schema: required makes the property required.
//
// # Dependency position
//
// core/jsonschema imports only the Go stdlib.
package jsonschema
