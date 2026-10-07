// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The workspace the benchmarks load: a thousand packages of ten
// files each, every file declaring twenty. That is ten thousand files
// and two hundred thousand declarations: the scale the graph contains,
// and not the scale a case reads.
const (
	benchPackages = 1_000
	benchFiles    = 10
	benchDecls    = 20
)

// addBatch is how many packages one iteration of the add benchmark adds
// to a graph of its own.
const addBatch = 1_000

// loadFrontends is how many goroutines a parallel load adds packages
// from.
const loadFrontends = 8

// The allocations of a new graph and of a reader handle, which
// TestGraphAllocs checks in the ordinary run and BenchmarkGraph in a
// benchmark run.
const (
	// newGraphAllocs is the graph.
	newGraphAllocs = 1
	// graphReaderAllocs is the reader handle.
	graphReaderAllocs = 1
)

// The graph the allocation check reads: readPackages packages of
// readFiles files, every file declaring readDecls structs.
const (
	readPackages = 10
	readFiles    = 2
	readDecls    = 3
)

// The ceilings of the graph's writes. Each counts containers whose
// growth depends on where the run's hashes fall, so each allows eight
// standard deviations of the spread 40 runs of one iteration measured
// above their mean.
const (
	// addAllocs is one iteration of the add benchmark: per package the
	// loaded entry, the boxed identity, the sync.Map entry, and the
	// walk's list of the package and its file, which grows twice, then
	// the map's root, and the trie nodes sync.Map adds where two hashes
	// share a prefix, 360 on average with a standard deviation of 10.
	addAllocs = addBatch*5 + 1 + 440
	// freezeAllocs is one seal of the canonical workspace, 562 on
	// average with a standard deviation of 4.4: the index by identity,
	// whose tables the map sizes for the 211,000 declarations and splits
	// where hashes crowd one table, the index by kind with a list per
	// kind, the index by package, and the sealed attachments.
	freezeAllocs = 562 + 36
	// loadFreezeAllocs is one parallel load of the canonical workspace
	// and its seal, 12,944 on average with a standard deviation of 10.3:
	// the adds of 1,000 packages, each with its walk's declaration
	// lists, the seal, and the eight goroutines of the load.
	loadFreezeAllocs = 12_944 + 83
)

