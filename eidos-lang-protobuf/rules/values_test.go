// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The enum schema the value cases load: numbers written in every
// base protobuf admits, an alias sharing a number, and a first value
// that is not zero, which proto2 admits.
const (
	enumsPath   = "svc/enums.proto"
	enumsSource = `syntax = "proto2";
package svc.store;
enum Mode {
  option allow_alias = true;
  MODE_HIGH = 0x10;
  MODE_TOP = 16;
  MODE_OCTAL = 017;
  MODE_LOW = - 1;
}
`
)

// The schema the refusal cases load: an enum with one number, a oneof
// whose one member derives nothing, and a message that contains only
// itself.
const (
	refusalsPath   = "svc/refusals.proto"
	refusalsSource = `syntax = "proto3";
package svc.store;
enum Single {
  SINGLE_ONLY = 0;
}
message Hollow {}
message Holder {
  oneof pick {
    Hollow hollow = 1;
  }
}
message Node {
  Node next = 1;
}
`
)

// The enum type the conversions name, and the width every enum
// number is stated at.
const (
	modeName = "Mode"
	enumBits = 32
)

// The spellings the value cases name: the wire's scalars, the
// well-known types, and a message no schema declares.
const (
	int32Spelling     = "int32"
	int64Spelling     = "int64"
	uint32Spelling    = "uint32"
	sfixed32Spelling  = "sfixed32"
	floatSpelling     = "float"
	doubleSpelling    = "double"
	boolSpelling      = "bool"
	stringSpelling    = "string"
	bytesSpelling     = "bytes"
	int32Wrapper      = "google.protobuf.Int32Value"
	int64Wrapper      = "google.protobuf.Int64Value"
	boolWrapper       = "google.protobuf.BoolValue"
	dottedWrapper     = ".google.protobuf.StringValue"
	timestampSpelling = "google.protobuf.Timestamp"
	anySpelling       = "google.protobuf.Any"
	ghostSpelling     = "some.Ghost"
)

// The fixture's declarations, fields and enum values the cases read.
const (
	rowName    = "Row"
	emptyName  = "Empty"
	ghostName  = "Ghost"
	colourName = "Colour"
	bodyName   = "body"
	nameField  = "name"
	textField  = "text"
	redValue   = "COLOUR_RED"
	octalValue = "MODE_OCTAL"
	ghostValue = "MODE_GHOST"
)

// The values the wire table derives, pinned: the numbers, the truth
// values, the hint a string takes where the caller names none with
// the suffixes of its two halves, and the widths.
const (
	derivedInt      = "42"
	derivedAltInt   = "7"
	derivedFloat    = "1.5"
	derivedAltFloat = "2.5"
	zeroText        = "0"
	trueText        = "true"
	falseText       = "false"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
	bits32          = 32
	bits64          = 64
)

// fixturePlugin is the plugin the cases' authored stamps claim.
const fixturePlugin = "fixture"

// The hints the cases pass, and the values an author states.
const (
	nameHint      = "name"
	tagHint       = "tag"
	keyHint       = "k"
	idHint        = "id"
	authoredText  = "us-east"
	otherText     = "eu-west"
	authoredValue = "COLOUR_UNSPECIFIED"
	authoredRow   = "Row{name: 1}"
	otherRow      = "Row{name: 2}"
	authoredEmpty = "Empty{}"
	longText      = "a string past sixteen bytes"
	longString    = `"` + longText + `"`
)

// The allocations of a value.
const (
	// messageSamplesAllocs is Row's pair: the reference to the message,
	// the field list of each composite, and the hinted texts of its first
	// field, a string.
	messageSamplesAllocs = 1 + 2 + 2
	// listSamplesAllocs is a repeated string's pair: the reference to the
	// list with its list of children and its element's reference, the
	// element list of each composite, and the hinted texts.
	listSamplesAllocs = 3 + 2 + 2
	// enumSamplesAllocs is Colour's pair: the reference to the enum, and
	// the conversion of each value.
	enumSamplesAllocs = 1 + 2
	// enumZeroAllocs is Colour's zero: the reference to the enum, and the
	// conversion.
	enumZeroAllocs = 1 + 1
	// stringLiteralAllocs is a string literal's unescaped text, sized
	// once.
	stringLiteralAllocs = 1
)

