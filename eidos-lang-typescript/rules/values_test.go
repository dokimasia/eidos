// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The classes whose fields the value cases read, and the keywords that
// the literal cases write.
const (
	shapesName    = "Shapes"
	colorName     = "Color"
	nullWord      = "null"
	undefinedWord = "undefined"
)

// The allocations of the value derivations above zero.
const (
	// stringPairAllocs counts the two hinted texts of a string's pair.
	stringPairAllocs = 2
	// listPairAllocs counts a list's pair. The pair allocates the
	// reference to the list with its list of children and its element,
	// the hinted texts of the two elements, and the list of entries of
	// each composite.
	listPairAllocs = 7
	// listZeroAllocs counts the reference to the list, with its list of
	// children and its element.
	listZeroAllocs = 3
	// numberLiteralAllocs counts the decimal text of a number.
	numberLiteralAllocs = 1
	// memberLiteralAllocs counts the list of the enum's members, the
	// reference to the enum, the assertion and the decimal text of the
	// member's value. The value has three digits, so its text is not the
	// static string that Go returns for a conversion of one byte.
	memberLiteralAllocs = 4
)

// The identities of the fixture's declarations that the value cases
// refer to.
var (
	colorID    = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: colorName, Kind: symbol.KindEnum}
	computedID = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Computed", Kind: symbol.KindEnum}
	codesID    = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Codes", Kind: symbol.KindEnum}
	loopID     = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Loop1", Kind: symbol.KindAlias}
	missingID  = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Missing", Kind: symbol.KindAlias}
	optionalID = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Optional", Kind: symbol.KindInterface}
	mixedID    = symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: "Mixed", Kind: symbol.KindStruct}
)