// The graph is the run's declarations: what it admits, when it
// seals, and what it returns afterwards.
func TestGraph(t *testing.T) {
	t.Parallel()

	t.Run("AddPackage", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a package the graph then contains", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			_, held := g.Lookup(decl.ID)
			assert.True(t, held, "the graph contains what AddPackage added")
		})

		t.Run("returns DuplicatePackage for a package the graph already contains", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath)),
				"the first add is admitted")

			err := g.AddPackage(coretest.Package(coretest.StorePath))
			assertRefused(t, err, store.DuplicatePackage)
		})

		t.Run("returns FrozenWrite for a write after Freeze", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t)

			err := g.AddPackage(coretest.Package(coretest.StorePath))
			assertRefused(t, err, store.FrozenWrite)
		})

		t.Run("returns FrozenWrite for a write to a sealed graph", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()

			err := g.AddPackage(coretest.Package("svc/index"))
			assertRefused(t, err, store.FrozenWrite)
		})

		t.Run("returns an error for a package naming no identity", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.HasError(t, g.AddPackage(&node.Package{Name: "store"}),
				"a package naming no identity cannot be indexed")
		})

		t.Run("returns an error for a nil package", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, store.New().AddPackage(nil),
				"no package at all is a defect, not a load")
		})

		t.Run("is safe to call concurrently", func(t *testing.T) {
			t.Parallel()

			const frontends = 8
			g := store.New()
			outcomes := history.Concurrently(frontends, time.Minute, func(frontend int) (any, error) {
				shard := strings.Join([]string{coretest.StorePath, string(rune('a' + frontend))}, "/")
				return shard, g.AddPackage(coretest.Package(shard))
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "every parallel add finishes")
				expect.NoError(t, o.Error, "every parallel add is admitted")
			}
			g.Freeze()

			held := 0
			for range g.ByKind(symbol.KindFile) {
				held++
			}
			assert.Equal(t, held, frontends, "no concurrent write is lost")
		})
	})

	t.Run("Sealed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frozen graph", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			assert.True(t, g.Frozen(), "a sealed graph is frozen from the start")
		})

		t.Run("decodes no region", func(t *testing.T) {
			t.Parallel()

			_, src := newSplit().sealed()
			assert.Equal(t, src.decoded(), []int{0, 0, 0}, "Sealed reads the summaries alone")
		})

		t.Run("returns a graph that hands out a reader", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			r, err := g.Reader(store.NewReadSet(), nil)
			assert.NoError(t, err, "a sealed graph hands out readers")
			assert.NotNil(t, r, "and returns one")
		})
	})

	t.Run("Damaged", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a graph New returned", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, coretest.Frozen(t).Damaged(), "a loaded graph decodes nothing")
		})

		t.Run("returns nil where every region decoded whole", func(t *testing.T) {
			t.Parallel()

			g, _ := newSplit().sealed()
			for range g.Packages() { // ranging is what decodes every region
			}
			assert.NoError(t, g.Damaged(), "every region read whole")
		})

		t.Run("returns the failure of a region that did not decode", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			src := newMemory(f.regions...)
			src.fails = map[int]error{1: errDecode}
			g := store.Sealed(src)
			for range g.Packages() { // ranging is what decodes every region
			}
			assert.ErrorIs(t, g.Damaged(), errDecode, "the source's failure is the damage")
		})

		t.Run("returns nil before a read decodes the region that fails", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			src := newMemory(f.regions...)
			src.fails = map[int]error{1: errDecode}
			g := store.Sealed(src)
			g.Lookup(f.store.ID)
			assert.NoError(t, g.Damaged(), "no read decoded the cache package's region")
		})

		t.Run("returns an error for a region the source decoded to nothing", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			infos := newMemory(f.regions...).Regions()
			f.regions[1] = nil
			g := store.Sealed(summarized{memory: newMemory(f.regions...), infos: infos})
			_, held := g.Lookup(coretest.EveryKindID(coretest.CachePath, coretest.StructName, symbol.KindStruct))
			assert.False(t, held, "the region reads as absent")
			assert.HasError(t, g.Damaged(), "and a region decoded to nothing is damage")
		})
	})

	t.Run("Freeze", func(t *testing.T) {
		t.Parallel()

		t.Run("seals the graph", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.False(t, g.Frozen(), "a fresh graph is not sealed")
			g.Freeze()
			assert.True(t, g.Frozen(), "Freeze seals it")
		})

		t.Run("is idempotent", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			before := slices.Collect(g.ByKind(symbol.KindStruct))
			g.Freeze()

			assert.Length(t, slices.Collect(g.ByKind(symbol.KindStruct)), len(before),
				"a second Freeze changes nothing")
		})

		t.Run("skips a declaration the resolution step has not named", func(t *testing.T) {
			t.Parallel()

			unresolved := &node.Struct{Name: "Store"}
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Cache"), unresolved))

			_, held := g.Lookup(unresolved.ID)
			assert.False(t, held,
				"a declaration the resolution step has not named is not indexed under the zero identity")
			assert.Length(t, slices.Collect(g.ByKind(symbol.KindStruct)), 1,
				"and does not enumerate")
		})

		t.Run("indexes every declaration a package contains", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for _, kind := range []symbol.Kind{
				symbol.KindPackage, symbol.KindFile, symbol.KindStruct,
			} {
				assert.NotEmpty(t, slices.Collect(g.ByKind(kind)),
					"the walk visits every kind a package contains")
			}
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration by identity", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, want))

			got, held := g.Lookup(want.ID)
			assert.True(t, held, "Lookup returns the declaration of the identity")
			assert.Equal(t, got, symbol.Symbol(want), "the very declaration", assert.ByIdentity())
		})

		t.Run("reports false for an identity the graph does not contain", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			_, held := g.Lookup(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "an identity the graph does not contain returns nothing")
		})

		t.Run("reports false before Freeze", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, want)),
				"the package is admitted")

			_, held := g.Lookup(want.ID)
			assert.False(t, held, "the index is built at Freeze, not before")
		})
	})

	t.Run("Holds", func(t *testing.T) {
		t.Parallel()

		want := coretest.Struct(coretest.StorePath, "Store")
		pkg := coretest.Package(coretest.StorePath, want)

		t.Run("reports true for a declaration", func(t *testing.T) {
			t.Parallel()

			assert.True(t, coretest.Frozen(t, pkg).Holds(want.ID), "the graph contains the declaration")
		})

		t.Run("reports true for the package of a declaration", func(t *testing.T) {
			t.Parallel()

			assert.True(t, coretest.Frozen(t, pkg).Holds(pkg.ID),
				"the graph contains the package too, which Lookup alone would call dangling")
		})

		t.Run("reports false for an identity the graph does not contain", func(t *testing.T) {
			t.Parallel()

			assert.False(t, coretest.Frozen(t, pkg).Holds(coretest.Struct(coretest.CachePath, "Cache").ID),
				"an identity the graph does not contain is not held")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		t.Run("enumerates the roster in identity order", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath),
				coretest.Package(coretest.CachePath),
			)
			var paths []string
			for pkg := range g.Packages() {
				paths = append(paths, pkg.ID.Package)
			}
			assert.Equal(t, paths, []string{coretest.CachePath, coretest.StorePath},
				"the seal's own order, which is the same across runs")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package that contains a declaration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			pkg, held := g.PackageOf(decl.ID)
			assert.True(t, held, "PackageOf returns the package of the declaration")
			assert.Equal(t, pkg.ID, coretest.PackageID(coretest.StorePath),
				"the package its identity names")
		})

		t.Run("reports false for an identity the graph does not contain", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			_, held := g.PackageOf(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "an identity the graph does not contain has no package")
		})

		t.Run("reports false before Freeze", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package is admitted")

			_, held := g.PackageOf(decl.ID)
			assert.False(t, held, "the index is built at Freeze, not before")
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			assert.Equal(t, coretest.Names(t, slices.Collect(g.ByKind(symbol.KindStruct))),
				[]string{"Cache", "Store"}, "ByKind returns every declaration of the kind")
		})

		t.Run("returns one order however the packages arrived", func(t *testing.T) {
			t.Parallel()

			first := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))
			second := coretest.Frozen(t,
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")),
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))

			assert.Equal(t,
				coretest.Names(t, slices.Collect(first.ByKind(symbol.KindStruct))),
				coretest.Names(t, slices.Collect(second.ByKind(symbol.KindStruct))),
				"the order is the graph's own, not the load order")
		})

		t.Run("returns a package's declarations in identity order", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			assert.Equal(t, coretest.Names(t, slices.Collect(g.ByKind(symbol.KindStruct))),
				[]string{"Cache", "Store"}, "the package declares Store first, and Cache sorts first")
		})

		t.Run("returns nothing before Freeze", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			loaded := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store"))
			assert.NoError(t, g.AddPackage(loaded), "the package is admitted")

			assert.Empty(t, slices.Collect(g.ByKind(symbol.KindStruct)),
				"an untracked read before the seal returns nothing rather than a partial result")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			seen := 0
			for range g.ByKind(symbol.KindStruct) {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("returns UnfrozenRead before Freeze", func(t *testing.T) {
			t.Parallel()

			_, err := store.New().Reader(store.NewReadSet(), nil)
			assertRefused(t, err, store.UnfrozenRead)
		})

		t.Run("returns an error without a read set", func(t *testing.T) {
			t.Parallel()

			_, err := coretest.Frozen(t).Reader(nil, nil)
			assert.HasError(t, err,
				"a read with nowhere to record would be an untracked read")
		})

		t.Run("returns a reader once the graph is sealed", func(t *testing.T) {
			t.Parallel()

			r, err := coretest.Frozen(t).Reader(store.NewReadSet(), nil)
			assert.NoError(t, err, "a sealed graph hands out readers")
			assert.NotNil(t, r, "and returns one")
		})
	})
}

