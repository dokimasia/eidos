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

// methodOn returns a method attached to a type by reference, the
// shape a Rust impl gathers.
func methodOn(receives *emit.TypeRef, name string, stmts ...emit.Stmt) *emit.Method {
	m := &emit.Method{Name: name, Receives: receives}
	m.Body = emit.Body{Stmts: stmts}
	return m
}

// renderImpl runs the impl template over one cluster, the body
// builtin stubbed to a return, and returns the text or the refusal.
func renderImpl(t *testing.T, decls ...symbol.Symbol) (string, error) {
	t.Helper()

	tmpl, err := template.New("impl").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body": func(any) string { return "        return;\n" },
		}).
		Parse(backend.ImplTemplate)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	err = tmpl.Execute(&b, render.Clustered{Group: backend.ImplGroup, Decls: decls})
	return b.String(), err
}

// Rust renders a method only inside an impl block, so the cluster
// is what makes the method kind renderable at all.
func TestCluster(t *testing.T) {
	t.Parallel()

	t.Run("gathers methods per attached type", func(t *testing.T) {
		t.Parallel()

		track := methodOn(ref("Row"), "track")
		load := methodOn(ref("Store"), "load")
		drop := methodOn(ref("Row"), "drop")
		free := &emit.Function{Name: "boot"}
		out := backend.Cluster([]symbol.Symbol{track, load, free, drop})
		assert.Equal(t, out, []render.Clustered{
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{track, drop}},
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{load}},
		}, "one impl per type in first-appearance order, functions untouched")
	})

	t.Run("keys the impl by the whole reference", func(t *testing.T) {
		t.Parallel()

		named := methodOn(generic("Wrapper", ref("String")), "name")
		counted := methodOn(generic("Wrapper", ref("i32")), "count")
		again := methodOn(generic("Wrapper", ref("String")), "again")
		out := backend.Cluster([]symbol.Symbol{named, counted, again})
		assert.Equal(t, out, []render.Clustered{
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{named, again}},
			{Group: backend.ImplGroup, Decls: []symbol.Symbol{counted}},
		}, "two instantiations of one type open two impl blocks")
	})

	t.Run("leaves an unattached method alone", func(t *testing.T) {
		t.Parallel()

		out := backend.Cluster([]symbol.Symbol{&emit.Method{Name: "orphan"}})
		assert.Equal(t, len(out), 0,
			"nothing to group by, so the render reports the kind instead")
	})

	t.Run("the impl template is pinned byte for byte", func(t *testing.T) {
		t.Parallel()

		track := methodOn(ref("Row"), "track", emit.Stmt{Kind: emit.StmtReturn})
		track.Doc = []string{"Track records one access."}
		track.Comment = "hot path"
		got, err := renderImpl(t, track)
		assert.NoError(t, err, "the template executes")
		assert.Equal(t, got,
			"impl Row {\n"+
				"    /// Track records one access.\n"+
				"    pub fn track(&self) {\n"+
				"        return;\n"+
				"    } // hot path\n"+
				"}\n",
			"the receiver-first signature under the attached type's block, "+
				"the trailing comment behind the closing brace")
	})

	t.Run("an associated function takes no receiver", func(t *testing.T) {
		t.Parallel()

		assoc := methodOn(ref("Row"), "make", emit.Stmt{Kind: emit.StmtReturn})
		assoc.Level = symbol.LevelType
		assoc.Async = true
		got, err := renderImpl(t, assoc)
		assert.NoError(t, err, "the template executes")
		assert.Equal(t, got,
			"impl Row {\n"+
				"    pub async fn make() {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"no receiver at type level, async before fn")
	})

	t.Run("a stated receiver spells as stated", func(t *testing.T) {
		t.Parallel()

		set := methodOn(ref("Row"), "set", emit.Stmt{Kind: emit.StmtReturn})
		set.Receiver = &emit.Param{Name: "self", Type: ref("&mut Self")}
		got, err := renderImpl(t, set)
		assert.NoError(t, err, "the template executes")
		assert.Equal(t, got,
			"impl Row {\n"+
				"    pub fn set(&mut self) {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"a mutable borrow, where the default is the shared one")
	})

	t.Run("a generic receiver opens the binder", func(t *testing.T) {
		t.Parallel()

		fold := methodOn(generic("Box", ref("T")), "fold", emit.Stmt{Kind: emit.StmtReturn})
		fold.TypeParams = []*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}},
		}
		fold.Params = []*emit.Param{{Name: "item", Type: ref("U")}}
		fold.Returns = []*emit.Return{{Type: ref("U")}}
		got, err := renderImpl(t, fold)
		assert.NoError(t, err, "the template executes")
		assert.Equal(t, got,
			"impl<T> Box<T> {\n"+
				"    pub fn fold<U: Codec>(&self, item: U) -> U {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"the receiver's parameters open the binder, the method's own "+
				"parameters behind its name")
	})

	t.Run("a concrete receiver argument binds nothing", func(t *testing.T) {
		t.Parallel()

		name := methodOn(generic("Wrapper", ref("String")), "name", emit.Stmt{Kind: emit.StmtReturn})
		got, err := renderImpl(t, name)
		assert.NoError(t, err, "the template executes")
		assert.Equal(t, got,
			"impl Wrapper<String> {\n"+
				"    pub fn name(&self) {\n"+
				"        return;\n"+
				"    }\n"+
				"}\n",
			"String names a type and never a parameter")
	})

	t.Run("a method's parameter default refuses", func(t *testing.T) {
		t.Parallel()

		fold := methodOn(ref("Row"), "fold", emit.Stmt{Kind: emit.StmtReturn})
		fold.TypeParams = []*emit.TypeParam{{Name: "U", Default: ref("String")}}
		_, err := renderImpl(t, fold)
		assert.HasError(t, err,
			"Rust takes a type parameter default on a type definition alone")
	})
}
