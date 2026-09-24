// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The well-known types are policy defaults the satellite ships: a
// target rendering a proto schema needs each one's canonical shape,
// not a message the workspace may not declare.
func TestWellKnown(t *testing.T) {
	t.Parallel()

	r := protorules.New()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		t.Run("maps the two the kernel registry blesses", func(t *testing.T) {
			t.Parallel()

			when := r.Builtin(builtin("google.protobuf.Timestamp"), rules.View{})
			assert.Equal(t, when.Form, symbol.FormReference, "Timestamp is a reference")
			assert.Equal(t, when.Ref, rules.WellKnownTimestamp,
				"to the timestamp, so a proto Timestamp and a Go time.Time project as one shape")
			dur := r.Builtin(builtin("google.protobuf.Duration"), rules.View{})
			assert.Equal(t, dur.Ref, rules.WellKnownDuration, "and Duration to the duration")
		})

		t.Run("maps a well-known type spelled with its leading dot", func(t *testing.T) {
			t.Parallel()

			when := r.Builtin(builtin(".google.protobuf.Timestamp"), rules.View{})
			assert.Equal(t, when.Ref, rules.WellKnownTimestamp,
				"the rooted spelling names the same type, and the frontend resolves neither")
			assert.Equal(t, when.Spelling, ".google.protobuf.Timestamp", "keeping the spelling as written")
		})

		t.Run("unwraps every wrapper to its scalar under presence", func(t *testing.T) {
			t.Parallel()

			cases := []struct {
				spelling string
				form     symbol.TypeForm
				class    rules.ScalarClass
				bits     int
			}{
				{"DoubleValue", symbol.FormScalar, rules.ScalarFloat, 64},
				{"FloatValue", symbol.FormScalar, rules.ScalarFloat, 32},
				{"Int64Value", symbol.FormScalar, rules.ScalarInt, 64},
				{"UInt64Value", symbol.FormScalar, rules.ScalarUint, 64},
				{"Int32Value", symbol.FormScalar, rules.ScalarInt, 32},
				{"UInt32Value", symbol.FormScalar, rules.ScalarUint, 32},
				{"BoolValue", symbol.FormBool, 0, 0},
				{"StringValue", symbol.FormText, 0, 0},
				{"BytesValue", symbol.FormBytes, 0, 0},
			}
			for _, tc := range cases {
				s := r.Builtin(builtin("google.protobuf."+tc.spelling), rules.View{})
				assert.Equal(t, s.Form, symbol.FormOptional,
					tc.spelling+" is the presence it exists to give")
				assert.Length(t, s.Elems, 1, "over one scalar")
				assert.Equal(t, s.Elems[0].Form, tc.form, tc.spelling+" wraps its own form")
				assert.Equal(t, s.Elems[0].Class, tc.class, "with the wire's class")
				assert.Equal(t, s.Elems[0].Bits, tc.bits, "and its width")
			}
		})

		t.Run("shapes the structural well-known types", func(t *testing.T) {
			t.Parallel()

			empty := r.Builtin(builtin("google.protobuf.Empty"), rules.View{})
			assert.Equal(t, empty.Form, symbol.FormInline, "Empty is the empty record")

			mask := r.Builtin(builtin("google.protobuf.FieldMask"), rules.View{})
			assert.Equal(t, mask.Form, symbol.FormList, "a FieldMask is the list of paths it contains")
			assert.Equal(t, mask.Elems[0].Form, symbol.FormText, "each a string")

			list := r.Builtin(builtin("google.protobuf.ListValue"), rules.View{})
			assert.Equal(t, list.Form, symbol.FormList, "a ListValue is a list")
			assert.Equal(t, list.Elems[0].Form, symbol.FormOpaque, "of values decided at run time")

			st := r.Builtin(builtin("google.protobuf.Struct"), rules.View{})
			assert.Equal(t, st.Form, symbol.FormMap, "a Struct is a map")
			assert.Equal(t, st.Elems[0].Form, symbol.FormText, "keyed by string")
			assert.Equal(t, st.Elems[1].Form, symbol.FormOpaque, "onto values decided at run time")
		})

		t.Run("keeps the dynamic ones opaque", func(t *testing.T) {
			t.Parallel()

			for _, spelling := range []string{
				"google.protobuf.Any", "google.protobuf.Value",
				"google.protobuf.NullValue", "google.protobuf.SourceContext",
			} {
				s := r.Builtin(builtin(spelling), rules.View{})
				assert.Equal(t, s.Form, symbol.FormOpaque, spelling+" is decided at run time")
				assert.Equal(t, s.Spelling, spelling, "so the projection keeps the spelling and claims nothing")
			}
		})
	})
}
