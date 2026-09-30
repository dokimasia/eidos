// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The end-to-end fixture: the brand the output contract stamps under,
// the file a unit of svcPkg renders into, and the method a stated
// receiver attaches to rowName.
const (
	contractBrand = "golang"
	unitKey       = svcPkg + "/row.go"
	unitWord      = "gen"
	touchName     = "touch"
)

// setup builds the backend over the kernel's canonical fixture,
// which emits every file-level kind: the backend spells each, lowers
// the enum into a defined type and constants, and refuses the sum.
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

// rowOf returns a struct of svcPkg named rowName with the given
// fields.
func rowOf(fields ...*emit.Field) *emit.Struct {
	row := &emit.Struct{
		Origin: symbol.Identity{Lang: golang.Lang, Package: svcPkg, Name: rowName, Kind: symbol.KindStruct},
		Name:   rowName,
	}
	row.Fields.Append(fields...)
	return row
}

// rendered settles one unit of svcPkg through the backend and renders
// it, and returns the file beside the run's findings.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, *diag.Sink) {
	tb.Helper()

	b := backend.New()
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(plugin.Unit{
		Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
		Pkg:   symbol.Identity{Lang: golang.Lang, Package: svcPkg, Kind: symbol.KindPackage},
		Decls: decls,
	}), "the unit is added")
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles")
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	files, err := r.Render(&plugin.RenderContext{Emit: e, Sink: sink, Plugin: golang.Name})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// BenchmarkNew measures the composed backend over the suite's
// scaled corpus: the real templates and gofmt per file, under the
// allocation ceiling pinned from measurement with headroom.
func BenchmarkNew(b *testing.B) {
	backendtest.BenchRender(b, benchSetup,
		backendtest.Budget{MaxAllocs: 17_000_000})
}

// BenchmarkSettle measures the settle over the suite's scaled
// corpus, the corpus build excluded from the measurement, under
// its own ceiling pinned from measurement with headroom.
func BenchmarkSettle(b *testing.B) {
	backendtest.BenchSettle(b, benchSetup,
		backendtest.Budget{MaxAllocs: 1_600_000})
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

			assert.Equal(t, backend.New().Name(), golang.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), golang.Target, "the rendering target")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, golang.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that exports neutral names through the settle", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), "type Row struct", "a neutral row exports as Row")
			assert.Contains(t, string(text), "func Fetch()", "and a neutral fetch as Fetch")
		})

		t.Run("returns a backend that imports the package a field's type names", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(&emit.Field{Name: "When", Type: imported(timePkg, timePkg, "Duration")}))
			assert.Length(t, collected(sink), 0, "the file renders clean")
			assert.Contains(t, body, "import (\n\t\"time\"\n)\n", "the import block")
			assert.Contains(t, body, "\tWhen time.Duration\n", "and the qualified field")
		})

		t.Run("returns a backend that renders two packages of one name under two names", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(
				&emit.Field{Name: "Current", Type: imported(storeName, storePkg, rowName)},
				&emit.Field{Name: "Legacy", Type: imported(storeName, legacyPkg, rowName)},
			))
			assert.Length(t, collected(sink), 0, "the file renders clean")
			assert.ContainsInOrder(t, body, []string{
				"\t" + storeName2 + " \"" + legacyPkg + "\"\n",
				"\t\"" + storePkg + "\"\n",
				"\tCurrent store.Row\n",
				"\tLegacy  " + storeName2 + ".Row\n",
			}, "the second package imports under a suffixed name")
		})

		t.Run("returns a backend that renders a pointer receiver a generator states", func(t *testing.T) {
			t.Parallel()

			row := rowOf()
			row.Methods.Append(golang.PointerReceiver(&emit.Method{
				Origin: symbol.Identity{
					Lang: golang.Lang, Package: svcPkg, Owner: rowName, Name: touchName, Kind: symbol.KindMethod,
				},
				Name:     touchName,
				Receives: &emit.TypeRef{Spelling: rowName},
			}))
			body, sink := rendered(t, row)
			assert.Length(t, collected(sink), 0, "the file renders clean")
			assert.Contains(t, body, "func (r *Row) Touch() {", "the receiver points at the host")
		})

		t.Run("reports a declaration whose reference records no package", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(&emit.Field{Name: "When", Type: ref("time.Duration")}))
			assert.Length(t, collected(sink), 1, "one finding for the skipped declaration")
			assert.NotContains(t, body, "When", "the declaration is skipped")
		})
	})
}

// collected returns every finding a sink contains.
func collected(sink *diag.Sink) []diag.Diag {
	var out []diag.Diag
	for d := range sink.All() {
		out = append(out, d)
	}
	return out
}
