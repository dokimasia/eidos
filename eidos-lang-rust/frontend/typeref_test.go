// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A type reference is what the resolution step resolves and what a
// backend spells, so its spelling, form, children and package are
// pinned.
func TestTypeRef(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a name as Named without a package", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "User")
			assert.Equal(t, ref.Form, symbol.FormNamed, "a name is Named")
			assert.Equal(t, ref.Spelling, "User", "under its spelling")
			assert.Equal(t, ref.Package, "", "a bare name no import binds needs none")
		})

		t.Run("lowers a path as Named with the module it names as its package", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "crate :: store :: Table")
			assert.Equal(t, ref.Spelling, "crate::store::Table", "spelled without the whitespace")
			assert.Equal(t, ref.Package, crateName+"/store", "crate names the crate root")
		})

		t.Run("records the module a use declaration binds a name from as its package", func(t *testing.T) {
			t.Parallel()

			ref := typeIn(t, "use std::collections::HashMap;\n", "HashMap<u8, u8>")
			assert.Equal(t, ref.Package, "std/collections", "the import's module")
		})

		t.Run("records the package through the item's own module after an inline module", func(t *testing.T) {
			t.Parallel()

			ref := typeIn(t, "pub mod m {}\nuse std::io::Read;\n", "Read")
			assert.Equal(t, ref.Package, "std/io", "the inline module's scope ends with its block")
		})

		t.Run("records no package for a path from Self", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeOf(t, "Self::Item").Package, "", "an associated item of a type")
		})

		t.Run("records no package for a qualified path", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "<T as Iterator>::Item")
			assert.Equal(t, ref.Spelling, "<T as Iterator>::Item", "spelled as written")
			assert.Equal(t, ref.Package, "", "an associated item of a type")
		})

		t.Run("lowers a generic type as Named with its bare name and its arguments", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "Map< String ,  u8 >")
			assert.Equal(t, ref.Spelling, "Map", "the bare name")
			assert.Equal(t, spellingsOf(ref.Args), []string{"String", "u8"}, "the arguments in order")
		})

		t.Run("lowers a lifetime, a const argument and an associated type binding as Named arguments",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spellingsOf(typeOf(t, "Foo<'a, 3, Item = u8>").Args), []string{"'a", "3", "Item=u8"},
					"each argument keeps its spelling")
			})

		t.Run("lowers a reference as a Borrow of its type", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "& 'a  mut  u8")
			assert.Equal(t, ref.Form, symbol.FormBorrow, "a reference borrows")
			assert.Equal(t, ref.Spelling, "&'a mut u8", "spelled with its lifetime and mutability")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8"}, "of its one child")
		})

		t.Run("lowers an array as an Array with its literal length", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "[u8; 16]")
			assert.Equal(t, ref.Form, symbol.FormArray, "a length makes an array")
			assert.Equal(t, ref.Length, 16, "the literal")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8"}, "of its element")
		})

		t.Run("lowers an array length with a type suffix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeOf(t, "[u8; 16usize]").Length, 16, "the suffix is not part of the value")
		})

		t.Run("lowers a hexadecimal array length", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeOf(t, "[u8; 0x10]").Length, 16, "the literal's base")
		})

		t.Run("lowers an array length that is an expression with no Length", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "[u8; N]")
			assert.Equal(t, ref.Form, symbol.FormArray, "still an array")
			assert.Equal(t, ref.Length, 0, "the spelling keeps the expression")
		})

		t.Run("lowers an array length Go does not read with no Length", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typeOf(t, "[u8; 10_]").Length, 0, "Rust admits a trailing underscore and Go does not")
		})

		t.Run("lowers a slice as a List of its element", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "Box<[u8]>").Args[0]
			assert.Equal(t, ref.Form, symbol.FormList, "a slice states no length")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8"}, "of its element")
		})

		t.Run("lowers a tuple as a Tuple of its members", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "( u8 ,  String )")
			assert.Equal(t, ref.Form, symbol.FormTuple, "a tuple")
			assert.Equal(t, ref.Spelling, "(u8,String)", "spelled without the whitespace")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8", "String"}, "of its members in order")
		})

		t.Run("lowers a function pointer as a Func of its parameters and then its return", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "fn(u8, u16) -> u32")
			assert.Equal(t, ref.Form, symbol.FormFunc, "a function pointer")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8", "u16", "u32"}, "the parameters, then the return")
			assert.Equal(t, ref.Split, 2, "the return begins after the parameters")
		})

		t.Run("lowers a function pointer's named parameters by their types", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(typeOf(t, "fn(a: u8) -> u32").Elems), []string{"u8", "u32"}, "the names drop")
		})

		t.Run("lowers a function pointer without a return", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "fn(u8)")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8"}, "the parameter alone")
			assert.Equal(t, ref.Split, 1, "and no return")
		})

		t.Run("lowers a variadic function pointer's parameters before its variadic", func(t *testing.T) {
			t.Parallel()

			ref := typeOf(t, "unsafe extern \"C\" fn(u8, ...)")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"u8"}, "the variadic states no type")
		})

		t.Run("lowers a function trait bound as Named", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "pub fn f<F: Fn(u8) -> u8>(f: F) {}\n"), "f")
			bound := fn.TypeParams[0].Bounds[0]
			assert.Equal(t, bound.Form, symbol.FormNamed, "a trait, not a function pointer")
			assert.Equal(t, bound.Spelling, "Fn(u8)->u8", "spelled as written")
		})

		t.Run("lowers a function trait in a trait object as a Named bound", func(t *testing.T) {
			t.Parallel()

			bound := typeOf(t, "Box<dyn Fn(u8) -> u8>").Args[0].Elems[0]
			assert.Equal(t, bound.Form, symbol.FormNamed, "a trait, not a function pointer")
			assert.Equal(t, bound.Spelling, "Fn(u8)->u8", "spelled as written")
		})

		intersections := []struct {
			name string
			give string
			want []string
		}{
			{
				name: "lowers dyn Trait as an Intersection of its one bound",
				give: "Box<dyn Send>", want: []string{"Send"},
			},
			{
				name: "lowers impl Trait as an Intersection of its one bound",
				give: "impl Into<u8>", want: []string{"Into"},
			},
			{
				name: "lowers dyn A + B as an Intersection of every trait bound in order",
				give: "Box<dyn Send + Sync>", want: []string{"Send", "Sync"},
			},
			{
				name: "lowers impl A + B as an Intersection of every trait bound in order",
				give: "impl Iterator<Item = u8> + Send", want: []string{"Iterator", "Send"},
			},
			{
				name: "leaves a lifetime out of a trait object's bounds",
				give: "Box<dyn Send + 'static>", want: []string{"Send"},
			},
			{
				name: "leaves a use bound out of an impl Trait type's bounds",
				give: "impl Copy + use<'a>", want: []string{"Copy"},
			},
			{
				name: "lowers a ?Sized bound as a Named bound",
				give: "impl ?Sized + Copy", want: []string{"?Sized", "Copy"},
			},
		}
		for _, tt := range intersections {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := typeOf(t, tt.give)
				if len(ref.Args) > 0 {
					ref = ref.Args[0]
				}
				assert.Equal(t, ref.Form, symbol.FormIntersection, "a value has every bound's type")
				assert.Equal(t, spellingsOf(ref.Elems), tt.want, "the bounds")
			})
		}

		spelled := []struct {
			name string
			give string
			want string
		}{
			{name: "spells a trait object with its dyn", give: "Box<dyn Send + 'static>", want: "dyn Send+'static"},
			{name: "spells an impl Trait type with its impl", give: "impl Into<u8>", want: "impl Into<u8>"},
		}
		for _, tt := range spelled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := typeOf(t, tt.give)
				if len(ref.Args) > 0 {
					ref = ref.Args[0]
				}
				assert.Equal(t, ref.Spelling, tt.want, "a backend restates the type from its spelling")
			})
		}

		tests := []struct {
			name string
			give string
		}{
			{name: "lowers a raw pointer as Named", give: "*const u8"},
			{name: "lowers the never type as Named", give: "!"},
			{name: "lowers the unit type as Named", give: "()"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := typeOf(t, tt.give)
				if len(ref.Args) > 0 {
					ref = ref.Args[0]
				}
				assert.Equal(t, ref.Form, symbol.FormNamed, "the projection folds the spelling into a shape")
				assert.Empty(t, ref.Elems, "and the frontend states no structure")
			})
		}
	})
}

// typeIn parses the fixture crate whose root states a prelude and then
// a struct H of one field of a type, and returns the field's type.
func typeIn(tb assert.TB, prelude, typ string) *node.TypeRef {
	tb.Helper()

	return structOf(tb, prelude+"pub struct H {\n    pub f: "+typ+",\n}\n", "H").Fields[0].Type
}

// typeOf returns the reference a field of a type lowers to.
func typeOf(tb assert.TB, typ string) *node.TypeRef {
	tb.Helper()

	return typeIn(tb, "", typ)
}

// spellingsOf returns the spellings of references, in order.
func spellingsOf(refs []*node.TypeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Spelling)
	}
	return out
}
