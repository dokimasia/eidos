// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The directive spellings and the fact keys the log fixtures read.
const (
	logDirective  directive.Name = "stub"
	markDirective directive.Name = "mark"
	logKey        meta.KeyName   = "t.flag"
	roleKey       meta.KeyName   = "t.role"
)

// grains is every enumeration of a read set, the five grains in one
// value a case compares.
type grains struct {
	identities []symbol.Identity
	packages   []symbol.Identity
	kinds      []symbol.Kind
	facts      []meta.FactRef
	directives []directive.Name
}

// The log keeps what each invocation read while the dispatcher reuses
// one set, so an entry loads back exactly the set it recorded.
func TestReadLog(t *testing.T) {
	t.Parallel()

	t.Run("Append", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each entry's index in order", func(t *testing.T) {
			t.Parallel()

			var log store.ReadLog
			assert.Equal(t, []int{log.Append(store.NewReadSet()), log.Append(store.NewReadSet())}, []int{0, 1},
				"the entries count from zero")
		})

		t.Run("leaves the set unchanged", func(t *testing.T) {
			t.Parallel()

			reads := everyGrain(t)
			before := grainsOf(reads)
			var log store.ReadLog
			log.Append(reads)
			assert.Equal(t, grainsOf(reads), before, "the set keeps its edges")
		})

		t.Run("keeps the edges of a set a log loaded", func(t *testing.T) {
			t.Parallel()

			var first, second store.ReadLog
			loaded := store.NewReadSet()
			first.Load(first.Append(everyGrain(t)), loaded)
			again := store.NewReadSet()
			second.Load(second.Append(loaded), again)
			assert.Equal(t, grainsOf(again), grainsOf(everyGrain(t)), "the second log keeps what the first loaded")
		})
	})

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("records the edges of every grain", func(t *testing.T) {
			t.Parallel()

			reads := everyGrain(t)
			var log store.ReadLog
			at := log.Append(reads)
			loaded := store.NewReadSet()
			log.Load(at, loaded)
			assert.Equal(t, grainsOf(loaded), grainsOf(reads), "the loaded set returns what the recorded set did")
		})

		t.Run("returns the edges of an entry past four edges in each grain's order", func(t *testing.T) {
			t.Parallel()

			reads := pastFourEdges(t)
			var log store.ReadLog
			loaded := store.NewReadSet()
			log.Load(log.Append(reads), loaded)
			assert.Equal(t, grainsOf(loaded), grainsOf(reads), "the loaded set returns the recorded set's order")
		})

		t.Run("records the edges of its own entry alone", func(t *testing.T) {
			t.Parallel()

			alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			var log store.ReadLog
			r.Lookup(alpha.ID)
			first := log.Append(reads)
			reads.Reset()
			r.Lookup(omega.ID)
			second := log.Append(reads)
			loaded := store.NewReadSet()
			log.Load(second, loaded)
			later := slices.Collect(loaded.Identities())
			log.Load(first, loaded)
			assert.Equal(t, [][]symbol.Identity{slices.Collect(loaded.Identities()), later},
				[][]symbol.Identity{{alpha.ID}, {omega.ID}}, "each entry loads its own read")
		})

		t.Run("drops the edges the set held", func(t *testing.T) {
			t.Parallel()

			reads := everyGrain(t)
			var log store.ReadLog
			at := log.Append(store.NewReadSet())
			log.Load(at, reads)
			assert.Equal(t, grainsOf(reads), grainsOf(store.NewReadSet()), "an empty entry leaves an empty set")
		})

		t.Run("panics for an index outside the log", func(t *testing.T) {
			t.Parallel()

			var log store.ReadLog
			log.Append(store.NewReadSet())
			assert.Panics(t, func() { log.Load(1, store.NewReadSet()) }, "the log has one entry")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("returns index zero for the next entry", func(t *testing.T) {
			t.Parallel()

			var log store.ReadLog
			log.Append(everyGrain(t))
			log.Append(everyGrain(t))
			log.Reset()
			assert.Equal(t, log.Append(store.NewReadSet()), 0, "the entries count from zero again")
		})

		t.Run("loads the edges of the entry appended after it", func(t *testing.T) {
			t.Parallel()

			var log store.ReadLog
			log.Append(everyGrain(t))
			log.Reset()
			at := log.Append(store.NewReadSet())
			loaded := everyGrain(t)
			log.Load(at, loaded)
			assert.Equal(t, grainsOf(loaded), grainsOf(store.NewReadSet()),
				"no edge of the entry before the reset loads")
		})
	})
}

