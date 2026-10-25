// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the import cases: the package of the file, the package of
// the referents, and an internal package of the tree of the referents.
const (
	webPath   = "example.com/shop/web"
	corePath  = "example.com/shop/core"
	clockPath = corePath + "/internal/clock"
)

// webFile is the package of the file that the import cases check.
var webFile = symbol.Identity{Lang: golang.Lang, Package: webPath, Name: "web", Kind: symbol.KindPackage}

// The layout refuses a declaration of a Go file with a reference that the
// file cannot refer to, so the rule reads the package of a reference as the
// backend imports it.
func TestImportable(t *testing.T) {
	t.Parallel()

	t.Run("Importable", func(t *testing.T) {
		t.Parallel()

		admitted := []struct {
			name string
			give *emit.TypeRef
		}{
			{name: "returns nil for a nil reference"},
			{name: "returns nil for a reference without a package", give: &emit.TypeRef{Spelling: "int"}},
			{
				name: "returns nil for an exported declaration of another package",
				give: &emit.TypeRef{Spelling: "core.Row", Package: corePath},
			},
			{
				name: "returns nil for an unexported declaration of the package of the file",
				give: &emit.TypeRef{
					Spelling: "rows", Target: symbol.Identity{Lang: golang.Lang, Package: webPath, Name: "rows"},
				},
			},
			{
				name: "returns nil for a type parameter",
				give: &emit.TypeRef{Spelling: "T", Target: symbol.Identity{
					Lang: golang.Lang, Package: corePath, Name: "T", Kind: symbol.KindTypeParam,
				}},
			},
		}
		for _, tt := range admitted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, spell.Importable(webFile, tt.give), "the file refers to the declaration")
			})
		}

		refused := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "returns an error for a target of an internal tree that does not contain the file",
				give: &emit.TypeRef{Spelling: "clock.Clock", Target: symbol.Identity{
					Lang: golang.Lang, Package: clockPath, Name: "Clock", Kind: symbol.KindInterface,
				}},
				want: "internal to " + corePath,
			},
			{
				name: "returns an error for an unexported target of another package",
				give: &emit.TypeRef{
					Spelling: "rows",
					Target:   symbol.Identity{Lang: golang.Lang, Package: corePath, Name: "rows"},
				},
				want: "rows is not exported",
			},
			{
				name: "returns an error for the qualified spelling of an unexported declaration of another package",
				give: &emit.TypeRef{Spelling: "core.rows", Package: corePath},
				want: "rows is not exported",
			},
			{
				name: "returns an error for an emitted reference into an external test package",
				give: &emit.TypeRef{
					Spelling: "Box", Target: symbol.Identity{Lang: "protobuf", Package: "shop", Name: "Box"},
					Emitted: true, Package: corePath + golang.TestSuffix,
				},
				want: "external test package",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := spell.Importable(webFile, tt.give)
				assert.HasError(t, err, "the file cannot refer to the declaration")
				assert.Contains(t, err.Error(), tt.want, "the error names the rule")
			})
		}
	})
}

// Importable allocates nothing for a reference that the file can refer to,
// because the layout calls it for each reference of a routed declaration.
// The ordinary run, which runs no benchmark, checks that ceiling here.
func TestImportableAllocs(t *testing.T) {
	ref := &emit.TypeRef{Spelling: "core.Row", Package: corePath}
	var err error
	assert.MaxAllocs(t, func() { err = spell.Importable(webFile, ref) }, 0,
		"Importable allocates nothing for a reference that the file can refer to")
	assert.NoError(t, err, "the file refers to the declaration")
}

// BenchmarkImportable measures the import rule that the layout runs for each
// reference of a routed declaration.
func BenchmarkImportable(b *testing.B) {
	b.Run("Importable", func(b *testing.B) {
		ref := &emit.TypeRef{Spelling: "core.Row", Package: corePath}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = spell.Importable(webFile, ref)
		}
		assert.NoError(b, err, "the file refers to the declaration")
	})
}
