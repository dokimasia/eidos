// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// ghostName is a declaration the fixture never declares.
const ghostName = "Ghost"

// otherParamName is the second type parameter the witness cases
// declare, beside paramName.
const otherParamName = "U"

// The generic reference the restatement case reads: a map from
// string to List[int].
const (
	listName    = "List"
	mapSpelling = "map[string]List[int]"
)

// outsidePackage and outsideName name a type outside the workspace,
// which a witness or a reference imports without a target.
const (
	outsidePackage = "time"
	outsideName    = "Duration"
)

// The texts and the width the pair constructors are driven with.
const (
	pairSample    = "on"
	pairAlternate = "off"
	pairBits      = 32
)

// The pair the scripted language derives for an int, and the values
// the precedence cases state beside it.
const (
	derivedInt           = "42"
	derivedAlternateInt  = "7"
	authoredCount        = "3"
	authoredOtherCount   = "4"
	authoredRowAlternate = "Row{N: 1}"
	oneText              = "1"
	twoText              = "2"
	threeText            = "3"
)

// emitRefAllocs is the restatement of a reference without children: the
// emit reference.
const emitRefAllocs = 1

// deriving counts the calls to the language's SamplesOf, so a case
// proves the kernel derives only what no author stated.
type deriving struct {
	rules.SourceRules
	asked int
}

// SamplesOf counts the call and returns the wrapped rules' pair.
func (d *deriving) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	d.asked++
	return d.SourceRules.SamplesOf(ref, hint, v)
}

