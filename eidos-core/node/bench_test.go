// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// The canonical scale every layer benches at: 1000 packages of 10
// files of 20 declarations, 200k declarations in all.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// benchSymbols is what a whole traversal of the corpus visits: each
// package, each of its files, and each declaration in them.
//
// Every traversal case compares its count against this. A range over
// an iterator inlines, so a walk that stopped reaching the graph
// would still report a plausible time and no allocations; only the
// count says the work happened.
const benchSymbols = benchPackages + benchPackages*benchFiles +
	benchPackages*benchFiles*benchDecls

// BenchmarkNode measures the read-side model at the scale a
// workspace reaches: the traversals every projection and every
// generator runs over the graph, and the encoding a warm cache
// writes and reads back.
//
// The corpus is built once and shared: what is measured is the walk
// and the codec, not the fixture.
func BenchmarkNode(b *testing.B) {
	pkgs := coretest.Workspace(benchPackages, benchFiles, benchDecls)

	b.Run("All", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			seen := 0
			for _, pkg := range pkgs {
				for range node.All(pkg) {
					seen++
				}
			}
			assertVisited(b, seen)
		}
	})

	b.Run("Declarations", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			seen := 0
			for _, pkg := range pkgs {
				for range node.Declarations(pkg) {
					seen++
				}
			}
			assertVisited(b, seen)
		}
	})

	b.Run("Walk", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			seen := 0
			for _, pkg := range pkgs {
				node.Walk(pkg, func(symbol.Symbol) bool {
					seen++
					return true
				})
			}
			assertVisited(b, seen)
		}
	})

	b.Run("EncodeJSON", func(b *testing.B) {
		one := pkgs[0]
		b.ReportAllocs()

		for b.Loop() {
			if _, err := node.EncodeJSON(one); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("DecodeJSON", func(b *testing.B) {
		encoded, err := node.EncodeJSON(pkgs[0])
		assert.NoError(b, err, "the fixture package encodes")
		b.ReportAllocs()

		for b.Loop() {
			if _, err := node.DecodeJSON(encoded); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// assertVisited fails the benchmark unless the traversal reached
// every symbol the corpus holds, so a measured time always belongs
// to a whole walk.
func assertVisited(b *testing.B, seen int) {
	b.Helper()

	if seen != benchSymbols {
		b.Fatalf("the traversal visited %d symbols, want %d: the number "+
			"measures a whole walk or it measures nothing", seen, benchSymbols)
	}
}
