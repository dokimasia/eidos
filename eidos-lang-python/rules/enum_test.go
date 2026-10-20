// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brokenPkg is the module of the fixture whose classes have cycles of bases.
const brokenPkg = "shop/broken"

// builtName is the enum that the cases build by hand, outside every view.
const builtName = "Built"

// The ceilings of EnumOf. An enum of integers of the fixture allocates its
// members, its variants, the copy of its bases and the box of the copy, and
// the reference to the enum and the conversion of its value out of range. A
// StrEnum also allocates the texts of its two members. An enum outside the
// view reads no bases, and the decimal text of its value out of range is
// above the constant texts of strconv.
const (
	enumAllocs      = 6
	strEnumAllocs   = 8
	wideEnumAllocs  = 5
	wideEnumOutside = "201"
)

// TestEnum pins the values of the members of the enums of Python, and what
// the projection derives from them.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			pkg  string
			want rules.EnumInfo
		}{
			{
				name: "projects an enum of integers with the identifiers as its texts",
				give: "Status",
				want: rules.EnumInfo{
					Form:      rules.EnumIdentifier,
					Variants:  variantTexts("NEW", "NEW", "PAID", "PAID", "SHIPPED", "SHIPPED", "LEGACY", "LEGACY"),
					Duplicate: "NEW",
				},
			},
			{
				name: "projects a StrEnum with the names of its members in lower case",
				give: "Color",
				want: rules.EnumInfo{Form: rules.EnumValue, Variants: variantTexts("RED", "red", "GREEN", "green")},
			},
			{
				name: "projects an enum of strings with its strings as its texts",
				give: "Level",
				want: rules.EnumInfo{Form: rules.EnumValue, Variants: variantTexts("LOW", "low", "HIGH", "high")},
			},
			{
				name: "projects a subclass of a StrEnum of the module as a StrEnum",
				give: "Mood",
				want: rules.EnumInfo{Form: rules.EnumValue, Variants: variantTexts("HAPPY", "happy")},
			},
			{
				name: "projects an IntFlag without a value out of range",
				give: "Perm",
				want: rules.EnumInfo{
					Form:     rules.EnumIdentifier,
					Variants: variantTexts("READ", "READ", "WRITE", "WRITE", "EXEC", "EXEC"),
				},
			},
			{
				name: "projects a Flag with its zero member",
				give: "Shape",
				want: rules.EnumInfo{
					Form:     rules.EnumIdentifier,
					Variants: variantTexts("NONE", "NONE", "ROUND", "ROUND", "SQUARE", "SQUARE"),
					Zero:     "NONE",
				},
			},
			{
				name: "projects an enum that chooses the values of auto itself without a value out of range",
				give: "Ordinal",
				want: rules.EnumInfo{Form: rules.EnumIdentifier, Variants: variantTexts("FIRST", "FIRST")},
			},
			{
				name: "projects an enum without members",
				give: "Lowered",
				want: rules.EnumInfo{Form: rules.EnumIdentifier, Variants: []rules.VariantText{}},
			},
			{
				name: "projects an enum whose bases form a cycle as an enum of integers",
				give: "Left", pkg: brokenPkg,
				want: rules.EnumInfo{Form: rules.EnumIdentifier, Variants: variantTexts("A", "A")},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				pkg := tt.pkg
				if pkg == "" {
					pkg = modelsPkg
				}
				e, is := f.decl(t, symbol.Identity{Lang: python.Lang, Package: pkg, Name: tt.give, Kind: symbol.KindEnum}).(*node.Enum)
				assert.True(t, is, tt.give+" is an enum")
				got := pythonrules.Rules{}.EnumOf(e, f.view)
				got.OutOfRange = emit.Value{}
				assert.Equal(t, got, tt.want, "the projection of "+tt.give, assert.EquateEmpty())
			})
		}

		outside := []struct {
			name string
			give string
			want emit.Value
		}{
			{
				name: "derives the value after the highest member out of range",
				give: "Status",
				want: emit.Conversion(enumRef("Status"), emit.Number(emit.LiteralInt, "4", 0)),
			},
			{
				name: "derives the next power of two of a Flag out of range",
				give: "Shape",
				want: emit.Conversion(enumRef("Shape"), emit.Number(emit.LiteralInt, "4", 0)),
			},
			{
				name: "derives the empty text of a StrEnum out of range",
				give: "Color",
				want: emit.Conversion(enumRef("Color"), emit.Literal(emit.LiteralString, "")),
			},
			{name: "derives no value out of range of an IntFlag", give: "Perm"},
			{name: "derives no value out of range of an enum without known values", give: "Ordinal"},
		}
		for _, tt := range outside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: tt.give, Kind: symbol.KindEnum}
				e, _ := f.decl(t, id).(*node.Enum)
				assert.Equal(t, pythonrules.Rules{}.EnumOf(e, f.view).OutOfRange, tt.want, "the value out of range")
			})
		}

		built := []struct {
			name     string
			give     []*node.EnumVariant
			want     rules.EnumInfo
			wantOut  emit.Value
			wantSome bool
		}{
			{
				name: "lists the packages of the members of other packages once each in order",
				give: []*node.EnumVariant{
					{Name: "A", Value: "1", ID: symbol.Identity{Package: modelsPkg}},
					{Name: "B", Value: "2", ID: symbol.Identity{Package: "shop/zeta"}},
					nil,
					{Name: "C", Value: "3", ID: symbol.Identity{Package: "shop/alpha"}},
					{Name: "D", Value: "4"},
					{Name: "E", Value: "5", ID: symbol.Identity{Package: "shop/zeta"}},
				},
				want: rules.EnumInfo{
					Form:     rules.EnumIdentifier,
					Variants: variantTexts("A", "A", "B", "B", "C", "C", "D", "D", "E", "E"),
					Foreign:  []string{"shop/alpha", "shop/zeta"},
				},
				wantOut:  emit.Conversion(enumRef(builtName), emit.Number(emit.LiteralInt, "6", 0)),
				wantSome: true,
			},
			{
				name: "names the member of the empty text as the zero of an enum of strings",
				give: []*node.EnumVariant{{Name: "A", Value: `""`}, {Name: "B", Value: `"b"`}},
				want: rules.EnumInfo{Form: rules.EnumValue, Variants: variantTexts("A", "", "B", "b"), Zero: "A"},
			},
			{
				name: "names the earlier member of a repeated text as the duplicate",
				give: []*node.EnumVariant{{Name: "A", Value: `"x"`}, {Name: "B", Value: `"x"`}},
				want: rules.EnumInfo{
					Form:      rules.EnumValue,
					Variants:  variantTexts("A", "x", "B", "x"),
					Duplicate: "A",
				},
				wantOut:  emit.Conversion(enumRef(builtName), emit.Literal(emit.LiteralString, "")),
				wantSome: true,
			},
			{
				name:     "names no duplicate for a text and an integer of one spelling",
				give:     []*node.EnumVariant{{Name: "A", Value: "1"}, {Name: "B", Value: `"1"`}},
				want:     rules.EnumInfo{Form: rules.EnumIdentifier, Variants: variantTexts("A", "A", "B", "B")},
				wantOut:  emit.Conversion(enumRef(builtName), emit.Number(emit.LiteralInt, "2", 0)),
				wantSome: true,
			},
			{
				name:     "reads a member of an expression as a value that bounds nothing",
				give:     []*node.EnumVariant{{Name: "A", Value: "1 + 1"}, {Name: "B", Value: "2"}},
				want:     rules.EnumInfo{Form: rules.EnumIdentifier, Variants: variantTexts("A", "A", "B", "B")},
				wantOut:  emit.Conversion(enumRef(builtName), emit.Number(emit.LiteralInt, "3", 0)),
				wantSome: true,
			},
			{
				name: "reads an integer beyond 64 bits as a value that bounds nothing",
				give: []*node.EnumVariant{{Name: "A", Value: "99999999999999999999"}},
				want: rules.EnumInfo{Form: rules.EnumIdentifier, Variants: variantTexts("A", "A")},
			},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := pythonrules.Rules{}.EnumOf(builtEnum(tt.give), rules.View{})
				out := got.OutOfRange
				got.OutOfRange = emit.Value{}
				expect.Equal(t, got, tt.want, "the projection of the enum", assert.EquateEmpty())
				if tt.wantSome {
					expect.Equal(t, out, tt.wantOut, "the value out of range")
					return
				}
				expect.Equal(t, out, emit.Value{}, "no value is out of range")
			})
		}

		t.Run("projects a nil enum as an enum of identifiers without members", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pythonrules.Rules{}.EnumOf(nil, rules.View{}), rules.EnumInfo{Form: rules.EnumIdentifier},
				"a nil enum has no members")
		})
	})
}

