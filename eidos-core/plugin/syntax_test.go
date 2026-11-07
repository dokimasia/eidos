// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
)

// A satellite declares one CommentSyntax for both kits, so the type has
// line forms with a canonical first form and block forms with their
// gutters. A data family states the syntax of the format of its file.
func TestCommentSyntax(t *testing.T) {
	t.Parallel()

	golike := plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
	assert.Equal(t, golike.Line[0], "//", "the first line form is canonical")
	assert.Length(t, golike.Blocks, 2, "the syntax lists the doc block with a gutter beside the plain form")

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.CommentSyntax
			want bool
		}{
			{name: "reports true for the zero syntax", want: true},
			{
				name: "reports true for empty lists of forms",
				give: plugin.CommentSyntax{Line: []string{}, Blocks: []plugin.CommentBlock{}},
				want: true,
			},
			{name: "reports false for a syntax with a line form", give: plugin.CommentSyntax{Line: []string{"#"}}},
			{
				name: "reports false for a syntax with a block form",
				give: plugin.CommentSyntax{Blocks: []plugin.CommentBlock{{Open: "<!--", Close: "-->"}}},
			},
			{name: "reports false for a syntax with directives alone", give: plugin.CommentSyntax{Directives: true}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.IsZero(), tt.want,
					"IsZero reports whether the syntax states a form or directives")
			})
		}
	})
}

// TestCommentSyntaxAllocs checks the ceiling of IsZero in the ordinary
// run, which runs no benchmark.
func TestCommentSyntaxAllocs(t *testing.T) {
	lock := plugin.CommentSyntax{Line: []string{"#"}}
	zero := true
	assert.MaxAllocs(t, func() { zero = lock.IsZero() }, 0, "IsZero allocates nothing")
	assert.False(t, zero, "IsZero reports false for a line form")
}

// BenchmarkCommentSyntax measures IsZero of the syntax of a lock file.
func BenchmarkCommentSyntax(b *testing.B) {
	b.Run("IsZero", func(b *testing.B) {
		lock := plugin.CommentSyntax{Line: []string{"#"}}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		zero := true
		for c.Loop() {
			zero = lock.IsZero()
		}
		assert.False(b, zero, "IsZero reports false for a line form")
	})
}
