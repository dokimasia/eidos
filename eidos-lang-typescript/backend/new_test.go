// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
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
	contractBrand = "typescript"
	unitModule    = "./svc"
	unitKey       = "svc/holder.ts"
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
	// template and the seven kind templates.
	newAllocs = 1_742
	// renderAllocs is one render of the scaled corpus, nearly all of its
	// allocations in text/template's execution and its reflective calls.
	// The count varies between processes: 13 fresh processes, three of
	// them with the collector off, counted 10,651,254 to 10,651,404. The
	// ceiling allows 256 above the lowest.
	renderAllocs = 10_651_254 + 256
	// settleAllocs is one settle of the scaled corpus. With the collector
	// off a settle allocates 1,125,967 times: the sum lowering, about half
	// of them, the respelling of every name, about a quarter, the rewrite
	// of references and the reindex. The collections that run during a
	// settle add more: fresh processes counted up to 12 more. The ceiling
	// allows 32 more.
	settleAllocs = 1_125_967 + 32
)

// allocCall is one call that an allocation test and a benchmark share:
// the method it calls, which names its benchmark, the case it measures
// where the method has more than one call, its allocation ceiling, the
// call, and the check of the result the call leaves. A case whose
// ceiling only a benchmark checks sets bench, which measures the case
// in place of the call, and no list an allocation test reads contains
// it.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
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

			assert.Equal(t, backend.New().Name(), typescript.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), typescript.Target, "the rendering target")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, typescript.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that spells neutral names through the settle", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), "export class Row", "a neutral row takes Pascal")
			assert.Contains(t, string(text), "  boot(): void {", "a neutral boot keeps camel case")
		})

		t.Run("returns a backend that lowers a sum into variant interfaces", func(t *testing.T) {
			t.Parallel()

			var text []byte
			for _, f := range backendtest.RenderSettled(t, setup) {
				text = append(text, f.Body...)
			}
			assert.Contains(t, string(text), `  kind: 'circle';`,
				"a lowered variant leads with its discriminant, quoted in TypeScript's grammar")
			assert.Contains(t, string(text), "export type Shape = ShapeCircle | ShapeEmpty;",
				"the union alias joins the lowered interfaces")
		})

		t.Run("returns a backend that imports the declaration a field's type names", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storeModule, rowName)})
			body, sink := rendered(t, holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "import type { Row } from './store';\n", "the type-only import")
			assert.Contains(t, body, "  row: Row;\n", "the field through the imported name")
		})

		t.Run("returns a backend that names the settled class in a list", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "rows", Type: &emit.TypeRef{
				Form:     symbol.FormList,
				Spelling: "readonly " + doubleName + "[]",
				Elems:    []*emit.TypeRef{{Spelling: doubleName}},
			}})
			body, sink := rendered(t, declared(doubleName), holder)
			for d := range sink.All() {
				t.Errorf("unexpected finding: %s", d.Msg)
			}
			assert.Contains(t, body, "  rows: readonly "+settledDouble+"[];\n",
				"the list names the settled class, readonly as written")
		})
	})
}

// The build allocates the kit's backend. The ordinary run, which runs
// no benchmark, checks that ceiling here.
func TestNewAllocs(t *testing.T) {
	checkAllocs(t, newCalls())
}

// BenchmarkNew measures the build of the backend, and the composed
// backend's settle and render of the suite's scaled corpus: the real
// templates, the sum lowering and the shared normalizer. The settle
// leaves the corpus build out of its count. Only -bench checks the two
// corpus ceilings. No count of [assert.MaxAllocs] leaves the corpus
// build out, as the settle's count does, and one render takes about
// 0.3 s on four cores, so an allocation check's 101 calls would take
// half a minute.
func BenchmarkNew(b *testing.B) {
	benchCalls(b, append(newCalls(),
		allocCall{name: "New", caseName: "the render of the scaled corpus", bench: func(b *testing.B) {
			b.Helper()
			backendtest.BenchRender(b, benchSetup, backendtest.Budget{MaxAllocs: renderAllocs})
		}},
		allocCall{name: "New", caseName: "the settle of the scaled corpus", bench: func(b *testing.B) {
			b.Helper()
			backendtest.BenchSettle(b, benchSetup, backendtest.Budget{MaxAllocs: settleAllocs})
		}},
	))
}

// newCalls returns a call of New.
func newCalls() []allocCall {
	var b plugin.Backend
	return []allocCall{
		{
			name:     "New",
			caseName: "the backend",
			allocs:   newAllocs,
			call:     func() { b = backend.New() },
			check:    func(tb assert.TB) { assert.Equal(tb, b.Name(), typescript.Name, "New returns the TypeScript backend") },
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and
// the result each call leaves.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling: one sub-benchmark for each method, in the order the methods
// first appear, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling,
// and checks the result the last call leaves. The call runs once before
// the contract starts, so what the first call initialises stays out of
// the count. A case that sets bench runs it instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	tt.call()
	c := bench.Start(b).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// setup builds the backend over the kernel's canonical fixture,
// which emits every file-level kind: the backend spells each, lowers
// the sum into variant interfaces and a union alias, and refuses the
// standalone method.
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
		Pkg:   symbol.Identity{Lang: typescript.Lang, Package: unitModule, Kind: symbol.KindPackage},
		Decls: decls,
	}), "the unit is added")
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles")
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	s, spells := b.(plugin.FileSpeller)
	assert.True(tb, spells, "the built backend spells filenames")
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, s), Sink: sink, Plugin: typescript.Name,
	})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// declared returns a struct of the unit's module named name.
func declared(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{Lang: typescript.Lang, Package: unitModule, Name: name, Kind: symbol.KindStruct},
		Name:   name,
	}
}
