// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Declarations render in the order the flush fixed, and a
// declaration the language cannot spell whole is skipped with the
// imports it recorded, so the order, the skip and the nested
// indentation are contract.
func TestDecls(t *testing.T) {
	t.Parallel()

	t.Run("renders declarations in the unit's canonical order", func(t *testing.T) {
		t.Parallel()

		files, _ := runPass(t, language(), seeded(t,
			unitOf("gen", "store.go", "Beta", "Alpha"),
		))
		assert.ContainsInOrder(t, string(files[0].Body), []string{"Beta", "Alpha"},
			"the unit's own declaration order holds: the flush ordered it")
	})

	t.Run("a missing kind template is a positioned finding", func(t *testing.T) {
		t.Parallel()

		l := language()
		u := unitOf("gen", "store.go", "Alpha")
		u.Decls = append(u.Decls, &emit.Method{
			Origin: coretest.Struct(coretest.StorePath, "M").ID,
			Name:   "Handle",
		})
		files, sink := runPass(t, l, seeded(t, u))
		assert.True(t, sink.Failed(), "a kind the language cannot spell reports")
		assert.Length(t, files, 1, "and the remaining declarations still render")
		assert.Contains(t, string(files[0].Body), "Alpha",
			"the file keeps what rendered")
	})

	t.Run("a declaration skipped mid-render records no imports", func(t *testing.T) {
		t.Parallel()

		load := fn("store.go", "Load", emit.Body{Stmts: []emit.Stmt{call("pkg.Run"), refused()}})
		load.Decls = append([]symbol.Symbol{&emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, "Alpha").ID,
			Name:   "Alpha",
		}}, load.Decls...)
		files, sink := runPass(t, language(), seeded(t, load))
		coretest.AssertCodes(t, sink, render.RefusedTemplate)
		assert.Length(t, files, 1, "the file renders without the skipped declaration")
		assert.Equal(t, string(files[0].Body), "type Alpha struct{}\n",
			"and without the import the skipped declaration recorded before it failed")
	})

	t.Run("nests declarations through their kind templates", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
			"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
		host := &emit.Struct{Name: "Row"}
		inner := &emit.Struct{Name: "Inner"}
		inner.Types.Append(&emit.Struct{Name: "Deepest"})
		host.Types.Append(inner)
		u := unitOf("gen", "store.go")
		u.Decls = append(u.Decls, host)

		files, sink := runPass(t, l, seeded(t, u))
		assert.Length(t, slices.Collect(sink.All()), 0, "the nesting renders clean")
		assert.Equal(t, string(files[0].Body),
			"type Row {\n"+
				"\ttype Inner {\n"+
				"\t\ttype Deepest {\n"+
				"\t\t}\n"+
				"\t}\n"+
				"}\n",
			"each level indents once more, blank lines stay bare")
	})

	t.Run("a nested kind without a template reports and is skipped", func(t *testing.T) {
		t.Parallel()

		l := language()
		l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
			"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
		host := &emit.Struct{Name: "Row"}
		host.Types.Append(&emit.Enum{Name: "Phase"})
		u := unitOf("gen", "store.go")
		u.Decls = append(u.Decls, host)

		files, sink := runPass(t, l, seeded(t, u))
		var codes []diag.Code
		for d := range sink.All() {
			codes = append(codes, d.Code)
		}
		assert.Equal(t, codes, []diag.Code{render.UnspeltKind},
			"the unspelt nested kind is one finding")
		assert.Equal(t, string(files[0].Body), "type Row {\n\n}\n",
			"and the host renders without it")
	})

	t.Run("nests through the member's own kind template", func(t *testing.T) {
		t.Parallel()

		hosting := func() render.Language {
			l := language()
			l.Kinds[symbol.KindStruct] = "type {{.Name}} {\n" +
				"{{- range .Types.Items}}\n{{nested \"\t\" .}}{{- end}}\n}\n"
			return l
		}
		hostOf := func(inner symbol.Symbol) plugin.Unit {
			host := &emit.Struct{Name: "Row"}
			host.Types.Append(inner)
			u := unitOf("gen", "store.go")
			u.Decls = append(u.Decls, host)
			return u
		}

		t.Run("a member's refusal propagates to its host", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = "{{.Missing}}"
			_, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: "Phase"})))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), "Missing",
				"a host never renders around a half-spelt member")
		})

		t.Run("a member spelling nothing indents nothing", func(t *testing.T) {
			t.Parallel()

			l := hosting()
			l.Kinds[symbol.KindEnum] = ""
			files, sink := runPass(t, l, seeded(t, hostOf(&emit.Enum{Name: "Phase"})))
			assert.False(t, sink.Failed(), "an empty spelling is a spelling")
			assert.Equal(t, string(files[0].Body), "type Row {\n\n}\n",
				"the block carries no indentation of its own")
		})
	})
}
