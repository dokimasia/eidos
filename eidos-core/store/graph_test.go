// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The graph is the run's declarations: what it admits, when it
// seals, and what it answers afterwards.
func TestGraph(t *testing.T) {
	t.Parallel()

	t.Run("AddPackage", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a package the graph then holds", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			_, held := g.Lookup(decl.ID)
			assert.True(t, held, "the graph holds what AddPackage added")
		})

		t.Run("refuses a package the graph already holds", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath)),
				"the first add is admitted")

			err := g.AddPackage(coretest.Package(coretest.StorePath))
			assertRefused(t, err, store.DuplicatePackage)
		})

		t.Run("refuses a write after Freeze", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t)

			err := g.AddPackage(coretest.Package(coretest.StorePath))
			assertRefused(t, err, store.FrozenWrite)
		})

		t.Run("refuses a package naming no identity", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.HasError(t, g.AddPackage(&node.Package{Name: "store"}),
				"a package naming no identity cannot be indexed")
		})

		t.Run("refuses no package at all", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, store.New().AddPackage(nil),
				"no package at all is a defect, not a load")
		})

		t.Run("is safe to call concurrently", func(t *testing.T) {
			t.Parallel()

			const frontends = 8
			g := store.New()
			var wg sync.WaitGroup
			for frontend := range frontends {
				wg.Go(func() {
					shard := strings.Join([]string{coretest.StorePath, string(rune('a' + frontend))}, "/")
					expect.NoError(t, g.AddPackage(coretest.Package(shard)),
						"every parallel add is admitted")
				})
			}
			wg.Wait()
			g.Freeze()

			held := 0
			for range g.ByKind(symbol.KindFile) {
				held++
			}
			assert.Equal(t, held, frontends, "no concurrent write is lost")
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

		t.Run("indexes every declaration a package holds", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for _, kind := range []symbol.Kind{
				symbol.KindPackage, symbol.KindFile, symbol.KindStruct,
			} {
				assert.NotEmpty(t, slices.Collect(g.ByKind(kind)),
					"the walk reaches every kind a package holds")
			}
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("answers a declaration by identity", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, want))

			got, held := g.Lookup(want.ID)
			assert.True(t, held, "Lookup answers a held identity")
			assert.True(t, got == symbol.Symbol(want), "with the very declaration")
		})

		t.Run("answers false for an identity nothing holds", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			_, held := g.Lookup(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "an identity nothing holds answers nothing")
		})

		t.Run("answers false before Freeze", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, want)),
				"the package is admitted")

			_, held := g.Lookup(want.ID)
			assert.False(t, held, "the index is built at Freeze, not before")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the package holding a declaration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			pkg, held := g.PackageOf(decl.ID)
			assert.True(t, held, "PackageOf answers a held declaration")
			assert.Equal(t, pkg.ID, coretest.PackageID(coretest.StorePath),
				"with the package its identity names")
		})

		t.Run("answers false for an identity nothing holds", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			_, held := g.PackageOf(coretest.Struct(coretest.CachePath, "Cache").ID)
			assert.False(t, held, "an identity nothing holds owns nothing")
		})

		t.Run("answers false before Freeze", func(t *testing.T) {
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

		t.Run("answers every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			assert.Equal(t, coretest.Names(t, slices.Collect(g.ByKind(symbol.KindStruct))),
				[]string{"Cache", "Store"}, "ByKind answers every declaration of the kind")
		})

		t.Run("answers one order however the packages arrived", func(t *testing.T) {
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

		t.Run("answers nothing before Freeze", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			loaded := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store"))
			assert.NoError(t, g.AddPackage(loaded), "the package is admitted")

			assert.Empty(t, slices.Collect(g.ByKind(symbol.KindStruct)),
				"an untracked read before the seal answers nothing rather than a partial result")
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

		t.Run("is refused before Freeze", func(t *testing.T) {
			t.Parallel()

			_, err := store.New().Reader(store.NewReadSet(), nil)
			assertRefused(t, err, store.UnfrozenRead)
		})

		t.Run("is refused without a read set", func(t *testing.T) {
			t.Parallel()

			_, err := coretest.Frozen(t).Reader(nil, nil)
			assert.HasError(t, err,
				"a read with nowhere to record would be an untracked read")
		})

		t.Run("answers a reader once the graph is sealed", func(t *testing.T) {
			t.Parallel()

			r, err := coretest.Frozen(t).Reader(store.NewReadSet(), nil)
			assert.NoError(t, err, "a sealed graph hands out readers")
			assert.NotNil(t, r, "and answers one")
		})
	})
}

