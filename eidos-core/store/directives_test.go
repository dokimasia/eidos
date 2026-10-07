// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"cmp"
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// attachBatch is how many subjects one iteration of the attachment
// benchmark attaches an instance to, in a graph of its own.
const attachBatch = 1_000

// attachOneAllocs is a subject's first attachment: the pending entry,
// the boxed identity, the sync.Map entry and the copy of the instances.
// The trie nodes sync.Map adds where two hashes share a prefix average
// below 0.4 per subject, which the mean over a check's calls rounds
// down.
const attachOneAllocs = 4

// attachAllocs is one iteration of the attachment benchmark: what
// attachOneAllocs counts for each subject, then the map's root, and the
// trie nodes sync.Map adds at random where two hashes share a prefix,
// 360 on average with a standard deviation of 10 for 1,000 subjects. The
// ceiling allows 440 trie nodes, eight standard deviations above the
// mean.
const attachAllocs = attachBatch*attachOneAllocs + 1 + 440

// Raw directives are the first raw attachment class: they attach during
// the parallel load, seal into one order, and index by spelling for the
// dispatcher, so their order and their reads are contract.
func TestDirectives(t *testing.T) {
	t.Parallel()

	t.Run("AttachDirectives", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches instances the graph then returns", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(7)}),
				"the instances attach")
			g.Freeze()

			assert.Length(t, g.DirectivesOf(decl.ID), 1,
				"the subject's instances are readable after the seal")
		})

		t.Run("returns an error for a zero subject", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.HasError(t, g.AttachDirectives(symbol.Identity{}, []directive.Raw{stubAt(1)}),
				"a directive on nothing indexes nowhere")
		})

		t.Run("returns FrozenWrite for an attachment to a sealed graph", func(t *testing.T) {
			t.Parallel()

			f := newSplit()
			g, _ := f.sealed()
			err := g.AttachDirectives(f.store.ID, []directive.Raw{stubAt(1)})
			assertRefused(t, err, store.FrozenWrite)
		})

		t.Run("returns an error for no instances", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			decl := coretest.Struct(coretest.StorePath, "Store")
			assert.HasError(t, g.AttachDirectives(decl.ID, nil),
				"attaching nothing is a defect, not a load")
		})

		t.Run("returns FrozenWrite for an attachment after Freeze", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t)
			decl := coretest.Struct(coretest.StorePath, "Store")
			err := g.AttachDirectives(decl.ID, []directive.Raw{stubAt(1)})
			assertRefused(t, err, store.FrozenWrite)
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("enumerates the declarations with the directive in identity order", func(t *testing.T) {
			t.Parallel()

			store1 := coretest.Struct(coretest.StorePath, "Store")
			cache := coretest.Struct(coretest.StorePath, "Cache")
			plain := coretest.Struct(coretest.StorePath, "Plain")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, store1, cache, plain)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(store1.ID, []directive.Raw{stubAt(7)}),
				"the first attaches")
			assert.NoError(t, g.AttachDirectives(cache.ID, []directive.Raw{stubAt(9)}),
				"and the second")
			g.Freeze()

			got := coretest.Names(t, slices.Collect(g.ByDirective("stub")))
			assert.Equal(t, got, []string{"Cache", "Store"},
				"the index returns carriers alone, in identity order")
		})

		t.Run("returns nothing for a spelling no declaration has", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			assert.Empty(t, slices.Collect(g.ByDirective("stub")),
				"a spelling no declaration has enumerates nothing")
		})

		t.Run("does not index a dangling subject", func(t *testing.T) {
			t.Parallel()

			ghost := coretest.Struct(coretest.CachePath, "Ghost")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(ghost.ID, []directive.Raw{stubAt(1)}),
				"the dangling attachment is admitted, and validation reports it")
			g.Freeze()

			assert.Empty(t, slices.Collect(g.ByDirective("stub")),
				"but a subject the graph does not contain is no dispatch match")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Store")
			two := coretest.Struct(coretest.StorePath, "Cache")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, one, two)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(one.ID, []directive.Raw{stubAt(1)}), "one attaches")
			assert.NoError(t, g.AttachDirectives(two.ID, []directive.Raw{stubAt(2)}), "and two")
			g.Freeze()

			seen := 0
			for range g.ByDirective("stub") {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})

	t.Run("DirectivesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns instances in position order", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(9)}),
				"the later line attaches first")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(3)}),
				"the earlier second")
			g.Freeze()

			got := g.DirectivesOf(decl.ID)
			assert.Length(t, got, 2, "both instances are returned")
			assert.Equal(t, got[0].Pos.Line, 3,
				"in position order, so two concurrent attachments produce one order")
			assert.Equal(t, got[1].Pos.Line, 9, "earliest first")
		})

		t.Run("returns nothing before Freeze", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(1)}),
				"the instance attaches")

			assert.Empty(t, g.DirectivesOf(decl.ID),
				"an untracked read before the seal returns nothing rather than a partial result")
		})
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("walks every attachment in identity order including a dangling one", func(t *testing.T) {
			t.Parallel()

			held := coretest.Struct(coretest.StorePath, "Store")
			ghost := coretest.Struct(coretest.CachePath, "Ghost")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, held)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(held.ID, []directive.Raw{stubAt(1)}),
				"the held subject attaches")
			assert.NoError(t, g.AttachDirectives(ghost.ID, []directive.Raw{stubAt(2)}),
				"and the dangling one")
			g.Freeze()

			var subjects []symbol.Identity
			for id, ds := range g.Directives() {
				subjects = append(subjects, id)
				assert.NotEmpty(t, ds, "every walked subject has instances")
			}
			assert.Equal(t, subjects, []symbol.Identity{ghost.ID, held.ID},
				"the walk is the validator's: every attachment, dangling included, "+
					"in identity order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Store")
			two := coretest.Struct(coretest.StorePath, "Cache")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, one, two)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(one.ID, []directive.Raw{stubAt(1)}), "one attaches")
			assert.NoError(t, g.AttachDirectives(two.ID, []directive.Raw{stubAt(2)}), "and two")
			g.Freeze()

			var seen []symbol.Identity
			for id := range g.Directives() {
				seen = append(seen, id)
				break
			}
			assert.Equal(t, seen, []symbol.Identity{two.ID},
				"the walk stops at the first subject when the range stops")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reader that records a directive enumeration", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(1)}),
				"the instance attaches")
			g.Freeze()
			reads := store.NewReadSet()
			r, err := g.Reader(reads, nil)
			assert.NoError(t, err, "the sealed graph hands out a reader")

			got := coretest.Names(t, slices.Collect(r.ByDirective("stub")))
			assert.Equal(t, got, []string{"Store"}, "the tracked enumeration returns carriers")
			assert.Equal(t, slices.Collect(reads.Directives()), []directive.Name{"stub"},
				"records the membership edge")
			assert.Contains(t, slices.Collect(reads.Identities()), decl.ID,
				"and a per-identity edge for what the caller reached")
		})

		t.Run("returns a reader that records nothing outside scope", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath)), "one in scope")
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.CachePath, hidden)),
				"one out of scope")
			assert.NoError(t, g.AttachDirectives(hidden.ID, []directive.Raw{stubAt(1)}),
				"the hidden subject attaches")
			g.Freeze()
			reads := store.NewReadSet()
			r, err := g.Reader(reads, onlyPackage(coretest.StorePath))
			assert.NoError(t, err, "the scoped reader hands out")

			assert.Empty(t, slices.Collect(r.ByDirective("stub")),
				"a declaration outside scope is not returned")
			assert.NotContains(t, slices.Collect(reads.Identities()), hidden.ID, "and not recorded")
		})

		t.Run("returns a reader that records one membership edge for three enumerations", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath, decl)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(decl.ID, []directive.Raw{stubAt(1)}),
				"the instance attaches")
			g.Freeze()
			reads := store.NewReadSet()
			r, err := g.Reader(reads, nil)
			assert.NoError(t, err, "the reader hands out")

			for range 3 {
				for range r.ByDirective("stub") { // ranging is what records the edge
				}
			}
			assert.Length(t, slices.Collect(reads.Directives()), 1,
				"three enumerations record one membership edge")
			assert.Equal(t, reads.Len(), 2,
				"one membership edge and one per-identity edge: all grains count")
		})
	})
}

