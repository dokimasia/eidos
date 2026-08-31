// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-rust/backend"
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
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"/// Row is one record.\n"+
				"pub struct Row {\n"+
				"    /// Key addresses the row.\n"+
				"    pub key: String,\n"+
				"}\n",
			"fields public with trailing commas, docs at field depth")
	})

	t.Run("trait takes the receiver by reference", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
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
				"    fn close(&self);\n"+
				"    fn load(&self, key: String) -> Row;\n"+
				"}\n",
			"self alone where no parameter follows, joined where one does")
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
			Annotations: emit.Annotations{{Name: "derive", Args: []string{"Debug"}}},
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
