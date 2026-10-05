// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fact grain's compile-time contract: one artifact's read set
// records declaration reads and fact reads into one place.
var _ meta.Recorder = (*store.ReadSet)(nil)

// The allocations of a read set and of a range over one of its
// enumerations, which TestReadSetAllocs checks in the ordinary run and
// BenchmarkReadSet in a benchmark run.
const (
	// newReadSetAllocs is the set.
	newReadSetAllocs = 1
	// enumerationAllocs is the sorted list of a grain's edges, where the
	// set moved its edges into maps.
	enumerationAllocs = 1
)

// pastInline is a number of edges past the four a read set keeps in
// place.
const pastInline = 6

// enumeration is one enumeration of a read set: its name and a count
// of the edges a range over it yields.
type enumeration struct {
	name  string
	count func() int
}

// A read set is what one derived artifact read. Its grain is the
// contract: edges deduplicate, and every grain counts.
func TestReadSet(t *testing.T) {
	t.Parallel()

	t.Run("Identities", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(store.NewReadSet().Identities()),
				"a set that read nothing returns no edge")
		})

		t.Run("records one edge for a declaration read twice", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			for range 3 {
				r.Lookup(decl.ID)
			}

			assert.Length(t, slices.Collect(reads.Identities()), 1,
				"a declaration read three times records one edge")
		})

		t.Run("returns one order however the reads arrived", func(t *testing.T) {
			t.Parallel()

			alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")

			forward, forwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			forward.Lookup(alpha.ID)
			forward.Lookup(omega.ID)

			backward, backwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			backward.Lookup(omega.ID)
			backward.Lookup(alpha.ID)

			assert.Equal(t,
				slices.Collect(forwardReads.Identities()),
				slices.Collect(backwardReads.Identities()),
				"the order is the set's own, not the read order")
		})

		t.Run("returns the edges of a set past four edges in identity order", func(t *testing.T) {
			t.Parallel()

			var decls []symbol.Symbol
			var ids []symbol.Identity
			for i := range pastInline {
				s := coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i))
				decls, ids = append(decls, s), append(ids, s.ID)
			}
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decls...))
			for _, id := range slices.Backward(ids) {
				r.Lookup(id)
			}

			slices.SortFunc(ids, symbol.Identity.Compare)
			assert.Equal(t, slices.Collect(reads.Identities()), ids, "the edges come back in identity order")
		})

		t.Run("stops when a range over the edges a log loaded stops", func(t *testing.T) {
			t.Parallel()

			seen := 0
			for range loadedFrom(pastFourEdges(t)).Identities() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
		})

		t.Run("returns nothing for a set past four edges that read no declaration", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			id := coretest.Struct(coretest.StorePath, "Store").ID
			for i := range pastInline {
				s.RecordFact(id, meta.KeyName("t.k"+strconv.Itoa(i)))
			}

			assert.Empty(t, slices.Collect(s.Identities()), "a set of fact edges has no declaration edge")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that took no package", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(store.NewReadSet().Packages()),
				"a set that took no package has no package edge")
		})

		t.Run("records one edge for a package taken twice", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.PackageOf(decl.ID)
			r.Lookup(coretest.PackageID(coretest.StorePath))

			assert.Equal(t, slices.Collect(reads.Packages()),
				[]symbol.Identity{coretest.PackageID(coretest.StorePath)},
				"PackageOf and a lookup of the package record one edge")
		})

		t.Run("returns one order however the reads arrived", func(t *testing.T) {
			t.Parallel()

			pkgs := []*node.Package{coretest.Package(coretest.StorePath), coretest.Package(coretest.CachePath)}
			forward, forwardReads := coretest.Reading(t, nil, pkgs...)
			forward.Lookup(coretest.PackageID(coretest.StorePath))
			forward.Lookup(coretest.PackageID(coretest.CachePath))

			backward, backwardReads := coretest.Reading(t, nil, pkgs...)
			backward.Lookup(coretest.PackageID(coretest.CachePath))
			backward.Lookup(coretest.PackageID(coretest.StorePath))

			assert.Equal(t,
				slices.Collect(forwardReads.Packages()),
				[]symbol.Identity{coretest.PackageID(coretest.CachePath), coretest.PackageID(coretest.StorePath)},
				"the edges come back in identity order")
			assert.Equal(t,
				slices.Collect(backwardReads.Packages()),
				slices.Collect(forwardReads.Packages()),
				"whatever order the reads arrived in")
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("records one edge for a kind enumerated twice", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			enumerate(r, symbol.KindStruct, symbol.KindStruct)

			assert.Length(t, slices.Collect(reads.Kinds()), 1,
				"a kind enumerated twice records one edge")
		})

		t.Run("returns one order however the enumerations arrived", func(t *testing.T) {
			t.Parallel()

			forward, forwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(forward, symbol.KindStruct, symbol.KindFile, symbol.KindPackage)

			backward, backwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(backward, symbol.KindPackage, symbol.KindFile, symbol.KindStruct)

			assert.Equal(t,
				slices.Collect(forwardReads.Kinds()),
				slices.Collect(backwardReads.Kinds()),
				"the order is the set's own, not the enumeration order")
		})

		t.Run("returns the edges of a set past four edges in kind order", func(t *testing.T) {
			t.Parallel()

			kinds := []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindFile, symbol.KindPackage, symbol.KindEnum,
			}
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(r, kinds...)

			assert.Equal(t, slices.Collect(reads.Kinds()), slices.Sorted(slices.Values(kinds)),
				"the edges come back in kind order")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("drops every grain", func(t *testing.T) {
			t.Parallel()

			s := everyGrain(t)
			assert.Equal(t, s.Len(), 5, "all five grains recorded")
			s.Reset()
			assert.Equal(t, s.Len(), 0, "a reset set has no edge")
		})

		t.Run("records an edge after the reset", func(t *testing.T) {
			t.Parallel()

			s := everyGrain(t)
			s.Reset()
			s.RecordFact(coretest.Struct(coretest.StorePath, "Store").ID, "shape.role")
			assert.Equal(t, s.Len(), 1, "the set records again")
		})

		t.Run("drops the edges a log loaded", func(t *testing.T) {
			t.Parallel()

			s := loadedFrom(everyGrain(t))
			s.Reset()
			assert.Equal(t, grainsOf(s), grainsOf(store.NewReadSet()), "a reset set returns no edge")
		})
	})

	t.Run("RecordFact", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the edges a log loaded into the set", func(t *testing.T) {
			t.Parallel()

			id := coretest.Struct(coretest.CachePath, "Cache").ID
			s := loadedFrom(everyGrain(t))
			s.RecordFact(id, "shape.role")
			want := everyGrain(t)
			want.RecordFact(id, "shape.role")
			assert.Equal(t, grainsOf(s), grainsOf(want), "the set has the loaded edges and the new one")
		})
	})

	t.Run("Facts", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that read none", func(t *testing.T) {
			t.Parallel()

			count := 0
			for range store.NewReadSet().Facts() {
				count++
			}
			assert.Equal(t, count, 0, "a set that read no fact returns none")
		})

		t.Run("returns each key of a subject once in key order", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			id := coretest.Struct(coretest.StorePath, "Store").ID
			for range 3 {
				s.RecordFact(id, "shape.role")
			}
			s.RecordFact(id, "shape.comparable")

			var got []meta.KeyName
			for subject, key := range s.Facts() {
				assert.Equal(t, subject, id, "every edge names its subject")
				got = append(got, key)
			}
			assert.Equal(t, got, []meta.KeyName{"shape.comparable", "shape.role"},
				"a key read three times records one edge, after the key that sorts first")
		})

		t.Run("returns one order however the reads arrived", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Alpha").ID
			other := coretest.Struct(coretest.StorePath, "Omega").ID

			forward, backward := store.NewReadSet(), store.NewReadSet()
			forward.RecordFact(one, "shape.role")
			forward.RecordFact(other, "shape.role")
			backward.RecordFact(other, "shape.role")
			backward.RecordFact(one, "shape.role")

			collect := func(s *store.ReadSet) []symbol.Identity {
				var out []symbol.Identity
				for subject := range s.Facts() {
					out = append(out, subject)
				}
				return out
			}
			assert.Equal(t, collect(forward), collect(backward),
				"the order is the set's own, not the read order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			s.RecordFact(coretest.Struct(coretest.StorePath, "Store").ID, "shape.role")
			s.RecordFact(coretest.Struct(coretest.StorePath, "Cache").ID, "shape.role")

			seen := 0
			for range s.Facts() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
		})

		t.Run("returns the edges of a set past four edges in subject then key order", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			one := coretest.Struct(coretest.StorePath, "Alpha").ID
			other := coretest.Struct(coretest.StorePath, "Omega").ID
			for _, key := range []meta.KeyName{"t.c", "t.a", "t.b"} {
				s.RecordFact(other, key)
				s.RecordFact(one, key)
			}

			var got []meta.FactRef
			for subject, key := range s.Facts() {
				got = append(got, meta.FactRef{Subject: subject, Key: key})
			}
			assert.Equal(t, got, []meta.FactRef{
				{Subject: one, Key: "t.a"},
				{Subject: one, Key: "t.b"},
				{Subject: one, Key: "t.c"},
				{Subject: other, Key: "t.a"},
				{Subject: other, Key: "t.b"},
				{Subject: other, Key: "t.c"},
			}, "the edges come back by subject, then by key")
		})
	})

	t.Run("AppendPointReads", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, store.NewReadSet().AppendPointReads(nil), "a set that read nothing has no point read")
		})

		t.Run("returns the point reads in identity order", func(t *testing.T) {
			t.Parallel()

			alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			r.Lookup(omega.ID)
			r.PackageOf(alpha.ID)
			r.Lookup(alpha.ID)

			want := []meta.Read{
				{Subject: alpha.ID},
				{Subject: omega.ID},
				{Subject: coretest.PackageID(coretest.StorePath)},
			}
			slices.SortFunc(want, func(a, b meta.Read) int { return a.Subject.Compare(b.Subject) })
			assert.Equal(t, reads.AppendPointReads(nil), want, "each identity reads once, in identity order")
		})

		t.Run("returns the facts after the declarations", func(t *testing.T) {
			t.Parallel()

			alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			reads.RecordFact(omega.ID, "shape.role")
			reads.RecordFact(alpha.ID, "shape.role")
			reads.RecordFact(alpha.ID, "shape.comparable")
			r.Lookup(omega.ID)

			assert.Equal(t, reads.AppendPointReads(nil), []meta.Read{
				{Subject: omega.ID},
				{Subject: alpha.ID, Key: "shape.comparable"},
				{Subject: alpha.ID, Key: "shape.role"},
				{Subject: omega.ID, Key: "shape.role"},
			}, "the facts follow in subject then key order")
		})

		t.Run("leaves out the membership edges", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(r, symbol.KindStruct)

			assert.Empty(t, reads.AppendPointReads(nil), "an enumeration by kind is no point read")
		})

		t.Run("appends after the elements of dst", func(t *testing.T) {
			t.Parallel()

			reads := everyGrain(t)
			first := meta.Read{Subject: coretest.PackageID(coretest.CachePath)}
			got := reads.AppendPointReads([]meta.Read{first})
			assert.Equal(t, got[0], first, "the element of dst comes first")
			assert.Equal(t, got[1:], reads.AppendPointReads(nil), "the point reads follow it")
		})

		t.Run("returns the point reads of a set a log loaded", func(t *testing.T) {
			t.Parallel()

			reads := pastFourEdges(t)
			assert.Equal(t, loadedFrom(reads).AppendPointReads(nil), reads.AppendPointReads(nil),
				"the loaded set derives what the recorded set does")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, store.NewReadSet().Len(), 0,
				"a set that read nothing counts nothing")
		})

		t.Run("counts every grain", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.Lookup(decl.ID)
			r.PackageOf(decl.ID)
			enumerate(r, symbol.KindFile)
			reads.RecordFact(decl.ID, "shape.role")

			// One declaration edge from the lookup, one package edge, one
			// declaration edge from the file the enumeration met, one
			// kind edge, and one fact edge.
			assert.Equal(t, reads.Len(), 5, "Len counts every grain")
		})

		t.Run("counts one edge for an edge read again past four edges", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			id := coretest.Struct(coretest.StorePath, "Store").ID
			keys := []meta.KeyName{"t.a", "t.b", "t.c", "t.d", "t.e"}
			for _, key := range keys {
				s.RecordFact(id, key)
			}
			s.RecordFact(id, keys[0])

			assert.Equal(t, s.Len(), len(keys), "the first key's second read records nothing")
		})

		t.Run("counts the edges a log loaded", func(t *testing.T) {
			t.Parallel()

			reads := pastFourEdges(t)
			assert.Equal(t, loadedFrom(reads).Len(), reads.Len(), "the loaded set counts what the recorded set does")
		})
	})
}

