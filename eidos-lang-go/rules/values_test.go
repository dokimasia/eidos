// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings the value cases name: Go builtins, standard-library
// types, and spellings no package in the view declares.
const (
	intSpelling        = "int"
	int8Spelling       = "int8"
	int64Spelling      = "int64"
	uint8Spelling      = "uint8"
	uint64Spelling     = "uint64"
	float32Spelling    = "float32"
	float64Spelling    = "float64"
	boolSpelling       = "bool"
	stringSpelling     = "string"
	byteSpelling       = "byte"
	runeSpelling       = "rune"
	anySpelling        = "any"
	errorSpelling      = "error"
	comparableSpelling = "comparable"
	complexSpelling    = "complex128"
	pointerSpelling    = "unsafe.Pointer"
	durationSpelling   = "time.Duration"
	uuidSpelling       = "uuid.UUID"
	unknownSpelling    = "Unknown"
	emptyStruct        = "struct{}"
	emptyInterface     = "interface{}"
)

// seedPath is the import path of a package outside the view, which the
// value cases name in full.
const seedPath = "example.com/shop/seed"

// The fixture's declarations and fields the value cases derive.
const (
	rowType    = "Row"
	weightType = "Weight"
	plainType  = "Plain"
	colorType  = "Color"
	modeType   = "Mode"
	switchType = "Switch"
	scaleType  = "Scale"
	momentType = "Moment"
	itemsType  = "Items"
	bareType   = "Bare"
	hookedType = "Hooked"
	readerType = "Reader"
	ghostType  = "Ghost"
	loopType   = "Loop"
	idField    = "ID"
	nextField  = "Next"
	nameField  = "Name"
)

// The values the builtin table derives, pinned: the numbers, the
// truth values, the hint a string takes where the caller names none
// with the suffixes of its two halves, time.Time's seconds, and the
// widths.
const (
	derivedInt       = "42"
	derivedAltInt    = "7"
	derivedFloat     = "1.5"
	derivedAltFloat  = "2.5"
	derivedSmall     = "1"
	derivedAltSmall  = "2"
	zeroText         = "0"
	trueText         = "true"
	falseText        = "false"
	defaultHint      = "sample"
	sampleSuffix     = "-a"
	alternateSuffix  = "-b"
	unixName         = "Unix"
	sampleSeconds    = "1700000000"
	alternateSeconds = "1700000001"
	bits8            = 8
	bits32           = 32
	bits64           = 64
)

// The hints the cases pass, and the values an author states.
const (
	nameHint       = "name"
	tagHint        = "tag"
	keyHint        = "k"
	authoredWeight = "0.5"
	otherWeight    = "0.25"
	authoredID     = "5"
	otherID        = "6"
	authoredRow    = "Row{ID: 1}"
	otherRow       = "Row{ID: 2}"
)

// The allocations of a value.
const (
	// structSamplesAllocs is a struct's pair: the reference to the type,
	// and the field list of each composite.
	structSamplesAllocs = 1 + 2
	// sliceSamplesAllocs is a []string's pair: the reference to the slice
	// with its list of children and its element's reference, the element
	// list of each composite, and the hinted texts.
	sliceSamplesAllocs = 3 + 2 + 2
	// mapSamplesAllocs is a map[string]int's pair: the reference to the
	// map with its list of children and its two children's references, the
	// entry list and the key of each composite, and the hinted texts of
	// the keys.
	mapSamplesAllocs = 4 + 2 + 2 + 2
	// funcSamplesAllocs is the pair of func(string) error: the reference to
	// the function type with its list of children and its two children's
	// references, and the list of the zero values of the results, which
	// both halves share.
	funcSamplesAllocs = 4 + 1
	// chanSamplesAllocs is the pair of chan int: the reference to the
	// channel with its list of children and its child's reference.
	chanSamplesAllocs = 3
	// anySamplesAllocs is the pair of any: the hinted texts.
	anySamplesAllocs = 2
	// structZeroAllocs is a struct's zero value: the reference to the
	// type of the empty composite.
	structZeroAllocs = 1
	// literalAllocs is a decimal integer read against int: the scanner's
	// file set and file, the token and spelling lists, the constant,
	// and the decimal text.
	literalAllocs = 6
	// nameLiteralAllocs is the name of a constant of the package: the
	// bindings of the four imports of the file, three allocations, and the
	// list of candidates of the resolution.
	nameLiteralAllocs = 3 + 1
	// callLiteralAllocs is the call of a function of a package outside
	// the view: the bindings of the imports of the file.
	callLiteralAllocs = 3
)

