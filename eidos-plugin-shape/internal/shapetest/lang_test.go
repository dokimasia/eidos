// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The rules of the test language classify parameters and returns by the
// spellings of their types, and fold types as the scripted rules do.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Rules", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the rules of the test language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, shapetest.Rules().Lang(), shapetest.Lang, "the rules declare the test language")
		})

		t.Run("gives a context parameter the context role", func(t *testing.T) {
			t.Parallel()

			p := &node.Param{Type: &node.TypeRef{Spelling: shapetest.Context}}
			assert.Equal(t, shapetest.Rules().ParamRole(p, rules.View{}), rules.ParamContext,
				"a parameter of the spelling context has the context role")
		})

		t.Run("gives every other parameter the input role", func(t *testing.T) {
			t.Parallel()

			p := &node.Param{Type: &node.TypeRef{Spelling: shapetest.String}}
			assert.Equal(t, shapetest.Rules().ParamRole(p, rules.View{}), rules.ParamInput,
				"a parameter of the spelling string has the input role")
		})

		t.Run("folds a bool into the truth leaf", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(shapetest.Rules(), rules.View{}, nil)
			assert.Equal(t, b.TypeOf(&node.TypeRef{Spelling: shapetest.Bool}).Form, symbol.FormBool,
				"the scripted fold classifies the builtin bool")
		})

		t.Run("folds a list of bytes into the bytes form", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(shapetest.Rules(), rules.View{}, nil)
			bytes := &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: shapetest.Byte}}}
			assert.Equal(t, b.TypeOf(bytes).Form, symbol.FormBytes, "a list of the eight-bit byte folds to bytes")
		})

		tests := []struct {
			name      string
			give      []*node.TypeRef
			wantRoles []rules.ReturnRole
			wantModel rules.ErrorModel
		}{
			{
				name:      "classifies a last error return under the last-return model",
				give:      []*node.TypeRef{{Spelling: shapetest.Int}, {Spelling: shapetest.Error}},
				wantRoles: []rules.ReturnRole{rules.ReturnValue, rules.ReturnError},
				wantModel: rules.ErrorsLastReturn,
			},
			{
				name:      "classifies an error return before the last return as a value",
				give:      []*node.TypeRef{{Spelling: shapetest.Error}, {Spelling: shapetest.Int}},
				wantRoles: []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue},
				wantModel: rules.ErrorsNone,
			},
			{
				name:      "classifies the second of two bool returns as the ok flag",
				give:      []*node.TypeRef{{Spelling: shapetest.String}, {Spelling: shapetest.Bool}},
				wantRoles: []rules.ReturnRole{rules.ReturnValue, rules.ReturnOkBool},
				wantModel: rules.ErrorsNone,
			},
			{
				name: "classifies a bool return among three returns as a value",
				give: []*node.TypeRef{
					{Spelling: shapetest.String},
					{Spelling: shapetest.Int},
					{Spelling: shapetest.Bool},
				},
				wantRoles: []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue, rules.ReturnValue},
				wantModel: rules.ErrorsNone,
			},
			{
				name: "classifies a return of the stream form as a stream",
				give: []*node.TypeRef{
					{
						Spelling: shapetest.String,
						Form:     symbol.FormStream,
						Elems:    []*node.TypeRef{{Spelling: shapetest.String}},
					},
				},
				wantRoles: []rules.ReturnRole{rules.ReturnStream},
				wantModel: rules.ErrorsNone,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				rs := make([]*node.Return, 0, len(tt.give))
				for _, ref := range tt.give {
					rs = append(rs, &node.Return{Type: ref})
				}
				roles, model := shapetest.Rules().ReturnRoles(rs, rules.View{})
				expect.Equal(t, roles, tt.wantRoles, "the rules give each return its role")
				expect.Equal(t, model, tt.wantModel, "the rules name the error model of the returns")
			})
		}
	})
}
