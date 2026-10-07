// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each stubbed builtin writes a marker, so a template's own bytes are
// distinguishable from the pass's.
const (
	bodyStub    = "    body();\n"
	importsStub = "IMPORTS\n"
	declsStub   = "DECLS\n"
)

// templateMapAllocs is a map of kinds onto templates or reasons: the
// map and its one group.
const templateMapAllocs = 2

// Each kind template is pinned byte for byte. The kind map leaves out
// a standalone method, which renders inside an impl block, and a
// variable, which the backend refuses because Rust declares a static
// in its place.
func TestTemplates(t *testing.T) {
	t.Parallel()

	kind := func(k symbol.Kind) string { return backend.KindTemplates()[k] }

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no template for a method", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, backend.KindTemplates(), symbol.KindMethod,
				"methods group under an impl block per receiver")
		})

		t.Run("returns no template for a variable", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, backend.KindTemplates(), symbol.KindVariable, "a static's initializer is constant")
		})

		t.Run("writes a struct's fields public", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Doc: []string{"Row is one record."}, Name: rowName}
			s.Fields.Append(
				&emit.Field{Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("String")},
				&emit.Field{Name: "count", Type: ref("u32"), Comment: "rows per call"},
			)
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"/// Row is one record.\n"+
					"pub struct Row {\n"+
					"    /// Key addresses the row.\n"+
					"    pub key: String,\n"+
					"    pub count: u32, // rows per call\n"+
					"}\n",
				"trailing commas, a trailing comment behind its comma")
		})

		t.Run("uses the item a field's type names", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Holder"}
			s.Fields.Append(&emit.Field{Name: "row", Type: imported(storeModule, rowName)})
			out, set, err := run(t, kind(symbol.KindStruct), s)
			assert.NoError(t, err, "the template executes")
			assert.Contains(t, out, "    pub row: Row,\n", "the item's name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the item's use")
		})

		t.Run("writes a trait's comments at the ends of their lines", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Comment: "read side"}
			i.Types.Append(&emit.Alias{Name: "Item", Comment: "what get yields"})
			i.Methods.Append(&emit.Method{
				Name: "get", Comment: "by key",
				Params:  []*emit.Param{{Name: "key", Type: ref("String"), Comment: "the row key"}},
				Returns: []*emit.Return{{Type: ref("String"), Comment: "the row"}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"pub trait Store {\n"+
					"    type Item; // what get yields\n"+
					"    fn get(&self, key: String /* the row key */) -> String /* the row */; // by key\n"+
					"} // read side\n",
				"a signature's comments as block comments")
		})

		t.Run("writes a function's trailing comment after its closing brace", func(t *testing.T) {
			t.Parallel()

			sort := &emit.Function{Name: "sort", Comment: "stable"}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), sort),
				"pub fn sort() {\n"+bodyStub+"} // stable\n", "on the brace's line")
		})

		t.Run("writes a sum's trailing comments at the ends of their lines", func(t *testing.T) {
			t.Parallel()

			s := &emit.Sum{Name: "Shape", Comment: "tagged"}
			s.Variants.Append(&emit.SumVariant{Name: "Empty", Comment: "no payload"})
			assert.Equal(t, execute(t, kind(symbol.KindSum), s),
				"pub enum Shape {\n    Empty, // no payload\n} // tagged\n", "behind the comma and the brace")
		})

		t.Run("writes a struct's methods in an impl block after it", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Cache", TypeParams: []*emit.TypeParam{
				{Name: "T", Bounds: []*emit.TypeRef{ref("Clone")}, Default: ref("String")},
			}}
			s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
			s.Methods.Append(&emit.Method{Name: "get", Comment: "cloned", Returns: returning("T")})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"pub struct Cache<T: Clone = String> {\n"+
					"    pub item: T,\n"+
					"}\n"+
					"\nimpl<T: Clone> Cache<T> {\n"+
					"    pub fn get(&self) -> T {\n"+bodyStub+"    } // cloned\n"+
					"}\n",
				"the parameters restated on the impl without their defaults")
		})

		t.Run("writes a trait's associated type before its methods", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Types.Append(&emit.Alias{
				Doc: []string{"Item is what the store yields."}, Name: "Item",
				Annotations: symbol.Annotations{{Name: "doc", Args: []string{"hidden"}}},
			})
			i.Methods.Append(
				&emit.Method{Name: "close"},
				&emit.Method{
					Name:    "load",
					Params:  []*emit.Param{{Name: "key", Type: ref("String")}},
					Returns: returning(rowName),
				},
			)
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"pub trait Store {\n"+
					"    /// Item is what the store yields.\n"+
					"    #[doc(hidden)]\n"+
					"    type Item;\n"+
					"    fn close(&self);\n"+
					"    fn load(&self, key: String) -> Row;\n"+
					"}\n",
				"self alone where no parameter follows")
		})

		t.Run("writes a function around its body", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: "load", Returns: returning(rowName)}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"pub fn load() -> Row {\n"+bodyStub+"}\n", "the function shape")
		})

		t.Run("writes an alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, execute(t, kind(symbol.KindAlias), &emit.Alias{Name: "Id", Target: ref("String")}),
				"pub type Id = String;\n", "the alias shape")
		})

		t.Run("writes a constant with its type", func(t *testing.T) {
			t.Parallel()

			limit := &emit.Constant{Name: "MAX", Type: ref("u32"), Value: "10"}
			assert.Equal(t, execute(t, kind(symbol.KindConstant), limit),
				"pub const MAX: u32 = 10;\n", "rustc requires the type")
		})

		t.Run("writes a struct's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
			s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"pub struct Box<T> {\n    pub item: T,\n}\n", "the struct's list")
		})

		t.Run("writes a trait's parameters with their defaults behind its name", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Keyed", TypeParams: []*emit.TypeParam{
				{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}, Default: ref("String")},
			}}
			i.Methods.Append(&emit.Method{
				Name:    "pick",
				Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
				Returns: returning("K"),
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"pub trait Keyed<K: Codec = String> {\n    fn pick(&self, key: K) -> K;\n}\n",
				"the trait's list with its default")
		})

		t.Run("writes a function's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{
				Name: "sort", TypeParams: codecBound("T"),
				Params: []*emit.Param{{Name: "items", Type: ref("T")}}, Returns: returning("T"),
			}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"pub fn sort<T: Codec>(items: T) -> T {\n"+bodyStub+"}\n", "the function's list")
		})

		t.Run("writes an alias's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			a := &emit.Alias{Name: "Match", TypeParams: codecBound("T"), Target: generic("Keyed", ref("T"))}
			assert.Equal(t, execute(t, kind(symbol.KindAlias), a),
				"pub type Match<T: Codec> = Keyed<T>;\n", "the alias's list")
		})

		t.Run("returns an error for a function's parameter default", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: "sort", TypeParams: []*emit.TypeParam{{Name: "T", Default: ref("String")}}}
			assert.HasError(t, refused(t, kind(symbol.KindFunction), f),
				"Rust takes a default on a type definition alone")
		})

		t.Run("writes a trait's supertrait bounds behind its name", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Keyed"), ref("Ord")}}
			i.Methods.Append(&emit.Method{Name: "get", Returns: returning("String")})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"pub trait Store: Keyed + Ord {\n    fn get(&self) -> String;\n}\n", "the bounds joined")
		})

		t.Run("writes a struct's attributes above it", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}}}
			s.Fields.Append(&emit.Field{Name: "key", Visibility: symbol.VisibilityInternal, Type: ref("String")})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"#[derive(Debug)]\npub struct Row {\n    pub(crate) key: String,\n}\n", "the field under its own scope")
		})

		t.Run("writes a trait's method modifiers", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Methods.Append(
				&emit.Method{Name: "close", Async: true},
				&emit.Method{Name: "make", Level: symbol.LevelType, Returns: returning(rowName)},
				&emit.Method{
					Name: "kind", HasDefault: true, Returns: returning("u32"),
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
				},
			)
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"pub trait Store {\n"+
					"    async fn close(&self);\n"+
					"    fn make() -> Row;\n"+
					"    fn kind(&self) -> u32 {\n"+bodyStub+"    }\n"+
					"}\n",
				"async before fn, no receiver at type level, a default body placed")
		})

		t.Run("writes a function's keywords before fn", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{
				Name:       "load",
				Async:      true,
				Visibility: symbol.VisibilityInternal,
				Returns:    returning(rowName),
			}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"pub(crate) async fn load() -> Row {\n"+bodyStub+"}\n", "the crate scope then async")
		})

		t.Run("writes a package-scoped constant without a keyword", func(t *testing.T) {
			t.Parallel()

			c := &emit.Constant{Name: "MAX", Visibility: symbol.VisibilityPackage, Type: ref("u32"), Value: "8"}
			assert.Equal(t, execute(t, kind(symbol.KindConstant), c), "const MAX: u32 = 8;\n", "module-private")
		})

		t.Run("writes one enum variant per line", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{
				Doc: []string{"Phase names a step."}, Name: "Phase",
				Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}},
			}
			e.Variants.Append(
				&emit.EnumVariant{Doc: []string{"Open admits writes."}, Name: "Open"},
				&emit.EnumVariant{Name: "Closed", Value: "9", Comment: "terminal"},
			)
			assert.Equal(t, execute(t, kind(symbol.KindEnum), e),
				"/// Phase names a step.\n"+
					"#[derive(Debug)]\n"+
					"pub enum Phase {\n"+
					"    /// Open admits writes.\n"+
					"    Open,\n"+
					"    Closed = 9, // terminal\n"+
					"}\n",
				"a stated value as its discriminant")
		})

		t.Run("writes each sum variant in its payload's form", func(t *testing.T) {
			t.Parallel()

			s := &emit.Sum{
				Doc:        []string{"Shape is one closed figure."},
				Name:       "Shape",
				TypeParams: []*emit.TypeParam{{Name: "T"}},
			}
			circle := &emit.SumVariant{Doc: []string{"Circle bounds by a radius."}, Name: "Circle"}
			circle.Fields.Append(&emit.Field{Name: "radius", Type: ref("f64")})
			write := &emit.SumVariant{Name: "Write"}
			write.Fields.Append(&emit.Field{Type: ref("String")}, &emit.Field{Type: ref("T")})
			s.Variants.Append(circle, write, &emit.SumVariant{Name: "Quit"})
			assert.Equal(t, execute(t, kind(symbol.KindSum), s),
				"/// Shape is one closed figure.\n"+
					"pub enum Shape<T> {\n"+
					"    /// Circle bounds by a radius.\n"+
					"    Circle { radius: f64 },\n"+
					"    Write(String, T),\n"+
					"    Quit,\n"+
					"}\n",
				"braces, parentheses and a unit variant bare")
		})

		t.Run("returns an error for a sum with methods", func(t *testing.T) {
			t.Parallel()

			s := &emit.Sum{Name: "Shape"}
			s.Methods.Append(&emit.Method{Name: "area"})
			assert.HasError(t, refused(t, kind(symbol.KindSum), s), "behaviour goes in impl blocks")
		})
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the uses then the declarations", func(t *testing.T) {
			t.Parallel()

			got := execute(t, backend.FileTemplate, struct {
				Name string
				Pkg  symbol.Identity
			}{Name: "store_stub.rs"})
			assert.Equal(t, got, importsStub+declsStub, "no module clause, because the file is the module")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the variable kind alone", func(t *testing.T) {
			t.Parallel()

			refused := backend.RefusedKinds()
			assert.Length(t, refused, 1, "one kind Rust cannot spell at module level")
			assert.NotEqual(t, refused[symbol.KindVariable], "", "the variable, with its reason")
		})

		t.Run("returns no kind the templates spell", func(t *testing.T) {
			t.Parallel()

			kinds := backend.KindTemplates()
			for k := range backend.RefusedKinds() {
				assert.NotContains(t, kinds, k, "a kind is spelt or refused: "+k.String())
			}
		})
	})
}

