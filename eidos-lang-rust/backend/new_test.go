// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"maps"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	rust "go.dokimi.dev/eidos/lang-rust"
	"go.dokimi.dev/eidos/lang-rust/backend"
)

// setup builds the backend over the kernel's canonical fixture,
// filtered to this module's rendered coverage: the declared kind
// templates, plus the method kind the impl cluster renders
// without one.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	inventory := maps.Clone(backend.KindTemplates())
	inventory[symbol.KindMethod] = ""
	return r, backendtest.CanonicalFixture(tb, inventory)
}

// benchSetup builds the backend over the suite's scaled corpus,
// filtered the way setup filters the coverage fixture: the
// declared templates plus the method kind the impl cluster
// renders without one.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	inventory := maps.Clone(backend.KindTemplates())
	inventory[symbol.KindMethod] = ""
	return r, backendtest.ScaledFixture(tb, inventory)
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus: the real templates and the impl clustering,
// through the pass-through formatter, under the allocation
// ceiling pinned from measurement with headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 5_300_000})
}

// The backend is the module's write half: the kernel suite holds
// it to the render checks over the canonical fixture, and the
// stamp check joins it to the output contract under this module's
// own comment forms.
func TestNew(t *testing.T) {
	t.Parallel()

	backendtest.RunBackendSuite(t, setup)

	t.Run("declares its identity and target", func(t *testing.T) {
		t.Parallel()

		b := backend.New()
		assert.Equal(t, b.Name(), rust.Name, "the plugin identity")
		assert.Equal(t, b.Target(), rust.Target, "the rendering target")
	})

	t.Run("stamps under the module's contract", func(t *testing.T) {
		t.Parallel()

		c, err := output.NewContract("rust", rust.Syntax())
		assert.NoError(t, err, "the module contract composes")
		backendtest.AssertStamped(t, setup, c)
	})
}