// Loading an entry into a set that once had as many edges allocates
// nothing, which is what lets a journal hand each record through one
// set, and a log reset and filled again allocates nothing within its
// earlier use. The check runs alone, because AllocsPerRun counts every
// goroutine's allocations and refuses to run beside parallel tests.
func TestReadLogZeroAlloc(t *testing.T) {
	reads := everyGrain(t)
	var log store.ReadLog
	at := log.Append(reads)
	loaded := store.NewReadSet()
	log.Load(at, loaded)
	assert.MaxAllocs(t, func() {
		log.Load(at, loaded)
		if loaded.Len() != reads.Len() {
			t.Fatal("Load records another number of edges")
		}
	}, 0, "Load allocates nothing into a set that held the entry")
	assert.MaxAllocs(t, func() {
		log.Reset()
		if log.Append(reads) != 0 {
			t.Fatal("Append after Reset returns another index than zero")
		}
	}, 0, "Reset and Append allocate nothing within the log's earlier use")

	one := store.NewReadSet()
	one.RecordFact(coretest.Struct(coretest.StorePath, "Store").ID, logKey)
	var small store.ReadLog
	entry := small.Append(one)
	fresh := store.NewReadSet()
	assert.MaxAllocs(t, func() {
		small.Load(entry, fresh)
		if fresh.Len() != 1 {
			t.Fatal("Load records another number of edges")
		}
	}, 0, "Load allocates nothing for an entry of at most four edges, which a set keeps in place")
}

// BenchmarkReadLog measures a set of five edges, one of each grain,
// entering the log, loading back into one set, and leaving the log at a
// reset. Each measured call works within storage an earlier entry grew,
// the way a dispatcher reuses its log.
func BenchmarkReadLog(b *testing.B) {
	reads := everyGrain(b)

	b.Run("Append", func(b *testing.B) {
		var log store.ReadLog
		log.Append(reads)
		reset := log.Reset
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := -1
		for c.Loop() {
			c.Excluding(reset)
			got = log.Append(reads)
		}
		if got != 0 {
			b.Fatalf("Append returns entry %d", got)
		}
	})

	b.Run("Reset", func(b *testing.B) {
		var log store.ReadLog
		log.Append(reads)
		fill := func() { log.Append(reads) }
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			c.Excluding(fill)
			log.Reset()
		}
		if got := log.Append(reads); got != 0 {
			b.Fatalf("the reset log appends entry %d", got)
		}
	})

	b.Run("Load", func(b *testing.B) {
		var log store.ReadLog
		at := log.Append(reads)
		loaded := store.NewReadSet()
		log.Load(at, loaded)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			log.Load(at, loaded)
		}
		if loaded.Len() != reads.Len() {
			b.Fatalf("Load records %d edges", loaded.Len())
		}
	})
}

// everyGrain returns a read set with one edge of every grain.
func everyGrain(tb assert.TB) *store.ReadSet {
	tb.Helper()

	decl := coretest.Struct(coretest.StorePath, "Store")
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, decl)), "the fixture package is admitted")
	assert.NoError(tb, g.AttachDirectives(decl.ID, []directive.Raw{{Name: logDirective}}), "the directive attaches")
	g.Freeze()
	reads := store.NewReadSet()
	r, err := g.Reader(reads, nil)
	assert.NoError(tb, err, "a sealed graph hands out a reader")
	r.PackageOf(decl.ID)
	for range r.ByKind(symbol.KindStruct) { // ranging is what records the edges
	}
	for range r.ByDirective(logDirective) { // ranging is what records the edges
	}
	reads.RecordFact(decl.ID, logKey)
	return reads
}

// pastFourEdges returns a read set past four edges with more than one
// edge of every grain, each grain's edges recorded against its order.
func pastFourEdges(tb assert.TB) *store.ReadSet {
	tb.Helper()

	alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")
	cache := coretest.Struct(coretest.CachePath, "Cache")
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, alpha, omega)),
		"the store package is admitted")
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.CachePath, cache)), "the cache package is admitted")
	assert.NoError(tb, g.AttachDirectives(omega.ID, []directive.Raw{{Name: logDirective}, {Name: markDirective}}),
		"the directives attach")
	g.Freeze()
	reads := store.NewReadSet()
	r, err := g.Reader(reads, nil)
	assert.NoError(tb, err, "a sealed graph hands out a reader")
	r.Lookup(omega.ID)
	r.Lookup(alpha.ID)
	r.PackageOf(omega.ID)
	r.PackageOf(cache.ID)
	for _, k := range []symbol.Kind{symbol.KindStruct, symbol.KindFile} {
		for range r.ByKind(k) { // ranging is what records the edges
		}
	}
	for _, n := range []directive.Name{logDirective, markDirective} {
		for range r.ByDirective(n) { // ranging is what records the edges
		}
	}
	reads.RecordFact(omega.ID, logKey)
	reads.RecordFact(alpha.ID, roleKey)
	reads.RecordFact(alpha.ID, logKey)
	return reads
}

// grainsOf returns every enumeration of a read set.
func grainsOf(s *store.ReadSet) grains {
	g := grains{
		identities: slices.Collect(s.Identities()),
		packages:   slices.Collect(s.Packages()),
		kinds:      slices.Collect(s.Kinds()),
		directives: slices.Collect(s.Directives()),
	}
	for id, key := range s.Facts() {
		g.facts = append(g.facts, meta.FactRef{Subject: id, Key: key})
	}
	return g
}
