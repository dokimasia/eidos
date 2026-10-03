// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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
// an iterator inlines, so a walk that stopped visiting the graph
// would still report a plausible time and no allocations. Only the
// count shows that the work happened.
const benchSymbols = benchPackages + benchPackages*benchFiles +
	benchPackages*benchFiles*benchDecls

// BenchmarkNode measures the read-side model at the scale of a large
// workspace: the traversals every projection and every
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

	b.Run("AppendBinary", func(b *testing.B) {
		var table node.StringTable
		buf, err := node.AppendBinary(nil, pkgs[0], &table)
		assert.NoError(b, err, "the fixture package encodes")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			table.Reset()
			buf, err = node.AppendBinary(buf[:0], pkgs[0], &table)
		}
		assert.NoError(b, err, "every encoding succeeds")
	})

	b.Run("DecodeBinary", func(b *testing.B) {
		encoded, table := binaryPackage(b, pkgs[0])
		c := bench.Start(b).MaxAllocs(decodePackageAllocs)
		defer c.End()
		var (
			got symbol.Symbol
			err error
		)
		for c.Loop() {
			got, _, err = node.DecodeBinary(encoded, table)
		}
		assert.NoError(b, err, "every decode succeeds")
		assert.Equal(b, got.Kind(), symbol.KindPackage, "to the package")
	})
}

// TestNodeAllocs checks the binary codec's allocation contract over one
// package of the canonical corpus: an encoding with a reused table and
// buffer allocates nothing, and a decode allocates each declaration and
// each list once. The check runs alone, because AllocsPerRun refuses to
// run beside parallel tests.
func TestNodeAllocs(t *testing.T) {
	pkg := coretest.Workspace(1, benchFiles, benchDecls)[0]
	var table node.StringTable
	buf, err := node.AppendBinary(nil, pkg, &table)
	assert.NoError(t, err, "the package encodes")
	assert.MaxAllocs(t, func() {
		table.Reset()
		if buf, err = node.AppendBinary(buf[:0], pkg, &table); err != nil {
			t.Fatal(err)
		}
	}, 0, "AppendBinary with a reused table and buffer allocates nothing")

	encoded, decodedTable := binaryPackage(t, pkg)
	assert.MaxAllocs(t, func() {
		if _, _, err := node.DecodeBinary(encoded, decodedTable); err != nil {
			t.Fatal(err)
		}
	}, decodePackageAllocs, "DecodeBinary allocates each declaration and each list once")
}

// decodePackageAllocs is the ceiling of one decode of a canonical
// package: the package and its path, its file list and its 10 files,
// each file's declaration list, and its 200 structs.
const decodePackageAllocs = 1 + 1 + 1 + benchFiles + benchFiles + benchFiles*benchDecls

// binaryPackage returns a package's encoding against a table, with the
// table as its own encoding decodes.
func binaryPackage(tb testing.TB, pkg *node.Package) ([]byte, *node.StringTable) {
	tb.Helper()

	var table node.StringTable
	encoded, err := node.AppendBinary(nil, pkg, &table)
	assert.NoError(tb, err, "the package encodes")
	tableBytes, err := table.AppendBinary(nil)
	assert.NoError(tb, err, "its table encodes")
	decoded, _, err := node.DecodeStringTable(tableBytes)
	assert.NoError(tb, err, "and decodes")
	return encoded, decoded
}

// assertVisited fails the benchmark unless the traversal visited
// every symbol the corpus contains, so a measured time is always the
// time of a whole walk.
func assertVisited(b *testing.B, seen int) {
	b.Helper()

	if seen != benchSymbols {
		b.Fatalf("the traversal visited %d symbols, want %d: the number "+
			"measures a whole walk or it measures nothing", seen, benchSymbols)
	}
}
