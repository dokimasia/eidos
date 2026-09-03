// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// names lists a set's member names in order.
func names(set rules.MemberSet) []string {
	out := make([]string, 0, len(set.Members))
	for _, m := range set.Members {
		switch d := m.Symbol.(type) {
		case *node.Field:
			out = append(out, d.Name)
		case *node.Method:
			out = append(out, d.Name)
		}
	}
	return out
}

// reasons lists a set's gap reasons in order.
func reasons(set rules.MemberSet) []rules.GapReason {
	out := make([]rules.GapReason, 0, len(set.Gaps))
	for _, g := range set.Gaps {
		out = append(out, g.Reason)
	}
	return out
}

// The walk is the kernel's and the shadowing is the language's, so
// each rule, each gap and the provenance are pinned here.
func TestMembers(t *testing.T) {
	t.Parallel()

	t.Run("MembersOf", func(t *testing.T) {
		t.Parallel()

		t.Run("walks embeds under promotion with provenance", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			derived, held := b.View().Lookup(coretest.ID(svcPath, derivedName, symbol.KindStruct))
			assert.True(t, held, "the fixture holds the embedding struct")
			set, is := b.MembersOf(derived)
			assert.True(t, is, "a struct walks")
			assert.Equal(t, names(set), []string{"name", "id", "ID"},
				"the declared member first, then the promoted ones in the contributor's order")
			assert.True(t, set.Complete(), "every contributor yielded")
			assert.Equal(t, set.Members[0].Depth, 0, "a declared member sits at depth zero")
			assert.Empty(t, set.Members[0].Through, "through nothing")
			assert.Equal(t, set.Members[1].Depth, 1, "a promoted one at depth one")
			base := coretest.ID(svcPath, baseName, symbol.KindStruct)
			assert.Equal(t, set.Members[1].Through, []symbol.Identity{base},
				"through the embed it arrived by")
			assert.Equal(t, set.Members[1].Owner, coretest.ID(svcPath, baseName, symbol.KindStruct),
				"owned by the declaration that declared it")
		})

		t.Run("lets a declared member shadow a promoted one", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
			derived := coretest.Struct(svcPath, derivedName)
			derived.Embeds = []*node.Embed{embedding(svcPath, baseName)}
			derived.Fields = []*node.Field{field(svcPath, derivedName, "id", builtin(strSpelling))}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, base, derived)))
			set, _ := b.MembersOf(derived)
			assert.Equal(t, names(set), []string{"id"}, "one member survives")
			assert.Equal(t, set.Members[0].Depth, 0, "the declared one")
		})

		t.Run("cancels two promotions at one depth", func(t *testing.T) {
			t.Parallel()

			left := coretest.Struct(svcPath, "Left")
			left.Fields = []*node.Field{field(svcPath, "Left", "id", builtin(intSpelling))}
			right := coretest.Struct(svcPath, "Right")
			right.Fields = []*node.Field{field(svcPath, "Right", "id", builtin(strSpelling))}
			both := coretest.Struct(svcPath, "Both")
			both.Embeds = []*node.Embed{embedding(svcPath, "Left"), embedding(svcPath, "Right")}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, left, right, both)))
			set, _ := b.MembersOf(both)
			assert.Empty(t, names(set), "an ambiguous selector is an error in Go, so neither promotes")
			assert.True(t, set.Complete(), "and that is no gap")
		})

		t.Run("settles under the other three rules", func(t *testing.T) {
			t.Parallel()

			left := coretest.Struct(svcPath, "Left")
			left.Methods = []*node.Method{method(svcPath, "Left", "Run")}
			right := coretest.Struct(svcPath, "Right")
			right.Methods = []*node.Method{method(svcPath, "Right", "Run")}
			both := coretest.Struct(svcPath, "Both")
			both.Extends = []*node.TypeRef{
				named(svcPath, "Left", symbol.KindStruct), named(svcPath, "Right", symbol.KindStruct),
			}
			g := coretest.Frozen(t, coretest.Package(svcPath, left, right, both))
			for _, tc := range []struct {
				rule rules.Shadowing
				want []string
			}{
				{rules.ShadowOverride, []string{"Run"}},
				{rules.ShadowLinearise, []string{"Run"}},
				{rules.ShadowMerge, []string{"Run", "Run"}},
			} {
				v, _, _ := viewOver(t, g)
				b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
					Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: tc.rule,
				}}, v, nil)
				set, _ := b.MembersOf(both)
				assert.Equal(t, names(set), tc.want, tc.rule.String()+" settles as the language states")
				if tc.rule != rules.ShadowMerge {
					assert.Equal(t, set.Members[0].Owner, coretest.ID(svcPath, "Left", symbol.KindStruct),
						"the first arrival wins")
				}
			}
		})

		t.Run("reports every contributor it cannot follow", func(t *testing.T) {
			t.Parallel()

			alias := coretest.Alias(svcPath, "Named")
			loopA := coretest.Struct(svcPath, "A")
			loopB := coretest.Struct(svcPath, "B")
			loopA.Embeds = []*node.Embed{embedding(svcPath, "B")}
			loopB.Embeds = []*node.Embed{embedding(svcPath, "A")}
			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{
				{Ref: builtin("Elsewhere")},
				{Ref: named(svcPath, "Missing", symbol.KindStruct)},
				embedOf(svcPath, "Named", symbol.KindAlias),
				embedding(svcPath, "A"),
			}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, alias, loopA, loopB, host)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{
				rules.GapUnresolved, rules.GapUnresolved, rules.GapNotMembered, rules.GapCyclic,
			}, "an untargeted reference, a missing target, an alias, and a loop each name their reason")
			assert.False(t, set.Complete(), "so the set is incomplete")
			assert.Equal(t, set.Gaps[0].Host, coretest.ID(svcPath, "Host", symbol.KindStruct),
				"each gap names the host that wrote the contributor")
		})

		t.Run("stops at the depth budget", func(t *testing.T) {
			t.Parallel()

			var decls []symbol.Symbol
			for i := range 4 {
				s := coretest.Struct(svcPath, "L"+string(rune('0'+i)))
				if i > 0 {
					s.Embeds = []*node.Embed{embedding(svcPath, "L"+string(rune('0'+i-1)))}
				}
				if i == 0 {
					s.Fields = []*node.Field{field(svcPath, "L0", "deep", builtin(intSpelling))}
				}
				decls = append(decls, s)
			}
			g := coretest.Frozen(t, coretest.Package(svcPath, decls...))
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote, Depth: 2,
			}}, v, nil)
			top := decls[3]
			set, _ := b.MembersOf(top)
			assert.Empty(t, names(set), "the field three embeds down is past a budget of two")
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapCyclic}, "and the budget reports as a cycle")
		})

		t.Run("binds a generic contributor's arguments or reports it", func(t *testing.T) {
			t.Parallel()

			box := coretest.Struct(svcPath, "Box")
			box.TypeParams = []*node.TypeParam{{Name: "T"}}
			box.Fields = []*node.Field{field(svcPath, "Box", "item", builtin("T"))}
			holder := coretest.Struct(svcPath, "Holder")
			ref := named(svcPath, "Box", symbol.KindStruct)
			ref.Args = []*node.TypeRef{builtin(intSpelling)}
			holder.Embeds = []*node.Embed{{Ref: ref}}
			g := coretest.Frozen(t, coretest.Package(svcPath, box, holder))

			b, _, _ := boundOver(t, g)
			set, _ := b.MembersOf(holder)
			assert.Equal(t, names(set), []string{"item"}, "the member arrives")
			assert.Equal(t, set.Members[0].Symbol.(*node.Field).Type.Spelling, intSpelling,
				"with the argument bound in place of the parameter")
			assert.True(t, set.Members[0].Symbol != box.Fields[0], "on a copy, never the graph's node")

			v, _, _ := viewOver(t, g)
			plain := rules.NewBound(nongeneric{scripted()}, v, nil)
			set, _ = plain.MembersOf(holder)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapGeneric},
				"a language without the generics capability reports the contributor")
		})

		t.Run("folds an interface's arrivals as a set", func(t *testing.T) {
			t.Parallel()

			reader := iface(svcPath, "Reader")
			reader.Methods = []*node.Method{method(svcPath, "Reader", "Read")}
			closer := iface(svcPath, "Closer")
			closer.Methods = []*node.Method{method(svcPath, "Closer", "Read")}
			other := iface(svcPath, "Other")
			other.Methods = []*node.Method{coretest.Method(svcPath, "Other", "Read")}
			both := iface(svcPath, "Both")
			both.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Reader", symbol.KindInterface)},
				{Ref: named(svcPath, "Closer", symbol.KindInterface)},
			}
			clash := iface(svcPath, "Clash")
			clash.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Reader", symbol.KindInterface)},
				{Ref: named(svcPath, "Other", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, reader, closer, other, both, clash)))
			set, _ := b.MembersOf(both)
			assert.Equal(t, names(set), []string{"Read"}, "one signature arriving twice is one member")
			assert.True(t, set.Complete(), "and no gap")
			set, _ = b.MembersOf(clash)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
				"two signatures under one name conflict")
		})

		t.Run("records the optional form on the path", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
			derived := coretest.Struct(svcPath, derivedName)
			derived.Embeds = []*node.Embed{{Ref: &node.TypeRef{
				Spelling: "*Base", Form: symbol.FormOptional,
				Elems: []*node.TypeRef{named(svcPath, baseName, symbol.KindStruct)},
			}}}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, base, derived)))
			set, _ := b.MembersOf(derived)
			assert.Equal(t, names(set), []string{"ID"}, "a structural embed resolves through its child")
			assert.True(t, set.Members[0].ViaOptional, "and the path records the optional form")
		})

		t.Run("walks a contributor in another language under its own policy", func(t *testing.T) {
			t.Parallel()

			foreign := coretest.Struct(svcPath, "Message")
			foreign.ID.Lang = "proto"
			foreign.Fields = []*node.Field{field(svcPath, "Message", "id", builtin(intSpelling))}
			foreign.Fields[0].ID.Lang = "proto"
			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{{Ref: &node.TypeRef{Spelling: "Message", Target: foreign.ID}}}
			g := coretest.Frozen(t, coretest.Package(svcPath, host), &node.Package{
				ID:   symbol.Identity{Lang: "proto", Package: svcPath, Kind: symbol.KindPackage},
				Path: []string{svcPath},
				Files: []*node.File{{
					ID:   symbol.Identity{Lang: "proto", Package: svcPath, Name: "m.proto", Kind: symbol.KindFile},
					Path: "m.proto", Decls: node.Symbols{foreign},
				}},
			})
			asked := map[symbol.Lang]bool{}
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(scripted(), v, func(lang symbol.Lang) rules.SourceRules {
				asked[lang] = true
				return rules.Absent(lang)
			})
			set, _ := b.MembersOf(host)
			assert.True(t, asked["proto"], "the contributor's language is asked for its policy")
			assert.Equal(t, names(set), []string{"id"}, "and its declared members arrive under it")
		})

		t.Run("refuses a symbol that is not a type", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, is := b.MembersOf(coretest.Function(svcPath, "F"))
			assert.False(t, is, "a function has no members")
			assert.Equal(t, rules.GapConflict.String(), "conflict", "a reason spells")
			assert.Equal(t, rules.GapReason(9).String(), "9", "and an undeclared one numbers")
		})

		t.Run("counts depth from the host and lets a lone deep member through", func(t *testing.T) {
			t.Parallel()

			var decls []symbol.Symbol
			for i := range 4 {
				s := coretest.Struct(svcPath, "L"+string(rune('0'+i)))
				if i > 0 {
					s.Embeds = []*node.Embed{embedding(svcPath, "L"+string(rune('0'+i-1)))}
				}
				decls = append(decls, s)
			}
			decls[0].(*node.Struct).Fields = []*node.Field{field(svcPath, "L0", "deep", builtin(intSpelling))}
			decls[1].(*node.Struct).Fields = []*node.Field{field(svcPath, "L1", "mid", builtin(intSpelling))}
			g := coretest.Frozen(t, coretest.Package(svcPath, decls...))
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote, Depth: 2,
			}}, v, nil)
			set, _ := b.MembersOf(decls[3])
			assert.Equal(t, names(set), []string{"mid"}, "a member two embeds down is within a budget of two")
			assert.Equal(t, set.Members[0].Depth, 2, "and reports its depth")
			whole, _, _ := boundOver(t, g)
			set, _ = whole.MembersOf(decls[3])
			assert.Equal(t, names(set), []string{"mid", "deep"}, "the default budget reaches the deepest")
			assert.Equal(t, set.Members[1].Depth, 3, "three embeds down")
		})

		t.Run("cancels two promotions at one depth and keeps a declared member over them", func(t *testing.T) {
			t.Parallel()

			left := coretest.Struct(svcPath, "Left")
			left.Fields = []*node.Field{field(svcPath, "Left", "shared", builtin(intSpelling))}
			right := coretest.Struct(svcPath, "Right")
			right.Fields = []*node.Field{
				field(svcPath, "Right", "shared", builtin(intSpelling)),
				field(svcPath, "Right", "own", builtin(intSpelling)),
			}
			host := coretest.Struct(svcPath, "Host")
			host.Fields = []*node.Field{field(svcPath, "Host", "own", builtin(strSpelling))}
			host.Embeds = []*node.Embed{embedding(svcPath, "Left"), embedding(svcPath, "Right")}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, left, right, host)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, names(set), []string{"own"}, "two arrivals at one depth cancel; the declared member stands")
			assert.Length(t, set.Members, 1, "and stands once")
			assert.Equal(t, set.Members[0].Owner, host.ID, "as the host's own")
			assert.True(t, set.Complete(), "a cancellation is the rule, not a gap")
		})

		t.Run("keeps an interface's fields arriving twice as two candidates", func(t *testing.T) {
			t.Parallel()

			left := iface(svcPath, "Left")
			left.Fields = []*node.Field{field(svcPath, "Left", "x", builtin(intSpelling))}
			right := iface(svcPath, "Right")
			right.Fields = []*node.Field{field(svcPath, "Right", "x", builtin(intSpelling))}
			both := iface(svcPath, "Both")
			both.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Left", symbol.KindInterface)},
				{Ref: named(svcPath, "Right", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, left, right, both)))
			set, _ := b.MembersOf(both)
			assert.Empty(t, names(set), "a field has no signature to fold on, so two at one depth cancel")
			assert.True(t, set.Complete(), "without a conflict, because neither states a signature")
		})

		t.Run("reports a typed nil as no type", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			for _, sym := range []symbol.Symbol{(*node.Struct)(nil), (*node.Interface)(nil), (*node.Enum)(nil), (*node.Sum)(nil), nil} {
				_, is := b.MembersOf(sym)
				assert.False(t, is, "nothing has no members, and the walk never panics")
			}
		})
	})
}
