// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"errors"
	"iter"
	"slices"
	"sync"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The files of the split fixture's store package: the first part's,
// which [coretest.Package] names, and the second part's.
const (
	firstFile  = coretest.UnitFile
	secondFile = "other.go"
)

// The documentation and the name the split fixture's second part
// states, which the merged package keeps because the first part states
// neither.
const (
	splitName = "store"
	splitDoc  = "Package store keeps the records."
)

// memory is a [store.Source] over regions in memory. It counts each
// region's decodes, and a region in fails returns that error.
type memory struct {
	regions []*store.Region
	fails   map[int]error

	mu      sync.Mutex
	decodes []int
}

// newMemory returns a source over the regions, in splice order.
func newMemory(regions ...*store.Region) *memory {
	return &memory{regions: regions, decodes: make([]int, len(regions))}
}

// Regions returns each region's summary, as [store.Region.Info]
// computes it.
func (m *memory) Regions() []store.RegionInfo {
	infos := make([]store.RegionInfo, len(m.regions))
	for i, r := range m.regions {
		infos[i] = r.Info()
	}
	return infos
}

// Region counts the decode and returns the region, or the region's
// failure.
func (m *memory) Region(i int) (*store.Region, error) {
	m.mu.Lock()
	m.decodes[i]++
	m.mu.Unlock()
	if err := m.fails[i]; err != nil {
		return nil, err
	}
	return m.regions[i], nil
}

// decoded returns how many times each region was decoded.
func (m *memory) decoded() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.decodes)
}

// split is a workspace of two packages in three regions, in splice
// order: the store package's first part, the cache package, and the
// store package's second part. The store package is one package that two
// regions contribute files to, and only its second part declares an
// interface.
type split struct {
	store   *node.Struct
	index   *node.Interface
	cache   *node.Package
	first   *node.Package
	second  *node.Package
	regions []*store.Region
}

// newSplit returns the split fixture. Each part of the store package
// attaches a directive to the package and one to its own declaration,
// and stamps its own declaration.
func newSplit() split {
	s := coretest.Struct(coretest.StorePath, "Store")
	ix := coretest.Interface(coretest.StorePath, "Index")
	first := coretest.Package(coretest.StorePath, s)
	second := &node.Package{
		ID:   coretest.PackageID(coretest.StorePath),
		Path: first.Path,
		Name: splitName,
		Doc:  []string{splitDoc},
		Files: []*node.File{{
			ID:    coretest.ID(coretest.StorePath, secondFile, symbol.KindFile),
			Path:  secondFile,
			Decls: node.Symbols{ix},
		}},
	}
	cache := coretest.EveryKind(coretest.CachePath)
	pkg := coretest.PackageID(coretest.StorePath)
	return split{
		store: s, index: ix, cache: cache, first: first, second: second,
		regions: []*store.Region{
			{
				Packages:   []*node.Package{first},
				Directives: map[symbol.Identity][]directive.Raw{pkg: {stubAt(9)}, s.ID: {stubAt(3)}},
				Stamps:     map[symbol.Identity][]meta.RawStamp{s.ID: {stampAt(2)}},
			},
			{Packages: []*node.Package{cache}},
			{
				Packages:   []*node.Package{second},
				Directives: map[symbol.Identity][]directive.Raw{pkg: {stubAt(1)}, ix.ID: {stubAt(4)}},
				Stamps:     map[symbol.Identity][]meta.RawStamp{ix.ID: {stampAt(5), stampAt(1)}},
			},
		},
	}
}

// sealed returns a sealed graph over the fixture's regions and the
// source that counts their decodes.
func (f split) sealed() (*store.Graph, *memory) {
	src := newMemory(f.regions...)
	return store.Sealed(src), src
}

// loaded returns the graph a load of the same packages builds through
// [store.New]: the store package merged the way the splice merges it,
// over the same declarations, with every region's attachments.
func (f split) loaded(tb testing.TB) *store.Graph {
	tb.Helper()

	merged := &node.Package{
		ID:    f.first.ID,
		Path:  f.first.Path,
		Name:  splitName,
		Doc:   []string{splitDoc},
		Files: append(slices.Clone(f.first.Files), f.second.Files...),
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(merged), "the merged store package loads")
	assert.NoError(tb, g.AddPackage(f.cache), "the cache package loads")
	for _, r := range f.regions {
		for subject, raws := range r.Directives {
			assert.NoError(tb, g.AttachDirectives(subject, raws), "the directives attach")
		}
		for subject, stamps := range r.Stamps {
			assert.NoError(tb, g.AttachStamps(subject, stamps), "the stamps attach")
		}
	}
	g.Freeze()
	return g
}

