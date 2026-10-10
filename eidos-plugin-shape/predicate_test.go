// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/sdk"
)

// gatedID is the name of the generator whose Build a case checks.
const gatedID = "gated"

// The predicates gate a rule on the keys that the plugin shape stamps.
func TestPredicate(t *testing.T) {
	t.Parallel()

	t.Run("Any", func(t *testing.T) {
		t.Parallel()

		t.Run("admits every classified callable", func(t *testing.T) {
			t.Parallel()

			assert.Permutation(t, visited(t, shape.Any()), []string{putName, findName, commitName, retryName},
				"the gate admits the methods with a classification, and not Get or Stale")
		})
	})

	t.Run("Is", func(t *testing.T) {
		t.Parallel()

		t.Run("admits the callables with the shape", func(t *testing.T) {
			t.Parallel()

			assert.Permutation(t, visited(t, shape.Is(shape.Writer)), []string{putName, findName},
				"the gate admits the detected writer and the declared writer")
		})

		t.Run("panics at the Build of a plugin for a name that no shape spec has", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				sdk.NewPlugin(gatedID).Handle(sdk.Where(shape.Is(ghostName), sdk.OnMethod(ignore))).Build()
			}, "a gate on no key is a defect of the plugin")
		})
	})

	t.Run("Has", func(t *testing.T) {
		t.Parallel()

		t.Run("admits the callables with the mixin", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, visited(t, shape.Has(shape.Atomic)), []string{commitName},
				"the gate admits the atomic method alone")
		})

		t.Run("panics at the Build of a plugin for a name that no mixin spec has", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				sdk.NewPlugin(gatedID).Handle(sdk.Where(shape.Has(ghostName), sdk.OnMethod(ignore))).Build()
			}, "a gate on no key is a defect of the plugin")
		})
	})

	t.Run("In", func(t *testing.T) {
		t.Parallel()

		t.Run("admits the callables with any role in the contract", func(t *testing.T) {
			t.Parallel()

			f := instanceFixture(t)
			var got []string
			f.run.Consume(t, sdk.Where(shape.In(shape.Tx), sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
				got = append(got, m.Method.Host.Name+m.Method.Name)
				return nil
			})))
			want := []string{
				storeName + beginName, storeName + commitName, storeName + rollbackName, cacheName + beginName,
			}
			assert.Permutation(t, got, want,
				"the gate admits the three roles of the transaction of Store and the begin of Cache")
		})

		t.Run("panics at the Build of a plugin for a name that no contract spec has", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				sdk.NewPlugin(gatedID).Handle(sdk.Where(shape.In(ghostName), sdk.OnMethod(ignore))).Build()
			}, "a gate on no key is a defect of the plugin")
		})
	})

	t.Run("Plays", func(t *testing.T) {
		t.Parallel()

		t.Run("admits the callables in the role", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, visited(t, shape.Plays(shape.Tx, shape.TxCommit)), []string{commitName},
				"the gate admits the method in the role commit")
		})

		t.Run("refuses the callables in another role", func(t *testing.T) {
			t.Parallel()

			assert.Empty(
				t,
				visited(t, shape.Plays(shape.Tx, shape.TxBegin)),
				"no method of the fixture begins a transaction",
			)
		})

		t.Run("panics at the Build of a plugin for a name that no contract spec has", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				sdk.NewPlugin(gatedID).
					Handle(sdk.Where(shape.Plays(ghostName, shape.TxBegin), sdk.OnMethod(ignore))).
					Build()
			}, "a gate on no key is a defect of the plugin")
		})
	})
}

// visited returns the names of the methods of the fixture of [classify]
// that a consumer gated on the predicate visits, in the order of its
// invocations.
func visited(t *testing.T, gate sdk.Pred) []string {
	t.Helper()

	c := classify(t)
	var names []string
	c.run.Consume(t, sdk.Where(gate, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
		names = append(names, m.Method.Name)
		return nil
	})))
	return names
}

// ignore is the handler of a generator whose Build a case checks. It
// emits nothing.
func ignore(*sdk.MethodMatch, *sdk.Emitter) error { return nil }
