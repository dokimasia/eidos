// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand that every unit of a case reads its carriers under.
const brand = string(frontendtest.Brand)

// The file of a case of one source, its package, and the project file at
// the root of the tree of a case, whose module root is the root of the
// workspace.
const (
	appFile     = "shop/app.py"
	appPkg      = "shop/app"
	projectFile = "pyproject.toml"
	rootProject = "[project]\nname = \"shop\"\n[build-system]\nbuild-backend = \"uv_build\"\n" +
		"[tool.uv.build-backend]\nmodule-root = \"\"\n"
)

// The carrier of the cases, under the brand of the suite, and the
// directive that it attaches.
const (
	carrier          = "# +fixture:gen:table name=rows\n"
	carrierDirective = "gen:table"
)

// scopeSource is a module whose names the cases of the scope resolve: two
// names of a module, one under an alias, a module, a module under an
// alias, a star import, a relative import, and a nested class.
const scopeSource = "from shop.models import Order, Line as L\nimport collections.abc\nimport os.path as osp\n" +
	"from typing import *\nfrom .base import Base\n\n\nclass Outer:\n    class Inner: ...\n"

// treeReader is the recorded door of the partition over a test tree. It
// returns the error of a name that the tree does not contain without the
// scan of every file that fstest.MapFS makes for a directory of the name.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns the bytes of one file of the tree.
func (r treeReader) Read(p string) ([]byte, error) {
	if _, held := r.tree[p]; !held {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	return r.tree.ReadFile(p)
}

// storeTree is a workspace tree with named stores beside it, as the load
// passes it to a unit.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one named store, and false for a name that the tree does
// not provide.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, provided := t.stores[name]
	return s, provided
}

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that
// it measures where the method has more than one call, its allocation
// ceiling, the call, and the check of the result of the call.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
}

// claimedIn returns every Python file of a tree as the selection claims
// it, in path order.
func claimedIn(tree fstest.MapFS) []plugin.SourceRef {
	claimed := make([]plugin.SourceRef, 0, len(tree))
	for p := range tree {
		if ext := path.Ext(p); ext == python.Extension || ext == python.StubExtension {
			claimed = append(claimed, plugin.SourceRef{Path: p})
		}
	}
	slices.SortFunc(claimed, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	return claimed
}

// parsedTree partitions the Python files of a tree through the frontend and
// parses each unit at the depth. It returns the builder of each unit in
// partition order and the sink of every finding of the parses.
func parsedTree(tb testing.TB, tree fstest.MapFS, depth plugin.Depth) ([]*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	f := frontend.New(nil)
	units, err := f.Partition(tb.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	sink := diag.NewSink()
	out := make([]*plugin.GraphBuilder, 0, len(units))
	for _, unit := range units {
		u := plugin.NewSourceUnit(unit, tree, depth, f.Syntax(), brand, sink, f.Name())
		assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
		out = append(out, u.Graph())
	}
	return out, sink
}

// parsedSource parses one source at appFile, beside the project file of
// the root, and returns the builder of its unit and the sink of its
// findings.
func parsedSource(tb testing.TB, src string) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	gbs, sink := parsedTree(tb, fstest.MapFS{
		appFile: {Data: []byte(src)}, projectFile: {Data: []byte(rootProject)},
	}, plugin.DepthFull)
	assert.Length(tb, gbs, 1, "one module is one unit")
	return gbs[0], sink
}

// fileOf returns the File of the source at appFile that a builder declares.
func fileOf(tb assert.TB, gb *plugin.GraphBuilder) *node.File {
	tb.Helper()

	for _, p := range gb.Packages() {
		for _, f := range p.Files {
			if f.Path == appFile {
				return f
			}
		}
	}
	tb.Fatalf("the builder declares no file %s", appFile)
	return nil
}

// declsOf parses one source at appFile and returns the declarations of its
// file.
func declsOf(tb testing.TB, src string) []symbol.Symbol {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	return fileOf(tb, gb).Decls
}

// declNames returns the names of the declarations of a file in their
// order.
func declNames(f *node.File) []string {
	out := make([]string, 0, len(f.Decls))
	for _, decl := range f.Decls {
		out = append(out, nameOf(decl))
	}
	return out
}

// nameOf returns the name of a declaration, and the empty name for a kind
// that a Python module does not declare at its top level.
func nameOf(decl symbol.Symbol) string {
	switch d := decl.(type) {
	case *node.Struct:
		return d.Name
	case *node.Interface:
		return d.Name
	case *node.Enum:
		return d.Name
	case *node.Alias:
		return d.Name
	case *node.Function:
		return d.Name
	case *node.Variable:
		return d.Name
	case *node.Constant:
		return d.Name
	default:
		return ""
	}
}

// named returns the declaration of a name among declarations, of the type
// that the case expects.
func named[T symbol.Symbol](tb testing.TB, decls []symbol.Symbol, name string) T {
	tb.Helper()

	at := slices.IndexFunc(decls, func(d symbol.Symbol) bool {
		_, is := d.(T)
		return is && nameOf(d) == name
	})
	assert.NotEqual(tb, at, -1, "a declaration of the expected kind is named "+name)
	decl, _ := decls[at].(T)
	return decl
}

// classField parses one source at appFile and returns the first field of
// the class C.
func classField(tb testing.TB, src string) *node.Field {
	tb.Helper()

	st := named[*node.Struct](tb, declsOf(tb, src), "C")
	assert.NotEmpty(tb, st.Fields, "the class C has a field")
	return st.Fields[0]
}

// variantNames returns the names of the variants of an enum, in order.
func variantNames(e *node.Enum) []string {
	out := make([]string, 0, len(e.Variants))
	for _, v := range e.Variants {
		out = append(out, v.Name)
	}
	return out
}

// bindingNames returns the names that an export lists, in order.
func bindingNames(x *node.Export) []string {
	out := make([]string, 0, len(x.Names))
	for _, b := range x.Names {
		out = append(out, b.Name)
	}
	return out
}

// spellings returns the spelling of each reference, in order.
func spellings(refs []*node.TypeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Spelling)
	}
	return out
}

// stampsOn returns the values that a builder stamped on one subject under
// one key, in record order. A list value contributes each of its entries,
// and a mark contributes true.
func stampsOn(gb *plugin.GraphBuilder, subject symbol.Symbol, key meta.KeyName) []any {
	var out []any
	for _, r := range gb.StampRecords() {
		if r.Subject != subject || r.Stamp.Key != key {
			continue
		}
		switch v := r.Stamp.Value.(type) {
		case []string:
			for _, e := range v {
				out = append(out, e)
			}
		default:
			out = append(out, v)
		}
	}
	return out
}

// codesOf returns the codes of the findings of a sink, in their order.
func codesOf(sink *diag.Sink) []diag.Code {
	out := []diag.Code{}
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}

// scopeOf parses one source at appFile and returns the scope that its
// parse recorded for its file.
func scopeOf(tb testing.TB, src string) plugin.ImportScope {
	tb.Helper()

	gb, _ := parsedSource(tb, src)
	records := gb.Scopes()
	assert.Length(tb, records, 1, "the module records one scope")
	return plugin.ImportScope{
		File:     symbol.Identity{Lang: frontend.Lang, Package: appPkg},
		Bindings: records[0].Bindings,
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and the
// result of each call.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its ceiling:
// one sub-benchmark for each method, in the order of the first call of
// each method, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if !slices.Contains(methods, c.name) {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling, and
// checks the result of the last call. The contract warms up with one call,
// so the count leaves out what the first call initialises.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}