// Authored values take precedence over derived ones by contract, and
// every refusal names its reason, which a check generator reads.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("Sample.OK", func(t *testing.T) {
		t.Parallel()

		valued := emit.Literal(emit.LiteralInt, derivedInt)
		tests := []struct {
			name string
			give rules.Sample
			want bool
		}{
			{name: "reports true for a sample with a value", give: rules.Of(valued), want: true},
			{name: "reports false for a refused sample", give: rules.Refused(rules.RefusedDepth), want: false},
			{
				name: "reports false for a refused sample with a value",
				give: rules.Sample{Value: valued, Refusal: rules.RefusedDepth},
				want: false,
			},
			{name: "reports false for a sample without a value", give: rules.Sample{}, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.OK(), tt.want, "OK reports whether the sample has a derived value")
			})
		}
	})

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sample with the value", func(t *testing.T) {
			t.Parallel()

			valued := emit.Literal(emit.LiteralInt, derivedInt)
			assert.Equal(t, rules.Of(valued), rules.Sample{Value: valued}, "the value without a refusal")
		})
	})

	t.Run("Refused", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sample with the refusal", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Refused(rules.RefusedDepth), rules.Sample{Refusal: rules.RefusedDepth},
				"the refusal without a value")
		})
	})

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an authored string as a string literal", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			subject := rowField(nameField)
			stamp(t, facts, b.View().Kernel.Sample, subject, authoredText)
			sample, _ := b.SamplesOf(subject, builtin(strSpelling), nameField)
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralString, authoredText),
				"a backend quotes the text the author wrote between quotes")
		})

		t.Run("returns both authored halves", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			refused := rules.Refused(rules.RefusedDepth)
			b := rules.NewBound(fixedSamples{scripted(), refused, refused}, v, nil)
			subject := rowField(countField)
			stamp(t, facts, v.Kernel.Sample, subject, authoredCount)
			stamp(t, facts, v.Kernel.Alternate, subject, authoredOtherCount)
			sample, alternate := b.SamplesOf(subject, builtin(intSpelling), countField)
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{
					rules.Of(emit.Literal(emit.LiteralInt, authoredCount)),
					rules.Of(emit.Literal(emit.LiteralInt, authoredOtherCount)),
				},
				"the derivation's refusal plays no part")
		})

		t.Run("derives nothing where both halves are authored", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			d := &deriving{SourceRules: scripted()}
			subject := rowField(countField)
			stamp(t, facts, v.Kernel.Sample, subject, authoredCount)
			stamp(t, facts, v.Kernel.Alternate, subject, authoredOtherCount)
			rules.NewBound(d, v, nil).SamplesOf(subject, builtin(intSpelling), countField)
			assert.Equal(t, d.asked, 0, "the language is asked only for a missing half")
		})

		t.Run("returns the values authored on the type's declaration", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			typ := coretest.ID(svcPath, rowName, symbol.KindStruct)
			stamp(t, facts, b.View().Kernel.Sample, typ, authoredRow)
			stamp(t, facts, b.View().Kernel.Alternate, typ, authoredRowAlternate)
			sample, alternate := b.SamplesOf(symbol.Identity{}, named(svcPath, rowName, symbol.KindStruct), rowName)
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{
					rules.Of(emit.Raw(coretest.Lang, authoredRow)),
					rules.Of(emit.Raw(coretest.Lang, authoredRowAlternate)),
				},
				"a struct's values are raw text in the language that declares it")
		})

		t.Run("derives the halves no author stated", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			sample, alternate := b.SamplesOf(symbol.Identity{}, builtin(intSpelling), countField)
			wantSample, wantAlternate := rules.Pair(emit.LiteralInt, derivedInt, derivedAlternateInt)
			assert.Equal(t, [2]rules.Sample{sample, alternate}, [2]rules.Sample{wantSample, wantAlternate},
				"the scripted language's pair")
		})

		t.Run("pairs an authored sample with the derived value that differs", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			subject := rowField(countField)
			stamp(t, facts, b.View().Kernel.Sample, subject, derivedAlternateInt)
			_, alternate := b.SamplesOf(subject, builtin(intSpelling), countField)
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralInt, derivedInt),
				"the derived alternate equals the authored sample, so the derived sample pairs")
		})

		t.Run("pairs an authored alternate with the derived value that differs", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			subject := rowField(countField)
			stamp(t, facts, b.View().Kernel.Alternate, subject, derivedInt)
			sample, _ := b.SamplesOf(subject, builtin(intSpelling), countField)
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralInt, derivedAlternateInt),
				"the derived sample equals the authored alternate, so the derived alternate pairs")
		})

		t.Run("returns RefusedNoLiteral where no derived value differs from the authored half", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			one := rules.Of(emit.Literal(emit.LiteralInt, oneText))
			b := rules.NewBound(fixedSamples{scripted(), one, one}, v, nil)
			subject := rowField(countField)
			stamp(t, facts, v.Kernel.Sample, subject, oneText)
			_, alternate := b.SamplesOf(subject, builtin(intSpelling), countField)
			assert.Equal(t, alternate.Refusal, rules.RefusedNoLiteral, "no distinguishable value remains")
		})

		t.Run("returns RefusedUnresolved for a type the view does not contain", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			sample, _ := b.SamplesOf(symbol.Identity{}, named(svcPath, ghostName, symbol.KindStruct), ghostName)
			assert.Equal(t, sample.Refusal, rules.RefusedUnresolved, "the language names the reason")
		})

		t.Run("returns RefusedNoLiteral for a type the language writes no literal for", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			sample, _ := b.SamplesOf(symbol.Identity{}, named(svcPath, rowName, symbol.KindStruct), rowName)
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "the scripted language writes no composite")
		})

		t.Run("returns RefusedNoView on the zero view", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(scripted(), rules.View{}, nil)
			sample, alternate := b.SamplesOf(symbol.Identity{}, builtin(intSpelling), countField)
			wantSample, wantAlternate := rules.RefusedPair(rules.RefusedNoView)
			assert.Equal(t, [2]rules.Sample{sample, alternate}, [2]rules.Sample{wantSample, wantAlternate},
				"no view, no read")
		})
	})

	t.Run("Complete", func(t *testing.T) {
		t.Parallel()

		one := rules.Of(emit.Literal(emit.LiteralInt, oneText))
		two := rules.Of(emit.Literal(emit.LiteralInt, twoText))
		three := rules.Of(emit.Literal(emit.LiteralInt, threeText))
		oneAsText := rules.Of(emit.Literal(emit.LiteralString, oneText))
		composed := rules.Of(emit.Composite(&emit.TypeRef{Spelling: rowName}))
		refused := rules.Refused(rules.RefusedDepth)
		tests := []struct {
			name                 string
			giveSample           rules.Sample
			giveAlternate        rules.Sample
			giveDerived          rules.Sample
			giveDerivedAlternate rules.Sample
			wantSample           rules.Sample
			wantAlternate        rules.Sample
		}{
			{
				name:        "returns the derived pair where no half is stated",
				giveDerived: one, giveDerivedAlternate: two,
				wantSample: one, wantAlternate: two,
			},
			{
				name:       "keeps both stated halves",
				giveSample: one, giveAlternate: two,
				giveDerived: refused, giveDerivedAlternate: refused,
				wantSample: one, wantAlternate: two,
			},
			{
				name:        "takes the derived alternate for a missing alternate",
				giveSample:  three,
				giveDerived: one, giveDerivedAlternate: two,
				wantSample: three, wantAlternate: two,
			},
			{
				name:          "takes the derived sample for a missing sample",
				giveAlternate: three,
				giveDerived:   one, giveDerivedAlternate: two,
				wantSample: one, wantAlternate: three,
			},
			{
				name:        "takes the derived sample where the derived alternate equals the stated sample",
				giveSample:  two,
				giveDerived: one, giveDerivedAlternate: two,
				wantSample: two, wantAlternate: one,
			},
			{
				name:          "takes the derived alternate where the derived sample equals the stated alternate",
				giveAlternate: one,
				giveDerived:   one, giveDerivedAlternate: two,
				wantSample: two, wantAlternate: one,
			},
			{
				name:        "returns RefusedNoLiteral where both derived values equal the stated half",
				giveSample:  one,
				giveDerived: one, giveDerivedAlternate: one,
				wantSample: one, wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:        "returns a refused derived value with its reason",
				giveSample:  one,
				giveDerived: two, giveDerivedAlternate: refused,
				wantSample: one, wantAlternate: refused,
			},
			{
				name:        "returns RefusedNoLiteral where the other derived value is refused",
				giveSample:  one,
				giveDerived: refused, giveDerivedAlternate: one,
				wantSample: one, wantAlternate: rules.Refused(rules.RefusedNoLiteral),
			},
			{
				name:        "takes a derived value of another literal kind as distinct",
				giveSample:  oneAsText,
				giveDerived: two, giveDerivedAlternate: one,
				wantSample: oneAsText, wantAlternate: one,
			},
			{
				name:        "takes a derived composite as distinct from a stated composite",
				giveSample:  composed,
				giveDerived: two, giveDerivedAlternate: composed,
				wantSample: composed, wantAlternate: composed,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sample, alternate := rules.Complete(
					tt.giveSample, tt.giveAlternate, tt.giveDerived, tt.giveDerivedAlternate,
				)
				assert.Equal(t, [2]rules.Sample{sample, alternate}, [2]rules.Sample{tt.wantSample, tt.wantAlternate},
					"the pair Complete returns")
			})
		}
	})

	t.Run("Pair", func(t *testing.T) {
		t.Parallel()

		t.Run("returns two literals of one kind", func(t *testing.T) {
			t.Parallel()

			sample, alternate := rules.Pair(emit.LiteralString, pairSample, pairAlternate)
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{
					rules.Of(emit.Literal(emit.LiteralString, pairSample)),
					rules.Of(emit.Literal(emit.LiteralString, pairAlternate)),
				},
				"the sample and its alternate")
		})
	})

	t.Run("NumberPair", func(t *testing.T) {
		t.Parallel()

		t.Run("returns two numbers of one kind at one width", func(t *testing.T) {
			t.Parallel()

			sample, alternate := rules.NumberPair(emit.LiteralFloat, pairSample, pairAlternate, pairBits)
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{
					rules.Of(emit.Number(emit.LiteralFloat, pairSample, pairBits)),
					rules.Of(emit.Number(emit.LiteralFloat, pairAlternate, pairBits)),
				},
				"both halves state the width")
		})
	})

	t.Run("RefusedPair", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one refusal as both halves", func(t *testing.T) {
			t.Parallel()

			sample, alternate := rules.RefusedPair(rules.RefusedDepth)
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{rules.Refused(rules.RefusedDepth), rules.Refused(rules.RefusedDepth)},
				"neither half has a value")
		})
	})

	t.Run("FirstRefusal", func(t *testing.T) {
		t.Parallel()

		valued := rules.Of(emit.Literal(emit.LiteralString, pairSample))
		tests := []struct {
			name string
			give []rules.Sample
			want rules.Refusal
		}{
			{
				name: "returns the earliest refusal in argument order",
				give: []rules.Sample{valued, rules.Refused(rules.RefusedUnresolved), rules.Refused(rules.RefusedDepth)},
				want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedNoLiteral where a part without a value states no reason",
				give: []rules.Sample{valued, {}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for no part",
				want: rules.RefusedNoLiteral,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rules.FirstRefusal(tt.give...), tt.want, "the refusal a composite reports")
			})
		}
	})

	t.Run("Lift", func(t *testing.T) {
		t.Parallel()

		t.Run("wraps a derived value", func(t *testing.T) {
			t.Parallel()

			inner := emit.Literal(emit.LiteralString, pairSample)
			assert.Equal(t, rules.Lift(rules.Of(inner), emit.Address), rules.Of(emit.Address(inner)),
				"the wrapped value replaces the part")
		})

		tests := []struct {
			name string
			give rules.Sample
		}{
			{name: "returns a refused sample unchanged", give: rules.Refused(rules.RefusedUnresolved)},
			{name: "returns an empty sample unchanged", give: rules.Sample{}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rules.Lift(tt.give, emit.Address), tt.give, "nothing is wrapped")
			})
		}
	})

	t.Run("Refusal.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.Refusal
			want string
		}{
			{name: "returns none for RefusedNone", give: rules.RefusedNone, want: "none"},
			{name: "returns no-view for RefusedNoView", give: rules.RefusedNoView, want: "no-view"},
			{name: "returns no-rules for RefusedNoRules", give: rules.RefusedNoRules, want: "no-rules"},
			{name: "returns no-literal for RefusedNoLiteral", give: rules.RefusedNoLiteral, want: "no-literal"},
			{name: "returns unresolved for RefusedUnresolved", give: rules.RefusedUnresolved, want: "unresolved"},
			{name: "returns depth for RefusedDepth", give: rules.RefusedDepth, want: "depth"},
			{name: "returns the number for an undeclared refusal", give: rules.Refusal(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "a finding names the refusal by this spelling")
			})
		}
	})

	t.Run("Witnesses", func(t *testing.T) {
		t.Parallel()

		params := func() []*node.TypeParam {
			return []*node.TypeParam{
				{ID: coretest.ID(svcPath, paramName, symbol.KindTypeParam), Name: paramName},
				{ID: coretest.ID(svcPath, otherParamName, symbol.KindTypeParam), Name: otherParamName},
			}
		}

		t.Run("returns the authored witness for a parameter the author names", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			ps := params()
			row := coretest.ID(svcPath, rowName, symbol.KindStruct)
			assert.NoError(t, stampWitness(facts, b.View(), ps[0].ID, row), "the author names one witness")
			got := b.Witnesses(ps)
			assert.Length(t, got, len(ps), "the list is whole")
			assert.Equal(t, got[0], &node.TypeRef{Spelling: rowName, Target: row},
				"the named declaration, spelled by its bare name")
		})

		t.Run("derives the witness of a parameter no author names", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			ps := params()
			assert.NoError(t, stampWitness(facts, b.View(), ps[0].ID, coretest.ID(svcPath, rowName, symbol.KindStruct)),
				"the author names one witness")
			got := b.Witnesses(ps)
			assert.Length(t, got, len(ps), "the list is whole")
			assert.Equal(t, got[1].Spelling, intSpelling, "the scripted language derives int")
		})

		t.Run("returns a witness outside the graph as a reference to its package", func(t *testing.T) {
			t.Parallel()

			b, _, facts := boundOver(t, coretest.Frozen(t, hierarchy()))
			ps := params()[:1]
			outside := symbol.Identity{Lang: coretest.Lang, Package: outsidePackage, Name: outsideName}
			assert.NoError(t, stampWitness(facts, b.View(), ps[0].ID, outside), "the author names an outside type")
			assert.Equal(t, b.Witnesses(ps), []*node.TypeRef{{Spelling: outsideName, Package: outsidePackage}},
				"a backend imports the package and spells the bare name")
		})

		t.Run("returns nil for a language without the generics capability", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			b := rules.NewBound(nongeneric{scripted()}, v, nil)
			assert.Length(t, b.Witnesses(params()), 0, "nothing derives the parameters")
		})

		t.Run("returns nil for no parameters", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Length(t, b.Witnesses(nil), 0, "there is nothing to instantiate")
		})

		t.Run("returns nil for a nil parameter", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Length(t, b.Witnesses([]*node.TypeParam{nil}), 0, "a nil entry has no witness")
		})

		t.Run("returns nil on the zero view", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(scripted(), rules.View{}, nil)
			assert.Length(t, b.Witnesses(params()), 0, "no view, no read")
		})
	})

	t.Run("EmitRef", func(t *testing.T) {
		t.Parallel()

		t.Run("restates a reference's structure", func(t *testing.T) {
			t.Parallel()

			list := coretest.ID(svcPath, listName, symbol.KindStruct)
			ref := &node.TypeRef{
				Spelling: mapSpelling, Form: symbol.FormMap,
				Elems: []*node.TypeRef{
					builtin(strSpelling),
					{Spelling: listName, Target: list, Args: []*node.TypeRef{builtin(intSpelling)}},
				},
			}
			assert.Equal(t, rules.EmitRef(ref), &emit.TypeRef{
				Spelling: mapSpelling, Form: symbol.FormMap,
				Elems: []*emit.TypeRef{
					{Spelling: strSpelling},
					{Spelling: listName, Target: list, Args: []*emit.TypeRef{{Spelling: intSpelling}}},
				},
			}, "the form, the children, their targets and their arguments")
		})

		t.Run("restates a reference's package", func(t *testing.T) {
			t.Parallel()

			got := rules.EmitRef(&node.TypeRef{Spelling: outsideName, Package: outsidePackage})
			assert.Equal(t, got.Package, outsidePackage, "the import a backend records is kept")
		})

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.True(t, rules.EmitRef(nil) == nil, "nothing restates as nothing")
		})
	})
}