// A generated check writes every value through these three. Each
// derivation, each refusal and each authored part is pinned here.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("DeriveSamples", func(t *testing.T) {
		t.Parallel()

		builtins := []struct {
			name          string
			give          string
			giveHint      string
			wantSample    emit.Value
			wantAlternate emit.Value
		}{
			{
				name:          "returns the sample 42 beside the alternate 7 for int",
				give:          intSpelling,
				wantSample:    emit.Literal(emit.LiteralInt, derivedInt),
				wantAlternate: emit.Literal(emit.LiteralInt, derivedAltInt),
			},
			{
				name:          "returns floats at 64 bits for float64",
				give:          float64Spelling,
				wantSample:    emit.Number(emit.LiteralFloat, derivedFloat, bits64),
				wantAlternate: emit.Number(emit.LiteralFloat, derivedAltFloat, bits64),
			},
			{
				name:          "returns floats at 32 bits for float32",
				give:          float32Spelling,
				wantSample:    emit.Number(emit.LiteralFloat, derivedFloat, bits32),
				wantAlternate: emit.Number(emit.LiteralFloat, derivedAltFloat, bits32),
			},
			{
				name:          "returns integers at 64 bits for int64",
				give:          int64Spelling,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits64),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits64),
			},
			{
				name:          "returns small integers at 8 bits for byte",
				give:          byteSpelling,
				wantSample:    emit.Number(emit.LiteralInt, derivedSmall, bits8),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltSmall, bits8),
			},
			{
				name:          "returns the sample true beside the alternate false for bool",
				give:          boolSpelling,
				wantSample:    emit.Literal(emit.LiteralBool, trueText),
				wantAlternate: emit.Literal(emit.LiteralBool, falseText),
			},
			{
				name:          "returns the hint with a suffix per half for string",
				give:          stringSpelling,
				giveHint:      nameHint,
				wantSample:    emit.Literal(emit.LiteralString, nameHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, nameHint+alternateSuffix),
			},
			{
				name:          "returns the default hint for a string without a hint",
				give:          stringSpelling,
				wantSample:    emit.Literal(emit.LiteralString, defaultHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, defaultHint+alternateSuffix),
			},
			{
				name:          "returns the hint with a suffix per half for any",
				give:          anySpelling,
				giveHint:      nameHint,
				wantSample:    emit.Literal(emit.LiteralString, nameHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, nameHint+alternateSuffix),
			},
		}
		for _, tt := range builtins {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, a := pairOf(t, loaded(t), builtin(tt.give), tt.giveHint)
				assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{tt.wantSample, tt.wantAlternate},
					"the builtin table's pair")
			})
		}

		t.Run("returns two calls of time.Unix for time.Time", func(t *testing.T) {
			t.Parallel()

			s, a := pairOf(t, loaded(t), builtin(timeSpelling), "")
			unix := symbol.Identity{Lang: golang.Lang, Package: timePath, Name: unixName, Kind: symbol.KindFunction}
			zero := emit.Literal(emit.LiteralInt, zeroText)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Call(unix, emit.Literal(emit.LiteralInt, sampleSeconds), zero),
				emit.Call(unix, emit.Literal(emit.LiteralInt, alternateSeconds), zero),
			}, "the standard library is never in the graph, so the call is curated")
		})

		t.Run("returns two conversions for time.Duration", func(t *testing.T) {
			t.Parallel()

			ref := builtin(durationSpelling)
			s, a := pairOf(t, loaded(t), ref, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, derivedSmall)),
				emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, derivedAltSmall)),
			}, "a duration converts a small integer")
		})

		t.Run("returns two conversions for time.Duration through a target outside the view", func(t *testing.T) {
			t.Parallel()

			ref := builtin(durationSpelling)
			ref.Target = id(timePath, "Duration", symbol.KindAlias)
			s, a := pairOf(t, loaded(t), ref, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, derivedSmall)),
				emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, derivedAltSmall)),
			}, "the dependency round resolves the reference, and the duration is still well-known")
		})

		t.Run("returns two calls of time.Unix for time.Time through a target outside the view", func(t *testing.T) {
			t.Parallel()

			ref := builtin(timeSpelling)
			ref.Target = id(timePath, "Time", symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), ref, "")
			unix := symbol.Identity{Lang: golang.Lang, Package: timePath, Name: unixName, Kind: symbol.KindFunction}
			zero := emit.Literal(emit.LiteralInt, zeroText)
			assert.Equal(t, s, emit.Call(unix, emit.Literal(emit.LiteralInt, sampleSeconds), zero),
				"the dependency round resolves the reference, and the time is still well-known")
		})

		t.Run("returns the hint with a suffix per half for the empty interface", func(t *testing.T) {
			t.Parallel()

			s, a := pairOf(t, loaded(t), &node.TypeRef{Spelling: emptyInterface, Form: symbol.FormInline}, nameHint)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Literal(emit.LiteralString, nameHint+sampleSuffix),
				emit.Literal(emit.LiteralString, nameHint+alternateSuffix),
			}, "a string is a value of the empty interface")
		})

		t.Run("returns two functions that return zero values for a function type", func(t *testing.T) {
			t.Parallel()

			fn := composite("func(string) (int, error)", symbol.FormFunc,
				builtin(stringSpelling), builtin(intSpelling), builtin(errorSpelling))
			fn.Split = 1
			s, a := pairOf(t, loaded(t), fn, "")
			want := emit.Func(
				rules.EmitRef(fn),
				emit.Literal(emit.LiteralInt, zeroText),
				emit.Literal(emit.LiteralNil, ""),
			)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{want, want},
				"two function literals are two function values")
		})

		t.Run("returns two functions without results for a function type without results", func(t *testing.T) {
			t.Parallel()

			fn := composite("func()", symbol.FormFunc)
			s, a := pairOf(t, loaded(t), fn, "")
			want := emit.Func(rules.EmitRef(fn))
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{want, want}, "each function has an empty body")
		})

		t.Run("returns two makes for a channel", func(t *testing.T) {
			t.Parallel()

			ch := composite("chan int", symbol.FormStream, builtin(intSpelling))
			s, a := pairOf(t, loaded(t), ch, "")
			want := emit.Make(rules.EmitRef(ch))
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{want, want}, "each make is a new channel")
		})

		t.Run("returns RefusedNoLiteral for a function type whose result has no zero value", func(t *testing.T) {
			t.Parallel()

			fn := composite("func() Ghost", symbol.FormFunc, ref(fxPath, ghostType, symbol.KindStruct))
			assert.Equal(t, refusalOf(loaded(t), fn), rules.RefusedNoLiteral, "the function cannot return a value")
		})

		t.Run("returns RefusedNoLiteral for a slice of functions", func(t *testing.T) {
			t.Parallel()

			list := composite("[]func()", symbol.FormList, composite("func()", symbol.FormFunc))
			assert.Equal(t, refusalOf(loaded(t), list), rules.RefusedNoLiteral,
				"two slices of new functions differ in the identity of the functions alone")
		})

		t.Run("returns RefusedNoLiteral for a map whose value is a channel", func(t *testing.T) {
			t.Parallel()

			m := composite("map[string]chan int", symbol.FormMap, builtin(stringSpelling),
				composite("chan int", symbol.FormStream, builtin(intSpelling)))
			assert.Equal(t, refusalOf(loaded(t), m), rules.RefusedNoLiteral, "a part derives no make")
		})

		t.Run("returns RefusedNoLiteral for a function type whose results start past its children", func(t *testing.T) {
			t.Parallel()

			fn := composite("func(int)", symbol.FormFunc, builtin(intSpelling))
			fn.Split = 2
			assert.Equal(t, refusalOf(loaded(t), fn), rules.RefusedNoLiteral, "the reference states no results")
		})

		refusals := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{name: "returns RefusedNoLiteral for error", give: builtin(errorSpelling), want: rules.RefusedNoLiteral},
			{
				name: "returns RefusedNoLiteral for comparable",
				give: builtin(comparableSpelling), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for unsafe.Pointer",
				give: builtin(pointerSpelling), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedUnresolved for a qualified spelling outside the view",
				give: builtin(uuidSpelling), want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedUnresolved for a bare spelling without a target",
				give: builtin(rowType), want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedNoLiteral for a pointer to a builtin",
				give: composite("*int", symbol.FormOptional, builtin(intSpelling)), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a map whose value has no literal",
				give: composite("map[string]error", symbol.FormMap, builtin(stringSpelling), builtin(errorSpelling)),
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for an inline struct",
				give: &node.TypeRef{
					Spelling: "struct{ A int }",
					Form:     symbol.FormInline,
					Fields: []*node.Field{
						{Name: "A", Type: builtin(intSpelling)},
					},
					Elems: []*node.TypeRef{builtin(intSpelling)},
				},
				want: rules.RefusedNoLiteral,
			},
			{name: "returns RefusedNoLiteral for a nil reference", want: rules.RefusedNoLiteral},
			{
				name: "returns RefusedNoLiteral for a struct with nothing settable",
				give: ref(fxPath, bareType, symbol.KindStruct), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for an interface",
				give: ref(fxPath, readerType, symbol.KindInterface), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedUnresolved for a type the view does not contain",
				give: ref(fxPath, ghostType, symbol.KindStruct), want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedUnresolved for a pointer to a type the view does not contain",
				give: composite("*Loop", symbol.FormOptional, ref(fxPath, loopType, symbol.KindStruct)),
				want: rules.RefusedUnresolved,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, refusalOf(loaded(t), tt.give), tt.want, "the refusal names its reason")
			})
		}

		t.Run("returns the address of a composite for a pointer to a struct", func(t *testing.T) {
			t.Parallel()

			row := ref(fxPath, rowType, symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), composite("*Row", symbol.FormOptional, row), "")
			assert.Equal(t, s, emit.Address(emit.Composite(rules.EmitRef(row),
				emit.NamedField(idField, emit.Literal(emit.LiteralInt, derivedInt)))),
				"Go takes the address of a composite literal")
		})

		t.Run("returns a one-element composite for a slice", func(t *testing.T) {
			t.Parallel()

			slice := composite("[]string", symbol.FormList, builtin(stringSpelling))
			s, a := pairOf(t, loaded(t), slice, tagHint)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(slice),
					emit.Element(emit.Literal(emit.LiteralString, tagHint+sampleSuffix))),
				emit.Composite(rules.EmitRef(slice),
					emit.Element(emit.Literal(emit.LiteralString, tagHint+alternateSuffix))),
			}, "the halves differ in the element")
		})

		t.Run("returns a one-entry composite for a map", func(t *testing.T) {
			t.Parallel()

			m := composite("map[string]int", symbol.FormMap, builtin(stringSpelling), builtin(intSpelling))
			s, a := pairOf(t, loaded(t), m, keyHint)
			value := emit.Literal(emit.LiteralInt, derivedInt)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+sampleSuffix), value)),
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+alternateSuffix), value)),
			}, "the halves differ in the key, because a map to an empty struct is a set")
		})

		t.Run("returns the one value of a struct without fields", func(t *testing.T) {
			t.Parallel()

			empty := &node.TypeRef{Spelling: emptyStruct, Form: symbol.FormInline}
			sample, alternate := gorules.New().DeriveSamples(empty, "", loaded(t).view)
			expect.Equal(t, sample, rules.Of(emit.Composite(rules.EmitRef(empty))), "the sample is struct{}{}")
			expect.Equal(t, alternate, rules.Refused(rules.RefusedNoLiteral),
				"a struct without fields has no second value")
		})

		t.Run("returns a one-entry composite of the empty struct for a map of empty structs", func(t *testing.T) {
			t.Parallel()

			empty := &node.TypeRef{Spelling: emptyStruct, Form: symbol.FormInline}
			set := composite("map[string]struct{}", symbol.FormMap, builtin(stringSpelling), empty)
			s, a := pairOf(t, loaded(t), set, keyHint)
			member := emit.Composite(rules.EmitRef(empty))
			expect.Equal(t, s, emit.Composite(rules.EmitRef(set),
				emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+sampleSuffix), member)),
				"the sample keys the empty struct by the sample of the key")
			expect.Equal(t, a, emit.Composite(rules.EmitRef(set),
				emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+alternateSuffix), member)),
				"the alternate keys the empty struct by the alternate of the key")
		})

		t.Run("refuses the alternate of a slice of empty structs", func(t *testing.T) {
			t.Parallel()

			slice := composite(
				"[]struct{}",
				symbol.FormList,
				&node.TypeRef{Spelling: emptyStruct, Form: symbol.FormInline},
			)
			_, alternate := gorules.New().DeriveSamples(slice, "", loaded(t).view)
			assert.Equal(t, alternate.Refusal, rules.RefusedNoLiteral,
				"the two halves of a slice differ in the element, and the empty struct has one value")
		})

		t.Run("returns a composite setting the first exported field for a struct", func(t *testing.T) {
			t.Parallel()

			row := ref(fxPath, rowType, symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), row, "")
			assert.Equal(t, s, emit.Composite(rules.EmitRef(row),
				emit.NamedField(idField, emit.Literal(emit.LiteralInt, derivedInt))),
				"a constructor in another package sets the exported field")
		})

		t.Run("returns a composite setting the next exported field where the first has no literal", func(t *testing.T) {
			t.Parallel()

			hooked := ref(fxPath, hookedType, symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), hooked, "")
			assert.Equal(t, s, emit.Composite(rules.EmitRef(hooked),
				emit.NamedField(nameField, emit.Literal(emit.LiteralString, nameField+sampleSuffix))),
				"a composite takes no function as a part, so the string field sets the composite")
		})

		t.Run("returns a conversion of the underlying pair for a defined type", func(t *testing.T) {
			t.Parallel()

			weight := ref(fxPath, weightType, symbol.KindAlias)
			s, _ := pairOf(t, loaded(t), weight, "")
			assert.Equal(t, s,
				emit.Conversion(rules.EmitRef(weight), emit.Number(emit.LiteralFloat, derivedFloat, bits64)),
				"Weight is a float64")
		})

		t.Run("returns the target's pair for a transparent alias", func(t *testing.T) {
			t.Parallel()

			s, _ := pairOf(t, loaded(t), ref(fxPath, plainType, symbol.KindAlias), "")
			assert.Equal(t, s, emit.Literal(emit.LiteralInt, derivedInt), "Plain is int")
		})

		enums := []struct {
			name          string
			give          *node.TypeRef
			wantSample    emit.Value
			wantAlternate emit.Value
		}{
			{
				name:          "returns conversions of the exact values of two variants for an enumeration",
				give:          ref(fxPath, colorType, symbol.KindEnum),
				wantSample:    emit.Literal(emit.LiteralInt, zeroText),
				wantAlternate: emit.Literal(emit.LiteralInt, derivedSmall),
			},
			{
				name:          "returns conversions of the exact strings of two variants for a string enumeration",
				give:          ref(fxPath, modeType, symbol.KindEnum),
				wantSample:    emit.Literal(emit.LiteralString, "read"),
				wantAlternate: emit.Literal(emit.LiteralString, "write"),
			},
			{
				name:          "returns conversions of the fractional value of a variant for a float enumeration",
				give:          ref(fxPath, scaleType, symbol.KindEnum),
				wantSample:    emit.Number(emit.LiteralFloat, authoredWeight, bits64),
				wantAlternate: emit.Number(emit.LiteralFloat, derivedSmall, bits64),
			},
			{
				name:          "returns conversions of the underlying pair for an enumeration with one value",
				give:          ref(fxPath, switchType, symbol.KindEnum),
				wantSample:    emit.Literal(emit.LiteralBool, trueText),
				wantAlternate: emit.Literal(emit.LiteralBool, falseText),
			},
		}
		for _, tt := range enums {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, a := pairOf(t, loaded(t), tt.give, "")
				expect.Equal(t, s, emit.Conversion(rules.EmitRef(tt.give), tt.wantSample), "the sample")
				expect.Equal(t, a, emit.Conversion(rules.EmitRef(tt.give), tt.wantAlternate), "the alternate")
			})
		}

		t.Run("returns composites that set a field of a generic struct at its type argument", func(t *testing.T) {
			t.Parallel()

			box := boxOf(builtin(intSpelling))
			s, a := pairOf(t, loaded(t), box, "")
			sample := emit.Composite(
				rules.EmitRef(box),
				emit.NamedField("Item", emit.Literal(emit.LiteralInt, derivedInt)),
			)
			other := emit.Composite(
				rules.EmitRef(box),
				emit.NamedField("Item", emit.Literal(emit.LiteralInt, derivedAltInt)),
			)
			expect.Equal(t, s, sample, "the sample sets Item to an int")
			expect.Equal(t, a, other, "the alternate sets another int")
		})

		t.Run("returns conversions of the pair at the type argument for a generic defined type", func(t *testing.T) {
			t.Parallel()

			items := &node.TypeRef{
				Spelling: itemsType, Target: id(fxPath, itemsType, symbol.KindAlias),
				Args: []*node.TypeRef{builtin(intSpelling)},
			}
			s, _ := pairOf(t, loaded(t), items, "")
			assert.Equal(t, s.Kind, emit.ValueConversion, "the sample converts to Items[int]")
			assert.Equal(t, s.Inner.Kind, emit.ValueComposite, "a slice of the type argument")
			assert.Length(t, s.Inner.Fields, 1, "with one element")
			assert.Equal(t, s.Inner.Fields[0].Value, emit.Literal(emit.LiteralInt, derivedInt), "an int")
		})

		t.Run("returns floats for complex128", func(t *testing.T) {
			t.Parallel()

			s, a := pairOf(t, loaded(t), builtin(complexSpelling), "")
			expect.Equal(t, s, emit.Literal(emit.LiteralFloat, derivedFloat), "Go converts the float to the complex")
			expect.Equal(t, a, emit.Literal(emit.LiteralFloat, derivedAltFloat), "the alternate float")
		})

		// An author sets the exact value of Red at directive authority,
		// as the meta directive does, over the value that the frontend
		// stamped. Green has 1 and Blue has 2.
		skipped := []struct {
			name    string
			giveRed string
		}{
			{name: "skips a variant whose exact value repeats the value of the first", giveRed: derivedSmall},
			{name: "skips a variant whose exact value does not read", giveRed: unknownSpelling},
			{name: "skips a variant whose exact fraction does not read", giveRed: "1/x"},
			{name: "skips a variant whose exact value has another type", giveRed: `"red"`},
		}
		for _, tt := range skipped {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				color := ref(fxPath, colorType, symbol.KindEnum)
				e, is := f.decl(t, color.Target).(*node.Enum)
				assert.True(t, is, "Color is an enumeration")
				key, held := f.constKey()
				assert.True(t, held, "the fixture registered the constant key")
				err := meta.Stamp(f.Facts, key, tt.giveRed, meta.Claim{
					Subject: e.Variants[0].ID, Authority: meta.AuthorityDirective, Plugin: "fixture",
				})
				assert.NoError(t, err, "the author's value applies")
				s, a := pairOf(t, f, color, "")
				one := emit.Conversion(rules.EmitRef(color), emit.Literal(emit.LiteralInt, derivedSmall))
				two := emit.Conversion(rules.EmitRef(color), emit.Literal(emit.LiteralInt, derivedAltSmall))
				expect.Equal(t, s, one, "the sample has the value 1")
				expect.Equal(t, a, two, "the alternate has the value 2")
			})
		}

		t.Run("derives a pointer to a struct whose first field is no pointer", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			sample, _ := gorules.New().DeriveSamples(f.field(t, rowType, nextField).Type, "", f.view)
			assert.True(t, sample.OK(), "Row's first field is ID, so the walk ends there")
		})

		t.Run("returns the values authored on an element's type for a slice", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			stampWeight(t, f)
			slice := composite("[]Weight", symbol.FormList, ref(fxPath, weightType, symbol.KindAlias))
			s, a := pairOf(t, f, slice, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(slice),
					emit.Element(emit.Number(emit.LiteralFloat, authoredWeight, bits64))),
				emit.Composite(rules.EmitRef(slice),
					emit.Element(emit.Number(emit.LiteralFloat, otherWeight, bits64))),
			}, "each element takes the number the author stated on Weight")
		})

		t.Run("returns the values authored on a key's type for a map", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			stampWeight(t, f)
			weight := ref(fxPath, weightType, symbol.KindAlias)
			m := composite("map[Weight]int", symbol.FormMap, weight, builtin(intSpelling))
			s, a := pairOf(t, f, m, "")
			value := emit.Literal(emit.LiteralInt, derivedInt)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Number(emit.LiteralFloat, authoredWeight, bits64), value)),
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Number(emit.LiteralFloat, otherWeight, bits64), value)),
			}, "each key takes the number the author stated on Weight")
		})

		t.Run("returns the value authored on a map value's type", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			stampWeight(t, f)
			weight := ref(fxPath, weightType, symbol.KindAlias)
			m := composite("map[string]Weight", symbol.FormMap, builtin(stringSpelling), weight)
			s, _ := pairOf(t, f, m, keyHint)
			assert.Equal(t, s, emit.Composite(rules.EmitRef(m), emit.KeyedEntry(
				emit.Literal(emit.LiteralString, keyHint+sampleSuffix),
				emit.Number(emit.LiteralFloat, authoredWeight, bits64),
			)), "the entry's value takes the number the author stated on Weight")
		})

		t.Run("returns the values authored on a field for a struct", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			field := f.field(t, rowType, idField).ID
			stamp(t, f.Facts, field, f.Keys.Sample, authoredID)
			stamp(t, f.Facts, field, f.Keys.Alternate, otherID)
			row := ref(fxPath, rowType, symbol.KindStruct)
			s, a := pairOf(t, f, row, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(row),
					emit.NamedField(idField, emit.Literal(emit.LiteralInt, authoredID))),
				emit.Composite(rules.EmitRef(row),
					emit.NamedField(idField, emit.Literal(emit.LiteralInt, otherID))),
			}, "the composite sets the values the author stated on ID")
		})

		t.Run("reads no field of an element whose type states both halves", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			row := ref(fxPath, rowType, symbol.KindStruct)
			stamp(t, f.Facts, row.Target, f.Keys.Sample, authoredRow)
			stamp(t, f.Facts, row.Target, f.Keys.Alternate, otherRow)
			pairOf(t, f, composite("[]Row", symbol.FormList, row), "")
			var read []symbol.Identity
			for subject := range f.reads.Facts() {
				read = append(read, subject)
			}
			assert.NotContains(t, read, f.field(t, rowType, idField).ID,
				"the authored pair depends on no derivation, so no field of Row is an edge")
		})

		t.Run("completes an element's unstated half with the derived value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			weight := ref(fxPath, weightType, symbol.KindAlias)
			stamp(t, f.Facts, weight.Target, f.Keys.Sample, authoredWeight)
			slice := composite("[]Weight", symbol.FormList, weight)
			s, a := pairOf(t, f, slice, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(slice),
					emit.Element(emit.Number(emit.LiteralFloat, authoredWeight, bits64))),
				emit.Composite(rules.EmitRef(slice), emit.Element(
					emit.Conversion(rules.EmitRef(weight), emit.Number(emit.LiteralFloat, derivedAltFloat, bits64)),
				)),
			}, "the authored element pairs with Weight's derived alternate")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		durationRef := builtin(durationSpelling)
		timeRef := builtin(timeSpelling)
		array := composite("[3]int", symbol.FormArray, builtin(intSpelling))
		row := ref(fxPath, rowType, symbol.KindStruct)
		color := ref(fxPath, colorType, symbol.KindEnum)
		mode := ref(fxPath, modeType, symbol.KindEnum)
		switchRef := ref(fxPath, switchType, symbol.KindEnum)
		scale := ref(fxPath, scaleType, symbol.KindEnum)
		weight := ref(fxPath, weightType, symbol.KindAlias)
		nilValue := emit.Literal(emit.LiteralNil, "")
		inlineInterface := &node.TypeRef{Spelling: "interface{ Close() error }", Form: symbol.FormInline}
		inlineStruct := &node.TypeRef{Spelling: "struct{ A int }", Form: symbol.FormInline}
		resolvedDuration, resolvedTime := builtin(durationSpelling), builtin(timeSpelling)
		resolvedDuration.Target = id(timePath, "Duration", symbol.KindAlias)
		resolvedTime.Target = id(timePath, "Time", symbol.KindStruct)
		tests := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{name: "returns 0 for int", give: builtin(intSpelling), want: emit.Literal(emit.LiteralInt, zeroText)},
			{
				name: "returns 0 at 64 bits for float64",
				give: builtin(float64Spelling), want: emit.Number(emit.LiteralFloat, zeroText, bits64),
			},
			{
				name: "returns 0 at 32 bits for float32",
				give: builtin(float32Spelling), want: emit.Number(emit.LiteralFloat, zeroText, bits32),
			},
			{name: "returns nil for unsafe.Pointer", give: builtin(pointerSpelling), want: nilValue},
			{
				name: "returns false for bool",
				give: builtin(boolSpelling), want: emit.Literal(emit.LiteralBool, falseText),
			},
			{
				name: "returns the empty string for string",
				give: builtin(stringSpelling), want: emit.Literal(emit.LiteralString, ""),
			},
			{name: "returns nil for any", give: builtin(anySpelling), want: nilValue},
			{
				name: "returns the empty composite for time.Time",
				give: timeRef, want: emit.Composite(rules.EmitRef(timeRef)),
			},
			{
				name: "returns a conversion of 0 for time.Duration",
				give: durationRef,
				want: emit.Conversion(rules.EmitRef(durationRef), emit.Literal(emit.LiteralInt, zeroText)),
			},
			{
				name: "returns nil for a pointer",
				give: composite("*int", symbol.FormOptional, builtin(intSpelling)), want: nilValue,
			},
			{
				name: "returns the empty composite for an array",
				give: array, want: emit.Composite(rules.EmitRef(array)),
			},
			{name: "returns the empty composite for a struct", give: row, want: emit.Composite(rules.EmitRef(row))},
			{
				name: "returns nil for an interface",
				give: ref(fxPath, readerType, symbol.KindInterface), want: nilValue,
			},
			{
				name: "returns a conversion of 0 for an enumeration",
				give: color,
				want: emit.Conversion(rules.EmitRef(color), emit.Literal(emit.LiteralInt, zeroText)),
			},
			{
				name: "returns a conversion of the empty string for a string enumeration",
				give: mode,
				want: emit.Conversion(rules.EmitRef(mode), emit.Literal(emit.LiteralString, "")),
			},
			{
				name: "returns a conversion of false for an enumeration over a bool",
				give: switchRef,
				want: emit.Conversion(rules.EmitRef(switchRef), emit.Literal(emit.LiteralBool, falseText)),
			},
			{
				name: "returns a conversion of 0 at 64 bits for an enumeration over a float",
				give: scale,
				want: emit.Conversion(rules.EmitRef(scale), emit.Number(emit.LiteralFloat, zeroText, bits64)),
			},
			{
				name: "returns a conversion of the underlying zero for a defined type",
				give: weight,
				want: emit.Conversion(rules.EmitRef(weight), emit.Number(emit.LiteralFloat, zeroText, bits64)),
			},
			{
				name: "returns the target's zero for a transparent alias",
				give: ref(fxPath, plainType, symbol.KindAlias), want: emit.Literal(emit.LiteralInt, zeroText),
			},
			{
				name: "returns 0 for complex128",
				give: builtin(complexSpelling), want: emit.Literal(emit.LiteralInt, zeroText),
			},
			{
				name: "returns nil for an inline interface",
				give: inlineInterface, want: nilValue,
			},
			{
				name: "returns the empty composite for an inline struct",
				give: inlineStruct, want: emit.Composite(rules.EmitRef(inlineStruct)),
			},
			{
				name: "returns a conversion of 0 for time.Duration through a target outside the view",
				give: resolvedDuration,
				want: emit.Conversion(rules.EmitRef(resolvedDuration), emit.Literal(emit.LiteralInt, zeroText)),
			},
			{
				name: "returns the empty composite for time.Time through a target outside the view",
				give: resolvedTime, want: emit.Composite(rules.EmitRef(resolvedTime)),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := gorules.New().ZeroValue(tt.give, loaded(t).view)
				assert.True(t, ok, "the zero derives")
				assert.Equal(t, got, tt.want, "Go's zero value")
			})
		}

		unplaced := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "reports false for a spelling the rules cannot place", give: builtin(unknownSpelling)},
			{
				name: "reports false for a type the view does not contain",
				give: ref(fxPath, ghostType, symbol.KindStruct),
			},
			{name: "reports false for a nil reference"},
			{
				name: "reports false for an enumeration whose underlying type the rules cannot place",
				give: ref(fxPath, momentType, symbol.KindEnum),
			},
			{
				name: "reports false for an inline type of another kind",
				give: &node.TypeRef{Spelling: "func()", Form: symbol.FormInline},
			},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := gorules.New().ZeroValue(tt.give, loaded(t).view)
				assert.False(t, ok, "no zero to compare a default against")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		// The cases read a text in the scope of Row in the fixture file.
		rowID := id(fxPath, rowType, symbol.KindStruct)
		untyped := []struct {
			name string
			give string
			want emit.Value
		}{
			{name: "reads nil", give: "nil", want: emit.Literal(emit.LiteralNil, "")},
			{
				name: "reads true behind spaces",
				give: " " + trueText + " ", want: emit.Literal(emit.LiteralBool, trueText),
			},
			{name: "reads false", give: falseText, want: emit.Literal(emit.LiteralBool, falseText)},
			{name: "reads a decimal integer", give: "42", want: emit.Literal(emit.LiteralInt, "42")},
			{name: "reads a hexadecimal integer", give: "0x1F", want: emit.Literal(emit.LiteralInt, "31")},
			{
				name: "reads an octal integer with a leading zero",
				give: "017", want: emit.Literal(emit.LiteralInt, "15"),
			},
			{name: "reads an octal integer with a prefix", give: "0o17", want: emit.Literal(emit.LiteralInt, "15")},
			{name: "reads a binary integer", give: "0b101", want: emit.Literal(emit.LiteralInt, "5")},
			{name: "reads an integer with separators", give: "1_000", want: emit.Literal(emit.LiteralInt, "1000")},
			{name: "reads a negative integer", give: "-5", want: emit.Literal(emit.LiteralInt, "-5")},
			{name: "reads a positive sign", give: "+3", want: emit.Literal(emit.LiteralInt, "3")},
			{
				name: "reads an integer past int64",
				give: "18446744073709551615", want: emit.Literal(emit.LiteralInt, "18446744073709551615"),
			},
			{name: "reads a decimal float", give: "2.5", want: emit.Literal(emit.LiteralFloat, "2.5")},
			{name: "reads a float behind a spaced sign", give: "- 2.5", want: emit.Literal(emit.LiteralFloat, "-2.5")},
			{name: "reads a hexadecimal float", give: "0x1p-2", want: emit.Literal(emit.LiteralFloat, "0.25")},
			{name: "reads a float with separators", give: "1_000.5", want: emit.Literal(emit.LiteralFloat, "1000.5")},
			{
				name: "writes a large float in exponent notation",
				give: "1e21", want: emit.Literal(emit.LiteralFloat, "1e+21"),
			},
			{
				name: "writes a small float in exponent notation",
				give: "1e-7", want: emit.Literal(emit.LiteralFloat, "1e-7"),
			},
			{
				name: "writes a float of 1e-6 in positional notation",
				give: "0.000001", want: emit.Literal(emit.LiteralFloat, "0.000001"),
			},
			{name: "reads a quoted string", give: `"hi"`, want: emit.Literal(emit.LiteralString, "hi")},
			{name: "reads a raw string", give: "`raw`", want: emit.Literal(emit.LiteralString, "raw")},
			{
				name: "reads an escape in a quoted string",
				give: `"tab\t"`, want: emit.Literal(emit.LiteralString, "tab\t"),
			},
			{name: "reads a rune as its integer value", give: "'a'", want: emit.Literal(emit.LiteralInt, "97")},
		}
		for _, tt := range untyped {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := gorules.New().LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
				assert.True(t, ok, "the text is one Go literal")
				assert.Equal(t, got, tt.want, "written as canonical decimal text")
			})
		}

		unreadable := []struct {
			name string
			give string
		}{
			{name: "reports false for empty text", give: ""},
			{name: "reports false for a composite literal", give: "Row{}"},
			{name: "reports false for Inf", give: "Inf"},
			{name: "reports false for NaN", give: "NaN"},
			{name: "reports false for an imaginary number", give: "1i"},
			{name: "reports false for a string expression", give: `"a" + "b"`},
			{name: "reports false for a literal before a comment", give: "1 // note"},
			{name: "reports false for a negated rune", give: "-'a'"},
			{name: "reports false for a doubled sign", give: "--1"},
			{name: "reports false for an identifier", give: "x"},
			{name: "reports false for a float past float64", give: "1e400"},
			{name: "reports false for an unterminated string", give: `"open`},
		}
		for _, tt := range unreadable {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := gorules.New().LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
				assert.False(t, ok, "the text is no single literal")
			})
		}

		typed := []struct {
			name     string
			give     *node.TypeRef
			giveText string
			want     emit.Value
		}{
			{
				name: "returns 127 at 8 bits for int8",
				give: builtin(int8Spelling), giveText: "127", want: emit.Number(emit.LiteralInt, "127", bits8),
			},
			{
				name: "returns -128 at 8 bits for int8",
				give: builtin(int8Spelling), giveText: "-128", want: emit.Number(emit.LiteralInt, "-128", bits8),
			},
			{
				name: "returns 255 at 8 bits for uint8",
				give: builtin(uint8Spelling), giveText: "255", want: emit.Number(emit.LiteralInt, "255", bits8),
			},
			{
				name:     "returns the largest uint64 at 64 bits",
				give:     builtin(uint64Spelling),
				giveText: "18446744073709551615",
				want:     emit.Number(emit.LiteralInt, "18446744073709551615", bits64),
			},
			{
				name: "returns an integral float as an int",
				give: builtin(intSpelling), giveText: "2.0", want: emit.Number(emit.LiteralInt, "2", 0),
			},
			{
				name: "returns a rune at 32 bits",
				give: builtin(runeSpelling), giveText: "'a'", want: emit.Number(emit.LiteralInt, "97", bits32),
			},
			{
				name: "returns 0.1 at 32 bits for float32",
				give: builtin(float32Spelling), giveText: "0.1", want: emit.Number(emit.LiteralFloat, "0.1", bits32),
			},
			{
				name: "returns an integer as a float32",
				give: builtin(float32Spelling), giveText: "3", want: emit.Number(emit.LiteralFloat, "3", bits32),
			},
			{
				name:     "returns a hexadecimal float at 64 bits for float64",
				give:     builtin(float64Spelling),
				giveText: "0x1p-2",
				want:     emit.Number(emit.LiteralFloat, "0.25", bits64),
			},
			{
				name: "returns a truth value for bool",
				give: builtin(boolSpelling), giveText: trueText, want: emit.Literal(emit.LiteralBool, trueText),
			},
			{
				name: "returns a string for string",
				give: builtin(stringSpelling), giveText: `"x"`, want: emit.Literal(emit.LiteralString, "x"),
			},
			{
				name: "returns nil for any",
				give: builtin(anySpelling), giveText: "nil", want: emit.Literal(emit.LiteralNil, ""),
			},
			{
				name:     "returns nil for a pointer",
				give:     composite("*int", symbol.FormOptional, builtin(intSpelling)),
				giveText: "nil",
				want:     emit.Literal(emit.LiteralNil, ""),
			},
			{
				name:     "returns the underlying float for a defined type",
				give:     ref(fxPath, weightType, symbol.KindAlias),
				giveText: "1.5",
				want:     emit.Number(emit.LiteralFloat, "1.5", bits64),
			},
			{
				name:     "returns the target's int for a transparent alias",
				give:     ref(fxPath, plainType, symbol.KindAlias),
				giveText: "7",
				want:     emit.Number(emit.LiteralInt, "7", 0),
			},
			{
				name:     "returns the underlying int for an enumeration",
				give:     ref(fxPath, colorType, symbol.KindEnum),
				giveText: "7",
				want:     emit.Number(emit.LiteralInt, "7", 0),
			},
		}
		for _, tt := range typed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, ok := gorules.New().LiteralFor(f.scope(rowID), tt.give, tt.giveText, f.view)
				assert.True(t, ok, "the text is a value of the type")
				assert.Equal(t, got, tt.want, "typed by the builtin the reference names")
			})
		}

		outside := []struct {
			name     string
			give     *node.TypeRef
			giveText string
		}{
			{name: "reports false for 128 as int8", give: builtin(int8Spelling), giveText: "128"},
			{name: "reports false for -129 as int8", give: builtin(int8Spelling), giveText: "-129"},
			{name: "reports false for -1 as uint8", give: builtin(uint8Spelling), giveText: "-1"},
			{name: "reports false for 2^64 as uint64", give: builtin(uint64Spelling), giveText: "18446744073709551616"},
			{name: "reports false for a fraction as int", give: builtin(intSpelling), giveText: "2.5"},
			{name: "reports false for a string as int", give: builtin(intSpelling), giveText: `"2"`},
			{name: "reports false for nil as int", give: builtin(intSpelling), giveText: "nil"},
			{name: "reports false for 1e39 as float32", give: builtin(float32Spelling), giveText: "1e39"},
			{name: "reports false for a truth value as float64", give: builtin(float64Spelling), giveText: trueText},
			{name: "reports false for a number as bool", give: builtin(boolSpelling), giveText: "1"},
			{name: "reports false for a rune as string", give: builtin(stringSpelling), giveText: "'a'"},
			{
				name: "reports false for a string as a defined float",
				give: ref(fxPath, weightType, symbol.KindAlias), giveText: `"heavy"`,
			},
			{
				name: "reports false for a string as an enumeration over an int",
				give: ref(fxPath, colorType, symbol.KindEnum), giveText: `"red"`,
			},
		}
		for _, tt := range outside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := gorules.New().LiteralFor(f.scope(rowID), tt.give, tt.giveText, f.view)
				assert.False(t, ok, "the value is outside the builtin")
			})
		}

		second := symbol.Identity{Lang: golang.Lang, Package: timePath, Name: "Second", Kind: symbol.KindConstant}
		now := symbol.Identity{Lang: golang.Lang, Package: timePath, Name: "Now", Kind: symbol.KindFunction}
		named := []struct {
			name string
			give string
			want emit.Value
		}{
			{
				name: "returns the name of a constant of the package of the subject",
				give: "Limit", want: emit.Name(id(fxPath, "Limit", symbol.KindConstant)),
			},
			{
				name: "returns the name of a variable of the package of the subject",
				give: "Registry", want: emit.Name(id(fxPath, "Registry", symbol.KindVariable)),
			},
			{
				name: "returns the name of a constant of an enumeration",
				give: "Green",
				want: emit.Name(symbol.Identity{
					Lang: golang.Lang, Package: fxPath, Owner: colorType, Name: "Green", Kind: symbol.KindEnumVariant,
				}),
			},
			{
				name: "returns the name of a constant of an imported package in the view",
				give: "dep.Ratio", want: emit.Name(id(depPath, "Ratio", symbol.KindConstant)),
			},
			{
				name: "returns a stand-in constant for a name of an imported package outside the view",
				give: "time.Second", want: emit.Name(second),
			},
			{
				name: "returns a call of a function of the package of the subject",
				give: "All()", want: emit.Call(id(fxPath, "All", symbol.KindFunction)),
			},
			{
				name: "returns a stand-in function for a call of an imported package outside the view",
				give: "time.Now()", want: emit.Call(now),
			},
			{
				name: "returns the name of a constant that a full import path names",
				give: depPath + ".Ratio", want: emit.Name(id(depPath, "Ratio", symbol.KindConstant)),
			},
			{
				name: "returns a stand-in constant for a name under a full import path outside the view",
				give: seedPath + ".Region", want: emit.Name(id(seedPath, "Region", symbol.KindConstant)),
			},
			{
				name: "returns a stand-in function for a call under a full import path outside the view",
				give: seedPath + ".Defaults()", want: emit.Call(id(seedPath, "Defaults", symbol.KindFunction)),
			},
			{
				name: "reads a name behind spaces",
				give: " Limit ", want: emit.Name(id(fxPath, "Limit", symbol.KindConstant)),
			},
		}
		for _, tt := range named {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, ok := gorules.New().LiteralFor(f.scope(rowID), nil, tt.give, f.view)
				assert.True(t, ok, "the text refers to a declaration in the scope")
				assert.Equal(t, got, tt.want, "the value refers to the declaration by its identity")
			})
		}

		unnamed := []struct {
			name string
			give string
		}{
			{name: "reports false for a name that the package of the subject does not declare", give: "Missing"},
			{
				name: "reports false for a name that an imported package in the view does not declare",
				give: "dep.Missing",
			},
			{name: "reports false for a qualifier that the file does not import", give: "unbound.Second"},
			{name: "reports false for the name of a function", give: "Load"},
			{name: "reports false for the name of a type", give: "Row"},
			{name: "reports false for a call of a constant", give: "Limit()"},
			{name: "reports false for a call of a method", give: "Rename()"},
			{name: "reports false for a call with an argument", give: "Find(1)"},
			{name: "reports false for a call of the result of a call", give: "All()()"},
			{name: "reports false for a selector of a variable", give: "Registry.Len"},
			{
				name: "reports false for a name that the package of a full import path does not declare",
				give: depPath + ".Missing",
			},
			{name: "reports false for a field of a type under a full import path", give: depPath + ".Target.V"},
		}
		for _, tt := range unnamed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := gorules.New().LiteralFor(f.scope(rowID), nil, tt.give, f.view)
				assert.False(t, ok, "the text refers to no constant, variable or function in the scope")
			})
		}

		t.Run("returns the name that an import of the package binds in a scope without a file", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, ok := gorules.New().LiteralFor(rules.Scope{Subject: rowID}, nil, "time.Second", f.view)
			assert.True(t, ok, "a file of the package binds the qualifier")
			second := symbol.Identity{Lang: golang.Lang, Package: "time", Name: "Second", Kind: symbol.KindConstant}
			assert.Equal(t, got, emit.Name(second), "the value refers to the stand-in of the constant")
		})

		t.Run("returns the name under a full import path in a file that does not import its package",
			func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: rowID, File: &node.File{}}
				got, ok := gorules.New().LiteralFor(scope, nil, depPath+".Ratio", f.view)
				assert.True(t, ok, "a full import path needs no import of the file")
				want := emit.Name(id(depPath, "Ratio", symbol.KindConstant))
				assert.Equal(t, got, want, "the value refers to the constant")
			})

		t.Run("returns the name under a full import path in a scope without a file", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, ok := gorules.New().LiteralFor(rules.Scope{Subject: rowID}, nil, depPath+".Ratio", f.view)
			assert.True(t, ok, "a full import path needs no file")
			want := emit.Name(id(depPath, "Ratio", symbol.KindConstant))
			assert.Equal(t, got, want, "the value refers to the constant")
		})

		t.Run("reads the call that an author states on a field of a type without a literal", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			when := f.field(t, rowType, "When")
			stamp(t, f.Facts, when.ID, f.Keys.Sample, "time.Now()")
			sample, _ := f.view.Authored(gorules.New(), when.ID, when.Type)
			assert.Equal(t, sample.Value, emit.Call(now), "View.Authored lifts the text through LiteralFor")
		})

		authored := []struct {
			name      string
			giveField string
			giveText  string
			want      rules.Sample
		}{
			{
				name:      "reads the name that an author states on a field of an integer type",
				giveField: idField,
				giveText:  "Limit",
				want:      rules.Of(emit.Name(id(fxPath, "Limit", symbol.KindConstant))),
			},
			{
				name:      "reads the name that an author states on a field of a float type",
				giveField: "Ratio",
				giveText:  "dep.Ratio",
				want:      rules.Of(emit.Name(id(depPath, "Ratio", symbol.KindConstant))),
			},
			{
				name:      "returns RefusedAuthored for a name that the scope does not declare on a field of an integer type",
				giveField: idField,
				giveText:  "Missing",
				want:      rules.Refused(rules.RefusedAuthored),
			},
		}
		for _, tt := range authored {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				field := f.field(t, rowType, tt.giveField)
				stamp(t, f.Facts, field.ID, f.Keys.Sample, tt.giveText)
				sample, _ := f.view.Authored(gorules.New(), field.ID, field.Type)
				assert.Equal(t, sample, tt.want, "View.Authored lifts the text of a number through LiteralFor")
			})
		}

		t.Run("reports false for a call of a sibling method from a method subject", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			_, ok := gorules.New().LiteralFor(f.scope(method(storeName, "Put")), nil, "Get()", f.view)
			assert.False(t, ok, "Get is a method of Store, and a call of a method needs a receiver")
		})
	})
}

