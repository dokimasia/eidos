// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// Annotations are what a generated declaration carries into a
// target that writes markers, so the codec holds them across the
// round trip and drops them when empty.
func TestAnnotation(t *testing.T) {
	t.Parallel()

	t.Run("rides its declaration through the codec", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name: "Row",
			Annotations: emit.Annotations{
				{Name: "Entity"},
				{Name: "Table", Args: []string{`name = "rows"`}},
			},
		}
		data, err := emit.EncodeJSON(s)
		assert.NoError(t, err, "the annotated declaration encodes")
		back, err := emit.DecodeJSON(data)
		assert.NoError(t, err, "and decodes")
		decoded, held := back.(*emit.Struct)
		assert.True(t, held, "the declaration decodes to its kind")
		assert.Equal(t, decoded, s,
			"names and argument spellings survive verbatim")
	})

	t.Run("an empty list disappears from the encoding", func(t *testing.T) {
		t.Parallel()

		data, err := emit.EncodeJSON(&emit.Struct{Name: "Row"})
		assert.NoError(t, err, "the bare declaration encodes")
		assert.False(t, strings.Contains(string(data), "annotations"),
			"an untouched list spells nothing")
	})
}
