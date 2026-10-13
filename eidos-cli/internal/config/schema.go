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
	keywordAdditional = "additionalProperties"
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
//
// The top level of a document and of a list admits any key that is not a
// key of the format, because the command line reads the values of its
// options from those keys and checks them itself. A document has no key
// workspaces, and a list has no other key of a document, so a file matches
// one of the two schemas at most.
func Schema(options map[string]any) []byte {
	document := jsonschema.Of(reflect.TypeFor[Document](), jsonschema.TagYAML)
	list := jsonschema.Of(reflect.TypeFor[List](), jsonschema.TagYAML)
	documentKeys, _ := document[keywordProperties].(map[string]any)
	listKeys, _ := list[keywordProperties].(map[string]any)
	if len(options) > 0 {
		section, _ := documentKeys[optionsKey].(map[string]any)
		section[keywordProperties] = options
	}
	for key := range documentKeys {
		if _, ok := listKeys[key]; !ok {
			listKeys[key] = false
		}
	}
	documentKeys[workspacesKey] = false
	document[keywordAdditional], list[keywordAdditional] = map[string]any{}, map[string]any{}
	root := map[string]any{
		keywordSchema: dialect,
		keywordTitle:  title,
		keywordOneOf:  []any{document, list},
	}
	// MarshalIndent cannot fail on a tree of maps, slices, strings, numbers
	// and booleans.
	out, _ := json.MarshalIndent(root, "", "  ")
	return append(out, '\n')
}
