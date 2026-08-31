// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-java"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(src string, data any) (string, error) {
	tmpl, err := template.New("kind").
		Funcs(java.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "        body();\n" },
			"use":     func(string) string { return "" },
			"imports": func() string { return "IMPORTS\n" },
			"decls":   func() string { return "DECLS\n" },
			"slots":   func() string { return "" },
			"slot":    func(string) string { return "" },
		}).
		Parse(src)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// The inventory is two templates by design, and both are pinned
// byte for byte; so is the refusal that keeps the return model
// honest.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("the inventory is class and interface alone", func(t *testing.T) {
		t.Parallel()

		kinds := java.KindTemplates()
		assert.Length(t, kinds, 2,
			"Java states everything inside a type, so nothing else "+
				"has a file-level spelling")
		_, held := kinds[symbol.KindStruct]
		assert.True(t, held, "the class")
		_, held = kinds[symbol.KindInterface]
		assert.True(t, held, "and the interface")
	})

	t.Run("class carries fields and methods with bodies", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key",
			Type: &emit.TypeRef{Spelling: "String"},
		})
		s.Methods.Append(&emit.Method{
			Name:    "load",
			Params:  []*emit.Param{{Name: "key", Type: &emit.TypeRef{Spelling: "String"}}},
			Returns: []*emit.Return{{Type: &emit.TypeRef{Spelling: "Row"}}},
		})
		got, err := execute(java.StructTemplate, s)
		assert.NoError(t, err, "the class renders")
		assert.Equal(t, got,
			"/**\n * Row is one record.\n */\n"+
				"public class Row {\n"+
				"    /**\n     * Key addresses the row.\n     */\n"+
				"    public String key;\n"+
				"    public Row load(String key) {\n"+
				"        body();\n"+
				"    }\n"+
				"}\n",
			"members public at member depth, docs indented whole")
	})

	t.Run("interface carries signatures alone", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{Name: "close"})
		got, err := execute(java.InterfaceTemplate, i)
		assert.NoError(t, err, "the interface renders")
		assert.Equal(t, got,
			"public interface Store {\n    void close();\n}\n",
			"implicitly public, no body, void for no result")
	})

	t.Run("a second return value refuses at render", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{Name: "load", Returns: []*emit.Return{
			{Type: &emit.TypeRef{Spelling: "Row"}},
			{Type: &emit.TypeRef{Spelling: "Exception"}},
		}})
		_, err := execute(java.InterfaceTemplate, i)
		assert.HasError(t, err,
			"a Java callable returns one value, and the second arrives thrown")
	})

	t.Run("the file skeleton opens with the package clause", func(t *testing.T) {
		t.Parallel()

		got, err := execute(java.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Pkg: symbol.Identity{Package: "svc/api"}})
		assert.NoError(t, err, "the skeleton renders")
		assert.Equal(t, got, "package svc.api;\n\nIMPORTS\nDECLS\n",
			"dots for slashes, then imports, then declarations")
	})
}
