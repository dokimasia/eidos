// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Stamps are the second raw attachment class, so their seal
// behaviour is pinned beside the directives they mirror.
func TestStamps(t *testing.T) {
	t.Parallel()

	t.Run("AttachStamps", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a zero subject", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, store.New().AttachStamps(symbol.Identity{}, []meta.RawStamp{stampAt(1)}),
				"a zero subject indexes nowhere")
		})

		t.Run("returns an error for an empty attachment", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, store.New().AttachStamps(stampSubject(), nil),
				"an empty attachment is a defect")
		})

		t.Run("returns a FrozenWrite refusal after Freeze", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			g.Freeze()
			assertRefused(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(1)}), store.FrozenWrite)
		})

		t.Run("returns a FrozenWrite refusal for a sealed graph", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			assertRefused(t, g.AttachStamps(f.store.ID, []meta.RawStamp{stampAt(1)}), store.FrozenWrite)
		})
	})

	t.Run("StampsOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing before the seal", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.NoError(t, g.AddPackage(&node.Package{ID: stampSubject()}), "the subject loads")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(9)}), "a stamp attaches")
			assert.Length(t, g.StampsOf(stampSubject()), 0, "nothing reads before the seal")
		})

		t.Run("returns a subject's stamps in position order", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.NoError(t, g.AddPackage(&node.Package{ID: stampSubject()}), "the subject loads")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(9)}), "one attaches")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(2)}), "another attaches")
			g.Freeze()

			assert.Equal(t, g.StampsOf(stampSubject()), []meta.RawStamp{stampAt(2), stampAt(9)},
				"in position order, whatever order they attached in")
		})
	})

	t.Run("Stamps", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each subject once with its stamps", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.NoError(t, g.AddPackage(&node.Package{ID: stampSubject()}), "the subject loads")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(9)}), "one attaches")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(2)}), "another attaches")
			g.Freeze()

			assert.Equal(t, attached(g.Stamps()), []attachment[meta.RawStamp]{
				{subject: stampSubject(), items: []meta.RawStamp{stampAt(2), stampAt(9)}},
			}, "the walk visits the subject once, with both stamps")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			other := symbol.Identity{Lang: "fake", Package: "svc/cache", Kind: symbol.KindPackage}
			g := store.New()
			assert.NoError(t, g.AddPackage(&node.Package{ID: stampSubject()}), "one subject loads")
			assert.NoError(t, g.AddPackage(&node.Package{ID: other}), "and a second")
			assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(1)}),
				"the first attaches")
			assert.NoError(t, g.AttachStamps(other, []meta.RawStamp{stampAt(2)}), "and the second")
			g.Freeze()

			var seen []symbol.Identity
			for id := range g.Stamps() {
				seen = append(seen, id)
				break
			}
			assert.Equal(t, seen, []symbol.Identity{stampSubject()},
				"the walk stops at the first subject in identity order")
		})
	})
}

// A subject's first stamps allocate what the graph keeps of them, as a
// first attachment of directives does, and the apply step's reads of
// the sealed stamps allocate nothing, in the ordinary run, which runs no
// benchmark. Each counted attachment is on a subject of its own. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestStampsAllocs(t *testing.T) {
	g, ids := store.New(), stampedSubjects(allocRuns)
	stamps, at := []meta.RawStamp{stampAt(1)}, 0
	assert.MaxAllocs(t, func() {
		if err := g.AttachStamps(ids[at], stamps); err != nil {
			t.Fatalf("AttachStamps: unexpected error: %v", err)
		}
		at++
	}, attachOneAllocs, "AttachStamps allocates what the graph keeps of a subject's first stamps")

	g.Freeze()
	var got []meta.RawStamp
	assert.MaxAllocs(t, func() { got = g.StampsOf(ids[0]) }, 0, "StampsOf allocates nothing")
	assert.Length(t, got, 1, "StampsOf returns the subject's stamp")
	assert.MaxAllocs(t, func() {
		n := 0
		for range g.Stamps() {
			n++
		}
		if n != allocRuns {
			t.Fatal("Stamps enumerated another number of subjects")
		}
	}, 0, "a range over Stamps allocates nothing")
}

// BenchmarkStamps measures the stamp side: attachment during the
// parallel load, and the apply step's reads of the sealed stamps.
func BenchmarkStamps(b *testing.B) {
	ids := stampedSubjects(attachBatch)

	b.Run("AttachStamps/a first attachment on each of a thousand subjects", func(b *testing.B) {
		stamps := []meta.RawStamp{stampAt(1)}
		var g *store.Graph
		fresh := func() { g = store.New() }
		c := bench.Start(b).MaxAllocs(attachAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			c.Excluding(fresh)
			for _, id := range ids {
				if err = g.AttachStamps(id, stamps); err != nil {
					break
				}
			}
		}
		assert.NoError(b, err, "every attachment is admitted")
	})

	g := store.New()
	for i, id := range ids {
		assert.NoError(b, g.AttachStamps(id, []meta.RawStamp{stampAt(i + 1)}), "the stamp attaches")
	}
	g.Freeze()

	b.Run("StampsOf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []meta.RawStamp
		for c.Loop() {
			got = g.StampsOf(ids[0])
		}
		assert.Length(b, got, 1, "StampsOf returns the subject's stamp")
	})

	b.Run("Stamps", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.Stamps() {
				seen++
			}
		}
		assert.Equal(b, seen, attachBatch, "Stamps returns every subject with stamps")
	})
}

// stampSubject is the package the stamp fixtures attach onto.
func stampSubject() symbol.Identity {
	return symbol.Identity{Lang: "fake", Package: "svc", Kind: symbol.KindPackage}
}

// stampAt returns one raw stamp at a line, so ordering is visible.
func stampAt(line int) meta.RawStamp {
	return meta.RawStamp{
		Key:   "fake.testFile",
		Value: true,
		Pos:   position.Pos{File: "a.zz", Line: line},
	}
}

// stampedSubjects returns n distinct struct subjects in the store
// package, the subjects the allocation check and the benchmark attach
// stamps onto.
func stampedSubjects(n int) []symbol.Identity {
	out := make([]symbol.Identity, 0, n)
	for i := range n {
		out = append(out, coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)).ID)
	}
	return out
}
