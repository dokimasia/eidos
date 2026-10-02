// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling fixture: the packages a reference names, the names
// their imports bind, and the declarations the references name.
const (
	// storePkg is a package whose import binds storeName, and
	// legacyPkg a second package whose last element is storeName too.
	storePkg  = "example.com/store"
	legacyPkg = "example.com/legacy/store"
	storeName = "store"
	// storeName2 is the name legacyPkg binds beside storePkg.
	storeName2 = storeName + "2"
	// aliasName is the alias a source file imported storePkg under.
	aliasName = "st"
	// yamlPkg is a path whose last element is no Go identifier, and
	// yamlName the name it assumes.
	yamlPkg  = "gopkg.in/yaml.v3"
	yamlName = "yaml"
	// timePkg and contextPkg are standard library packages.
	timePkg    = "time"
	contextPkg = "context"
	// rowName names a declaration of storePkg.
	rowName = "Row"
	// qualifiedRow is rowName qualified by the name storePkg binds.
	qualifiedRow = storeName + "." + rowName
	// protoLang is a language other than Go that a target can be in.
	protoLang symbol.Lang = "protobuf"
)

// ref returns an unresolved reference spelled s.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// imported returns a reference to name in pkg, spelled through the
// qualifier the source file bound.
func imported(qualifier, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: qualifier + "." + name, Package: pkg}
}

// aliased is the reference to rowName a source file spelled through
// aliasName.
func aliased() *emit.TypeRef { return imported(aliasName, storePkg, rowName) }

// composite returns a structural reference over children.
func composite(form symbol.TypeForm, spelling string, children ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Form: form, Spelling: spelling, Elems: children}
}

// fnType returns a function type over its parameters then its
// results, the results beginning at split.
func fnType(spelling string, split int, children ...*emit.TypeRef) *emit.TypeRef {
	f := composite(symbol.FormFunc, spelling, children...)
	f.Split = split
	return f
}

// sizedArray returns an array of four aliased rows.
func sizedArray() *emit.TypeRef {
	array := composite(symbol.FormArray, "[4]st.Row", aliased())
	array.Length = 4
	return array
}

// speller returns a speller over a fresh set, and the set.
func speller() (backend.Speller, *render.ImportSet) {
	set := &render.ImportSet{}
	return backend.NewSpeller(set), set
}