// A builtin's pair and zero allocate nothing, a struct's the reference
// to its type and its composites, and a literal its reading. The
// ordinary run, which runs no benchmark, checks those ceilings here.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valueCalls(t))
}

// BenchmarkValues measures the values a generated check derives for
// every type it writes a value of.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valueCalls(b))
}

// pairOf derives a reference's pair and fails unless both derived.
func pairOf(tb assert.TB, f *fixture, ref *node.TypeRef, hint string) (emit.Value, emit.Value) {
	tb.Helper()

	sample, alternate := gorules.New().DeriveSamples(ref, hint, f.view)
	assert.True(tb, sample.OK(), "the sample derives: "+sample.Refusal.String())
	assert.True(tb, alternate.OK(), "the alternate derives: "+alternate.Refusal.String())
	return sample.Value, alternate.Value
}

// refusalOf returns the refusal of a reference's sample.
func refusalOf(f *fixture, ref *node.TypeRef) rules.Refusal {
	sample, _ := gorules.New().DeriveSamples(ref, "", f.view)
	return sample.Refusal
}

// valueCalls returns a call of each value derivation over a builtin
// and over the fixture's Row, and of LiteralFor over a decimal integer.
func valueCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := gorules.New()
	scalar, row := builtin(intSpelling), ref(fxPath, rowType, symbol.KindStruct)
	scope := f.scope(row.Target)
	slice := composite("[]string", symbol.FormList, builtin(stringSpelling))
	m := composite("map[string]int", symbol.FormMap, builtin(stringSpelling), builtin(intSpelling))
	fn := composite("func(string) error", symbol.FormFunc, builtin(stringSpelling), builtin(errorSpelling))
	fn.Split = 1
	ch := composite("chan int", symbol.FormStream, builtin(intSpelling))
	top := builtin(anySpelling)
	var (
		sample rules.Sample
		value  emit.Value
	)
	return []allocCall{
		{
			name: "DeriveSamples", caseName: "a scalar",
			call:  func() { sample, _ = r.DeriveSamples(scalar, "", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, sample.Value.Text, derivedInt, "DeriveSamples returns 42") },
		},
		{
			name: "DeriveSamples", caseName: "a struct", allocs: structSamplesAllocs,
			call:  func() { sample, _ = r.DeriveSamples(row, "", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "DeriveSamples derives Row") },
		},
		{
			name: "DeriveSamples", caseName: "a slice", allocs: sliceSamplesAllocs,
			call: func() { sample, _ = r.DeriveSamples(slice, tagHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueComposite, "DeriveSamples derives the slice")
			},
		},
		{
			name: "DeriveSamples", caseName: "a map", allocs: mapSamplesAllocs,
			call: func() { sample, _ = r.DeriveSamples(m, keyHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueComposite, "DeriveSamples derives the map")
			},
		},
		{
			name: "DeriveSamples", caseName: "a function type", allocs: funcSamplesAllocs,
			call: func() { sample, _ = r.DeriveSamples(fn, "", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueFunc, "DeriveSamples derives the function")
			},
		},
		{
			name: "DeriveSamples", caseName: "a channel", allocs: chanSamplesAllocs,
			call: func() { sample, _ = r.DeriveSamples(ch, "", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueMake, "DeriveSamples derives the make")
			},
		},
		{
			name: "DeriveSamples", caseName: "any", allocs: anySamplesAllocs,
			call: func() { sample, _ = r.DeriveSamples(top, tagHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Text, tagHint+sampleSuffix, "DeriveSamples derives the hinted string")
			},
		},
		{
			name: "ZeroValue", caseName: "a scalar",
			call:  func() { value, _ = r.ZeroValue(scalar, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, zeroText, "ZeroValue returns 0") },
		},
		{
			name: "ZeroValue", caseName: "a struct", allocs: structZeroAllocs,
			call: func() { value, _ = r.ZeroValue(row, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueComposite, "ZeroValue returns the empty composite")
			},
		},
		{
			name: "LiteralFor", caseName: "a decimal integer", allocs: literalAllocs,
			call:  func() { value, _ = r.LiteralFor(scope, scalar, "42", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, "42", "LiteralFor reads 42") },
		},
		{
			name: "LiteralFor", caseName: "the name of a constant", allocs: nameLiteralAllocs,
			call: func() { value, _ = r.LiteralFor(scope, scalar, "Limit", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value, emit.Name(id(fxPath, "Limit", symbol.KindConstant)), "LiteralFor reads Limit")
			},
		},
		{
			name: "LiteralFor", caseName: "a call of a stand-in function", allocs: callLiteralAllocs,
			call: func() { value, _ = r.LiteralFor(scope, scalar, "time.Now()", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueCall, "LiteralFor reads the call of time.Now")
			},
		},
		{
			name: "LiteralFor", caseName: "a constant under a full import path",
			call: func() { value, _ = r.LiteralFor(scope, scalar, depPath+".Ratio", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value, emit.Name(id(depPath, "Ratio", symbol.KindConstant)), "LiteralFor reads Ratio")
			},
		},
		{
			name: "LiteralFor", caseName: "a call of a stand-in function under a full import path",
			call: func() { value, _ = r.LiteralFor(scope, scalar, seedPath+".Defaults()", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueCall, "LiteralFor reads the call of Defaults")
			},
		},
	}
}

// stampWeight states a sample and an alternate on the fixture's
// Weight, a defined float64.
func stampWeight(tb assert.TB, f *fixture) {
	tb.Helper()

	weight := id(fxPath, weightType, symbol.KindAlias)
	stamp(tb, f.Facts, weight, f.Keys.Sample, authoredWeight)
	stamp(tb, f.Facts, weight, f.Keys.Alternate, otherWeight)
}
