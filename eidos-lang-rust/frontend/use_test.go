// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// imported is one Import or Export as the cases compare it: the module
// it names, its bindings as name and alias pairs, its alias, and
// whether it is a glob.
type imported struct {
	path     string
	names    [][2]string
	alias    string
	wildcard bool
}

// importsIn parses the fixture crate whose library root is src and
// returns the crate root File node's imports.
func importsIn(tb testing.TB, src string) []imported {
	tb.Helper()

	imports := fileIn(tb, mustParse(tb, src), crateName).Imports
	out := make([]imported, 0, len(imports))
	for _, imp := range imports {
		names := bindingsOf(imp.Names)
		out = append(out, imported{path: imp.Path, names: names, alias: imp.Alias, wildcard: imp.Wildcard})
	}
	return out
}

// exportsIn parses the fixture crate whose library root is src and
// returns the crate root File node's exports.
func exportsIn(tb testing.TB, src string) []imported {
	tb.Helper()

	exports := fileIn(tb, mustParse(tb, src), crateName).Exports
	out := make([]imported, 0, len(exports))
	for _, ex := range exports {
		out = append(out, imported{path: ex.Path, names: bindingsOf(ex.Names), wildcard: ex.Wildcard})
	}
	return out
}

// A module's use and extern crate declarations are its import scope as
// written, so the File node's imports and exports are pinned.
func TestUse(t *testing.T) {
	t.Parallel()

	t.Run("record", func(t *testing.T) {
		t.Parallel()

		t.Run("records a use declaration as an Import of the module its path names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::collections::HashMap;\n"),
				[]imported{{path: "std/collections", names: [][2]string{{"HashMap", ""}}}}, "the module and the name")
		})

		t.Run("records the names of one module in one Import", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::{Read, Write};\n"),
				[]imported{{path: "std/io", names: [][2]string{{"Read", ""}, {"Write", ""}}}}, "one module, two names")
		})

		t.Run("records one Import for each module a use tree names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::{io::Read, fmt::{Debug, Display}};\n"), []imported{
				{path: "std/io", names: [][2]string{{"Read", ""}}},
				{path: "std/fmt", names: [][2]string{{"Debug", ""}, {"Display", ""}}},
			}, "the tree's modules in source order")
		})

		t.Run("records a renamed name with its alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::Result as IoResult;\n")[0].names,
				[][2]string{{"Result", "IoResult"}}, "the name and its local spelling")
		})

		t.Run("records an import as _ with the alias _", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::fmt::Write as _;\n")[0].names, [][2]string{{"Write", "_"}},
				"the trait in scope under no name")
		})

		t.Run("records self in a list as the list's module", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::{self, Read};\n"), []imported{
				{path: "std", names: [][2]string{{"io", ""}}},
				{path: "std/io", names: [][2]string{{"Read", ""}}},
			}, "self binds io")
		})

		t.Run("records self renamed in a list as the list's module under the alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::{self as stdio};\n"),
				[]imported{{path: "std", names: [][2]string{{"io", "stdio"}}}}, "self as stdio binds io")
		})

		t.Run("records a glob as a wildcard Import of its module", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::*;\n"), []imported{{path: "std/io", wildcard: true}},
				"every name of the module")
		})

		t.Run("records a use of one segment as an Import of the name at the root of every path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use serde;\n"), []imported{{names: [][2]string{{"serde", ""}}}},
				"the crate's name, from no module")
		})

		t.Run("records nothing for a glob of no path", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, importsIn(t, "use *;\n"), "the glob names no module")
		})

		t.Run("records a crate-relative path as the crate's package path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use crate::a::B;\n")[0].path, crateName+"/a", "crate names the root")
		})

		t.Run("records a path from a declared child module below the module", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "mod a {}\nuse a::B;\n")[0].path, crateName+"/a", "the mod item declares a")
		})

		t.Run("records a path from a name no mod item declares as an external crate's", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use a::B;\n")[0].path, "a", "a is a crate")
		})

		t.Run("records a path from a use binding as the module the binding names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use store::Table;\nuse crate::store;\n")[0].path, crateName+"/store",
				"the binding after the use declaration names the module")
		})

		t.Run("records an extern crate declaration as an Import of the crate", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "extern crate serde;\n"), []imported{{path: "serde"}}, "the crate")
		})

		t.Run("records a renamed extern crate with its alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "extern crate serde as sd;\n"), []imported{{path: "serde", alias: "sd"}},
				"the crate under its local name")
		})

		t.Run("records the imports in source order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, importsIn(t, "use std::io::Read;\nextern crate serde;\n"), []imported{
				{path: "std/io", names: [][2]string{{"Read", ""}}}, {path: "serde"},
			}, "the use declaration, then the extern crate")
		})

		t.Run("records a pub use declaration as an Export too", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, exportsIn(t, "pub use crate::a::{B, C as D};\npub use crate::e::*;\n"), []imported{
				{path: crateName + "/a", names: [][2]string{{"B", ""}, {"C", "D"}}},
				{path: crateName + "/e", wildcard: true},
			}, "the published names")
		})

		t.Run("records no Export for a use declaration of restricted visibility", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, exportsIn(t, "pub(crate) use crate::a::B;\nuse crate::a::C;\n"),
				"neither is in the crate's public surface")
		})

		t.Run("records nothing for a use declaration a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "#[cfg(feature = \"y\")]\nuse crate::a::B;\n", crateName)
			assert.Equal(t, resolved(scope, "B"), plugin.Candidates{{id(crateName, "B")}}, "B binds nothing")
			assert.Empty(t, importsIn(t, "#[cfg(feature = \"y\")]\nuse crate::a::B;\n"), "and imports nothing")
		})

		t.Run("records the bindings of a use declaration after an item for the item's types", func(t *testing.T) {
			t.Parallel()

			ref := structOf(t, "pub struct S {\n    pub f: Table,\n}\nuse crate::store::Table;\n", "S").Fields[0].Type
			assert.Equal(t, ref.Package, crateName+"/store", "the boundary is read before the items")
		})
	})
}

// mustParse parses the fixture crate whose library root is src and
// returns its builder.
func mustParse(tb testing.TB, src string) *plugin.GraphBuilder {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return gb
}

// bindingsOf returns bindings as name and alias pairs, in order, and nil
// for none.
func bindingsOf(bs []*node.Binding) [][2]string {
	if len(bs) == 0 {
		return nil
	}
	out := make([][2]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, [2]string{b.Name, b.Alias})
	}
	return out
}
