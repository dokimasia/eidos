// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Attachments arrive from concurrent units in any interleaving, so
// the seal has to fix one order that does not depend on arrival,
// even where two items share a position.
func TestAttach(t *testing.T) {
	t.Parallel()

	subject := symbol.Identity{Lang: "fixture", Package: "svc", Name: "Row", Kind: symbol.KindStruct}
	at := position.Pos{File: "row.go", Line: 3, Col: 1}

	t.Run("orders stamps sharing one position the same whatever their arrival", func(t *testing.T) {
		t.Parallel()

		first := meta.RawStamp{Key: "fixture.kind", Value: "struct", Pos: at, Origin: "front"}
		second := meta.RawStamp{Key: "fixture.kind", Value: "record", Pos: at, Origin: "front"}
		third := meta.RawStamp{Key: "fixture.alias", Value: "row", Pos: at, Origin: "front"}
		sealed := func(ss ...meta.RawStamp) []meta.RawStamp {
			g := store.New()
			for _, s := range ss {
				assert.NoError(t, g.AttachStamps(subject, []meta.RawStamp{s}), "the stamp attaches")
			}
			g.Freeze()
			return g.StampsOf(subject)
		}
		assert.Equal(t, sealed(first, second, third), sealed(third, second, first),
			"the seal's order is the claim sequence, so it cannot follow arrival")
	})

	t.Run("orders directives sharing one position the same whatever their arrival", func(t *testing.T) {
		t.Parallel()

		arg := func(v string) []directive.RawArg {
			return []directive.RawArg{{Key: "tag", Value: directive.RawValue{Text: v}}}
		}
		first := directive.Raw{Name: "stub", Args: arg("a"), Pos: at}
		second := directive.Raw{Name: "stub", Args: arg("b"), Pos: at}
		third := directive.Raw{Name: "mock", Pos: at}
		sealed := func(ds ...directive.Raw) []directive.Raw {
			g := store.New()
			for _, d := range ds {
				assert.NoError(t, g.AttachDirectives(subject, []directive.Raw{d}), "the instance attaches")
			}
			g.Freeze()
			return g.DirectivesOf(subject)
		}
		assert.Equal(t, sealed(first, second, third), sealed(third, second, first),
			"instances at one position seal in one order")
	})
}
