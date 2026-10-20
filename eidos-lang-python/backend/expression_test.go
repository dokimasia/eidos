// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestExpression pins the texts of declarations that the lowering accepts as
// Python source: the value of an assignment, the default of a parameter, and
// an argument of a decorator.
func TestExpression(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		values := []struct {
			name string
			give string
			want bool
		}{
			{name: "accepts a number as a value", give: "1", want: true},
			{name: "accepts a string as a value", give: `"a"`, want: true},
			{name: "accepts a call as a value", give: "field(default=0)", want: true},
			{name: "accepts a tuple without parentheses as a value", give: "1, 2", want: true},
			{name: "accepts a conditional expression as a value", give: "a if b else c", want: true},
			{name: "accepts a lambda as a value", give: "lambda: 1", want: true},
			{name: "accepts a tuple of starred expressions as a value", give: "*a, *b", want: true},
			{name: "refuses a starred expression as a value", give: "*a"},
			{name: "refuses a starred expression in parentheses as a value", give: "(*a)"},
			{name: "accepts a tuple of one starred expression as a value", give: "(*a,)", want: true},
			{name: "accepts a tuple in parentheses as a value", give: "(1, 2)", want: true},
			{name: "refuses an assignment as a value", give: "x = 1"},
			{name: "refuses an incomplete expression as a value", give: "1 +"},
			{name: "refuses two statements as a value", give: "1\n2"},
			{name: "refuses a statement as a value", give: "import os"},
		}
		for _, tt := range values {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(&emit.Constant{Name: "LIMIT", Value: tt.give})
				assert.Equal(t, err == nil, tt.want, "the check of the value")
			})
		}

		defaults := []struct {
			name string
			give string
			want bool
		}{
			{name: "accepts None as a default", give: "None", want: true},
			{name: "accepts a call as a default", give: "field(default=0)", want: true},
			{name: "refuses a tuple without parentheses as a default", give: "1, 2"},
			{name: "refuses an unpacking as a default", give: "*items"},
			{name: "refuses a default that closes the parameters", give: "1): pass\ndef g(x=1"},
		}
		for _, tt := range defaults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(&emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
					Name: "limit", Default: tt.give,
				})})
				assert.Equal(t, err == nil, tt.want, "the check of the default")
			})
		}

		arguments := []struct {
			name string
			give string
			want bool
		}{
			{name: "accepts a keyword argument of a decorator", give: "frozen=True", want: true},
			{name: "accepts an unpacking of a decorator", give: "*items", want: true},
			{name: "accepts a keyword unpacking of a decorator", give: "**options", want: true},
			{name: "refuses two arguments in one argument of a decorator", give: "1, 2"},
			{name: "refuses an argument that closes the call", give: "1)\nprint(2"},
			{name: "refuses an incomplete keyword argument of a decorator", give: "frozen="},
		}
		for _, tt := range arguments {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(&emit.Function{Name: "load", Annotations: symbol.Annotations{{
					Name: "register", Args: []string{tt.give},
				}}})
				assert.Equal(t, err == nil, tt.want, "the check of the argument")
			})
		}
	})
}
