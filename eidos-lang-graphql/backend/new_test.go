// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contractBrand is the brand of the end-to-end fixture, under which the
// output contract stamps.
const contractBrand = "graphql"

// idName is the name of the canonical alias, and scalarName is the name that
// the adapted fixture gives it.
const (
	idName     = "id"
	scalarName = "identifier"
)

// These constants are the allocations of a build and of the scaled
// corpus.
const (
	// newAllocs is a build of the backend, chiefly the parse of the file
	// template, the five kind templates and the extend template, and the
	// function maps that text/template keeps for each template.
	newAllocs = 434
	// renderAllocs is one render of the scaled corpus of 10,000 files. One
	// run counted 1,583,833: about 1.1 million in text/template's execution
	// of the kind templates and its reflective calls of the vocabulary, about
	// 267,000 in the text of each declaration that the vocabulary writes, and
	// the rest in the pass over the files. The budget is a round number about
	// 5% above the count.
	renderAllocs = 1_660_000
	// settleAllocs is one settle of the scaled corpus. One run counted
	// 1,134,821: about 343,000 in the lowering of the 28,571 sums into unions
	// and object types, about 257,000 in the parse that checks the selection
	// set of each of the 28,572 operations, and the rest in the plan of the
	// names and the respelling of every reference. The budget is a round
	// number about 5% above the count.
	settleAllocs = 1_190_000
)

// canonicalFiles are the files that the backend writes for the canonical
// fixture, by the line that opens the declaration of each. The generic
// siblings and the declarations of the kinds that GraphQL refuses do not
// render.
var canonicalFiles = map[string]string{
	"enum Phase": "\"phase names a lifecycle step.\"\nenum Phase { # closed set\n" +
		"  \"open admits writes.\"\n  OPEN\n  CLOSED\n}\n",
	"union Shape": "\"shape is one closed figure.\"\nunion Shape = ShapeCircle | ShapeEmpty # tagged\n\n" +
		"\"circle bounds by a radius.\"\ntype ShapeCircle { # round\n  radius: int!\n}\n\n" +
		"type ShapeEmpty {\n  size: int!\n}\n",
	"type Row": "\"Row is one canonical record.\"\ntype Row { # one per fetch\n" +
		"  \"Name keys the row.\"\n  name: string! # unique per store\n" +
		"  boot: String!\n  fetch: String!\n  push: String!\n  trace: String!\n}\n",
	"interface Store": "\"Store reads rows back.\"\ninterface Store implements Closer { # read side\n" +
		"  \"Get returns the row key names.\"\n  get(\n    key: string! # the row key\n  ): string! # by key\n}\n",
	"query Boot": "query Boot " + selection + "\n\nquery Fetch " + selection + "\n\nquery Push " + selection +
		"\n\nquery Trace " + selection + "\n",
	"extend type Row":   "extend type Row {\n  track: String! # detached\n}\n",
	"scalar Identifier": "\"ID names a row.\"\nscalar Identifier # opaque\n",
}

// The backend is the write half of the module. The kernel suite runs the
// render checks over it on the canonical fixture, whose declarations take
// what GraphQL needs, and the stamp check joins it to the output contract
// under the comment forms of the module.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		backendtest.RunBackendSuite(t, setup)

		t.Run("returns a backend named after the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Name(), graphql.Name, "the plugin identity")
		})

		t.Run("returns a backend for the language's target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Target(), graphql.Target, "the rendering target")
		})

		t.Run("returns a backend whose spoke spells a type of another language", func(t *testing.T) {
			t.Parallel()

			got, err := backend.New().SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got.Spelling, graphql.StringScalar, "the backend's spoke is spell.Type")
		})

		t.Run("returns a backend whose policies are the choices of the spoke", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.New().Policies(), graphql.Policies(), "the policies of the satellite")
		})

		t.Run("returns a backend whose files stamp under the module's contract", func(t *testing.T) {
			t.Parallel()

			c, err := output.NewContract(contractBrand, graphql.Syntax())
			assert.NoError(t, err, "the module contract composes")
			backendtest.AssertStamped(t, setup, c)
		})

		t.Run("returns a backend that writes each file of the canonical fixture", func(t *testing.T) {
			t.Parallel()

			files, _ := backendtest.Rendered(t, setup)
			got := map[string]string{}
			for _, f := range files {
				body := string(f.Body)
				for opening := range canonicalFiles {
					if strings.HasPrefix(body, opening) || strings.Contains(body, "\n"+opening+" ") {
						got[opening] = body
					}
				}
			}
			assert.Equal(t, got, canonicalFiles, "the file of each declaration that renders")
		})
	})
}

