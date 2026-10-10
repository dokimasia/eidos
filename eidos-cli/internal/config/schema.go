// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"reflect"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// [Schema] returns a schema of this dialect with this title.
const (
	dialect = "https://json-schema.org/draft/2020-12/schema"
	title   = "The config file of a binary built with eidos"
)

// The keywords and the types of JSON Schema that the schema of the file and
// the schemas of the config's values state.
const (
	keywordSchema  = "$schema"
	keywordTitle   = "title"
	keywordOneOf   = "oneOf"
	keywordType    = "type"
	keywordMinimum = "minimum"
	keywordPattern = "pattern"
	keywordConst   = "const"
	keywordEnum    = "enum"
	typeInteger    = "integer"
	typeString     = "string"
)

// Schema returns the JSON Schema of the config file format, draft 2020-12,
// as indented JSON that ends with a newline. A file is valid when it is a
// valid [Document] or a valid [List]. Each options section is an object
// with any keys, because each plugin defines its own options and Build
// validates them.
//
// Schema builds the schema from the yaml, doc and schema tags of the
// fields through [jsonschema.Of], and returns the same bytes on each call.
func Schema() []byte {
	root := map[string]any{
		keywordSchema: dialect,
		keywordTitle:  title,
		keywordOneOf:  []any{jsonschema.Of(reflect.TypeFor[Document]()), jsonschema.Of(reflect.TypeFor[List]())},
	}
	// MarshalIndent cannot fail on a tree of maps, slices, strings, numbers
	// and booleans.
	out, _ := json.MarshalIndent(root, "", "  ")
	return append(out, '\n')
}
