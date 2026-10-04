// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The imports the classification cases name: the standard library's
// time package, and a workspace package whose last element is time
// too.
const (
	timePath    = "time"
	foreignTime = "example.test/time"
	foreignPkg  = "example.test/p"
)

// Builtin classifies what the resolution step left without a target.
// Every spelling it maps, and every type it reads through an import's
// package, is pinned.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			give  string
			form  symbol.TypeForm
			class rules.ScalarClass
			bits  int
		}{
			{
				name: "classifies int as a signed scalar of the platform width",
				give: "int", form: symbol.FormScalar, class: rules.ScalarInt,
			},
			{
				name: "classifies int8 as a signed scalar of 8 bits",
				give: "int8", form: symbol.FormScalar, class: rules.ScalarInt, bits: 8,
			},
			{
				name: "classifies int16 as a signed scalar of 16 bits",
				give: "int16", form: symbol.FormScalar, class: rules.ScalarInt, bits: 16,
			},
			{
				name: "classifies int32 as a signed scalar of 32 bits",
				give: "int32", form: symbol.FormScalar, class: rules.ScalarInt, bits: 32,
			},
			{
				name: "classifies rune as a signed scalar of 32 bits",
				give: "rune", form: symbol.FormScalar, class: rules.ScalarInt, bits: 32,
			},
			{
				name: "classifies int64 as a signed scalar of 64 bits",
				give: "int64", form: symbol.FormScalar, class: rules.ScalarInt, bits: 64,
			},
			{
				name: "classifies uint as an unsigned scalar of the platform width",
				give: "uint", form: symbol.FormScalar, class: rules.ScalarUint,
			},
			{
				name: "classifies uintptr as an unsigned scalar of the platform width",
				give: "uintptr", form: symbol.FormScalar, class: rules.ScalarUint,
			},
			{
				name: "classifies uint8 as an unsigned scalar of 8 bits",
				give: "uint8", form: symbol.FormScalar, class: rules.ScalarUint, bits: 8,
			},
			{
				name: "classifies byte as an unsigned scalar of 8 bits",
				give: "byte", form: symbol.FormScalar, class: rules.ScalarUint, bits: 8,
			},
			{
				name: "classifies uint16 as an unsigned scalar of 16 bits",
				give: "uint16", form: symbol.FormScalar, class: rules.ScalarUint, bits: 16,
			},
			{
				name: "classifies uint32 as an unsigned scalar of 32 bits",
				give: "uint32", form: symbol.FormScalar, class: rules.ScalarUint, bits: 32,
			},
			{
				name: "classifies uint64 as an unsigned scalar of 64 bits",
				give: "uint64", form: symbol.FormScalar, class: rules.ScalarUint, bits: 64,
			},
			{
				name: "classifies float32 as a float of 32 bits",
				give: "float32", form: symbol.FormScalar, class: rules.ScalarFloat, bits: 32,
			},
			{
				name: "classifies float64 as a float of 64 bits",
				give: "float64", form: symbol.FormScalar, class: rules.ScalarFloat, bits: 64,
			},
			{name: "classifies bool as a boolean leaf", give: "bool", form: symbol.FormBool},
			{name: "classifies string as a text leaf", give: "string", form: symbol.FormText},
			{name: "classifies any as opaque", give: "any", form: symbol.FormOpaque},
			{name: "classifies error as opaque", give: "error", form: symbol.FormOpaque},
			{name: "classifies comparable as opaque", give: "comparable", form: symbol.FormOpaque},
			{name: "classifies complex128 as opaque", give: "complex128", form: symbol.FormOpaque},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := gorules.New().Builtin(builtin(tt.give), rules.View{})
				assert.Equal(t, s.Form, tt.form, "the form")
				assert.Equal(t, s.Class, tt.class, "the class")
				assert.Equal(t, s.Bits, tt.bits, "the width")
				assert.Equal(t, s.Spelling, tt.give, "the spelling as written")
			})
		}

		t.Run("classifies a missing reference as opaque", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().Builtin(nil, rules.View{}).Form, symbol.FormOpaque, "nothing is opaque")
		})

		t.Run("maps time.Time onto the well-known timestamp", func(t *testing.T) {
			t.Parallel()

			when := gorules.New().Builtin(builtin("time.Time"), rules.View{})
			assert.Equal(t, when.Form, symbol.FormReference, "time.Time is a reference")
			assert.Equal(t, when.Ref, rules.WellKnownTimestamp, "to the timestamp")
		})

		t.Run("maps time.Duration onto the well-known duration", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().Builtin(builtin("time.Duration"), rules.View{}).Ref,
				rules.WellKnownDuration, "the duration")
		})

		t.Run("maps a Time through an aliased import onto the well-known timestamp", func(t *testing.T) {
			t.Parallel()

			aliased := &node.TypeRef{Spelling: "tm.Time", Package: timePath}
			assert.Equal(t, gorules.New().Builtin(aliased, rules.View{}).Ref, rules.WellKnownTimestamp,
				"the package the import names decides, not the qualifier")
		})

		t.Run("maps a witness's bare Duration of the time package onto the well-known duration", func(t *testing.T) {
			t.Parallel()

			witness := &node.TypeRef{Spelling: "Duration", Package: timePath}
			assert.Equal(t, gorules.New().Builtin(witness, rules.View{}).Ref, rules.WellKnownDuration,
				"a witness spells its bare name beside the package")
		})

		t.Run("classifies a Time another package named time declares as opaque", func(t *testing.T) {
			t.Parallel()

			foreign := &node.TypeRef{Spelling: "time.Time", Package: foreignTime}
			assert.Equal(t, gorules.New().Builtin(foreign, rules.View{}).Form, symbol.FormOpaque,
				"the qualifier matches, and the import path does not")
		})

		t.Run("classifies a qualified predeclared name as opaque", func(t *testing.T) {
			t.Parallel()

			qualified := &node.TypeRef{Spelling: "p.int", Package: foreignPkg}
			assert.Equal(t, gorules.New().Builtin(qualified, rules.View{}).Form, symbol.FormOpaque,
				"a package's own type named int is no builtin")
		})

		t.Run("folds a byte slice to bytes through the kernel", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s := f.bound().TypeOf(composite("[]byte", symbol.FormList, builtin("byte")))
			assert.Equal(t, s.Form, symbol.FormBytes, "the one rule beyond structure applies to Go's spelling")
		})
	})
}

// A classification reads the reference and allocates nothing. The
// ordinary run, which runs no benchmark, checks that here.
func TestBuiltinZeroAlloc(t *testing.T) {
	checkAllocs(t, builtinCalls())
}

// BenchmarkBuiltin measures the classification the kernel makes for
// every reference the resolution step left without a target.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls())
}

// builtinCalls returns a call of Builtin over a scalar and over a
// well-known type.
func builtinCalls() []allocCall {
	r := gorules.New()
	scalar, when := builtin("int"), builtin("time.Time")
	var shape rules.TypeShape
	return []allocCall{
		{
			name: "Builtin", caseName: "a scalar",
			call:  func() { shape = r.Builtin(scalar, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, shape.Class, rules.ScalarInt, "Builtin classifies int") },
		},
		{
			name: "Builtin", caseName: "a well-known type",
			call: func() { shape = r.Builtin(when, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Ref, rules.WellKnownTimestamp, "Builtin maps time.Time")
			},
		},
	}
}
