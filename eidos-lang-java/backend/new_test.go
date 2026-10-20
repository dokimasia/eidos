// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants are the brand of the end-to-end fixture, under which
// the output contract stamps, and the unit of a rendered declaration.
const (
	contractBrand = "java"
	unitPkg       = "svc/app"
	unitKey       = "svc/app/Holder.java"
	unitWord      = "gen"
	holderName    = "Holder"
)

// doubleName is the name of a generated class in the neutral convention,
// and settledDouble is the name that the settle gives it.
const (
	doubleName    = "stubRow"
	settledDouble = "StubRow"
)

// These constants are the allocations of a build and of the scaled
// corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template and the three kind templates, and the function maps that
	// text/template keeps for each template.
	newAllocs = 1_503
	// renderAllocs is one render of the scaled corpus, nearly all of its
	// allocations in text/template's execution and its reflective calls.
	// One run counted 22,812,619, and the budget is a round number about
	// 5% above it.
	renderAllocs = 24_000_000
	// settleAllocs is one settle of the scaled corpus. With the collector
	// off a settle allocates 1,986,667 times: the lowering pass, about two
	// thirds of them, and the respelling of every name, about a third. The
	// collections that run during a settle add more: 10 fresh processes
	// counted up to 5 more. The ceiling allows 32 more.
	settleAllocs = 1_986_667 + 32
)

// allocCall is one call that an allocation test and a benchmark share.
// It has the method that it calls, whose name is the name of its
// benchmark, the case that it measures where the method has more than
// one call, its allocation ceiling, the call, and the check of the
// result of the call. A case whose ceiling only a benchmark checks sets
// bench, which measures the case in place of the call. No list that an
// allocation test reads contains such a case.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// The backend is the write half of the module. The kernel suite runs the
// render checks over it on the canonical fixture, and the stamp check
// joins it to the output contract under the comment forms of the module.
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

		t.Run("returns a backend whose spoke spells a type of another language", func(t *testing.T) {
			t.Parallel()

			got, err := backend.New().SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, "String", "the backend's spoke is spell.Type")
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
			assert.That(t, string(text)).
				Contains("public class Row", "a neutral row takes Pascal").
				Contains("public void boot()", "a neutral boot keeps camel case").
				Contains("public final class ShapeCircle implements Shape",
					"a lowered variant implements the respelled principal").
				Contains("public sealed interface Shape permits ShapeCircle, ShapeEmpty",
					"the sealed principal permits the respelled classes")
		})

		t.Run("returns a backend that imports the class a field's type names", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storePkg, rowName)})
			body, findings := rendered(t, holder)
			assert.Empty(t, findings, "the file renders clean")
			assert.That(t, body).
				Contains("import svc.store.Row;\n", "the class's import").
				Contains("    public Row row;\n", "the field through the simple name")
		})

		t.Run("returns a backend that names the settled class in an array", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "rows", Type: &emit.TypeRef{
				Form:     symbol.FormList,
				Spelling: doubleName + "[]",
				Elems:    []*emit.TypeRef{{Spelling: doubleName}},
			}})
			body, findings := rendered(t, declared(doubleName), holder)
			assert.Empty(t, findings, "the file renders clean")
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
// the contract starts, so the count leaves out what the first call
// initialises. A case that sets bench runs it instead.
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
// which emits every file-level kind: the backend spells the three
// type kinds, with every body content form arriving through the
// class template's members, lowers the sum into the interface and
// its classes, and refuses the rest.
func setup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	return backend.New(), backendtest.CanonicalFixture(tb)
}

// benchSetup builds the backend over the suite's scaled corpus,
// without the kinds the backend refuses.
func benchSetup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	return backend.New(), backendtest.ScaledFixture(tb, backend.RefusedKinds())
}

// rendered settles one unit of declarations through the backend and
// renders it. It returns the body of the first file and the findings of
// the run.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, []diag.Diag) {
	tb.Helper()

	files, findings := backendtest.Rendered(tb, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
		e := plugin.NewEmit()
		assert.NoError(tb, e.Add(plugin.Unit{
			Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
			Pkg:   symbol.Identity{Lang: java.Lang, Package: unitPkg, Kind: symbol.KindPackage},
			Decls: decls,
		}), "the unit is added")
		return backend.New(), &backendtest.Fixture{Emit: e}
	})
	if len(files) == 0 {
		return "", findings
	}
	return string(files[0].Body), findings
}

// declared returns a struct of the unit's package named name.
func declared(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{Lang: java.Lang, Package: unitPkg, Name: name, Kind: symbol.KindStruct},
		Name:   name,
	}
}
