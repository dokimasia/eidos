// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The texts the authored-value cases state, the module path the fact
// cases stamp, and the language no fixture declaration is in.
const (
	authoredText      = "us-east"
	authoredAlternate = "eu-west"
	authoredTrue      = "true"
	authoredNumber    = "7"
	authoredFloat     = "1.5"
	authoredRow       = "Row{}"
	modulePath        = "example.test/svc"
	foreignLang       = symbol.Lang("other")
)

// The sized spellings [widths] classifies, at their widths.
const (
	boolSpelling    = "bool"
	int64Spelling   = "int64"
	float32Spelling = "float32"
	int64Bits       = 64
	float32Bits     = 32
)

// The package [aliases] declares and the aliases in it.
const (
	aliasPath  = "alias"
	regionName = "Region" // an alias of string
	zoneName   = "Zone"   // an alias of Region
	hollowName = "Hollow" // an alias with no target
	cycleName  = "Cycle"  // an alias of Loop
	loopName   = "Loop"   // an alias of Cycle
)

// authoredAllocs is the lift of a value an author stated on the
// declaration that has a text type: the binding's memo of folded shapes,
// its first group, and the shape it stores out of line.
const authoredAllocs = 3

// widths classifies the sized spellings the way a language with
// number widths does, and defers every other spelling to the rules
// it wraps.
type widths struct {
	rules.SourceRules
}

// Builtin classifies int64 and float32 at their widths.
func (w widths) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	switch ref.Spelling {
	case int64Spelling:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, int64Bits)
	case float32Spelling:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, float32Bits)
	default:
		return w.SourceRules.Builtin(ref, v)
	}
}

