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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "    body();\n" },
			"use":     func(string) string { return "" },
			"imports": func() string { return "IMPORTS\n" },
			"decls":   func() string { return "DECLS\n" },
			"slots":   func() string { return "" },
			"slot":    func(string) string { return "" },
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	assert.NoError(t, tmpl.Execute(&b, data), "the template executes")
	return b.String()
}

// Each kind template is pinned byte for byte, and so are the two
// absences that keep the inventory honest: no standalone method
// without an impl block, and no static without an initialiser.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("methods and variables have no module-level spelling", func(t *testing.T) {
		t.Parallel()

		kinds := backend.KindTemplates()
		_, held := kinds[symbol.KindMethod]
		assert.False(t, held,
			"methods group under an impl block per receiver, which one "+
				"declaration at a time cannot write")
		_, held = kinds[symbol.KindVariable]
		assert.False(t, held,
			"a static requires an initialiser the model does not carry")
	})

	t.Run("struct carries public fields", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("String"),
		})
		s.Fields.Append(&emit.Field{
			Name: "count", Type: ref("u32"), Comment: "rows per call",
		})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"/// Row is one record.\n"+
				"pub struct Row {\n"+
				"    /// Key addresses the row.\n"+
				"    pub key: String,\n"+
				"    pub count: u32, // rows per call\n"+
				"}\n",
			"fields public with trailing commas, docs at field depth, "+
				"a trailing comment behind its comma")
	})

	t.Run("trailing comments close every kind that ends a line", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store", Comment: "read side"}
		i.Methods.Append(&emit.Method{
			Name: "get", Comment: "by key",
			Params:  []*emit.Param{{Name: "key", Type: ref("String"), Comment: "the row key"}},
			Returns: []*emit.Return{{Type: ref("String"), Comment: "the row"}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"pub trait Store {\n"+
				"    fn get(&self, key: String /* the row key */) -> String /* the row */; // by key\n"+
				"} // read side\n",
			"a signature's comments spell as block comments, the method's and the trait's close their lines")
		assert.Equal(t,
			execute(t, backend.FunctionTemplate, &emit.Function{Name: "sort", Comment: "stable"}),
			"pub fn sort() {\n    body();\n} // stable\n", "a function's comment follows its closing brace")
		s := &emit.Sum{Name: "Shape", Comment: "tagged"}
		s.Variants.Append(&emit.SumVariant{Name: "Empty", Comment: "no payload"})
		assert.Equal(t, execute(t, backend.SumTemplate, s),
			"pub enum Shape {\n    Empty, // no payload\n} // tagged\n",
			"a variant's comment follows its comma and the enum's closes its brace")
	})

	t.Run("struct methods render in an impl block", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Cache"}
		s.TypeParams = []*emit.TypeParam{{Name: "T"}}
		s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
		s.Methods.Append(&emit.Method{Name: "get", Returns: []*emit.Return{{Type: ref("T")}}})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"pub struct Cache<T> {\n"+
				"    pub item: T,\n"+
				"}\n"+
				"\nimpl<T> Cache<T> {\n"+
				"    pub fn get(&self) -> T {\n"+
				"    body();\n"+
				"    }\n"+
				"}\n",
			"member methods follow in one impl block, the parameters "+
				"restated on the impl and its target")
	})

	t.Run("trait takes the receiver by reference", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Types.Append(&emit.Alias{
			Doc: []string{"Item is what the store yields."}, Name: "Item",
		})
		i.Methods.Append(
			&emit.Method{Name: "close"},
			&emit.Method{
				Name:    "load",
				Params:  []*emit.Param{{Name: "key", Type: ref("String")}},
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
		)
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"pub trait Store {\n"+
				"    /// Item is what the store yields.\n"+
				"    type Item;\n"+
				"    fn close(&self);\n"+
				"    fn load(&self, key: String) -> Row;\n"+
				"}\n",
			"the associated type first as a bare name, then self alone "+
				"where no parameter follows, joined where one does")
	})

	t.Run("function, alias and constant", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref("Row")}}}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"pub fn load() -> Row {\n    body();\n}\n", "the function shape")
		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "Id", Target: ref("String")}),
			"pub type Id = String;\n", "the alias shape")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "MAX", Type: ref("u32"), Value: "10"}),
			"pub const MAX: u32 = 10;\n", "a constant states its type")
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
		s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"pub struct Box<T> {\n    pub item: T,\n}\n",
			"the struct's parameter list behind its name")

		i := &emit.Interface{
			Name: "Keyed",
			TypeParams: []*emit.TypeParam{
				{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}},
			},
		}
		i.Methods.Append(&emit.Method{
			Name:    "pick",
			Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
			Returns: []*emit.Return{{Type: ref("K")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"pub trait Keyed<K: Codec> {\n    fn pick(&self, key: K) -> K;\n}\n",
			"the bound behind the colon, members referencing it")

		f := &emit.Function{
			Name:       "sort",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "items", Type: ref("T")}},
			Returns:    []*emit.Return{{Type: ref("T")}},
		}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"pub fn sort<T: Codec>(items: T) -> T {\n    body();\n}\n",
			"the function's parameter list behind its name")

		a := &emit.Alias{
			Name:       "Match",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Target:     &emit.TypeRef{Spelling: "Keyed", Args: []*emit.TypeRef{ref("T")}},
		}
		assert.Equal(t, execute(t, backend.AliasTemplate, a),
			"pub type Match<T: Codec> = Keyed<T>;\n",
			"the alias parameterizes and its target restates the argument")
	})

	t.Run("supertypes", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed"), ref("Ord")},
		}
		i.Methods.Append(&emit.Method{
			Name:    "get",
			Returns: []*emit.Return{{Type: ref("String")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"pub trait Store: Keyed + Ord {\n"+
				"    fn get(&self) -> String;\n"+
				"}\n",
			"the supertrait bounds behind the name")
	})

	t.Run("modifiers", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:        "Row",
			Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}},
		}
		s.Fields.Append(&emit.Field{
			Name: "key", Visibility: symbol.VisibilityInternal, Type: ref("String"),
		})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"#[derive(Debug)]\n"+
				"pub struct Row {\n"+
				"    pub(crate) key: String,\n"+
				"}\n",
			"attributes above, the field under its own scope")

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(
			&emit.Method{Name: "close", Async: true},
			&emit.Method{
				Name:  "make",
				Level: symbol.LevelType,
				Returns: []*emit.Return{
					{Type: ref("Row")},
				},
			},
			&emit.Method{
				Name:       "kind",
				HasDefault: true,
				Returns:    []*emit.Return{{Type: ref("u32")}},
				Body:       emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
			},
		)
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"pub trait Store {\n"+
				"    async fn close(&self);\n"+
				"    fn make() -> Row;\n"+
				"    fn kind(&self) -> u32 {\n    body();\n    }\n"+
				"}\n",
			"async before fn, an associated function without a receiver, "+
				"and a default body placed")

		f := &emit.Function{
			Name: "load", Async: true,
			Visibility: symbol.VisibilityInternal,
			Returns:    []*emit.Return{{Type: ref("Row")}},
		}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"pub(crate) async fn load() -> Row {\n    body();\n}\n",
			"the crate scope and async before fn")

		c := &emit.Constant{
			Name: "MAX", Visibility: symbol.VisibilityPackage,
			Type: ref("u32"), Value: "8",
		}
		assert.Equal(t, execute(t, backend.ConstantTemplate, c),
			"const MAX: u32 = 8;\n",
			"package scope spells no keyword, which is module-private")
	})

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		e := &emit.Enum{
			Doc:         []string{"Phase names a step."},
			Name:        "Phase",
			Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}},
		}
		e.Variants.Append(
			&emit.EnumVariant{Doc: []string{"Open admits writes."}, Name: "Open"},
			&emit.EnumVariant{Name: "Closed", Value: "9", Comment: "terminal"},
		)
		assert.Equal(t, execute(t, backend.EnumTemplate, e),
			"/// Phase names a step.\n"+
				"#[derive(Debug)]\n"+
				"pub enum Phase {\n"+
				"    /// Open admits writes.\n"+
				"    Open,\n"+
				"    Closed = 9, // terminal\n"+
				"}\n",
			"one variant per line, a stated value as its discriminant, "+
				"the trailing comment behind the comma")
	})

	t.Run("sum", func(t *testing.T) {
		t.Parallel()

		s := &emit.Sum{
			Doc:        []string{"Shape is one closed figure."},
			Name:       "Shape",
			TypeParams: []*emit.TypeParam{{Name: "T"}},
		}
		circle := &emit.SumVariant{
			Doc:  []string{"Circle bounds by a radius."},
			Name: "Circle",
		}
		circle.Fields.Append(&emit.Field{Name: "radius", Type: ref("f64")})
		write := &emit.SumVariant{Name: "Write"}
		write.Fields.Append(
			&emit.Field{Type: ref("String")},
			&emit.Field{Type: ref("T")},
		)
		s.Variants.Append(circle, write, &emit.SumVariant{Name: "Quit"})
		assert.Equal(t, execute(t, backend.SumTemplate, s),
			"/// Shape is one closed figure.\n"+
				"pub enum Shape<T> {\n"+
				"    /// Circle bounds by a radius.\n"+
				"    Circle { radius: f64 },\n"+
				"    Write(String, T),\n"+
				"    Quit,\n"+
				"}\n",
			"a struct variant in braces, a tuple variant in parentheses, "+
				"a unit variant bare")

		withMethods := &emit.Sum{Name: "Shape"}
		withMethods.Methods.Append(&emit.Method{Name: "area"})
		tmpl, err := template.New("kind").Funcs(backend.Funcs()).Parse(backend.SumTemplate)
		assert.NoError(t, err, "the template parses")
		var b strings.Builder
		assert.HasError(t, tmpl.Execute(&b, withMethods),
			"a sum carrying methods refuses: behaviour goes in impl blocks")
	})

	t.Run("the file skeleton is uses then declarations", func(t *testing.T) {
		t.Parallel()

		got := execute(t, backend.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store_stub.rs"})
		assert.Equal(t, got, "IMPORTS\nDECLS\n",
			"no module clause, because the file is the module")
	})
}
