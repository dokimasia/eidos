// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The messages the syntax cases expect: the one that counts the
// findings past the cap, and the marks of a quoted line cut short.
const (
	moreSyntax = "and more syntax errors"
	cutMark    = "..."
)

// A module's items lower into its package through one walk, so the
// module's header, its items, and what an item's visibility keeps, is
// pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("header", func(t *testing.T) {
		t.Parallel()

		t.Run("documents a module with its #![doc] attributes", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![doc = \"Crate.\"]\n")
			assert.Equal(t, packageIn(t, gb, crateName).Doc, []string{"Crate."}, "the attribute's text")
		})

		t.Run("attaches the carriers of a module's inner doc comments to its package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "//! +fixture:gen:table name=t\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "a module is a package")
		})

		t.Run("annotates a module's File node with its inner attributes", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![allow(dead_code)]\n")
			assert.Equal(t, fileIn(t, gb, crateName).Annotations,
				symbol.Annotations{{Name: "allow", Args: []string{"dead_code"}}}, "the inner attribute")
		})

		t.Run("stamps a module #![cfg(test)] marks rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![cfg(test)]\npub struct A;\n")
			assert.True(t, stamped(gb, packageIn(t, gb, crateName), string(rust.TestKey)), "the test module")
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
		})

		t.Run("documents an inline module with its inner doc comments", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod a {\n    //! The a.\n}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/a").Doc, []string{"The a."}, "the block's inner comment")
		})

		t.Run("documents a module with its outer doc comments before its inner ones", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: "/// Outer.\npub mod a;\n", "src/a.rs": "//! Inner.\n",
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, packageIn(t, gb, crateName+"/a").Doc, []string{"Outer.", "Inner."}, "rustdoc's order")
		})

		t.Run("leaves out an inline module whose inner cfg predicate is false", func(t *testing.T) {
			t.Parallel()

			src := "pub mod gated {\n    #![cfg(feature = \"y\")]\n    //! Gated.\n    pub struct A;\n}\n"
			gb, _ := parsedSource(t, src)
			assert.Empty(t, fileIn(t, gb, crateName+"/gated").Decls, "its items are kept out")
			assert.Empty(t, packageIn(t, gb, crateName+"/gated").Doc, "and so is its documentation")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"feature = \"y\""}},
				"and the file records the predicate")
		})
	})

	t.Run("items", func(t *testing.T) {
		t.Parallel()

		t.Run("passes over a shebang and an empty item", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#!/usr/bin/env rust-script\n;\npub struct A;\n")
			assert.Empty(t, found, "neither reports")
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
		})
	})

	t.Run("item", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing for a macro invocation", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "m!();\nmacro_rules! n { () => {} }\n")
			assert.Empty(t, found, "a macro without a carrier reports nothing")
			assert.Empty(t, fileIn(t, gb, crateName).Decls, "and declares nothing")
		})

		t.Run("stamps the file with each cfg predicate that keeps one of its items out", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(windows)]\npub struct A;\n#[cfg(feature = \"y\")]\npub struct B;\n")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"windows", "feature = \"y\""}},
				"the predicates in source order")
		})

		t.Run("leaves out the members below the file a path attribute names for a module a cfg predicate keeps "+
			"out", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, crateTree(map[string]string{
				libRoot:                "#[cfg(feature = \"y\")]\n#[path = \"other/g.rs\"]\nmod gated;\n",
				"src/other/g.rs":       "mod inner;\n",
				"src/other/g/inner.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile, frontend.ExcludedFile},
				"the module's file and the member below it")
		})

		t.Run("keeps an item in for a feature the load enables", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{libRoot: "#[cfg(feature = \"x\")]\npub struct A;\n"}),
				libRoot, plugin.DepthFull, &frontend.Options{Features: []string{"x"}})
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			assert.Empty(t, stampsOf(gb, string(rust.CfgKey)), "and records no predicate")
		})
	})

	t.Run("module", func(t *testing.T) {
		t.Parallel()

		t.Run("names a module's package after its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod store {}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/store").Name, "store", "the module's name")
		})

		t.Run("documents a module with the doc comments above its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "/// The store.\npub mod store {}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/store").Doc, []string{"The store."}, "the outer comment")
		})

		t.Run("attaches the carriers above a mod item to the module's package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, carrierPlain+"pub mod store {}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "on the module's package")
		})

		t.Run("stamps a module #[cfg(test)] marks rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(test)]\nmod tests {}\n")
			assert.True(t, stamped(gb, packageIn(t, gb, crateName+"/tests"), string(rust.TestKey)), "the test module")
		})

		t.Run("lowers an inline module into the package below its parent", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod a {\n    pub mod b {\n        pub struct C;\n    }\n}\n")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/a/b").Decls, "C")
		})

		t.Run("loads a private module's pub items at signature depth", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{libRoot: "mod inner {\n    pub struct A;\n    struct B;\n}\n"})
			gb, _ := parsedTree(t, tree, libRoot, plugin.DepthSignatures, nil)
			decls := fileIn(t, gb, crateName+"/inner").Decls
			assert.Length(t, decls, 1, "a pub use can publish A")
			named[*node.Struct](t, decls, "A")
		})
	})

	t.Run("moduleFile", func(t *testing.T) {
		t.Parallel()

		t.Run("walks <name>.rs before <name>/mod.rs", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod a;\n", "src/a.rs": publicStruct, "src/a/mod.rs": "pub struct B;\n",
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnlinkedFile}, "mod.rs loads as unlinked")
			assert.Equal(t, found[0].Pos.File, "src/a/mod.rs", "the file the walk did not take")
			assert.Length(t, packageIn(t, gb, crateName+"/a").Files, 2, "under the module both name")
		})

		t.Run("walks no file for a path attribute that names no member", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#[path = \"missing.rs\"]\npub mod x;\n")
			assert.Empty(t, found, "the missing file reports nothing")
			assert.Empty(t, packageIn(t, gb, crateName+"/x").Files, "and the module has no File node")
		})
	})

	t.Run("visibility", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    string
			want    symbol.Visibility
			spelled []any
		}{
			{name: "lowers pub as public", give: "pub", want: symbol.VisibilityPublic},
			{name: "lowers pub(crate) as internal", give: "pub(crate)", want: symbol.VisibilityInternal},
			{name: "lowers no modifier as private", give: "", want: symbol.VisibilityPrivate},
			{
				name: "lowers pub(super) as internal and stamps it", give: "pub(super)",
				want: symbol.VisibilityInternal, spelled: []any{"pub(super)"},
			},
			{
				name: "lowers pub(self) as internal and stamps it", give: "pub(self)",
				want: symbol.VisibilityInternal, spelled: []any{"pub(self)"},
			},
			{
				name: "lowers pub(in path) as internal and stamps it as written", give: "pub( in crate :: a )",
				want: symbol.VisibilityInternal, spelled: []any{"pub(in crate::a)"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, tt.give+" fn f() {}\n")
				fn := named[*node.Function](t, fileIn(t, gb, crateName).Decls, "f")
				assert.Equal(t, fn.Visibility, tt.want, "the visibility")
				assert.Equal(t, stampsOf(gb, string(rust.VisibilityKey)), tt.spelled, "and the restriction's spelling")
			})
		}
	})

	t.Run("kept", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps an item without pub at full depth", func(t *testing.T) {
			t.Parallel()

			named[*node.Function](t, declsOf(t, "fn f() {}\n"), "f")
		})

		t.Run("leaves out a pub(crate) item at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "pub(crate) fn f() {}\n"), "the crate alone can name it")
		})
	})

	t.Run("reportSyntax", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a MISSING node as missing", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A {\n    a: u8\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnparsedFile}, "the missing brace reports")
			assert.True(t, strings.HasSuffix(found[0].Msg, "is missing here"), "as missing")
		})

		t.Run("reports an ERROR node quoting its source", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A;\n%% bad\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.True(t, strings.Contains(found[0].Msg, "%%"), "quoting the source")
		})

		t.Run("quotes at most the first line of an ERROR node, cut at its cap", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "%% "+strings.Repeat("x", 40)+"\n")
			assert.NotEmpty(t, found, "the error reports")
			assert.True(t, strings.Contains(found[0].Msg, cutMark), "the quote is cut")
		})

		t.Run("caps a file's syntax findings and reports the rest once", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, strings.Repeat("%% x;\n", 12))
			assert.Length(t, found, 11, "ten findings and the remainder")
			assert.Equal(t, found[10].Msg, moreSyntax, "counted once")
		})
	})
}