// TestValues checks the pair that each type form derives, its zero
// value, and the literals that it takes.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		literals := []struct {
			name      string
			host      string
			field     string
			sample    emit.Value
			alternate emit.Value
		}{
			{
				name: "derives a fixed pair of numbers", host: rowName, field: "id",
				sample:    emit.Number(emit.LiteralFloat, "1.5", 64),
				alternate: emit.Number(emit.LiteralFloat, "2.5", 64),
			},
			{
				name: "derives a pair of strings from the hint", host: rowName, field: "name",
				sample:    emit.Literal(emit.LiteralString, "name-a"),
				alternate: emit.Literal(emit.LiteralString, "name-b"),
			},
			{
				name: "derives the two truth values", host: rowName, field: "ok",
				sample:    emit.Literal(emit.LiteralBool, "true"),
				alternate: emit.Literal(emit.LiteralBool, "false"),
			},
			{
				name: "derives a pair of bigints as TypeScript text", host: rowName, field: "big",
				sample: emit.Raw(typescript.Lang, "42n"), alternate: emit.Raw(typescript.Lang, "7n"),
			},
			{
				name: "derives a pair of dates as TypeScript text", host: rowName, field: "when",
				sample:    emit.Raw(typescript.Lang, "new Date(1700000000000)"),
				alternate: emit.Raw(typescript.Lang, "new Date(1700000001000)"),
			},
			{
				name: "derives a pair of byte arrays as TypeScript text", host: rowName, field: "bytes",
				sample:    emit.Raw(typescript.Lang, "new Uint8Array([1])"),
				alternate: emit.Raw(typescript.Lang, "new Uint8Array([2])"),
			},
			{
				name: "derives the first two literal types of a union", host: rowName, field: "mode",
				sample:    emit.Literal(emit.LiteralString, "read"),
				alternate: emit.Literal(emit.LiteralString, "write"),
			},
			{
				name: "derives the pair of the first member of a union that derives one", host: shapesName,
				field:     "either",
				sample:    emit.Literal(emit.LiteralString, "either-a"),
				alternate: emit.Literal(emit.LiteralString, "either-b"),
			},
			{
				name: "derives the pair of the type of an optional", host: shapesName, field: "nullable",
				sample:    emit.Literal(emit.LiteralString, "nullable-a"),
				alternate: emit.Literal(emit.LiteralString, "nullable-b"),
			},
			{
				name: "derives the pair of the target of an alias", host: shapesName, field: "plain",
				sample:    emit.Number(emit.LiteralFloat, "1.5", 64),
				alternate: emit.Number(emit.LiteralFloat, "2.5", 64),
			},
		}
		for _, tt := range literals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				fd := f.field(t, tt.host, tt.field)
				sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
				expect.Equal(t, sample, rules.Of(tt.sample), "SamplesOf returns the expected sample")
				expect.Equal(t, alternate, rules.Of(tt.alternate), "SamplesOf returns the expected alternate")
			})
		}

		enums := []struct {
			name      string
			field     string
			enum      string
			sample    emit.Value
			alternate emit.Value
		}{
			{
				name: "derives the first two members of an enum with distinct values", field: "color", enum: colorName,
				sample:    emit.Number(emit.LiteralFloat, "0", 64),
				alternate: emit.Number(emit.LiteralFloat, "1", 64),
			},
			{
				name: "derives the first two members of a string enum", field: "mode", enum: "Mode",
				sample:    emit.Literal(emit.LiteralString, "read"),
				alternate: emit.Literal(emit.LiteralString, "write"),
			},
			{
				name: "derives the first two members whose values are known", field: "computed", enum: "Computed",
				sample:    emit.Number(emit.LiteralFloat, "1", 64),
				alternate: emit.Number(emit.LiteralFloat, "2", 64),
			},
		}
		for _, tt := range enums {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				fd := f.field(t, shapesName, tt.field)
				sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
				assert.True(t, sample.OK(), "the sample derives")
				assert.True(t, alternate.OK(), "the alternate derives")
				enum := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: tt.enum, Kind: symbol.KindEnum}
				expect.Equal(t, sample.Value.Kind, emit.ValueConversion, "a member is asserted to the enum")
				expect.Equal(t, sample.Value.Type.Target, enum, "the assertion is to the enum's type")
				expect.Equal(t, *sample.Value.Inner, tt.sample, "the sample is the first member's value")
				expect.Equal(t, *alternate.Value.Inner, tt.alternate, "the alternate is the second member's value")
			})
		}

		composites := []struct {
			name      string
			host      string
			field     string
			form      symbol.TypeForm
			sample    []emit.ValueField
			alternate []emit.ValueField
		}{
			{
				name: "derives an array of one element of a list", host: rowName, field: "tags", form: symbol.FormList,
				sample:    []emit.ValueField{emit.Element(emit.Literal(emit.LiteralString, "tags-a"))},
				alternate: []emit.ValueField{emit.Element(emit.Literal(emit.LiteralString, "tags-b"))},
			},
			{
				name: "derives an array of one element of an Array", host: rowName, field: "list",
				form:      symbol.FormNamed,
				sample:    []emit.ValueField{emit.Element(emit.Number(emit.LiteralFloat, "1.5", 64))},
				alternate: []emit.ValueField{emit.Element(emit.Number(emit.LiteralFloat, "2.5", 64))},
			},
			{
				name: "derives an object of one entry of a Record that differs in the key", host: rowName,
				field: "counts", form: symbol.FormNamed,
				sample: []emit.ValueField{emit.KeyedEntry(emit.Literal(emit.LiteralString, "counts-a"),
					emit.Number(emit.LiteralFloat, "1.5", 64))},
				alternate: []emit.ValueField{emit.KeyedEntry(emit.Literal(emit.LiteralString, "counts-b"),
					emit.Number(emit.LiteralFloat, "1.5", 64))},
			},
			{
				name: "derives an object of one entry of an index signature", host: shapesName, field: "lookup",
				form: symbol.FormMap,
				sample: []emit.ValueField{emit.KeyedEntry(emit.Literal(emit.LiteralString, "lookup-a"),
					emit.Number(emit.LiteralFloat, "1.5", 64))},
				alternate: []emit.ValueField{emit.KeyedEntry(emit.Literal(emit.LiteralString, "lookup-b"),
					emit.Number(emit.LiteralFloat, "1.5", 64))},
			},
			{
				name: "derives an array of one value per member of a tuple", host: rowName, field: "pair",
				form: symbol.FormTuple,
				sample: []emit.ValueField{
					emit.Element(emit.Literal(emit.LiteralString, "pair-a")),
					emit.Element(emit.Number(emit.LiteralFloat, "1.5", 64)),
				},
				alternate: []emit.ValueField{
					emit.Element(emit.Literal(emit.LiteralString, "pair-b")),
					emit.Element(emit.Number(emit.LiteralFloat, "2.5", 64)),
				},
			},
			{
				name: "derives the tuple of a generic alias with its arguments substituted", host: shapesName,
				field: "couple", form: symbol.FormTuple,
				sample: []emit.ValueField{
					emit.Element(emit.Literal(emit.LiteralString, "couple-a")),
					emit.Element(emit.Number(emit.LiteralFloat, "1.5", 64)),
				},
				alternate: []emit.ValueField{
					emit.Element(emit.Literal(emit.LiteralString, "couple-b")),
					emit.Element(emit.Number(emit.LiteralFloat, "2.5", 64)),
				},
			},
		}
		for _, tt := range composites {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				fd := f.field(t, tt.host, tt.field)
				sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
				assert.Equal(t, sample.Value.Kind, emit.ValueComposite, "the sample is a composite")
				assert.Equal(t, alternate.Value.Kind, emit.ValueComposite, "the alternate is a composite")
				expect.Equal(t, sample.Value.Type.Form, tt.form, "the composite has the type's form")
				expect.Equal(t, sample.Value.Fields, tt.sample, "the sample has the expected entries")
				expect.Equal(t, alternate.Value.Fields, tt.alternate, "the alternate has the expected entries")
			})
		}

		objects := []struct {
			name     string
			host     string
			field    string
			target   symbol.Identity
			property string
			sample   emit.Value
		}{
			{
				name: "derives an object that sets the first required property of a class", host: rowName,
				field: "next", target: rowID, property: "id",
				sample: emit.Number(emit.LiteralFloat, "1.5", 64),
			},
			{
				name: "derives an object of a class of another module", host: rowName, field: "target",
				target: targetID, property: "v",
				sample: emit.Number(emit.LiteralFloat, "1.5", 64),
			},
			{
				name: "derives an object that sets the first property of an interface without a required one",
				host: shapesName, field: "opt", target: optionalID, property: "label",
				sample: emit.Literal(emit.LiteralString, "label-a"),
			},
			{
				name: "derives an object that sets the first required property whose pair derives",
				host: shapesName, field: "mixed", target: mixedID, property: "id",
				sample: emit.Number(emit.LiteralFloat, "1.5", 64),
			},
		}
		for _, tt := range objects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				fd := f.field(t, tt.host, tt.field)
				sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
				assert.Equal(t, sample.Value.Kind, emit.ValueConversion, "the object is asserted to the type")
				assert.True(t, alternate.OK(), "the alternate derives")
				expect.Equal(t, sample.Value.Type.Target, tt.target, "the assertion is to the type")
				expect.Equal(t, sample.Value.Inner.Kind, emit.ValueComposite, "the asserted value is an object literal")
				expect.Equal(t, sample.Value.Inner.Fields, []emit.ValueField{emit.NamedField(tt.property, tt.sample)},
					"the object literal sets the one property")
			})
		}

		t.Run("derives an object from the values that an author stated for its property", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			v := symbol.Identity{
				Lang: typescript.Lang, Package: depPkg, Owner: targetName, Name: "v", Kind: symbol.KindField,
			}
			claim := meta.Claim{Subject: v, Authority: meta.AuthorityPlugin, Plugin: "fixture"}
			for key, value := range map[meta.Key[string]]string{f.Keys.Sample: "7", f.Keys.Alternate: "8"} {
				assert.NoError(t, meta.Stamp(f.Facts, key, value, claim), "the fixture stamp applies")
			}
			fd := f.field(t, rowName, "target")
			sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
			expect.Equal(t, sample.Value.Inner.Fields[0].Value, emit.Number(emit.LiteralFloat, "7", 64),
				"the sample is the stated value")
			expect.Equal(t, alternate.Value.Inner.Fields[0].Value, emit.Number(emit.LiteralFloat, "8", 64),
				"the alternate is the stated value")
		})

		t.Run("derives one value of a literal type and refuses the second", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fd := f.field(t, shapesName, "only")
			sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
			expect.Equal(t, sample, rules.Of(emit.Literal(emit.LiteralString, "read")), "the sample is the one value")
			expect.Equal(t, alternate, rules.Refused(rules.RefusedNoLiteral), "a literal type has no second value")
		})

		refusals := []struct {
			name  string
			host  string
			field string
			want  rules.Refusal
		}{
			{
				name: "refuses a Map, which no literal constructs", host: rowName, field: "map",
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a name that an import binds from outside the view", host: rowName, field: "outside",
				want: rules.RefusedUnresolved,
			},
			{name: "refuses a Record over an enum", host: shapesName, field: "byColor", want: rules.RefusedNoLiteral},
			{name: "refuses an inline object type", host: shapesName, field: "box", want: rules.RefusedNoLiteral},
			{
				name: "refuses a class without a public property", host: shapesName, field: "bare",
				want: rules.RefusedNoLiteral,
			},
			{name: "refuses a function type", host: shapesName, field: "fn", want: rules.RefusedNoLiteral},
			{name: "refuses an intersection", host: shapesName, field: "both", want: rules.RefusedNoLiteral},
			{
				name: "refuses a type that refers to itself below the depth", host: shapesName, field: "deep",
				want: rules.RefusedDepth,
			},
			{name: "refuses a keyword type", host: shapesName, field: "anything", want: rules.RefusedNoLiteral},
			{name: "refuses the empty tuple", host: shapesName, field: "empty", want: rules.RefusedNoLiteral},
			{
				name: "refuses a tuple with a member that refuses", host: shapesName, field: "broken",
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses an enum whose members share one value", host: shapesName, field: "same",
				want: rules.RefusedNoLiteral,
			},
			{name: "refuses an asynchronous stream", host: shapesName, field: "stream", want: rules.RefusedNoLiteral},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				fd := f.field(t, tt.host, tt.field)
				sample, alternate := tsrules.New().SamplesOf(fd.Type, fd.Name, f.view)
				expect.Equal(t, sample, rules.Refused(tt.want), "the sample refuses with the reason")
				expect.Equal(t, alternate, rules.Refused(tt.want), "the alternate refuses with the reason")
			})
		}

		built := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{
				name: "refuses a union of one value with the first member's reason",
				give: &node.TypeRef{
					Form:  symbol.FormUnion,
					Elems: []*node.TypeRef{{Spelling: `"a"`}, {Spelling: `"a"`}},
				},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a union whose members derive nothing with the first member's reason",
				give: &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{
					{Spelling: "Gone", Package: outsideLib}, {Spelling: "unknown"},
				}},
				want: rules.RefusedUnresolved,
			},
			{name: "refuses a nil reference", give: nil, want: rules.RefusedNoLiteral},
			{
				name: "refuses a map without a key",
				give: &node.TypeRef{Form: symbol.FormMap},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a map whose value refuses",
				give: &node.TypeRef{
					Form:  symbol.FormMap,
					Elems: []*node.TypeRef{{Spelling: stringName}, {Spelling: "unknown"}},
				},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a list whose element refuses",
				give: &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: "unknown"}}},
				want: rules.RefusedNoLiteral,
			},
			{
				name: "refuses a Record of one argument",
				give: &node.TypeRef{Spelling: "Record", Args: []*node.TypeRef{{Spelling: stringName}}},
				want: rules.RefusedNoLiteral,
			},
		}
		for _, tt := range built {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				sample, _ := tsrules.New().SamplesOf(tt.give, "", rules.View{})
				assert.Equal(t, sample, rules.Refused(tt.want), "the sample refuses with the reason")
			})
		}

		t.Run("derives a composite of a union's first member that derives one", func(t *testing.T) {
			t.Parallel()

			list := &node.TypeRef{
				Spelling: "string[]",
				Form:     symbol.FormList,
				Elems:    []*node.TypeRef{{Spelling: stringName}},
			}
			union := &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{list, {Spelling: stringName}}}
			sample, alternate := tsrules.New().SamplesOf(union, "", rules.View{})
			expect.Equal(t, sample.Value.Kind, emit.ValueComposite, "the sample is the list's sample")
			expect.Equal(t, alternate.Value.Kind, emit.ValueComposite, "the alternate is the list's alternate")
		})

		t.Run("derives the default hint for a string without one", func(t *testing.T) {
			t.Parallel()

			sample, _ := tsrules.New().SamplesOf(&node.TypeRef{Spelling: stringName}, "", rules.View{})
			assert.Equal(t, sample, rules.Of(emit.Literal(emit.LiteralString, "sample-a")),
				"the sample has the default hint")
		})

		t.Run("refuses an alias that the view does not contain", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			alias := &node.TypeRef{Spelling: "Missing", Target: missingID}
			sample, _ := tsrules.New().SamplesOf(alias, "", f.view)
			assert.Equal(t, sample, rules.Refused(rules.RefusedUnresolved), "the view does not declare the alias")
		})

		t.Run("refuses a reference to a function", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fn := f.function(t, loadName)
			sample, _ := tsrules.New().SamplesOf(&node.TypeRef{Spelling: loadName, Target: fn.ID}, "", f.view)
			assert.Equal(t, sample, rules.Refused(rules.RefusedNoLiteral), "a function is not a type")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		zeros := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{
				name: "returns 0 for number", give: &node.TypeRef{Spelling: numberName},
				want: emit.Number(emit.LiteralFloat, "0", 64),
			},
			{
				name: "returns the empty string for string", give: &node.TypeRef{Spelling: stringName},
				want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name: "returns false for boolean", give: &node.TypeRef{Spelling: "boolean"},
				want: emit.Literal(emit.LiteralBool, "false"),
			},
			{
				name: "returns 0n for bigint", give: &node.TypeRef{Spelling: "bigint"},
				want: emit.Raw(typescript.Lang, "0n"),
			},
		}
		for _, tt := range zeros {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := tsrules.New().ZeroValue(tt.give, rules.View{})
				assert.True(t, ok, "the type has a zero")
				assert.Equal(t, got, tt.want, "ZeroValue returns the zero of the type")
			})
		}

		optionals := []struct {
			name     string
			spelling string
			want     emit.Value
		}{
			{
				name: "returns undefined for an optional of undefined", spelling: "string|undefined",
				want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name: "returns null for an optional of null", spelling: "string|null",
				want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name: "returns undefined for an optional of null and undefined", spelling: "null|string|undefined",
				want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name: "returns undefined for an optional member of a tuple", spelling: "a?:string",
				want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name:     "returns undefined for an optional whose null is inside a type argument",
				spelling: "Map<string,null|number>|undefined", want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name:     "returns null for an optional of a function type that returns null",
				spelling: "(()=>null)|null", want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name:     "returns undefined for an optional whose null is inside a literal type",
				spelling: `"a\"|null"|undefined`, want: emit.Raw(typescript.Lang, undefinedWord),
			},
		}
		for _, tt := range optionals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				optional := &node.TypeRef{
					Spelling: tt.spelling, Form: symbol.FormOptional, Elems: []*node.TypeRef{{Spelling: stringName}},
				}
				got, ok := tsrules.New().ZeroValue(optional, rules.View{})
				assert.True(t, ok, "an optional has a zero")
				assert.Equal(t, got, tt.want, "the zero is the absent value that the union lists")
			})
		}

		t.Run("returns the empty array for a list", func(t *testing.T) {
			t.Parallel()

			list := &node.TypeRef{
				Spelling: "string[]",
				Form:     symbol.FormList,
				Elems:    []*node.TypeRef{{Spelling: stringName}},
			}
			got, ok := tsrules.New().ZeroValue(list, rules.View{})
			assert.True(t, ok, "a list has a zero")
			expect.Equal(t, got.Kind, emit.ValueComposite, "the zero is an array literal")
			expect.Empty(t, got.Fields, "the array literal has no element")
		})

		t.Run("returns the empty array for an Array", func(t *testing.T) {
			t.Parallel()

			array := &node.TypeRef{Spelling: "Array", Args: []*node.TypeRef{{Spelling: numberName}}}
			got, ok := tsrules.New().ZeroValue(array, rules.View{})
			assert.True(t, ok, "an Array has a zero")
			expect.Equal(t, got.Kind, emit.ValueComposite, "the zero is an array literal")
		})

		t.Run("returns the zero of the target of an alias", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fd := f.field(t, shapesName, "plain")
			got, ok := tsrules.New().ZeroValue(fd.Type, f.view)
			assert.True(t, ok, "Plain is an alias of number")
			assert.Equal(t, got, emit.Number(emit.LiteralFloat, "0", 64), "the zero is the zero of number")
		})

		nones := []struct {
			name string
			give func(tb assert.TB, f *fixture) *node.TypeRef
		}{
			{name: "reports false for Date", give: func(assert.TB, *fixture) *node.TypeRef {
				return &node.TypeRef{Spelling: "Date"}
			}},
			{name: "reports false for a class", give: func(tb assert.TB, f *fixture) *node.TypeRef {
				return f.field(tb, rowName, "next").Type
			}},
			{name: "reports false for an enum", give: func(tb assert.TB, f *fixture) *node.TypeRef {
				return f.field(tb, shapesName, "color").Type
			}},
			{name: "reports false for a tuple", give: func(tb assert.TB, f *fixture) *node.TypeRef {
				return f.field(tb, rowName, "pair").Type
			}},
			{name: "reports false for a cycle of aliases", give: func(assert.TB, *fixture) *node.TypeRef {
				return &node.TypeRef{Spelling: "Loop1", Target: loopID}
			}},
			{name: "reports false for a nil reference", give: func(assert.TB, *fixture) *node.TypeRef {
				return nil
			}},
		}
		for _, tt := range nones {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := tsrules.New().ZeroValue(tt.give(t, f), f.view)
				assert.False(t, ok, "the type has no zero")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		color := &node.TypeRef{Spelling: colorName, Target: colorID}
		takes := []struct {
			name string
			ref  *node.TypeRef
			text string
			want emit.Value
		}{
			{
				name: "lifts a number for number", ref: &node.TypeRef{Spelling: numberName}, text: "42",
				want: emit.Number(emit.LiteralFloat, "42", 64),
			},
			{
				name: "lifts a string for string", ref: &node.TypeRef{Spelling: stringName}, text: `"a"`,
				want: emit.Literal(emit.LiteralString, "a"),
			},
			{
				name: "lifts a truth value for boolean", ref: &node.TypeRef{Spelling: "boolean"}, text: "true",
				want: emit.Literal(emit.LiteralBool, "true"),
			},
			{
				name: "lifts a bigint for bigint", ref: &node.TypeRef{Spelling: "bigint"}, text: "1n",
				want: emit.Raw(typescript.Lang, "1n"),
			},
			{
				name: "lifts null for null", ref: &node.TypeRef{Spelling: nullWord}, text: nullWord,
				want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name: "lifts undefined for undefined", ref: &node.TypeRef{Spelling: undefinedWord}, text: undefinedWord,
				want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name: "lifts undefined for void", ref: &node.TypeRef{Spelling: "void"}, text: undefinedWord,
				want: emit.Raw(typescript.Lang, undefinedWord),
			},
			{
				name: "lifts a literal type's own value", ref: &node.TypeRef{Spelling: `"read"`}, text: `'read'`,
				want: emit.Literal(emit.LiteralString, "read"),
			},
			{
				name: "lifts every literal for unknown", ref: &node.TypeRef{Spelling: "unknown"}, text: `"x"`,
				want: emit.Literal(emit.LiteralString, "x"),
			},
			{
				name: "lifts every literal for any", ref: &node.TypeRef{Spelling: "any"}, text: "1",
				want: emit.Number(emit.LiteralFloat, "1", 64),
			},
			{
				name: "lifts the literal for the first member of a union that takes it",
				ref: &node.TypeRef{
					Form:  symbol.FormUnion,
					Elems: []*node.TypeRef{{Spelling: stringName}, {Spelling: numberName}},
				},
				text: "1",
				want: emit.Number(emit.LiteralFloat, "1", 64),
			},
		}
		for _, tt := range takes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := tsrules.New().LiteralFor(nil, tt.ref, tt.text, rules.View{})
				assert.True(t, ok, "the type takes the literal")
				assert.Equal(t, got, tt.want, "LiteralFor returns the value of the literal")
			})
		}

		optionals := []struct {
			name     string
			spelling string
			text     string
			want     emit.Value
			takes    bool
		}{
			{
				name: "lifts undefined for an optional of undefined", spelling: "string|undefined", text: undefinedWord,
				want: emit.Raw(typescript.Lang, undefinedWord), takes: true,
			},
			{
				name: "lifts undefined for an optional member", spelling: "a?:string", text: undefinedWord,
				want: emit.Raw(typescript.Lang, undefinedWord), takes: true,
			},
			{
				name: "lifts null for an optional of null", spelling: "string|null", text: nullWord,
				want: emit.Literal(emit.LiteralNil, ""), takes: true,
			},
			{
				name: "lifts the literal of an optional's type", spelling: "string|undefined", text: `"a"`,
				want: emit.Literal(emit.LiteralString, "a"), takes: true,
			},
			{
				name:     "reports false for null for an optional of undefined",
				spelling: "string|undefined", text: nullWord,
			},
			{
				name:     "reports false for undefined for an optional of null",
				spelling: "string|null", text: undefinedWord,
			},
		}
		for _, tt := range optionals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				optional := &node.TypeRef{
					Spelling: tt.spelling, Form: symbol.FormOptional, Elems: []*node.TypeRef{{Spelling: stringName}},
				}
				got, ok := tsrules.New().LiteralFor(nil, optional, tt.text, rules.View{})
				assert.Equal(t, ok, tt.takes, "LiteralFor reports whether the optional takes the literal")
				assert.Equal(t, got, tt.want, "LiteralFor returns the expected value")
			})
		}

		members := []struct {
			name string
			enum string
			text string
			want emit.Value
		}{
			{
				name: "lifts a member of an enum through the enum's name", enum: colorName, text: "Color.Green",
				want: emit.Number(emit.LiteralFloat, "1", 64),
			},
			{
				name: "lifts a member of an enum by its name", enum: colorName, text: "Blue",
				want: emit.Number(emit.LiteralFloat, "2", 64),
			},
			{
				name: "lifts a member of an enum of constant expressions", enum: "Flags", text: "Flags.AB",
				want: emit.Number(emit.LiteralFloat, "3", 64),
			},
			{
				name: "lifts a member of a string enum as its text", enum: "Mode", text: "Mode.Write",
				want: emit.Literal(emit.LiteralString, "write"),
			},
		}
		for _, tt := range members {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				enum := symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: tt.enum, Kind: symbol.KindEnum}
				ref := &node.TypeRef{Spelling: tt.enum, Target: enum}
				got, ok := tsrules.New().LiteralFor(nil, ref, tt.text, f.view)
				assert.True(t, ok, "the enum takes its member")
				expect.Equal(t, got.Kind, emit.ValueConversion, "the member is asserted to the enum")
				expect.Equal(t, *got.Inner, tt.want, "the asserted value is the member's value")
			})
		}

		t.Run("lifts a literal for the target of an alias", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, ok := tsrules.New().LiteralFor(nil, f.field(t, shapesName, "plain").Type, "3", f.view)
			assert.True(t, ok, "Plain is an alias of number")
			assert.Equal(t, got, emit.Number(emit.LiteralFloat, "3", 64), "LiteralFor returns the number")
		})

		refusals := []struct {
			name string
			ref  func(tb assert.TB, f *fixture) *node.TypeRef
			text string
		}{
			{
				name: "reports false for a string for number", text: `"a"`,
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: numberName} },
			},
			{
				name: "reports false for a number for string", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: stringName} },
			},
			{
				name: "reports false for a number for bigint", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: "bigint"} },
			},
			{
				name: "reports false for another literal of a literal type", text: `"write"`,
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: `"read"`} },
			},
			{
				name: "reports false for a member path for any", text: "Color.Red",
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: "any"} },
			},
			{
				name: "reports false for a literal that no member of a union takes", text: "true",
				ref: func(assert.TB, *fixture) *node.TypeRef {
					return &node.TypeRef{Form: symbol.FormUnion, Elems: []*node.TypeRef{{Spelling: stringName}}}
				},
			},
			{
				name: "reports false for a member through another enum's name", text: "Other.Green",
				ref: func(assert.TB, *fixture) *node.TypeRef { return color },
			},
			{
				name: "reports false for a name that no member has", text: "Purple",
				ref: func(assert.TB, *fixture) *node.TypeRef { return color },
			},
			{
				name: "reports false for a number for an enum", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef { return color },
			},
			{
				name: "reports false for a member whose value only the program's run decides", text: "Computed.X",
				ref: func(assert.TB, *fixture) *node.TypeRef {
					return &node.TypeRef{Spelling: "Computed", Target: computedID}
				},
			},
			{
				name: "reports false for a class", text: "1",
				ref: func(tb assert.TB, f *fixture) *node.TypeRef { return f.field(tb, rowName, "next").Type },
			},
			{
				name: "reports false for a list", text: "1",
				ref: func(tb assert.TB, f *fixture) *node.TypeRef { return f.field(tb, rowName, "tags").Type },
			},
			{
				name: "reports false for a name that an import binds", text: "1",
				ref: func(tb assert.TB, f *fixture) *node.TypeRef { return f.field(tb, rowName, "outside").Type },
			},
			{
				name: "reports false for Date", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: "Date"} },
			},
			{
				name: "reports false for a cycle of aliases", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef {
					return &node.TypeRef{Spelling: "Loop1", Target: loopID}
				},
			},
			{
				name: "reports false for an alias that the view does not contain", text: "1",
				ref: func(assert.TB, *fixture) *node.TypeRef {
					return &node.TypeRef{Spelling: "Missing", Target: missingID}
				},
			},
			{
				name: "reports false for text that is not a literal", text: "f()",
				ref: func(assert.TB, *fixture) *node.TypeRef { return &node.TypeRef{Spelling: numberName} },
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := tsrules.New().LiteralFor(nil, tt.ref(t, f), tt.text, f.view)
				assert.False(t, ok, "the type does not take the literal")
			})
		}
	})
}

