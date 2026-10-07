// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// traitImplItem is how a finding names an item of a trait impl, which
// the trait declares.
const traitImplItem = "an item of a trait impl"

// An impl block anywhere in a crate adds to a type the crate declares,
// so where each impl's methods, constants and trait fold, and what
// becomes of what has no home, is pinned.
func TestImpl(t *testing.T) {
	t.Parallel()

	t.Run("implItem", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an inherent impl's methods onto the struct it names", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A;\nimpl A {\n    pub fn new() -> Self { A }\n"+
				"    pub fn get(&self) -> u8 { 0 }\n}\n", "A")
			assert.Equal(t, methodNames(st.Methods), []string{"new", "get"}, "the methods in order")
			assert.Equal(t, st.Methods[0].Level, symbol.LevelType, "new has no receiver")
			assert.Equal(t, st.Methods[1].Visibility, symbol.VisibilityPublic, "a pub method is public")
		})

		t.Run("lowers an inherent impl's method without pub as private", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A;\nimpl A {\n    fn helper(&self) {}\n}\n", "A")
			assert.Equal(t, st.Methods[0].Visibility, symbol.VisibilityPrivate, "an impl item is private by default")
		})

		t.Run("leaves out an inherent impl's method without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, shallowDecls(t, "pub struct A;\nimpl A {\n    fn helper(&self) {}\n"+
				"    pub fn get(&self) {}\n}\n"), "A")
			assert.Equal(t, methodNames(st.Methods), []string{"get"}, "the published method")
		})

		t.Run("lowers an inherent impl's associated constant as a type-level immutable field", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A;\nimpl A {\n    pub const K: u8 = 1;\n}\n", "A")
			assert.Equal(t, fieldNames(st.Fields), []string{"K"}, "the constant joins the fields")
			assert.Equal(t, st.Fields[0].Level, symbol.LevelType, "at the type level")
			assert.Equal(t, st.Fields[0].Mutability, symbol.MutabilityImmutable, "immutable")
			assert.Equal(t, st.Fields[0].Value, "1", "with its value as written")
		})

		t.Run("stamps an associated constant's restricted visibility and test mark", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct A;\nimpl A {\n    pub(super) const K: u8 = 1;\n"+
				"    #[cfg(test)]\n    pub const T: u8 = 2;\n}\n")
			st := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			expect.Contains(t, stampKeys(gb, st.Fields[0]), rust.VisibilityKey, "the restriction")
			expect.Contains(t, stampKeys(gb, st.Fields[1]), rust.TestKey, "the test mark")
		})

		t.Run("attaches the carriers of an associated constant to its field", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct A;\nimpl A {\n    "+carrierPlain+"    pub const K: u8 = 1;\n}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onField := gb.Attachments()[0].Subject.(*node.Field)
			assert.True(t, onField, "on the field")
		})

		t.Run("leaves out an impl item a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			src := "pub struct A;\nimpl A {\n    #[cfg(feature = \"y\")]\n    pub fn gated(&self) {}\n}\n"
			assert.Empty(t, structOf(t, src, "A").Methods, "the method is kept out")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a macro in an impl", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A;\nimpl A {\n    "+carrierPlain+"    m!();\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a macro is not in the model")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a trait impl's item", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub struct A;\nimpl Clone for A {\n    "+carrierPlain+
				"    #[inline]\n    fn clone(&self) -> Self { A }\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"the trait declares the item, and the attribute between is no item")
			assert.Contains(t, found[0].Msg, traitImplItem, "the finding names why")
		})
	})

	t.Run("fold", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a trait impl's trait to the struct's Implements", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A;\nimpl Clone for A {\n    fn clone(&self) -> Self { A }\n}\n", "A")
			assert.Equal(t, spellingsOf(st.Implements), []string{"Clone"}, "the trait")
			assert.Empty(t, st.Methods, "and no method, because the trait declares them")
		})

		t.Run("adds no trait of an impl for a reference to the struct", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, structOf(t, "pub struct A;\nimpl Clone for &A {}\n", "A").Implements,
				"&A implements the trait, not A")
		})

		t.Run("folds an inherent impl's methods and constants onto an enum", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "pub enum E {\n    A,\n}\nimpl E {\n    pub const K: u8 = 1;\n"+
				"    pub fn f(&self) {}\n}\n"), "E")
			assert.Equal(t, methodNames(e.Methods), []string{"f"}, "the method")
			assert.Equal(t, fieldNames(e.Fields), []string{"K"}, "and the constant")
		})

		t.Run("folds an inherent impl's methods onto a data enum", func(t *testing.T) {
			t.Parallel()

			sum := named[*node.Sum](t, declsOf(t, "pub enum S {\n    A(u8),\n}\nimpl S {\n    pub fn f(&self) {}\n}\n"),
				"S")
			assert.Equal(t, methodNames(sum.Methods), []string{"f"}, "the method")
		})

		t.Run("reports UnmodeledItem for an associated constant of a data enum", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub enum S {\n    A(u8),\n}\nimpl S {\n    "+carrierPlain+
				"    pub const K: u8 = 1;\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnmodeledItem, frontend.UnaddressedCarrier},
				"the constant has no home, and neither has its carrier")
			assert.Equal(t, found[0].Severity, diag.SeverityInfo, "as information")
		})

		t.Run("folds an impl in another module onto the type its path names", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot: "pub mod a;\npub struct A;\n", "src/a.rs": "impl super::A {\n    pub fn m(&self) {}\n}\n",
			}), libRoot, plugin.DepthFull, nil)
			st := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			assert.Equal(t, methodNames(st.Methods), []string{"m"}, "the method folds across modules")
		})

		t.Run("folds an impl onto the type a use declaration binds", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot:    "pub mod a;\npub struct A;\n",
				"src/a.rs": "use crate::A;\nimpl A {\n    pub fn m(&self) {}\n}\n",
			}), libRoot, plugin.DepthFull, nil)
			st := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			assert.Equal(t, methodNames(st.Methods), []string{"m"}, "the binding names the type")
		})

		t.Run("folds an impl onto the first type of a name the crate declares", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{
				libRoot:    "pub mod a;\npub struct A;\npub struct A;\nimpl A {\n    pub fn m(&self) {}\n}\n",
				"src/a.rs": "",
			}), libRoot, plugin.DepthFull, nil)
			decls := fileIn(t, gb, crateName).Decls
			assert.Equal(t, methodNames(decls[0].(*node.Struct).Methods), []string{"m"}, "the first declaration")
			assert.Empty(t, decls[1].(*node.Struct).Methods, "and not the duplicate")
		})

		t.Run("declares the methods of an impl of a type the crate does not declare outside their type",
			func(t *testing.T) {
				t.Parallel()

				fn := named[*node.Method](t, declsOf(t, "impl Missing {\n    pub fn m(&self) {}\n}\n"), "m")
				assert.Equal(t, fn.Receives.Spelling, "Missing", "the type that receives the method")
			})

		t.Run("reports UnmodeledItem for an associated constant of a type the crate does not declare",
			func(t *testing.T) {
				t.Parallel()

				_, found := parsedSource(t, "impl Missing {\n    pub const K: u8 = 1;\n}\n")
				assert.Equal(t, codesOf(found), []diag.Code{frontend.UnmodeledItem}, "the constant has no home")
			})
	})
}

// methodNames returns the names of methods, in order.
func methodNames(methods []*node.Method) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Name)
	}
	return out
}
