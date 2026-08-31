// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(typescript.Funcs()).
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

// Each kind template is pinned byte for byte, and the absent
// method kind is pinned too: members render inside their host.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("a method has no module-level spelling", func(t *testing.T) {
		t.Parallel()

		_, held := typescript.KindTemplates()[symbol.KindMethod]
		assert.False(t, held,
			"TypeScript states members inside their type, so a standalone "+
				"method degrades to a reported kind rather than guessed text")
	})

	t.Run("class carries fields and methods with bodies", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("string"),
		})
		s.Methods.Append(&emit.Method{
			Name: "load", Returns: []*emit.Return{{Type: ref("Row")}},
		})
		assert.Equal(t, execute(t, typescript.StructTemplate, s),
			"/**\n * Row is one record.\n */\n"+
				"export class Row {\n"+
				"  /**\n   * Key addresses the row.\n   */\n"+
				"  key: string;\n"+
				"  load(): Row {\n    body();\n  }\n"+
				"}\n",
			"members at member depth, docs indented whole")
	})

	t.Run("interface carries signatures alone", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{
			Name:   "load",
			Params: []*emit.Param{{Name: "key", Type: ref("string")}},
			Returns: []*emit.Return{
				{Type: ref("Row")},
			},
		})
		assert.Equal(t, execute(t, typescript.InterfaceTemplate, i),
			"export interface Store {\n  load(key: string): Row;\n}\n",
			"a signature closes with a semicolon and carries no body")
	})

	t.Run("function, alias, constant and variable", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref("Row")}}}
		assert.Equal(t, execute(t, typescript.FunctionTemplate, f),
			"export function load(): Row {\n    body();\n}\n", "the function shape")
		assert.Equal(t,
			execute(t, typescript.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string")}),
			"export type ID = string;\n", "the alias shape")
		assert.Equal(t,
			execute(t, typescript.ConstantTemplate,
				&emit.Constant{Name: "MAX", Type: ref("number"), Value: "10"}),
			"export const MAX: number = 10;\n", "a typed constant")
		assert.Equal(t,
			execute(t, typescript.VariableTemplate, &emit.Variable{Name: "count"}),
			"export let count;\n", "an untyped binding stays untyped")
	})

	t.Run("the file skeleton is imports then declarations", func(t *testing.T) {
		t.Parallel()

		got := execute(t, typescript.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store.stub.ts"})
		assert.Equal(t, got, "IMPORTS\nDECLS\n",
			"no package clause, because the file is the module")
	})
}