// A sealed graph reads what a graph that loaded the same regions reads,
// in the same order, and decodes only the regions a read needs.
func TestSealed(t *testing.T) {
	t.Parallel()

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration a region declares", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			for _, want := range []node.Declaration{f.store, f.index} {
				got, held := g.Lookup(want.Identity())
				assert.True(t, held, "Lookup returns a declaration of either part")
				assert.Equal(t, got, symbol.Symbol(want), "the region's own declaration", assert.ByIdentity())
			}
		})

		t.Run("decodes only the regions of the package the identity names", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, src := f.sealed()
			g.Lookup(f.store.ID)
			assert.Equal(t, src.decoded(), []int{1, 0, 1},
				"both parts of the store package decode, and the cache package does not")
		})

		t.Run("decodes nothing for a package no region lists", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, src := f.sealed()
			_, held := g.Lookup(coretest.Struct("svc/absent", "Absent").ID)
			assert.False(t, held, "a package no region lists contains nothing")
			assert.Equal(t, src.decoded(), []int{0, 0, 0}, "and no region decodes")
		})

		t.Run("decodes each region once under concurrent readers", func(t *testing.T) {
			t.Parallel()

			const readers = 8
			f := newSplit()
			g, src := f.sealed()
			outcomes := history.Concurrently(readers, time.Minute, func(int) (any, error) {
				_, store := g.Lookup(f.store.ID)
				_, index := g.Lookup(f.index.ID)
				return store && index, nil
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "every reader finishes")
				expect.Equal(t, o.Output, any(true), "every reader finds both parts")
			}
			assert.Equal(t, src.decoded(), []int{1, 0, 1}, "each region decodes once")
		})

		t.Run("returns false for a declaration of a region that did not decode", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			src := newMemory(f.regions...)
			src.fails = map[int]error{2: errDecode}
			g := store.Sealed(src)

			_, held := g.Lookup(f.index.ID)
			assert.False(t, held, "the second part reads as absent")
			_, held = g.Lookup(f.store.ID)
			assert.True(t, held, "and the first part reads whole")
		})
	})

	t.Run("Holds", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a package two regions contribute to", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.True(t, g.Holds(coretest.PackageID(coretest.StorePath)), "the merged package is held")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the merged package of a declaration of either part", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			fromFirst, held := g.PackageOf(f.store.ID)
			assert.True(t, held, "the first part's declaration has a package")
			fromSecond, _ := g.PackageOf(f.index.ID)
			assert.Equal(t, fromSecond, fromFirst, "both parts share one merged package", assert.ByIdentity())
			assert.Equal(t, filePaths(fromFirst), []string{firstFile, secondFile},
				"whose files are both parts' files")
		})

		t.Run("returns false for an identity no region declares", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			_, held := g.PackageOf(coretest.Struct(coretest.StorePath, "Absent").ID)
			assert.False(t, held, "a declaration no region declares has no package")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each package once in identity order", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			var got []symbol.Identity
			for pkg := range g.Packages() {
				got = append(got, pkg.ID)
			}
			assert.Equal(t, got, []symbol.Identity{
				coretest.PackageID(coretest.CachePath), coretest.PackageID(coretest.StorePath),
			}, "the packages come back once each, in identity order")
		})

		t.Run("merges the files of a package in region order", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			store := packageAt(t, g, coretest.StorePath)
			assert.Equal(t, filePaths(store), []string{firstFile, secondFile},
				"the first region's files come first")
		})

		t.Run("keeps the first non-empty name", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.Equal(t, packageAt(t, g, coretest.StorePath).Name, splitName,
				"the second part names the package, because the first does not")
		})

		t.Run("keeps the first non-empty documentation", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.Equal(t, packageAt(t, g, coretest.StorePath).Doc, []string{splitDoc},
				"the second part documents the package, because the first does not")
		})

		t.Run("leaves the regions' package nodes unchanged", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			for range g.Packages() { // ranging is what merges the parts
			}
			assert.Equal(t, filePaths(f.first), []string{firstFile}, "the first part keeps its own file")
			assert.Equal(t, f.first.Name, "", "and its own name")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			seen := 0
			for range g.Packages() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what a graph that loaded the same packages returns", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			sealed, _ := f.sealed()
			loaded := f.loaded(t)
			for _, kind := range append(coretest.MatchableKinds(), symbol.KindPackage, symbol.KindFile) {
				assert.Equal(t, identities(sealed.ByKind(kind)), identities(loaded.ByKind(kind)),
					"each kind enumerates the same declarations in the same order")
			}
		})

		t.Run("returns the declarations of a package's parts in identity order", func(t *testing.T) {
			t.Parallel()

			first := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store"))
			second := &node.Package{ID: first.ID, Path: first.Path, Files: []*node.File{{
				ID:    coretest.ID(coretest.StorePath, secondFile, symbol.KindFile),
				Path:  secondFile,
				Decls: node.Symbols{coretest.Struct(coretest.StorePath, "Cache")},
			}}}
			g := store.Sealed(newMemory(
				&store.Region{Packages: []*node.Package{first}},
				&store.Region{Packages: []*node.Package{second}},
			))
			assert.Equal(t, coretest.Names(t, slices.Collect(g.ByKind(symbol.KindStruct))),
				[]string{"Cache", "Store"}, "the second part declares Cache, which sorts first")
		})

		t.Run("decodes only the regions whose summary lists the kind", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, src := f.sealed()
			assert.Length(t, slices.Collect(g.ByKind(symbol.KindInterface)), 2,
				"the cache package and the store package's second part each declare an interface")
			assert.Equal(t, src.decoded(), []int{0, 1, 1},
				"and the store package's first part, which declares none, does not decode")
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what a graph that loaded the same packages returns", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			sealed, _ := f.sealed()
			assert.Equal(t, identities(sealed.ByDirective("stub")), identities(f.loaded(t).ByDirective("stub")),
				"the carriers come back in identity order")
		})

		t.Run("returns nothing for a spelling no region lists", func(t *testing.T) {
			t.Parallel()

			g, src := newSplit().sealed()
			assert.Empty(t, slices.Collect(g.ByDirective("absent")), "no region attaches the spelling")
			assert.Equal(t, src.decoded(), []int{0, 0, 0}, "so no region decodes")
		})

		t.Run("decodes every region of a package a listing region names", func(t *testing.T) {
			t.Parallel()

			g, src := newSplit().sealed()
			for range g.ByDirective("stub") { // ranging is what decodes
			}
			assert.Equal(t, src.decoded(), []int{1, 0, 1},
				"both parts of the store package decode, because a subject's directives come from both")
		})
	})

	t.Run("DirectivesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every region's instances on a subject in the seal's order", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.Equal(t, g.DirectivesOf(coretest.PackageID(coretest.StorePath)),
				[]directive.Raw{stubAt(1), stubAt(9)}, "the second part's earlier instance sorts first")
		})

		t.Run("returns nothing for a package no region lists", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.Empty(t, g.DirectivesOf(coretest.PackageID("svc/absent")), "no region lists the package")
		})
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what a graph that loaded the same packages returns", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			sealed, _ := f.sealed()
			assert.Equal(t, attached(sealed.Directives()), attached(f.loaded(t).Directives()),
				"every subject comes back with its instances, in identity order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			seen := 0
			for range g.Directives() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})

	t.Run("StampsOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a subject's stamps in the seal's order", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			assert.Equal(t, g.StampsOf(f.index.ID), []meta.RawStamp{stampAt(1), stampAt(5)},
				"the stamps sort by position")
		})

		t.Run("returns nothing for a package no region lists", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.Empty(t, g.StampsOf(coretest.PackageID("svc/absent")), "no region lists the package")
		})
	})

	t.Run("Stamps", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what a graph that loaded the same packages returns", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			sealed, _ := f.sealed()
			assert.Equal(t, attached(sealed.Stamps()), attached(f.loaded(t).Stamps()),
				"every subject comes back with its stamps, in identity order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			seen := 0
			for range g.Stamps() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})
}

