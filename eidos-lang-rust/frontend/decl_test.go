// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each Rust item lowers to one model kind, so the shape of every kind's
// declaration is pinned.
func TestDecl(t *testing.T) {
	t.Parallel()

	t.Run("structItem", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a struct with its named fields", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    pub a: u8,\n    b: String,\n}\n", "A")
			assert.Equal(t, fieldNames(st.Fields), []string{"a", "b"}, "the fields in order")
			assert.Equal(t, st.Fields[0].Visibility, symbol.VisibilityPublic, "a pub field is public")
			assert.Equal(t, st.Fields[1].Visibility, symbol.VisibilityPrivate, "and a field without pub private")
		})

		t.Run("lowers a tuple struct's fields unnamed", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct T(pub u8, String);\n", "T")
			assert.Equal(t, fieldNames(st.Fields), []string{"", ""}, "a positional field has no name")
			assert.Equal(t, st.Fields[0].Visibility, symbol.VisibilityPublic, "its own visibility")
			assert.Equal(t, st.Fields[1].Visibility, symbol.VisibilityPrivate, "or none")
		})

		t.Run("lowers a unit struct without fields", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, structOf(t, "pub struct U;\n", "U").Fields, "a unit struct states none")
		})

		t.Run("lowers a union as a struct stamped rust.union", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub union U {\n    a: u32,\n    b: f32,\n}\n")
			u := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "U")
			assert.Length(t, u.Fields, 2, "its fields")
			assert.Contains(t, stampKeys(gb, u), rust.UnionKey, "and the union mark")
		})

		t.Run("stamps a struct's lifetime parameters, which the model has no parameter for", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct R<'a, 'b, T> {\n    pub r: &'a T,\n    pub s: &'b T,\n}\n")
			st := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "R")
			assert.Length(t, st.TypeParams, 1, "T is the one type parameter")
			assert.Equal(t, stampsOf(gb, string(rust.LifetimeParamsKey)), []any{[]string{"'a", "'b"}},
				"the lifetimes as written")
		})

		t.Run("leaves out a struct without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "struct A;\npub(crate) struct B;\n"), "neither is published")
		})
	})

	t.Run("field", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out a field a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct A {\n    #[cfg(feature = \"y\")]\n    pub a: u8,\n    pub b: u8,\n}\n")
			assert.Equal(t, fieldNames(named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A").Fields),
				[]string{"b"}, "a is kept out")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"feature = \"y\""}},
				"and the file records the predicate")
		})

		t.Run("leaves out a positional field a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct T(#[cfg(feature = \"y\")] pub u8, pub String);\n", "T")
			assert.Length(t, st.Fields, 1, "the second field is the one left")
			assert.Equal(t, st.Fields[0].Type.Spelling, "String", "of its own type")
		})

		t.Run("leaves out a field without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, shallowDecls(t, "pub struct A {\n    pub a: u8,\n    b: u8,\n}\n"), "A")
			assert.Equal(t, fieldNames(st.Fields), []string{"a"}, "b is private")
		})

		t.Run("stamps a field's restricted visibility as written", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub struct A {\n    pub(super) a: u8,\n}\n")
			st := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			assert.Equal(t, st.Fields[0].Visibility, symbol.VisibilityInternal, "restricted to a path")
			assert.Equal(t, stampsOf(gb, string(rust.VisibilityKey)), []any{"pub(super)"}, "which the stamp spells")
		})

		t.Run("annotates a field with its attributes", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "pub struct A {\n    #[serde(skip)]\n    pub a: u8,\n}\n", "A")
			assert.Equal(t, st.Fields[0].Annotations, symbol.Annotations{{Name: "serde", Args: []string{"skip"}}},
				"the attribute")
		})
	})

	t.Run("enumItem", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an enum whose variants state no payload as an Enum with each discriminant verbatim",
			func(t *testing.T) {
				t.Parallel()

				e := named[*node.Enum](t, declsOf(t, "pub enum E {\n    A,\n    B = 1 << 2,\n}\n"), "E")
				assert.Length(t, e.Variants, 2, "both variants")
				assert.Equal(t, e.Variants[0].Value, "", "an implicit discriminant")
				assert.Equal(t, e.Variants[1].Value, "1 << 2", "an explicit one as written")
			})

		t.Run("lowers an enum with a payload as a Sum", func(t *testing.T) {
			t.Parallel()

			sum := named[*node.Sum](t, declsOf(t, "pub enum S<T> {\n    V(T),\n    W { x: i32 },\n    U,\n}\n"), "S")
			assert.Length(t, sum.TypeParams, 1, "its type parameter")
			assert.Equal(t, fieldNames(sum.Variants[0].Fields), []string{""}, "a tuple variant's field is unnamed")
			assert.Equal(t, fieldNames(sum.Variants[1].Fields), []string{"x"}, "a struct variant's is named")
			assert.Empty(t, sum.Variants[2].Fields, "and a unit variant states none")
		})

		t.Run("makes a variant's fields as public as the enum", func(t *testing.T) {
			t.Parallel()

			sum := named[*node.Sum](t, declsOf(t, "pub enum S {\n    W { x: i32 },\n}\n"), "S")
			field := sum.Variants[0].Fields[0]
			assert.Equal(t, field.Visibility, symbol.VisibilityPublic, "a variant field states none")
		})

		t.Run("leaves out a variant a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "pub enum E {\n    #[cfg(feature = \"y\")]\n    A,\n    B,\n}\n"), "E")
			assert.Length(t, e.Variants, 1, "A is kept out")
			assert.Equal(t, e.Variants[0].Name, "B", "B is kept")
		})

		t.Run("decides the payload on the variants a cfg predicate keeps in", func(t *testing.T) {
			t.Parallel()

			named[*node.Enum](t, declsOf(t, "pub enum E {\n    #[cfg(feature = \"y\")]\n    A(u8),\n    B,\n}\n"), "E")
		})

		t.Run("documents a variant with its doc comments", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "pub enum E {\n    /// The a.\n    A,\n}\n"), "E")
			assert.Equal(t, e.Variants[0].Doc, []string{"The a."}, "the comment above the variant")
		})

		t.Run("attaches a variant's carriers to the variant", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub enum E {\n    "+carrierPlain+"    A(u8),\n}\n")
			assert.Length(t, gb.Attachments(), 1, "one directive")
			_, onVariant := gb.Attachments()[0].Subject.(*node.SumVariant)
			assert.True(t, onVariant, "on the variant")
		})

		t.Run("stamps an enum's lifetime parameters", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub enum S<'a> {\n    V(&'a u8),\n}\n")
			assert.Equal(t, stampsOf(gb, string(rust.LifetimeParamsKey)), []any{[]string{"'a"}}, "the lifetime")
		})

		t.Run("leaves out an enum without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "enum E {\n    A,\n}\n"), "the enum is private")
		})
	})

	t.Run("traitItem", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a trait as an Interface extending its supertraits", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, declsOf(t, "pub trait T: Clone + Send + 'static {}\n"), "T")
			assert.Equal(t, spellingsOf(it.Extends), []string{"Clone", "Send"}, "the lifetime bound is left out")
		})

		t.Run("lowers a method without a body as abstract", func(t *testing.T) {
			t.Parallel()

			m := traitOf(t, "    fn req(&self);\n").Methods[0]
			assert.True(t, m.Abstract, "the implementation supplies it")
			assert.False(t, m.HasDefault, "the trait states no body")
		})

		t.Run("lowers a method with a body as having a default", func(t *testing.T) {
			t.Parallel()

			m := traitOf(t, "    fn def(&self) -> u8 { 0 }\n").Methods[0]
			assert.True(t, m.HasDefault, "the trait's body")
			assert.False(t, m.Abstract, "an implementation may keep")
		})

		t.Run("lowers a method as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, traitOf(t, "    fn req(&self);\n").Methods[0].Visibility, symbol.VisibilityPublic,
				"a trait's items are as public as the trait")
		})

		t.Run("lowers an associated type as an Alias without a target", func(t *testing.T) {
			t.Parallel()

			alias := traitOf(t, "    type Item;\n").Types[0].(*node.Alias)
			assert.Equal(t, alias.Name, "Item", "the associated type")
			assert.Nil(t, alias.Target, "the implementation supplies the target")
		})

		t.Run("lowers an associated type's default as its target", func(t *testing.T) {
			t.Parallel()

			alias := traitOf(t, "    type Item = u8;\n").Types[0].(*node.Alias)
			assert.Equal(t, alias.Target.Spelling, "u8", "the default")
		})

		t.Run("lowers an associated constant as a Constant", func(t *testing.T) {
			t.Parallel()

			k := traitOf(t, "    const K: usize = 1;\n").Types[0].(*node.Constant)
			assert.Equal(t, k.Name, "K", "the constant")
			assert.Equal(t, k.Value, "1", "with its default as written")
		})

		t.Run("documents a trait item with its doc comments", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, traitOf(t, "    /// Requires.\n    fn req(&self);\n").Methods[0].Doc, []string{"Requires."},
				"the comment above the method")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a macro in a trait", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "pub trait T {\n    "+carrierPlain+"    m!();\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a macro is not in the model")
		})

		t.Run("leaves out a trait item a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			it := traitOf(t, "    #[cfg(feature = \"y\")]\n    fn gated(&self);\n    fn kept(&self);\n")
			assert.Length(t, it.Methods, 1, "the gated method is kept out")
			assert.Equal(t, it.Methods[0].Name, "kept", "the other loads")
		})

		t.Run("annotates a trait item with its attributes", func(t *testing.T) {
			t.Parallel()

			it := traitOf(t, "    #[must_use]\n    fn f(&self) -> u8;\n")
			assert.Equal(t, it.Methods[0].Annotations, symbol.Annotations{{Name: "must_use"}}, "the attribute")
		})

		t.Run("stamps a trait's lifetime parameters", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub trait T<'a> {}\n")
			assert.Equal(t, stampsOf(gb, string(rust.LifetimeParamsKey)), []any{[]string{"'a"}}, "the lifetime")
		})

		t.Run("keeps a public trait's items at signature depth", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, shallowDecls(t, "pub trait T {\n    fn f(&self);\n}\n"), "T")
			assert.Length(t, it.Methods, 1, "a trait item states no pub of its own")
		})

		t.Run("leaves out a trait without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "trait T {}\n"), "the trait is private")
		})
	})

	t.Run("function", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a function with its parameters and its result", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f(a: u8, b: &str) -> u32 { 0 }\n"), "f")
			assert.Equal(t, paramNames(fn.Params), []string{"a", "b"}, "the parameters in order")
			assert.Equal(t, fn.Params[1].Type.Form, symbol.FormBorrow, "each with its type")
			assert.Equal(t, fn.Returns[0].Type.Spelling, "u32", "and the one result")
		})

		t.Run("lowers a function without a result type with no result", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, named[*node.Function](t, declsOf(t, "pub fn f() {}\n"), "f").Returns, "the unit result")
		})

		t.Run("lowers an async function as async", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Function](t, declsOf(t, "pub async fn f() {}\n"), "f").Async, "the modifier")
		})

		t.Run("lowers a function other modifiers mark as not async", func(t *testing.T) {
			t.Parallel()

			assert.False(t, named[*node.Function](t, declsOf(t, "pub const unsafe fn f() {}\n"), "f").Async,
				"const and unsafe are no async")
		})

		t.Run("lowers a function an extern block declares", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "extern \"C\" {\n    pub fn ext(x: i32) -> i32;\n}\n"), "ext")
			assert.Equal(t, paramNames(fn.Params), []string{"x"}, "its signature")
		})

		t.Run("stamps a function's lifetime parameters", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub fn f<'a>(x: &'a u8) -> &'a u8 { x }\n")
			assert.Equal(t, stampsOf(gb, string(rust.LifetimeParamsKey)), []any{[]string{"'a"}}, "the lifetime")
		})

		t.Run("leaves out a function without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "fn f() {}\n"), "the function is private")
		})
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("names a mutable parameter", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f(mut a: u8) {}\n"), "f")
			assert.Equal(t, paramNames(fn.Params), []string{"a"}, "mut is a binding mode")
		})

		t.Run("leaves a parameter whose pattern binds no one name unnamed", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f((a, b): (u8, u8), _: u8) {}\n"), "f")
			assert.Equal(t, paramNames(fn.Params), []string{"", ""}, "each is positional")
		})

		t.Run("lowers a variadic parameter as positionally variadic", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "extern \"C\" {\n    pub fn printf(fmt: *const u8, ...);\n}\n"),
				"printf")
			assert.Length(t, fn.Params, 2, "the variadic is a parameter")
			assert.Equal(t, fn.Params[1].Variadic, symbol.VariadicPositional, "that collects positions")
			assert.Nil(t, fn.Params[1].Type, "and states no type")
		})

		t.Run("leaves out a parameter a cfg predicate keeps out", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f(#[cfg(feature = \"y\")] a: u8, b: u8) {}\n"), "f")
			assert.Equal(t, paramNames(fn.Params), []string{"b"}, "a is kept out")
		})

		t.Run("annotates a parameter with its attributes", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f(#[allow(unused)] a: u8) {}\n"), "f")
			assert.Equal(t, fn.Params[0].Annotations, symbol.Annotations{{Name: "allow", Args: []string{"unused"}}},
				"the attribute")
		})
	})

	t.Run("methodOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "spells the receiver of &self as &Self", give: "&self", want: "&Self"},
			{name: "spells the receiver of &mut self as &mut Self", give: "&mut self", want: "&mut Self"},
			{name: "spells the receiver of a self with a lifetime with it", give: "&'a self", want: "&'a Self"},
			{name: "spells the receiver of self as Self", give: "self", want: "Self"},
			{name: "spells the receiver of mut self as Self", give: "mut self", want: "Self"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				m := traitOf(t, "    fn f("+tt.give+");\n").Methods[0]
				assert.Equal(t, m.Receiver.Name, "self", "the receiver's name")
				assert.Equal(t, m.Receiver.Type.Spelling, tt.want, "the receiver's type")
				assert.Empty(t, m.Params, "the receiver is no parameter")
			})
		}

		t.Run("lowers a typed self parameter as the receiver of its type", func(t *testing.T) {
			t.Parallel()

			m := traitOf(t, "    fn f(self: Box<Self>);\n").Methods[0]
			assert.Equal(t, m.Receiver.Type.Spelling, "Box", "the type's bare name")
			assert.Equal(t, spellingsOf(m.Receiver.Type.Args), []string{"Self"}, "and its argument")
		})

		t.Run("lowers a method with a receiver at the instance level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, traitOf(t, "    fn f(&self);\n").Methods[0].Level, symbol.LevelInstance,
				"the receiver is an instance")
		})

		t.Run("lowers a method without self at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, traitOf(t, "    fn new() -> Self;\n").Methods[0].Level, symbol.LevelType,
				"an associated function")
		})
	})

	t.Run("typeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type parameter's bounds and default", func(t *testing.T) {
			t.Parallel()

			tp := structOf(t, "pub struct A<T: Clone + 'static = u8>(T);\n", "A").TypeParams[0]
			assert.Equal(t, spellingsOf(tp.Bounds), []string{"Clone"}, "the lifetime bound is left out")
			assert.Equal(t, tp.Default.Spelling, "u8", "the default")
		})

		t.Run("adds the bounds a where clause states", func(t *testing.T) {
			t.Parallel()

			tp := structOf(t, "pub struct A<T: Send>(T)\nwhere\n    T: Clone;\n", "A").TypeParams[0]
			assert.Equal(t, spellingsOf(tp.Bounds), []string{"Send", "Clone"}, "the inline bound, then the clause's")
		})

		t.Run("adds no bound a where clause states for another type", func(t *testing.T) {
			t.Parallel()

			tp := structOf(t, "pub struct A<T>(T)\nwhere\n    Vec<T>: Clone;\n", "A").TypeParams[0]
			assert.Empty(t, tp.Bounds, "the predicate bounds Vec<T>")
		})

		t.Run("lowers a const parameter with its type and default", func(t *testing.T) {
			t.Parallel()

			tp := structOf(t, "pub struct A<const N: usize = 3>;\n", "A").TypeParams[0]
			assert.True(t, tp.Const, "its argument is a value")
			assert.Equal(t, tp.Type.Spelling, "usize", "of its type")
			assert.Equal(t, tp.DefaultValue, "3", "and its default as written")
		})
	})

	t.Run("constant", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a const item as a Constant with its type and value verbatim", func(t *testing.T) {
			t.Parallel()

			k := named[*node.Constant](t, declsOf(t, "pub const LIMIT: u32 = 1 << 4;\n"), "LIMIT")
			assert.Equal(t, k.Type.Spelling, "u32", "its type")
			assert.Equal(t, k.Value, "1 << 4", "its value as written")
		})

		t.Run("declares nothing for an anonymous constant", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, declsOf(t, "const _: () = ();\n"), "no path names it")
		})

		t.Run("stamps a constant's restricted visibility as written", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub(in crate) const X: u8 = 1;\n")
			assert.Equal(t, stampsOf(gb, string(rust.VisibilityKey)), []any{"pub(in crate)"}, "the restriction")
		})

		t.Run("leaves out a constant without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "const X: u8 = 1;\n"), "the constant is private")
		})
	})

	t.Run("static", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a static as an immutable Variable", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "pub static NAME: &str = \"x\";\n"), "NAME")
			assert.Equal(t, v.Mutability, symbol.MutabilityImmutable, "a static is immutable")
			assert.Equal(t, v.Value, "\"x\"", "its value as written")
		})

		t.Run("lowers a static mut as a mutable Variable", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "pub static mut COUNT: u64 = 0;\n"), "COUNT")
			assert.Equal(t, v.Mutability, symbol.MutabilityMutable, "mut makes it mutable")
		})

		t.Run("leaves out a static without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "static X: u8 = 1;\n"), "the static is private")
		})
	})

	t.Run("typeAlias", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type item as an Alias of its type", func(t *testing.T) {
			t.Parallel()

			alias := named[*node.Alias](t, declsOf(t, "pub type Map<T> = HashMap<String, T>;\n"), "Map")
			assert.Length(t, alias.TypeParams, 1, "its type parameter")
			assert.Equal(t, alias.Target.Spelling, "HashMap", "the target's bare name")
			assert.Equal(t, spellingsOf(alias.Target.Args), []string{"String", "T"}, "and its arguments")
		})

		t.Run("stamps an alias's lifetime parameters", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "pub type R<'a> = &'a u8;\n")
			assert.Equal(t, stampsOf(gb, string(rust.LifetimeParamsKey)), []any{[]string{"'a"}}, "the lifetime")
		})

		t.Run("leaves out an alias without pub at signature depth", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, shallowDecls(t, "type A = u8;\n"), "the alias is private")
		})
	})
}

// shallowDecls parses the fixture crate whose library root is src at
// signature depth and returns the declarations of the crate root.
func shallowDecls(tb testing.TB, src string) node.Symbols {
	tb.Helper()

	gb, _ := parsedTree(tb, crateTree(map[string]string{libRoot: src}), libRoot, plugin.DepthSignatures, nil)
	return fileIn(tb, gb, crateName).Decls
}

// fieldNames returns the names of fields, in order.
func fieldNames(fields []*node.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

// paramNames returns the names of parameters, in order.
func paramNames(params []*node.Param) []string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		out = append(out, p.Name)
	}
	return out
}

// traitOf parses the fixture crate whose root declares a pub trait T of
// a body and returns the trait.
func traitOf(tb testing.TB, body string) *node.Interface {
	tb.Helper()

	return named[*node.Interface](tb, declsOf(tb, "pub trait T {\n"+body+"}\n"), "T")
}
