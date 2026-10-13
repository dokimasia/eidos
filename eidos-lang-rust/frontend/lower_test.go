// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The items of a module lower into its package through one walk. The cases
// pin the header of the module, its items, and what the visibility of an
// item keeps.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("header", func(t *testing.T) {
		t.Parallel()

		t.Run("documents a module with its #![doc] attributes", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![doc = \"Crate.\"]\n")
			assert.Equal(t, packageIn(t, gb, crateName).Doc, []string{"Crate."}, "the text of the attribute")
		})

		t.Run("attaches the carriers of the inner doc comments of a module to its package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "//! +fixture:gen:table name=t\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "a module is a package")
		})

		t.Run("annotates the File node of a module with its inner attributes", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![allow(dead_code)]\n")
			assert.Equal(t, fileIn(t, gb, crateName).Annotations,
				symbol.Annotations{{Name: "allow", Args: []string{"dead_code"}}}, "the inner attribute")
		})

		t.Run("stamps rust.test on a module that #![cfg(test)] marks", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#![cfg(test)]\npub struct A;\n")
			assert.Contains(t, stampKeys(gb, packageIn(t, gb, crateName)), rust.TestKey, "the test module")
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
		})

		t.Run("documents an inline module with its inner doc comments", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod a {\n    //! The a.\n}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/a").Doc, []string{"The a."}, "the inner comment of the block")
		})

		t.Run("documents a module with its outer doc comments before its inner ones", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: "/// Outer.\npub mod a;\n", "src/a.rs": "//! Inner.\n",
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, packageIn(t, gb, crateName+"/a").Doc, []string{"Outer.", "Inner."},
				"the order of rustdoc")
		})

		t.Run("leaves out an inline module whose inner cfg predicate is false", func(t *testing.T) {
			t.Parallel()

			src := "pub mod gated {\n    #![cfg(feature = \"y\")]\n    //! Gated.\n    pub struct A;\n}\n"
			gb, _ := parsedSource(t, src)
			assert.Empty(t, fileIn(t, gb, crateName+"/gated").Decls, "its items are kept out")
			assert.Empty(t, packageIn(t, gb, crateName+"/gated").Doc, "the module has no documentation")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"feature = \"y\""}},
				"the file records the predicate")
		})
	})

	t.Run("items", func(t *testing.T) {
		t.Parallel()

		t.Run("passes over the items that declare nothing", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#!/usr/bin/env rust-script\n;\npub struct A;\n")
			assert.Empty(t, found, "neither the shebang nor the empty item reports")
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
		})
	})

	t.Run("item", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing for a macro invocation", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "m!();\nmacro_rules! n { () => {} }\n")
			assert.Empty(t, found, "a macro without a carrier reports nothing")
			assert.Empty(t, fileIn(t, gb, crateName).Decls, "the macro declares nothing")
		})

		t.Run("stamps the file with each cfg predicate that keeps one of its items out", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(windows)]\npub struct A;\n#[cfg(feature = \"y\")]\npub struct B;\n")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"windows", "feature = \"y\""}},
				"the predicates in source order")
		})

		t.Run("leaves out the members below the file of the path attribute of a module that a cfg predicate "+
			"keeps out", func(t *testing.T) {
			t.Parallel()

			_, found := parsedTree(t, crateTree(map[string]string{
				libRoot:                "#[cfg(feature = \"y\")]\n#[path = \"other/g.rs\"]\nmod gated;\n",
				"src/other/g.rs":       "mod inner;\n",
				"src/other/g/inner.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.ExcludedFile, frontend.ExcludedFile},
				"the file of the module and the member below it")
		})

		t.Run("keeps an item in for a feature that the load enables", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{libRoot: "#[cfg(feature = \"x\")]\npub struct A;\n"}),
				libRoot, plugin.DepthFull, &frontend.Options{Features: []string{"x"}})
			named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			assert.Empty(t, stampsOf(gb, string(rust.CfgKey)), "the file records no predicate")
		})
	})

	t.Run("module", func(t *testing.T) {
		t.Parallel()

		t.Run("names the package of a module after its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod store {}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/store").Name, "store", "the name of the module")
		})

		t.Run("documents a module with the doc comments above its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "/// The store.\npub mod store {}\n")
			assert.Equal(t, packageIn(t, gb, crateName+"/store").Doc, []string{"The store."}, "the outer comment")
		})

		t.Run("attaches the carriers above a mod item to the package of the module", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, carrierPlain+"pub mod store {}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onPackage := gb.Attachments()[0].Subject.(*node.Package)
			assert.True(t, onPackage, "the subject is the package of the module")
		})

		t.Run("stamps rust.test on a module that #[cfg(test)] marks", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(test)]\nmod tests {}\n")
			assert.Contains(t, stampKeys(gb, packageIn(t, gb, crateName+"/tests")), rust.TestKey, "the test module")
		})

		t.Run("annotates the File node of an inline module with the attributes of its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[allow(dead_code)]\nmod inner {\n    #![deny(missing_docs)]\n}\n")
			assert.Equal(t, fileIn(t, gb, crateName+"/inner").Annotations, symbol.Annotations{
				{Name: "allow", Args: []string{"dead_code"}}, {Name: "deny", Args: []string{"missing_docs"}},
			}, "the outer attributes, then the inner ones")
		})

		t.Run("annotates the File node of a file module with the attributes of its mod item", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: "#[allow(dead_code)]\nmod a;\n", "src/a.rs": publicStruct,
			}), libRoot, plugin.DepthFull, nil)
			assert.Equal(t, fileIn(t, gb, crateName+"/a").Annotations,
				symbol.Annotations{{Name: "allow", Args: []string{"dead_code"}}}, "the outer attribute of the module")
		})

		t.Run("lowers an inline module into the package below its parent", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub mod a {\n    pub mod b {\n        pub struct C;\n    }\n}\n")
			named[*node.Struct](t, fileIn(t, gb, crateName+"/a/b").Decls, "C")
		})

		t.Run("loads the pub items of a private module at signature depth", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{libRoot: "mod inner {\n    pub struct A;\n    struct B;\n}\n"})
			gb, _ := parsedTree(t, tree, libRoot, plugin.DepthSignatures, nil)
			decls := fileIn(t, gb, crateName+"/inner").Decls
			assert.Length(t, decls, 1, "a pub use can publish A")
			named[*node.Struct](t, decls, "A")
		})

		t.Run("passes the sweep over an inline module that its inner cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedTree(t, fstest.MapFS{
				manifestPath: {Data: []byte(manifestSrc)},
				libRoot: {Data: []byte("mod gated {\n    #![cfg(feature = \"x\")]\n    " + carrierPlain +
					"    pub struct A;\n}\n")},
			}, libRoot, plugin.DepthFull, nil)
			assert.Empty(t, found, "the carrier leaves with the module")
			assert.Empty(t, gb.Attachments(), "the carrier attaches nowhere")
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
			assert.Equal(t, found[0].Pos.File, "src/a/mod.rs", "the finding is at the file that the walk did not take")
			assert.Length(t, packageIn(t, gb, crateName+"/a").Files, 2, "both files are in the package of the module")
		})

		t.Run("walks no file for a path attribute without a member", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#[path = \"missing.rs\"]\npub mod x;\n")
			assert.Empty(t, found, "the missing file reports nothing")
			assert.Empty(t, packageIn(t, gb, crateName+"/x").Files, "the module has no File node")
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
				name: "lowers pub(super) as internal with a stamp", give: "pub(super)",
				want: symbol.VisibilityInternal, spelled: []any{"pub(super)"},
			},
			{
				name: "lowers pub(self) as internal with a stamp", give: "pub(self)",
				want: symbol.VisibilityInternal, spelled: []any{"pub(self)"},
			},
			{
				name: "lowers pub(in path) as internal with a stamp as written", give: "pub( in crate :: a )",
				want: symbol.VisibilityInternal, spelled: []any{"pub(in crate::a)"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedSource(t, tt.give+" fn f() {}\n")
				fn := named[*node.Function](t, fileIn(t, gb, crateName).Decls, "f")
				assert.Equal(t, fn.Visibility, tt.want, "the visibility")
				assert.Equal(t, stampsOf(gb, string(rust.VisibilityKey)), tt.spelled, "the spelling of the restriction")
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

			assert.Empty(t, shallowDecls(t, "pub(crate) fn f() {}\n"), "only the crate can refer to it")
		})
	})
}
