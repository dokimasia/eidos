// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"io/fs"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// referencePattern matches the templates of the emitter's tree.
const referencePattern = "*.tpl"

// The canonical fixture is what every satellite's suite renders, so
// the kinds it emits, the forms it covers and the order it keeps are
// contract.
func TestCanonical(t *testing.T) {
	t.Parallel()

	t.Run("CanonicalFixture", func(t *testing.T) {
		t.Parallel()

		t.Run("emits one unit per file-level kind", func(t *testing.T) {
			t.Parallel()

			units := unitsOf(t, backendtest.CanonicalFixture(t))
			kinds := make([]symbol.Kind, 0, len(units))
			for _, u := range units {
				assert.NotEmpty(t, u.Decls, "every unit has declarations: "+u.Key)
				kinds = append(kinds, u.Decls[0].Kind())
			}
			assert.Permutation(t, kinds, fileLevelKinds(), "each file-level kind, once")
		})

		t.Run("emits declarations of one kind per unit", func(t *testing.T) {
			t.Parallel()

			for _, u := range unitsOf(t, backendtest.CanonicalFixture(t)) {
				for _, d := range u.Decls {
					assert.Equal(t, d.Kind(), u.Decls[0].Kind(),
						"a unit's generic sibling shares its kind: "+u.Key)
				}
			}
		})

		t.Run("covers the four body forms on its functions", func(t *testing.T) {
			t.Parallel()

			forms := map[emit.Form]bool{}
			for _, u := range unitsOf(t, backendtest.CanonicalFixture(t)) {
				for _, d := range u.Decls {
					if d.Kind() == symbol.KindFunction {
						formsIn(t, forms, d)
					}
				}
			}
			assert.Equal(t, forms, everyForm(),
				"the file-level functions state every content form")
		})

		t.Run("covers the four body forms through the struct's members", func(t *testing.T) {
			t.Parallel()

			forms := map[emit.Form]bool{}
			for _, u := range unitsOf(t, backendtest.CanonicalFixture(t)) {
				for _, d := range u.Decls {
					s, isStruct := d.(*emit.Struct)
					if !isStruct {
						continue
					}
					for _, m := range s.Methods.Items() {
						formsIn(t, forms, m)
					}
				}
			}
			assert.Equal(t, forms, everyForm(),
				"a backend refusing file-level callables still meets every content form")
		})

		t.Run("returns the tree the reference form resolves in", func(t *testing.T) {
			t.Parallel()

			f := backendtest.CanonicalFixture(t)
			assert.Length(t, f.Schedule, 1, "one emitting plugin")
			assert.Contains(t, f.Trees, f.Schedule[0], "the emitter declares its template tree")
			names, err := fs.Glob(f.Trees[f.Schedule[0]], referencePattern)
			assert.NoError(t, err, "the tree lists")
			assert.NotEmpty(t, names, "the tree contains the reference template")
		})

		t.Run("routes every unit with the full key", func(t *testing.T) {
			t.Parallel()

			units := unitsOf(t, backendtest.CanonicalFixture(t))
			keys := make([]string, 0, len(units))
			for _, u := range units {
				expect.NotEmpty(t, u.Word, "every unit has its family word")
				expect.NotEmpty(t, u.Key, "every unit has its routing key")
				expect.NotEqual(t, u.Pkg, symbol.Identity{}, "every unit has its package")
				expect.NotEmpty(t, u.Origins, "every unit has provenance")
				keys = append(keys, u.Key)
			}
			assert.NoDuplicates(t, func() ([]string, error) { return keys, nil }, "routing keys are distinct")
		})

		t.Run("orders every unit the way a flush leaves it", func(t *testing.T) {
			t.Parallel()

			assertFlushOrder(t, unitsOf(t, backendtest.CanonicalFixture(t)))
		})

		t.Run("builds the same fixture on every call", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(tb assert.TB) ([]plugin.Unit, error) {
				return unitsOf(tb, backendtest.CanonicalFixture(tb)), nil
			}, assert.TB(t), "every build contains the same units in the same order")
		})
	})
}

// fileLevelKinds pins the kinds an emit declaration takes at file
// level, which the canonical fixture emits one unit of each.
func fileLevelKinds() []symbol.Kind {
	return []symbol.Kind{
		symbol.KindFunction,
		symbol.KindMethod,
		symbol.KindEnum,
		symbol.KindSum,
		symbol.KindVariable,
		symbol.KindConstant,
		symbol.KindStruct,
		symbol.KindInterface,
		symbol.KindAlias,
	}
}

// unitsOf collects the fixture's units in store order.
func unitsOf(tb assert.TB, f *backendtest.Fixture) []plugin.Unit {
	tb.Helper()

	assert.NotNil(tb, f, "the fixture is built")
	assert.NotNil(tb, f.Emit, "the fixture contains a store")
	return slices.Collect(f.Emit.Units())
}

// formsIn folds every body form found on the given callables into
// the set, reading functions and methods alike.
func formsIn(tb assert.TB, forms map[emit.Form]bool, decls ...symbol.Symbol) {
	tb.Helper()

	for _, d := range decls {
		var body emit.Body
		switch c := d.(type) {
		case *emit.Function:
			body = c.Body
		case *emit.Method:
			body = c.Body
		default:
			continue
		}
		form, err := body.Form()
		assert.NoError(tb, err, "every fixture body states one content form")
		forms[form] = true
	}
}

// assertFlushOrder checks that every unit is in the order a flush
// leaves: the declarations by origin identity, and the origins
// sorted and distinct.
func assertFlushOrder(tb assert.TB, units []plugin.Unit) {
	tb.Helper()

	for _, u := range units {
		origins := make([]symbol.Identity, 0, len(u.Decls))
		for _, d := range u.Decls {
			id, _ := emit.OriginOf(d)
			origins = append(origins, id)
		}
		assert.Pairwise(tb, origins, func(earlier, later symbol.Identity) bool {
			return earlier.Compare(later) <= 0
		}, "the declarations order by origin: "+u.Key)
		assert.Pairwise(tb, u.Origins, func(earlier, later symbol.Identity) bool {
			return earlier.Compare(later) < 0
		}, "the origins are sorted and distinct: "+u.Key)
	}
}

// everyForm is the set of content forms a covering fixture states.
func everyForm() map[emit.Form]bool {
	return map[emit.Form]bool{
		emit.FormDefault:  true,
		emit.FormStmts:    true,
		emit.FormTemplate: true,
		emit.FormVerbatim: true,
	}
}
