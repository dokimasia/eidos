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
	"go.dokimi.dev/eidos/lang-go/backend"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker, so
// the twin holds the template's own bytes and nothing else's.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "\tbody()\n" },
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

// Each kind template is pinned byte for byte over a declaration
// exercising its whole shape, member docblocks included.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("every declared kind carries a template", func(t *testing.T) {
		t.Parallel()

		kinds := backend.KindTemplates()
		for _, k := range []symbol.Kind{
			symbol.KindStruct, symbol.KindInterface, symbol.KindFunction,
			symbol.KindMethod, symbol.KindAlias, symbol.KindConstant,
			symbol.KindVariable,
		} {
			_, held := kinds[k]
			assert.True(t, held, "the inventory spells "+k.String())
		}
	})

	t.Run("struct", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(
			&emit.Field{Doc: []string{"Key addresses the row."}, Name: "Key", Type: ref("string")},
			&emit.Field{
				Name: "N", Type: ref("int"),
				Tag: `json:"n"`, Comment: "counted",
			},
		)
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"// Row is one record.\n"+
				"type Row struct {\n"+
				"\t// Key addresses the row.\n"+
				"\tKey string\n"+
				"\tN int `json:\"n\"` // counted\n"+
				"}\n",
			"fields under their own docblocks, tag and trailing comment beside")
	})

	t.Run("interface", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Doc: []string{"Store loads rows."}, Name: "Store"}
		i.Methods.Append(&emit.Method{
			Doc:     []string{"Load fetches one row."},
			Name:    "Load",
			Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
			Returns: []*emit.Return{{Type: ref("Row")}, {Type: ref("error")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"// Store loads rows.\n"+
				"type Store interface {\n"+
				"\t// Load fetches one row.\n"+
				"\tLoad(key string) (Row, error)\n"+
				"}\n",
			"methods under their own docblocks, at member depth")
	})

	t.Run("function and method place the body", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "Load", Returns: []*emit.Return{{Type: ref("error")}}}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"func Load() error {\n\tbody()\n}\n", "the function's shape")

		m := &emit.Method{
			Name:     "Close",
			Receiver: &emit.Param{Name: "s", Type: ref("*Store")},
		}
		assert.Equal(t, execute(t, backend.MethodTemplate, m),
			"func (s *Store) Close() {\n\tbody()\n}\n", "the method's shape")
	})

	t.Run("alias, constant and variable", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string")}),
			"type ID = string\n", "the alias shape")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "Max", Type: ref("int"), Value: "10"}),
			"const Max int = 10\n", "a typed constant")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate, &emit.Constant{Name: "Max", Value: "10"}),
			"const Max = 10\n", "an untyped one")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "Max", Value: "10", Comment: "inclusive"}),
			"const Max = 10 // inclusive\n", "the trailing comment beside the value")
		assert.Equal(t,
			execute(t, backend.VariableTemplate,
				&emit.Variable{Name: "count", Type: ref("int")}),
			"var count int\n", "the variable shape")
	})

	t.Run("the file skeleton", func(t *testing.T) {
		t.Parallel()

		got := execute(t, backend.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store_stub.go", Pkg: symbol.Identity{Package: "svc/api"}})
		assert.Equal(t, got, "package api\n\nIMPORTS\n\nDECLS\n",
			"package clause, import block, a blank line, then the "+
				"declarations, which is the shape gofmt leaves")
	})
}
