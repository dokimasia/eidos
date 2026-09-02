// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// The canonical scale every layer benches at: 1000 packages of 10
// files of 20 declarations, 200k declarations in all.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// scaledTree writes the canonical corpus in the fake language:
// per file, four cross-referencing types and sixteen constants.
func scaledTree() fstest.MapFS {
	tree := fstest.MapFS{"mod.zz": &fstest.MapFile{Data: []byte("mod bench\n")}}
	for p := range benchPackages {
		prev := fmt.Sprintf("p%d", (p+benchPackages-1)%benchPackages)
		for f := range benchFiles {
			var b strings.Builder
			fmt.Fprintf(&b, "package p%d\nimport prev %s\n", p, prev)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "type T%d_%d prev.T%d_0 int\n", f, d, f)
				} else {
					fmt.Fprintf(&b, "const c%d_%d\n", f, d)
				}
			}
			tree[fmt.Sprintf("p%d/f%d.zz", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
	}
	return tree
}

// BenchmarkLoad drives the whole pipeline at the canonical scale:
// select, partition, parse, splice, assign, resolve, seal, key.
// The fake language's parse is part of the measurement, so the
// number is a ceiling on driver overhead, not a frontend budget.
// The branded case adds the ownership proof, one read per claimed
// file, which is the exclusion's whole cost.
func BenchmarkLoad(b *testing.B) {
	tree := scaledTree()
	fronts := []plugin.Frontend{frontendtest.NewScripted()}

	for _, tc := range []struct {
		name  string
		brand output.Brand
	}{
		{name: "unbranded"},
		{name: "branded", brand: "bench"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				g, report, err := load.Load(context.Background(), load.Config{
					FS:        tree,
					Frontends: fronts,
					Sink:      diag.NewSink(),
					PluginSet: []byte("bench"),
					Brand:     tc.brand,
				})
				if err != nil || !g.Frozen() || len(report.Units) != benchPackages {
					b.Fatalf("the corpus loads: %v", err)
				}
			}
		})
	}
}