// The values are what a check generator writes, so each form's
// sample pair, its zero and the literals a schema's text types to are
// pinned at the wire's widths.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		scalars := []struct {
			name          string
			give          string
			giveHint      string
			wantSample    emit.Value
			wantAlternate emit.Value
		}{
			{
				name:          "returns integers at 64 bits for int64",
				give:          int64Spelling,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits64),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits64),
			},
			{
				name:          "returns integers at 32 bits for sfixed32",
				give:          sfixed32Spelling,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits32),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits32),
			},
			{
				name:          "returns floats at 32 bits for float",
				give:          floatSpelling,
				wantSample:    emit.Number(emit.LiteralFloat, derivedFloat, bits32),
				wantAlternate: emit.Number(emit.LiteralFloat, derivedAltFloat, bits32),
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
				name:          "returns the default hint for bytes without a hint",
				give:          bytesSpelling,
				wantSample:    emit.Literal(emit.LiteralString, defaultHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, defaultHint+alternateSuffix),
			},
			{
				name:          "returns the wrapped scalar's pair for a wrapper",
				give:          int32Wrapper,
				wantSample:    emit.Number(emit.LiteralInt, derivedInt, bits32),
				wantAlternate: emit.Number(emit.LiteralInt, derivedAltInt, bits32),
			},
			{
				name:          "returns the wrapped scalar's pair for a wrapper spelled with a leading dot",
				give:          dottedWrapper,
				giveHint:      idHint,
				wantSample:    emit.Literal(emit.LiteralString, idHint+sampleSuffix),
				wantAlternate: emit.Literal(emit.LiteralString, idHint+alternateSuffix),
			},
		}
		for _, tt := range scalars {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, a := pairOf(t, loaded(t), builtin(tt.give), tt.giveHint)
				assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{tt.wantSample, tt.wantAlternate},
					"the wire table's pair")
			})
		}

		refusals := []struct {
			name string
			give *node.TypeRef
			want rules.Refusal
		}{
			{
				name: "returns RefusedNoLiteral for google.protobuf.Timestamp",
				give: builtin(timestampSpelling), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for google.protobuf.Any",
				give: builtin(anySpelling), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedUnresolved for a message no schema declares",
				give: builtin(ghostSpelling), want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedNoLiteral for a stream",
				give: composite("stream Row", symbol.FormStream, ref(svcPkg, rowName, symbol.KindStruct)),
				want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedUnresolved for a map whose value is unresolved",
				give: composite("map<string, Ghost>", symbol.FormMap,
					builtin(stringSpelling), ref(svcPkg, ghostName, symbol.KindStruct)),
				want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedUnresolved for a repeated field of a message the view does not contain",
				give: composite("repeated Ghost", symbol.FormList, ref(svcPkg, ghostName, symbol.KindStruct)),
				want: rules.RefusedUnresolved,
			},
			{
				name: "returns RefusedUnresolved for a message the view does not contain",
				give: ref(svcPkg, ghostName, symbol.KindStruct), want: rules.RefusedUnresolved,
			},
			{name: "returns RefusedNoLiteral for a nil reference", want: rules.RefusedNoLiteral},
			{
				name: "returns RefusedNoLiteral for a message with nothing it can set",
				give: ref(svcPkg, emptyName, symbol.KindStruct), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a form protobuf does not write",
				give: composite("func()", symbol.FormFunc), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a service",
				give: ref(svcPkg, "Store", symbol.KindInterface), want: rules.RefusedNoLiteral,
			},
			{
				name: "returns RefusedNoLiteral for a repeated field without an element",
				give: composite("repeated", symbol.FormList), want: rules.RefusedNoLiteral,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, refusalOf(loaded(t), tt.give), tt.want, "the refusal names its reason")
			})
		}

		underived := []struct {
			name string
			give *node.TypeRef
		}{
			{
				name: "returns RefusedNoLiteral for an enum with one number",
				give: ref(svcPkg, "Single", symbol.KindEnum),
			},
			{
				name: "returns RefusedNoLiteral for a oneof whose members derive nothing",
				give: &node.TypeRef{Spelling: "pick", Target: member(svcPkg, "Holder", "pick", symbol.KindSum)},
			},
			{
				name: "returns RefusedNoLiteral for a message that contains only itself",
				give: ref(svcPkg, "Node", symbol.KindStruct),
			},
		}
		for _, tt := range underived {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loadedFrom(t, map[string]string{refusalsPath: refusalsSource})
				assert.Equal(t, refusalOf(f, tt.give), rules.RefusedNoLiteral, "no two values derive")
			})
		}

		t.Run("returns a one-element composite for a repeated field", func(t *testing.T) {
			t.Parallel()

			list := composite("repeated string", symbol.FormList, builtin(stringSpelling))
			s, a := pairOf(t, loaded(t), list, tagHint)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(list),
					emit.Element(emit.Literal(emit.LiteralString, tagHint+sampleSuffix))),
				emit.Composite(rules.EmitRef(list),
					emit.Element(emit.Literal(emit.LiteralString, tagHint+alternateSuffix))),
			}, "the halves differ in the element")
		})

		t.Run("returns a one-entry composite for a map", func(t *testing.T) {
			t.Parallel()

			m := composite("map<string, int64>", symbol.FormMap, builtin(stringSpelling), builtin(int64Spelling))
			s, a := pairOf(t, loaded(t), m, keyHint)
			value := emit.Number(emit.LiteralInt, derivedInt, bits64)
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+sampleSuffix), value)),
				emit.Composite(rules.EmitRef(m),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, keyHint+alternateSuffix), value)),
			}, "the halves differ in the key")
		})

		t.Run("returns the element's pair for an optional field", func(t *testing.T) {
			t.Parallel()

			s, a := pairOf(t, loaded(t), composite("optional int64", symbol.FormOptional, builtin(int64Spelling)), "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Number(emit.LiteralInt, derivedInt, bits64),
				emit.Number(emit.LiteralInt, derivedAltInt, bits64),
			}, "presence is not a value")
		})

		t.Run("returns a composite setting the first field for a message", func(t *testing.T) {
			t.Parallel()

			row := ref(svcPkg, rowName, symbol.KindStruct)
			s, _ := pairOf(t, loaded(t), row, "")
			assert.Equal(t, s, emit.Composite(rules.EmitRef(row),
				emit.NamedField(nameField, emit.Literal(emit.LiteralString, nameField+sampleSuffix))),
				"the field's name is the string's hint")
		})

		t.Run("returns the first two numbers converted for an enum", func(t *testing.T) {
			t.Parallel()

			colour := ref(svcPkg, colourName, symbol.KindEnum)
			s, a := pairOf(t, loaded(t), colour, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{enumValue(colour, "0"), enumValue(colour, "1")},
				"every target spells a number converted to the enum type")
		})

		t.Run("returns two distinct numbers in any base for an enum", func(t *testing.T) {
			t.Parallel()

			mode := ref(svcPkg, modeName, symbol.KindEnum)
			s, a := pairOf(t, loadedFrom(t, map[string]string{enumsPath: enumsSource}), mode, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{enumValue(mode, "16"), enumValue(mode, "15")},
				"the alias sharing sixteen is passed over and the octal number is read")
		})

		t.Run("returns a composite setting the first variant's field for a oneof", func(t *testing.T) {
			t.Parallel()

			body := &node.TypeRef{Spelling: bodyName, Target: member(svcPkg, rowName, bodyName, symbol.KindSum)}
			s, _ := pairOf(t, loaded(t), body, "")
			assert.Equal(t, s, emit.Composite(rules.EmitRef(body),
				emit.NamedField(textField, emit.Literal(emit.LiteralString, textField+sampleSuffix))),
				"a oneof's value is one variant's field")
		})

		t.Run("returns the values authored on an element's type for a repeated field", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			colour := ref(svcPkg, colourName, symbol.KindEnum)
			stamp(t, f, f.Keys.Sample, colour.Target, authoredValue)
			stamp(t, f, f.Keys.Alternate, colour.Target, redValue)
			list := composite("repeated Colour", symbol.FormList, colour)
			s, a := pairOf(t, f, list, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(list), emit.Element(emit.Raw(protobuf.Lang, authoredValue))),
				emit.Composite(rules.EmitRef(list), emit.Element(emit.Raw(protobuf.Lang, redValue))),
			}, "each element takes the text the author stated on Colour")
		})

		t.Run("returns the value authored on a map value's type", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			colour := ref(svcPkg, colourName, symbol.KindEnum)
			stamp(t, f, f.Keys.Sample, colour.Target, authoredValue)
			m := composite("map<string, Colour>", symbol.FormMap, builtin(stringSpelling), colour)
			s, _ := pairOf(t, f, m, keyHint)
			assert.Equal(t, s, emit.Composite(rules.EmitRef(m), emit.KeyedEntry(
				emit.Literal(emit.LiteralString, keyHint+sampleSuffix),
				emit.Raw(protobuf.Lang, authoredValue),
			)), "the entry's value takes the text the author stated on Colour")
		})

		t.Run("returns the values authored on a field for a message", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			row := ref(svcPkg, rowName, symbol.KindStruct)
			name := fieldOf(t, f, row.Target, nameField)
			stamp(t, f, f.Keys.Sample, name.ID, authoredText)
			stamp(t, f, f.Keys.Alternate, name.ID, otherText)
			s, a := pairOf(t, f, row, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(row),
					emit.NamedField(nameField, emit.Literal(emit.LiteralString, authoredText))),
				emit.Composite(rules.EmitRef(row),
					emit.NamedField(nameField, emit.Literal(emit.LiteralString, otherText))),
			}, "the composite sets the strings the author stated on name")
		})

		t.Run("returns the values authored on a field for a oneof", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			body := &node.TypeRef{Spelling: bodyName, Target: member(svcPkg, rowName, bodyName, symbol.KindSum)}
			sum, is := f.decl(t, body.Target).(*node.Sum)
			assert.True(t, is, "body is a oneof")
			text := sum.Variants[0].Fields[0]
			stamp(t, f, f.Keys.Sample, text.ID, authoredText)
			stamp(t, f, f.Keys.Alternate, text.ID, otherText)
			s, a := pairOf(t, f, body, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(body),
					emit.NamedField(textField, emit.Literal(emit.LiteralString, authoredText))),
				emit.Composite(rules.EmitRef(body),
					emit.NamedField(textField, emit.Literal(emit.LiteralString, otherText))),
			}, "the composite sets the strings the author stated on text")
		})

		t.Run("reads no field of an element whose type states both halves", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			row := ref(svcPkg, rowName, symbol.KindStruct)
			stamp(t, f, f.Keys.Sample, row.Target, authoredRow)
			stamp(t, f, f.Keys.Alternate, row.Target, otherRow)
			pairOf(t, f, composite("repeated Row", symbol.FormList, row), "")
			reads, recorded := f.view.Reads.(*store.ReadSet)
			assert.True(t, recorded, "the view records into a read set")
			var read []symbol.Identity
			for subject := range reads.Facts() {
				read = append(read, subject)
			}
			assert.NotContains(t, read, fieldOf(t, f, row.Target, nameField).ID,
				"the authored pair depends on no derivation, so no field of Row is an edge")
		})

		t.Run("returns RefusedNoLiteral for both halves when a repeated element has one value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			empty := ref(svcPkg, emptyName, symbol.KindStruct)
			stamp(t, f, f.Keys.Sample, empty.Target, authoredEmpty)
			s, a := protorules.New().SamplesOf(composite("repeated Empty", symbol.FormList, empty), "", f.view)
			assert.Equal(t, [2]rules.Refusal{s.Refusal, a.Refusal},
				[2]rules.Refusal{rules.RefusedNoLiteral, rules.RefusedNoLiteral},
				"Empty derives no alternate, so neither half has a value")
		})

		t.Run("completes an element's unstated half with the derived value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			colour := ref(svcPkg, colourName, symbol.KindEnum)
			stamp(t, f, f.Keys.Sample, colour.Target, authoredValue)
			list := composite("repeated Colour", symbol.FormList, colour)
			s, a := pairOf(t, f, list, "")
			assert.Equal(t, [2]emit.Value{s, a}, [2]emit.Value{
				emit.Composite(rules.EmitRef(list), emit.Element(emit.Raw(protobuf.Lang, authoredValue))),
				emit.Composite(rules.EmitRef(list), emit.Element(enumValue(colour, "1"))),
			}, "the authored element pairs with Colour's derived alternate")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		list := composite("repeated string", symbol.FormList, builtin(stringSpelling))
		m := composite("map<string, int64>", symbol.FormMap, builtin(stringSpelling), builtin(int64Spelling))
		colour := ref(svcPkg, colourName, symbol.KindEnum)
		nilValue := emit.Literal(emit.LiteralNil, "")
		tests := []struct {
			name string
			give *node.TypeRef
			want emit.Value
		}{
			{
				name: "returns 0 at 64 bits for int64",
				give: builtin(int64Spelling), want: emit.Number(emit.LiteralInt, zeroText, bits64),
			},
			{
				name: "returns 0 at 64 bits for double",
				give: builtin(doubleSpelling), want: emit.Number(emit.LiteralFloat, zeroText, bits64),
			},
			{
				name: "returns false for bool",
				give: builtin(boolSpelling), want: emit.Literal(emit.LiteralBool, falseText),
			},
			{
				name: "returns the empty string for string",
				give: builtin(stringSpelling), want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name: "returns the empty string for bytes",
				give: builtin(bytesSpelling), want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name: "returns the empty composite for a repeated field",
				give: list, want: emit.Composite(rules.EmitRef(list)),
			},
			{name: "returns the empty composite for a map", give: m, want: emit.Composite(rules.EmitRef(m))},
			{
				name: "returns nil for an optional field",
				give: composite("optional int64", symbol.FormOptional, builtin(int64Spelling)), want: nilValue,
			},
			{name: "returns nil for a wrapper", give: builtin(boolWrapper), want: nilValue},
			{name: "returns nil for a message", give: ref(svcPkg, rowName, symbol.KindStruct), want: nilValue},
			{name: "returns the first declared value for an enum", give: colour, want: enumValue(colour, zeroText)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := protorules.New().ZeroValue(tt.give, loaded(t).view)
				assert.True(t, ok, "the zero derives")
				assert.Equal(t, got, tt.want, "the value an absent field reads as")
			})
		}

		t.Run("returns an enum's first declared value whatever its number", func(t *testing.T) {
			t.Parallel()

			mode := ref(svcPkg, modeName, symbol.KindEnum)
			got, ok := protorules.New().ZeroValue(mode, loadedFrom(t, map[string]string{enumsPath: enumsSource}).view)
			assert.True(t, ok, "an enum with values has a zero")
			assert.Equal(t, got, enumValue(mode, "16"),
				"protobuf's default for an enum field is the first value declared")
		})

		unplaced := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "reports false for a spelling the rules cannot place", give: builtin(ghostSpelling)},
			{
				name: "reports false for a message the view does not contain",
				give: ref(svcPkg, ghostName, symbol.KindStruct),
			},
			{name: "reports false for a stream", give: composite("stream Row", symbol.FormStream)},
			{name: "reports false for a service", give: ref(svcPkg, "Store", symbol.KindInterface)},
			{name: "reports false for a nil reference"},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := protorules.New().ZeroValue(tt.give, loaded(t).view)
				assert.False(t, ok, "no zero to compare a default against")
			})
		}
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		typed := []struct {
			name     string
			give     *node.TypeRef
			giveText string
			want     emit.Value
		}{
			{
				name: "returns a hexadecimal integer in decimal at 32 bits for int32",
				give: builtin(int32Spelling), giveText: "0x1F", want: emit.Number(emit.LiteralInt, "31", bits32),
			},
			{
				name: "reads a leading zero as octal for int32",
				give: builtin(int32Spelling), giveText: "017", want: emit.Number(emit.LiteralInt, "15", bits32),
			},
			{
				name: "returns an integer as a float at 64 bits for double",
				give: builtin(doubleSpelling), giveText: "5", want: emit.Number(emit.LiteralFloat, "5", bits64),
			},
			{
				name: "writes an exponent in canonical decimal for float",
				give: builtin(floatSpelling), giveText: "1e3", want: emit.Number(emit.LiteralFloat, "1000", bits32),
			},
			{
				name: "returns the scalar's value for a wrapper",
				give: builtin(int64Wrapper), giveText: "7", want: emit.Number(emit.LiteralInt, "7", bits64),
			},
			{
				name: "returns a truth value for bool",
				give: builtin(boolSpelling), giveText: trueText, want: emit.Literal(emit.LiteralBool, trueText),
			},
			{
				name:     "returns the bytes octal escapes write for string",
				give:     builtin(stringSpelling),
				giveText: `"caf\303\251"`,
				want:     emit.Literal(emit.LiteralString, "café"),
			},
			{
				name: "returns any bytes for bytes",
				give: builtin(bytesSpelling), giveText: `"\xff"`, want: emit.Literal(emit.LiteralString, "\xff"),
			},
			{
				name:     "returns the element's value for an optional field",
				give:     composite("optional int64", symbol.FormOptional, builtin(int64Spelling)),
				giveText: "1",
				want:     emit.Number(emit.LiteralInt, "1", bits64),
			},
			{
				name:     "returns a literal as its own kind for a message the view does not contain",
				give:     ref(svcPkg, ghostName, symbol.KindStruct),
				giveText: "1",
				want:     emit.Literal(emit.LiteralInt, "1"),
			},
		}
		for _, tt := range typed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := protorules.New().LiteralFor(nil, tt.give, tt.giveText, loaded(t).view)
				assert.True(t, ok, "the text is a value of the type")
				assert.Equal(t, got, tt.want, "written back at the wire's width")
			})
		}

		outside := []struct {
			name     string
			give     *node.TypeRef
			giveText string
		}{
			{name: "reports false for an integer outside int32", give: builtin(int32Spelling), giveText: "2147483648"},
			{name: "reports false for a negative uint32", give: builtin(uint32Spelling), giveText: "-1"},
			{name: "reports false for a float literal as int64", give: builtin(int64Spelling), giveText: "2.0"},
			{name: "reports false for inf as float", give: builtin(floatSpelling), giveText: "inf"},
			{name: "reports false for a number as bool", give: builtin(boolSpelling), giveText: "1"},
			{name: "reports false for invalid UTF-8 as string", give: builtin(stringSpelling), giveText: `"\xff"`},
			{name: "reports false for a number as string", give: builtin(stringSpelling), giveText: "7"},
			{name: "reports false for a number as bytes", give: builtin(bytesSpelling), giveText: "7"},
			{name: "reports false for a message", give: ref(svcPkg, rowName, symbol.KindStruct), giveText: "1"},
			{
				name: "reports false for a repeated field",
				give: composite("repeated int64", symbol.FormList, builtin(int64Spelling)), giveText: "1",
			},
		}
		for _, tt := range outside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := protorules.New().LiteralFor(nil, tt.give, tt.giveText, loaded(t).view)
				assert.False(t, ok, "no value of the type")
			})
		}

		t.Run("returns an enum value's number for its name", func(t *testing.T) {
			t.Parallel()

			mode := ref(svcPkg, modeName, symbol.KindEnum)
			got, ok := protorules.New().LiteralFor(nil, mode, octalValue,
				loadedFrom(t, map[string]string{enumsPath: enumsSource}).view)
			assert.True(t, ok, "a value's name is a value of the enum")
			assert.Equal(t, got, enumValue(mode, "15"), "converted from its number")
		})

		enumOutside := []struct {
			name string
			give string
		}{
			{name: "reports false for a name the enum does not declare", give: ghostValue},
			{name: "reports false for a number as an enum", give: "15"},
		}
		for _, tt := range enumOutside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				mode := ref(svcPkg, modeName, symbol.KindEnum)
				_, ok := protorules.New().LiteralFor(nil, mode, tt.give,
					loadedFrom(t, map[string]string{enumsPath: enumsSource}).view)
				assert.False(t, ok, "a default names an enum value by its name")
			})
		}
	})
}