// A derivation allocates its one slice, and a set reused across
// invocations records within the storage an earlier invocation grew. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestReadSetAllocs(t *testing.T) {
	reads := everyGrain(t)
	room := make([]meta.Read, 0, reads.Len())
	assert.MaxAllocs(t, func() {
		if len(reads.AppendPointReads(room)) != 3 {
			t.Fatal("AppendPointReads returned another number of reads")
		}
	}, 0, "AppendPointReads allocates nothing into a slice with room")
	assert.MaxAllocs(t, func() {
		if len(reads.AppendPointReads(nil)) != 3 {
			t.Fatal("AppendPointReads returned another number of reads")
		}
	}, 1, "AppendPointReads allocates one slice into nil")

	for _, tt := range enumerations(reads) {
		msg := "a range over " + tt.name + " of edges in maps allocates the sorted list of edges"
		assert.MaxAllocs(t, func() {
			if tt.count() != 1 {
				t.Fatalf("%s enumerated another number of edges", tt.name)
			}
		}, enumerationAllocs, msg)
	}
	for _, tt := range enumerations(loadedFrom(reads)) {
		msg := "a range over " + tt.name + " of edges a log loaded allocates nothing"
		assert.MaxAllocs(t, func() {
			if tt.count() != 1 {
				t.Fatalf("%s enumerated another number of edges", tt.name)
			}
		}, 0, msg)
	}

	var built *store.ReadSet
	assert.MaxAllocs(t, func() { built = store.NewReadSet() }, newReadSetAllocs, "NewReadSet allocates the set")
	assert.Equal(t, built.Len(), 0, "NewReadSet returns a set without an edge")

	id := coretest.Struct(coretest.StorePath, "Store").ID
	assert.MaxAllocs(t, func() {
		reads.Reset()
		reads.RecordFact(id, "shape.role")
		if reads.Len() != 1 {
			t.Fatal("the reset set records another number of edges")
		}
	}, 0, "Reset and RecordFact allocate nothing within the set's earlier use")

	for _, tt := range enumerations(reads) {
		msg := "a range over " + tt.name + " of edges in place allocates nothing"
		assert.MaxAllocs(t, func() { tt.count() }, 0, msg)
	}
}

