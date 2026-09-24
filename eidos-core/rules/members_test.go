// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strconv"
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
		case *node.Embed:
			out = append(out, d.ID.Name)
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

		t.Run("takes the nearest arrival under override and linearise", func(t *testing.T) {
			t.Parallel()

			grand := coretest.Struct(svcPath, "Grand")
			grand.Methods = []*node.Method{method(svcPath, "Grand", "Run")}
			left := coretest.Struct(svcPath, "Left")
			left.Extends = []*node.TypeRef{named(svcPath, "Grand", symbol.KindStruct)}
			right := coretest.Struct(svcPath, "Right")
			right.Methods = []*node.Method{method(svcPath, "Right", "Run")}
			both := coretest.Struct(svcPath, "Both")
			both.Extends = []*node.TypeRef{
				named(svcPath, "Left", symbol.KindStruct), named(svcPath, "Right", symbol.KindStruct),
			}
			g := coretest.Frozen(t, coretest.Package(svcPath, grand, left, right, both))
			for _, rule := range []rules.Shadowing{rules.ShadowOverride, rules.ShadowLinearise} {
				v, _, _ := viewOver(t, g)
				b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
					Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rule,
				}}, v, nil)
				set, _ := b.MembersOf(both)
				assert.Equal(t, names(set), []string{"Run"}, rule.String()+" keeps one Run")
				assert.Equal(t, set.Members[0].Owner, right.ID,
					rule.String()+" takes Right's, one level up, over Grand's, which the walk reaches first")
			}
		})

		t.Run("keeps an inherited overload beside a declared one under override", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			overload := method(svcPath, baseName, "Run")
			overload.Params[0].Type = builtin(strSpelling)
			base.Methods = []*node.Method{overload, method(svcPath, baseName, "Stop")}
			derived := coretest.Struct(svcPath, derivedName)
			derived.Extends = []*node.TypeRef{named(svcPath, baseName, symbol.KindStruct)}
			derived.Methods = []*node.Method{method(svcPath, derivedName, "Run"), method(svcPath, derivedName, "Stop")}
			g := coretest.Frozen(t, coretest.Package(svcPath, base, derived))
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowOverride,
			}}, v, nil)
			set, _ := b.MembersOf(derived)
			assert.Equal(t, names(set), []string{"Run", "Run", "Stop"},
				"Run(string) overloads the declared Run(int), and the declared Stop overrides Base's")
			assert.Equal(t, set.Members[1].Owner, base.ID, "the overload is Base's")
			assert.Equal(t, set.Members[2].Owner, derived.ID, "and the overriding Stop is Derived's")
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

		t.Run("binds a generic contributor's own contributors", func(t *testing.T) {
			t.Parallel()

			inner := coretest.Struct(svcPath, "Inner")
			inner.TypeParams = []*node.TypeParam{{Name: "U"}}
			inner.Fields = []*node.Field{field(svcPath, "Inner", "item", builtin("U"))}
			mid := coretest.Struct(svcPath, "Mid")
			mid.TypeParams = []*node.TypeParam{{Name: "T"}}
			innerRef := named(svcPath, "Inner", symbol.KindStruct)
			innerRef.Args = []*node.TypeRef{builtin("T")}
			mid.Embeds = []*node.Embed{{Ref: innerRef}}
			outer := coretest.Struct(svcPath, "Outer")
			midRef := named(svcPath, "Mid", symbol.KindStruct)
			midRef.Args = []*node.TypeRef{builtin(intSpelling)}
			outer.Embeds = []*node.Embed{{Ref: midRef}}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, inner, mid, outer)))
			set, _ := b.MembersOf(outer)
			assert.Equal(t, names(set), []string{"item"}, "the member two generic embeds down arrives")
			assert.Equal(t, set.Members[0].Symbol.(*node.Field).Type.Spelling, intSpelling,
				"bound to the outer argument through the middle level")
			assert.Equal(t, innerRef.Args[0].Spelling, "T", "and the graph's own reference is unchanged")
		})

		t.Run("records an embedded field and selects it over a deeper name", func(t *testing.T) {
			t.Parallel()

			leaf := coretest.Struct(svcPath, "Leaf")
			leaf.Fields = []*node.Field{field(svcPath, "Leaf", "Mid", builtin(intSpelling))}
			mid := coretest.Struct(svcPath, "Mid")
			mid.Embeds = []*node.Embed{namedEmbed(svcPath, "Mid", "Leaf", symbol.KindStruct)}
			top := coretest.Struct(svcPath, "Top")
			top.Embeds = []*node.Embed{namedEmbed(svcPath, "Top", "Mid", symbol.KindStruct)}
			g := coretest.Frozen(t, coretest.Package(svcPath, leaf, mid, top))
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote,
				EmbedsAreFields: true,
			}}, v, nil)
			set, _ := b.MembersOf(top)
			assert.Equal(t, names(set), []string{"Mid", "Leaf"}, "each embedded field is a member")
			assert.Equal(t, set.Members[0].Symbol, symbol.Symbol(top.Embeds[0]),
				"Top's own embedded field Mid is selected over Leaf's field Mid two embeds down")
			assert.Equal(t, set.Members[0].Depth, 0, "at the depth of the type declaring it")
			assert.Equal(t, set.Members[1].Depth, 1, "and Mid's embedded field Leaf promotes one level")

			plain, _, _ := boundOver(t, g)
			set, _ = plain.MembersOf(top)
			assert.Equal(t, names(set), []string{"Mid"}, "a policy without the flag records no embedded field")
			assert.Equal(t, set.Members[0].Depth, 2, "so the name selects the field two embeds down")
		})

		t.Run("records no embedded field for an interface", func(t *testing.T) {
			t.Parallel()

			reader := iface(svcPath, "Reader")
			reader.Methods = []*node.Method{method(svcPath, "Reader", "Read")}
			both := iface(svcPath, "Both")
			both.Embeds = []*node.Embed{namedEmbed(svcPath, "Both", "Reader", symbol.KindInterface)}
			g := coretest.Frozen(t, coretest.Package(svcPath, reader, both))
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote,
				EmbedsAreFields: true,
			}}, v, nil)
			set, _ := b.MembersOf(both)
			assert.Equal(t, names(set), []string{"Read"}, "an embedded interface declares no field")
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
			assert.Equal(t, set.Gaps[0].Host, clash.ID, "the conflict names the host")
			assert.Equal(t, set.Gaps[0].Contributor, clash.Embeds[1].Ref,
				"and the contributor the second signature arrived by")
		})

		t.Run("compares signatures by target and variadic kind", func(t *testing.T) {
			t.Parallel()

			// Each pair of interfaces spells one Read(Buf) alike and
			// differs in what the parameter names or how it collects.
			reading := func(host, pkg string, variadic symbol.Variadic) *node.Interface {
				i := iface(svcPath, host)
				m := coretest.Method(svcPath, host, "Read")
				m.Params = []*node.Param{{
					Name: "p", Type: named(pkg, "Buf", symbol.KindStruct), Variadic: variadic,
				}}
				i.Methods = []*node.Method{m}
				return i
			}
			local := reading("Local", svcPath, symbol.VariadicNone)
			foreign := reading("Foreign", depPath, symbol.VariadicNone)
			spread := reading("Spread", svcPath, symbol.VariadicPositional)
			byPackage := iface(svcPath, "ByPackage")
			byPackage.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Foreign", symbol.KindInterface)},
			}
			byVariadic := iface(svcPath, "ByVariadic")
			byVariadic.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Spread", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t,
				coretest.Package(svcPath, local, foreign, spread, byPackage, byVariadic)))
			for _, host := range []*node.Interface{byPackage, byVariadic} {
				set, _ := b.MembersOf(host)
				assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
					host.Name+": one spelling naming two signatures conflicts")
			}
		})

		t.Run("compares a structural parameter by its elements", func(t *testing.T) {
			t.Parallel()

			// Each interface's Read takes a []Buf, spelled alike and
			// naming the Buf of its own package.
			listOf := func(host, pkg string) *node.Interface {
				i := iface(svcPath, host)
				m := coretest.Method(svcPath, host, "Read")
				m.Params = []*node.Param{{Name: "p", Type: &node.TypeRef{
					Spelling: "[]Buf", Form: symbol.FormList,
					Elems: []*node.TypeRef{named(pkg, "Buf", symbol.KindStruct)},
				}}}
				i.Methods = []*node.Method{m}
				return i
			}
			local, twin, foreign := listOf("Local", svcPath), listOf("Twin", svcPath), listOf("Foreign", depPath)
			same := iface(svcPath, "Same")
			same.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Twin", symbol.KindInterface)},
			}
			apart := iface(svcPath, "Apart")
			apart.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Foreign", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t,
				coretest.Package(svcPath, local, twin, foreign, same, apart)))
			set, _ := b.MembersOf(same)
			assert.Equal(t, names(set), []string{"Read"}, "one element type twice is one member")
			assert.True(t, set.Complete(), "and no gap")
			set, _ = b.MembersOf(apart)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
				"two element types under one spelling conflict")
		})

		t.Run("names the host that lists a deeper conflicting contributor", func(t *testing.T) {
			t.Parallel()

			reader := iface(svcPath, "Reader")
			reader.Methods = []*node.Method{method(svcPath, "Reader", "Read")}
			other := iface(svcPath, "Other")
			other.Methods = []*node.Method{coretest.Method(svcPath, "Other", "Read")}
			mid := iface(svcPath, "Mid")
			mid.Embeds = []*node.Embed{{Ref: named(svcPath, "Other", symbol.KindInterface)}}
			deep := iface(svcPath, "Deep")
			deep.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Reader", symbol.KindInterface)},
				{Ref: named(svcPath, "Mid", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, reader, other, mid, deep)))
			set, _ := b.MembersOf(deep)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
				"Other's Read conflicts with Reader's two contributors down")
			assert.Equal(t, set.Gaps[0].Host, mid.ID, "the gap names Mid, whose list holds Other")
			assert.Equal(t, set.Gaps[0].Contributor, mid.Embeds[0].Ref, "and Mid's reference to it")
		})

		t.Run("gives each member a path of its own", func(t *testing.T) {
			t.Parallel()

			var decls []symbol.Symbol
			for i := range 4 {
				s := coretest.Struct(svcPath, "L"+string(rune('0'+i)))
				if i > 0 {
					s.Embeds = []*node.Embed{embedding(svcPath, "L"+string(rune('0'+i-1)))}
				}
				decls = append(decls, s)
			}
			decls[0].(*node.Struct).Fields = []*node.Field{
				field(svcPath, "L0", "first", builtin(intSpelling)),
				field(svcPath, "L0", "second", builtin(intSpelling)),
			}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, decls...)))
			set, _ := b.MembersOf(decls[3])
			assert.Equal(t, names(set), []string{"first", "second"}, "both fields arrive three embeds down")
			for _, m := range set.Members {
				assert.Equal(t, cap(m.Through), len(m.Through),
					"a path shared by one visit's members leaves no room an append could write into")
			}
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

// A promoted method set is the walk's common case: a struct
// embedding a type whose methods arrive once each.
func BenchmarkMembers(b *testing.B) {
	const methods = 20
	base := coretest.Struct(svcPath, baseName)
	for i := range methods {
		base.Methods = append(base.Methods, method(svcPath, baseName, "M"+strconv.Itoa(i)))
	}
	derived := coretest.Struct(svcPath, derivedName)
	derived.Embeds = []*node.Embed{embedding(svcPath, baseName)}
	bound, _, _ := boundOver(b, coretest.Frozen(b, coretest.Package(svcPath, base, derived)))

	b.ReportAllocs()
	for b.Loop() {
		set, _ := bound.MembersOf(derived)
		if len(set.Members) != methods {
			b.Fatalf("MembersOf returned %d members, want %d", len(set.Members), methods)
		}
	}
}
