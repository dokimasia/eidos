// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang-rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// methodOn returns a method attached to a type by reference, the
// shape a Rust impl gathers.
func methodOn(receives, name string, stmts ...emit.Stmt) *emit.Method {
	m := &emit.Method{Name: name, Receives: &emit.TypeRef{Spelling: receives}}
	m.Body = emit.Body{Stmts: stmts}
	return m
}

// Rust renders a method only inside an impl block, so the cluster
// is what makes the method kind renderable at all.
func TestCluster(t *testing.T) {
	t.Parallel()

	t.Run("gathers methods per attached type", func(t *testing.T) {
		t.Parallel()

		track := methodOn("Row", "track")
		load := methodOn("Store", "load")
		drop := methodOn("Row", "drop")
		free := &emit.Function{Name: "boot"}
		out := backend.Cluster([]symbol.Symbol{track, load, free, drop})
		assert.Equal(t, out, []render.Clustered{
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{track, drop}},
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{load}},
		}, "one impl per type in first-appearance order, functions untouched")
	})

	t.Run("leaves an unattached method alone", func(t *testing.T) {
		t.Parallel()

		out := backend.Cluster([]symbol.Symbol{&emit.Method{Name: "orphan"}})
		assert.Equal(t, len(out), 0,
			"nothing to group by, so the render reports the kind instead")
	})

	t.Run("the impl template is pinned byte for byte", func(t *testing.T) {
		t.Parallel()

		tmpl, err := template.New("impl").
			Funcs(backend.Funcs()).
			Funcs(template.FuncMap{
				"body": func(any) string { return "        return;\n" },
			}).
			Parse(backend.ImplTemplate)
		assert.NoError(t, err, "the template parses")

		track := methodOn("Row", "track", emit.Stmt{Kind: emit.StmtReturn})
		track.Doc = []string{"Track records one access."}
		var b strings.Builder
		assert.NoError(t, tmpl.Execute(&b, render.Clustered{
			Group: backend.ImplGroup,
			Decls: []symbol.Symbol{track},
		}), "the template executes")
		assert.Equal(t, b.String(),
			"impl Row {\n"+
				"    /// Track records one access.\n"+
				"    pub fn track(&self) {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"the receiver-first signature under the attached type's block")
	})

	t.Run("an associated function stands without a receiver", func(t *testing.T) {
		t.Parallel()

		tmpl, err := template.New("impl").
			Funcs(backend.Funcs()).
			Funcs(template.FuncMap{
				"body": func(any) string { return "        return;\n" },
			}).
			Parse(backend.ImplTemplate)
		assert.NoError(t, err, "the template parses")

		assoc := methodOn("Row", "make", emit.Stmt{Kind: emit.StmtReturn})
		assoc.Level = symbol.LevelType
		assoc.Async = true
		var b strings.Builder
		assert.NoError(t, tmpl.Execute(&b, render.Clustered{
			Group: backend.ImplGroup,
			Decls: []symbol.Symbol{assoc},
		}), "the template executes")
		assert.Equal(t, b.String(),
			"impl Row {\n"+
				"    pub async fn make() {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"no receiver at type level, async before fn")
	})

	t.Run("a generic receiver opens the binder", func(t *testing.T) {
		t.Parallel()

		tmpl, err := template.New("impl").
			Funcs(backend.Funcs()).
			Funcs(template.FuncMap{
				"body": func(any) string { return "        return;\n" },
			}).
			Parse(backend.ImplTemplate)
		assert.NoError(t, err, "the template parses")

		fold := &emit.Method{
			Name: "fold",
			Receives: &emit.TypeRef{
				Spelling: "Box",
				Args:     []*emit.TypeRef{{Spelling: "T"}},
			},
			TypeParams: []*emit.TypeParam{
				{Name: "U", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}},
			},
			Params:  []*emit.Param{{Name: "item", Type: &emit.TypeRef{Spelling: "U"}}},
			Returns: []*emit.Return{{Type: &emit.TypeRef{Spelling: "U"}}},
		}
		fold.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
		var b strings.Builder
		assert.NoError(t, tmpl.Execute(&b, render.Clustered{
			Group: backend.ImplGroup,
			Decls: []symbol.Symbol{fold},
		}), "the template executes")
		assert.Equal(t, b.String(),
			"impl<T> Box<T> {\n"+
				"    pub fn fold<U: Codec>(&self, item: U) -> U {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"the receiver's arguments restate as the binder, the method's "+
				"own parameters behind its name")
	})
}