// TestGraphAllocs checks the ceiling of sealing the summaries of the
// canonical workspace, the cost of a warm run before it decodes
// anything, the ceilings of a seal and of a parallel load of the
// canonical workspace, the adds of a thousand packages, and the
// allocations of a new graph, of a reader and of the untracked reads,
// in the ordinary run, which runs no benchmark. The seal and the adds
// each take a graph built outside the count. The check runs alone,
// because the count includes every goroutine's allocations.
func TestGraphAllocs(t *testing.T) {
	src := canonicalSource()
	var sealed *store.Graph
	assert.MaxAllocs(t, func() { sealed = store.Sealed(src) }, sealAllocs,
		"Sealed allocates its index of the summaries")
	assert.True(t, sealed.Frozen(), "Sealed returns a frozen graph")

	loaded := coretest.Workspace(benchPackages, benchFiles, benchDecls)
	assert.MaxAllocs(t, func() { sealed = parallelLoad(t, loaded) }, loadFreezeAllocs,
		"a parallel load of the canonical workspace and its seal allocate within their ceiling")
	assert.True(t, sealed.Frozen(), "the load seals the graph")

	assert.MaxAllocsWithSetup(t, func() *store.Graph { sealed = addedAll(t, loaded); return sealed },
		(*store.Graph).Freeze, freezeAllocs, "Freeze allocates the indexes of the canonical workspace")
	assert.True(t, sealed.Frozen(), "Freeze seals the graph")

	pool := emptyPackages(addBatch)
	var added error
	assert.MaxAllocsWithSetup(t, store.New, func(g *store.Graph) {
		for _, pkg := range pool {
			added = cmp.Or(added, g.AddPackage(pkg))
		}
	}, addAllocs, "AddPackage allocates the entries of a thousand empty packages")
	assert.NoError(t, added, "every package is added")

	var built *store.Graph
	assert.MaxAllocs(t, func() { built = store.New() }, newGraphAllocs, "New allocates the graph")
	assert.False(t, built.Frozen(), "New returns a graph that is not frozen")

	g, hit := readGraph(t)
	reads := store.NewReadSet()
	var err error
	assert.MaxAllocs(t, func() { _, err = g.Reader(reads, nil) }, graphReaderAllocs, "Reader allocates the handle")
	assert.NoError(t, err, "the frozen graph hands out a reader")
	var contained, frozen, holds bool
	assert.MaxAllocs(t, func() {
		_, contained = g.PackageOf(hit)
		frozen, holds, err = g.Frozen(), g.Holds(hit), g.Damaged()
	}, 0, "PackageOf, Frozen, Holds and Damaged allocate nothing")
	expect.True(t, contained, "PackageOf finds the loaded declaration's package")
	expect.True(t, frozen, "Frozen reports the sealed graph")
	expect.True(t, holds, "Holds reports the loaded declaration")
	expect.NoError(t, err, "a loaded graph decodes nothing, so nothing is damaged")
	var held bool
	assert.MaxAllocs(t, func() { _, held = g.Lookup(hit) }, 0, "Lookup allocates nothing")
	assert.True(t, held, "Lookup finds the loaded declaration")
	n := 0
	assert.MaxAllocs(t, func() {
		n = 0
		for range g.ByKind(symbol.KindStruct) {
			n++
		}
	}, 0, "a range over ByKind allocates nothing")
	assert.Equal(t, n, readPackages*readFiles*readDecls, "ByKind enumerates every struct")
	assert.MaxAllocs(t, func() {
		n = 0
		for range g.Packages() {
			n++
		}
	}, 0, "a range over Packages allocates nothing")
	assert.Equal(t, n, readPackages, "Packages enumerates every package")
}

