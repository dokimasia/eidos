// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The end-to-end fixture: the brand the output contract stamps under,
// and the unit a rendered declaration comes from.
const (
	contractBrand = "java"
	unitPkg       = "svc/app"
	unitKey       = "svc/app/Holder.java"
	unitWord      = "gen"
	holderName    = "Holder"
)

// setup builds the backend over the kernel's canonical fixture,
// which emits every file-level kind: the backend spells the three
// type kinds, with every body content form arriving through the
// class template's members, lowers the sum into the interface and
// its classes, and refuses the rest.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.CanonicalFixture(tb)
}

// benchSetup builds the backend over the suite's scaled corpus,
// without the kinds the backend refuses.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.ScaledFixture(tb, backend.RefusedKinds())
}

// rendered settles one unit of declarations through the backend and
// renders it, and returns the first file beside the run's findings.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, *diag.Sink) {
	tb.Helper()

	b := backend.New()
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(plugin.Unit{
		Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
		Pkg:   symbol.Identity{Lang: java.Lang, Package: unitPkg, Kind: symbol.KindPackage},
		Decls: decls,
	}), "the unit is added")
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles")
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	files, err := r.Render(&plugin.RenderContext{Emit: e, Sink: sink, Plugin: java.Name})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus. The split fans every typed declaration into its
// own file, so the corpus renders far more files than units, and
// the allocation ceiling includes that fan-out. The ceiling is
// pinned from a measured 29.22M allocations with 1.3% headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 29_600_000})
}

// BenchmarkSettle measures the settle over the suite's scaled
// corpus, the corpus build excluded from the measurement, under
// its own ceiling pinned from measurement with headroom.
func BenchmarkSettle(b *testing.B) {
	backendtest.BenchSettle(b, benchSetup,
		backendtest.Budget{MaxAllocs: 3_400_000})
}

// The backend is the module's write half: the kernel suite runs the
// render checks over it on the canonical fixture, and the stamp
// check joins it to the output contract under this module's own
// comment forms.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, setup)

		t.Run("returns a backend named after the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Name(), java.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), java.Target, "the rendering target")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, java.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that spells neutral names through the settle", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), "public class Row", "a neutral row takes Pascal")
			assert.Contains(t, string(text), "public void boot()", "a neutral boot keeps camel case")
			assert.Contains(t, string(text), "public final class ShapeCircle implements Shape",
				"a lowered variant implements the respelled principal")
			assert.Contains(t, string(text), "public sealed interface Shape permits ShapeCircle, ShapeEmpty",
				"the sealed principal permits the respelled classes")
		})

		t.Run("returns a backend that imports the class a field's type names", func(t *testing.T) {
			t.Parallel()

			holder := &emit.Struct{
				Origin: symbol.Identity{Lang: java.Lang, Package: unitPkg, Name: holderName, Kind: symbol.KindStruct},
				Name:   holderName,
			}
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storePkg, rowName)})
			body, sink := rendered(t, holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "import svc.store.Row;\n", "the class's import")
			assert.Contains(t, body, "    public Row row;\n", "the field through the simple name")
		})
	})
}