// A subject's first attachment allocates what the graph keeps of it,
// and the validator's reads of the sealed instances allocate nothing, in
// the ordinary run, which runs no benchmark. Each counted attachment is
// on a subject of its own, which the setup names outside the count. The
// check runs alone, because the count includes every goroutine's
// allocations.
func TestDirectivesAllocs(t *testing.T) {
	g := store.New()
	attached := 0
	next := func() symbol.Identity {
		attached++
		return coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(attached-1)).ID
	}
	raws := []directive.Raw{stubAt(1)}
	var err error
	assert.MaxAllocsWithSetup(t, next, func(id symbol.Identity) { err = cmp.Or(err, g.AttachDirectives(id, raws)) },
		attachOneAllocs, "AttachDirectives allocates what the graph keeps of a subject's first attachment")
	assert.NoError(t, err, "every first attachment is admitted")

	g.Freeze()
	first := coretest.Struct(coretest.StorePath, "Decl0").ID
	var got []directive.Raw
	assert.MaxAllocs(t, func() { got = g.DirectivesOf(first) }, 0, "DirectivesOf allocates nothing")
	assert.Length(t, got, 1, "DirectivesOf returns the subject's instance")
	n := 0
	assert.MaxAllocs(t, func() {
		n = 0
		for range g.Directives() {
			n++
		}
	}, 0, "a range over Directives allocates nothing")
	assert.Equal(t, n, attached, "Directives enumerates every subject with an attachment")
}