// A scalar's pair, zero and literal allocate nothing, and a composite,
// an enum value and a string literal allocate their parts. The
// ordinary run, which runs no benchmark, checks those ceilings here.
func TestValuesAllocs(t *testing.T) {
	checkAllocs(t, valueCalls(t))
}

// BenchmarkValues measures the values a generated check derives for
// every type it writes a value of.
func BenchmarkValues(b *testing.B) {
	benchCalls(b, valueCalls(b))
}

// pairOf derives a reference's two values and fails unless both
// derived.
func pairOf(tb assert.TB, f *fixture, ref *node.TypeRef, hint string) (emit.Value, emit.Value) {
	tb.Helper()

	sample, alternate := protorules.New().SamplesOf(ref, hint, f.view)
	assert.True(tb, sample.OK(), "the sample derives: "+sample.Refusal.String())
	assert.True(tb, alternate.OK(), "the alternate derives: "+alternate.Refusal.String())
	return sample.Value, alternate.Value
}

// refusalOf returns the refusal of a reference's sample.
func refusalOf(f *fixture, ref *node.TypeRef) rules.Refusal {
	sample, _ := protorules.New().SamplesOf(ref, "", f.view)
	return sample.Refusal
}

// enumValue returns an enum number converted to the enum a reference
// names, at the wire's width.
func enumValue(ref *node.TypeRef, number string) emit.Value {
	return emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, number, enumBits))
}

