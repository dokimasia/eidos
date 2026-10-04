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

// implBodyStub is what the stubbed body builtin writes inside an impl
// method.
const implBodyStub = "        return;\n"

// The generic type and its parameter the cluster cases name.
const (
	boxName   = "Box"
	itemParam = "T"
)

// The allocations of a cluster and of the group map.
const (
	// clusterAllocs is two methods of one type, a method of an
	// instantiated type and a struct: the instantiated receiver's key,
	// the list of blocks as it grows to two, and each block's list of
	// methods as it grows, two and one.
	clusterAllocs = 1 + 2 + 2 + 1
	// groupsAllocs is the map and the one table of slots that stores the
	// template.
	groupsAllocs = 2
)

// Rust renders a method only inside an impl block, so the cluster is
// what makes the method kind renderable at all.
func TestCluster(t *testing.T) {
	t.Parallel()

	ret := emit.Stmt{Kind: emit.StmtReturn}

	t.Run("Cluster", func(t *testing.T) {
		t.Parallel()

		t.Run("gathers methods per attached type in first-appearance order", func(t *testing.T) {
			t.Parallel()

			track := methodOn(ref(rowName), "track")
			load := methodOn(ref("Store"), "load")
			drop := methodOn(ref(rowName), "drop")
			free := &emit.Function{Name: "boot"}
			assert.Equal(t, backend.Cluster([]symbol.Symbol{track, load, free, drop}), []render.Clustered{
				{Group: backend.ImplGroup, Decls: []symbol.Symbol{track, drop}},
				{Group: backend.ImplGroup, Decls: []symbol.Symbol{load}},
			}, "functions untouched")
		})

		t.Run("opens one impl block per instantiation of a type", func(t *testing.T) {
			t.Parallel()

			named := methodOn(generic("Wrapper", ref("String")), "name")
			counted := methodOn(generic("Wrapper", ref("i32")), "count")
			again := methodOn(generic("Wrapper", ref("String")), "again")
			assert.Equal(t, backend.Cluster([]symbol.Symbol{named, counted, again}), []render.Clustered{
				{Group: backend.ImplGroup, Decls: []symbol.Symbol{named, again}},
				{Group: backend.ImplGroup, Decls: []symbol.Symbol{counted}},
			}, "the whole reference keys the block")
		})

		t.Run("leaves an unattached method unassigned", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, backend.Cluster([]symbol.Symbol{&emit.Method{Name: "orphan"}}), 0,
				"the lowering refuses such a method before any render")
		})
	})

	t.Run("Groups", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the impl template under the impl group", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Groups(), map[render.GroupName]string{backend.ImplGroup: backend.ImplTemplate},
				"the one group the cluster selects")
		})

		t.Run("writes a method under its type's impl block", func(t *testing.T) {
			t.Parallel()

			track := methodOn(ref(rowName), "track", ret)
			track.Doc = []string{"Track records one access."}
			track.Comment = "hot path"
			assert.Equal(t, implOf(t, track),
				"impl Row {\n"+
					"    /// Track records one access.\n"+
					"    pub fn track(&self) {\n"+implBodyStub+"    } // hot path\n"+
					"}\n",
				"the receiver first, the trailing comment behind the closing brace")
		})

		t.Run("writes an associated function without a receiver", func(t *testing.T) {
			t.Parallel()

			assoc := methodOn(ref(rowName), "make", ret)
			assoc.Level = symbol.LevelType
			assoc.Async = true
			assert.Equal(t, implOf(t, assoc),
				"impl Row {\n    pub async fn make() {\n"+implBodyStub+"    }\n}\n", "async before fn")
		})

		t.Run("writes a stated receiver as stated", func(t *testing.T) {
			t.Parallel()

			set := methodOn(ref(rowName), "set", ret)
			set.Receiver = &emit.Param{Name: "self", Type: ref("&mut Self")}
			assert.Equal(t, implOf(t, set),
				"impl Row {\n    pub fn set(&mut self) {\n"+implBodyStub+"    }\n}\n", "a mutable borrow")
		})

		t.Run("opens the binder with a generic receiver's parameters", func(t *testing.T) {
			t.Parallel()

			fold := methodOn(generic("Box", paramRef("T")), "fold", ret)
			fold.TypeParams = []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}}
			fold.Params = []*emit.Param{{Name: "item", Type: ref("U")}}
			fold.Returns = []*emit.Return{{Type: ref("U")}}
			assert.Equal(t, implOf(t, fold),
				"impl<T> Box<T> {\n    pub fn fold<U: Codec>(&self, item: U) -> U {\n"+implBodyStub+"    }\n}\n",
				"the method's own parameters behind its name")
		})

		t.Run("binds nothing for a concrete receiver argument", func(t *testing.T) {
			t.Parallel()

			name := methodOn(generic("Wrapper", ref("String")), "name", ret)
			assert.Equal(t, implOf(t, name),
				"impl Wrapper<String> {\n    pub fn name(&self) {\n"+implBodyStub+"    }\n}\n",
				"String names a type and never a parameter")
		})

		t.Run("uses the item a method's parameter type names", func(t *testing.T) {
			t.Parallel()

			load := methodOn(ref("Store"), "load", ret)
			load.Params = []*emit.Param{{Name: "row", Type: imported(storeModule, rowName)}}
			_, set, err := renderImpl(t, load)
			assert.NoError(t, err, "the template executes")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the item's use")
		})

		t.Run("returns an error for a method's parameter default", func(t *testing.T) {
			t.Parallel()

			fold := methodOn(ref(rowName), "fold", ret)
			fold.TypeParams = []*emit.TypeParam{{Name: "U", Default: ref("String")}}
			_, _, err := renderImpl(t, fold)
			assert.HasError(t, err, "Rust takes a default on a type definition alone")
		})
	})
}

