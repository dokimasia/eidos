// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// renderless is a fixture backend: carried and validated by a
// composition while nothing renders.
type renderless struct {
	named
}

func (renderless) Target() plugin.Target { return "fixture" }

// A backend is a plugin carrying the target name its plan resolves
// at composition, so the contract has to assert like every other
// surface and stay opt-in for plugins that render nothing.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("a backend carries its target", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = renderless{name: "printer"}
		b, held := p.(plugin.Backend)
		assert.True(t, held, "the backend surface asserts")
		assert.Equal(t, b.Target(), plugin.Target("fixture"),
			"answering the declared target name")
	})

	t.Run("a bare plugin holds no target", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = named{name: "bare"}
		_, held := p.(plugin.Backend)
		assert.False(t, held, "the backend surface is opt-in")
	})
}
