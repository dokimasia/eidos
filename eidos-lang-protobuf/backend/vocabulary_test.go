// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// filledLine is a word as wide as the documentation, so the wrap starts a
// line with the word after it.
var filledLine = strings.Repeat("a", textfmt.DocsWidth)

// The allocations of the vocabulary.
const (
	// docsAllocs is the documentation of a declaration: the comments in one
	// buffer.
	docsAllocs = 1
	// wrapDocsAllocs is the wrap of a single-spaced text: the list of its
	// lines, each of which is a part of the text.
	wrapDocsAllocs = 1
	// newVocabularyAllocs is the vocabulary of a file: the vocabulary and
	// its two record maps.
	newVocabularyAllocs = 3
	// funcsAllocs is the template vocabulary of a file: the vocabulary, the
	// map of the helpers and the three helpers that are methods of the
	// vocabulary.
	funcsAllocs = 8
)

// TestVocabulary pins the names of the template vocabulary and the
// documentation that it writes.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the helpers of the templates under their names", func(t *testing.T) {
			t.Parallel()

			got := slices.Sorted(maps.Keys(backend.Funcs(&render.ImportSet{})))
			assert.Equal(t, got, []string{
				backend.FuncBlock, backend.FuncDocs, backend.FuncExtension,
				backend.FuncFileOptions, backend.FuncSyntax, backend.FuncUseImports, backend.FuncWrapDocs,
			}, "the names of the vocabulary")
		})
	})

	t.Run("WrapDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("wraps the text at the width of the documentation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" next"), []string{filledLine, "next"},
				"the word past the width starts the next line")
		})

		t.Run("starts a line with a word that starts with @", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" @see next"), []string{filledLine, "@see next"},
				"a protobuf comment is text")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []string
			want string
		}{
			{name: "returns nothing for no lines"},
			{
				name: "returns one comment for each line",
				give: []string{"Row is one record.", "It has a key."},
				want: "// Row is one record.\n// It has a key.\n",
			},
			{
				name: "returns the mark alone for an empty line",
				give: []string{"Row is one record.", "", "It has a key."},
				want: "// Row is one record.\n//\n// It has a key.\n",
			},
			{
				name: "returns one comment for each part of a line with a line break",
				give: []string{"Row is one record.\r\nIt has a key."},
				want: "// Row is one record.\n// It has a key.\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Docs(tt.give), tt.want, "the documentation as line comments")
			})
		}
	})
}

// The render binds a vocabulary to each file, and Docs allocates its
// comments in one buffer. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestVocabularyAllocs(t *testing.T) {
	set := &render.ImportSet{}
	var (
		v     *backend.Vocabulary
		funcs template.FuncMap
		got   string
	)
	assert.MaxAllocs(t, func() { v = backend.NewVocabulary(set) }, newVocabularyAllocs,
		"NewVocabulary allocates the vocabulary and its records")
	assert.NotNil(t, v, "NewVocabulary returns the vocabulary")
	assert.MaxAllocs(t, func() { funcs = backend.Funcs(set) }, funcsAllocs,
		"Funcs allocates the vocabulary and the map of its helpers")
	assert.Length(t, funcs, 7, "Funcs returns the seven helpers")

	lines := []string{"Row is one record.", "It has a key."}
	assert.MaxAllocs(t, func() { got = backend.Docs(lines) }, docsAllocs, "Docs allocates the comments once")
	assert.HasPrefix(t, got, "// Row", "Docs writes the comments")

	wrapped := filledLine + " next"
	var wrappedLines []string
	assert.MaxAllocs(t, func() { wrappedLines = backend.WrapDocs(wrapped) }, wrapDocsAllocs,
		"WrapDocs allocates the list of lines")
	assert.Equal(t, wrappedLines, []string{filledLine, "next"}, "WrapDocs wraps the text")
}

// BenchmarkVocabulary measures the vocabulary that the render binds to each
// file, and the documentation that each declaration of a file writes.
func BenchmarkVocabulary(b *testing.B) {
	b.Run("NewVocabulary", func(b *testing.B) {
		set := &render.ImportSet{}
		c := bench.Start(b).MaxAllocs(newVocabularyAllocs)
		defer c.End()
		var v *backend.Vocabulary
		for c.Loop() {
			v = backend.NewVocabulary(set)
		}
		assert.NotNil(b, v, "NewVocabulary returns the vocabulary")
	})

	b.Run("Funcs", func(b *testing.B) {
		set := &render.ImportSet{}
		c := bench.Start(b).MaxAllocs(funcsAllocs)
		defer c.End()
		var funcs template.FuncMap
		for c.Loop() {
			funcs = backend.Funcs(set)
		}
		assert.Length(b, funcs, 7, "Funcs returns the seven helpers")
	})

	b.Run("WrapDocs", func(b *testing.B) {
		wrapped := filledLine + " next"
		c := bench.Start(b).MaxAllocs(wrapDocsAllocs)
		defer c.End()
		var lines []string
		for c.Loop() {
			lines = backend.WrapDocs(wrapped)
		}
		assert.Equal(b, lines, []string{filledLine, "next"}, "WrapDocs wraps the text")
	})

	b.Run("Docs", func(b *testing.B) {
		lines := []string{"Row is one record.", "It has a key."}
		c := bench.Start(b).MaxAllocs(docsAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = backend.Docs(lines)
		}
		assert.HasPrefix(b, got, "// Row", "Docs writes the comments")
	})
}
