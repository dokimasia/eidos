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

// The directive spelling and the fact key the log fixtures read.
const (
	logDirective directive.Name = "stub"
	logKey       meta.KeyName   = "t.flag"
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
}

// Loading an entry into a set that once had as many edges allocates
// nothing, which is what lets a journal hand each record through one
// set. The check runs alone, because AllocsPerRun counts every
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
}

// BenchmarkReadLog measures a set of five edges, one of each grain,
// entering the log and loading back into one set.
func BenchmarkReadLog(b *testing.B) {
	reads := everyGrain(b)

	b.Run("Append", func(b *testing.B) {
		var log store.ReadLog
		// The log's slices grow by doubling, which amortizes to below one
		// allocation per entry.
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got int
		for c.Loop() {
			got = log.Append(reads)
		}
		if got < 0 {
			b.Fatalf("Append returns entry %d", got)
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
