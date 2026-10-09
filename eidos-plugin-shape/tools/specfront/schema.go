// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"encoding/json"
	"reflect"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// The keywords and the values of the published schema's root.
const (
	keywordSchema = "$schema"
	keywordTitle  = "title"
	keywordOneOf  = "oneOf"
	draft         = "https://json-schema.org/draft/2020-12/schema"
	title         = "A spec of the shape catalog: a shape, a mixin or a contract"
	indent        = "  "
)

// Schema returns the published JSON Schema of a spec, draft 2020-12, as
// indented JSON that ends with a line break. The schema is a oneOf of the
// closed objects of [Shape], [Mixin] and [Contract], which the JSON Schema
// builder derives from their tags, so the published schema and the
// decoder require the same sections. Two calls return equal bytes,
// because the encoder orders the keys of each object.
//
// Error modes: none, as every value of the schema is a map, a list, a
// string or a bool that JSON encodes.
func Schema() []byte {
	root := map[string]any{
		keywordSchema: draft,
		keywordTitle:  title,
		keywordOneOf: []any{
			jsonschema.Of(reflect.TypeFor[Shape]()),
			jsonschema.Of(reflect.TypeFor[Mixin]()),
			jsonschema.Of(reflect.TypeFor[Contract]()),
		},
	}
	out, _ := json.MarshalIndent(root, "", indent)
	return append(out, lineBreak...)
}