// The constructors and combinators of a sample return values without
// allocating, and a restated reference allocates itself, in the ordinary
// run, which runs no benchmark. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestValuesAllocs(t *testing.T) {
	for _, tt := range valueCalls(t) {
		msg := tt.name + " allocates what it returns"
		assert.MaxAllocs(t, tt.call, tt.allocs, msg)
	}
}

// BenchmarkValues measures each constructor and combinator a language's
// sample derivation returns through, the spelling of a refusal, and the
// restatement of a reference a value's type takes.
func BenchmarkValues(b *testing.B) {
	for _, tt := range valueCalls(b) {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
		})
	}
}

// valueCalls returns one call of each function and method of the
// values file over declared values, with what the call allocates. Each
// call checks what it returned, so a call that measured another path
// fails.
func valueCalls(tb assert.TB) []allocCall {
	tb.Helper()

	lit := emit.Literal(emit.LiteralInt, derivedInt)
	one, two := rules.Of(lit), rules.Of(emit.Literal(emit.LiteralInt, twoText))
	wrap := func(v emit.Value) emit.Value { return v }
	refusal := rules.RefusedDepth
	ref := &node.TypeRef{Spelling: outsideName, Package: outsidePackage}
	return []allocCall{
		{name: "Sample.OK", call: func() {
			if !one.OK() {
				tb.Fatalf("OK reported false for a sample with a value")
			}
		}},
		{name: "Of", call: func() {
			if rules.Of(lit).Value.Text != derivedInt {
				tb.Fatalf("Of returned another value")
			}
		}},
		{name: "Refused", call: func() {
			if rules.Refused(refusal).Refusal != refusal {
				tb.Fatalf("Refused returned another refusal")
			}
		}},
		{name: "Pair", call: func() {
			if sample, _ := rules.Pair(emit.LiteralString, pairSample, pairAlternate); !sample.OK() {
				tb.Fatalf("Pair returned a sample without a value")
			}
		}},
		{name: "NumberPair", call: func() {
			if _, alternate := rules.NumberPair(
				emit.LiteralFloat,
				pairSample,
				pairAlternate,
				pairBits,
			); !alternate.OK() {
				tb.Fatalf("NumberPair returned an alternate without a value")
			}
		}},
		{name: "RefusedPair", call: func() {
			if _, alternate := rules.RefusedPair(refusal); alternate.Refusal != refusal {
				tb.Fatalf("RefusedPair returned another refusal")
			}
		}},
		{name: "Lift", call: func() {
			if !rules.Lift(one, wrap).OK() {
				tb.Fatalf("Lift returned a sample without a value")
			}
		}},
		{name: "Complete", call: func() {
			if _, alternate := rules.Complete(two, rules.Sample{}, one, two); alternate.Value.Text != derivedInt {
				tb.Fatalf("Complete paired another alternate")
			}
		}},
		{name: "FirstRefusal", call: func() {
			if rules.FirstRefusal(one, rules.Refused(refusal)) != refusal {
				tb.Fatalf("FirstRefusal returned another refusal")
			}
		}},
		{name: "Refusal.String", call: func() {
			if refusal.String() != "depth" {
				tb.Fatalf("String returned another spelling")
			}
		}},
		{name: "EmitRef", allocs: emitRefAllocs, call: func() {
			if rules.EmitRef(ref).Package != outsidePackage {
				tb.Fatalf("EmitRef restated another package")
			}
		}},
	}
}

// stampWitness states one authored witness on a type parameter, the
// way the witness annotator stamps it.
func stampWitness(facts *meta.Facts, v rules.View, param, witness symbol.Identity) error {
	return meta.Stamp(facts, v.Kernel.Witness, witness, meta.Claim{Subject: param})
}
