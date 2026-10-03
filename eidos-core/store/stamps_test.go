// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

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

		t.Run("refuses a write after Freeze", func(t *testing.T) {
			t.Parallel()

			g := store.New()
			g.Freeze()
			assertRefused(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(1)}), store.FrozenWrite)
		})

		t.Run("refuses a write to a sealed graph", func(t *testing.T) {
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