// Each map allocates itself. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestTemplatesAllocs(t *testing.T) {
	checkAllocs(t, templatesCalls())
}

// BenchmarkTemplates measures the maps the backend reads once per
// build.
func BenchmarkTemplates(b *testing.B) {
	benchCalls(b, templatesCalls())
}

// templatesCalls returns a call of KindTemplates and of RefusedKinds.
func templatesCalls() []allocCall {
	var kinds map[symbol.Kind]string
	return []allocCall{
		{
			name: "KindTemplates", allocs: templateMapAllocs,
			call:  func() { kinds = backend.KindTemplates() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 7, "KindTemplates returns seven templates") },
		},
		{
			name: "RefusedKinds", allocs: templateMapAllocs,
			call:  func() { kinds = backend.RefusedKinds() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 1, "RefusedKinds returns the variable") },
		},
	}
}

// run runs one template over one declaration the way the render pass
// does, the builtins stubbed, and returns the text or the refusal
// beside the file's import set.
func run(t *testing.T, src string, data any) (string, *render.ImportSet, error) {
	t.Helper()

	set := &render.ImportSet{}
	tmpl, err := template.New("kind").
		Funcs(backend.Funcs(set)).
		Funcs(template.FuncMap{
			render.BuiltinBody:    func(any) string { return bodyStub },
			render.BuiltinUse:     func(string) string { return "" },
			render.BuiltinImports: func() string { return importsStub },
			render.BuiltinDecls:   func() string { return declsStub },
			render.BuiltinSlots:   func() string { return "" },
			render.BuiltinSlot:    func(string) string { return "" },
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	err = tmpl.Execute(&b, data)
	return b.String(), set, err
}

// execute runs one template over one declaration and asserts it
// executes.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	got, _, err := run(t, src, data)
	assert.NoError(t, err, "the template executes")
	return got
}

// refused runs one template over one declaration and returns the
// refusal.
func refused(t *testing.T, src string, data any) error {
	t.Helper()

	_, _, err := run(t, src, data)
	return err
}

// codecBound returns the type parameter list of one parameter
// bounded by Codec.
func codecBound(name string) []*emit.TypeParam {
	return []*emit.TypeParam{{Name: name, Bounds: []*emit.TypeRef{ref("Codec")}}}
}

// returning returns the return list of one result.
func returning(spelling string) []*emit.Return { return []*emit.Return{{Type: ref(spelling)}} }
