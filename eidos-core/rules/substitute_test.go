// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The spellings and the package of the substitution cases.
const (
	substArg     = "string"
	substPlain   = "number"
	substGeneric = "Box"
	substPkg     = "svc/subst"
)

// substituteAllocs counts the copy of the argument that replaces a
// reference to a parameter.
const substituteAllocs = 1

// substParam is the type parameter of the substitution cases, and
// substUse a reference to it. Substitute never changes a reference that
// it receives, so the cases share both.
var (
	substParam = &node.TypeParam{
		ID:   symbol.Identity{Package: substPkg, Name: paramName, Kind: symbol.KindTypeParam},
		Name: paramName,
	}
	substUse = &node.TypeRef{Spelling: paramName, Target: substParam.ID}
)

// Substitute replaces a reference by its target alone, and copies what
// it rewrites, so the reference that it receives never changes.
func TestSubstitute(t *testing.T) {
	t.Parallel()

	t.Run("Substitute", func(t *testing.T) {
		t.Parallel()

		params := []*node.TypeParam{substParam}
		arg := &node.TypeRef{Spelling: substArg}
		args := []*node.TypeRef{arg}

		t.Run("replaces a reference that targets a parameter with a copy of its argument", func(t *testing.T) {
			t.Parallel()

			got := rules.Substitute(substUse, params, args)
			expect.Equal(t, got.Spelling, substArg, "the copy has the spelling of the argument")
			expect.NotEqual(t, got, arg, "the result is a copy of the argument", assert.ByIdentity())
		})

		t.Run("returns a reference without a target that has the name of a parameter as it is", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: paramName}
			assert.Equal(t, rules.Substitute(ref, params, args), ref,
				"the resolution step targets a parameter in scope, so a reference without a target is no parameter",
				assert.ByIdentity())
		})

		t.Run("returns a reference to another declaration with the name of a parameter as it is", func(t *testing.T) {
			t.Parallel()

			class := symbol.Identity{Package: substPkg, Name: paramName, Kind: symbol.KindStruct}
			ref := &node.TypeRef{Spelling: paramName, Target: class}
			assert.Equal(t, rules.Substitute(ref, params, args), ref, "the struct T is not the parameter T",
				assert.ByIdentity())
		})

		t.Run("restates the children and the arguments that refer to a parameter", func(t *testing.T) {
			t.Parallel()

			list := &node.TypeRef{Spelling: "[]T", Form: symbol.FormList, Elems: []*node.TypeRef{substUse}}
			generic := &node.TypeRef{Spelling: substGeneric, Args: []*node.TypeRef{substUse}}
			gotList := rules.Substitute(list, params, args)
			gotGeneric := rules.Substitute(generic, params, args)
			expect.Equal(t, gotList.Elems[0].Spelling, substArg, "the element is the argument")
			expect.Equal(t, gotGeneric.Args[0].Spelling, substArg, "the type argument is the argument")
			expect.Equal(t, list.Elems[0].Spelling, paramName, "the element of the input stays the parameter")
		})

		t.Run("clears the spelling of a restated structural reference", func(t *testing.T) {
			t.Parallel()

			list := &node.TypeRef{Spelling: "[]T", Form: symbol.FormList, Elems: []*node.TypeRef{substUse}}
			assert.Empty(t, rules.Substitute(list, params, args).Spelling,
				"the written spelling of the list contains the parameter")
		})

		t.Run("keeps the spelling of a restated named reference", func(t *testing.T) {
			t.Parallel()

			generic := &node.TypeRef{Spelling: substGeneric, Args: []*node.TypeRef{substUse}}
			assert.Equal(t, rules.Substitute(generic, params, args).Spelling, substGeneric,
				"the spelling of a named reference is its name")
		})

		t.Run("restates the members of an inline body that refer to a parameter", func(t *testing.T) {
			t.Parallel()

			inline := &node.TypeRef{
				Spelling: "{item:T;get():T}",
				Form:     symbol.FormInline,
				Fields: []*node.Field{
					nil,
					{Name: "item", Type: substUse},
					{Name: "fixed", Type: arg},
				},
				Methods: []*node.Method{nil, {
					Name:    "get",
					Params:  []*node.Param{nil, {Name: "p", Type: substUse}},
					Returns: []*node.Return{nil, {Type: substUse}},
				}, {Name: "plain", Returns: []*node.Return{{Type: arg}}}},
			}
			got := rules.Substitute(inline, params, args)
			expect.Equal(t, got.Fields[1].Type.Spelling, substArg, "the type of the field is the argument")
			expect.Equal(t, got.Fields[2], inline.Fields[2], "a field without a parameter is the same field",
				assert.ByIdentity())
			expect.Equal(t, got.Methods[1].Params[0].Type.Spelling, substArg,
				"the parameter of the method has the type of the argument")
			expect.Equal(t, got.Methods[1].Returns[0].Type.Spelling, substArg,
				"the result of the method has the type of the argument")
			expect.Equal(t, got.Methods[2], inline.Methods[2], "a method without a parameter is the same method",
				assert.ByIdentity())
			expect.Equal(t, inline.Fields[1].Type.Spelling, paramName, "the field of the input keeps its type")
		})

		t.Run("restates a method whose return alone refers to a parameter", func(t *testing.T) {
			t.Parallel()

			inline := &node.TypeRef{Form: symbol.FormInline, Methods: []*node.Method{{
				Name:    "get",
				Params:  []*node.Param{{Name: "p", Type: arg}},
				Returns: []*node.Return{{Type: substUse}},
			}}}
			got := rules.Substitute(inline, params, args)
			assert.Equal(t, got.Methods[0].Returns[0].Type.Spelling, substArg,
				"the result of the method has the type of the argument")
		})

		t.Run("restates a parameter type that refers to a parameter inside an inline body", func(t *testing.T) {
			t.Parallel()

			nested := &node.TypeRef{Form: symbol.FormInline, Fields: []*node.Field{{Name: "x", Type: substUse}}}
			inline := &node.TypeRef{Form: symbol.FormInline, Methods: []*node.Method{{
				Name: "take", Params: []*node.Param{{Name: "p", Type: nested}},
			}}}
			got := rules.Substitute(inline, params, args)
			assert.Equal(t, got.Methods[0].Params[0].Type.Fields[0].Type.Spelling, substArg,
				"the field inside the parameter type has the type of the argument")
		})

		t.Run("restates a method of an inline body inside an argument", func(t *testing.T) {
			t.Parallel()

			body := &node.TypeRef{Form: symbol.FormInline, Methods: []*node.Method{{
				Name: "get", Returns: []*node.Return{{Type: substUse}},
			}}}
			generic := &node.TypeRef{Spelling: substGeneric, Args: []*node.TypeRef{body}}
			got := rules.Substitute(generic, params, args)
			assert.Equal(t, got.Args[0].Methods[0].Returns[0].Type.Spelling, substArg,
				"the method inside the argument returns the argument")
		})

		t.Run("returns a reference without a parameter as it is", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: substGeneric, Args: []*node.TypeRef{{Spelling: substPlain}}}
			assert.Equal(t, rules.Substitute(ref, params, args), ref, "Substitute returns the same reference",
				assert.ByIdentity())
		})

		unchanged := []struct {
			name   string
			params []*node.TypeParam
			args   []*node.TypeRef
		}{
			{name: "returns the reference for no parameters", params: nil, args: nil},
			{name: "returns the reference for a count of arguments that differs", params: params, args: nil},
			{name: "returns the reference for a nil argument", params: params, args: []*node.TypeRef{nil}},
			{name: "returns the reference for a nil parameter", params: []*node.TypeParam{nil}, args: args},
		}
		for _, tt := range unchanged {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := substUse
				assert.Equal(t, rules.Substitute(ref, tt.params, tt.args), ref,
					"Substitute returns the same reference", assert.ByIdentity())
			})
		}

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, rules.Substitute(nil, params, args), "Substitute returns nil")
		})
	})
}

