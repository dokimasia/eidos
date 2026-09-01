// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The canonical scale every layer benches at: 1000 packages of 10
// files of 20 declarations, 200k declarations in all.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// scaledTree writes the canonical corpus as Go source: per file,
// four cross-referencing types and sixteen constants.
func scaledTree() (fstest.MapFS, [][]plugin.SourceRef) {
	tree := fstest.MapFS{"go.mod": {Data: []byte("module bench.test/corpus\n")}}
	units := make([][]plugin.SourceRef, 0, benchPackages)
	for p := range benchPackages {
		prev := fmt.Sprintf("bench.test/corpus/p%d", (p+benchPackages-1)%benchPackages)
		unit := make([]plugin.SourceRef, 0, benchFiles)
		for f := range benchFiles {
			var b strings.Builder
			fmt.Fprintf(&b, "package p%d\n\nimport prev %q\n\nvar _ = prev.T%d_0{}\n\n", p, prev, f)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "type T%d_%d struct {\n\tf0 prev.T%d_0\n\tf1 int\n}\n\n", f, d, f)
				} else {
					fmt.Fprintf(&b, "const c%d_%d = %d\n\n", f, d, d)
				}
			}
			path := fmt.Sprintf("p%d/f%d.go", p, f)
			tree[path] = &fstest.MapFile{Data: []byte(b.String())}
			unit = append(unit, plugin.SourceRef{Path: path, Shared: []string{"go.mod"}})
		}
		units = append(units, unit)
	}
	return tree, units
}

// BenchmarkParse drives the frontend's own layer at the canonical
// scale: every unit through Parse into a private builder, the
// driver's splice and seal excluded, so the number prices the Go
// lowering alone.
func BenchmarkParse(b *testing.B) {
	tree, units := scaledTree()
	f := frontend.New(nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, unit := range units {
			u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull,
				f.Syntax(), diag.NewSink(), f.Name())
			if err := f.Parse(context.Background(), u); err != nil {
				b.Fatalf("the corpus parses: %v", err)
			}
		}
	}
}