// TestEnumAllocs checks the ceilings of EnumOf in the ordinary run, which
// does not run benchmarks.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls(t))
}

// BenchmarkEnum measures EnumOf under its ceilings.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls(b))
}

// enumCalls returns the projections of an enum of integers, of a StrEnum
// and of an enum whose value out of range has three digits.
func enumCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := pythonrules.Rules{}
	status, _ := f.decl(tb, statusID).(*node.Enum)
	color, _ := f.decl(tb, symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Color", Kind: symbol.KindEnum}).(*node.Enum)
	wide := builtEnum([]*node.EnumVariant{{Name: "A", Value: "100"}, {Name: "B", Value: "200"}})
	var info rules.EnumInfo
	return []allocCall{
		{
			name: "EnumOf", caseName: "an enum of integers", allocs: enumAllocs,
			call:  func() { info = r.EnumOf(status, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, info.Variants, 4, "Status has four members") },
		},
		{
			name: "EnumOf", caseName: "a StrEnum", allocs: strEnumAllocs,
			call:  func() { info = r.EnumOf(color, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, info.Form, rules.EnumValue, "Color is value-formed") },
		},
		{
			name: "EnumOf", caseName: "an enum with a value out of range of three digits", allocs: wideEnumAllocs,
			call: func() { info = r.EnumOf(wide, rules.View{}) },
			check: func(tb assert.TB) {
				want := emit.Conversion(enumRef(builtName), emit.Number(emit.LiteralInt, wideEnumOutside, 0))
				assert.Equal(tb, info.OutOfRange, want, "the value after 200")
			},
		},
	}
}

// builtEnum returns the enum Built of the package of the models with the
// variants, which no view contains.
func builtEnum(variants []*node.EnumVariant) *node.Enum {
	id := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: builtName, Kind: symbol.KindEnum}
	return &node.Enum{ID: id, Name: id.Name, Variants: variants}
}

// variantTexts returns the variants of the pairs of a name and its text, in
// order.
func variantTexts(pairs ...string) []rules.VariantText {
	out := make([]rules.VariantText, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, rules.VariantText{Name: pairs[i], Text: emit.Literal(emit.LiteralString, pairs[i+1])})
	}
	return out
}

// enumRef returns the emitted reference to the enum of the models of the
// name.
func enumRef(name string) *emit.TypeRef {
	id := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: name, Kind: symbol.KindEnum}
	return rules.EmitRef(&node.TypeRef{Spelling: name, Target: id})
}