// assertRefused fails unless err is a refusal under want.
func assertRefused(t *testing.T, err error, want diag.Code) {
	t.Helper()

	refused := assert.ErrorAs[*store.RefusedError](t, err,
		"the graph refuses with its typed error")
	assert.Equal(t, refused.Code, want,
		"under the code consumers script against")
}

// The graph is loaded once per run and read from for the rest of
// it, so the cost that matters is the seal and the untracked read
// the dispatcher makes.
func BenchmarkGraph(b *testing.B) {
	const packages, files, decls = benchPackages, benchFiles, benchDecls

	b.Run("Freeze", func(b *testing.B) {
		b.ReportAllocs()

		// One workspace loads into a fresh graph per iteration: a
		// package is not mutated by an add, so the fixture is built
		// once and only the seal is timed.
		loaded := coretest.Workspace(packages, files, decls)
		for b.Loop() {
			b.StopTimer()
			g := store.New()
			for _, pkg := range loaded {
				if err := g.AddPackage(pkg); err != nil {
					b.Fatalf("AddPackage: unexpected error: %v", err)
				}
			}
			b.StartTimer()

			g.Freeze()
		}
	})

	b.Run("load then freeze", func(b *testing.B) {
		b.ReportAllocs()

		// The number that matters for a large workspace: the whole
		// pipeline, packages added from parallel goroutines the way
		// frontends load, then the seal.
		const frontends = 8
		loaded := coretest.Workspace(packages, files, decls)
		for b.Loop() {
			g := store.New()
			var wg sync.WaitGroup
			for shard := range slices.Chunk(loaded, (len(loaded)+frontends-1)/frontends) {
				wg.Go(func() {
					for _, pkg := range shard {
						if err := g.AddPackage(pkg); err != nil {
							b.Errorf("AddPackage: unexpected error: %v", err)
						}
					}
				})
			}
			wg.Wait()
			g.Freeze()
		}
	})

	b.Run("AddPackage", func(b *testing.B) {
		b.ReportAllocs()

		// The packages are built with the timer stopped, a chunk at a
		// time: what is measured is the write, not the fixture.
		const chunk = 4096
		var (
			pool  []*node.Package
			next  int
			built int
		)
		g := store.New()
		for b.Loop() {
			if next == len(pool) {
				b.StopTimer()
				pool = pool[:0]
				for range chunk {
					pool = append(pool, coretest.Package(
						coretest.StorePath+"/"+strconv.Itoa(built),
					))
					built++
				}
				next = 0
				b.StartTimer()
			}
			if err := g.AddPackage(pool[next]); err != nil {
				b.Fatalf("AddPackage: unexpected error: %v", err)
			}
			next++
		}
	})

	b.Run("AddPackage in parallel", func(b *testing.B) {
		b.ReportAllocs()

		// The claim under test is that two frontends adding two
		// packages do not contend. The pool is built before the
		// timer starts and handed out by index, so every add writes
		// a distinct package.
		pool := make([]*node.Package, b.N)
		for i := range pool {
			pool[i] = coretest.Package(coretest.StorePath + "/" + strconv.Itoa(i))
		}
		g := store.New()
		var next atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				if err := g.AddPackage(pool[next.Add(1)-1]); err != nil {
					b.Errorf("AddPackage: unexpected error: %v", err)
				}
			}
		})
	})

	b.Run("Lookup", func(b *testing.B) {
		b.ReportAllocs()
		g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)
		id := coretest.Struct(coretest.StorePath+"/0", "Decl0_0").Identity()

		for b.Loop() {
			if _, held := g.Lookup(id); !held {
				b.Fatalf("Lookup(%v) = false, want the declaration", id)
			}
		}
	})

	b.Run("ByKind", func(b *testing.B) {
		b.ReportAllocs()
		g := coretest.Frozen(b, coretest.Workspace(packages, files, decls)...)

		for b.Loop() {
			seen := 0
			for range g.ByKind(symbol.KindStruct) {
				seen++
			}
			if seen != packages*files*decls {
				b.Fatalf("ByKind answered %d declarations, want %d", seen, packages*files*decls)
			}
		}
	})
}

// The workspace the benchmarks load: a thousand packages of ten
// files each, every file holding twenty declarations. That is ten
// thousand files and two hundred thousand declarations, which is the
// scale the graph has to hold rather than the scale a case reads.
const (
	benchPackages = 1_000
	benchFiles    = 10
	benchDecls    = 20
)
