// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the well-known types the cases name.
const (
	timestampType = "google.protobuf.Timestamp"
	durationType  = "google.protobuf.Duration"
	emptyType     = "google.protobuf.Empty"
	fieldMaskType = "google.protobuf.FieldMask"
	listValueType = "google.protobuf.ListValue"
	structType    = "google.protobuf.Struct"
	valueType     = "google.protobuf.Value"
)

// elementsAllocs is a well-known type over element shapes: the list of
// its element shapes.
const elementsAllocs = 1

// Builtin classifies what the resolution step left without a target.
// Every scalar of the wire table, every well-known type and the fold
// of the frontend's label forms are pinned.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		scalars := []struct {
			name string
			give string
			want rules.TypeShape
		}{
			{
				name: "classifies int32 as a signed scalar of 32 bits",
				give: "int32", want: rules.Scalar("int32", rules.ScalarInt, 32),
			},
			{
				name: "classifies sint32 as a signed scalar of 32 bits",
				give: "sint32", want: rules.Scalar("sint32", rules.ScalarInt, 32),
			},
			{
				name: "classifies sfixed32 as a signed scalar of 32 bits",
				give: "sfixed32", want: rules.Scalar("sfixed32", rules.ScalarInt, 32),
			},
			{
				name: "classifies int64 as a signed scalar of 64 bits",
				give: "int64", want: rules.Scalar("int64", rules.ScalarInt, 64),
			},
			{
				name: "classifies sint64 as a signed scalar of 64 bits",
				give: "sint64", want: rules.Scalar("sint64", rules.ScalarInt, 64),
			},
			{
				name: "classifies sfixed64 as a signed scalar of 64 bits",
				give: "sfixed64", want: rules.Scalar("sfixed64", rules.ScalarInt, 64),
			},
			{
				name: "classifies uint32 as an unsigned scalar of 32 bits",
				give: "uint32", want: rules.Scalar("uint32", rules.ScalarUint, 32),
			},
			{
				name: "classifies fixed32 as an unsigned scalar of 32 bits",
				give: "fixed32", want: rules.Scalar("fixed32", rules.ScalarUint, 32),
			},
			{
				name: "classifies uint64 as an unsigned scalar of 64 bits",
				give: "uint64", want: rules.Scalar("uint64", rules.ScalarUint, 64),
			},
			{
				name: "classifies fixed64 as an unsigned scalar of 64 bits",
				give: "fixed64", want: rules.Scalar("fixed64", rules.ScalarUint, 64),
			},
			{
				name: "classifies float as a float of 32 bits",
				give: "float", want: rules.Scalar("float", rules.ScalarFloat, 32),
			},
			{
				name: "classifies double as a float of 64 bits",
				give: "double", want: rules.Scalar("double", rules.ScalarFloat, 64),
			},
			{name: "classifies bool as a boolean leaf", give: "bool", want: rules.Leaf(symbol.FormBool, "bool")},
			{name: "classifies string as a text leaf", give: "string", want: rules.Leaf(symbol.FormText, "string")},
			{name: "classifies bytes as the byte string", give: "bytes", want: rules.Leaf(symbol.FormBytes, "bytes")},
		}
		for _, tt := range scalars {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, protorules.New().Builtin(builtin(tt.give), rules.View{}), tt.want, "the shape")
			})
		}

		t.Run("classifies only the scalars of the frontend's grammar", func(t *testing.T) {
			t.Parallel()

			for _, tt := range scalars {
				assert.True(t, protobuf.IsScalar(tt.give), tt.give+" is a scalar the frontend leaves unresolved")
			}
		})

		t.Run("classifies a message no schema declares as opaque", func(t *testing.T) {
			t.Parallel()

			ghost := builtin("some.Message")
			assert.Equal(t, protorules.New().Builtin(ghost, rules.View{}), rules.Opaque(ghost),
				"the reference keeps its spelling for a consumer to read")
		})

		t.Run("classifies a missing reference as opaque", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(nil, rules.View{}), rules.Opaque(nil), "nothing is opaque")
		})

		t.Run("maps Timestamp onto the well-known timestamp", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(timestampType), rules.View{}),
				rules.Reference(timestampType, rules.WellKnownTimestamp),
				"a proto Timestamp and a Go time.Time project as one shape")
		})

		t.Run("maps Duration onto the well-known duration", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(durationType), rules.View{}),
				rules.Reference(durationType, rules.WellKnownDuration), "the registry's duration")
		})

		t.Run("maps a well-known type spelled with its leading dot", func(t *testing.T) {
			t.Parallel()

			rooted := protobuf.NameSep + timestampType
			assert.Equal(t, protorules.New().Builtin(builtin(rooted), rules.View{}),
				rules.Reference(rooted, rules.WellKnownTimestamp), "the spelling as written")
		})

		wrappers := []struct {
			name string
			give string
			want rules.TypeShape
		}{
			{
				name: "classifies DoubleValue as an optional float of 64 bits",
				give: "DoubleValue", want: rules.Scalar("double", rules.ScalarFloat, 64),
			},
			{
				name: "classifies FloatValue as an optional float of 32 bits",
				give: "FloatValue", want: rules.Scalar("float", rules.ScalarFloat, 32),
			},
			{
				name: "classifies Int64Value as an optional signed scalar of 64 bits",
				give: "Int64Value", want: rules.Scalar("int64", rules.ScalarInt, 64),
			},
			{
				name: "classifies UInt64Value as an optional unsigned scalar of 64 bits",
				give: "UInt64Value", want: rules.Scalar("uint64", rules.ScalarUint, 64),
			},
			{
				name: "classifies Int32Value as an optional signed scalar of 32 bits",
				give: "Int32Value", want: rules.Scalar("int32", rules.ScalarInt, 32),
			},
			{
				name: "classifies UInt32Value as an optional unsigned scalar of 32 bits",
				give: "UInt32Value", want: rules.Scalar("uint32", rules.ScalarUint, 32),
			},
			{
				name: "classifies BoolValue as an optional boolean leaf",
				give: "BoolValue", want: rules.Leaf(symbol.FormBool, "bool"),
			},
			{
				name: "classifies StringValue as an optional text leaf",
				give: "StringValue", want: rules.Leaf(symbol.FormText, "string"),
			},
			{
				name: "classifies BytesValue as an optional byte string",
				give: "BytesValue", want: rules.Leaf(symbol.FormBytes, "bytes"),
			},
		}
		for _, tt := range wrappers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				spelling := protobuf.WellKnownPackage + protobuf.NameSep + tt.give
				assert.Equal(t, protorules.New().Builtin(builtin(spelling), rules.View{}),
					rules.TypeShape{Form: symbol.FormOptional, Spelling: spelling, Elems: []rules.TypeShape{tt.want}},
					"the scalar under the presence the wrapper exists to give")
			})
		}

		t.Run("maps Empty onto the well-known empty value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(emptyType), rules.View{}),
				rules.Reference(emptyType, rules.WellKnownEmpty), "the registry's value without data")
		})

		t.Run("classifies FieldMask as a list of strings", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(fieldMaskType), rules.View{}),
				rules.TypeShape{
					Form: symbol.FormList, Spelling: fieldMaskType,
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormText, "string")},
				}, "the list of paths the mask contains")
		})

		t.Run("classifies ListValue as a list of the top type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(listValueType), rules.View{}),
				rules.TypeShape{
					Form: symbol.FormList, Spelling: listValueType,
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormDynamic, valueType)},
				}, "each element decided at run time")
		})

		t.Run("classifies Struct as a map from strings onto the top type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protorules.New().Builtin(builtin(structType), rules.View{}),
				rules.TypeShape{
					Form: symbol.FormMap, Spelling: structType,
					Elems: []rules.TypeShape{
						rules.Leaf(symbol.FormText, "string"), rules.Leaf(symbol.FormDynamic, valueType),
					},
				}, "each value decided at run time")
		})

		dynamic := []struct {
			name string
			give string
		}{
			{name: "classifies Any as the top type", give: "google.protobuf.Any"},
			{name: "classifies Value as the top type", give: valueType},
		}
		for _, tt := range dynamic {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, protorules.New().Builtin(builtin(tt.give), rules.View{}),
					rules.Leaf(symbol.FormDynamic, tt.give), "the content is decided at run time")
			})
		}

		opaque := []struct {
			name string
			give string
		}{
			{name: "classifies NullValue as opaque", give: "google.protobuf.NullValue"},
			{name: "classifies SourceContext as opaque", give: "google.protobuf.SourceContext"},
		}
		for _, tt := range opaque {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := builtin(tt.give)
				assert.Equal(t, protorules.New().Builtin(ref, rules.View{}), rules.Opaque(ref),
					"the projection keeps the spelling for a consumer to read")
			})
		}

		t.Run("folds a message the workspace declares to a reference through the kernel", func(t *testing.T) {
			t.Parallel()

			target := ref(depPkg, "Target", symbol.KindStruct)
			assert.Equal(t, loaded(t).bound().TypeOf(target).Form, symbol.FormReference, "the declaration decides")
		})

		t.Run("folds a repeated bytes field to a list of byte strings through the kernel", func(t *testing.T) {
			t.Parallel()

			list := loaded(t).bound().TypeOf(composite("repeated bytes", symbol.FormList, builtin("bytes")))
			assert.Equal(t, list.Form, symbol.FormList, "a list, not one byte string")
			assert.Equal(t, list.Elems[0].Form, symbol.FormBytes, "of byte strings")
		})

		t.Run("folds the frontend's repeated field to a list of its element", func(t *testing.T) {
			t.Parallel()

			tags := rowField(t, "tags")
			assert.Equal(t, tags.Form, symbol.FormList, "a list")
			assert.Equal(t, tags.Elems[0].Form, symbol.FormText, "of strings")
		})

		t.Run("folds the frontend's map field to a map onto its value", func(t *testing.T) {
			t.Parallel()

			counts := rowField(t, "counts")
			assert.Equal(t, counts.Form, symbol.FormMap, "a map")
			assert.Equal(t, counts.Elems[1].Class, rules.ScalarInt, "onto signed integers")
		})

		t.Run("folds the frontend's reference into a sibling namespace to the declared message", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rowField(t, "target").Ref, id(depPkg, "Target", symbol.KindStruct),
				"the message the other file declares")
		})
	})
}

