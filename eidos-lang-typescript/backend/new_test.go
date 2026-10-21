// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants are the brand of the end-to-end fixture, under which
// the output contract stamps, and the unit of a rendered declaration.
const (
	contractBrand = "typescript"
	unitModule    = "./svc"
	unitKey       = "svc/holder.ts"
	unitWord      = "gen"
	holderName    = "Holder"
)

// doubleName is the name of a generated class in the neutral convention,
// and settledDouble is the name that the settle gives it.
const (
	doubleName    = "stubRow"
	settledDouble = "StubRow"
)

// The members that the cases name after reserved words of ECMAScript,
// and the parameter of the method.
const (
	reservedMethod = "delete"
	reservedField  = "default"
	deletedKey     = "key"
)

// The source module of a builder unit, the class that the module declares,
// the family word of the unit, and the file that a plan routes it into.
const (
	catalogModule = "catalog"
	accountName   = "Account"
	builderWord   = "builder"
	builderPath   = "catalog.builder.ts"
)

// These constants are the allocations of a build and of the scaled
// corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template and the seven kind templates. The seven lowering policies
	// add eleven: their specs, the kit's list of them, and the check of the
	// specs that Build makes. Each of the two vocabularies that the build
	// binds allocates the twelve bound helpers of the speller.
	newAllocs = 1_844
	// renderAllocs is one render of the scaled corpus, nearly all of its
	// allocations in text/template's execution and its reflective calls.
	// One run counted 8,851,050, and the budget is a round number about
	// 5% above it.
	renderAllocs = 9_300_000
	// settleAllocs is one settle of the scaled corpus. With the collector
	// off a settle allocates 1,075,969 times: the sum lowering, about three
	// fifths of them, the respelling of every name and the rewrite of
	// references, about a third, and the reindex. The collections that run
	// during a settle add more: fresh processes counted up to 12 more. The
	// ceiling allows 32 more.
	settleAllocs = 1_075_969 + 32
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

			assert.Equal(t, backend.New().Name(), typescript.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), typescript.Target, "the rendering target")
		})

		t.Run("returns a backend that declares the TypeScript policies", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Policies(), typescript.Policies(), "the policies are the satellite's")
		})

		t.Run("returns a backend whose spoke spells a type under the plan's policy", func(t *testing.T) {
			t.Parallel()

			p, err := plugin.NewPolicy(typescript.Target, typescript.Policies(),
				map[plugin.PolicyKey]plugin.Choice{typescript.Int64: typescript.String})
			assert.NoError(t, err, "the policy resolves")
			got, err := backend.New().SpellType(rules.Scalar("int64", rules.ScalarInt, 64), p)
			assert.NoError(t, err, "the spoke spells the integer")
			assert.Equal(t, got.Spelling, "string", "the backend's spoke reads the policy's choice")
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
			assert.Contains(t, string(text), `  kind: "circle";`,
				"a lowered variant leads with its discriminant, quoted in TypeScript's grammar")
			assert.Contains(t, string(text), "export type Shape = ShapeCircle | ShapeEmpty;",
				"the union alias joins the lowered interfaces")
		})

		t.Run("returns a backend that imports the declaration a field's type names", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "row", Type: imported(storeModule, rowName)})
			body, findings := rendered(t, holder)
			assert.Empty(t, findings, "the file renders clean")
			assert.That(t, body).
				Contains("import type { Row } from \"./store\";\n", "the type-only import").
				Contains("  row: Row;\n", "the field through the imported name")
		})

		t.Run("returns a backend that writes the strings of a plan in the quote of its policy", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(
				&emit.Field{Name: "row", Type: imported(storeModule, rowName)},
				&emit.Field{Name: wireName, Type: ref(doubleLiterals)},
			)
			body, findings := renderedUnder(t, singleQuoted(t).Policy(), holder)
			assert.Empty(t, findings, "the file renders clean")
			assert.That(t, body).
				Contains("import type { Row } from './store';\n", "the specifier in the single quote").
				Contains("  "+singleWire+": "+singleLiterals+";\n", "the key and the literals in the single quote")
		})

		t.Run("returns a backend that imports a class of the source module of the unit", func(t *testing.T) {
			t.Parallel()

			module := symbol.Identity{Lang: typescript.Lang, Package: catalogModule, Kind: symbol.KindPackage}
			account := symbol.Identity{
				Lang: typescript.Lang, Package: catalogModule, Name: accountName, Kind: symbol.KindStruct,
			}
			builder := &emit.Struct{Origin: account, Name: accountName + "Builder", Visibility: symbol.VisibilityPublic}
			builder.Fields.Append(&emit.Field{
				Name: "value", Type: &emit.TypeRef{Spelling: accountName, Target: account},
				Visibility: symbol.VisibilityPublic,
			})
			files, findings := backendtest.Rendered(t, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
				e := plugin.NewEmit()
				assert.NoError(tb, e.Add(plugin.Unit{
					Plugin: unitWord, Per: plugin.PerSource, Word: builderWord,
					Key: catalogModule + typescript.Extension, Pkg: module, Decls: []symbol.Symbol{builder},
				}), "the unit is added")
				return backend.New(), &backendtest.Fixture{Emit: e}
			})
			assert.Empty(t, findings, "the file renders clean")
			assert.Length(t, files, 1, "the unit renders into one file")
			assert.Equal(t, files[0].Path, builderPath, "the file is beside the source module, as a plan routes it")
			assert.Contains(t, string(files[0].Body), "import type { Account } from \"./catalog\";\n",
				"the file imports the class from the source module")
		})

		t.Run("returns a backend that writes the overload signatures of a function before its implementation",
			func(t *testing.T) {
				t.Parallel()

				body, findings := rendered(t, loader("unknown", false), loader("number", true))
				assert.Empty(t, findings, "the file renders clean")
				assert.ContainsInOrder(t, body, []string{
					"export function load(id: number): Row;\n", "export function load(id: unknown): Row {\n",
				}, "the signature right before the implementation")
			})

		t.Run("returns a backend that skips the overload signatures of a function without an implementation",
			func(t *testing.T) {
				t.Parallel()

				_, findings := rendered(t, declared(rowName), loader("number", true))
				assert.Length(t, findings, 1, "one finding")
				assert.Equal(t, findings[0].Code, render.UnimplementedOverload,
					"TypeScript requires the implementation")
			})

		t.Run("returns a backend that writes a class after the class that it extends", func(t *testing.T) {
			t.Parallel()

			base := declared(baseName)
			derived := declared(holderName)
			derived.Extends = []*emit.TypeRef{{Spelling: baseName, Target: base.Origin}}
			body, findings := rendered(t, derived, base)
			assert.Empty(t, findings, "the file renders clean")
			assert.ContainsInOrder(t, body, []string{"export class " + baseName, "extends " + baseName},
				"TypeScript defines a class before a class extends it")
		})

		t.Run("returns a backend that writes a method named after a reserved word", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Methods.Append(&emit.Method{
				Name: reservedMethod, Visibility: symbol.VisibilityPublic,
				Params: emit.SlotOf(&emit.Param{Name: deletedKey, Type: ref("string")}),
			})
			body, findings := rendered(t, holder)
			assert.Empty(t, findings, "the file renders clean")
			assert.Contains(t, body, "  "+reservedMethod+"("+deletedKey+": string): void {\n",
				"the method keeps its name, which TypeScript reads as a property name")
		})

		t.Run("returns a backend that writes a field named after a reserved word", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: reservedField, Type: ref("string")})
			body, findings := rendered(t, holder)
			assert.Empty(t, findings, "the file renders clean")
			assert.Contains(t, body, "  "+reservedField+": string;\n",
				"the field keeps its name, which TypeScript reads as a property name")
		})

		t.Run("returns a backend that names the settled class in a list", func(t *testing.T) {
			t.Parallel()

			holder := declared(holderName)
			holder.Fields.Append(&emit.Field{Name: "rows", Type: &emit.TypeRef{
				Form:     symbol.FormList,
				Spelling: "readonly " + doubleName + "[]",
				Elems:    []*emit.TypeRef{{Spelling: doubleName}},
			}})
			body, findings := rendered(t, declared(doubleName), holder)
			assert.Empty(t, findings, "the file renders clean")
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
// which emits every file-level kind: the backend spells each, lowers
// the sum into variant interfaces and a union alias, and refuses the
// standalone method.
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
// renders it under the zero policy. It returns the body of the file and
// the findings of the run.
func rendered(tb assert.TB, decls ...symbol.Symbol) (string, []diag.Diag) {
	tb.Helper()

	return renderedUnder(tb, plugin.Policy{}, decls...)
}

// renderedUnder settles one unit of declarations through the backend and
// renders it under the policy p. It returns the body of the file and the
// findings of the run.
func renderedUnder(tb assert.TB, p plugin.Policy, decls ...symbol.Symbol) (string, []diag.Diag) {
	tb.Helper()

	files, findings := backendtest.Rendered(tb, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
		e := plugin.NewEmit()
		assert.NoError(tb, e.Add(plugin.Unit{
			Plugin: unitWord, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
			Pkg:   symbol.Identity{Lang: typescript.Lang, Package: unitModule, Kind: symbol.KindPackage},
			Decls: decls,
		}), "the unit is added")
		return backend.New(), &backendtest.Fixture{Emit: e, Policy: p}
	})
	if len(files) == 0 {
		return "", findings
	}
	return string(files[0].Body), findings
}

// singleQuoted returns an empty import set whose policy selects the single
// quote.
func singleQuoted(tb assert.TB) *render.ImportSet {
	tb.Helper()

	p, err := plugin.NewPolicy(typescript.Target, typescript.Policies(),
		map[plugin.PolicyKey]plugin.Choice{typescript.QuoteStyle: typescript.SingleQuote})
	assert.NoError(tb, err, "the policy resolves")
	set := &render.ImportSet{}
	set.SetPolicy(p)
	return set
}

// declared returns a struct of the unit's module named name.
func declared(name string) *emit.Struct {
	return &emit.Struct{
		Origin: symbol.Identity{Lang: typescript.Lang, Package: unitModule, Name: name, Kind: symbol.KindStruct},
		Name:   name,
	}
}

// loader returns the function load of the unit's module, which takes an id
// of the type param and returns a Row, marked as an overload signature
// where signature is set.
func loader(param string, signature bool) *emit.Function {
	return &emit.Function{
		Origin: symbol.Identity{
			Lang: typescript.Lang, Package: unitModule, Name: loadName, Kind: symbol.KindFunction, Disc: param,
		},
		Name: loadName, Overload: signature,
		Params:  emit.SlotOf(&emit.Param{Name: "id", Type: ref(param)}),
		Returns: emit.SlotOf(&emit.Return{Type: ref(rowName)}),
	}
}
