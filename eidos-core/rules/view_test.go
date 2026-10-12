// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The texts that the authored-value cases stamp, the module path that the
// fact cases stamp, and a language that no fixture declaration has.
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

// The sized spellings that [widths] classifies, and their widths.
const (
	boolSpelling    = "bool"
	int64Spelling   = "int64"
	float32Spelling = "float32"
	int64Bits       = 64
	float32Bits     = 32
)

// The package that [aliases] declares, and the aliases in it.
const (
	aliasPath  = "alias"
	regionName = "Region" // an alias of string
	zoneName   = "Zone"   // an alias of Region
	hollowName = "Hollow" // an alias without a target
	cycleName  = "Cycle"  // an alias of Loop
	loopName   = "Loop"   // an alias of Cycle
	codeName   = "Code"   // a type defined over string
)

// authoredAllocs is the allocation count of the lift of a value that an
// author stated on a declaration of a text type. The lift allocates the
// memo of folded shapes of the binding, the first group of the memo, and
// the shape that the memo stores out of line.
const authoredAllocs = 3

// widths classifies the sized spellings as a language with number widths
// does, and passes every other spelling to the rules that it wraps.
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

// TestView pins what a view returns, what it records and how it lifts an
// authored value. A projection reads only through a view.
func TestView(t *testing.T) {
	t.Parallel()

	t.Run("NewView", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the zero view for a nil reader", func(t *testing.T) {
			t.Parallel()

			facts := meta.NewFacts(meta.NewRegistry())
			assert.True(t, rules.NewView(nil, facts, nil, meta.KernelKeys{}).IsZero(),
				"a view without a reader reads nothing")
		})

		t.Run("returns a view over the reader", func(t *testing.T) {
			t.Parallel()

			v := viewOnly(t)
			assert.Equal(t, rules.NewView(v.Reader(), nil, nil, v.Kernel()).Reader(), v.Reader(),
				"the view reads through the reader that NewView receives", assert.ByIdentity())
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero view", func(t *testing.T) {
			t.Parallel()

			assert.True(t, rules.View{}.IsZero(), "the zero view reads nothing")
		})

		t.Run("reports false for a view with a reader", func(t *testing.T) {
			t.Parallel()

			assert.False(t, viewOnly(t).IsZero(), "a view with a reader reads its graph")
		})
	})

	t.Run("Reader", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for the zero view", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, rules.View{}.Reader(), "the zero view has no reader")
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the keys that NewView receives", func(t *testing.T) {
			t.Parallel()

			keys, err := meta.Kernel(meta.NewRegistry())
			assert.NoError(t, err, "the kernel keys register")
			assert.Equal(t, rules.NewView(nil, nil, nil, keys).Kernel(), keys, "the view has the keys of the kernel")
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of an identity", func(t *testing.T) {
			t.Parallel()

			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			decl, held := viewOnly(t).Lookup(want)
			assert.True(t, held, "the fixture declares the struct")
			assert.Equal(t, decl.(node.Declaration).Identity(), want, "the declaration has the identity")
		})

		t.Run("records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			v.Lookup(want)
			assert.Equal(t, slices.Collect(reads.Identities()), []symbol.Identity{want},
				"the read set contains the declaration")
		})

		t.Run("reports false on the zero view", func(t *testing.T) {
			t.Parallel()

			_, held := rules.View{}.Lookup(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.False(t, held, "the zero view has no declaration")
		})

		t.Run("reports false for the zero identity", func(t *testing.T) {
			t.Parallel()

			_, held := viewOnly(t).Lookup(symbol.Identity{})
			assert.False(t, held, "the zero identity has no declaration")
		})
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the package of a declaration", func(t *testing.T) {
			t.Parallel()

			pkg, held := viewOnly(t).PackageOf(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.True(t, held, "the declaration is in a package")
			assert.Equal(t, pkg.ID, coretest.PackageID(svcPath), "the package of the fixture")
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
				name:     "returns the literal of the target type of an alias",
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
				stamp(t, facts, v.Kernel().Sample, subject, tt.giveText)
				sample, _ := v.Authored(widths{scripted()}, subject, tt.giveRef)
				assert.Equal(t, sample.Value, tt.want, "the stated text lifts by the shape of the type")
			})
		}

		t.Run("returns the value of the subject before the value of the type", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			subject := rowField(nameField)
			stamp(t, facts, v.Kernel().Sample, subject, authoredText)
			stamp(t, facts, v.Kernel().Sample, coretest.ID(aliasPath, regionName, symbol.KindAlias), authoredAlternate)
			sample, _ := v.Authored(scripted(), subject, named(aliasPath, regionName, symbol.KindAlias))
			assert.Equal(t, sample.Value, emit.Literal(emit.LiteralString, authoredText),
				"the declaration that has the type is read first")
		})

		t.Run("returns the value of the type for a half without a value on the subject", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			subject := rowField(nameField)
			stamp(t, facts, v.Kernel().Sample, subject, authoredText)
			region := coretest.ID(aliasPath, regionName, symbol.KindAlias)
			stamp(t, facts, v.Kernel().Alternate, region, authoredAlternate)
			_, alternate := v.Authored(scripted(), subject, named(aliasPath, regionName, symbol.KindAlias))
			assert.Equal(t, alternate.Value, emit.Literal(emit.LiteralString, authoredAlternate),
				"each half resolves on its own")
		})

		t.Run("returns zero samples where no declaration has a stated value", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			region := named(aliasPath, regionName, symbol.KindAlias)
			sample, alternate := v.Authored(scripted(), rowField(nameField), region)
			expect.Equal(t, sample, rules.Sample{},
				"a sample that no declaration states is zero, so the caller derives it")
			expect.Equal(t, alternate, rules.Sample{}, "an alternate that no declaration states is zero")
		})

		t.Run("tags raw text with the language of the declaration that has the value", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			stamp(t, facts, v.Kernel().Sample, coretest.ID(svcPath, rowName, symbol.KindStruct), authoredRow)
			foreign := symbol.Identity{
				Lang: foreignLang, Package: svcPath, Owner: rowName, Name: nameField, Kind: symbol.KindField,
			}
			sample, _ := v.Authored(otherLang{scripted()}, foreign, named(svcPath, rowName, symbol.KindStruct))
			assert.Equal(t, sample.Value, emit.Raw(coretest.Lang, authoredRow),
				"the declaration of the type states the text, whatever the language of the rules")
		})

		t.Run("records the fact read of the declaration of the type", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			region := coretest.ID(aliasPath, regionName, symbol.KindAlias)
			v.Authored(scripted(), rowField(nameField), named(aliasPath, regionName, symbol.KindAlias))
			var read []symbol.Identity
			for id := range reads.Facts() {
				read = append(read, id)
			}
			assert.Contains(t, read, region,
				"a value that the type states later changes the result, so the read is an edge")
		})

		t.Run("does not fold the type where no declaration has a stated value", func(t *testing.T) {
			t.Parallel()

			c := &counting{SourceRules: scripted()}
			viewOnly(t).Authored(c, rowField(nameField), builtin(intSpelling))
			assert.Equal(t, c.asked, 0, "the type folds only for a stated half")
		})
	})

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value that the arbitration selects", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel().Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			got, held := rules.Fact(v, subject, v.Kernel().Module)
			assert.True(t, held, "the stamped fact reads back")
			assert.Equal(t, got, modulePath, "as stamped")
		})

		t.Run("records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel().Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			rules.Fact(v, subject, v.Kernel().Module)
			var read []symbol.Identity
			for id := range reads.Facts() {
				read = append(read, id)
			}
			assert.Equal(t, read, []symbol.Identity{subject}, "the read set contains the subject")
		})

		t.Run("returns the value without a recorder", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			untracked := rules.NewView(v.Reader(), facts, nil, v.Kernel())
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel().Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			got, held := rules.Fact(untracked, subject, v.Kernel().Module)
			assert.True(t, held, "the fact reads untracked")
			assert.Equal(t, got, modulePath, "as stamped")
		})

		t.Run("returns the value of a named handle", func(t *testing.T) {
			t.Parallel()

			v, reads, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel().Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			got, held := rules.Fact(v, subject, meta.Named[string](v.Kernel().Module.Name()))
			assert.True(t, held, "the named handle resolves in the registry of the facts")
			assert.Equal(t, got, modulePath, "the named handle reads the stamped value")
			var read []symbol.Identity
			for id := range reads.Facts() {
				read = append(read, id)
			}
			assert.Equal(t, read, []symbol.Identity{subject}, "the read of a named handle records")
		})

		t.Run("reports false for a named handle of another type", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel().Module, modulePath, meta.Claim{Subject: subject}),
				"a fact stamps")
			_, held := rules.Fact(v, subject, meta.Named[int64](v.Kernel().Module.Name()))
			assert.False(t, held, "the registry has no key of the name under int64")
		})

		t.Run("reports false for a zero key", func(t *testing.T) {
			t.Parallel()

			_, held := rules.Fact(viewOnly(t), coretest.PackageID(svcPath), meta.Key[string]{})
			assert.False(t, held, "the zero key reads no fact")
		})

		t.Run("reports false on the zero view", func(t *testing.T) {
			t.Parallel()

			v := viewOnly(t)
			_, held := rules.Fact(rules.View{}, coretest.PackageID(svcPath), v.Kernel().Module)
			assert.False(t, held, "the zero view has no facts")
		})
	})

	t.Run("Unaliased", func(t *testing.T) {
		t.Parallel()

		region := named(aliasPath, regionName, symbol.KindAlias)
		tests := []struct {
			name string
			give *node.TypeRef
			want *node.TypeRef
		}{
			{
				name: "returns the target of an alias",
				give: region,
				want: builtin(strSpelling),
			},
			{
				name: "returns the reference at the end of an alias chain",
				give: named(aliasPath, zoneName, symbol.KindAlias),
				want: builtin(strSpelling),
			},
			{
				name: "returns the reference to a defined type unchanged",
				give: named(aliasPath, codeName, symbol.KindAlias),
				want: named(aliasPath, codeName, symbol.KindAlias),
			},
			{
				name: "returns the reference to an alias without a target unchanged",
				give: named(aliasPath, hollowName, symbol.KindAlias),
				want: named(aliasPath, hollowName, symbol.KindAlias),
			},
			{
				name: "returns a reference to a struct unchanged",
				give: named(svcPath, rowName, symbol.KindStruct),
				want: named(svcPath, rowName, symbol.KindStruct),
			},
			{
				name: "returns nil for a nil reference",
				give: nil,
				want: nil,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
				got := rules.Unaliased(tt.give, v)
				assert.Equal(t, got, tt.want, "Unaliased returns the reference at the end of the alias chain")
			})
		}

		t.Run("returns one of the aliases of a cycle", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			got := rules.Unaliased(named(aliasPath, cycleName, symbol.KindAlias), v)
			assert.NotNil(t, got, "the walk ends at a reference")
			assert.Contains(t, []string{cycleName, loopName}, got.Target.Name, "the walk ends inside the cycle")
		})

		t.Run("records the read of each alias", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy(), aliases()))
			rules.Unaliased(named(aliasPath, zoneName, symbol.KindAlias), v)
			assert.Permutation(t, slices.Collect(reads.Identities()),
				[]symbol.Identity{coretest.ID(aliasPath, zoneName, symbol.KindAlias), region.Target},
				"a change of an alias of the chain changes the result, so each read is an edge")
		})
	})
}

