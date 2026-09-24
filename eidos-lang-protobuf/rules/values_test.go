// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
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

// The enum type the conversions name, and the width every enum
// number is stated at.
const (
	modeName = "Mode"
	enumBits = 32
)

// pairOf derives a reference's two values and fails unless both
// derived.
func pairOf(tb assert.TB, f *fixture, ref *node.TypeRef, hint string) (emit.Value, emit.Value) {
	tb.Helper()

	sample, alternate := protorules.New().SamplesOf(ref, hint, f.view)
	assert.True(tb, sample.OK(), "the sample derives: "+sample.Refusal.String())
	assert.True(tb, alternate.OK(), "the alternate derives: "+alternate.Refusal.String())
	return sample.Value, alternate.Value
}

// enumValue returns an enum number converted to the enum a reference
// names, at the wire's width.
func enumValue(ref *node.TypeRef, number string) emit.Value {
	return emit.Conversion(rules.EmitRef(ref), emit.Number(emit.LiteralInt, number, enumBits))
}

// The values are what a check generator writes, so each form's
// sample pair, its zero and the literals a schema's text types to are
// pinned at the wire's widths.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("derives the wire's scalars at the wire's width", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, builtin("int64"), "")
			assert.Equal(t, s, emit.Number(emit.LiteralInt, "42", 64), "an integer at sixty-four bits")
			assert.Equal(t, a, emit.Number(emit.LiteralInt, "7", 64), "and its alternate")
			s, _ = pairOf(t, f, builtin("sfixed32"), "")
			assert.Equal(t, s.Bits, 32, "a fixed spelling at the width its name states")
			s, _ = pairOf(t, f, builtin("float"), "")
			assert.Equal(t, s, emit.Number(emit.LiteralFloat, "1.5", 32), "a float at thirty-two bits")
			s, a = pairOf(t, f, builtin("bool"), "")
			assert.Equal(t, s.Text, "true", "a boolean")
			assert.Equal(t, a.Text, "false", "and its opposite")
			s, a = pairOf(t, f, builtin("string"), "name")
			assert.Equal(t, s.Text, "name-a", "a string with the hint")
			assert.Equal(t, a.Text, "name-b", "and differs in its suffix")
			s, _ = pairOf(t, f, builtin("bytes"), "")
			assert.Equal(t, s.Text, "sample-a", "bytes take the default hint")
		})

		t.Run("derives a wrapper as the scalar it wraps", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, _ := pairOf(t, f, builtin("google.protobuf.Int32Value"), "")
			assert.Equal(t, s, emit.Number(emit.LiteralInt, "42", 32),
				"a wrapper's value is its scalar's, because the wrapper adds presence alone")
			s, _ = pairOf(t, f, builtin(".google.protobuf.StringValue"), "id")
			assert.Equal(t, s.Text, "id-a", "spelled with its leading dot too")

			for _, spelling := range []string{"google.protobuf.Timestamp", "google.protobuf.Any"} {
				refused, _ := protorules.New().SamplesOf(builtin(spelling), "", f.view)
				assert.Equal(t, refused.Refusal, rules.RefusedNoLiteral,
					spelling+" has no literal protobuf states, so the value is the target's own")
			}
			refused, _ := protorules.New().SamplesOf(builtin("some.Ghost"), "", f.view)
			assert.Equal(t, refused.Refusal, rules.RefusedUnresolved,
				"and a message the workspace does not declare is unresolved")
		})

		t.Run("derives the composites the labels state", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, composite("repeated string", symbol.FormList, builtin("string")), "tag")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a repeated field is a composite")
			assert.Length(t, s.Fields, 1, "of one element")
			assert.True(t, s.Fields[0].Value.Text != a.Fields[0].Value.Text, "differing in the element")

			s, a = pairOf(t, f,
				composite("map<string, int64>", symbol.FormMap, builtin("string"), builtin("int64")), "k")
			assert.Length(t, s.Fields, 1, "a map has one keyed entry")
			assert.NotNil(t, s.Fields[0].Key, "which states its key")
			assert.True(t, s.Fields[0].Key.Text != a.Fields[0].Key.Text, "differing in the key")

			s, _ = pairOf(t, f, composite("optional int64", symbol.FormOptional, builtin("int64")), "")
			assert.Equal(t, s.Literal, emit.LiteralInt,
				"an optional field's value is its type's, because presence is not a value")

			refused, _ := protorules.New().SamplesOf(
				composite("stream Row", symbol.FormStream, ref(svcPkg, "Row", symbol.KindStruct)),
				"", f.view,
			)
			assert.Equal(t, refused.Refusal, rules.RefusedNoLiteral,
				"a stream is many of its message, and one value of it is not a value")

			refused, _ = protorules.New().SamplesOf(
				composite("map<string, Ghost>", symbol.FormMap,
					builtin("string"), ref(svcPkg, "Ghost", symbol.KindStruct)),
				"", f.view,
			)
			assert.Equal(t, refused.Refusal, rules.RefusedUnresolved,
				"a map passes its value's refusal through")
		})

		t.Run("derives a message and an enum from the schema", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, ref(svcPkg, "Row", symbol.KindStruct), "")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a message is a composite")
			assert.Length(t, s.Fields, 1, "setting one field")
			assert.True(t, s.Fields[0].Value.Text != a.Fields[0].Value.Text, "differing there")

			colour := ref(svcPkg, "Colour", symbol.KindEnum)
			s, a = pairOf(t, f, colour, "")
			assert.Equal(t, s, enumValue(colour, "0"),
				"an enum's value is its number converted to the enum type, which every target spells")
			assert.Equal(t, a, enumValue(colour, "1"), "and the second value's number")

			refused, _ := protorules.New().SamplesOf(
				ref(svcPkg, "Ghost", symbol.KindStruct), "", f.view,
			)
			assert.Equal(t, refused.Refusal, rules.RefusedUnresolved,
				"a message the view does not contain is unresolved")
			refused, _ = protorules.New().SamplesOf(nil, "", f.view)
			assert.Equal(t, refused.Refusal, rules.RefusedNoLiteral, "and nothing has no value")
		})

		t.Run("derives an enum pair from two distinct numbers in any base", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{enumsPath: enumsSource})
			mode := ref(svcPkg, modeName, symbol.KindEnum)
			s, a := pairOf(t, f, mode, "")
			assert.Equal(t, s, enumValue(mode, "16"), "the first value's hexadecimal number")
			assert.Equal(t, a, enumValue(mode, "15"),
				"then the next distinct number, the alias sharing sixteen passed over and octal read")
		})

		t.Run("derives a oneof through the first variant that yields a pair", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			body := &node.TypeRef{Spelling: "body", Target: member(svcPkg, "Row", "body", symbol.KindSum)}
			s, a := pairOf(t, f, body, "")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a oneof's value sets one variant's field")
			assert.Length(t, s.Fields, 1, "one field")
			assert.True(t, s.Fields[0].Value.Text != a.Fields[0].Value.Text,
				"and the pair differs there")
		})

		t.Run("refuses a message with nothing it can set", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			refused, _ := protorules.New().SamplesOf(
				ref(svcPkg, "Empty", symbol.KindStruct), "", f.view,
			)
			assert.Equal(t, refused.Refusal, rules.RefusedNoLiteral,
				"every value of a message with no field is one value, and a check needs two")
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each form's zero", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			zero := func(ref *node.TypeRef) emit.Value {
				v, ok := r.ZeroValue(ref, f.view)
				assert.True(t, ok, "the zero derives for "+ref.Spelling)
				return v
			}
			assert.Equal(t, zero(builtin("int64")), emit.Number(emit.LiteralInt, "0", 64),
				"an integer's zero at its width")
			assert.Equal(t, zero(builtin("double")), emit.Number(emit.LiteralFloat, "0", 64), "a float's")
			assert.Equal(t, zero(builtin("bool")).Text, "false", "a boolean's")
			assert.Equal(t, zero(builtin("string")), emit.Literal(emit.LiteralString, ""), "a string's")
			assert.Equal(t, zero(builtin("bytes")).Literal, emit.LiteralString, "and bytes'")

			assert.Equal(t, zero(composite("repeated string", symbol.FormList, builtin("string"))).Kind,
				emit.ValueComposite, "a repeated field's zero is the empty list")
			assert.Equal(t,
				zero(composite("map<string, int64>", symbol.FormMap, builtin("string"), builtin("int64"))).Kind,
				emit.ValueComposite, "a map's is the empty map")
			assert.Equal(t,
				zero(composite("optional int64", symbol.FormOptional, builtin("int64"))).Literal,
				emit.LiteralNil,
				"an absent optional field is absent, not zero, which is what presence means")
			assert.Equal(t, zero(builtin("google.protobuf.BoolValue")).Literal, emit.LiteralNil,
				"and so is an absent wrapper, which exists to give its scalar presence")
			assert.Equal(t, zero(ref(svcPkg, "Row", symbol.KindStruct)).Literal, emit.LiteralNil,
				"and an absent message: proto3 gives a message field no zero of its own")

			colour := ref(svcPkg, "Colour", symbol.KindEnum)
			assert.Equal(t, zero(colour), enumValue(colour, "0"),
				"an enum's zero is its first declared value, converted to the enum type")
		})

		t.Run("returns an enum's first declared value, whatever its number", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{enumsPath: enumsSource})
			mode := ref(svcPkg, modeName, symbol.KindEnum)
			v, ok := protorules.New().ZeroValue(mode, f.view)
			assert.True(t, ok, "an enum with values has a zero")
			assert.Equal(t, v, enumValue(mode, "16"),
				"protobuf's default for an enum field is the first value declared, not the one valued zero")
		})

		t.Run("reports false where no zero exists", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			_, ok := r.ZeroValue(builtin("some.Ghost"), f.view)
			assert.False(t, ok, "a spelling the rules cannot place has no zero")
			_, ok = r.ZeroValue(ref(svcPkg, "Ghost", symbol.KindStruct), f.view)
			assert.False(t, ok, "nor does a message the view does not contain")
			_, ok = r.ZeroValue(composite("stream Row", symbol.FormStream), f.view)
			assert.False(t, ok, "nor a stream")
			_, ok = r.ZeroValue(nil, f.view)
			assert.False(t, ok, "nor nothing")
		})
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		t.Run("types a number by the wire's width", func(t *testing.T) {
			t.Parallel()

			r := protorules.New()
			lit := func(spelling, text string) (emit.Value, bool) {
				return r.LiteralFor(nil, builtin(spelling), text, rules.View{})
			}
			v, ok := lit("int32", "0x1F")
			assert.True(t, ok, "a hexadecimal integer is an int32")
			assert.Equal(t, v, emit.Number(emit.LiteralInt, "31", 32), "written back in decimal at its width")
			v, _ = lit("int32", "017")
			assert.Equal(t, v.Text, "15", "a leading zero is octal")
			_, ok = lit("int32", "2147483648")
			assert.False(t, ok, "and an integer outside the width refuses")
			_, ok = lit("uint32", "-1")
			assert.False(t, ok, "as a negative one does for an unsigned type")
			_, ok = lit("int64", "2.0")
			assert.False(t, ok, "an integer type takes an integer literal alone, as protoc does")
			v, ok = lit("double", "5")
			assert.True(t, ok, "a float type takes an integer")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "5", 64), "as a float at its width")
			v, _ = lit("float", "1e3")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "1000", 32),
				"and an exponent in canonical decimal text")
			_, ok = lit("float", "inf")
			assert.False(t, ok, "infinity has no literal a target spells")
			v, ok = lit("google.protobuf.Int64Value", "7")
			assert.True(t, ok && v.Bits == 64, "a wrapper takes what its scalar takes")
		})

		t.Run("types a boolean, a string and bytes", func(t *testing.T) {
			t.Parallel()

			r := protorules.New()
			lit := func(spelling, text string) (emit.Value, bool) {
				return r.LiteralFor(nil, builtin(spelling), text, rules.View{})
			}
			v, ok := lit("bool", "true")
			assert.True(t, ok && v.Literal == emit.LiteralBool, "a truth value is a bool")
			_, ok = lit("bool", "1")
			assert.False(t, ok, "and a number is not")
			v, ok = lit("string", `"caf\303\251"`)
			assert.True(t, ok && v.Text == "café", "a string is its escaped bytes")
			_, ok = lit("string", `"\xff"`)
			assert.False(t, ok, "which must be valid UTF-8 for a string")
			v, ok = lit("bytes", `"\xff"`)
			assert.True(t, ok && v.Text == "\xff", "and may be any bytes for bytes")
			_, ok = lit("string", "7")
			assert.False(t, ok, "a number is no string")
		})

		t.Run("types an enum value by its name", func(t *testing.T) {
			t.Parallel()

			f := loadedFrom(t, map[string]string{enumsPath: enumsSource})
			r := protorules.New()
			mode := ref(svcPkg, modeName, symbol.KindEnum)
			v, ok := r.LiteralFor(nil, mode, "MODE_OCTAL", f.view)
			assert.True(t, ok, "a value's name is a value of the enum")
			assert.Equal(t, v, enumValue(mode, "15"), "converted from its number")
			_, ok = r.LiteralFor(nil, mode, "MODE_GHOST", f.view)
			assert.False(t, ok, "a name the enum does not declare refuses")
			_, ok = r.LiteralFor(nil, mode, "15", f.view)
			assert.False(t, ok, "and so does a number, which a default names by its value's name")
		})

		t.Run("refuses a type no literal is a value of", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			_, ok := r.LiteralFor(nil, ref(svcPkg, "Row", symbol.KindStruct), "1", f.view)
			assert.False(t, ok, "a message")
			_, ok = r.LiteralFor(nil, composite("repeated int64", symbol.FormList, builtin("int64")), "1", f.view)
			assert.False(t, ok, "and a repeated field")
			v, ok := r.LiteralFor(nil, composite("optional int64", symbol.FormOptional, builtin("int64")), "1", f.view)
			assert.True(t, ok && v.Bits == 64, "an optional field types as its element")
		})
	})
}
