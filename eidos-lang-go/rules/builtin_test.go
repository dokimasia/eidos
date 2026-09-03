// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("classifies every builtin spelling", func(t *testing.T) {
		t.Parallel()

		r := gorules.New()
		cases := []struct {
			spelling string
			form     symbol.TypeForm
			class    rules.ScalarClass
			bits     int
		}{
			{"int", symbol.FormScalar, rules.ScalarInt, 0},
			{"int8", symbol.FormScalar, rules.ScalarInt, 8},
			{"int16", symbol.FormScalar, rules.ScalarInt, 16},
			{"int32", symbol.FormScalar, rules.ScalarInt, 32},
			{"rune", symbol.FormScalar, rules.ScalarInt, 32},
			{"int64", symbol.FormScalar, rules.ScalarInt, 64},
			{"uint", symbol.FormScalar, rules.ScalarUint, 0},
			{"uintptr", symbol.FormScalar, rules.ScalarUint, 0},
			{"uint8", symbol.FormScalar, rules.ScalarUint, 8},
			{"byte", symbol.FormScalar, rules.ScalarUint, 8},
			{"uint16", symbol.FormScalar, rules.ScalarUint, 16},
			{"uint32", symbol.FormScalar, rules.ScalarUint, 32},
			{"uint64", symbol.FormScalar, rules.ScalarUint, 64},
			{"float32", symbol.FormScalar, rules.ScalarFloat, 32},
			{"float64", symbol.FormScalar, rules.ScalarFloat, 64},
			{"bool", symbol.FormBool, 0, 0},
			{"string", symbol.FormText, 0, 0},
			{"any", symbol.FormOpaque, 0, 0},
			{"error", symbol.FormOpaque, 0, 0},
			{"comparable", symbol.FormOpaque, 0, 0},
			{"complex128", symbol.FormOpaque, 0, 0},
		}
		for _, tc := range cases {
			s := r.Builtin(builtin(tc.spelling), rules.View{})
			assert.Equal(t, s.Form, tc.form, tc.spelling+" folds to its form")
			assert.Equal(t, s.Class, tc.class, tc.spelling+" with its class")
			assert.Equal(t, s.Bits, tc.bits, tc.spelling+" and its width")
			assert.Equal(t, s.Spelling, tc.spelling, "keeping the spelling")
		}
		assert.Equal(t, r.Builtin(nil, rules.View{}).Form, symbol.FormOpaque, "nothing is opaque")
	})

	t.Run("maps the two standard types onto the well-known registry", func(t *testing.T) {
		t.Parallel()

		r := gorules.New()
		when := r.Builtin(builtin("time.Time"), rules.View{})
		assert.Equal(t, when.Form, symbol.FormReference, "time.Time is a reference")
		assert.Equal(t, when.Ref, rules.WellKnownTimestamp, "to the timestamp")
		dur := r.Builtin(builtin("time.Duration"), rules.View{})
		assert.Equal(t, dur.Ref, rules.WellKnownDuration, "and time.Duration to the duration")
	})

	t.Run("folds a byte slice to bytes through the kernel", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		s := f.bound().TypeOf(composite("[]byte", symbol.FormList, builtin("byte")))
		assert.Equal(
			t,
			s.Form,
			symbol.FormBytes,
			"the one rule beyond structure applies to Go's spelling",
		)
	})
}
