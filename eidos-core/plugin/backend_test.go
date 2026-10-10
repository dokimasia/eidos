// Copyright ThesmOS B.V. 2026
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

// The target the fixture backend renders to, and the name key of the
// golang target, pinned as its spelling.
const (
	fixtureTarget plugin.Target = "fixture"
	golangNameKey meta.KeyName  = "golang.name"
)

// renderless is a fixture backend: a composition validates it and
// nothing renders.
type renderless struct {
	named
}

// Target returns the fixture's target.
func (renderless) Target() plugin.Target { return fixtureTarget }

// textSpelling is the spelling of a text shape, in the source and in the
// fixture spoke's target.
const textSpelling = "string"

// speller is a fixture spoke. It spells a text shape as string under
// every policy, and it refuses every other shape.
type speller struct{}

// SpellType spells a text shape and refuses the rest.
func (speller) SpellType(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	if s.Form != symbol.FormText {
		return nil, errors.New("fixture: the spoke spells text alone")
	}
	return &emit.TypeRef{Spelling: textSpelling}, nil
}

// A backend is a plugin with the target name its plan resolves at
// composition, so the contract asserts like every other surface and
// is opt-in for plugins that render nothing.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the target a backend declares", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = renderless{name: "printer"}
			b, held := p.(plugin.Backend)
			assert.True(t, held, "the backend surface asserts")
			assert.Equal(t, b.Target(), fixtureTarget, "the declared target name")
		})

		t.Run("is not declared by a plugin that renders nothing", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "bare"}
			_, held := p.(plugin.Backend)
			assert.False(t, held, "the backend surface is opt-in")
		})
	})

	t.Run("NameKey", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the target's spelling followed by .name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, plugin.Target("golang").NameKey(), golangNameKey,
				"the key a Go name override is stamped under")
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
			assert.HasError(t, err, "the spoke refuses a bool")
		})
	})
}

// A target's name key allocates its joined spelling alone in the
// ordinary run, which runs no benchmark. The check runs alone, because
// the count includes every goroutine's allocations.
func TestBackendAllocs(t *testing.T) {
	target := plugin.Target("golang")
	var got meta.KeyName
	assert.MaxAllocs(t, func() { got = target.NameKey() }, 1, "NameKey allocates the joined spelling")
	assert.Equal(t, got, golangNameKey, "NameKey returns the golang target's key")
}

// BenchmarkBackend measures the spelling of a target's name key, which
// a settle makes once per plan.
func BenchmarkBackend(b *testing.B) {
	b.Run("NameKey", func(b *testing.B) {
		target := plugin.Target("golang")
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got meta.KeyName
		for c.Loop() {
			got = target.NameKey()
		}
		assert.Equal(b, got, golangNameKey, "NameKey returns the golang target's key")
	})
}