// spelled spells one reference and asserts it spells.
func spelled(tb assert.TB, s backend.Speller, t *emit.TypeRef) string {
	tb.Helper()

	out, err := s.Spell(t)
	assert.NoError(tb, err, "the reference spells")
	return out
}

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte, and every import a
// spelling needs is pinned beside it.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a spell helper bound to the file's import set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spell, is := backend.Funcs(&set)[backend.FuncSpell].(func(*emit.TypeRef) (string, error))
			assert.True(t, is, "the helper spells a reference")
			_, err := spell(imported(timePkg, timePkg, "Duration"))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the import is the file's")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs(nil), "", "no comment")
		})

		t.Run("writes one line comment per line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "// One.\n// Two.\n", "at the top level")
		})

		t.Run("writes at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"Inner."}, "\t"), "\t// Inner.\n", "at the member's depth")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a reference that names no package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref(rowName)), rowName, "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no import")
		})

		t.Run("returns any for a missing reference", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, nil), backend.Anonymous, "the empty interface")
		})

		t.Run("returns any for a reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{}), backend.Anonymous, "the empty interface")
		})

		t.Run("writes an argument list in brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{
				Spelling: "Map",
				Args: []*emit.TypeRef{
					ref("string"),
					{Spelling: "List", Args: []*emit.TypeRef{ref("User")}},
				},
			}), "Map[string, List[User]]", "the arguments recurse")
		})

		t.Run("binds the import of the package a reference names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, imported(timePkg, timePkg, "Duration")), "time.Duration",
				"the qualifier the import binds")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timePkg, Name: timePkg}},
				"the import under the name it binds")
		})

		t.Run("qualifies through the name the package's path assumes", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, aliased()), qualifiedRow, "the source file's alias is replaced")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: storeName}},
				"the import binds the assumed name")
		})

		t.Run("qualifies a path whose last element is no identifier through its assumed name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, imported(yamlName, yamlPkg, "Node")), yamlName+".Node",
				"the major version is dropped")
		})

		t.Run("suffixes the qualifier of a second package that assumes the same name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storeName, storePkg, rowName))
			assert.Equal(t, spelled(t, s, imported(storeName, legacyPkg, rowName)), storeName2+"."+rowName,
				"the first package keeps the plain name")
		})

		t.Run("writes a reference to the file's own package bare", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(storePkg)
			assert.Equal(t, spelled(t, s, aliased()), rowName, "a package never qualifies its own names")
			assert.Equal(t, set.Len(), 0, "a package never imports itself")
		})

		t.Run("qualifies a Go target through its package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName,
				Target:   symbol.Identity{Lang: golang.Lang, Package: storePkg, Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, target), qualifiedRow, "a declaration of another package is qualified")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the package is imported")
		})

		t.Run("writes a target of another language as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: "svc.Row", Package: "svc/row.proto",
				Target: symbol.Identity{Lang: protoLang, Package: "svc", Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, target), "svc.Row", "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no import of another language's package")
		})

		t.Run("writes the arguments of a qualified instantiation", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			box := imported(aliasName, storePkg, "Box")
			box.Args = []*emit.TypeRef{ref("int")}
			assert.Equal(t, spelled(t, s, box), storeName+".Box[int]", "the qualifier, then the arguments")
		})

		t.Run("returns an error for a qualified reference that records no package", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(ref(aliasName + "." + rowName))
			assert.HasError(t, err, "the import cannot be named")
			assert.Contains(t, err.Error(), "no package", "naming what is missing")
		})

		t.Run("returns an error for a spelling that is no Go type name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(imported("google.protobuf", "google/protobuf/timestamp.proto", "Timestamp"))
			assert.HasError(t, err, "another language's qualified name is no Go reference")
			assert.Contains(t, err.Error(), "no Go type name", "naming the defect")
		})

		t.Run("keeps a composite's written spelling where every child spells as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			ctxRef := imported(contextPkg, contextPkg, "Context")
			fn := fnType("func(ctx context.Context) error", 1, ctxRef, ref("error"))
			assert.Equal(t, spelled(t, s, fn), "func(ctx context.Context) error", "the parameter name is kept")
			assert.Equal(t, set.Paths(), []string{contextPkg}, "the child's import is bound")
		})

		tests := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "restates a pointer whose element spells otherwise",
				give: composite(symbol.FormOptional, "*st.Row", aliased()),
				want: "*" + qualifiedRow,
			},
			{
				name: "restates a slice whose element spells otherwise",
				give: composite(symbol.FormList, "[]st.Row", aliased()),
				want: "[]" + qualifiedRow,
			},
			{
				name: "restates a variadic parameter whose element spells otherwise",
				give: composite(symbol.FormList, "...st.Row", aliased()),
				want: "..." + qualifiedRow,
			},
			{
				name: "restates a sized array whose element spells otherwise",
				give: sizedArray(),
				want: "[4]" + qualifiedRow,
			},
			{
				name: "restates a map whose value spells otherwise",
				give: composite(symbol.FormMap, "map[string]st.Row", ref("string"), aliased()),
				want: "map[string]" + qualifiedRow,
			},
			{
				name: "restates a receive-only channel whose element spells otherwise",
				give: composite(symbol.FormStream, "<-chan st.Row", aliased()),
				want: "<-chan " + qualifiedRow,
			},
			{
				name: "restates a send-only channel whose element spells otherwise",
				give: composite(symbol.FormStream, "chan <- st.Row", aliased()),
				want: "chan<- " + qualifiedRow,
			},
			{
				name: "restates a bidirectional channel whose element spells otherwise",
				give: composite(symbol.FormStream, "chan st.Row", aliased()),
				want: "chan " + qualifiedRow,
			},
			{
				name: "restates a function type of several results without its parameter names",
				give: fnType("func(r st.Row) (st.Row, error)", 1, aliased(), aliased(), ref("error")),
				want: "func(" + qualifiedRow + ") (" + qualifiedRow + ", error)",
			},
			{
				name: "restates a function type of one result bare",
				give: fnType("func() st.Row", 0, aliased()),
				want: "func() " + qualifiedRow,
			},
			{
				name: "restates a function type of no result without a result list",
				give: fnType("func(r st.Row)", 1, aliased()),
				want: "func(" + qualifiedRow + ")",
			},
			{
				name: "restates a composite inside a composite",
				give: composite(symbol.FormList, "[]*st.Row", composite(symbol.FormOptional, "*st.Row", aliased())),
				want: "[]*" + qualifiedRow,
			},
			{
				name: "restates a slice whose element's type argument spells otherwise",
				give: composite(symbol.FormList, "[]Box[st.Row]",
					&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{aliased()}}),
				want: "[]Box[" + qualifiedRow + "]",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				assert.Equal(t, spelled(t, s, tt.give), tt.want, "restated from the structure")
			})
		}

		t.Run("returns an error restating an array whose length is an expression", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormArray, "[N]st.Row", aliased()))
			assert.HasError(t, err, "the structure states no length")
			assert.Contains(t, err.Error(), "expression", "naming the length")
		})

		t.Run("returns an error restating a form Go does not write", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormTuple, "(st.Row, int)", aliased(), ref("int")))
			assert.HasError(t, err, "Go has no tuple type")
		})

		t.Run("returns an error restating a composite missing a child", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormMap, "map[st.Row]", aliased()))
			assert.HasError(t, err, "a map takes a key and a value")
		})

		t.Run("returns an error for an inline body that names another package", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormInline, "struct{ When time.Time }"))
			assert.HasError(t, err, "the body's import is not recorded")
			assert.Contains(t, err.Error(), "inline body", "naming the form")
		})

		t.Run("writes an inline body that names no other package as written", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, composite(symbol.FormInline, "struct{ N int }")), "struct{ N int }",
				"the body as written")
		})

		t.Run("writes an inline body that does not parse as written", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, composite(symbol.FormInline, "struct{")), "struct{",
				"the formatter reports the body")
		})

		t.Run("writes any for an argument that states no type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{nil}}), "Box[any]",
				"the empty interface in the argument's place")
		})

		t.Run("returns an error for an argument that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{ref("x.Y")}})
			assert.HasError(t, err, "the argument records no package")
		})

		t.Run("returns an error for a composite whose child does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormOptional, "*x.Y", ref("x.Y")))
			assert.HasError(t, err, "the child records no package")
		})
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		typeParams := func(t *testing.T, ps ...*emit.TypeParam) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.TypeParams(ps)
		}

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t)
			assert.NoError(t, err, "no parameters spell")
			assert.Equal(t, got, "", "as nothing")
		})

		t.Run("writes an unbounded parameter under any", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "T"})
			assert.NoError(t, err, "an unbounded parameter spells")
			assert.Equal(t, got, "[T any]", "under the empty interface")
		})

		t.Run("writes one bound alone", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "K", Bounds: []*emit.TypeRef{ref("comparable")}})
			assert.NoError(t, err, "a bounded parameter spells")
			assert.Equal(t, got, "[K comparable]", "the bound behind the name")
		})

		t.Run("folds several bounds into a constraint interface", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t,
				&emit.TypeParam{Name: "V", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}})
			assert.NoError(t, err, "a parameter of several bounds spells")
			assert.Equal(t, got, "[V interface{ Codec; Closer }]", "Go's intersection")
		})

		t.Run("separates parameters with commas", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "K"}, &emit.TypeParam{Name: "V"})
			assert.NoError(t, err, "the parameters spell")
			assert.Equal(t, got, "[K any, V any]", "in declaration order")
		})

		t.Run("binds the import a bound names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			got, err := s.TypeParams([]*emit.TypeParam{{
				Name: "T", Bounds: []*emit.TypeRef{imported("fmt", "fmt", "Stringer")},
			}})
			assert.NoError(t, err, "the bound spells")
			assert.Equal(t, got, "[T fmt.Stringer]", "qualified")
			assert.Equal(t, set.Paths(), []string{"fmt"}, "the bound's package is imported")
		})

		tests := []struct {
			name string
			give *emit.TypeParam
		}{
			{name: "returns an error for a variance", give: &emit.TypeParam{Name: "T", Variance: symbol.VarianceOut}},
			{
				name: "returns an error for a value parameter",
				give: &emit.TypeParam{Name: "N", Const: true, Type: ref("int")},
			},
			{name: "returns an error for a default", give: &emit.TypeParam{Name: "T", Default: ref("string")}},
			{
				name: "returns an error for a bound that does not spell",
				give: &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{aliased(), ref("x.Y")}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := typeParams(t, tt.give)
				assert.HasError(t, err, "Go's type parameters state none of it")
			})
		}
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		params := func(t *testing.T, ps ...*emit.Param) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Params(ps)
			assert.NoError(t, err, "the parameters spell")
			return got
		}

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t), "", "no list")
		})

		t.Run("writes each parameter's name before its type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t,
				&emit.Param{Name: "ctx", Type: imported(contextPkg, contextPkg, "Context")},
				&emit.Param{Name: "id", Type: ref("int")},
			), "ctx context.Context, id int", "comma-separated")
		})

		t.Run("writes a variadic parameter's type behind three dots", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{
				Name: "keys", Type: ref("string"), Variadic: symbol.VariadicPositional,
			}), "keys ...string", "the variadic marker")
		})

		t.Run("writes an unnamed parameter as its type alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Type: ref("int")}), "int", "no name")
		})

		t.Run("writes a trailing comment as a block comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Name: "id", Type: ref("int"), Comment: "the key"}),
				"id int /* the key */", "the comment form gofmt keeps inside a list")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params([]*emit.Param{{Name: "id", Type: ref("x.Y")}})
			assert.HasError(t, err, "the list is refused")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		results := func(t *testing.T, rs ...*emit.Return) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Results(rs)
			assert.NoError(t, err, "the results spell")
			return got
		}

		t.Run("returns nothing for no results", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t), "", "no list")
		})

		t.Run("writes one unnamed result bare", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref("error")}), " error", "without parentheses")
		})

		t.Run("parenthesises several results", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref(rowName)}, &emit.Return{Type: ref("error")}),
				" (Row, error)", "comma-separated in parentheses")
		})

		t.Run("parenthesises one named result", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Name: "n", Type: ref("int")}), " (n int)",
				"because Go requires it")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results([]*emit.Return{{Type: ref("x.Y")}})
			assert.HasError(t, err, "the list is refused")
		})
	})

	t.Run("Receiver", func(t *testing.T) {
		t.Parallel()

		receiver := func(t *testing.T, m *emit.Method) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Receiver(m)
			assert.NoError(t, err, "the receiver spells")
			return got
		}

		t.Run("writes a declared receiver's name and type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{Receiver: &emit.Param{Name: "s", Type: ref("*Store")}}),
				"s *Store", "as a parameter")
		})

		t.Run("writes the type alone for a method attached from outside", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{Receives: ref("*Store")}), "*Store", "no name")
		})

		t.Run("returns nothing for a method without a receiver", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{}), "", "no spelling")
		})
	})

	t.Run("VarType", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a stated type behind a space", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.VarType(&emit.Variable{Name: "count", Type: ref("int")})
			assert.NoError(t, err, "a typed variable spells")
			assert.Equal(t, got, " int", "its type behind the space")
		})

		t.Run("returns nothing for a variable with an initializer alone", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.VarType(&emit.Variable{Name: "count", Value: "8"})
			assert.NoError(t, err, "an initialized variable spells")
			assert.Equal(t, got, "", "Go infers the type")
		})

		t.Run("returns an error for a variable stating neither", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.VarType(&emit.Variable{Name: "count"})
			assert.HasError(t, err, "var x alone declares nothing Go accepts")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.VarType(&emit.Variable{Name: "count", Type: ref("x.Y")})
			assert.HasError(t, err, "the slot is refused")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity's own name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Package(symbol.Identity{Name: storeName, Package: yamlPkg})
			assert.NoError(t, err, "a named package spells")
			assert.Equal(t, got, storeName, "the declared name")
		})

		t.Run("returns the name the path assumes for an identity without a name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Package(symbol.Identity{Package: yamlPkg})
			assert.NoError(t, err, "a package path spells")
			assert.Equal(t, got, yamlName, "the major version is dropped")
		})

		t.Run("returns an error for an identity naming no package", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Package(symbol.Identity{})
			assert.HasError(t, err, "every Go file declares a package")
			assert.Contains(t, err.Error(), "import base", "the error names what the plan can state")
		})
	})

	t.Run("Guard", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    symbol.Symbol
			wantErr bool
		}{
			{name: "writes nothing for a final struct", give: &emit.Struct{Name: rowName, Final: true}},
			{
				name: "writes nothing for a package-scoped variable",
				give: &emit.Variable{Name: "count", Visibility: symbol.VisibilityPackage},
			},
			{
				name: "writes nothing for a method attached to a type",
				give: &emit.Method{Name: "Close", Receives: ref(rowName)},
			},
			{
				name: "writes nothing for an annotated constant",
				give: &emit.Constant{Name: "Max", Value: "8", Annotations: symbol.Annotations{{Name: "nolint"}}},
			},
			{
				name: "writes nothing for a struct stating supertypes",
				give: &emit.Struct{
					Name:       rowName,
					Extends:    []*emit.TypeRef{ref("Base")},
					Implements: []*emit.TypeRef{ref("Keyed")},
				},
			},
			{
				name:    "returns an error for an abstract struct",
				give:    &emit.Struct{Name: rowName, Abstract: true},
				wantErr: true,
			},
			{
				name:    "returns an error for an async function",
				give:    &emit.Function{Name: "Load", Async: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a static method",
				give:    &emit.Method{Name: "Load", Level: symbol.LevelType},
				wantErr: true,
			},
			{
				name:    "returns an error for an override marker",
				give:    &emit.Method{Name: "Load", Override: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a method naming no receiver type",
				give:    &emit.Method{Name: "Close"},
				wantErr: true,
			},
			{
				name:    "returns an error for a field initializer",
				give:    &emit.Field{Name: "Key", Value: "1"},
				wantErr: true,
			},
			{
				name:    "returns an error for a field's own mutability",
				give:    &emit.Field{Name: "Key", Mutability: symbol.MutabilityImmutable},
				wantErr: true,
			},
			{
				name:    "returns an error for an immutable variable",
				give:    &emit.Variable{Name: "count", Mutability: symbol.MutabilityImmutable},
				wantErr: true,
			},
			{
				name:    "returns an error for a constant without a value",
				give:    &emit.Constant{Name: "Max"},
				wantErr: true,
			},
			{
				name:    "returns an error for a visibility no case expresses",
				give:    &emit.Alias{Name: "ID", Visibility: symbol.VisibilityProtected},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Guard(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "Go states it nowhere")
					return
				}
				assert.NoError(t, err, "the declaration passes")
				assert.Equal(t, got, "", "the guard writes nothing")
			})
		}
	})

	t.Run("EmbedGuard", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing for an embed with a comment", func(t *testing.T) {
			t.Parallel()

			got, err := backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Comment: "stream side"})
			assert.NoError(t, err, "the embed passes")
			assert.Equal(t, got, "", "the guard writes nothing")
		})

		t.Run("returns an error for a tagged embed", func(t *testing.T) {
			t.Parallel()

			_, err := backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Tag: `json:"r"`})
			assert.HasError(t, err, "Go gives a struct field alone a tag")
		})
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Directives(nil), "", "no lines")
		})

		t.Run("writes one directive comment per annotation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Directives(symbol.Annotations{
				{Name: "go:embed", Args: []string{"schema.sql"}},
				{Name: "nolint", Args: []string{"errcheck"}},
			}), "//go:embed schema.sql\n//nolint errcheck\n", "the arguments space-joined behind the name")
		})

		t.Run("writes at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Directives(symbol.Annotations{{Name: "go:fix"}}, "\t"), "\t//go:fix\n",
				"at the member's depth")
		})
	})

	t.Run("SigGuard", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    *emit.Method
			wantErr bool
		}{
			{name: "writes nothing for an abstract signature", give: &emit.Method{Name: "Get", Abstract: true}},
			{name: "writes nothing for a final signature", give: &emit.Method{Name: "Get", Final: true}},
			{
				name:    "returns an error for a default body",
				give:    &emit.Method{Name: "Get", HasDefault: true},
				wantErr: true,
			},
			{
				name:    "returns an error for type parameters",
				give:    &emit.Method{Name: "Map", TypeParams: []*emit.TypeParam{{Name: "T"}}},
				wantErr: true,
			},
			{
				name:    "returns an error for an async signature",
				give:    &emit.Method{Name: "Get", Async: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a body",
				give:    &emit.Method{Name: "Get", Body: emit.Body{Verbatim: "return nil"}},
				wantErr: true,
			},
			{
				name:    "returns an error for an override marker",
				give:    &emit.Method{Name: "Get", Override: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a static method",
				give:    &emit.Method{Name: "Get", Level: symbol.LevelType},
				wantErr: true,
			},
			{
				name:    "returns an error for annotations",
				give:    &emit.Method{Name: "Get", Annotations: symbol.Annotations{{Name: "nolint"}}},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.SigGuard(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "an interface method states it nowhere")
					return
				}
				assert.NoError(t, err, "the signature passes")
				assert.Equal(t, got, "", "the guard writes nothing")
			})
		}
	})
}