// A scalar's shape allocates nothing, and a wrapper's the list of its
// element shapes. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestBuiltinAllocs(t *testing.T) {
	checkAllocs(t, builtinCalls())
}

// BenchmarkBuiltin measures the classification the kernel makes for
// every reference the resolution step left without a target.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls())
}

// builtinCalls returns a call of Builtin over a scalar and over a
// wrapper.
func builtinCalls() []allocCall {
	r := protorules.New()
	scalar, wrapper := builtin("int32"), builtin("google.protobuf.Int32Value")
	var shape rules.TypeShape
	return []allocCall{
		{
			name: "Builtin", caseName: "a scalar",
			call:  func() { shape = r.Builtin(scalar, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, shape.Bits, 32, "Builtin classifies int32") },
		},
		{
			name: "Builtin", caseName: "a wrapper", allocs: elementsAllocs,
			call: func() { shape = r.Builtin(wrapper, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormOptional, "Builtin classifies Int32Value")
			},
		},
	}
}

// rowField returns the kernel's fold of one field of the fixture's Row.
func rowField(tb assert.TB, name string) rules.TypeShape {
	tb.Helper()

	f := loaded(tb)
	row, is := f.decl(tb, id(svcPkg, "Row", symbol.KindStruct)).(*node.Struct)
	assert.True(tb, is, "Row is a message")
	for _, field := range row.Fields {
		if field.Name == name {
			return f.bound().TypeOf(field.Type)
		}
	}
	tb.Fatalf("Row declares no field %s", name)
	return rules.TypeShape{}
}
