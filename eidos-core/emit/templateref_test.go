// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// A template reference is the value that keeps a generator never seeing
// languages: a name resolved in the emitting plugin's tree, and a
// payload the codec carries as generic JSON.
func TestTemplateRef(t *testing.T) {
	t.Parallel()

	t.Run("codec", func(t *testing.T) {
		t.Parallel()

		t.Run("a bare reference carries only its name", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(emit.TemplateRef{Name: "method1"})
			assert.NoError(t, err, "the reference encodes")
			assert.Equal(t, string(encoded), `{"name":"method1"}`,
				"an absent payload is omitted")
		})

		t.Run("the payload encodes deterministically", func(t *testing.T) {
			t.Parallel()

			ref := emit.TemplateRef{
				Name: "method1",
				Data: map[string]any{"zeta": 1, "alpha": "x", "mid": true},
			}
			first, err := json.Marshal(ref)
			assert.NoError(t, err, "the payload encodes")
			second, err := json.Marshal(ref)
			assert.NoError(t, err, "and encodes again")
			assert.Equal(t, string(second), string(first),
				"map keys arrive in one order")

			var decoded emit.TemplateRef
			assert.NoError(t, json.Unmarshal(first, &decoded), "and decodes")
			again, err := json.Marshal(decoded)
			assert.NoError(t, err, "a decoded reference encodes")
			assert.Equal(t, string(again), string(first),
				"the round trip returns the same bytes")
		})
	})
}
