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
	golang "go.dokimi.dev/eidos/lang-go"
	"go.dokimi.dev/eidos/lang-go/backend"
)

// setup builds the backend over the kernel's canonical fixture,
// filtered to this module's rendered coverage: the declared kind
// templates, plus the enum kind the lowering reshapes into a
// defined type and constants before any template runs.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	inventory := maps.Clone(backend.KindTemplates())
	inventory[symbol.KindEnum] = ""
	return r, backendtest.CanonicalFixture(tb, inventory)
}

// benchSetup builds the backend over the suite's scaled corpus,
// filtered the way setup filters the coverage fixture.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	inventory := maps.Clone(backend.KindTemplates())
	inventory[symbol.KindEnum] = ""
	return r, backendtest.ScaledFixture(tb, inventory)
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus: the real templates and gofmt per file, under the
// allocation ceiling pinned from measurement with headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 14_800_000})
}

// BenchmarkSettle measures the settle over the suite's scaled
// corpus, the corpus build inside the number, under its own
// ceiling pinned from measurement with headroom.
func BenchmarkSettle(b *testing.B) {
	backendtest.BenchSettle(b, benchSetup,
		backendtest.Budget{MaxAllocs: 4_800_000})
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
		assert.Equal(t, b.Name(), golang.Name, "the plugin identity")
		assert.Equal(t, b.Target(), golang.Target, "the rendering target")
	})

	t.Run("stamps under the module's contract", func(t *testing.T) {
		t.Parallel()

		c, err := output.NewContract("golang", golang.Syntax())
		assert.NoError(t, err, "the module contract composes")
		backendtest.AssertStamped(t, setup, c)
	})

	t.Run("spells its convention through the settle", func(t *testing.T) {
		t.Parallel()

		var text []byte
		for _, f := range backendtest.RenderSettled(t, setup) {
			text = append(text, f.Body...)
		}
		assert.Contains(t, string(text), "type Row struct",
			"a neutral row exports as Row")
		assert.Contains(t, string(text), "func Fetch()",
			"and a neutral fetch as Fetch")
	})
}