// TestSealedZeroAlloc checks that a read of a built index allocates
// nothing: every lookup, and every enumeration ranged directly, once
// each has decoded its regions. The check runs alone, because the count
// includes every goroutine's allocations.
func TestSealedZeroAlloc(t *testing.T) {
	f := newSplit()
	g, _ := f.sealed()
	warm(g, f.store.ID)
	for range g.Packages() { // ranging is what decodes every region
	}

	var held bool
	assert.MaxAllocs(t, func() { _, held = g.Lookup(f.store.ID) }, 0, "Lookup allocates nothing")
	assert.True(t, held, "Lookup finds the declaration")
	assert.MaxAllocs(t, func() { _, held = g.PackageOf(f.store.ID) }, 0, "PackageOf allocates nothing")
	assert.True(t, held, "PackageOf finds the declaration's package")
	assert.MaxAllocs(t, func() {
		for range g.ByKind(symbol.KindStruct) {
		}
	}, 0, "ByKind allocates nothing")
	assert.MaxAllocs(t, func() {
		for range g.ByDirective("stub") {
		}
	}, 0, "ByDirective allocates nothing")
	var raws []directive.Raw
	assert.MaxAllocs(t, func() { raws = g.DirectivesOf(f.store.ID) }, 0, "DirectivesOf allocates nothing")
	assert.NotEmpty(t, raws, "DirectivesOf returns the declaration's instances")
	var stamps []meta.RawStamp
	assert.MaxAllocs(t, func() { stamps = g.StampsOf(f.store.ID) }, 0, "StampsOf allocates nothing")
	assert.NotEmpty(t, stamps, "StampsOf returns the declaration's stamps")
	assert.MaxAllocs(t, func() { held = g.Holds(f.store.ID) }, 0, "Holds allocates nothing")
	assert.True(t, held, "Holds reports the declaration")
	assert.MaxAllocs(t, func() {
		for range g.Packages() {
		}
	}, 0, "Packages allocates nothing")
	assert.MaxAllocs(t, func() {
		for range g.Directives() {
		}
	}, 0, "Directives allocates nothing")
	assert.MaxAllocs(t, func() {
		for range g.Stamps() {
		}
	}, 0, "Stamps allocates nothing")
}

