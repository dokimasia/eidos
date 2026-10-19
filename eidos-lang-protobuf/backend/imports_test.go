// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The files that the import cases import, and the modifiers of an import.
const (
	typesProto  = "acme/types.proto"
	commonProto = "acme/common.proto"
	legacyProto = "acme/legacy.proto"
	rulesProto  = "acme/rules.proto"
	publicMod   = "public"
	weakMod     = "weak"
	optionMod   = "option"
)

// importsAllocs is the import block of a filled set: the sorted entries
// and the block.
const importsAllocs = 2

// TestImports pins the import block of a protobuf file: each file once, in
// the order of the paths, with its modifier, and the option imports last.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give func(set *render.ImportSet)
			want string
		}{
			{
				name: "returns nothing for a file without imports",
				give: func(*render.ImportSet) {},
			},
			{
				name: "returns each file once in the order of the paths",
				give: func(set *render.ImportSet) {
					set.Add(typesProto)
					set.Add(commonProto)
					set.Add(typesProto)
				},
				want: "import \"" + commonProto + "\";\nimport \"" + typesProto + "\";\n",
			},
			{
				name: "returns the modifier of a file that a reference imports too",
				give: func(set *render.ImportSet) {
					set.Add(typesProto)
					set.AddNamed(typesProto, publicMod)
				},
				want: "import public \"" + typesProto + "\";\n",
			},
			{
				name: "returns a weak import with its modifier",
				give: func(set *render.ImportSet) { set.AddNamed(legacyProto, weakMod) },
				want: "import weak \"" + legacyProto + "\";\n",
			},
			{
				name: "returns a regular import of a file that a reference imports beside its option import",
				give: func(set *render.ImportSet) {
					set.AddNamed(rulesProto, optionMod)
					set.Add(rulesProto)
				},
				want: "import \"" + rulesProto + "\";\n",
			},
			{
				name: "returns a public import of a file that the header also imports for its options",
				give: func(set *render.ImportSet) {
					set.AddNamed(rulesProto, optionMod)
					set.AddNamed(rulesProto, publicMod)
				},
				want: "import public \"" + rulesProto + "\";\n",
			},
			{
				name: "returns the option imports after the other imports",
				give: func(set *render.ImportSet) {
					set.AddNamed(commonProto, optionMod)
					set.Add(typesProto)
					set.AddNamed(rulesProto, optionMod)
				},
				want: "import \"" + typesProto + "\";\nimport option \"" + commonProto + "\";\n" +
					"import option \"" + rulesProto + "\";\n",
			},
			{
				name: "returns a path with a quote escaped",
				give: func(set *render.ImportSet) { set.Add(`acme/"odd".proto`) },
				want: "import \"acme/\\\"odd\\\".proto\";\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var set render.ImportSet
				tt.give(&set)
				assert.Equal(t, backend.Imports(&set), tt.want, "the import block")
			})
		}
	})

	t.Run("TemplateImport", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
		}{
			{name: "reports false for an import without a modifier", give: ""},
			{name: "reports false for an import with a modifier", give: publicMod},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				local, item, ok := backend.TemplateImport(typesProto, tt.give)
				assert.False(t, ok, "an import of protobuf binds no name")
				assert.Empty(t, local, "the import has no local name")
				assert.Empty(t, item, "the import names no declaration")
			})
		}
	})
}

// Imports allocates the sorted entries and the block. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestImportsAllocs(t *testing.T) {
	set := filledImports()
	var got string
	assert.MaxAllocs(t, func() { got = backend.Imports(set) }, importsAllocs,
		"Imports allocates the entries and the block")
	assert.HasPrefix(t, got, "import \""+commonProto, "Imports writes the block")
}

// BenchmarkImports measures the import block that the skeleton of every
// file writes.
func BenchmarkImports(b *testing.B) {
	b.Run("Imports", func(b *testing.B) {
		set := filledImports()
		c := bench.Start(b).MaxAllocs(importsAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = backend.Imports(set)
		}
		assert.HasPrefix(b, got, "import \""+commonProto, "Imports writes the block")
	})
}

// filledImports returns a set with a plain, a public and an option import.
func filledImports() *render.ImportSet {
	set := &render.ImportSet{}
	set.Add(commonProto)
	set.AddNamed(typesProto, publicMod)
	set.AddNamed(rulesProto, optionMod)
	return set
}
