// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants name the end-to-end fixture's brand, which the
// output contract stamps under, the file a unit of svcPkg renders into,
// the method a stated receiver attaches to its host, and a field of a
// function type.
const (
	contractBrand = "golang"
	unitKey       = svcPkg + "/row.go"
	unitWord      = "gen"
	touchName     = "touch"
	visitName     = "Visit"
)

// emittedName is a generated struct's name in the neutral convention,
// and settledName the name the settle gives it for a public
// declaration.
const (
	emittedName = "stubStore"
	settledName = "StubStore"
)

// The allocations of a build and of the scaled corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template and the seven kind templates.
	newAllocs = 1_602
	// renderAllocs is one render of the scaled corpus of 200,000
	// declarations: text/template's reflective calls, about half of its
	// allocations, go/format's parse and print of every file, about four
	// in ten, and the vocabulary's spellings. One run counted 20,763,550,
	// and the budget is a round number about 5% above it.
	renderAllocs = 21_800_000
	// settleAllocs is one settle of the scaled corpus of 200,000
	// declarations: the lowering of sums, enums and receivers, the
	// respelling of every name, and the reindex. One run counted
	// 1,817,925, and the budget is a round number about 5% above it.
	settleAllocs = 1_910_000
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

			assert.Equal(t, backend.New().Name(), golang.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), golang.Target, "the rendering target")
		})

		t.Run("returns a backend whose spoke spells a type of another language", func(t *testing.T) {
			t.Parallel()

			speller, spells := backend.New().(plugin.TypeSpeller)
			assert.True(t, spells, "the backend serves the spoke")
			got, err := speller.SpellType(rules.Leaf(symbol.FormText, "String"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, "string", "the backend's spoke is spell.Type")
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
			assert.That(t, string(text)).
				Contains("type Row struct", "a neutral row exports as Row").
				Contains("func Fetch()", "and a neutral fetch as Fetch")
		})

		t.Run("returns a backend that imports the package a field's type names", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(&emit.Field{Name: "When", Type: imported(timePkg, timePkg, "Duration")}))
			assert.Empty(t, collected(sink), "the file renders clean")
			assert.That(t, body).
				Contains("import (\n\t\"time\"\n)\n", "the import block").
				Contains("\tWhen time.Duration\n", "and the qualified field")
		})

		t.Run("returns a backend that renders two packages of one name under two names", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(
				&emit.Field{Name: "Current", Type: imported(storeName, storePkg, rowName)},
				&emit.Field{Name: "Legacy", Type: imported(storeName, legacyPkg, rowName)},
			))
			assert.Empty(t, collected(sink), "the file renders clean")
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
			assert.Empty(t, collected(sink), "the file renders clean")
			assert.Contains(t, body, "func (r *Row) Touch() {", "the receiver points at the host")
		})

		t.Run("returns a backend that renders a pointer receiver over a host the settle respells", func(t *testing.T) {
			t.Parallel()

			double := structOf(emittedName)
			double.Methods.Append(golang.PointerReceiver(&emit.Method{
				Origin: symbol.Identity{
					Lang: golang.Lang, Package: svcPkg, Owner: emittedName, Name: touchName, Kind: symbol.KindMethod,
				},
				Name:     touchName,
				Receives: &emit.TypeRef{Spelling: emittedName},
			}))
			body, sink := rendered(t, double)
			assert.Empty(t, collected(sink), "the file renders clean")
			assert.Contains(t, body, "func (s *"+settledName+") Touch() {", "the receiver names the settled host")
		})

		t.Run("returns a backend that renders a function type over a respelled declaration", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, structOf(emittedName), rowOf(&emit.Field{Name: visitName, Type: &emit.TypeRef{
				Form:     symbol.FormFunc,
				Spelling: "func(next " + emittedName + ") error",
				Split:    1,
				Elems:    []*emit.TypeRef{{Spelling: emittedName}, {Spelling: "error"}},
			}}))
			assert.Empty(t, collected(sink), "the file renders clean")
			assert.Contains(t, body, "\t"+visitName+" func(next "+settledName+") error\n",
				"the function type names the settled declaration, its parameter's name as written")
		})

		t.Run("reports a declaration whose reference records no package", func(t *testing.T) {
			t.Parallel()

			body, sink := rendered(t, rowOf(&emit.Field{Name: "When", Type: ref("time.Duration")}))
			assert.Length(t, collected(sink), 1, "one finding for the skipped declaration")
			assert.NotContains(t, body, "When", "the declaration is skipped")
		})

		t.Run("reports a file without a package as a refused skeleton", func(t *testing.T) {
			t.Parallel()

			body, sink := renderedIn(t, symbol.Identity{}, rowOf())
			assert.Equal(t, body, "", "the file is withheld")
			findings := collected(sink)
			assert.Length(t, findings, 1, "one finding for the withheld file")
			assert.Equal(t, findings[0].Code, render.RefusedTemplate, "the skeleton refuses the file")
			assert.Contains(t, findings[0].Msg, "import base", "the finding names what the plan can state")
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
// templates and gofmt per file. The settle leaves the corpus build out
// of its count. Only -bench checks the two corpus ceilings. No count of
// [assert.MaxAllocs] leaves the corpus build out, as the settle's count
// does, and one render takes about 0.4 s on four cores, so an
// allocation check's 101 calls would take 40 s.
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
			name: "New", caseName: "the backend", allocs: newAllocs,
			call:  func() { b = backend.New() },
			check: func(tb assert.TB) { assert.Equal(tb, b.Name(), golang.Name, "New returns the Go backend") },
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
// the enum into a defined type and constants, and lowers the sum into
// an interface and its variant structs.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.CanonicalFixture(tb)
}

// benchSetup builds the backend over the suite's scaled corpus of every
// canonical kind, because the backend refuses none.
func benchSetup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.ScaledFixture(tb, nil)
}

// structOf returns a struct of svcPkg named name.
func structOf(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{Lang: golang.Lang, Package: svcPkg, Name: name, Kind: symbol.KindStruct},
		Name:   name,
	}
}

// rowOf returns a struct of svcPkg named rowName with the given
// fields.
func rowOf(fields ...*emit.Field) *emit.Struct {
	row := structOf(rowName)
	row.Fields.Append(fields...)
	return row
}

// rendered settles one unit of svcPkg through the backend and renders
// it, and returns the file beside the run's findings.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, *diag.Sink) {
	tb.Helper()

	return renderedIn(tb, symbol.Identity{Lang: golang.Lang, Package: svcPkg, Kind: symbol.KindPackage}, decls...)
}

// renderedIn settles one unit of a package through the backend and
// renders it, and returns the file beside the run's findings. The file
// declares the unit's package, so the zero package is a file the
// layout derives no package for.
func renderedIn(tb assert.TB, pkg symbol.Identity, decls ...symbol.Symbol) (string, *diag.Sink) {
	tb.Helper()

	b := backend.New()
	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(plugin.Unit{
		Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey, Pkg: pkg, Decls: decls,
	}), "the unit is added")
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles")
	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	s, spells := b.(plugin.FileSpeller)
	assert.True(tb, spells, "the built backend spells filenames")
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, s), Sink: sink, Plugin: golang.Name,
	})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// collected returns every finding a sink contains.
func collected(sink *diag.Sink) []diag.Diag {
	var out []diag.Diag
	for d := range sink.All() {
		out = append(out, d)
	}
	return out
}