// BenchmarkSealed measures reads of a sealed graph over the canonical
// workspace, one region for each of its 1,000 packages: a lookup that
// decodes nothing new, and an enumeration by kind once its regions have
// decoded. A read of a built index allocates nothing.
func BenchmarkSealed(b *testing.B) {
	const packages, files, decls = benchPackages, benchFiles, benchDecls
	summaries := canonicalSource()
	id := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()

	b.Run("Lookup", func(b *testing.B) {
		g := store.Sealed(summaries)
		warm(g, id)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var held bool
		for c.Loop() {
			_, held = g.Lookup(id)
		}
		assert.True(b, held, "Lookup finds the declaration")
	})

	b.Run("ByKind", func(b *testing.B) {
		g := store.Sealed(summaries)
		// One warm-up iteration runs at the call site the contract
		// measures. It decodes the regions, and the runtime builds the
		// site's 48-byte cache for converting a declaration to a symbol. A
		// pass at any other call site builds a cache of its own.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.ByKind(symbol.KindStruct) {
				seen++
			}
		}
		assert.Equal(b, seen, packages*files*decls, "ByKind returns every declaration")
	})
}

// sealAllocs is the ceiling of one Sealed call over the canonical
// workspace: the graph, the index, the region slots, the package slots,
// the roster, the counts, the shared array of region numbers, and the
// two maps with their tables.
const sealAllocs = 15

// summarized is a source whose summaries are computed once, so a
// benchmark of Sealed measures the index and not the summaries.
type summarized struct {
	*memory
	infos []store.RegionInfo
}

// Regions returns the summaries computed once.
func (s summarized) Regions() []store.RegionInfo { return s.infos }

// canonicalSource returns a source over the canonical workspace, one
// region for each of its 1,000 packages, with its summaries computed.
func canonicalSource() summarized {
	loaded := coretest.Workspace(benchPackages, benchFiles, benchDecls)
	regions := make([]*store.Region, len(loaded))
	for i, pkg := range loaded {
		regions[i] = &store.Region{Packages: []*node.Package{pkg}}
	}
	src := newMemory(regions...)
	return summarized{memory: src, infos: src.Regions()}
}

// errDecode is the failure the damaged fixtures' sources return.
var errDecode = errors.New("the region's bytes do not decode")

// warm builds the indexes a benchmark or an allocation check reads, so
// what it measures is a read of a built index.
func warm(g *store.Graph, id symbol.Identity) {
	g.Lookup(id)
	for range g.ByKind(symbol.KindStruct) {
	}
	for range g.ByDirective("stub") {
	}
}

// packageAt returns the package of one path a graph enumerates. It
// stops the check, naming the paths the graph enumerates, where none is
// the path.
func packageAt(tb testing.TB, g *store.Graph, path string) *node.Package {
	tb.Helper()

	var paths []string
	for pkg := range g.Packages() {
		if pkg.ID.Package == path {
			return pkg
		}
		paths = append(paths, pkg.ID.Package)
	}
	assert.Contains(tb, paths, path, "the graph enumerates the package")
	return nil
}

// filePaths returns the paths of a package's files, in order.
func filePaths(p *node.Package) []string {
	out := make([]string, 0, len(p.Files))
	for _, f := range p.Files {
		out = append(out, f.Path)
	}
	return out
}

// identities returns the identities of what an enumeration yields, in
// order.
func identities(seq iter.Seq[symbol.Symbol]) []symbol.Identity {
	var out []symbol.Identity
	for s := range seq {
		if decl, names := s.(node.Declaration); names {
			out = append(out, decl.Identity())
		}
	}
	return out
}

// attachment is one subject and what is attached to it.
type attachment[T any] struct {
	subject symbol.Identity
	items   []T
}

// attached returns what an enumeration of attachments yields, in
// order.
func attached[T any](seq iter.Seq2[symbol.Identity, []T]) []attachment[T] {
	var out []attachment[T]
	for subject, items := range seq {
		out = append(out, attachment[T]{subject: subject, items: items})
	}
	return out
}
