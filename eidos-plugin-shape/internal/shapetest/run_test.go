// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// A run is a fixture with the rules and the keys of the catalog, whose
// instances have the names, the numbers and the positions that validation
// gives them.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the rules of the test language", func(t *testing.T) {
			t.Parallel()

			r := shapetest.New(t, shapetest.Package())
			_, registered := r.Rules.For(shapetest.Lang)
			assert.True(t, registered, "the fixture binds the test language to its rules")
		})

		t.Run("registers the keys of the catalog", func(t *testing.T) {
			t.Parallel()

			r := shapetest.New(t, shapetest.Package())
			_, registered := meta.Lookup[string](r.Keys, shape.KeyShape)
			assert.True(t, registered, "the registry of the fixture contains the summary key of the shape")
		})
	})

	t.Run("Declare", func(t *testing.T) {
		t.Parallel()

		t.Run("records each instance under the canonical name of its directive", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, nil)
			r := shapetest.New(t, shapetest.Package(put))
			got := r.Declare(t, put.ID,
				shapetest.Instance{Shape: shape.Writer},
				shapetest.Instance{Mixin: shape.Atomic},
				shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit})
			directives, provides := catalog.Annotators()[0].(plugin.DirectiveProvider)
			assert.True(t, provides, "the plugin shape declares its directives")
			specs := directives.Directives()
			want := make([]directive.Name, 0, len(specs))
			for _, s := range specs {
				want = append(want, s.Canonical())
			}
			assert.Length(t, got, 3, "the run records the three instances")
			names := []directive.Name{got[0].Name, got[1].Name, got[2].Name}
			assert.Equal(t, names, want, "each instance has the canonical name of the directive of its form")
		})

		t.Run("numbers the instances of one directive on a subject", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, nil)
			r := shapetest.New(t, shapetest.Package(put))
			got := r.Declare(t, put.ID,
				shapetest.Instance{Mixin: shape.Atomic},
				shapetest.Instance{Mixin: shape.Idempotent},
				shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit})
			assert.Length(t, got, 3, "the run records the three instances")
			assert.Equal(t, []int{got[0].Instance, got[1].Instance, got[2].Instance}, []int{0, 1, 0},
				"the instances of each directive are numbered from 0")
		})

		t.Run("gives each instance a line of its own", func(t *testing.T) {
			t.Parallel()

			put, get := shapetest.Method(storeName, putName, nil), shapetest.Method(storeName, getName, nil)
			r := shapetest.New(t, shapetest.Package(put, get))
			first := r.Declare(
				t,
				put.ID,
				shapetest.Instance{Mixin: shape.Atomic},
				shapetest.Instance{Mixin: shape.Pure},
			)
			second := r.Declare(t, get.ID, shapetest.Instance{Mixin: shape.Atomic})
			assert.NoDuplicates(t, func() ([]position.Pos, error) {
				return []position.Pos{first[0].Pos, first[1].Pos, second[0].Pos, put.Pos, get.Pos}, nil
			}, "no instance has the position of another instance or of a declaration")
		})

		t.Run("overrides the detectors with an instance of the shape directive alone", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, nil)
			r := shapetest.New(t, shapetest.Package(put))
			got := r.Declare(
				t,
				put.ID,
				shapetest.Instance{Shape: shape.Writer},
				shapetest.Instance{Mixin: shape.Atomic},
			)
			assert.Length(t, got, 2, "the run records the two instances")
			expect.True(t, got[0].Overrides, "the shape instance overrides the detectors")
			expect.False(t, got[1].Overrides, "the mixin instance overrides nothing")
		})
	})

	t.Run("Classify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the findings of the plugin shapecheck", func(t *testing.T) {
			t.Parallel()

			put := shapetest.Method(storeName, putName, nil)
			r := shapetest.New(t, shapetest.Package(put))
			r.Declare(t, put.ID, shapetest.Instance{Contract: shape.Tx, Role: shape.TxCommit})
			got := r.Classify(t)
			assert.NotEmpty(t, got, "the instance without a begin and a rollback has findings")
			for _, d := range got {
				expect.Equal(t, d.Code, catalog.RoleArity, "the finding reports the arity of a role")
				expect.Equal(t, d.Severity, diag.SeverityError, "the finding is an Error")
			}
		})
	})

	t.Run("Consume", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the rules over the classified callables", func(t *testing.T) {
			t.Parallel()

			put, get := shapetest.Method(storeName, putName, nil), shapetest.Method(storeName, getName, nil)
			r := shapetest.New(t, shapetest.Package(put, get))
			r.Declare(t, put.ID, shapetest.Instance{Mixin: shape.Atomic})
			assert.Empty(t, r.Classify(t), "the classification has no findings")
			var visited []string
			r.Consume(
				t,
				sdk.Where(shape.Has(shape.Atomic), sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
					visited = append(visited, m.Method.Name)
					return nil
				})),
			)
			assert.Equal(t, visited, []string{putName}, "the consumer visits the atomic method alone")
		})
	})
}