// BenchmarkReadSet measures a set of five edges, one of each grain: the
// derivation a stamp records from it, the reuse of the set by the next
// invocation, and a range over each enumeration of the set's edges in
// maps and of the same edges a log loaded into another set.
func BenchmarkReadSet(b *testing.B) {
	reads := everyGrain(b)
	id := coretest.Struct(coretest.StorePath, "Store").ID

	b.Run("AppendPointReads", func(b *testing.B) {
		b.Run("into a slice with room", func(b *testing.B) {
			room := make([]meta.Read, 0, reads.Len())
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got []meta.Read
			for c.Loop() {
				got = reads.AppendPointReads(room)
			}
			assert.Length(b, got, 3, "AppendPointReads returns the declaration, package and fact reads")
		})

		b.Run("into nil", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got []meta.Read
			for c.Loop() {
				got = reads.AppendPointReads(nil)
			}
			assert.Length(b, got, 3, "AppendPointReads returns the declaration, package and fact reads")
		})
	})

	b.Run("RecordFact", func(b *testing.B) {
		s := store.NewReadSet()
		s.RecordFact(id, "shape.role")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			s.RecordFact(id, "shape.role")
		}
		assert.Equal(b, s.Len(), 1, "RecordFact records one edge")
	})

	b.Run("Reset", func(b *testing.B) {
		s := everyGrain(b)
		fill := func() { s.RecordFact(id, "shape.role") }
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			c.Excluding(fill)
			s.Reset()
		}
		assert.Equal(b, s.Len(), 0, "Reset drops every edge")
	})

	b.Run("Len", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		n := 0
		for c.Loop() {
			n = reads.Len()
		}
		assert.Equal(b, n, 5, "Len counts every grain")
	})

	b.Run("NewReadSet", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newReadSetAllocs)
		defer c.End()
		var s *store.ReadSet
		for c.Loop() {
			s = store.NewReadSet()
		}
		assert.Equal(b, s.Len(), 0, "NewReadSet returns a set without an edge")
	})

	loaded := enumerations(loadedFrom(reads))
	for i, tt := range enumerations(reads) {
		b.Run(tt.name, func(b *testing.B) {
			b.Run("of edges in maps", func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(enumerationAllocs)
				defer c.End()
				n := 0
				for c.Loop() {
					n = tt.count()
				}
				assert.Equal(b, n, 1, "the enumeration returns the grain's one edge")
			})

			b.Run("of edges a log loaded", func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(0)
				defer c.End()
				n := 0
				for c.Loop() {
					n = loaded[i].count()
				}
				assert.Equal(b, n, 1, "the enumeration returns the grain's one edge")
			})
		})
	}
}