// The view is the one door a projection reads through, so what it
// returns, what it records and how it lifts an authored value are
// pinned here.
func TestView(t *testing.T) {
	t.Parallel()

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero view", func(t *testing.T) {
			t.Parallel()

			assert.True(t, rules.View{}.IsZero(), "the zero view reads nothing")
		})

		t.Run("reports false for a view with a reader", func(t *testing.T) {
			t.Parallel()

			assert.False(t, viewOnly(t).IsZero(), "a minted view reads its graph")
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration an identity names", func(t *testing.T) {
			t.Parallel()

			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			decl, held := viewOnly(t).Lookup(want)
			assert.True(t, held, "the fixture declares the struct")
			assert.Equal(t, decl.(node.Declaration).Identity(), want, "the declaration is the one named")
		})

		t.Run("records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			v.Lookup(want)
			assert.Equal(t, slices.Collect(reads.Identities()), []symbol.Identity{want},
				"the read set names the declaration")
		})

		t.Run("reports false on the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.View{}.Lookup(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.False(t, held, "the zero view has no declaration")
		})

		t.Run("reports false for the zero identity", func(t *testing.T) {
			t.Parallel()

			_, held := viewOnly(t).Lookup(symbol.Identity{})
			assert.False(t, held, "the zero identity names nothing")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package a declaration is in", func(t *testing.T) {
			t.Parallel()

			pkg, held := viewOnly(t).PackageOf(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.True(t, held, "the declaration is in a package")
			assert.Equal(t, pkg.ID, coretest.PackageID(svcPath), "the fixture's package")
		})

		t.Run("reports false on the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.View{}.PackageOf(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.False(t, held, "the zero view has no package")
		})
	})

	t.Run("Authored", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveRef  *node.TypeRef
			giveText string
			want     emit.Value
		}{
			{
				name:     "returns a string literal for a text type",
				giveRef:  builtin(strSpelling),
				giveText: authoredText,
				want:     emit.Literal(emit.LiteralString, authoredText),
			},
			{
				name:     "returns a truth value for a boolean type",
				giveRef:  builtin(boolSpelling),
				giveText: authoredTrue,
				want:     emit.Literal(emit.LiteralBool, authoredTrue),
			},
			{
				name:     "returns an integer at the width of an integer type",
				giveRef:  builtin(int64Spelling),
				giveText: authoredNumber,
				want:     emit.Number(emit.LiteralInt, authoredNumber, int64Bits),
			},
			{
				name:     "returns a float at the width of a float type",
				giveRef:  builtin(float32Spelling),
				giveText: authoredFloat,
				want:     emit.Number(emit.LiteralFloat, authoredFloat, float32Bits),
			},
			{
				name:     "returns raw text for a struct type",
				giveRef:  named(svcPath, rowName, symbol.KindStruct),
				giveText: authoredRow,
				want:     emit.Raw(coretest.Lang, authoredRow),
			},
			{
				name:     "returns the literal of the type an alias names",
				giveRef:  named(aliasPath, regionName, symbol.KindAlias),
				giveText: authoredText,
				want:     emit.Literal(emit.LiteralString, authoredText),
			},
			{
				name:     "returns the literal of the type at the end of an alias chain",
				giveRef:  named(aliasPath, zoneName, symbol.KindAlias),
				giveText: authoredText,
				want:     emit.Literal(emit.LiteralString, authoredText),
			},
			{
				name:     "returns raw text for an alias without a target",
				giveRef:  named(aliasPath, hollowName, symbol.KindAlias),
				giveText: authoredText,
				want:     emit.Raw(coretest.Lang, authoredText),
			},
			{
				name:     "returns raw text for an alias cycle",
				giveRef:  named(aliasPath, cycleName, symbol.KindAlias),
				giveText: authoredText,
				want:     emit.Raw(coretest.Lang, authoredText),
			},
			{
				name:     "returns raw text for a nil reference",
				giveText: authoredText,
				want:     emit.Raw(coretest.Lang, authoredText),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
				subject := rowField(nameField)
				stamp(t, facts, v.Kernel.Sample, subject, tt.giveText)
				sample, _ := v.Authored(widths{scripted()}, subject, tt.giveRef)
				assert.Equal(t, sample.Value, tt.want, "the stated text lifts by the shape of the type")
			})
		}

		t.Run("returns the subject's value before the type's", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			subject := rowField(nameField)
			stamp(t, facts, v.Kernel.Sample, subject, authoredText)
			stamp(t, facts, v.Kernel.Sample, coretest.ID(aliasPath, regionName, symbol.KindAlias), authoredAlternate)
			sample, _ := v.Authored(scripted(), subject, named(aliasPath, regionName, symbol.KindAlias))
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralString, authoredText),
				"the declaration that has the type is read first")
		})

		t.Run("returns the type's value for a half the subject does not state", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			subject := rowField(nameField)
			stamp(t, facts, v.Kernel.Sample, subject, authoredText)
			stamp(t, facts, v.Kernel.Alternate, coretest.ID(aliasPath, regionName, symbol.KindAlias), authoredAlternate)
			_, alternate := v.Authored(scripted(), subject, named(aliasPath, regionName, symbol.KindAlias))
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralString, authoredAlternate),
				"each half resolves on its own")
		})

		t.Run("returns zero samples where no declaration states a value", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			region := named(aliasPath, regionName, symbol.KindAlias)
			sample, alternate := v.Authored(scripted(), rowField(nameField), region)
			assert.Equal(t, [2]rules.Sample{sample, alternate}, [2]rules.Sample{},
				"a half nobody stated is left for the caller to derive")
		})

		t.Run("tags raw text with the language of the declaration that states it", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			stamp(t, facts, v.Kernel.Sample, coretest.ID(svcPath, rowName, symbol.KindStruct), authoredRow)
			foreign := symbol.Identity{
				Lang: foreignLang, Package: svcPath, Owner: rowName, Name: nameField, Kind: symbol.KindField,
			}
			sample, _ := v.Authored(otherLang{scripted()}, foreign, named(svcPath, rowName, symbol.KindStruct))
			assert.Equal(t, sample.Value, emit.Raw(coretest.Lang, authoredRow),
				"the type's declaration wrote the text, whichever language's rules read it")
		})

		t.Run("records the fact read of the type's declaration", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			region := coretest.ID(aliasPath, regionName, symbol.KindAlias)
			v.Authored(scripted(), rowField(nameField), named(aliasPath, regionName, symbol.KindAlias))
			var read []symbol.Identity
			for id := range reads.Facts() {
				read = append(read, id)
			}
			assert.True(t, slices.Contains(read, region),
				"a value stated on the type later changes the result, so the read is an edge")
		})

		t.Run("folds nothing where no declaration states a value", func(t *testing.T) {
			t.Parallel()

			c := &counting{SourceRules: scripted()}
			viewOnly(t).Authored(c, rowField(nameField), builtin(intSpelling))
			assert.Equal(t, c.asked, 0, "the type folds only for a stated half")
		})
	})

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value arbitration selects", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel.Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			got, held := rules.Fact(v, subject, v.Kernel.Module)
			assert.True(t, held, "the stamped fact reads back")
			assert.Equal(t, got, modulePath, "as stamped")
		})

		t.Run("records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel.Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			rules.Fact(v, subject, v.Kernel.Module)
			var read []symbol.Identity
			for id := range reads.Facts() {
				read = append(read, id)
			}
			assert.Equal(t, read, []symbol.Identity{subject}, "the read set names the subject")
		})

		t.Run("returns the value without a recorder", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			v.Reads = nil
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel.Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			got, held := rules.Fact(v, subject, v.Kernel.Module)
			assert.True(t, held, "the fact reads untracked")
			assert.Equal(t, got, modulePath, "as stamped")
		})

		t.Run("reports false for a zero key", func(t *testing.T) {
			t.Parallel()

			_, held := rules.Fact(viewOnly(t), coretest.PackageID(svcPath), meta.Key[string]{})
			assert.False(t, held, "the zero key names no fact")
		})

		t.Run("reports false on the zero view", func(t *testing.T) {
			t.Parallel()

			v := viewOnly(t)
			_, held := rules.Fact(rules.View{}, coretest.PackageID(svcPath), v.Kernel.Module)
			assert.False(t, held, "the zero view has no facts")
		})
	})
}

