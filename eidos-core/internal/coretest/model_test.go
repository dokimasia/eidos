// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

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
			want := symbol.Identity{
				Lang:    coretest.Lang,
				Package: coretest.StorePath,
				Name:    "Store",
				Kind:    symbol.KindStruct,
			}
			if got.Identity() != want {
				t.Fatalf("Identity() = %v, want %v", got.Identity(), want)
			}
		})

		t.Run("separates two declarations in one package", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Store")
			other := coretest.Struct(coretest.StorePath, "Cache")
			if one.Identity() == other.Identity() {
				t.Fatal("two fixtures share one identity, so a graph would hold one")
			}
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
			if !reached {
				t.Fatal("the traversal did not reach the declaration the fixture was given")
			}
		})

		t.Run("names itself and its file", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			if pkg.ID != coretest.PackageID(coretest.StorePath) {
				t.Fatalf("ID = %v, want %v", pkg.ID, coretest.PackageID(coretest.StorePath))
			}
			if len(pkg.Files) != 1 {
				t.Fatalf("the fixture holds %d files, want 1", len(pkg.Files))
			}
			if got := pkg.Files[0].ID; got != coretest.FileID(coretest.StorePath) {
				t.Fatalf("file ID = %v, want %v", got, coretest.FileID(coretest.StorePath))
			}
		})

		t.Run("separates two packages", func(t *testing.T) {
			t.Parallel()

			if coretest.PackageID(coretest.StorePath) == coretest.PackageID(coretest.CachePath) {
				t.Fatal("the two fixture paths answer one identity")
			}
		})
	})

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the packages and declarations it was asked for", func(t *testing.T) {
			t.Parallel()

			const packages, files, decls = 3, 2, 4
			got := coretest.Workspace(packages, files, decls)
			if len(got) != packages {
				t.Fatalf("Workspace answered %d packages, want %d", len(got), packages)
			}

			held := 0
			for decl := range node.Declarations(got[0]) {
				if decl.Kind() == symbol.KindStruct {
					held++
				}
			}
			if held != files*decls {
				t.Fatalf("a package holds %d declarations, want %d", held, files*decls)
			}
		})

		t.Run("separates every package and declaration it built", func(t *testing.T) {
			t.Parallel()

			const packages, files, decls = 4, 2, 4
			seen := map[symbol.Identity]struct{}{}
			for _, pkg := range coretest.Workspace(packages, files, decls) {
				for decl := range node.Declarations(pkg) {
					if _, taken := seen[decl.Identity()]; taken {
						t.Fatalf("%v is built twice, so a graph would hold one", decl.Identity())
					}
					seen[decl.Identity()] = struct{}{}
				}
			}
		})

		t.Run("answers nothing for a workspace of no packages", func(t *testing.T) {
			t.Parallel()

			if got := coretest.Workspace(0, 2, 4); len(got) != 0 {
				t.Fatalf("Workspace with no packages answered %d, want none", len(got))
			}
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
			got := coretest.Names(t, decls)
			if want := []string{"Store", "Cache"}; !slices.Equal(got, want) {
				t.Fatalf("Names() = %v, want %v", got, want)
			}
		})

		t.Run("answers nothing for a traversal that yielded nothing", func(t *testing.T) {
			t.Parallel()

			if got := coretest.Names(t, nil); len(got) != 0 {
				t.Fatalf("Names(nil) = %v, want none", got)
			}
		})
	})
}