// BenchmarkGraph measures the graph, which is loaded once per run and
// read from for the rest of it: the adds the frontends make, the seal,
// and the untracked reads the dispatcher makes.
func BenchmarkGraph(b *testing.B) {
	const packages, files, decls = benchPackages, benchFiles, benchDecls
	loaded := coretest.Workspace(packages, files, decls)

	b.Run("Sealed", func(b *testing.B) {
		src := canonicalSource()
		c := bench.Start(b).MaxAllocs(sealAllocs)
		defer c.End()
		var got *store.Graph
		for c.Loop() {
			got = store.Sealed(src)
		}
		assert.True(b, got.Frozen(), "Sealed returns a frozen graph")
	})

	b.Run("Freeze", func(b *testing.B) {
		b.Run("a loaded workspace", func(b *testing.B) {
			// One workspace loads into a fresh graph per iteration: a
			// package is not mutated by an add, so the fixture is built
			// once and the seal alone is measured.
			var g *store.Graph
			load := func() { g = addedAll(b, loaded) }
			c := bench.Start(b).MaxAllocs(freezeAllocs)
			defer c.End()
			for c.Loop() {
				c.Excluding(load)
				g.Freeze()
			}
			assert.True(b, g.Frozen(), "the graph is sealed")
		})

		b.Run("after a parallel load", func(b *testing.B) {
			// The number that matters for a large workspace: the whole
			// pipeline, packages added from parallel goroutines the way
			// frontends load, then the seal.
			c := bench.Start(b).MaxAllocs(loadFreezeAllocs)
			defer c.End()
			var g *store.Graph
			for c.Loop() {
				g = parallelLoad(b, loaded)
			}
			assert.True(b, g.Frozen(), "the graph is sealed")
		})
	})

	b.Run("AddPackage", func(b *testing.B) {
		b.Run("a thousand empty packages", func(b *testing.B) {
			pool := emptyPackages(addBatch)
			var g *store.Graph
			fresh := func() { g = store.New() }
			c := bench.Start(b).MaxAllocs(addAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				for _, pkg := range pool {
					if err = g.AddPackage(pkg); err != nil {
						break
					}
				}
			}
			assert.NoError(b, err, "every package is added")
		})
	})

	b.Run("Lookup", func(b *testing.B) {
		g := coretest.Frozen(b, loaded...)
		id := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = g.Lookup(id)
		}
		assert.True(b, held, "Lookup returns the declaration")
	})

	b.Run("ByKind", func(b *testing.B) {
		g := coretest.Frozen(b, loaded...)
		// One warm-up iteration runs at the call site the contract
		// measures, where the runtime builds the site's 48-byte cache for
		// converting a declaration to a symbol. A pass at any other call
		// site builds a cache of its own.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.ByKind(symbol.KindStruct) {
				seen++
			}
		}
		assert.Equal(b, seen, packages*files*decls, "ByKind returns every struct")
	})

	b.Run("New", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newGraphAllocs)
		defer c.End()
		var g *store.Graph
		for c.Loop() {
			g = store.New()
		}
		assert.False(b, g.Frozen(), "New returns a graph that is not frozen")
	})

	g := coretest.Frozen(b, loaded...)
	hit := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()

	b.Run("Reader", func(b *testing.B) {
		reads := store.NewReadSet()
		c := bench.Start(b).MaxAllocs(graphReaderAllocs)
		defer c.End()
		var (
			r   *store.Reader
			err error
		)
		for c.Loop() {
			r, err = g.Reader(reads, nil)
		}
		assert.NoError(b, err, "the frozen graph hands out a reader")
		assert.NotNil(b, r, "the reader")
	})

	b.Run("Frozen", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		frozen := false
		for c.Loop() {
			frozen = g.Frozen()
		}
		assert.True(b, frozen, "Frozen reports true after Freeze")
	})

	b.Run("Holds", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			held = g.Holds(hit)
		}
		assert.True(b, held, "Holds reports true for a loaded declaration")
	})

	b.Run("PackageOf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = g.PackageOf(hit)
		}
		assert.True(b, held, "PackageOf returns the declaration's package")
	})

	b.Run("Damaged", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = g.Damaged()
		}
		assert.NoError(b, err, "a loaded graph decodes nothing, so nothing is damaged")
	})

	b.Run("Packages", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.Packages() {
				seen++
			}
		}
		assert.Equal(b, seen, packages, "Packages returns every package")
	})
}