// A cluster allocates its blocks, and the group map allocates itself.
// The ordinary run, which runs no benchmark, checks those ceilings
// here.
func TestClusterAllocs(t *testing.T) {
	checkAllocs(t, clusterCalls())
}

// BenchmarkCluster measures the gathering the render runs over every
// unit, and the group map the backend reads once per build.
func BenchmarkCluster(b *testing.B) {
	benchCalls(b, clusterCalls())
}

// clusterCalls returns a call of Cluster over two methods of one type,
// a method of an instantiated type and a struct, and of Groups.
func clusterCalls() []allocCall {
	decls := []symbol.Symbol{
		methodOn(ref(rowName), "get"), methodOn(ref(rowName), "put"),
		methodOn(generic(boxName, paramRef(itemParam)), "take"), &emit.Struct{Name: rowName},
	}
	var (
		clusters []render.Clustered
		groups   map[render.GroupName]string
	)
	return []allocCall{
		{
			name: "Cluster", allocs: clusterAllocs,
			call:  func() { clusters = backend.Cluster(decls) },
			check: func(tb assert.TB) { assert.Length(tb, clusters, 2, "Cluster opens a block per type") },
		},
		{
			name: "Groups", allocs: groupsAllocs,
			call:  func() { groups = backend.Groups() },
			check: func(tb assert.TB) { assert.Length(tb, groups, 1, "Groups returns the impl group") },
		},
	}
}

// methodOn returns a method attached to a type by reference, the
// shape a Rust impl gathers.
func methodOn(receives *emit.TypeRef, name string, stmts ...emit.Stmt) *emit.Method {
	m := &emit.Method{Name: name, Receives: receives}
	m.Body = emit.Body{Stmts: stmts}
	return m
}

// renderImpl runs the impl template over one cluster, the body
// builtin stubbed to a return, and returns the text or the refusal
// beside the file's import set.
func renderImpl(t *testing.T, decls ...symbol.Symbol) (string, *render.ImportSet, error) {
	t.Helper()

	set := &render.ImportSet{}
	tmpl, err := template.New("impl").
		Funcs(backend.Funcs(set)).
		Funcs(template.FuncMap{
			render.BuiltinBody: func(any) string { return implBodyStub },
		}).
		Parse(backend.Groups()[backend.ImplGroup])
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	err = tmpl.Execute(&b, render.Clustered{Group: backend.ImplGroup, Decls: decls})
	return b.String(), set, err
}

// implOf runs the impl template over one cluster and asserts it
// executes.
func implOf(t *testing.T, decls ...symbol.Symbol) string {
	t.Helper()

	got, _, err := renderImpl(t, decls...)
	assert.NoError(t, err, "the template executes")
	return got
}
