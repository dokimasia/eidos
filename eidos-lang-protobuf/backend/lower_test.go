// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// lowerAllocs is the lowering of a message that protobuf writes as it is.
const lowerAllocs = 0

// shapeName is the name of the sum that the lowering cases rewrite into a
// message.
const shapeName = "shape"

// TestLower pins the lowering of each kind: what it keeps, what it
// rewrites into the shapes of protobuf, and what it refuses.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "keeps a function, whose kind the render refuses",
				give: &emit.Function{Name: "boot"},
			},
			{
				name: "keeps a generic message, which the coverage rejects",
				give: &emit.Struct{
					Name: "box", TypeParams: []*emit.TypeParam{{Name: "T"}},
					Fields: emit.SlotOf(&emit.Field{Name: "item", Type: &emit.TypeRef{Spelling: "T"}}),
				},
			},
			{
				name: "keeps a generic sum, which the coverage rejects",
				give: &emit.Sum{Name: "pack", TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
			{
				name: "keeps a generic service, which the coverage rejects",
				give: &emit.Interface{
					Name: "keyed", TypeParams: []*emit.TypeParam{{Name: "K"}},
					Fields: emit.SlotOf(&emit.Field{Name: idField}),
				},
			},
			{
				name: "keeps a message whose members have numbers",
				give: &emit.Struct{
					Name: rowName,
					Fields: emit.SlotOf(
						&emit.Field{Name: nameField, Type: &emit.TypeRef{Spelling: stringType}, Tags: tag1},
						&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag2},
					),
					Types: emit.SlotOf[symbol.Symbol](
						&emit.Struct{Name: "Inner", Fields: emit.SlotOf(
							&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
						)},
						&emit.Enum{Name: "Kind", Variants: emit.SlotOf(&emit.EnumVariant{Name: "plain", Value: "0"})},
						&emit.Sum{Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
							Name: "circle", Fields: emit.SlotOf(
								&emit.Field{Name: "circle", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag3},
							),
						})},
						&emit.Field{
							Name: extName, Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1,
							Receives: &emit.TypeRef{Spelling: otherName},
						},
					),
				},
			},
			{
				name: "keeps an extension at file level",
				give: &emit.Field{
					Name: extName, Type: &emit.TypeRef{Spelling: stringType}, Tags: tag100,
					Receives: &emit.TypeRef{Spelling: otherName},
				},
			},
			{
				name: "keeps an enum of values in decimal, in hexadecimal and in octal",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
					&emit.EnumVariant{Name: "open", Value: "0"},
					&emit.EnumVariant{Name: "shut", Value: "0x1"},
					&emit.EnumVariant{Name: "held", Value: "02"},
					&emit.EnumVariant{Name: "lost", Value: "-2147483648"},
					&emit.EnumVariant{Name: "last", Value: "2147483647"},
				)},
			},
			{
				name: "keeps an enum whose values share a number under allow_alias",
				give: &emit.Enum{
					Name:        phaseName,
					Annotations: symbol.Annotations{{Name: "allow_alias", Args: []string{trueValue}}},
					Variants: emit.SlotOf(
						&emit.EnumVariant{Name: "open", Value: "0"},
						&emit.EnumVariant{Name: "started", Value: "0"},
					),
				},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(tt.give)
				assert.NoError(t, err, "protobuf writes the declaration")
				assert.Nil(t, out, "the lowering keeps the declaration as it is")
			})
		}

		t.Run("returns a message with one oneof for a sum at file level", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{
				Lang:    "golang",
				Package: "example.com/shapes",
				Name:    shapeName,
				Kind:    symbol.KindSum,
			}
			annotations := symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}}
			radius := &emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1}
			circle := &emit.SumVariant{Name: "circle", Fields: emit.SlotOf(radius)}
			empty := &emit.SumVariant{
				Name:        "empty",
				Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"2"}}},
			}
			out, err := backend.Lower(&emit.Sum{
				Origin: origin, Doc: []string{"shape is one figure."}, Comment: "tagged", Name: shapeName,
				Annotations: annotations, Variants: emit.SlotOf(circle, empty),
			})
			assert.NoError(t, err, "protobuf writes the sum")
			assert.Equal(t, out, []symbol.Symbol{&emit.Struct{
				Origin: origin, Doc: []string{"shape is one figure."}, Comment: "tagged", Name: shapeName,
				Annotations: annotations,
				Types: emit.SlotOf[symbol.Symbol](&emit.Sum{Name: shapeName, Variants: emit.SlotOf(
					&emit.SumVariant{Name: "circle", Fields: emit.SlotOf(radius)},
					&emit.SumVariant{Name: "empty", Fields: emit.SlotOf(&emit.Field{
						Name: "empty", Type: &emit.TypeRef{Spelling: protobuf.TypeEmpty, Package: protobuf.EmptyFile},
						Tags: tag2,
					})},
				)}),
			}}, "the message of the sum, whose variant without a field is a member of google.protobuf.Empty",
				assert.EquateEmpty())

			again, err := backend.Lower(out[0])
			assert.NoError(t, err, "protobuf writes the lowered message")
			assert.Nil(t, again, "a second lowering changes nothing")
		})

		t.Run("returns a variant of two fields as a member of a message of the variant", func(t *testing.T) {
			t.Parallel()

			row := &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
				Name: shapeName,
				Variants: emit.SlotOf(&emit.SumVariant{
					Name:        "point",
					Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"3"}}},
					Fields: emit.SlotOf(
						&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: int32Type}},
						&emit.Field{Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag3},
					),
				}),
			})}
			out, err := backend.Lower(row)
			assert.NoError(t, err, "protobuf writes the variant")
			assert.Nil(t, out, "the lowering rewrites the message in place")
			assert.Equal(t, row.Types.Items(), []symbol.Symbol{
				&emit.Sum{Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
					Name:   "point",
					Fields: emit.SlotOf(&emit.Field{Name: "point", Type: &emit.TypeRef{Spelling: "Point"}, Tags: tag3}),
				})},
				&emit.Struct{Name: "Point", Fields: emit.SlotOf(
					&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
					&emit.Field{Name: "field_2", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag3},
				)},
			}, "the member of the variant and its message, whose fields keep their numbers or take their positions",
				assert.EquateEmpty())
		})

		t.Run("returns the member of an optional as the type of the optional", func(t *testing.T) {
			t.Parallel()

			member := &emit.Field{
				Name: "row",
				Tags: tag1,
				Type: &emit.TypeRef{
					Spelling: "optional Row",
					Form:     symbol.FormOptional,
					Elems:    []*emit.TypeRef{{Spelling: "Row"}},
				},
			}
			_, err := backend.Lower(&emit.Struct{Name: "holder", Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
				Name: "choice", Variants: emit.SlotOf(&emit.SumVariant{Name: "row", Fields: emit.SlotOf(member)}),
			})})
			assert.NoError(t, err, "protobuf writes the member")
			assert.Equal(t, member.Type, &emit.TypeRef{Spelling: "Row"}, "a member has presence through its oneof")
		})

		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for a field of a message without a number",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: nameField})},
				want: "protobuf: a field of a message takes a number, and name states none",
			},
			{
				name: "returns an error for a field with two tag entries",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(
						&emit.TagEntry{
							Key:   protobuf.NumberTag,
							Value: "1",
						},
						&emit.TagEntry{Key: "json", Value: nameField},
					),
				})},
				want: "protobuf: a field takes one tag entry, and name states 2",
			},
			{
				name: "returns an error for a field with the tag entry of another key",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: "json", Value: nameField}),
				})},
				want: "protobuf: a field takes the tag entry number, and name states the entry json",
			},
			{
				name: "returns an error for a number that is not decimal",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "0x1"}),
				})},
				want: `protobuf: a wire number is a decimal number, and name states "0x1"`,
			},
			{
				name: "returns an error for the number 0",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "0"}),
				})},
				want: "protobuf: a wire number is from 1 to 536870911, and name states 0",
			},
			{
				name: "returns an error for a number above 536870911",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "536870912"}),
				})},
				want: "protobuf: a wire number is from 1 to 536870911, and name states 536870912",
			},
			{
				name: "returns an error for the first number that protobuf reserves",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "19000"}),
				})},
				want: "protobuf: protobuf reserves the wire numbers 19000 to 19999, and name states 19000",
			},
			{
				name: "returns an error for the last number that protobuf reserves",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "19999"}),
				})},
				want: "protobuf: protobuf reserves the wire numbers 19000 to 19999, and name states 19999",
			},
			{
				name: "returns an error for two fields with one number",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(
					&emit.Field{Name: nameField, Tags: tag1}, &emit.Field{Name: idField, Tags: tag1},
				)},
				want: "protobuf: name and id of row take the wire number 1",
			},
			{
				name: "returns an error for a field and a member of a oneof with one number",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: nameField, Tags: tag1}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Sum{Name: shapeName, Variants: emit.SlotOf(
						&emit.SumVariant{Name: "circle", Fields: emit.SlotOf(&emit.Field{Name: "circle", Tags: tag1})},
					)}),
				},
				want: "protobuf: name and circle of row take the wire number 1",
			},
			{
				name: "returns an error for a field and a nested message of one name",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: nameField, Tags: tag1}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: nameField}),
				},
				want: "protobuf: row declares two members named name",
			},
			{
				name: "returns an error for a oneof of the name of a field",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: shapeName, Tags: tag1}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Sum{Name: shapeName, Variants: emit.SlotOf(
						&emit.SumVariant{Name: "circle", Fields: emit.SlotOf(&emit.Field{Name: "circle", Tags: tag2})},
					)}),
				},
				want: "protobuf: row declares two members named shape",
			},
			{
				name: "returns an error for a nested enum of the name of a field",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: phaseName, Tags: tag1}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "0"}),
					}),
				},
				want: "protobuf: row declares two members named phase",
			},
			{
				name: "returns an error for an extension of the name of a field",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: extName, Tags: tag1}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Field{
						Name: extName, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
					}),
				},
				want: "protobuf: row declares two members named ext",
			},
			{
				name: "returns an error for a read-only field",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Tags: tag1, Mutability: symbol.MutabilityImmutable,
				})},
				want: "protobuf: a message has no read-only field, and name is read-only",
			},
			{
				name: "returns an error for an extension among the fields of a message",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: extName, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
				})},
				want: "protobuf: an extension is a nested declaration of a message, and row lists ext among its fields",
			},
			{
				name: "returns an error for a nested field that extends no message",
				give: &emit.Struct{
					Name:  rowName,
					Types: emit.SlotOf[symbol.Symbol](&emit.Field{Name: nameField, Tags: tag1}),
				},
				want: "protobuf: a field of a message is one of its fields, and row nests name among its nested declarations",
			},
			{
				name: "returns an error for a nested service",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Interface{Name: storeName})},
				want: "protobuf: a message nests messages, enums, oneofs and extensions, " +
					"and row nests a declaration of the kind Interface",
			},
			{
				name: "returns an error for a field of a nested message without a number",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Struct{
					Name: "Inner", Fields: emit.SlotOf(&emit.Field{Name: idField}),
				})},
				want: "protobuf: a field of a message takes a number, and id states none",
			},
			{
				name: "returns an error for a nested enum without a value",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Enum{Name: phaseName})},
				want: "protobuf: an enum declares at least one value, and phase declares none",
			},
			{
				name: "returns an error for a nested extension without a number",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Field{
					Name: extName, Receives: &emit.TypeRef{Spelling: otherName},
				})},
				want: "protobuf: a field of a message takes a number, and ext states none",
			},
			{
				name: "returns an error for a field at file level that extends no message",
				give: &emit.Field{Name: "count", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
				want: "protobuf: a file declares no field of its own, and count extends no message",
			},
			{
				name: "returns an error for an extension at file level without a number",
				give: &emit.Field{Name: extName, Receives: &emit.TypeRef{Spelling: otherName}},
				want: "protobuf: a field of a message takes a number, and ext states none",
			},
			{
				name: "returns an error for a sum at file level of a variant without a number",
				give: &emit.Sum{Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{Name: "empty"})},
				want: "protobuf: a variant without exactly one field takes the marker protobuf:number, and empty states none",
			},
			{
				name: "returns an error for a oneof without a member",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{Name: shapeName})},
				want: "protobuf: a oneof declares at least one member, and shape declares none",
			},
			{
				name: "returns an error for a marker on a oneof",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Annotations: symbol.Annotations{{Name: string(protobuf.LocalMarker)}},
				})},
				want: "protobuf: a oneof does not take the marker protobuf:local, and shape states it",
			},
			{
				name: "returns an error for a marker of an enum on a variant",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
						Name: "empty", Annotations: symbol.Annotations{{Name: string(protobuf.ClosedMarker)}},
					}),
				})},
				want: "protobuf: a variant does not take the marker protobuf:closed, and empty states it",
			},
			{
				name: "returns an error for a variant of one field with a number marker",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
						Name:        "circle",
						Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"2"}}},
						Fields:      emit.SlotOf(&emit.Field{Name: "radius", Tags: tag1}),
					}),
				})},
				want: "protobuf: the marker protobuf:number numbers a variant without exactly one field, and circle has one",
			},
			{
				name: "returns an error for a number marker that is not a wire number",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
						Name:        "empty",
						Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"x"}}},
					}),
				})},
				want: `protobuf: a wire number is a decimal number, and empty states "x"`,
			},
			{
				name: "returns an error for a field of a variant message with the tag entry of another key",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
						Name:        "point",
						Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"2"}}},
						Fields: emit.SlotOf(
							&emit.Field{Name: "x", Tags: emit.SlotOf(&emit.TagEntry{Key: "json", Value: "x"})},
							&emit.Field{Name: "y"},
						),
					}),
				})},
				want: "protobuf: a field takes the tag entry number, and x states the entry json",
			},
			{
				name: "returns an error for a member without a number",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
					Name: shapeName, Variants: emit.SlotOf(&emit.SumVariant{
						Name: "circle", Fields: emit.SlotOf(&emit.Field{Name: "radius"}),
					}),
				})},
				want: "protobuf: a field of a message takes a number, and circle states none",
			},
			{
				name: "returns an error for an enum without a value",
				give: &emit.Enum{Name: phaseName},
				want: "protobuf: an enum declares at least one value, and phase declares none",
			},
			{
				name: "returns an error for an enum value without a number",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "open"})},
				want: "protobuf: an enum value takes a number, and open states none",
			},
			{
				name: "returns an error for an enum value that is not a number",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "one"})},
				want: "protobuf: an enum value is a number in decimal, hexadecimal or octal, and open states one",
			},
			{
				name: "returns an error for an octal enum value with the digit 8",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "08"})},
				want: "protobuf: an enum value is a number in decimal, hexadecimal or octal, and open states 08",
			},
			{
				name: "returns an error for an enum value with a plus sign",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "+1"})},
				want: "protobuf: an enum value is a number in decimal, hexadecimal or octal, and open states +1",
			},
			{
				name: "returns an error for an enum value above the range of an int32",
				give: &emit.Enum{
					Name:     phaseName,
					Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "2147483648"}),
				},
				want: "protobuf: an enum value is an int32, and open states 2147483648",
			},
			{
				name: "returns an error for an enum value below the range of an int32",
				give: &emit.Enum{
					Name:     phaseName,
					Variants: emit.SlotOf(&emit.EnumVariant{Name: "open", Value: "-2147483649"}),
				},
				want: "protobuf: an enum value is an int32, and open states -2147483649",
			},
			{
				name: "returns an error for an enum value beyond 64 bits",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
					&emit.EnumVariant{Name: "open", Value: "0xFFFFFFFFFFFFFFFFF"},
				)},
				want: "protobuf: an enum value is an int32, and open states 0xFFFFFFFFFFFFFFFFF",
			},
			{
				name: "returns an error for two values of one number without allow_alias",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
					&emit.EnumVariant{Name: "open", Value: "0"}, &emit.EnumVariant{Name: "shut", Value: "0x0"},
				)},
				want: "protobuf: open and shut of phase take the value 0, and phase does not state allow_alias = true",
			},
			{
				name: "returns an error for two values of one number under allow_alias false",
				give: &emit.Enum{
					Name:        phaseName,
					Annotations: symbol.Annotations{{Name: "allow_alias", Args: []string{"false"}}},
					Variants: emit.SlotOf(
						&emit.EnumVariant{Name: "open", Value: "1"}, &emit.EnumVariant{Name: "shut", Value: "1"},
					),
				},
				want: "protobuf: open and shut of phase take the value 1, and phase does not state allow_alias = true",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(tt.give)
				assert.HasError(t, err, "protobuf has no form of the construct")
				assert.Equal(t, err.Error(), tt.want, "the error states the construct")
				assert.Nil(t, out, "a refusal returns no declaration")
			})
		}
	})
}

