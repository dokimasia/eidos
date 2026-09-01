// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"maps"
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang-java"
	"go.dokimi.dev/eidos/lang-java/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// setup builds the backend over the kernel's canonical fixture,
// filtered to this module's rendered coverage: the declared kind
// templates, with every body content form arriving through the
// class template's members, plus the sum kind the lowering
// reshapes into the interface and its classes before any template
// runs.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.CanonicalFixture(tb, inventory())
}

// benchSetup builds the backend over the suite's scaled corpus,
// filtered the way setup filters the coverage fixture.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.ScaledFixture(tb, inventory())
}

// inventory is the module's rendered coverage: the declared kind
// templates plus the kind the lowering consumes.
func inventory() map[symbol.Kind]string {
	i := maps.Clone(backend.KindTemplates())
	i[symbol.KindSum] = ""
	return i
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus. The split fans every typed declaration into its
// own file, so the corpus renders far more files than units; the
// allocation ceiling carries that fact, pinned from measurement
// with headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 28_700_000})
}

// BenchmarkSettle measures the settle over the suite's scaled
// corpus, the corpus build excluded from the measurement, under
// its own ceiling pinned from measurement with headroom.
func BenchmarkSettle(b *testing.B) {
	backendtest.BenchSettle(b, benchSetup,
		backendtest.Budget{MaxAllocs: 3_400_000})
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
		assert.Equal(t, b.Name(), java.Name, "the plugin identity")
		assert.Equal(t, b.Target(), java.Target, "the rendering target")
	})

	t.Run("stamps under the module's contract", func(t *testing.T) {
		t.Parallel()

		c, err := output.NewContract("java", java.Syntax())
		assert.NoError(t, err, "the module contract composes")
		backendtest.AssertStamped(t, setup, c)
	})

	t.Run("spells its convention through the settle", func(t *testing.T) {
		t.Parallel()

		var text []byte
		for _, f := range backendtest.RenderSettled(t, setup) {
			text = append(text, f.Body...)
		}
		assert.Contains(t, string(text), "public class Row",
			"a neutral row takes Pascal")
		assert.Contains(t, string(text), "public void boot()",
			"and a neutral boot stays camel")
		assert.Contains(t, string(text),
			"public final class ShapeCircle implements Shape",
			"a lowered variant implements the respelled principal")
		assert.Contains(t, string(text),
			"public sealed interface Shape permits ShapeCircle, ShapeEmpty",
			"and the sealed principal permits the respelled classes")
	})
}