// valueCalls returns a call of each value derivation over a scalar, a
// message, a repeated field and an enum, and of LiteralFor over an
// integer and a string.
func valueCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := protorules.New()
	scalar, text := builtin(int64Spelling), builtin(stringSpelling)
	row, colour := ref(svcPkg, rowName, symbol.KindStruct), ref(svcPkg, colourName, symbol.KindEnum)
	list := composite("repeated string", symbol.FormList, text)
	var (
		sample rules.Sample
		value  emit.Value
	)
	return []allocCall{
		{
			name: "SamplesOf", caseName: "a scalar",
			call:  func() { sample, _ = r.SamplesOf(scalar, "", f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, sample.Value.Text, derivedInt, "SamplesOf returns 42") },
		},
		{
			name: "SamplesOf", caseName: "a message", allocs: messageSamplesAllocs,
			call:  func() { sample, _ = r.SamplesOf(row, "", f.view) },
			check: func(tb assert.TB) { assert.True(tb, sample.OK(), "SamplesOf derives Row") },
		},
		{
			name: "SamplesOf", caseName: "a repeated field", allocs: listSamplesAllocs,
			call: func() { sample, _ = r.SamplesOf(list, tagHint, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueComposite, "SamplesOf derives the list")
			},
		},
		{
			name: "SamplesOf", caseName: "an enum", allocs: enumSamplesAllocs,
			call: func() { sample, _ = r.SamplesOf(colour, "", f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, sample.Value.Kind, emit.ValueConversion, "SamplesOf derives Colour")
			},
		},
		{
			name: "ZeroValue", caseName: "a scalar",
			call:  func() { value, _ = r.ZeroValue(scalar, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, zeroText, "ZeroValue returns 0") },
		},
		{
			name: "ZeroValue", caseName: "an enum", allocs: enumZeroAllocs,
			call: func() { value, _ = r.ZeroValue(colour, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Kind, emit.ValueConversion, "ZeroValue returns the first value")
			},
		},
		{
			name: "LiteralFor", caseName: "an integer",
			call:  func() { value, _ = r.LiteralFor(nil, scalar, derivedInt, f.view) },
			check: func(tb assert.TB) { assert.Equal(tb, value.Text, derivedInt, "LiteralFor reads 42") },
		},
		{
			name: "LiteralFor", caseName: "a string", allocs: stringLiteralAllocs,
			call: func() { value, _ = r.LiteralFor(nil, text, longString, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, value.Text, longText, "LiteralFor reads the string")
			},
		},
	}
}

// stamp states one authored text on a subject under a kernel key, at
// plugin authority, the way the sample annotator stamps it.
func stamp(tb assert.TB, f *fixture, k meta.Key[string], subject symbol.Identity, text string) {
	tb.Helper()

	err := meta.Stamp(f.Facts, k, text, meta.Claim{
		Subject: subject, Authority: meta.AuthorityPlugin, Plugin: fixturePlugin,
	})
	assert.NoError(tb, err, "the author states a value")
}

// fieldOf returns a message's field by name.
func fieldOf(tb assert.TB, f *fixture, message symbol.Identity, name string) *node.Field {
	tb.Helper()

	s, is := f.decl(tb, message).(*node.Struct)
	assert.True(tb, is, "the declaration is a message")
	for _, field := range s.Fields {
		if field.Name == name {
			return field
		}
	}
	tb.Fatalf("%s declares no field %s", message.Name, name)
	return nil
}
