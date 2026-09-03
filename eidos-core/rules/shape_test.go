// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fold is the kernel's: every structural form folds with its
// children, a resolved name classifies by its declaration, and an
// unresolved one goes to the language.
func TestShape(t *testing.T) {
	t.Parallel()

	t.Run("TypeOf", func(t *testing.T) {
		t.Parallel()

		t.Run("folds every structural form with its children", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			optional := &node.TypeRef{
				Spelling: "*Row", Form: symbol.FormOptional,
				Elems: []*node.TypeRef{named(svcPath, rowName, symbol.KindStruct)},
			}
			s := b.TypeOf(optional)
			assert.Equal(t, s.Form, symbol.FormOptional, "the form carries")
			assert.Equal(t, s.Spelling, "*Row", "with the spelling")
			assert.Length(t, s.Elems, 1, "and its child")
			assert.Equal(t, s.Elems[0].Form, symbol.FormReference, "folded to a reference")
			assert.Equal(t, s.Elems[0].Ref, coretest.ID(svcPath, rowName, symbol.KindStruct), "to the declaration")

			mapped := &node.TypeRef{
				Spelling: "map[string]int", Form: symbol.FormMap,
				Elems: []*node.TypeRef{builtin(strSpelling), builtin(intSpelling)},
			}
			m := b.TypeOf(mapped)
			assert.Equal(t, m.Elems[0].Form, symbol.FormText, "a map's key folds")
			assert.Equal(t, m.Elems[1].Form, symbol.FormScalar, "and its value")

			fn := &node.TypeRef{
				Spelling: "func(int) string", Form: symbol.FormFunc, Split: 1,
				Elems: []*node.TypeRef{builtin(intSpelling), builtin(strSpelling)},
			}
			f := b.TypeOf(fn)
			assert.Equal(t, f.Split, 1, "a function type keeps its split")
			arr := &node.TypeRef{
				Spelling: "[4]int", Form: symbol.FormArray, Length: 4,
				Elems: []*node.TypeRef{builtin(intSpelling)},
			}
			assert.Equal(t, b.TypeOf(arr).Length, 4, "an array keeps its length")
			wild := &node.TypeRef{
				Spelling: "? extends Row", Form: symbol.FormWildcard,
				Variance: symbol.VarianceOut, Elems: []*node.TypeRef{named(svcPath, rowName, symbol.KindStruct)},
			}
			assert.Equal(t, b.TypeOf(wild).Variance, symbol.VarianceOut, "a wildcard keeps its variance")
			assert.Empty(t, b.TypeOf(&node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline}).Elems,
				"an inline body has no children")
		})

		t.Run("folds a list of eight-bit unsigned scalars to bytes", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(bytesLang{scripted()}, viewOnly(t), nil)
			list := &node.TypeRef{
				Spelling: "[]byte", Form: symbol.FormList,
				Elems: []*node.TypeRef{builtin("byte")},
			}
			s := b.TypeOf(list)
			assert.Equal(t, s.Form, symbol.FormBytes, "the one rule beyond structure")
			assert.Equal(t, s.Spelling, "[]byte", "keeps the spelling")
			assert.Empty(t, s.Elems, "and drops the child")
			ints := &node.TypeRef{
				Spelling: "[]int", Form: symbol.FormList,
				Elems: []*node.TypeRef{builtin(intSpelling)},
			}
			assert.Equal(t, b.TypeOf(ints).Form, symbol.FormList, "any other element stays a list")
		})

		t.Run("classifies a resolved name by its declaration", func(t *testing.T) {
			t.Parallel()

			sum := coretest.Sum(depPath, "Shape", "Circle")
			g := coretest.Frozen(t, hierarchy(), coretest.Package(depPath, sum))
			b, _, _ := boundOver(t, g)
			ref := named(svcPath, rowName, symbol.KindStruct)
			ref.Args = []*node.TypeRef{builtin(intSpelling)}
			s := b.TypeOf(ref)
			assert.Equal(t, s.Form, symbol.FormReference, "a struct is a reference")
			assert.Length(t, s.Args, 1, "with its arguments folded")
			assert.Equal(t, s.Args[0].Form, symbol.FormScalar, "in turn")
			assert.Equal(t, b.TypeOf(named(depPath, "Shape", symbol.KindSum)).Form, symbol.FormSum,
				"and a sum is a sum")
		})

		t.Run("sends an unresolved name to the language", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.TypeOf(builtin(intSpelling)).Form, symbol.FormScalar, "a builtin classifies")
			assert.Equal(t, b.TypeOf(builtin("Unknown")).Form, symbol.FormOpaque, "an unknown spelling is opaque")
			assert.Equal(t, b.TypeOf(builtin("Unknown")).Spelling, "Unknown", "carrying its spelling")
			assert.Equal(t, b.TypeOf(nil).Form, symbol.FormOpaque, "and nil is opaque too")
		})

		t.Run("treats a target outside the scope as opaque", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, hierarchy(), coretest.Package(depPath, coretest.Struct(depPath, "Far")))
			reads := store.NewReadSet()
			reader, err := g.Reader(reads, func(pkg symbol.Identity) bool { return pkg.Package == svcPath })
			assert.NoError(t, err, "a scoped reader mints")
			b := rules.NewBound(scripted(), rules.View{Decls: reader, Reads: reads}, nil)
			s := b.TypeOf(named(depPath, "Far", symbol.KindStruct))
			assert.Equal(t, s.Form, symbol.FormOpaque, "what the plan could never read folds opaque")
		})

		t.Run("memoises per reference", func(t *testing.T) {
			t.Parallel()

			b, reads, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			ref := named(svcPath, rowName, symbol.KindStruct)
			first := b.TypeOf(ref)
			second := b.TypeOf(ref)
			assert.Equal(t, second, first, "one reference folds once")
			recorded := 0
			for range reads.Identities() {
				recorded++
			}
			assert.Equal(t, recorded, 1, "and the read records once")
		})

		t.Run("folds a reference once per binding", func(t *testing.T) {
			t.Parallel()

			c := &counting{SourceRules: scripted()}
			b := rules.NewBound(c, viewOnly(t), nil)
			ref := builtin(intSpelling)
			first := b.TypeOf(ref)
			second := b.TypeOf(ref)
			assert.Equal(t, c.asked, 1, "the language is asked once")
			assert.Equal(t, first.Form, second.Form, "and the memo answers the same")
			other := builtin(intSpelling)
			b.TypeOf(other)
			assert.Equal(t, c.asked, 2, "a distinct reference folds on its own")
			assert.True(t, first.Args == nil, "a reference without arguments carries none")
		})
	})

	t.Run("constructors", func(t *testing.T) {
		t.Parallel()

		t.Run("build the leaves and the well-known references", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Scalar("int32", rules.ScalarInt, 32),
				rules.TypeShape{Form: symbol.FormScalar, Spelling: "int32", Class: rules.ScalarInt, Bits: 32},
				"a scalar")
			assert.Equal(t, rules.Leaf(symbol.FormBool, "bool").Form, symbol.FormBool, "a leaf")
			ref := rules.Reference("time.Time", rules.WellKnownTimestamp)
			assert.Equal(t, ref.Ref, rules.WellKnownTimestamp, "a well-known reference")
			assert.True(t, rules.IsWellKnown(rules.WellKnownDuration), "which the registry blesses")
			assert.False(t, rules.IsWellKnown(coretest.ID(svcPath, rowName, symbol.KindStruct)),
				"and an ordinary identity is not")
			assert.Equal(t, rules.Opaque(nil).Spelling, "", "opaque of nothing spells nothing")
			assert.Equal(t, rules.ScalarFloat.String(), "float", "a class spells")
			assert.Equal(t, rules.ScalarClass(9).String(), "9", "and an undeclared one numbers")
		})
	})
}

// bytesLang classifies "byte" as the eight-bit unsigned scalar, the
// way a language with the spelling does.
type bytesLang struct {
	rules.SourceRules
}

// Builtin classifies byte and defers the rest.
func (b bytesLang) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	if ref.Spelling == "byte" {
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 8)
	}
	return b.SourceRules.Builtin(ref, v)
}

// viewOnly mints a view over the walk fixture.
func viewOnly(tb assert.TB) rules.View {
	tb.Helper()

	v, _, _ := viewOver(tb, coretest.Frozen(tb, hierarchy()))
	return v
}

// counting counts how often the language's Builtin is asked.
type counting struct {
	rules.SourceRules
	asked int
}

func (c *counting) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	c.asked++
	return c.SourceRules.Builtin(ref, v)
}
