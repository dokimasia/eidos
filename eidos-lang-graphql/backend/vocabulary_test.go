// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The allocations of a description: the text of a block string, and the
// text and the quoted line of a description of one line.
const (
	blockDocsAllocs = 1
	lineDocsAllocs  = 1 + 1
)

// TestVocabulary pins the template vocabulary: the names of its helpers and
// the description of the helper that every backend declares.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a helper under each name of the vocabulary", func(t *testing.T) {
			t.Parallel()

			names := slices.Collect(maps.Keys(backend.Funcs(&render.ImportSet{})))
			assert.Permutation(t, names, []string{
				backend.FuncDocs, backend.FuncStruct, backend.FuncInterface, backend.FuncEnum,
				backend.FuncAlias, backend.FuncFunction, backend.FuncSchema, backend.FuncExtension,
			}, "the names of the helpers")
		})

		t.Run("returns the documentation helper under the name that every backend declares", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, backend.Funcs(&render.ImportSet{}), render.HelperDocs,
				"the vocabulary declares the shared documentation helper")
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
				name: "returns one line as a string",
				give: []string{"Row is one record."},
				want: "\"Row is one record.\"\n",
			},
			{
				name: "returns two lines as a block string",
				give: []string{"Row is one record.", "It has a key."},
				want: "\"\"\"\nRow is one record.\nIt has a key.\n\"\"\"\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&backend.Vocabulary{}).Docs(tt.give), tt.want, "the description")
			})
		}
	})
}

// Docs allocates the text of the description, and the quoted line of a
// description of one line. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestVocabularyAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	block := []string{"Row is one record.", "It has a key."}
	line := []string{"Row is one record."}
	var got string
	assert.MaxAllocs(t, func() { got = v.Docs(block) }, blockDocsAllocs, "Docs allocates the block string once")
	assert.HasPrefix(t, got, `"""`, "Docs writes a block string")
	assert.MaxAllocs(t, func() { got = v.Docs(line) }, lineDocsAllocs,
		"Docs allocates the description and its quoted line")
	assert.HasPrefix(t, got, `"Row`, "Docs writes a string")
	assert.MaxAllocs(t, func() { got = v.Docs(nil) }, 0, "Docs allocates nothing for no lines")
	assert.Empty(t, got, "Docs writes no description")
}

// BenchmarkVocabulary measures the description that the template of a
// generator writes before a declaration.
func BenchmarkVocabulary(b *testing.B) {
	docs := []struct {
		name   string
		give   []string
		want   string
		allocs uint64
	}{
		{
			name: "two lines", give: []string{"Row is one record.", "It has a key."},
			want: "\"\"\"\nRow is one record.\nIt has a key.\n\"\"\"\n", allocs: blockDocsAllocs,
		},
		{
			name: "one line", give: []string{"Row is one record."}, want: "\"Row is one record.\"\n",
			allocs: lineDocsAllocs,
		},
		{name: "no lines"},
	}
	b.Run("Docs", func(b *testing.B) {
		for _, tt := range docs {
			b.Run(tt.name, func(b *testing.B) {
				v := &backend.Vocabulary{}
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = v.Docs(tt.give)
				}
				assert.Equal(b, got, tt.want, "Docs writes the description")
			})
		}
	})
}