// Lower keeps a message that protobuf writes as it is without allocating.
// The ordinary run, which runs no benchmark, checks that ceiling here.
func TestLowerAllocs(t *testing.T) {
	row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(
		&emit.Field{
			Name: nameField,
			Type: stringRef,
			Tags: tag1,
		},
		&emit.Field{Name: idField, Type: int32Ref, Tags: tag2},
	)}
	var (
		out []symbol.Symbol
		err error
	)
	assert.MaxAllocs(
		t,
		func() { out, err = backend.Lower(row) },
		lowerAllocs,
		"Lower keeps the message without allocating",
	)
	assert.NoError(t, err, "Lower writes the message")
	assert.Nil(t, out, "Lower keeps the message as it is")
}

// BenchmarkLower measures the lowering that the settle runs on every
// declaration of a plan.
func BenchmarkLower(b *testing.B) {
	b.Run("Lower", func(b *testing.B) {
		row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(
			&emit.Field{
				Name: nameField,
				Type: stringRef,
				Tags: tag1,
			},
			&emit.Field{Name: idField, Type: int32Ref, Tags: tag2},
		)}
		c := bench.Start(b).MaxAllocs(lowerAllocs)
		defer c.End()
		var (
			out []symbol.Symbol
			err error
		)
		for c.Loop() {
			out, err = backend.Lower(row)
		}
		assert.NoError(b, err, "Lower writes the message")
		assert.Nil(b, out, "Lower keeps the message as it is")
	})
}
