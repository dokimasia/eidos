// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
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
	contractBrand = "rust"
	unitModule    = "crate/app"
	unitKey       = "app/holder.rs"
	unitWord      = "gen"
	holderName    = "Holder"
)

// setup builds the backend over the kernel's canonical fixture,
// which emits every file-level kind: the backend spells each, the
// method through the impl cluster, and refuses the variable.
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
// renders it, and returns the file beside the run's findings.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, *diag.Sink) {
	tb.Helper()

	b := backend.New()
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(plugin.Unit{
		Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
		Pkg:   symbol.Identity{Lang: rust.Lang, Package: unitModule, Kind: symbol.KindPackage},
		Decls: decls,
	}), "the unit is added")
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles")
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	files, err := r.Render(&plugin.RenderContext{Emit: e, Sink: sink, Plugin: rust.Name})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
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

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, setup)

		t.Run("returns a backend named after the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Name(), rust.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), rust.Target, "the rendering target")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, rust.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that spells neutral names through the settle", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), "pub struct Row", "a neutral row takes Pascal")
			assert.Contains(t, string(text), "pub fn track(&self)", "a neutral track takes snake")
		})

		t.Run("returns a backend that binds a generic receiver's parameter", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), "impl<T> Box<T> {",
				"the receiver's argument targets the host's parameter, so the impl binds it")
		})

		t.Run("returns a backend that uses the item a field's type names", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{Lang: rust.Lang, Package: unitModule, Name: holderName, Kind: symbol.KindStruct}
			holder := &emit.Struct{Origin: origin, Name: holderName}
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storeModule, rowName)})
			body, sink := rendered(t, holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "use crate::store::Row;\n", "the item's use")
			assert.Contains(t, body, "    pub row: Row,\n", "the field through the item's name")
		})
	})
}
