// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("names sentinel errors and recognises them", func(t *testing.T) {
		t.Parallel()

		r, is := gorules.New().(rules.ErrorValueRules)
		assert.True(t, is, "the Go rules name error values")
		assert.Equal(
			t,
			r.SentinelName("not found"),
			"ErrNotFound",
			"Err and the base in PascalCase",
		)
		assert.True(t, r.IsSentinelName("ErrNotFound"), "which the inverse recognises")
		assert.False(t, r.IsSentinelName("Err"), "the prefix alone names nothing")
		assert.False(t, r.IsSentinelName("Errors"), "and a lower-case rune after it is a word")
		assert.False(t, r.IsSentinelName("NotFound"), "no prefix, no sentinel")
	})

	t.Run("reads a struct tag by key", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		r, is := gorules.New().(rules.TagRules)
		assert.True(t, is, "the Go rules read tags")
		name := f.field(t, "Tagged", "Name")
		v, held := r.Tag(name, "json")
		assert.True(t, held && v == "name,omitempty", "the json key")
		v, held = r.Tag(name, "db")
		assert.True(t, held && v == "n", "and the db key")
		_, held = r.Tag(name, "xml")
		assert.False(t, held, "an absent key is absent")
		_, held = r.Tag(nil, "json")
		assert.False(t, held, "no field, no tag")
	})

	t.Run(
		"derives witnesses for the trivially closed bounds and substitutes arguments",
		func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r, is := gorules.New().(rules.GenericsRules)
			assert.True(t, is, "the Go rules handle generics")
			assert.False(t, r.Reified(), "Go erases type arguments")
			box, _ := f.decl(t, id(fxPath, "Box", symbol.KindStruct)).(*node.Struct)
			w, derived := r.Derive(box.TypeParams[0], f.view)
			assert.True(t, derived && w.Spelling == "int", "any takes int")
			pair, _ := f.decl(t, id(fxPath, "Pair", symbol.KindStruct)).(*node.Struct)
			w, derived = r.Derive(pair.TypeParams[0], f.view)
			assert.True(t, derived && w.Spelling == "int", "comparable takes int")
			bound, _ := f.decl(t, id(fxPath, "Bound", symbol.KindStruct)).(*node.Struct)
			_, derived = r.Derive(bound.TypeParams[0], f.view)
			assert.False(t, derived, "a bound naming an interface is authored or nothing")
			_, derived = r.Derive(nil, f.view)
			assert.False(t, derived, "no parameter, no witness")
			witnesses := f.bound().Witnesses(pair.TypeParams)
			assert.Length(t, witnesses, 2, "the bound assembles a whole list")

			item := &node.TypeRef{Spelling: "T"}
			listOfT := composite("[]T", symbol.FormList, item)
			got := r.Substitute(listOfT, box.TypeParams, []*node.TypeRef{builtin("string")})
			assert.Equal(
				t,
				got.Elems[0].Spelling,
				"string",
				"the argument binds inside a composite",
			)
			assert.Equal(t, listOfT.Elems[0].Spelling, "T", "on a copy")
			untouched := builtin("int")
			assert.True(
				t,
				r.Substitute(
					untouched,
					box.TypeParams,
					[]*node.TypeRef{builtin("string")},
				) == untouched,
				"a reference naming no parameter is returned as it is",
			)
			assert.True(
				t,
				r.Substitute(item, box.TypeParams, nil) == item,
				"mismatched arguments substitute nothing",
			)
		},
	)

	t.Run("lists the settable members promotion reaches", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		r, is := gorules.New().(rules.PromotionRules)
		assert.True(t, is, "the Go rules know promotion")
		derived, _ := f.decl(t, id(fxPath, "Derived", symbol.KindStruct)).(*node.Struct)
		var names []string
		for _, m := range r.Settable(derived, f.view) {
			names = append(names, m.Symbol.(*node.Field).Name)
		}
		assert.Equal(t, names, []string{"Name", "Kind"}, "the exported fields, own then promoted")
		row, _ := f.decl(t, id(fxPath, "Row", symbol.KindStruct)).(*node.Struct)
		for _, m := range r.Settable(row, f.view) {
			assert.True(
				t,
				m.Symbol.(*node.Field).Name != "name",
				"an unexported field is not settable elsewhere",
			)
		}
		assert.Empty(t, r.Settable(nil, f.view), "nothing has no members")
	})

	t.Run("proves comparability by Go's rules", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		r, is := gorules.New().(rules.EqualityRules)
		assert.True(t, is, "the Go rules know equality")
		ok, problems := r.Comparable(ref(fxPath, "Base", symbol.KindStruct), f.view)
		assert.True(t, ok && len(problems) == 0, "a struct of strings compares")
		ok, problems = r.Comparable(ref(fxPath, "Uncomparable", symbol.KindStruct), f.view)
		assert.False(t, ok, "a struct holding a slice does not")
		assert.Length(t, problems, 1, "naming the slice")
		assert.Equal(t, problems[0].Form, symbol.FormList, "as the problem")
		ok, _ = r.Comparable(ref(fxPath, "Row", symbol.KindStruct), f.view)
		assert.False(t, ok, "nor one holding a slice and a map among its fields")
		ok, _ = r.Comparable(ref(fxPath, "Derived", symbol.KindStruct), f.view)
		assert.True(t, ok, "an embedded comparable struct compares")
		ok, _ = r.Comparable(ref(fxPath, "Reader", symbol.KindInterface), f.view)
		assert.True(t, ok, "an interface compares")
		ok, _ = r.Comparable(ref(fxPath, "Color", symbol.KindEnum), f.view)
		assert.True(t, ok, "an enumeration compares")
		ok, _ = r.Comparable(ref(fxPath, "Weight", symbol.KindAlias), f.view)
		assert.True(t, ok, "a defined type compares as its target")
		for _, ref := range []*node.TypeRef{
			composite("*Row", symbol.FormOptional, builtin("Row")),
			composite("chan int", symbol.FormStream, builtin("int")),
			composite("[2]int", symbol.FormArray, builtin("int")),
			builtin("any"), builtin("string"), builtin("complex64"),
		} {
			ok, _ := r.Comparable(ref, f.view)
			assert.True(t, ok, ref.Spelling+" compares")
		}
		for _, ref := range []*node.TypeRef{
			composite("func()", symbol.FormFunc),
			composite("map[int]int", symbol.FormMap, builtin("int"), builtin("int")),
			{Spelling: "struct{}", Form: symbol.FormInline},
			builtin("Outside"),
			ref(fxPath, "Ghost", symbol.KindStruct),
			nil,
		} {
			ok, _ := r.Comparable(ref, f.view)
			assert.False(t, ok, "does not compare, or cannot be proven to")
		}
	})
}
