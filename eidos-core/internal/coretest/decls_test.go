// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixtures are what cases assert against, so their own
// contracts are held here: every declaration carries the identity
// the resolution step would have assigned, every member carries its
// host, and the every-kind package really does hold every kind it
// claims. A fixture that quietly omits a kind reports as a passing
// test over a rule that never fired.
func TestDecls(t *testing.T) {
	t.Parallel()

	t.Run("EveryKind", func(t *testing.T) {
		t.Parallel()

		t.Run("declares every matchable kind", func(t *testing.T) {
			t.Parallel()

			found := map[symbol.Kind]bool{}
			for decl := range node.All(coretest.EveryKind(coretest.StorePath)) {
				found[decl.Kind()] = true
			}
			for _, kind := range coretest.MatchableKinds() {
				assert.True(t, found[kind],
					"the fixture declares a "+kind.String()+
						": a kind it omits is a rule that never fires")
			}
		})

		t.Run("loads into a sealed graph", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, coretest.EveryKind(coretest.StorePath))
			assert.NotNil(t, g, "the every-kind fixture is a graph the store accepts")
		})

		t.Run("names each declaration for its kind", func(t *testing.T) {
			t.Parallel()

			names := map[symbol.Kind]string{
				symbol.KindStruct:    coretest.StructName,
				symbol.KindInterface: coretest.InterfaceName,
				symbol.KindEnum:      coretest.EnumName,
				symbol.KindSum:       coretest.SumName,
				symbol.KindAlias:     coretest.AliasName,
				symbol.KindFunction:  coretest.FunctionName,
				symbol.KindVariable:  coretest.VariableName,
				symbol.KindConstant:  coretest.ConstantName,
				symbol.KindField:     coretest.FieldName,
				symbol.KindMethod:    coretest.MethodName,
			}
			for decl := range node.All(coretest.EveryKind(coretest.StorePath)) {
				want, named := names[decl.Kind()]
				if !named {
					continue
				}
				held, declares := decl.(node.Declaration)
				assert.True(t, declares, "a named kind declares an identity")
				if !declares {
					continue
				}
				assert.Equal(t, held.Identity().Name, want,
					"the "+decl.Kind().String()+" carries its fixture name")
			}
		})
	})

	t.Run("Populated", func(t *testing.T) {
		t.Parallel()

		t.Run("hangs its members on their host", func(t *testing.T) {
			t.Parallel()

			s := coretest.Populated(coretest.StorePath, coretest.StructName)
			assert.Length(t, s.Fields, 1, "the struct carries one field")
			assert.Length(t, s.Methods, 1, "and one method")
			assert.Equal(t, s.Fields[0].Host, s.ID,
				"the field's back-pointer names its host")
			assert.Equal(t, s.Methods[0].Host, s.ID,
				"and so does the method's")
		})
	})

	t.Run("Enum", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the host identity on every variant", func(t *testing.T) {
			t.Parallel()

			e := coretest.Enum(coretest.StorePath, coretest.EnumName,
				coretest.EnumVariantName)
			assert.Length(t, e.Variants, 1, "one name in, one variant out")
			assert.Equal(t, e.Variants[0].Host, e.ID,
				"the variant's back-pointer names its enum")
			assert.Equal(t, e.Variants[0].Kind(), symbol.KindEnumVariant,
				"a variant declares its own kind")
		})
	})

	t.Run("Sum", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the host identity on every variant", func(t *testing.T) {
			t.Parallel()

			s := coretest.Sum(coretest.StorePath, coretest.SumName,
				coretest.SumVariantName)
			assert.Length(t, s.Variants, 1, "one name in, one variant out")
			assert.Equal(t, s.Variants[0].Host, s.ID,
				"the variant's back-pointer names its sum")
		})
	})

	t.Run("Interface", func(t *testing.T) {
		t.Parallel()

		t.Run("hosts its method on the interface, not a struct", func(t *testing.T) {
			t.Parallel()

			i := coretest.Interface(coretest.StorePath, coretest.InterfaceName)
			assert.Length(t, i.Methods, 1, "the interface declares one method")
			assert.Equal(t, i.Methods[0].Host, i.ID,
				"the method's back-pointer names the interface")
			assert.True(t, i.Methods[0].Abstract,
				"an interface method carries no body")
		})
	})

	t.Run("Alias", func(t *testing.T) {
		t.Parallel()

		t.Run("names its target by spelling alone", func(t *testing.T) {
			t.Parallel()

			a := coretest.Alias(coretest.StorePath, coretest.AliasName)
			assert.NotNil(t, a.Target, "an alias carries a target reference")
			assert.Equal(t, a.Target.Spelling, coretest.StructName,
				"an unlinked reference arrives as spelling")
			assert.True(t, a.Target.Target.IsZero(),
				"and carries no resolved identity until Link runs")
		})
	})

	t.Run("KindCount", func(t *testing.T) {
		t.Parallel()

		t.Run("counts the declarations the store would index", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.EveryKind(coretest.StorePath)
			for _, kind := range coretest.MatchableKinds() {
				assert.True(t, coretest.KindCount(kind, pkg) > 0,
					"the every-kind package holds a "+kind.String())
			}
			assert.Equal(t, coretest.KindCount(symbol.KindStruct, pkg, pkg), 2,
				"a kind counts once per package it is declared in")
			assert.Equal(t, coretest.KindCount(symbol.KindInvalid, pkg), 0,
				"and a kind nothing declares counts nothing")
		})
	})

	t.Run("Foreign", func(t *testing.T) {
		t.Parallel()

		t.Run("names a kind it is not the model's type for", func(t *testing.T) {
			t.Parallel()

			ghost := coretest.Foreign(coretest.StorePath, coretest.ForeignName,
				symbol.KindStruct)
			assert.Equal(t, ghost.Kind(), symbol.KindStruct,
				"the declaration answers with the kind its identity names")
			assert.Equal(t, ghost.Identity(),
				coretest.ID(coretest.StorePath, coretest.ForeignName, symbol.KindStruct),
				"under the identity the fixture door builds")
			_, isStruct := ghost.(*node.Struct)
			assert.False(t, isStruct,
				"and it is not the model's type for that kind, "+
					"which is what a rule's type assertion refuses")
			assert.True(t, ghost.Position().IsZero(),
				"it stands for a declaration no source produced, so it has no position")
			assert.Nil(t, ghost.Docs(), "and no documentation")
		})

		t.Run("loads into a sealed graph under its kind", func(t *testing.T) {
			t.Parallel()

			ghost := coretest.Foreign(coretest.StorePath, coretest.ForeignName,
				symbol.KindStruct)
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, ghost))
			held := slices.Collect(g.ByKind(symbol.KindStruct))
			assert.Equal(t, held, []symbol.Symbol{ghost},
				"the store indexes it under the kind it names, "+
					"so dispatch reaches it")
		})
	})

	t.Run("MatchableKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("names each kind once", func(t *testing.T) {
			t.Parallel()

			kinds := coretest.MatchableKinds()
			seen := map[symbol.Kind]bool{}
			for _, kind := range kinds {
				assert.False(t, seen[kind],
					"no kind is listed twice: "+kind.String())
				seen[kind] = true
			}
			assert.Equal(t, len(kinds), len(seen), "the list is its own set")
			assert.False(t, slices.Contains(kinds, symbol.KindInvalid),
				"the invalid kind is not matchable")
		})
	})
}
