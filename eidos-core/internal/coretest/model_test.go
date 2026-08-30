// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// foreign is a symbol from outside the node model: it answers the
// shared vocabulary and no identity, which is what [coretest.Names]
// refuses.
type foreign struct{}

func (foreign) Kind() symbol.Kind      { return symbol.KindInvalid }
func (foreign) Position() position.Pos { return position.Pos{} }
func (foreign) Docs() []string         { return nil }

// A fixture every package's cases are written against is itself a
// contract: a case reading the wrong shape fails for the wrong
// reason.
func TestModel(t *testing.T) {
	t.Parallel()

	t.Run("Struct", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the identity the resolution step would assign", func(t *testing.T) {
			t.Parallel()

			got := coretest.Struct(coretest.StorePath, "Store")
			assert.Equal(t, got.Identity(), symbol.Identity{
				Lang:    coretest.Lang,
				Package: coretest.StorePath,
				Name:    "Store",
				Kind:    symbol.KindStruct,
			}, "the fixture carries the identity the resolution step would assign")
		})

		t.Run("separates two declarations in one package", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Store")
			other := coretest.Struct(coretest.StorePath, "Cache")
			assert.NotEqual(t, one.Identity(), other.Identity(),
				"two fixtures in one package stay two declarations")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("holds the declarations it was given", func(t *testing.T) {
			t.Parallel()

			store := coretest.Struct(coretest.StorePath, "Store")
			pkg := coretest.Package(coretest.StorePath, store)

			var reached bool
			for decl := range node.Declarations(pkg) {
				reached = reached || decl.Identity() == store.Identity()
			}
			assert.True(t, reached,
				"the traversal reaches the declaration the fixture was given")
		})

		t.Run("names itself and its file", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			assert.Equal(t, pkg.ID, coretest.PackageID(coretest.StorePath),
				"the fixture names itself")
			assert.Length(t, pkg.Files, 1, "holds one file")
			assert.Equal(t, pkg.Files[0].ID, coretest.FileID(coretest.StorePath),
				"which names itself too")
		})

		t.Run("separates two packages", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t,
				coretest.PackageID(coretest.StorePath), coretest.PackageID(coretest.CachePath),
				"the two fixture paths answer two identities")
		})
	})

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the packages and declarations it was asked for", func(t *testing.T) {
			t.Parallel()

			const packages, files, decls = 3, 2, 4
			got := coretest.Workspace(packages, files, decls)
			assert.Length(t, got, packages, "Workspace answers the packages asked for")

			held := 0
			for decl := range node.Declarations(got[0]) {
				if decl.Kind() == symbol.KindStruct {
					held++
				}
			}
			assert.Equal(t, held, files*decls,
				"and every package holds files times decls declarations")
		})

		t.Run("separates every package and declaration it built", func(t *testing.T) {
			t.Parallel()

			const packages, files, decls = 4, 2, 4
			seen := map[symbol.Identity]struct{}{}
			total := 0
			for _, pkg := range coretest.Workspace(packages, files, decls) {
				for decl := range node.Declarations(pkg) {
					seen[decl.Identity()] = struct{}{}
					total++
				}
			}
			assert.Length(t, seen, total,
				"every built declaration is distinct, or a graph would drop some")
		})

		t.Run("answers nothing for a workspace of no packages", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, coretest.Workspace(0, 2, 4),
				"a workspace of no packages holds none")
		})
	})

	t.Run("Names", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the declared names in the order it read them", func(t *testing.T) {
			t.Parallel()

			decls := []symbol.Symbol{
				coretest.Struct(coretest.StorePath, "Store"),
				coretest.Struct(coretest.StorePath, "Cache"),
			}
			assert.Equal(t, coretest.Names(t, decls), []string{"Store", "Cache"},
				"Names answers the declared names in the order it read them")
		})

		t.Run("answers nothing for a traversal that yielded nothing", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, coretest.Names(t, nil),
				"a traversal that yielded nothing names nothing")
		})
	})
}
