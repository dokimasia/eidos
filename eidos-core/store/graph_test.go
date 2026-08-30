// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

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

			if _, held := g.Lookup(decl.ID); !held {
				t.Fatalf("Lookup(%v) = false, want the added declaration", decl.ID)
			}
		})

		t.Run("refuses a package the graph already holds", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			if err := g.AddPackage(coretest.Package(coretest.StorePath)); err != nil {
				t.Fatalf("AddPackage: unexpected error: %v", err)
			}

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
			if err := g.AddPackage(&node.Package{Name: "store"}); err == nil {
				t.Fatal("AddPackage without an identity: error = nil, want non-nil")
			}
		})

		t.Run("refuses no package at all", func(t *testing.T) {
			t.Parallel()

			if err := store.New().AddPackage(nil); err == nil {
				t.Fatal("AddPackage(nil): error = nil, want non-nil")
			}
		})

		t.Run("is safe to call concurrently", func(t *testing.T) {
			t.Parallel()

			const frontends = 8
			g := store.New()
			var wg sync.WaitGroup
			for frontend := range frontends {
				wg.Go(func() {
					shard := strings.Join([]string{coretest.StorePath, string(rune('a' + frontend))}, "/")
					pkg := coretest.Package(shard)
					if err := g.AddPackage(pkg); err != nil {
						t.Errorf("AddPackage: unexpected error: %v", err)
					}
				})
			}
			wg.Wait()
			g.Freeze()

			held := 0
			for range g.ByKind(symbol.KindFile) {
				held++
			}
			if held != frontends {
				t.Fatalf("the graph holds %d files, want %d: a write was lost", held, frontends)
			}
		})
	})

	t.Run("Freeze", func(t *testing.T) {
		t.Parallel()

		t.Run("seals the graph", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			if g.Frozen() {
				t.Fatal("Frozen() = true before Freeze, want false")
			}
			g.Freeze()
			if !g.Frozen() {
				t.Fatal("Frozen() = false after Freeze, want true")
			}
		})

		t.Run("is idempotent", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			before := slices.Collect(g.ByKind(symbol.KindStruct))
			g.Freeze()

			if after := slices.Collect(g.ByKind(symbol.KindStruct)); len(after) != len(before) {
				t.Fatalf("a second Freeze answered %d structs, want %d", len(after), len(before))
			}
		})

		t.Run("skips a declaration the resolution step has not named", func(t *testing.T) {
			t.Parallel()

			unresolved := &node.Struct{Name: "Store"}
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Cache"), unresolved))

			if _, held := g.Lookup(unresolved.ID); held {
				t.Fatal("a declaration carrying the zero identity was indexed under it, " +
					"which every other unnamed declaration would answer too")
			}
			if got := len(slices.Collect(g.ByKind(symbol.KindStruct))); got != 1 {
				t.Fatalf("ByKind(Struct) answered %d declarations, want 1", got)
			}
		})

		t.Run("indexes every declaration a package holds", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store"), coretest.Struct(coretest.StorePath, "Cache")))

			for _, kind := range []symbol.Kind{
				symbol.KindPackage, symbol.KindFile, symbol.KindStruct,
			} {
				if len(slices.Collect(g.ByKind(kind))) == 0 {
					t.Fatalf("ByKind(%v) is empty: the walk did not reach that kind", kind)
				}
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
			if !held {
				t.Fatalf("Lookup(%v) = false, want the declaration", want.ID)
			}
			if got != symbol.Symbol(want) {
				t.Fatalf("Lookup(%v) = %v, want %v", want.ID, got, want)
			}
		})

		t.Run("answers false for an identity nothing holds", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			if _, held := g.Lookup(coretest.Struct(coretest.CachePath, "Cache").ID); held {
				t.Fatal("Lookup of an unheld identity = true, want false")
			}
		})

		t.Run("answers false before Freeze", func(t *testing.T) {
			t.Parallel()

			want := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			if err := g.AddPackage(coretest.Package(coretest.StorePath, want)); err != nil {
				t.Fatalf("AddPackage: unexpected error: %v", err)
			}

			if _, held := g.Lookup(want.ID); held {
				t.Fatal("Lookup before Freeze = true, want false: the index is built at Freeze")
			}
		})
	})

	t.Run("ByKind", func(t *testing.T) {
		t.Parallel()

		t.Run("answers every declaration of one kind", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))

			got := coretest.Names(t, slices.Collect(g.ByKind(symbol.KindStruct)))
			if want := []string{"Cache", "Store"}; !slices.Equal(got, want) {
				t.Fatalf("ByKind(Struct) = %v, want %v", got, want)
			}
		})

		t.Run("answers one order however the packages arrived", func(t *testing.T) {
			t.Parallel()

			first := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")))
			second := coretest.Frozen(t,
				coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")),
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))

			one := coretest.Names(t, slices.Collect(first.ByKind(symbol.KindStruct)))
			other := coretest.Names(t, slices.Collect(second.ByKind(symbol.KindStruct)))
			if !slices.Equal(one, other) {
				t.Fatalf("ByKind answered %v and %v: the order follows the load order", one, other)
			}
		})

		t.Run("answers nothing before Freeze", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			loaded := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store"))
			if err := g.AddPackage(loaded); err != nil {
				t.Fatalf("AddPackage: unexpected error: %v", err)
			}

			if got := slices.Collect(g.ByKind(symbol.KindStruct)); len(got) != 0 {
				t.Fatalf("ByKind before Freeze answered %d declarations, want 0", len(got))
			}
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
			if seen != 1 {
				t.Fatalf("ByKind yielded %d declarations after a break, want 1", seen)
			}
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

			if _, err := coretest.Frozen(t).Reader(nil, nil); err == nil {
				t.Fatal("Reader(nil, nil): error = nil, want non-nil: " +
					"a read with nowhere to record is an untracked read")
			}
		})

		t.Run("answers a reader once the graph is sealed", func(t *testing.T) {
			t.Parallel()

			r, err := coretest.Frozen(t).Reader(store.NewReadSet(), nil)
			if err != nil {
				t.Fatalf("Reader: unexpected error: %v", err)
			}
			if r == nil {
				t.Fatal("Reader answered no reader and no error")
			}
		})
	})
}

// assertRefused fails unless err is a refusal under want.
func assertRefused(t *testing.T, err error, want diag.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want a refusal under %v", want)
	}
	var refused *store.RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v (%T), want a *store.RefusedError", err, err)
	}
	if refused.Code != want {
		t.Fatalf("refused under %v, want %v", refused.Code, want)
	}
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
