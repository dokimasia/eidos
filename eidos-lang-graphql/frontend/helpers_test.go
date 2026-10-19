// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brand is the brand that every unit of a case reads its carriers under.
const brand = string(frontendtest.Brand)

// The file of a case of one source, the package of its directory and the
// package of its executable definitions.
const (
	appFile = "app/schema.graphql"
	appPkg  = "app"
	appDocs = "app/documents"
)

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all. A directory without a
// configuration is one unit, so the corpus is 1000 units.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// treeReader is the recorded door of the partition over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns the bytes of one file of the tree. It returns the error of a
// name that the tree does not contain without the scan of every file that
// fstest.MapFS makes for a directory of the name, as a file system answers a
// missing name.
func (r treeReader) Read(p string) ([]byte, error) {
	if _, held := r.tree[p]; !held {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	return r.tree.ReadFile(p)
}

// partitioned partitions the claimed files of a tree through the frontend.
func partitioned(tb testing.TB, tree fstest.MapFS, claimed ...string) [][]plugin.SourceRef {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(claimed))
	for _, p := range claimed {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	units, err := frontend.New().Partition(tb.Context(), refs, treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	return units
}

// scaledSchema returns the canonical corpus as GraphQL schemas: a directory
// for each package of files of four object types, whose first field names an
// object type of the file before it, and sixteen enums of one value.
func scaledSchema() fstest.MapFS {
	tree := fstest.MapFS{}
	for p := range benchPackages {
		for f := range benchFiles {
			prev := (f + benchFiles - 1) % benchFiles
			var b strings.Builder
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "type T%d_%d_%d {\n  f0: T%d_%d_0\n  f1: Int\n}\n\n", p, f, d, p, prev)
				} else {
					fmt.Fprintf(&b, "enum E%d_%d_%d {\n  ZERO\n}\n\n", p, f, d)
				}
			}
			tree[fmt.Sprintf("p%d/f%d.graphqls", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
	}
	return tree
}

// claimedIn returns a reference to every file of a tree, the claim that the
// load passes to the partition.
func claimedIn(tree fstest.MapFS) []plugin.SourceRef {
	refs := make([]plugin.SourceRef, 0, len(tree))
	for p := range tree {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	return refs
}

// membersOf returns the paths of the members of each unit, in partition
// order.
func membersOf(units [][]plugin.SourceRef) [][]string {
	out := make([][]string, 0, len(units))
	for _, u := range units {
		paths := make([]string, 0, len(u))
		for _, ref := range u {
			paths = append(paths, ref.Path)
		}
		out = append(out, paths)
	}
	return out
}

// parsedTree partitions the GraphQL files of a tree through the frontend
// and parses each unit at full depth. It returns the builder of each unit
// in partition order and the sink of every finding of the parses.
func parsedTree(tb testing.TB, tree fstest.MapFS) ([]*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	f := frontend.New()
	var refs []plugin.SourceRef
	for p := range tree {
		if slices.Contains([]string{graphql.Extension, graphql.SchemaExtension, graphql.ShortExtension}, path.Ext(p)) {
			refs = append(refs, plugin.SourceRef{Path: p})
		}
	}
	units, err := f.Partition(tb.Context(), refs, treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	sink := diag.NewSink()
	out := make([]*plugin.GraphBuilder, 0, len(units))
	for _, unit := range units {
		u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
		assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
		out = append(out, u.Graph())
	}
	return out, sink
}

// parsedSource parses one source at appFile, and returns the builder of its
// unit and the sink of its findings.
func parsedSource(tb testing.TB, src string) (*plugin.GraphBuilder, *diag.Sink) {
	tb.Helper()

	gbs, sink := parsedTree(tb, fstest.MapFS{appFile: {Data: []byte(src)}})
	assert.Length(tb, gbs, 1, "one file is one unit")
	return gbs[0], sink
}

// fileIn returns the file at p in the package pkg of a builder.
func fileIn(tb assert.TB, gb *plugin.GraphBuilder, pkg, p string) *node.File {
	tb.Helper()

	for _, candidate := range gb.Packages() {
		if candidate.ID.Package != pkg {
			continue
		}
		for _, f := range candidate.Files {
			if f.Path == p {
				return f
			}
		}
	}
	tb.Fatalf("the package %s has no file %s", pkg, p)
	return nil
}

// declOf returns the declaration of a file with the name, whatever its kind,
// and fails where the file declares none.
func declOf(tb assert.TB, f *node.File, name string) symbol.Symbol {
	tb.Helper()

	for _, decl := range f.Decls {
		if nameOf(decl) == name {
			return decl
		}
	}
	tb.Fatalf("the file %s declares no %s", f.Path, name)
	return nil
}

// declNames returns the names of the declarations of a file in their order.
func declNames(f *node.File) []string {
	out := make([]string, 0, len(f.Decls))
	for _, decl := range f.Decls {
		out = append(out, nameOf(decl))
	}
	return out
}

// nameOf returns the name of a declaration of a file, and the empty name for
// a kind that a GraphQL file does not declare.
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
	case *node.Sum:
		return d.Name
	default:
		return ""
	}
}

// stampsOn returns the values that a builder stamped on one subject under
// one key, in record order. A list value contributes each of its entries.
func stampsOn(gb *plugin.GraphBuilder, subject symbol.Symbol, key meta.KeyName) []string {
	var out []string
	for _, r := range gb.StampRecords() {
		if r.Subject != subject || r.Stamp.Key != key {
			continue
		}
		switch v := r.Stamp.Value.(type) {
		case string:
			out = append(out, v)
		case []string:
			out = append(out, v...)
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

// findingsOf returns the findings of a sink under one code, in their order.
func findingsOf(sink *diag.Sink, code diag.Code) []diag.Diag {
	var out []diag.Diag
	for d := range sink.All() {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
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
		if _, seen := byMethod[c.name]; !seen {
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
