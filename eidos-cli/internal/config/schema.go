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
	keywordSchema     = "$schema"
	keywordTitle      = "title"
	keywordOneOf      = "oneOf"
	keywordType       = "type"
	keywordProperties = "properties"
	keywordMinimum    = "minimum"
	keywordPattern    = "pattern"
	keywordConst      = "const"
	keywordEnum       = "enum"
	typeInteger       = "integer"
	typeString        = "string"
)

// optionsKey is the key of the options of a document.
const optionsKey = "options"

// Schema returns the JSON Schema of the config file format, draft 2020-12,
// as indented JSON that ends with a newline. A file is valid when it is a
// valid [Document] or a valid [List].
//
// options maps the name of a plugin to the JSON Schema of its options.
// Each schema becomes a property of the options of a document. The options
// of a document admit any other name with any keys, because a plan
// directory adds the options of its declarative plugins and Build validates
// them. Schema builds the rest of the schema from the yaml, doc and schema
// tags of the fields through [jsonschema.Of], and returns the same bytes on
// each call with the same options.
func Schema(options map[string]any) []byte {
	document := jsonschema.Of(reflect.TypeFor[Document](), jsonschema.TagYAML)
	if len(options) > 0 {
		properties, _ := document[keywordProperties].(map[string]any)
		section, _ := properties[optionsKey].(map[string]any)
		section[keywordProperties] = options
	}
	root := map[string]any{
		keywordSchema: dialect,
		keywordTitle:  title,
		keywordOneOf:  []any{document, jsonschema.Of(reflect.TypeFor[List](), jsonschema.TagYAML)},
	}
	// MarshalIndent cannot fail on a tree of maps, slices, strings, numbers
	// and booleans.
	out, _ := json.MarshalIndent(root, "", "  ")
	return append(out, '\n')
}
