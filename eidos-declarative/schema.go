// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"encoding/json"
	"reflect"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// [Schema] returns a schema of this dialect with this title.
const (
	dialect = "https://json-schema.org/draft/2020-12/schema"
	title   = "The manifest of a declarative plugin of eidos"
)

// The keywords and the types of JSON Schema that the schema of the manifest
// and the schemas of its vocabularies state.
const (
	keywordSchema = "$schema"
	keywordTitle  = "title"
	keywordType   = "type"
	keywordConst  = "const"
	keywordEnum   = "enum"
	keywordItems  = "items"
	typeString    = "string"
	typeInteger   = "integer"
	typeBoolean   = "boolean"
	typeArray     = "array"
)

// Schema returns the JSON Schema of the manifest format, draft 2020-12, as
// indented JSON that ends with a newline. It builds the schema from the
// yaml, doc and schema tags of the manifest's types through
// [jsonschema.Of], so each key that the decoder accepts is a property of
// the schema, with the doc tag of its field as its description. The type
// of a closed vocabulary, such as the type of a param, has an enum of its
// spellings. Schema returns the same bytes on each call.
func Schema() []byte {
	root := jsonschema.Of(reflect.TypeFor[manifest]())
	root[keywordSchema] = dialect
	root[keywordTitle] = title
	// MarshalIndent cannot fail on a tree of maps, slices, strings, numbers
	// and booleans.
	out, _ := json.MarshalIndent(root, "", "  ")
	return append(out, '\n')
}