// Each read of a view allocates what it lifts, in the ordinary run,
// which runs no benchmark. The check runs alone, because AllocsPerRun
// counts every goroutine's allocations and refuses to run beside
// parallel tests.
func TestViewAllocs(t *testing.T) {
	for _, tt := range viewCalls(t) {
		msg := tt.name + " allocates what it lifts"
		if tt.caseName != "" {
			msg = tt.name + " for " + tt.caseName + " allocates what it lifts"
		}
		assert.MaxAllocs(t, tt.call, tt.allocs, msg)
	}
}

// BenchmarkView measures each read a projection makes through a view:
// the check for the zero view, a declaration, its package, the values an
// author stated or did not state, and a fact.
func BenchmarkView(b *testing.B) {
	benchCalls(b, viewCalls(b))
}

// viewCalls returns one call of each read of a view over the walk
// fixture, with what the call allocates. Row's name field states a
// sample, and its count field states none. Each call checks what it
// returned, so a call that measured another path fails.
func viewCalls(tb assert.TB) []allocCall {
	tb.Helper()

	v, _, facts := viewOver(tb, coretest.Frozen(tb, hierarchy()))
	source := scripted()
	row := coretest.ID(svcPath, rowName, symbol.KindStruct)
	pkg := coretest.PackageID(svcPath)
	stated, unstated := rowField(nameField), rowField(countField)
	stamp(tb, facts, v.Kernel.Sample, stated, authoredText)
	assert.NoError(tb, meta.Stamp(facts, v.Kernel.Module, modulePath, meta.Claim{Subject: pkg}), "a fact stamps")
	strRef, intRef := builtin(strSpelling), builtin(intSpelling)
	return []allocCall{
		{name: "IsZero", call: func() {
			if v.IsZero() {
				tb.Fatalf("IsZero reported true for a minted view")
			}
		}},
		{name: "Lookup", call: func() {
			if _, held := v.Lookup(row); !held {
				tb.Fatalf("Lookup missed the fixture's struct")
			}
		}},
		{name: "PackageOf", call: func() {
			if _, held := v.PackageOf(row); !held {
				tb.Fatalf("PackageOf missed the fixture's package")
			}
		}},
		{name: "Authored", caseName: "a type without a stated value", call: func() {
			if sample, _ := v.Authored(source, unstated, intRef); !sample.Value.IsZero() {
				tb.Fatalf("Authored returned a value nobody stated")
			}
		}},
		{name: "Authored", caseName: "a stated value", allocs: authoredAllocs, call: func() {
			if sample, _ := v.Authored(source, stated, strRef); sample.Value.Text != authoredText {
				tb.Fatalf("Authored returned another value")
			}
		}},
		{name: "Fact", call: func() {
			if got, _ := rules.Fact(v, pkg, v.Kernel.Module); got != modulePath {
				tb.Fatalf("Fact returned another value")
			}
		}},
	}
}

// aliases declares the alias chains an authored value's lift
// follows: Region names string, Zone names Region, Hollow names
// nothing, and Cycle and Loop name each other.
func aliases() *node.Package {
	region := coretest.Alias(aliasPath, regionName)
	region.Target = builtin(strSpelling)
	zone := coretest.Alias(aliasPath, zoneName)
	zone.Target = named(aliasPath, regionName, symbol.KindAlias)
	hollow := coretest.Alias(aliasPath, hollowName)
	hollow.Target = nil
	cycle := coretest.Alias(aliasPath, cycleName)
	cycle.Target = named(aliasPath, loopName, symbol.KindAlias)
	loop := coretest.Alias(aliasPath, loopName)
	loop.Target = named(aliasPath, cycleName, symbol.KindAlias)
	return coretest.Package(aliasPath, region, zone, hollow, cycle, loop)
}