// enumerations returns a count of every enumeration of a read set, each
// a range over the enumeration.
func enumerations(s *store.ReadSet) []enumeration {
	return []enumeration{
		{name: "Identities", count: func() int {
			n := 0
			for range s.Identities() {
				n++
			}
			return n
		}},
		{name: "Packages", count: func() int {
			n := 0
			for range s.Packages() {
				n++
			}
			return n
		}},
		{name: "Kinds", count: func() int {
			n := 0
			for range s.Kinds() {
				n++
			}
			return n
		}},
		{name: "Facts", count: func() int {
			n := 0
			for range s.Facts() {
				n++
			}
			return n
		}},
		{name: "Directives", count: func() int {
			n := 0
			for range s.Directives() {
				n++
			}
			return n
		}},
	}
}

// loadedFrom returns a new set that a log loaded the edges of reads into.
func loadedFrom(reads *store.ReadSet) *store.ReadSet {
	var log store.ReadLog
	loaded := store.NewReadSet()
	log.Load(log.Append(reads), loaded)
	return loaded
}

// enumerate ranges every kind through the reader, which is what
// records a set-membership edge for each.
func enumerate(r *store.Reader, kinds ...symbol.Kind) {
	for _, kind := range kinds {
		for range r.ByKind(kind) { // ranging is what records the edge
		}
	}
}
