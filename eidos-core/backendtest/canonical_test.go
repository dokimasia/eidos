// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// fullInventory is the widest inventory any backend declares
// today: the union the canonical fixture must hold a declaration
// for. Values are what a backend's own map carries and the
// fixture must not read.
func fullInventory() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindEnum:      "unread",
		symbol.KindSum:       "unread",
		symbol.KindStruct:    "unread",
		symbol.KindInterface: "unread",
		symbol.KindFunction:  "unread",
		symbol.KindMethod:    "unread",
		symbol.KindAlias:     "unread",
		symbol.KindConstant:  "unread",
		symbol.KindVariable:  "unread",
	}
}

// unitsOf collects the fixture's units in store order.
func unitsOf(tb assert.TB, f *backendtest.Fixture) []plugin.Unit {
	tb.Helper()

	assert.True(tb, f != nil && f.Emit != nil, "the fixture carries a store")
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
		assert.NoError(tb, err, "every fixture body holds one content form")
		forms[form] = true
	}
}

// everyForm is what a covering fixture must reach.
func everyForm() map[emit.Form]bool {
	return map[emit.Form]bool{
		emit.FormDefault:  true,
		emit.FormStmts:    true,
		emit.FormTemplate: true,
		emit.FormVerbatim: true,
	}
}

func TestCanonicalFixture(t *testing.T) {
	t.Parallel()

	t.Run("emits one unit per requested kind", func(t *testing.T) {
		t.Parallel()

		f := backendtest.CanonicalFixture(t, fullInventory())
		units := unitsOf(t, f)
		assert.Equal(t, len(units), len(fullInventory()),
			"one unit per kind in the inventory")

		seen := map[symbol.Kind]bool{}
		for _, u := range units {
			assert.True(t, len(u.Decls) > 0, "every unit carries declarations")
			seen[u.Decls[0].Kind()] = true
		}
		for k := range fullInventory() {
			assert.True(t, seen[k], "the inventory kind arrives: "+k.String())
		}
	})

	t.Run("keeps every top-level declaration inside the inventory", func(t *testing.T) {
		t.Parallel()

		narrow := map[symbol.Kind]string{
			symbol.KindStruct:    "unread",
			symbol.KindInterface: "unread",
		}
		f := backendtest.CanonicalFixture(t, narrow)
		for _, u := range unitsOf(t, f) {
			for _, d := range u.Decls {
				assert.True(t, narrow[d.Kind()] != "",
					"no unit smuggles a kind the backend never declared: "+
						d.Kind().String())
			}
		}
	})

	t.Run("covers the four body forms on its functions", func(t *testing.T) {
		t.Parallel()

		f := backendtest.CanonicalFixture(t, fullInventory())
		forms := map[emit.Form]bool{}
		for _, u := range unitsOf(t, f) {
			formsIn(t, forms, u.Decls...)
		}
		assert.Equal(t, forms, everyForm(),
			"a callable inventory reaches every content form at file level")
	})

	t.Run("covers the four body forms through the struct's members", func(t *testing.T) {
		t.Parallel()

		narrow := map[symbol.Kind]string{symbol.KindStruct: "unread"}
		f := backendtest.CanonicalFixture(t, narrow)
		forms := map[emit.Form]bool{}
		for _, u := range unitsOf(t, f) {
			for _, d := range u.Decls {
				s, held := d.(*emit.Struct)
				if !held {
					continue
				}
				for _, m := range s.Methods.Items() {
					formsIn(t, forms, m)
				}
			}
		}
		assert.Equal(t, forms, everyForm(),
			"a memberful inventory reaches every content form through its host")
	})

	t.Run("carries the tree the reference form resolves in", func(t *testing.T) {
		t.Parallel()

		f := backendtest.CanonicalFixture(t, fullInventory())
		assert.Equal(t, len(f.Schedule), 1, "one emitting plugin")
		tree, held := f.Trees[f.Schedule[0]]
		assert.True(t, held, "the emitter declares its template tree")
		if !held {
			return
		}
		names, err := fs.Glob(tree, "*.tpl")
		assert.NoError(t, err, "the tree lists")
		assert.True(t, len(names) > 0, "the tree holds the reference template")
	})

	t.Run("routes every unit with the full key", func(t *testing.T) {
		t.Parallel()

		f := backendtest.CanonicalFixture(t, fullInventory())
		keys := map[string]bool{}
		for _, u := range unitsOf(t, f) {
			assert.True(t, u.Word != "", "every unit carries its family word")
			assert.True(t, u.Key != "", "every unit carries its routing key")
			assert.True(t, !u.Pkg.IsZero(), "every unit carries its package")
			assert.True(t, len(u.Origins) > 0, "every unit carries provenance")
			assert.True(t, !keys[u.Key], "routing keys stay distinct: "+u.Key)
			keys[u.Key] = true
		}
	})

	t.Run("builds the same fixture twice", func(t *testing.T) {
		t.Parallel()

		first := unitsOf(t, backendtest.CanonicalFixture(t, fullInventory()))
		second := unitsOf(t, backendtest.CanonicalFixture(t, fullInventory()))
		assert.Equal(t, first, second,
			"two builds hold the same units in the same order")
	})

	t.Run("rejects an empty inventory", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "an inventory holding nothing must fail",
			func(tb assert.TB) {
				backendtest.CanonicalFixture(tb, nil)
			})
		assert.Contains(t, failure, "inventory",
			"the refusal names what was empty")
	})

	t.Run("rejects a kind it holds no declaration for", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "an uncovered kind must fail",
			func(tb assert.TB) {
				backendtest.CanonicalFixture(tb, map[symbol.Kind]string{
					symbol.KindEnumVariant: "unread",
				})
			})
		assert.Contains(t, failure, symbol.KindEnumVariant.String(),
			"the refusal names the kind the fixture does not hold")
	})
}
