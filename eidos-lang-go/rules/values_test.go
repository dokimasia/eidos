// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/emit"
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
)

// The fixture's declarations and fields the value cases derive.
const (
	rowType    = "Row"
	weightType = "Weight"
	plainType  = "Plain"
	colorType  = "Color"
	modeType   = "Mode"
	bareType   = "Bare"
	readerType = "Reader"
	ghostType  = "Ghost"
	loopType   = "Loop"
	idField    = "ID"
	nextField  = "Next"
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
	// structZeroAllocs is a struct's zero value: the reference to the
	// type of the empty composite.
	structZeroAllocs = 1
	// literalAllocs is a decimal integer read against int: the scanner's
	// file set and file, the token and spelling lists, the constant,
	// and the decimal text.
	literalAllocs = 6
)

// A generated check writes every value through these three. Each
// derivation, each refusal and each authored part is pinned here.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
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

		refusals := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{name: "returns RefusedNoLiteral for any", give: builtin(anySpelling), want: rules.RefusedNoLiteral},
			{name: "returns RefusedNoLiteral for error", give: builtin(errorSpelling), want: rules.RefusedNoLiteral},
			{
				name: "returns RefusedNoLiteral for comparable",
				give: builtin(comparableSpelling), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for complex128",
				give: builtin(complexSpelling), want: rules.RefusedNoLiteral,
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
				give: composite("map[string]any", symbol.FormMap, builtin(stringSpelling), builtin(anySpelling)),
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a function type",
				give: composite("func()", symbol.FormFunc), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a channel",
				give: composite("chan int", symbol.FormStream, builtin(intSpelling)), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for an inline struct",
				give: &node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline}, want: rules.RefusedNoLiteral,
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

		t.Run("returns a composite setting the first exported field for a struct", func(t *testing.T) {
			t.Parallel()

			row := ref(fxPath, rowType, symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), row, "")
			assert.Equal(t, s, emit.Composite(rules.EmitRef(row),
				emit.NamedField(idField, emit.Literal(emit.LiteralInt, derivedInt))),
				"a constructor in another package sets the exported field")
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

		t.Run("returns conversions of the stamped values for an enumeration", func(t *testing.T) {
			t.Parallel()

			color := ref(fxPath, colorType, symbol.KindEnum)
			s, a := pairOf(t, loaded(t), color, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Conversion(rules.EmitRef(color), emit.Raw(golang.Lang, "0")),
				emit.Conversion(rules.EmitRef(color), emit.Raw(golang.Lang, "1")),
			}, "the exact values the frontend stamped, as Go text")
		})

		t.Run("returns conversions of the stamped strings for a string enumeration", func(t *testing.T) {
			t.Parallel()

			mode := ref(fxPath, modeType, symbol.KindEnum)
			s, a := pairOf(t, loaded(t), mode, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Conversion(rules.EmitRef(mode), emit.Raw(golang.Lang, `"read"`)),
				emit.Conversion(rules.EmitRef(mode), emit.Raw(golang.Lang, `"write"`)),
			}, "the exact values the frontend stamped, quotes included")
		})

		t.Run("derives a pointer to a struct whose first field is no pointer", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			sample, _ := gorules.New().SamplesOf(f.field(t, rowType, nextField).Type, "", f.view)
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
			assert.False(t, slices.Contains(read, f.field(t, rowType, idField).ID),
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
		weight := ref(fxPath, weightType, symbol.KindAlias)
		nilValue := emit.Literal(emit.LiteralNil, "")
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
				name: "returns a conversion of the underlying zero for a defined type",
				give: weight,
				want: emit.Conversion(rules.EmitRef(weight), emit.Number(emit.LiteralFloat, zeroText, bits64)),
			},
			{
				name: "returns the target's zero for a transparent alias",
				give: ref(fxPath, plainType, symbol.KindAlias), want: emit.Literal(emit.LiteralInt, zeroText),
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

				got, ok := gorules.New().LiteralFor(nil, nil, tt.give, rules.View{})
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
		}
		for _, tt := range unreadable {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := gorules.New().LiteralFor(nil, nil, tt.give, rules.View{})
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
		}
		for _, tt := range typed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				got, ok := gorules.New().LiteralFor(f.file, tt.give, tt.giveText, f.view)
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
		}
		for _, tt := range outside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, ok := gorules.New().LiteralFor(f.file, tt.give, tt.giveText, f.view)
				assert.False(t, ok, "the value is outside the builtin")
			})
		}
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

	sample, alternate := gorules.New().SamplesOf(ref, hint, f.view)
	assert.True(tb, sample.OK(), "the sample derives: "+sample.Refusal.String())
	assert.True(tb, alternate.OK(), "the alternate derives: "+alternate.Refusal.String())
	return sample.Value, alternate.Value
}

// refusalOf returns the refusal of a reference's sample.
func refusalOf(f *fixture, ref *node.TypeRef) rules.Refusal {
	sample, _ := gorules.New().SamplesOf(ref, "", f.view)
	return sample.Refusal
}

// valueCalls returns a call of each value derivation over a builtin
// and over the fixture's Row, and of LiteralFor over a decimal integer.
func valueCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := gorules.New()
	scalar, row := builtin(intSpelling), ref(fxPath, rowType, symbol.KindStruct)
	slice := composite("[]string", symbol.FormList, builtin(stringSpelling))
	m := composite("map[string]int", symbol.FormMap, builtin(stringSpelling), builtin(intSpelling))
	var (
		sample rules.Sample
		value  emit.Value
	)
	return []allocCall{
		{
			name:  "SamplesOf",
			call:  func() { sample, _ = r.SamplesOf(scalar, "", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, sample.Value.Text, derivedInt, "SamplesOf returns 42") },
		},
		{
			name: "SamplesOf/a struct", allocs: structSamplesAllocs,
			call:  func() { sample, _ = r.SamplesOf(row, "", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives Row") },
		},
		{
			name: "SamplesOf/a slice", allocs: sliceSamplesAllocs,
			call: func() { sample, _ = r.SamplesOf(slice, tagHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueComposite, "SamplesOf derives the slice")
			},
		},
		{
			name: "SamplesOf/a map", allocs: mapSamplesAllocs,
			call: func() { sample, _ = r.SamplesOf(m, keyHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueComposite, "SamplesOf derives the map")
			},
		},
		{
			name:  "ZeroValue",
			call:  func() { value, _ = r.ZeroValue(scalar, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, zeroText, "ZeroValue returns 0") },
		},
		{
			name: "ZeroValue/a struct", allocs: structZeroAllocs,
			call: func() { value, _ = r.ZeroValue(row, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueComposite, "ZeroValue returns the empty composite")
			},
		},
		{
			name: "LiteralFor", allocs: literalAllocs,
			call:  func() { value, _ = r.LiteralFor(f.file, scalar, "42", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, "42", "LiteralFor reads 42") },
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
