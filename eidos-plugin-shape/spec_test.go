// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/sdk/directive"
)

// The spellings of the forms, the arities and the sources in a spec, and
// the spelling of a value that no constant declares.
const (
	shapeSpelling    = "shape"
	mixinSpelling    = "mixin"
	contractSpelling = "contract"
	oneSpelling      = "one"
	optionalSpelling = "optional"
	manySpelling     = "many"
	anySpelling      = "any"
	inputSpelling    = "input"
	resultSpelling   = "result"
	undeclared       = 9
	undeclaredText   = "9"
	ghostName        = "ghost"
)

// The descriptions of the specs agree with the generated table, and the
// enumerations of a spec have the spellings of the spec files.
func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("Form", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give shape.Form
				want string
			}{
				{name: "returns shape for the form of a shape", give: shape.FormShape, want: shapeSpelling},
				{name: "returns mixin for the form of a mixin", give: shape.FormMixin, want: mixinSpelling},
				{name: "returns contract for the form of a contract", give: shape.FormContract, want: contractSpelling},
				{
					name: "returns the number of a form that no constant declares",
					give: undeclared,
					want: undeclaredText,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the form has its spelling")
				})
			}
		})
	})

	t.Run("Arity", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give shape.Arity
				want string
			}{
				{name: "returns one for the arity one", give: shape.ArityOne, want: oneSpelling},
				{name: "returns optional for the arity optional", give: shape.ArityOptional, want: optionalSpelling},
				{name: "returns many for the arity many", give: shape.ArityMany, want: manySpelling},
				{name: "returns any for the arity any", give: shape.ArityAny, want: anySpelling},
				{
					name: "returns the number of an arity that no constant declares",
					give: undeclared,
					want: undeclaredText,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the arity has its spelling")
				})
			}
		})

		t.Run("Admits", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name  string
				arity shape.Arity
				give  []int
				want  []bool
			}{
				{
					name: "admits exactly one callable for the arity one", arity: shape.ArityOne,
					give: []int{0, 1, 2}, want: []bool{false, true, false},
				},
				{
					name: "admits one callable or none for the arity optional", arity: shape.ArityOptional,
					give: []int{0, 1, 2}, want: []bool{true, true, false},
				},
				{
					name: "admits one callable or more for the arity many", arity: shape.ArityMany,
					give: []int{0, 1, 2}, want: []bool{false, true, true},
				},
				{
					name: "admits any number for the arity any", arity: shape.ArityAny,
					give: []int{0, 1, 5}, want: []bool{true, true, true},
				},
				{
					name: "admits no number for an arity that no constant declares", arity: undeclared,
					give: []int{0, 1}, want: []bool{false, false},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got := make([]bool, 0, len(tt.give))
					for _, n := range tt.give {
						got = append(got, tt.arity.Admits(n))
					}
					assert.Equal(t, got, tt.want, "the arity admits the numbers of callables that it states")
				})
			}
		})
	})

	t.Run("Source", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give shape.Source
				want string
			}{
				{name: "returns input for the inputs", give: shape.SourceInput, want: inputSpelling},
				{name: "returns result for the results", give: shape.SourceResult, want: resultSpelling},
				{
					name: "returns the number of a source that no constant declares",
					give: undeclared,
					want: undeclaredText,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the source has its spelling")
				})
			}
		})
	})

	t.Run("Variant", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name and the claim of a spec without params", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.Associative)
			assert.Equal(t, s.Variant(), directive.Variant{Name: s.Name, Doc: s.Doc},
				"the variant of a mixin without params has the name and the claim alone")
		})

		t.Run("returns the directive spec of each param in order", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.TTL)
			want := make([]directive.ParamSpec, 0, len(s.Params))
			for _, p := range s.Params {
				want = append(want, p.ParamSpec)
			}
			assert.Equal(t, s.Variant().Params, want, "the variant has the directive spec of each param")
		})

		t.Run("returns the roles of a contract", func(t *testing.T) {
			t.Parallel()

			s, _ := shape.SpecOf(shape.Tx)
			v := s.Variant()
			expect.Equal(t, v.Roles, []string{string(shape.TxBegin), string(shape.TxCommit), string(shape.TxRollback)},
				"the variant has the roles of the contract in order")
			expect.True(t, v.RolesRequired, "an instance of the contract writes a role")
		})
	})

	t.Run("SpecOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spec of the constant of a name", func(t *testing.T) {
			t.Parallel()

			s, found := shape.SpecOf(shape.Writer)
			assert.True(t, found, "the catalog has the spec writer")
			expect.Equal(t, s.Name, string(shape.Writer), "the spec has the name")
			expect.Equal(t, s.Form, shape.FormShape, "the spec writer is a shape")
		})

		t.Run("returns the spec of every name of the catalog", func(t *testing.T) {
			t.Parallel()

			for _, want := range shape.Specs() {
				got, found := shape.SpecOf(want.Name)
				expect.True(t, found, "the catalog has the spec "+want.Name)
				expect.Equal(t, got, want, "SpecOf returns the description that Specs returns for "+want.Name)
			}
		})

		t.Run("reports false for a name that no spec has", func(t *testing.T) {
			t.Parallel()

			_, found := shape.SpecOf(ghostName)
			assert.False(t, found, "the catalog has no spec ghost")
		})
	})
}

// SpecOf allocates nothing once the catalog is built.
func TestSpecAllocs(t *testing.T) {
	name := string(shape.Writer)
	var found bool
	expect.MaxAllocs(t, func() { _, found = shape.SpecOf(name) }, 0, "SpecOf allocates nothing")
	assert.True(t, found, "the catalog has the spec writer")
}

// BenchmarkSpec measures the search of a spec by its name.
func BenchmarkSpec(b *testing.B) {
	b.Run("SpecOf", func(b *testing.B) {
		name := string(shape.Writer)
		var found bool
		c := bench.Start(b).MaxAllocs(0).Warmup(1)
		defer c.End()
		for c.Loop() {
			_, found = shape.SpecOf(name)
		}
		assert.True(b, found, "the catalog has the spec writer")
	})
}
