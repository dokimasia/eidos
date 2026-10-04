// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants name the end-to-end fixture's brand, which the
// output contract stamps under, and the unit a rendered declaration
// comes from.
const (
	contractBrand = "java"
	unitPkg       = "svc/app"
	unitKey       = "svc/app/Holder.java"
	unitWord      = "gen"
	holderName    = "Holder"
)

// doubleName is a generated class's name in the neutral convention,
// and settledDouble the name the settle gives it.
const (
	doubleName    = "stubRow"
	settledDouble = "StubRow"
)

// The allocations of a build and of the scaled corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template and the three kind templates.
	newAllocs = 1_512
	// renderAllocs is one render of the scaled corpus, nearly all of its
	// allocations in text/template's execution and its reflective calls.
	// The count varies between processes: 13 fresh processes, three of
	// them with the collector off, counted 27,063,032 to 27,063,477. The
	// ceiling allows 512 above the lowest.
	renderAllocs = 27_063_032 + 512
	// settleAllocs is one settle of the scaled corpus. With the collector
	// off a settle allocates 2,186,665 times: the lowering pass, about two
	// thirds of them, and the respelling of every name, about a third. The
	// collections that run during a settle add more: 10 fresh processes
	// counted up to 5 more. The ceiling allows 32 more.
	settleAllocs = 2_186_665 + 32
)

// allocCall is one call that an allocation test and a benchmark share:
// its benchmark path, its allocation ceiling, the call, and the check
// of the result the call leaves.
type allocCall struct {
	name   string
	allocs uint64
	call   func()
	check  func(tb assert.TB)
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

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storePkg, rowName)})
			body, sink := rendered(t, holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "import svc.store.Row;\n", "the class's import")
			assert.Contains(t, body, "    public Row row;\n", "the field through the simple name")
		})

		t.Run("returns a backend that names the settled class in an array", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "rows", Type: &emit.TypeRef{
				Form:     symbol.FormList,
				Spelling: doubleName + "[]",
				Elems:    []*emit.TypeRef{{Spelling: doubleName}},
			}})
			body, sink := rendered(t, declared(doubleName), holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "    public "+settledDouble+"[] rows;\n", "the array names the settled class")
		})
	})
}

// The build allocates the kit's backend. The ordinary run, which runs
// no benchmark, checks that ceiling here.
func TestNewAllocs(t *testing.T) {
	checkAllocs(t, newCalls())
}

// BenchmarkNew measures the build of the backend, and the composed
// backend's settle and render of the suite's scaled corpus. The split
// fans every typed declaration into its own file, so the corpus renders
// far more files than units. The settle leaves the corpus build out of
// its count. Only -bench checks the two corpus ceilings. No count of
// [assert.MaxAllocs] leaves the corpus build out, as the settle's count
// does, and one render takes about 1 s on four cores, so an allocation
// check's 101 calls would take nearly two minutes.
func BenchmarkNew(b *testing.B) {
	benchCalls(b, newCalls())

	b.Run("New/the render of the scaled corpus", func(b *testing.B) {
		backendtest.BenchRender(b, benchSetup, backendtest.Budget{MaxAllocs: renderAllocs})
	})
	b.Run("New/the settle of the scaled corpus", func(b *testing.B) {
		backendtest.BenchSettle(b, benchSetup, backendtest.Budget{MaxAllocs: settleAllocs})
	})
}

// newCalls returns a call of New.
func newCalls() []allocCall {
	var b plugin.Backend
	return []allocCall{
		{
			name: "New", allocs: newAllocs,
			call:  func() { b = backend.New() },
			check: func(tb assert.TB) { assert.Equal(tb, b.Name(), java.Name, "New returns the Java backend") },
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and
// the result each call leaves.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling, one sub-benchmark each. Each call runs once before the
// contract starts, so what the first call initialises stays out of the
// count.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	for _, tt := range calls {
		b.Run(tt.name, func(b *testing.B) {
			tt.call()
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
			tt.check(b)
		})
	}
}

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
	s, spells := b.(plugin.FileSpeller)
	assert.True(tb, spells, "the built backend spells filenames")
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, s), Sink: sink, Plugin: java.Name,
	})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// declared returns a struct of the unit's package named name.
func declared(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{Lang: java.Lang, Package: unitPkg, Name: name, Kind: symbol.KindStruct},
		Name:   name,
	}
}
