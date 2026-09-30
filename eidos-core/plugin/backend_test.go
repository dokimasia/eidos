// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
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
}
