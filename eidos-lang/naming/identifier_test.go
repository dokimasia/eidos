// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// The predicate runs before every wire-name refusal, so the shape's
// edges are pinned here.
func TestIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("IsIdentifier", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for letters around an underscore", give: "content_type", want: true},
			{name: "reports true for an underscore opening", give: "_x9", want: true},
			{name: "reports false for a hyphen", give: "content-type", want: false},
			{name: "reports false for a leading digit", give: "9lives", want: false},
			{name: "reports false for a non-ASCII letter", give: "café", want: false},
			{name: "reports false for the empty string", give: "", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, naming.IsIdentifier(tt.give), tt.want, "whether the name has the shape")
			})
		}
	})
}

// The predicate allocates nothing in the ordinary run, which runs no
// benchmark.
func TestIdentifierZeroAlloc(t *testing.T) {
	var got bool
	assert.MaxAllocs(t, func() { got = naming.IsIdentifier("content_type") }, 0, "IsIdentifier allocates nothing")
	assert.True(t, got, "IsIdentifier reports true for an identifier")
}

// BenchmarkIdentifier measures the shape check a backend runs on every
// wire name.
func BenchmarkIdentifier(b *testing.B) {
	b.Run("IsIdentifier", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := false
		for c.Loop() {
			got = naming.IsIdentifier("content_type")
		}
		assert.True(b, got, "IsIdentifier reports true for an identifier")
	})
}