// BenchmarkDirectives measures the directive side, which scales with the
// declarations that have directives: attachment during the parallel
// load, the enumeration each gated rule makes, and the validator's
// reads.
func BenchmarkDirectives(b *testing.B) {
	// The scale: a tenth of the subjects have one directive, which
	// is a directive-heavy workspace.
	const carriers = 20_000

	b.Run("AttachDirectives", func(b *testing.B) {
		b.Run("a first attachment on each of a thousand subjects", func(b *testing.B) {
			ids := make([]symbol.Identity, 0, attachBatch)
			raws := make([][]directive.Raw, 0, attachBatch)
			for i := range attachBatch {
				ids = append(ids, coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)).ID)
				raws = append(raws, []directive.Raw{stubAt(i + 1)})
			}
			var g *store.Graph
			fresh := func() { g = store.New() }
			c := bench.Start(b).MaxAllocs(attachAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				for i, id := range ids {
					if err = g.AttachDirectives(id, raws[i]); err != nil {
						break
					}
				}
			}
			assert.NoError(b, err, "every attachment is admitted")
		})
	})

	b.Run("ByDirective", func(b *testing.B) {
		decls := make([]symbol.Symbol, 0, carriers)
		for i := range carriers {
			decls = append(decls, coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)))
		}
		g := store.New()
		assert.NoError(b, g.AddPackage(coretest.Package(coretest.StorePath, decls...)), "the package loads")
		for i, decl := range decls {
			named, _ := decl.(node.Declaration)
			assert.NoError(b, g.AttachDirectives(named.Identity(), []directive.Raw{stubAt(i + 1)}),
				"the directive attaches")
		}
		g.Freeze()
		// The warm-up pass runs at the call site it measures, where the
		// runtime builds the site's 48-byte cache for converting a
		// declaration to a symbol. A pass at any other call site builds a
		// cache of its own.
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.ByDirective("stub") {
				seen++
			}
		}
		assert.Equal(b, seen, carriers, "ByDirective returns every declaration with the directive")
	})

	g := store.New()
	ids := make([]symbol.Identity, 0, attachBatch)
	for i := range attachBatch {
		id := coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)).ID
		ids = append(ids, id)
		assert.NoError(b, g.AttachDirectives(id, []directive.Raw{stubAt(i + 1)}), "the directive attaches")
	}
	g.Freeze()

	b.Run("DirectivesOf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []directive.Raw
		for c.Loop() {
			got = g.DirectivesOf(ids[0])
		}
		assert.Length(b, got, 1, "DirectivesOf returns the subject's instance")
	})

	b.Run("Directives", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range g.Directives() {
				seen++
			}
		}
		assert.Equal(b, seen, attachBatch, "Directives returns every subject with instances")
	})
}

// stubAt returns a raw stub instance positioned at line.
func stubAt(line int) directive.Raw {
	return directive.Raw{
		Name: "stub",
		Pos:  position.Pos{File: "svc/store/unit.go", Line: line, Col: 1},
	}
}
