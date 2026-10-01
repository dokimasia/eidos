// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The modules the placement cases resolve against: a module at the
// tree's root, one nested below it, and one of another language.
const (
	// rootModule is declared at the tree's root.
	rootModule = "example.com/acme"
	// nestedModule is declared in nestedRoot.
	nestedModule = "example.com/acme/tools"
	nestedRoot   = "tools"
	// otherLang is a language no fixture package is in.
	otherLang symbol.Lang = "other"
)

// placed is a fixture target that names every file's package after
// its origin.
type placed struct {
	named
}

// PackageAt returns the origin's package.
func (placed) PackageAt(p plugin.Placement) (symbol.Identity, error) { return p.Origin, nil }

// placement returns a placement over the fixture's modules, innermost
// root first.
func placement() plugin.Placement {
	return plugin.Placement{Modules: []plugin.Module{
		{Lang: coretest.Lang, Path: nestedModule, Root: nestedRoot},
		{Lang: coretest.Lang, Path: rootModule, Root: "."},
	}}
}

// A package rule reads the modules and the residents the run
// derived, and the module arithmetic is the kernel's, so every target
// joins a directory onto its module the same way.
func TestPackager(t *testing.T) {
	t.Parallel()

	t.Run("Contains", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			root string
			dir  string
			want bool
		}{
			{name: "reports true for the module's root", root: nestedRoot, dir: nestedRoot, want: true},
			{name: "reports true for a directory below the root", root: nestedRoot, dir: "tools/gen", want: true},
			{
				name: "reports false for a sibling sharing the root's prefix",
				root: nestedRoot,
				dir:  "toolsmith",
				want: false,
			},
			{name: "reports false for a directory outside the root", root: nestedRoot, dir: "svc", want: false},
			{name: "reports true for every directory under the tree's root", root: ".", dir: "svc/store", want: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				m := plugin.Module{Root: tt.root}
				assert.Equal(t, m.Contains(tt.dir), tt.want, "containment")
			})
		}
	})

	t.Run("Rel", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			root string
			dir  string
			want string
		}{
			{name: "returns a dot for the module's root", root: nestedRoot, dir: nestedRoot, want: "."},
			{name: "returns the path below a nested root", root: nestedRoot, dir: "tools/gen/mocks", want: "gen/mocks"},
			{
				name: "returns the directory itself below the tree's root",
				root: ".",
				dir:  "svc/store",
				want: "svc/store",
			},
			{name: "returns a dot for the tree's root under the tree's root", root: ".", dir: ".", want: "."},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				m := plugin.Module{Root: tt.root}
				assert.Equal(t, m.Rel(tt.dir), tt.want, "the relative directory")
			})
		}
	})

	t.Run("ModuleOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the innermost module containing the directory", func(t *testing.T) {
			t.Parallel()

			m, held := placement().ModuleOf(coretest.Lang, "tools/gen")
			assert.True(t, held, "a module contains the directory")
			assert.Equal(t, m.Path, nestedModule, "the nested module is innermost")
		})

		t.Run("returns the enclosing module outside a nested root", func(t *testing.T) {
			t.Parallel()

			m, held := placement().ModuleOf(coretest.Lang, "svc")
			assert.True(t, held, "the root module contains the directory")
			assert.Equal(t, m.Path, rootModule, "the root module encloses it")
		})

		t.Run("reports false for a language no module is in", func(t *testing.T) {
			t.Parallel()

			_, held := placement().ModuleOf(otherLang, "svc")
			assert.False(t, held, "no module of the language contains it")
		})
	})

	t.Run("PackageAt", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package a target's rule names", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = placed{name: "packager"}
			pk, names := p.(plugin.Packager)
			assert.True(t, names, "the package surface asserts")
			origin := coretest.PackageID(coretest.StorePath)
			got, err := pk.PackageAt(plugin.Placement{Path: "svc/store_stub.go", Origin: origin})
			assert.NoError(t, err, "the fixture derives a package")
			assert.Equal(t, got, origin, "the fixture returns the origin")
		})

		t.Run("reports false asserting a plugin that names no package", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, names := p.(plugin.Packager)
			assert.False(t, names, "the package surface is opt-in")
		})
	})
}
