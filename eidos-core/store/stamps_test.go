// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"errors"
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
// behavior is pinned beside the directives they mirror.
func TestStamps(t *testing.T) {
	t.Parallel()

	t.Run("sorts by position at the seal, per subject", func(t *testing.T) {
		t.Parallel()

		g := store.New()
		assert.NoError(t, g.AddPackage(&node.Package{ID: stampSubject()}), "the subject loads")
		assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(9)}), "one attaches")
		assert.NoError(t, g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(2)}), "another attaches")

		assert.Length(t, g.StampsOf(stampSubject()), 0, "nothing reads before the seal")
		g.Freeze()

		held := g.StampsOf(stampSubject())
		assert.Length(t, held, 2, "both stamps survive the seal")
		assert.True(t, held[0].Pos.Line == 2 && held[1].Pos.Line == 9,
			"in position order, whatever order they attached in")

		subjects := 0
		for id, ss := range g.Stamps() {
			subjects++
			assert.Equal(t, id, stampSubject(), "under the one subject")
			assert.Length(t, ss, 2, "with its stamps")
		}
		assert.Equal(t, subjects, 1, "the walk visits each subject once")
	})

	t.Run("refuses what cannot index", func(t *testing.T) {
		t.Parallel()

		g := store.New()
		assert.HasError(t, g.AttachStamps(symbol.Identity{}, []meta.RawStamp{stampAt(1)}),
			"a zero subject indexes nowhere")
		assert.HasError(t, g.AttachStamps(stampSubject(), nil),
			"an empty attachment is a defect")

		g.Freeze()
		err := g.AttachStamps(stampSubject(), []meta.RawStamp{stampAt(1)})
		assert.HasError(t, err, "the seal excludes writes")
		var refused *store.RefusedError
		assert.True(t, errors.As(err, &refused) && refused.Code == store.FrozenWrite,
			"under the frozen write code")
	})
}