// TestSubstituteAllocs checks the allocation ceilings of Substitute in a
// run without benchmarks. A replaced parameter allocates its copy, and a
// reference without a parameter allocates nothing.
func TestSubstituteAllocs(t *testing.T) {
	for _, tt := range substituteCases() {
		var got *node.TypeRef
		assert.MaxAllocs(t, func() { got = rules.Substitute(tt.ref, tt.params, tt.args) }, tt.allocs,
			"Substitute allocates within its ceiling for "+tt.name)
		assert.Equal(t, got.Spelling, tt.want, "Substitute returns the expected reference for "+tt.name)
	}
}

// BenchmarkSubstitute measures Substitute over a reference to a
// parameter and over a reference without one.
func BenchmarkSubstitute(b *testing.B) {
	b.Run("Substitute", func(b *testing.B) {
		for _, tt := range substituteCases() {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got *node.TypeRef
				for c.Loop() {
					got = rules.Substitute(tt.ref, tt.params, tt.args)
				}
				assert.Equal(b, got.Spelling, tt.want, "Substitute returns the expected reference")
			})
		}
	})
}

// substituteCase is one measured call of Substitute and its ceiling.
type substituteCase struct {
	name   string
	ref    *node.TypeRef
	params []*node.TypeParam
	args   []*node.TypeRef
	allocs uint64
	want   string
}

// substituteCases returns the measured calls of Substitute: a reference
// to a parameter, and a generic reference without one.
func substituteCases() []substituteCase {
	params := []*node.TypeParam{substParam}
	args := []*node.TypeRef{{Spelling: substArg}}
	plain := &node.TypeRef{Spelling: substGeneric, Args: []*node.TypeRef{{Spelling: substPlain}}}
	return []substituteCase{
		{
			name: "a reference to a parameter", ref: substUse, params: params, args: args,
			allocs: substituteAllocs, want: substArg,
		},
		{name: "a reference without a parameter", ref: plain, params: params, args: args, want: substGeneric},
	}
}
