// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The target of the fixture backend, and the spelling of the name key of
// the golang target.
const (
	fixtureTarget plugin.Target = "fixture"
	golangNameKey meta.KeyName  = "golang.name"
)

// renderless is a fixture backend that a composition validates and that
// renders nothing. The methods of a rendering backend belong to the nil
// interface that it embeds.
type renderless struct {
	plugin.Backend
	name plugin.ID
}

// Name returns the name of the backend.
func (b renderless) Name() plugin.ID { return b.name }

// Target returns the fixture's target.
func (renderless) Target() plugin.Target { return fixtureTarget }

// textSpelling is the spelling of a text shape, in the source and in the
// fixture spoke's target.
const textSpelling = "string"

// speller is a fixture spoke. It spells a text shape as string under
// every policy, and returns an error for every other shape.
type speller struct{}

// SpellType spells a text shape, and returns an error for every other
// shape.
func (speller) SpellType(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	if s.Form != symbol.FormText {
		return nil, errors.New("fixture: the spoke spells text alone")
	}
	return &emit.TypeRef{Spelling: textSpelling}, nil
}

// TestBackend tests the backend role. A plugin that renders nothing
// does not implement it.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the target a backend declares", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = renderless{name: "printer"}
			b, held := p.(plugin.Backend)
			assert.True(t, held, "the fixture implements the backend role")
			assert.Equal(t, b.Target(), fixtureTarget, "the backend returns its target")
		})

		t.Run("is not declared by a plugin that renders nothing", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, held := p.(plugin.Backend)
			assert.False(t, held, "a plugin that renders nothing does not implement the backend role")
		})
	})

	t.Run("NameKey", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the target's spelling followed by .name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, plugin.Target("golang").NameKey(), golangNameKey,
				"the settle reads the name override of a Go declaration under this key")
		})
	})

	t.Run("TypeSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a text shape", func(t *testing.T) {
			t.Parallel()

			var spoke plugin.TypeSpeller = speller{}
			got, err := spoke.SpellType(rules.Leaf(symbol.FormText, textSpelling), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, textSpelling, "the spelling is the spelling of the target")
		})

		t.Run("returns an error for a shape that is not text", func(t *testing.T) {
			t.Parallel()

			var spoke plugin.TypeSpeller = speller{}
			_, err := spoke.SpellType(rules.Leaf(symbol.FormBool, "bool"), plugin.Policy{})
			assert.HasError(t, err, "the spoke does not spell a bool")
		})
	})
}

// TestBackendAllocs checks that the name key of a target allocates its
// joined spelling alone, because the ordinary run runs no benchmark. The
// test does not run in parallel, because the count includes the
// allocations of every goroutine.
func TestBackendAllocs(t *testing.T) {
	target := plugin.Target("golang")
	var got meta.KeyName
	assert.MaxAllocs(t, func() { got = target.NameKey() }, 1, "NameKey allocates the joined spelling")
	assert.Equal(t, got, golangNameKey, "NameKey returns the key of the golang target")
}

// BenchmarkBackend measures the spelling of the name key of a target. A
// settle spells it once for each plan.
func BenchmarkBackend(b *testing.B) {
	b.Run("NameKey", func(b *testing.B) {
		target := plugin.Target("golang")
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got meta.KeyName
		for c.Loop() {
			got = target.NameKey()
		}
		assert.Equal(b, got, golangNameKey, "NameKey returns the key of the golang target")
	})
}