// The build allocates the kit's backend. The ordinary run, which runs no
// benchmark, checks that ceiling here.
func TestNewAllocs(t *testing.T) {
	checkAllocs(t, newCalls())
}

// BenchmarkNew measures the build of the backend, and the composed
// backend's settle and render of the suite's scaled corpus: the real
// templates, the extend clustering and the normalizer. The settle leaves
// the corpus build out of its count. Only -bench checks the two corpus
// ceilings, because no count of [assert.MaxAllocs] leaves the corpus build
// out, as the settle's count does.
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
			check: func(tb assert.TB) { assert.Equal(tb, b.Name(), graphql.Name, "New returns the GraphQL backend") },
		},
	}
}

// setup builds the backend over the kernel's canonical fixture, adapted to
// what GraphQL needs. The fixture emits every file-level kind: the backend
// writes the struct, the enum, the sum as a union, the interface, the
// functions as operations, the extension method and the alias as a scalar,
// rejects their generic siblings, and refuses the other kinds.
func setup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.CanonicalFixture(tb)
	adapted(f)
	return backend.New(), f
}

// benchSetup builds the backend over the suite's scaled corpus without the
// kinds that the backend refuses, adapted to what GraphQL needs. Its
// declarations lose their type parameters, so the render measures the
// declarations and no rejection.
func benchSetup(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
	tb.Helper()

	f := backendtest.ScaledFixture(tb, backend.RefusedKinds())
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			switch x := d.(type) {
			case *emit.Struct:
				x.TypeParams = nil
				for _, m := range x.Methods.Items() {
					m.TypeParams = nil
				}
			case *emit.Sum:
				x.TypeParams = nil
			case *emit.Interface:
				x.TypeParams = nil
				for _, m := range x.Methods.Items() {
					m.TypeParams = nil
				}
			case *emit.Function:
				x.TypeParams = nil
			case *emit.Method:
				x.TypeParams = nil
			case *emit.Alias:
				x.TypeParams = nil
			}
		}
	}
	adapted(f)
	return backend.New(), f
}

// adapted states in place what GraphQL needs of the declarations of the
// fixture f and other languages leave out: the marker of a query and a
// selection set of each function without type parameters, a result of each
// method without one, a field of each variant without one, and a scalar for
// each alias without type parameters. The alias id takes the name
// [scalarName], because GraphQL spells id as ID, a built-in scalar.
func adapted(f *backendtest.Fixture) {
	result := func(m *emit.Method) {
		if m.Returns.Len() == 0 {
			m.Returns = emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}})
		}
	}
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			switch x := d.(type) {
			case *emit.Function:
				if len(x.TypeParams) == 0 {
					x.Annotations = append(x.Annotations, symbol.Annotation{
						Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationQuery)},
					})
					x.Body = emit.Body{Verbatim: selection}
				}
			case *emit.Method:
				result(x)
			case *emit.Struct:
				for _, m := range x.Methods.Items() {
					result(m)
				}
			case *emit.Sum:
				for _, v := range x.Variants.Items() {
					if v.Fields.Len() == 0 {
						v.Fields = emit.SlotOf(&emit.Field{Name: "size", Type: &emit.TypeRef{Spelling: "int"}})
					}
				}
			case *emit.Alias:
				if len(x.TypeParams) == 0 {
					x.Target, x.Defined = nil, true
				}
				if x.Name == idName {
					x.Name = scalarName
				}
			}
		}
	}
}