// BenchmarkGraphParallel measures adds from GOMAXPROCS goroutines, the
// claim that two frontends adding two packages do not contend. It
// states no ceiling: the graph's maps grow by a number of steps that
// b.N decides, so BenchmarkGraph states the allocation contract of a
// batch of adds.
func BenchmarkGraphParallel(b *testing.B) {
	b.Run("AddPackage", func(b *testing.B) {
		// The pool is built before the measurement starts and handed out
		// by index, so every add writes a distinct package.
		pool := make([]*node.Package, b.N)
		for i := range pool {
			pool[i] = coretest.Package(coretest.StorePath + "/" + strconv.Itoa(i))
		}
		g := store.New()
		var next atomic.Int64
		c := bench.Start(b)
		defer c.End()
		c.RunParallel(func(pb *bench.PB) {
			for pb.Next() {
				expect.NoError(b, g.AddPackage(pool[next.Add(1)-1]), "every parallel add is admitted")
			}
		})
	})
}

// addedAll returns a graph with every package added and not frozen.
func addedAll(tb assert.TB, pkgs []*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	assert.Total(tb, g.AddPackage, pkgs, "the package is added")
	return g
}

// emptyPackages returns n packages without files, each under a path of
// its own.
func emptyPackages(n int) []*node.Package {
	out := make([]*node.Package, 0, n)
	for i := range n {
		out = append(out, coretest.Package(coretest.StorePath+"/"+strconv.Itoa(i)))
	}
	return out
}

// parallelLoad adds every package from loadFrontends goroutines, the way
// frontends load, and seals the graph.
func parallelLoad(tb assert.TB, loaded []*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	var wg sync.WaitGroup
	for shard := range slices.Chunk(loaded, (len(loaded)+loadFrontends-1)/loadFrontends) {
		wg.Go(func() { expect.Total(tb, g.AddPackage, shard, "every package is added") })
	}
	wg.Wait()
	g.Freeze()
	return g
}

// readGraph returns a frozen graph of readPackages packages, and the
// identity of a declaration it loaded.
func readGraph(tb assert.TB) (*store.Graph, symbol.Identity) {
	tb.Helper()

	g := coretest.Frozen(tb, coretest.Workspace(readPackages, readFiles, readDecls)...)
	return g, coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()
}

// assertRefused fails unless err is a refusal under want.
func assertRefused(t *testing.T, err error, want diag.Code) {
	t.Helper()

	refused := assert.ErrorAs[*store.RefusedError](t, err,
		"the graph refuses with its typed error")
	assert.Equal(t, refused.Code, want,
		"under the code consumers script against")
}
