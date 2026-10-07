// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
)

// An annotation is stored and compared through its JSON form, so the
// spelling of that form is contract.
func TestAnnotation(t *testing.T) {
	t.Parallel()

	t.Run("spells a name without arguments alone", func(t *testing.T) {
		t.Parallel()

		b, err := json.Marshal(symbol.Annotation{Name: "Deprecated"})
		assert.NoError(t, err, "an annotation encodes")
		assert.Equal(t, string(b), `{"name":"Deprecated"}`, "the empty argument list is left out")
	})

	t.Run("keeps a list's order and each argument verbatim", func(t *testing.T) {
		t.Parallel()

		list := symbol.Annotations{
			{Name: "JsonProperty", Args: []string{`"id"`}},
			{Name: "Nullable"},
		}
		assert.RoundTrip(t, func(l symbol.Annotations) ([]byte, error) { return json.Marshal(l) },
			func(b []byte) (symbol.Annotations, error) {
				var l symbol.Annotations
				err := json.Unmarshal(b, &l)
				return l, err
			}, list, "the list decodes in its order, the argument's quotes kept")
	})
}
