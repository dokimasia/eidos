// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"io/fs"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

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

			var kinds []symbol.Kind
			for _, u := range unitsOf(t, backendtest.CanonicalFixture(t)) {
				assert.True(t, len(u.Decls) > 0, "every unit has declarations: "+u.Key)
				if len(u.Decls) > 0 {
					kinds = append(kinds, u.Decls[0].Kind())
				}
			}
			slices.Sort(kinds)
			want := fileLevelKinds()
			slices.Sort(want)
			assert.Equal(t, kinds, want, "each file-level kind, once")
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
			tree, declared := f.Trees[f.Schedule[0]]
			assert.True(t, declared, "the emitter declares its template tree")
			if !declared {
				return
			}
			names, err := fs.Glob(tree, referencePattern)
			assert.NoError(t, err, "the tree lists")
			assert.NotEmpty(t, names, "the tree contains the reference template")
		})

		t.Run("routes every unit with the full key", func(t *testing.T) {
			t.Parallel()

			keys := map[string]bool{}
			for _, u := range unitsOf(t, backendtest.CanonicalFixture(t)) {
				assert.True(t, u.Word != "", "every unit has its family word")
				assert.True(t, u.Key != "", "every unit has its routing key")
				assert.True(t, !u.Pkg.IsZero(), "every unit has its package")
				assert.True(t, len(u.Origins) > 0, "every unit has provenance")
				assert.True(t, !keys[u.Key], "routing keys are distinct: "+u.Key)
				keys[u.Key] = true
			}
		})

		t.Run("orders every unit the way a flush leaves it", func(t *testing.T) {
			t.Parallel()

			assertFlushOrder(t, unitsOf(t, backendtest.CanonicalFixture(t)))
		})

		t.Run("builds the same fixture twice", func(t *testing.T) {
			t.Parallel()

			first := unitsOf(t, backendtest.CanonicalFixture(t))
			second := unitsOf(t, backendtest.CanonicalFixture(t))
			assert.Equal(t, first, second,
				"two builds contain the same units in the same order")
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

	assert.True(tb, f != nil && f.Emit != nil, "the fixture contains a store")
	var units []plugin.Unit
	for u := range f.Emit.Units() {
		units = append(units, u)
	}
	return units
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
		assert.True(tb, slices.IsSortedFunc(origins, symbol.Identity.Compare),
			"the declarations order by origin: "+u.Key)
		assert.True(tb, slices.IsSortedFunc(u.Origins, symbol.Identity.Compare),
			"and so do the origins: "+u.Key)
		assert.Equal(tb, len(slices.Compact(slices.Clone(u.Origins))), len(u.Origins),
			"each origin once: "+u.Key)
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
