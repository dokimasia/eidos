// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-rust"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(rust.Funcs()).
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

		kinds := rust.KindTemplates()
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
		assert.Equal(t, execute(t, rust.StructTemplate, s),
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
		assert.Equal(t, execute(t, rust.InterfaceTemplate, i),
			"pub trait Store {\n"+
				"    fn close(&self);\n"+
				"    fn load(&self, key: String) -> Row;\n"+
				"}\n",
			"self alone where no parameter follows, joined where one does")
	})

	t.Run("function, alias and constant", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref("Row")}}}
		assert.Equal(t, execute(t, rust.FunctionTemplate, f),
			"pub fn load() -> Row {\n    body();\n}\n", "the function shape")
		assert.Equal(t,
			execute(t, rust.AliasTemplate, &emit.Alias{Name: "Id", Target: ref("String")}),
			"pub type Id = String;\n", "the alias shape")
		assert.Equal(t,
			execute(t, rust.ConstantTemplate,
				&emit.Constant{Name: "MAX", Type: ref("u32"), Value: "10"}),
			"pub const MAX: u32 = 10;\n", "a constant states its type")
	})

	t.Run("the file skeleton is uses then declarations", func(t *testing.T) {
		t.Parallel()

		got := execute(t, rust.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store_stub.rs"})
		assert.Equal(t, got, "IMPORTS\nDECLS\n",
			"no module clause, because the file is the module")
	})
}
