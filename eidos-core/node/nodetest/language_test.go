// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/node/nodetest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The spellings of the children of the references that the cases spell.
const (
	elemName   = "T"
	keyName    = "K"
	valueName  = "V"
	stringName = "string"
	mapName    = "Map"
	firstName  = "A"
	secondName = "B"
	resultName = "R"
	fieldName  = "a"
	methodName = "m"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Neutral", func(t *testing.T) {
		t.Parallel()

		t.Run("Lang", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the language of the fixture", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, scripted.Lang(), frontendtest.ScriptedLang, "Lang returns the language of the value")
			})
		})

		t.Run("Overloads", func(t *testing.T) {
			t.Parallel()

			t.Run("reports the overloading of the value", func(t *testing.T) {
				t.Parallel()

				expect.True(t, scripted.Overloads(), "a value that overloads reports true")
				expect.False(t, nodetest.Neutral{Language: scripted.Language}.Overloads(),
					"a value that does not overload reports false")
			})
		})

		t.Run("Spell", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *node.TypeRef
				want string
			}{
				{
					name: "writes an optional with its child",
					give: nodetest.Optional(nodetest.Named(elemName)),
					want: "optional(T)",
				},
				{
					name: "writes a list with its child",
					give: nodetest.List(nodetest.Named(elemName)),
					want: "list(T)",
				},
				{
					name: "writes an array with its length",
					give: nodetest.Array(3, nodetest.Named(elemName)),
					want: "array[3](T)",
				},
				{
					name: "writes a map with its key and its value",
					give: nodetest.Map(nodetest.Named(keyName), nodetest.Named(valueName)),
					want: "map(K, V)",
				},
				{
					name: "writes a function type with its parameters before its results",
					give: nodetest.Func(
						[]*node.TypeRef{nodetest.Named(firstName), nodetest.Named(secondName)},
						[]*node.TypeRef{nodetest.Named(resultName)},
					),
					want: "func(A, B)(R)",
				},
				{
					name: "writes a function type without parameters and results",
					give: nodetest.Func(nil, nil),
					want: "func()()",
				},
				{
					name: "writes a tuple with its members",
					give: nodetest.Tuple(nodetest.Named(firstName), nodetest.Named(secondName)),
					want: "tuple(A, B)",
				},
				{
					name: "writes a union with its members",
					give: nodetest.Union(nodetest.Named(firstName), nodetest.Named(secondName)),
					want: "union(A, B)",
				},
				{
					name: "writes an intersection with its members",
					give: nodetest.Intersection(nodetest.Named(firstName), nodetest.Named(secondName)),
					want: "intersection(A, B)",
				},
				{
					name: "writes a stream with its child",
					give: nodetest.Stream(nodetest.Named(elemName)),
					want: "stream(T)",
				},
				{
					name: "writes a borrow with its child",
					give: nodetest.Borrow(nodetest.Named(elemName)),
					want: "borrow(T)",
				},
				{
					name: "writes an unbounded wildcard without a child",
					give: nodetest.Wildcard(symbol.VarianceInvariant, nil),
					want: "wildcard()",
				},
				{
					name: "writes the upper bound of a wildcard after out",
					give: nodetest.Wildcard(symbol.VarianceOut, nodetest.Named(elemName)),
					want: "wildcard(out T)",
				},
				{
					name: "writes the lower bound of a wildcard after in",
					give: nodetest.Wildcard(symbol.VarianceIn, nodetest.Named(elemName)),
					want: "wildcard(in T)",
				},
				{
					name: "writes an invariant bound of a wildcard without a mark",
					give: nodetest.Wildcard(symbol.VarianceInvariant, nodetest.Named(elemName)),
					want: "wildcard(T)",
				},
				{
					name: "writes an inline body with the names of its fields and methods",
					give: nodetest.Inline([]*node.Field{{Name: fieldName}}, []*node.Method{{Name: methodName}}),
					want: "inline(a, m())",
				},
				{
					name: "writes a child with its type arguments in angle brackets",
					give: nodetest.List(nodetest.Named(mapName, nodetest.Named(keyName),
						nodetest.Named(mapName, nodetest.Named(stringName), nodetest.Named(valueName)))),
					want: "list(Map<K, Map<string, V>>)",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := scripted.Spell(tt.give)
					assert.NoError(t, err, "the notation spells the form")
					assert.Equal(t, got, tt.want, "the spelling is the notation of the form")
				})
			}

			t.Run("returns an error for a named reference", func(t *testing.T) {
				t.Parallel()

				_, err := scripted.Spell(nodetest.Named(elemName))
				assert.HasError(t, err, "a named reference has no structural spelling")
			})

			t.Run("returns an error for a form that only the fold returns", func(t *testing.T) {
				t.Parallel()

				_, err := scripted.Spell(&node.TypeRef{Form: symbol.FormScalar})
				assert.HasError(t, err, "a scalar has no structural spelling")
			})
		})

		t.Run("Qualify", func(t *testing.T) {
			t.Parallel()

			t.Run("writes the name of the package before the name", func(t *testing.T) {
				t.Parallel()

				got := scripted.Qualify(&node.Package{Name: cacheName}, entryName)
				assert.Equal(t, got, cacheName+symbol.OwnerSep+entryName, "the name follows the package and a dot")
			})
		})
	})
}