// TestViewAllocs checks in the ordinary run that NewView allocates
// nothing, and that each read of a view allocates only what it lifts. The
// test does not run in parallel, because the count includes the
// allocations of every goroutine.
func TestViewAllocs(t *testing.T) {
	checkCalls(t, viewCalls(t))
}

// BenchmarkView measures the construction of a view and each read that a
// projection makes through it. The reads are the check for the zero view,
// the accessors, a declaration, its package, the values that an author
// stated or did not state, and a fact.
func BenchmarkView(b *testing.B) {
	benchCalls(b, viewCalls(b))
}

// viewCalls returns one call of NewView and of each read of a view over
// the walk fixture, with what the call allocates. The fixture stamps a
// sample on the name field of Row and none on its count field. Each check
// reads what the last call returned, so a call that measured another path
// fails.
func viewCalls(tb assert.TB) []allocCall {
	tb.Helper()

	v, reads, facts := viewOver(tb, coretest.Frozen(tb, hierarchy(), aliases()))
	source := scripted()
	row := coretest.ID(svcPath, rowName, symbol.KindStruct)
	zone := named(aliasPath, zoneName, symbol.KindAlias)
	pkg := coretest.PackageID(svcPath)
	stated, unstated := rowField(nameField), rowField(countField)
	keys := v.Kernel()
	stamp(tb, facts, keys.Sample, stated, authoredText)
	assert.NoError(tb, meta.Stamp(facts, keys.Module, modulePath, meta.Claim{Subject: pkg}), "a fact stamps")
	strRef, intRef := builtin(strSpelling), builtin(intSpelling)
	reader := v.Reader()
	var (
		built  rules.View
		held   bool
		sample rules.Sample
		got    string
		kernel meta.KernelKeys
		end    *node.TypeRef
	)
	return []allocCall{
		{
			name: "NewView", call: func() { built = rules.NewView(reader, facts, reads, keys) },
			check: func(tb assert.TB) { assert.False(tb, built.IsZero(), "NewView returns a view with a reader") },
		},
		{
			name: "IsZero", call: func() { held = v.IsZero() },
			check: func(tb assert.TB) { assert.False(tb, held, "IsZero reports false for a view with a reader") },
		},
		{
			name: "Reader", call: func() { held = v.Reader() != nil },
			check: func(tb assert.TB) { assert.True(tb, held, "Reader returns the reader of the view") },
		},
		{
			name: "Kernel", call: func() { kernel = v.Kernel() },
			check: func(tb assert.TB) {
				assert.Equal(tb, kernel.Module, keys.Module, "Kernel returns the keys of the kernel")
			},
		},
		{
			name: "Lookup", call: func() { _, held = v.Lookup(row) },
			check: func(tb assert.TB) { assert.True(tb, held, "Lookup finds the struct of the fixture") },
		},
		{
			name: "PackageOf", call: func() { _, held = v.PackageOf(row) },
			check: func(tb assert.TB) { assert.True(tb, held, "PackageOf finds the package of the fixture") },
		},
		{
			name: "Authored", caseName: "a type without a stated value",
			call: func() { sample, _ = v.Authored(source, unstated, intRef) },
			check: func(tb assert.TB) {
				assert.True(tb, sample.Value.IsZero(), "Authored returns no value where no declaration states one")
			},
		},
		{
			name: "Authored", caseName: "a stated value", allocs: authoredAllocs,
			call: func() { sample, _ = v.Authored(source, stated, strRef) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Text, authoredText, "Authored returns the stated value")
			},
		},
		{
			name: "Fact", call: func() { got, _ = rules.Fact(v, pkg, keys.Module) },
			check: func(tb assert.TB) { assert.Equal(tb, got, modulePath, "Fact returns the stamped value") },
		},
		{
			name: "Unaliased", call: func() { end = rules.Unaliased(zone, v) },
			check: func(tb assert.TB) {
				assert.Equal(tb, end.Spelling, strSpelling, "Unaliased returns the end of the alias chain")
			},
		},
	}
}

// aliases declares the alias chains that the lift of an authored value
// and [rules.Unaliased] follow. Region is an alias of string, and Zone is
// an alias of Region. Hollow has no target, Cycle and Loop are aliases of
// each other, and Code is a type defined over string.
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
	code := coretest.Alias(aliasPath, codeName)
	code.Target = builtin(strSpelling)
	code.Defined = true
	return coretest.Package(aliasPath, region, zone, hollow, cycle, loop, code)
}
