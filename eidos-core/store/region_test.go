// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"math"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// A region's summary is what a sealed graph decides every decode by,
// so a summary that leaves out a package, a kind or a spelling hides
// the region from the reads that need it.
func TestRegion(t *testing.T) {
	t.Parallel()

	t.Run("Info", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each package once in identity order", func(t *testing.T) {
			t.Parallel()

			r := &store.Region{Packages: []*node.Package{
				coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath),
			}}
			assert.Equal(t, r.Info().Packages, []symbol.Identity{
				coretest.PackageID(coretest.CachePath), coretest.PackageID(coretest.StorePath),
			}, "the packages sort by identity")
		})

		t.Run("returns the package of a subject only an attachment names", func(t *testing.T) {
			t.Parallel()

			r := &store.Region{
				Packages:   []*node.Package{coretest.Package(coretest.StorePath)},
				Directives: map[symbol.Identity][]directive.Raw{coretest.PackageID(coretest.CachePath): {stubAt(1)}},
				Stamps:     map[symbol.Identity][]meta.RawStamp{coretest.Struct("svc/index", "Index").ID: {stampAt(1)}},
			}
			assert.Equal(t, r.Info().Packages, []symbol.Identity{
				coretest.PackageID(coretest.CachePath),
				coretest.PackageID("svc/index"),
				coretest.PackageID(coretest.StorePath),
			}, "a directive's and a stamp's package join the region's packages")
		})

		t.Run("returns each file with its package in path order", func(t *testing.T) {
			t.Parallel()

			split := newSplit()
			r := &store.Region{Packages: []*node.Package{split.second, split.first}}
			assert.Equal(t, r.Info().Files, []store.RegionFile{
				{Path: secondFile, Pkg: coretest.PackageID(coretest.StorePath)},
				{Path: firstFile, Pkg: coretest.PackageID(coretest.StorePath)},
			}, "the files sort by path")
		})

		t.Run("returns every kind a graph that loaded the region enumerates", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.EveryKind(coretest.StorePath)
			loaded := coretest.Frozen(t, pkg)
			var want []symbol.Kind
			for kind := range symbol.Kind(math.MaxUint8) {
				if len(slices.Collect(loaded.ByKind(kind))) > 0 {
					want = append(want, kind)
				}
			}
			r := &store.Region{Packages: []*node.Package{pkg}}
			assert.Equal(t, r.Info().Kinds, want, "the kinds the graph indexes, in kind order")
		})

		t.Run("leaves out the kind of a declaration whose identity is zero", func(t *testing.T) {
			t.Parallel()

			r := &store.Region{Packages: []*node.Package{
				coretest.Package(coretest.StorePath, &node.Interface{Name: "Dropped"}),
			}}
			assert.Equal(t, r.Info().Kinds, []symbol.Kind{symbol.KindPackage, symbol.KindFile},
				"no index reaches a declaration without an identity")
		})

		t.Run("returns each directive spelling once in name order", func(t *testing.T) {
			t.Parallel()

			s := coretest.Struct(coretest.StorePath, "Store")
			late := stubAt(2)
			late.Name = "alpha"
			r := &store.Region{
				Packages:   []*node.Package{coretest.Package(coretest.StorePath, s)},
				Directives: map[symbol.Identity][]directive.Raw{s.ID: {stubAt(1), late, stubAt(3)}},
			}
			assert.Equal(t, r.Info().Directives, []directive.Name{"alpha", "stub"},
				"the spellings sort, each once")
		})
	})
}

// TestRegionAllocs checks the allocations of one summary of a package
// of every kind: its package list, its file list and its kind list,
// each once. The package attaches no directive, so its spelling list is
// empty and allocates nothing. The check runs alone, because the count
// includes every goroutine's allocations.
func TestRegionAllocs(t *testing.T) {
	r := &store.Region{Packages: []*node.Package{coretest.EveryKind(coretest.StorePath)}}
	var got store.RegionInfo
	assert.MaxAllocs(t, func() { got = r.Info() }, infoAllocs, "Info allocates its summary")
	assert.NotEmpty(t, got.Kinds, "Info returns the package's kinds")
}

// BenchmarkRegion measures the summary of a package of every kind under
// its allocation ceiling.
func BenchmarkRegion(b *testing.B) {
	r := &store.Region{Packages: []*node.Package{coretest.EveryKind(coretest.StorePath)}}

	b.Run("Info", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(infoAllocs)
		defer c.End()
		var got store.RegionInfo
		for c.Loop() {
			got = r.Info()
		}
		assert.NotEmpty(b, got.Kinds, "Info returns the package's kinds")
	})
}

// infoAllocs is the ceiling of one summary of a package of every kind:
// one allocation for each of its three non-empty lists.
const infoAllocs = 3
