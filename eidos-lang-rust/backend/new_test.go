// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"maps"
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// setup builds the backend over the kernel's canonical fixture,
// filtered to this module's rendered coverage: the declared kind
// templates, plus the method kind the impl cluster renders
// without one.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.CanonicalFixture(tb, inventory())
}

// benchSetup builds the backend over the suite's scaled corpus,
// filtered the way setup filters the coverage fixture: the
// declared templates plus the method kind the impl cluster
// renders without one.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.ScaledFixture(tb, inventory())
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus: the real templates, the impl clustering and the
// shared normalizer, under the allocation ceiling pinned from
// measurement with headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 11_500_000})
}

// BenchmarkSettle measures the settle over the suite's scaled
// corpus, the corpus build excluded from the measurement, under
// its own ceiling pinned from measurement with headroom.
func BenchmarkSettle(b *testing.B) {
	backendtest.BenchSettle(b, benchSetup,
		backendtest.Budget{MaxAllocs: 1_120_000})
}

// The backend is the module's write half: the kernel suite runs the
// render checks over it on the canonical fixture, and the stamp
// check joins it to the output contract under this module's own
// comment forms.
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

	t.Run("spells its convention through the settle", func(t *testing.T) {
		t.Parallel()

		var text []byte
		for _, f := range backendtest.RenderSettled(t, setup) {
			text = append(text, f.Body...)
		}
		assert.Contains(t, string(text), "pub struct Row",
			"a neutral row takes Pascal")
		assert.Contains(t, string(text), "pub fn track(&self)",
			"and a neutral track takes snake")
	})
}

// inventory is the kind set the fixtures span: the declared
// templates plus the method kind the impl cluster renders without
// one of its own.
func inventory() map[symbol.Kind]string {
	out := maps.Clone(backend.KindTemplates())
	out[symbol.KindMethod] = ""
	return out
}