// TestValuesAllocs checks the allocation ceiling of each value
// derivation. The ordinary test run runs no benchmark, so this test
// checks the ceilings.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valuesCalls(t))
}

// BenchmarkValues measures each value derivation under its ceiling.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valuesCalls(b))
}

// valuesCalls returns the measured calls of values.go. They derive a
// pair, a zero and a literal of a builtin, of a list and of an enum.
func valuesCalls(tb assert.TB) []allocCall {
	f := loaded(tb)
	r := tsrules.New()
	number := &node.TypeRef{Spelling: numberName}
	str := &node.TypeRef{Spelling: stringName}
	list := &node.TypeRef{Spelling: "string[]", Form: symbol.FormList, Elems: []*node.TypeRef{str}}
	codes := &node.TypeRef{Spelling: "Codes", Target: codesID}
	var (
		sample rules.Sample
		value  emit.Value
	)
	return []allocCall{
		{
			name: "SamplesOf", caseName: "a number",
			call:  func() { sample, _ = r.SamplesOf(number, "", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives the number's pair") },
		},
		{
			name: "SamplesOf", caseName: "a string", allocs: stringPairAllocs,
			call:  func() { sample, _ = r.SamplesOf(str, "s", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives the string's pair") },
		},
		{
			name: "SamplesOf", caseName: "a list", allocs: listPairAllocs,
			call:  func() { sample, _ = r.SamplesOf(list, "s", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives the list's pair") },
		},
		{
			name: "ZeroValue", caseName: "a number",
			call:  func() { value, _ = r.ZeroValue(number, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, "0", "ZeroValue returns 0") },
		},
		{
			name: "ZeroValue", caseName: "a list", allocs: listZeroAllocs,
			call: func() { value, _ = r.ZeroValue(list, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueComposite, "ZeroValue returns an array")
			},
		},
		{
			name: "LiteralFor", caseName: "a string",
			call:  func() { value, _ = r.LiteralFor(nil, str, `"plain"`, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, "plain", "LiteralFor lifts the string") },
		},
		{
			name: "LiteralFor", caseName: "a number", allocs: numberLiteralAllocs,
			call:  func() { value, _ = r.LiteralFor(nil, number, "1234", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, "1234", "LiteralFor lifts the number") },
		},
		{
			name: "LiteralFor", caseName: "a member of an enum", allocs: memberLiteralAllocs,
			call:  func() { value, _ = r.LiteralFor(nil, codes, "Codes.NotFound", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Inner.Text, "404", "LiteralFor lifts the member") },
		},
	}
}
