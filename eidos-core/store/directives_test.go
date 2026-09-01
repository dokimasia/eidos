// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// stubAt returns a raw stub instance positioned at line.
func stubAt(line int) directive.Raw {
	return directive.Raw{
		Name: "stub",
		Pos:  position.Pos{File: "svc/store/unit.go", Line: line, Col: 1},
	}
}

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

		t.Run("refuses a zero subject", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			assert.HasError(t, g.AttachDirectives(symbol.Identity{}, []directive.Raw{stubAt(1)}),
				"a directive on nothing indexes nowhere")
		})

		t.Run("refuses no instances at all", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			decl := coretest.Struct(coretest.StorePath, "Store")
			assert.HasError(t, g.AttachDirectives(decl.ID, nil),
				"attaching nothing is a defect, not a load")
		})

		t.Run("refuses an attachment after Freeze", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t)
			decl := coretest.Struct(coretest.StorePath, "Store")
			err := g.AttachDirectives(decl.ID, []directive.Raw{stubAt(1)})
			assertRefused(t, err, store.FrozenWrite)
		})
	})

	t.Run("ByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("enumerates carrying declarations in identity order", func(t *testing.T) {
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

		t.Run("returns nothing for a spelling nothing carries", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.Package(coretest.StorePath))
			assert.Empty(t, slices.Collect(g.ByDirective("stub")),
				"an uncarried spelling enumerates nothing")
		})

		t.Run("does not index a dangling subject", func(t *testing.T) {
			t.Parallel()

			ghost := coretest.Struct(coretest.CachePath, "Ghost")
			g := store.New()
			assert.NoError(t, g.AddPackage(coretest.Package(coretest.StorePath)),
				"the package loads")
			assert.NoError(t, g.AttachDirectives(ghost.ID, []directive.Raw{stubAt(1)}),
				"the dangling attachment is admitted; validation reports it")
			g.Freeze()

			assert.Empty(t, slices.Collect(g.ByDirective("stub")),
				"but a subject the graph does not hold is no dispatch match")
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

		t.Run("walks every attachment in identity order, dangling included", func(t *testing.T) {
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
				assert.NotEmpty(t, ds, "every walked subject carries instances")
			}
			assert.Equal(t, subjects, []symbol.Identity{ghost.ID, held.ID},
				"the walk is the validator's: every attachment, dangling included, "+
					"in identity order")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("ByDirective records the membership grain and what was reached", func(t *testing.T) {
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
			assert.True(t, recorded(reads, decl.ID),
				"and a per-identity edge for what the caller reached")
		})

		t.Run("neither returns nor records outside scope", func(t *testing.T) {
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
			assert.False(t, recorded(reads, hidden.ID), "and not recorded")
		})
	})

	t.Run("ReadSet", func(t *testing.T) {
		t.Parallel()

		t.Run("Len counts the fourth grain and edges deduplicate", func(t *testing.T) {
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

// The directive side scales with carriers: attachment during the
// parallel load, one index pass at the seal, and enumeration per
// gated rule.
func BenchmarkDirectives(b *testing.B) {
	// The scale: a tenth of the subjects carry one directive, which
	// is a directive-heavy workspace.
	const carriers = 20_000

	b.Run("AttachDirectives", func(b *testing.B) {
		b.ReportAllocs()

		g := store.New()
		next := 0
		for b.Loop() {
			id := coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(next))
			if err := g.AttachDirectives(id.ID, []directive.Raw{stubAt(next + 1)}); err != nil {
				b.Fatalf("AttachDirectives: unexpected error: %v", err)
			}
			next++
		}
	})

	b.Run("ByDirective", func(b *testing.B) {
		b.ReportAllocs()

		decls := make([]symbol.Symbol, 0, carriers)
		for i := range carriers {
			decls = append(decls, coretest.Struct(coretest.StorePath, "Decl"+strconv.Itoa(i)))
		}
		g := store.New()
		if err := g.AddPackage(coretest.Package(coretest.StorePath, decls...)); err != nil {
			b.Fatalf("AddPackage: unexpected error: %v", err)
		}
		for i, decl := range decls {
			named, _ := decl.(node.Declaration)
			if err := g.AttachDirectives(named.Identity(), []directive.Raw{stubAt(i + 1)}); err != nil {
				b.Fatalf("AttachDirectives: unexpected error: %v", err)
			}
		}
		g.Freeze()

		for b.Loop() {
			seen := 0
			for range g.ByDirective("stub") {
				seen++
			}
			if seen != carriers {
				b.Fatalf("ByDirective returned %d carriers, want %d", seen, carriers)
			}
		}
	})
}
