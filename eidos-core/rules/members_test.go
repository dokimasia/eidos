// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// TestMembers pins each rule, each gap and the provenance of the member
// walk. The walk belongs to the kernel, and the shadowing belongs to the
// language.
func TestMembers(t *testing.T) {
	t.Parallel()

	t.Run("MembersOf", func(t *testing.T) {
		t.Parallel()

		t.Run("walks embeds under promotion with provenance", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			derived, held := b.View().Lookup(coretest.ID(svcPath, derivedName, symbol.KindStruct))
			assert.True(t, held, "the fixture declares the embedding struct")
			set, is := b.MembersOf(derived)
			assert.True(t, is, "a struct walks")
			assert.Equal(t, names(set), []string{"name", "id", "ID"},
				"the declared member first, then the promoted ones in the contributor's order")
			assert.True(t, set.Complete(), "every contributor yielded")
			assert.Equal(t, set.Members[0].Depth, 0, "a declared member is at depth zero")
			assert.Empty(t, set.Members[0].Through, "through nothing")
			assert.Equal(t, set.Members[1].Depth, 1, "a promoted one at depth one")
			base := coretest.ID(svcPath, baseName, symbol.KindStruct)
			assert.Equal(t, set.Members[1].Through, []symbol.Identity{base},
				"through the embed it arrived by")
			assert.Equal(t, set.Members[1].Owner, coretest.ID(svcPath, baseName, symbol.KindStruct),
				"owned by the declaration that declared it")
		})

		tests := []struct {
			name string
			give func(*node.Struct)
			want []string
		}{
			{
				name: "lists a computed field as a member",
				give: func(s *node.Struct) {
					f := field(svcPath, derivedName, "[key]", builtin(intSpelling))
					f.Computed = true
					s.Fields = append(s.Fields, f)
				},
				want: []string{"id", "[key]"},
			},
			{
				name: "lists a computed method as a member",
				give: func(s *node.Struct) {
					m := method(svcPath, derivedName, "[Symbol.iterator]")
					m.Computed = true
					s.Methods = append(s.Methods, m)
				},
				want: []string{"id", "[Symbol.iterator]"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := coretest.Struct(svcPath, derivedName)
				s.Fields = []*node.Field{field(svcPath, derivedName, "id", builtin(intSpelling))}
				tt.give(s)
				b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, s)))
				set, _ := b.MembersOf(s)
				assert.Equal(t, names(set), tt.want, "the walk lists the computed member under its name as written")
				assert.True(t, set.Complete(), "the walk of a type with a computed member has no gap")
			})
		}

		contracts := []struct {
			name string
			give func(implements []*node.TypeRef) symbol.Symbol
		}{
			{
				name: "walks the contracts of an enum under implements",
				give: func(implements []*node.TypeRef) symbol.Symbol {
					id := coretest.ID(svcPath, "Phase", symbol.KindEnum)
					return &node.Enum{ID: id, Name: "Phase", Implements: implements}
				},
			},
			{
				name: "walks the contracts of a sum under implements",
				give: func(implements []*node.TypeRef) symbol.Symbol {
					id := coretest.ID(svcPath, "Shape", symbol.KindSum)
					return &node.Sum{ID: id, Name: "Shape", Implements: implements}
				},
			},
		}
		for _, tt := range contracts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				stringer := &node.Interface{
					ID: coretest.ID(svcPath, "Stringer", symbol.KindInterface), Name: "Stringer",
					Methods: []*node.Method{method(svcPath, "Stringer", "String")},
				}
				decl := tt.give([]*node.TypeRef{named(svcPath, "Stringer", symbol.KindInterface)})
				v, _, _ := viewOver(t, coretest.Frozen(t, coretest.Package(svcPath, stringer, decl)))
				b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
					Contributes: []rules.Contribution{rules.ContributesImplements}, Shadowing: rules.ShadowOverride,
				}}, v, nil)
				set, is := b.MembersOf(decl)
				assert.True(t, is, "a variant set walks")
				assert.Equal(t, names(set), []string{"String"}, "the method of the contract arrives through implements")
			})
		}

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
						"the first arrival takes the name")
				}
			}
		})

		t.Run("takes the nearest arrival under ShadowOverride", func(t *testing.T) {
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
			v, _, _ := viewOver(t, coretest.Frozen(t, coretest.Package(svcPath, grand, left, right, both)))
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowOverride,
			}}, v, nil)
			set, _ := b.MembersOf(both)
			assert.Equal(t, names(set), []string{"Run"}, "one Run remains")
			assert.Equal(t, set.Members[0].Owner, right.ID,
				"Right's Run, one level up, over Grand's, which the walk meets first")
		})

		// extending returns a struct that extends the structs of bases in
		// their order, with a method Run where run is true.
		extending := func(name string, run bool, bases ...string) *node.Struct {
			s := coretest.Struct(svcPath, name)
			for _, base := range bases {
				s.Extends = append(s.Extends, named(svcPath, base, symbol.KindStruct))
			}
			if run {
				s.Methods = []*node.Method{method(svcPath, name, "Run")}
			}
			return s
		}
		linearised := []struct {
			name      string
			give      []*node.Struct
			wantOwner string
			wantGaps  []rules.GapReason
		}{
			{
				name: "takes the arrival of the first type in the C3 order under ShadowLinearise",
				give: []*node.Struct{
					extending("Grand", true), extending("Left", false, "Grand"), extending("Right", true),
					extending("Both", false, "Left", "Right"),
				},
				wantOwner: "Grand",
			},
			{
				name: "takes the arrival of a type before the shared base of a diamond under ShadowLinearise",
				give: []*node.Struct{
					extending("A", true), extending("B", false, "A"), extending("C", true, "A"),
					extending("D", false, "B", "C"),
				},
				wantOwner: "C",
			},
			{
				name: "takes the shared base of a diamond once under ShadowLinearise",
				give: []*node.Struct{
					extending("A", true), extending("B", false, "A"), extending("C", false, "A"),
					extending("D", false, "B", "C"),
				},
				wantOwner: "A",
			},
			{
				name: "takes the nearest arrival for a hierarchy without a C3 order under ShadowLinearise",
				give: []*node.Struct{
					extending("G", true), extending("A", false, "G"), extending("X", true, "A"),
					extending("Z", false, "A", "X"),
				},
				wantOwner: "X",
			},
			{
				name: "takes the nearest arrival for a cyclic hierarchy under ShadowLinearise",
				give: []*node.Struct{
					extending("A", false, "B"), extending("B", true, "A"), extending("C", true),
					extending("R", false, "A", "C"),
				},
				wantOwner: "C", wantGaps: []rules.GapReason{rules.GapCyclic},
			},
		}
		for _, tt := range linearised {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				decls := make([]symbol.Symbol, 0, len(tt.give))
				for _, s := range tt.give {
					decls = append(decls, s)
				}
				v, _, _ := viewOver(t, coretest.Frozen(t, coretest.Package(svcPath, decls...)))
				b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
					Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowLinearise,
				}}, v, nil)
				set, _ := b.MembersOf(tt.give[len(tt.give)-1])
				assert.Equal(t, names(set), []string{"Run"}, "one Run remains")
				assert.Equal(t, set.Members[0].Owner, coretest.ID(svcPath, tt.wantOwner, symbol.KindStruct),
					"the Run of the type that Python finds first")
				assert.Equal(t, reasons(set), tt.wantGaps, "the gaps of the walk", assert.EquateEmpty())
			})
		}

		t.Run("keeps the overloads of the first type in the C3 order under ShadowLinearise", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			wide := method(svcPath, baseName, "Run")
			wide.ID.Disc = strSpelling
			wide.Params[0].Type = builtin(strSpelling)
			base.Methods = []*node.Method{method(svcPath, baseName, "Run"), wide}
			other := extending("Other", true)
			derived := extending(derivedName, false, baseName, "Other")
			v, _, _ := viewOver(t, coretest.Frozen(t, coretest.Package(svcPath, base, other, derived)))
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowLinearise,
			}}, v, nil)
			set, _ := b.MembersOf(derived)
			assert.Equal(t, names(set), []string{"Run", "Run"}, "both overloads of Base remain")
			assert.Equal(t, set.Members[0].Owner, base.ID, "the first overload is Base's")
			assert.Equal(t, set.Members[1].Owner, base.ID, "the second overload is Base's")
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
			alias.Target = &node.TypeRef{Spelling: "struct{ id int }", Form: symbol.FormInline}
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
			}, "an untargeted reference, a missing target, an alias of an inline body, and a loop each name their reason")
			assert.False(t, set.Complete(), "so the set is incomplete")
			assert.Equal(t, set.Gaps[0].Host, coretest.ID(svcPath, "Host", symbol.KindStruct),
				"each gap names the host that wrote the contributor")
		})

		beneath := []struct {
			name       string
			giveTarget *node.TypeRef
			wantGaps   []rules.GapReason
		}{
			{
				name: "promotes the methods of an embedded defined type over an array",
				giveTarget: &node.TypeRef{
					Spelling: "[16]byte", Form: symbol.FormArray, Length: 16, Elems: []*node.TypeRef{builtin("byte")},
				},
			},
			{
				name: "promotes the methods of an embedded defined type over a list",
				giveTarget: &node.TypeRef{
					Spelling: "[]byte", Form: symbol.FormList, Elems: []*node.TypeRef{builtin("byte")},
				},
			},
			{
				name:       "promotes the methods of an embedded defined type over an integer",
				giveTarget: builtin(intSpelling),
			},
			{
				name:       "promotes the methods of an embedded defined type over a string",
				giveTarget: builtin(strSpelling),
			},
			{
				name:       "reports GapUnresolved for a defined type over a name that the language does not classify",
				giveTarget: builtin("elsewhere.Thing"),
				wantGaps:   []rules.GapReason{rules.GapUnresolved},
			},
			{
				name:       "reports GapUnresolved for a defined type over a declaration that the view does not contain",
				giveTarget: named(svcPath, "Missing", symbol.KindStruct),
				wantGaps:   []rules.GapReason{rules.GapUnresolved},
			},
		}
		for _, tt := range beneath {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				key := coretest.Alias(svcPath, keyName)
				key.Defined, key.Target = true, tt.giveTarget
				keyed := coretest.Struct(svcPath, keyedName)
				keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
				pkg := coretest.Package(svcPath, key, method(svcPath, keyName, "Len"), keyed)
				b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
				set, _ := b.MembersOf(keyed)
				assert.Equal(t, names(set), []string{"Len"}, "the method that the package declares for Key promotes")
				expect.Equal(t, set.Members[0].Owner, key.ID, "Key owns the method")
				expect.Equal(t, set.Members[0].Through, []symbol.Identity{key.ID},
					"the method arrives through the embed of Key")
				expect.Equal(t, reasons(set), tt.wantGaps,
					"the walk reports a gap where it cannot follow the type under Key", assert.EquateEmpty())
			})
		}

		t.Run("promotes the fields and the embeds of the struct under an embedded defined type", func(t *testing.T) {
			t.Parallel()

			inner := coretest.Struct(svcPath, "Inner")
			inner.Methods = []*node.Method{method(svcPath, "Inner", "Close")}
			base := coretest.Struct(svcPath, baseName)
			base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
			base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
			base.Embeds = []*node.Embed{embedding(svcPath, "Inner")}
			key := coretest.Alias(svcPath, keyName)
			key.Defined, key.Target = true, named(svcPath, baseName, symbol.KindStruct)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			pkg := coretest.Package(svcPath, inner, base, key, method(svcPath, keyName, "Reset"), keyed)
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, names(set), []string{"Reset", "id", "Close"},
				"Key promotes its method, the field of Base and the method of Inner, without the method of Base")
			expect.Equal(t, set.Members[1].Owner, base.ID, "Base declares the field")
			expect.Equal(t, set.Members[1].Through, []symbol.Identity{key.ID},
				"the field arrives through the embed of Key")
			expect.Equal(t, set.Members[1].Depth, 1, "the field is at the depth of Key")
			expect.Equal(t, set.Members[2].Depth, 2, "the method of Inner is one embed further down")
			expect.True(t, set.Complete(), "the walk follows every contributor")
		})

		t.Run("promotes the methods of the interface under an embedded defined type", func(t *testing.T) {
			t.Parallel()

			reader := iface(svcPath, "Reader")
			reader.Methods = []*node.Method{method(svcPath, "Reader", "Read")}
			key := coretest.Alias(svcPath, keyName)
			key.Defined, key.Target = true, named(svcPath, "Reader", symbol.KindInterface)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, reader, key, keyed)))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, names(set), []string{"Read"},
				"a defined type over an interface has the methods of the interface")
			expect.Equal(t, set.Members[0].Owner, reader.ID, "Reader declares the method")
			expect.True(t, set.Complete(), "the walk follows every contributor")
		})

		t.Run("promotes no method of the enum under an embedded defined type", func(t *testing.T) {
			t.Parallel()

			phase := coretest.Enum(svcPath, "Phase", "Open")
			phase.Methods = []*node.Method{method(svcPath, "Phase", "String")}
			key := coretest.Alias(svcPath, keyName)
			key.Defined, key.Target = true, named(svcPath, "Phase", symbol.KindEnum)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			pkg := coretest.Package(svcPath, phase, key, method(svcPath, keyName, "Len"), keyed)
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, names(set), []string{"Len"}, "a defined type has none of the methods of the type under it")
			expect.True(t, set.Complete(), "an enum declares no field or embed for a defined type over it")
		})

		t.Run("promotes the members of the target of an embedded transparent alias", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
			base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
			key := coretest.Alias(svcPath, keyName)
			key.Target = named(svcPath, baseName, symbol.KindStruct)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			pkg := coretest.Package(svcPath, base, key, method(svcPath, keyName, "Reset"), keyed)
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, names(set), []string{"Reset", "id", "ID"},
				"the alias promotes the method that the package declares for it and every member of Base")
			expect.Equal(t, set.Members[2].Owner, base.ID, "Base declares its method")
			expect.Equal(t, set.Members[2].Through, []symbol.Identity{key.ID},
				"the method of Base arrives through the embed of the alias")
			expect.True(t, set.Complete(), "the walk follows every contributor")
		})

		t.Run("promotes the members of a defined type that an embedded transparent alias names", func(t *testing.T) {
			t.Parallel()

			base := coretest.Struct(svcPath, baseName)
			base.Fields = []*node.Field{field(svcPath, baseName, "id", builtin(intSpelling))}
			base.Methods = []*node.Method{method(svcPath, baseName, "ID")}
			copied := coretest.Alias(svcPath, "Copy")
			copied.Defined, copied.Target = true, named(svcPath, baseName, symbol.KindStruct)
			key := coretest.Alias(svcPath, keyName)
			key.Target = named(svcPath, "Copy", symbol.KindAlias)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			pkg := coretest.Package(svcPath, base, copied, method(svcPath, "Copy", "Reset"), key, keyed)
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, names(set), []string{"Reset", "id"},
				"the alias promotes the method of Copy and the field of Base, without the method of Base")
			expect.Equal(t, set.Members[0].Owner, copied.ID, "Copy owns its method")
			expect.Equal(t, set.Members[1].Through, []symbol.Identity{key.ID},
				"the field arrives through the embed of the alias")
		})

		// boxedWalk walks Holder, which embeds the alias Boxed[U] = Box[U]
		// with args, where Box[T] declares the field item of type T.
		boxedWalk := func(t *testing.T, args ...*node.TypeRef) rules.MemberSet {
			t.Helper()

			box, boxT := genericStruct(svcPath, "Box", "T")
			box.Fields = []*node.Field{field(svcPath, "Box", "item", boxT)}
			boxed := coretest.Alias(svcPath, "Boxed")
			param := coretest.MemberID(svcPath, "Boxed", "U", symbol.KindTypeParam)
			boxed.TypeParams = []*node.TypeParam{{ID: param, Name: "U"}}
			boxed.Target = named(svcPath, "Box", symbol.KindStruct)
			boxed.Target.Args = []*node.TypeRef{{Spelling: "U", Target: param}}
			holder := coretest.Struct(svcPath, "Holder")
			ref := named(svcPath, "Boxed", symbol.KindAlias)
			ref.Args = args
			holder.Embeds = []*node.Embed{{Ref: ref}}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, box, boxed, holder)))
			set, _ := b.MembersOf(holder)
			return set
		}

		t.Run("binds the arguments of an embedded generic alias", func(t *testing.T) {
			t.Parallel()

			set := boxedWalk(t, builtin(intSpelling))
			assert.Equal(t, names(set), []string{"item"}, "the field of Box arrives through the alias")
			assert.Equal(t, set.Members[0].Symbol.(*node.Field).Type.Spelling, intSpelling,
				"with the argument of the alias in place of the parameter of Box")
		})

		t.Run("reports GapGeneric for an alias whose arguments differ in number from its parameters",
			func(t *testing.T) {
				t.Parallel()

				set := boxedWalk(t, builtin(intSpelling), builtin(strSpelling))
				assert.Equal(t, reasons(set), []rules.GapReason{rules.GapGeneric},
					"two arguments do not bind one parameter")
			})

		spelled := []struct {
			name       string
			giveTarget *node.TypeRef
			want       []rules.GapReason
		}{
			{
				name:       "reports GapUnresolved for a transparent alias of a reference without a target",
				giveTarget: builtin(intSpelling),
				want:       []rules.GapReason{rules.GapUnresolved},
			},
			{
				name: "reports GapNotMembered for a transparent alias of a list",
				giveTarget: &node.TypeRef{
					Spelling: "[]byte", Form: symbol.FormList, Elems: []*node.TypeRef{builtin("byte")},
				},
				want: []rules.GapReason{rules.GapNotMembered},
			},
			{
				name:       "reports GapNotMembered for a transparent alias of a function",
				giveTarget: named(svcPath, "F", symbol.KindFunction),
				want:       []rules.GapReason{rules.GapNotMembered},
			},
			{
				name: "reports GapNotMembered for an alias without a target",
				want: []rules.GapReason{rules.GapNotMembered},
			},
		}
		for _, tt := range spelled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				key := coretest.Alias(svcPath, keyName)
				key.Target = tt.giveTarget
				keyed := coretest.Struct(svcPath, keyedName)
				keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
				pkg := coretest.Package(svcPath, coretest.Function(svcPath, "F"), key, keyed)
				b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
				set, _ := b.MembersOf(keyed)
				assert.Equal(t, reasons(set), tt.want, "the walk does not read the members of the target of the alias")
			})
		}

		t.Run("reports GapNotMembered for a contributor whose declaration is a function", func(t *testing.T) {
			t.Parallel()

			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{embedOf(svcPath, "F", symbol.KindFunction)}
			pkg := coretest.Package(svcPath, coretest.Function(svcPath, "F"), host)
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapNotMembered}, "a function has no members")
		})

		t.Run("reports GapCyclic for a defined type over the walked struct", func(t *testing.T) {
			t.Parallel()

			tree := coretest.Struct(svcPath, "Tree")
			tree.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			key := coretest.Alias(svcPath, keyName)
			key.Defined, key.Target = true, named(svcPath, "Tree", symbol.KindStruct)
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, tree, key)))
			set, _ := b.MembersOf(tree)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapCyclic}, "the struct under Key is on the path")
		})

		t.Run("reports GapCyclic for a chain of more than eight aliases", func(t *testing.T) {
			t.Parallel()

			decls := make([]symbol.Symbol, 0, 11)
			decls = append(decls, coretest.Struct(svcPath, baseName))
			for i := range 9 {
				a := coretest.Alias(svcPath, "A"+string(rune('0'+i)))
				a.Target = named(svcPath, "A"+string(rune('0'+i+1)), symbol.KindAlias)
				if i == 8 {
					a.Target = named(svcPath, baseName, symbol.KindStruct)
				}
				decls = append(decls, a)
			}
			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{embedOf(svcPath, "A0", symbol.KindAlias)}
			decls = append(decls, host)
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, decls...)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapCyclic}, "the walk follows eight aliases at most")
		})

		t.Run("reports GapGeneric for an alias of a generic struct without its arguments", func(t *testing.T) {
			t.Parallel()

			box, boxT := genericStruct(svcPath, "Box", "T")
			box.Fields = []*node.Field{field(svcPath, "Box", "item", boxT)}
			key := coretest.Alias(svcPath, keyName)
			key.Target = named(svcPath, "Box", symbol.KindStruct)
			keyed := coretest.Struct(svcPath, keyedName)
			keyed.Embeds = []*node.Embed{embedOf(svcPath, keyName, symbol.KindAlias)}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, box, key, keyed)))
			set, _ := b.MembersOf(keyed)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapGeneric},
				"the target of Key does not bind the parameter of Box")
		})

		t.Run("binds the arguments of an embedded generic defined type in its methods", func(t *testing.T) {
			t.Parallel()

			param := coretest.MemberID(svcPath, "Boxed", "U", symbol.KindTypeParam)
			boxed := coretest.Alias(svcPath, "Boxed")
			boxed.Defined, boxed.TypeParams = true, []*node.TypeParam{{ID: param, Name: "U"}}
			boxed.Target = &node.TypeRef{
				Spelling: "[]U", Form: symbol.FormList, Elems: []*node.TypeRef{{Spelling: "U", Target: param}},
			}
			get := coretest.Method(svcPath, "Boxed", "Get")
			get.Returns = []*node.Return{{Type: &node.TypeRef{Spelling: "U", Target: param}}}
			holder := coretest.Struct(svcPath, "Holder")
			ref := named(svcPath, "Boxed", symbol.KindAlias)
			ref.Args = []*node.TypeRef{builtin(intSpelling)}
			holder.Embeds = []*node.Embed{{Ref: ref}}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, boxed, get, holder)))
			set, _ := b.MembersOf(holder)
			assert.Equal(t, names(set), []string{"Get"}, "the method of Boxed promotes")
			assert.Equal(t, set.Members[0].Symbol.(*node.Method).Returns[0].Type.Spelling, intSpelling,
				"with the argument of the embed in place of the parameter of Boxed")
			assert.NotEqual(t, set.Members[0].Symbol, symbol.Symbol(get), "on a copy, never the graph's node",
				assert.ByIdentity())
		})

		t.Run("walks an alias of another language under the policy of its language", func(t *testing.T) {
			t.Parallel()

			foreign := coretest.Struct(svcPath, "Message")
			foreign.ID.Lang = "proto"
			foreign.Fields = []*node.Field{field(svcPath, "Message", "id", builtin(intSpelling))}
			foreign.Fields[0].ID.Lang = "proto"
			alias := &node.Alias{
				ID:     symbol.Identity{Lang: "proto", Package: svcPath, Name: "Spelled", Kind: symbol.KindAlias},
				Name:   "Spelled",
				Target: &node.TypeRef{Spelling: "Message", Target: foreign.ID},
			}
			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{{Ref: &node.TypeRef{Spelling: "Spelled", Target: alias.ID}}}
			g := coretest.Frozen(t, coretest.Package(svcPath, host), &node.Package{
				ID:   symbol.Identity{Lang: "proto", Package: svcPath, Kind: symbol.KindPackage},
				Path: []string{svcPath},
				Files: []*node.File{{
					ID:   symbol.Identity{Lang: "proto", Package: svcPath, Name: "m.proto", Kind: symbol.KindFile},
					Path: "m.proto", Decls: node.Symbols{alias, foreign},
				}},
			})
			var asked []symbol.Lang
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(scripted(), v, func(lang symbol.Lang) rules.SourceRules {
				asked = append(asked, lang)
				return rules.Absent(lang)
			})
			set, _ := b.MembersOf(host)
			assert.Contains(t, asked, symbol.Lang("proto"), "the language of the alias is asked for its policy")
			assert.Equal(t, names(set), []string{"id"}, "and the field of its target arrives under it")
		})

		t.Run("reports GapCyclic for a chain of aliases that names its first alias", func(t *testing.T) {
			t.Parallel()

			first := coretest.Alias(svcPath, "First")
			first.Target = named(svcPath, "Second", symbol.KindAlias)
			second := coretest.Alias(svcPath, "Second")
			second.Target = named(svcPath, "First", symbol.KindAlias)
			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{embedOf(svcPath, "First", symbol.KindAlias)}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, first, second, host)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapCyclic},
				"the target of Second is the alias on the path")
		})

		t.Run("orders the target of an alias base in the C3 order under ShadowLinearise", func(t *testing.T) {
			t.Parallel()

			other, foo := extending("Other", true), extending("Foo", true)
			base := coretest.Alias(svcPath, baseName)
			base.Target = named(svcPath, "Foo", symbol.KindStruct)
			both := coretest.Struct(svcPath, "Both")
			both.Extends = []*node.TypeRef{
				named(svcPath, "Other", symbol.KindStruct), named(svcPath, baseName, symbol.KindAlias),
			}
			v, _, _ := viewOver(t, coretest.Frozen(t, coretest.Package(svcPath, other, foo, base, both)))
			b := rules.NewBound(policy{scripted(), rules.MemberPolicy{
				Contributes: []rules.Contribution{rules.ContributesExtends}, Shadowing: rules.ShadowLinearise,
			}}, v, nil)
			set, _ := b.MembersOf(both)
			assert.Equal(t, names(set), []string{"Run"}, "one Run remains")
			assert.Equal(t, set.Members[0].Owner, other.ID,
				"the Run of Other, which comes before the target of Base in the order of Python")
		})

		t.Run("stops at the depth budget", func(t *testing.T) {
			t.Parallel()

			decls := make([]symbol.Symbol, 0, 4)
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

		t.Run("binds a generic contributor's arguments", func(t *testing.T) {
			t.Parallel()

			box, boxT := genericStruct(svcPath, "Box", "T")
			box.Fields = []*node.Field{field(svcPath, "Box", "item", boxT)}
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
			assert.NotEqual(t, set.Members[0].Symbol, symbol.Symbol(box.Fields[0]), "on a copy, never the graph's node",
				assert.ByIdentity())

			v, _, _ := viewOver(t, g)
			plain := rules.NewBound(nongeneric{scripted()}, v, nil)
			set, _ = plain.MembersOf(holder)
			assert.Equal(t, set.Members[0].Symbol.(*node.Field).Type.Spelling, intSpelling,
				"a language without the generics capability binds the argument too")
		})

		t.Run("reports a gap for a count of arguments that differs from the parameters", func(t *testing.T) {
			t.Parallel()

			box, boxT := genericStruct(svcPath, "Box", "T")
			box.Fields = []*node.Field{field(svcPath, "Box", "item", boxT)}
			holder := coretest.Struct(svcPath, "Holder")
			ref := named(svcPath, "Box", symbol.KindStruct)
			ref.Args = []*node.TypeRef{builtin(intSpelling), builtin(strSpelling)}
			holder.Embeds = []*node.Embed{{Ref: ref}}
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, box, holder)))
			set, _ := b.MembersOf(holder)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapGeneric},
				"two arguments do not bind one parameter")
		})

		t.Run("binds a generic contributor's own contributors", func(t *testing.T) {
			t.Parallel()

			inner, innerU := genericStruct(svcPath, "Inner", "U")
			inner.Fields = []*node.Field{field(svcPath, "Inner", "item", innerU)}
			mid, midT := genericStruct(svcPath, "Mid", "T")
			innerRef := named(svcPath, "Inner", symbol.KindStruct)
			innerRef.Args = []*node.TypeRef{midT}
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

		t.Run("selects an embedded field over a deeper name", func(t *testing.T) {
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

		t.Run("does not record an embedded field for an interface", func(t *testing.T) {
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

		// reading declares an interface whose Read takes one parameter
		// spelled Buf, naming a package's Buf and collecting as variadic
		// states.
		reading := func(host, pkg string, variadic symbol.Variadic) *node.Interface {
			i := iface(svcPath, host)
			m := coretest.Method(svcPath, host, "Read")
			m.Params = []*node.Param{{
				Name: "p", Type: named(pkg, "Buf", symbol.KindStruct), Variadic: variadic,
			}}
			i.Methods = []*node.Method{m}
			return i
		}
		signatures := []struct {
			name     string
			pkg      string
			variadic symbol.Variadic
		}{
			{
				name: "reports GapConflict for parameters of one spelling that name two targets",
				pkg:  depPath, variadic: symbol.VariadicNone,
			},
			{
				name: "reports GapConflict for parameters of one spelling with two variadic kinds",
				pkg:  svcPath, variadic: symbol.VariadicPositional,
			},
		}
		for _, tt := range signatures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				host := iface(svcPath, "Host")
				host.Embeds = []*node.Embed{
					{Ref: named(svcPath, "Local", symbol.KindInterface)},
					{Ref: named(svcPath, "Second", symbol.KindInterface)},
				}
				local := reading("Local", svcPath, symbol.VariadicNone)
				second := reading("Second", tt.pkg, tt.variadic)
				b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, local, second, host)))
				set, _ := b.MembersOf(host)
				assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
					"one spelling of two signatures conflicts")
			})
		}

		t.Run("reports GapConflict for parameters of one spelling with two passings", func(t *testing.T) {
			t.Parallel()

			host := iface(svcPath, "Host")
			host.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Second", symbol.KindInterface)},
			}
			local := reading("Local", svcPath, symbol.VariadicNone)
			second := reading("Second", svcPath, symbol.VariadicNone)
			second.Methods[0].Params[0].Passing = symbol.PassingKeyword
			b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, local, second, host)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
				"a parameter that a call passes by keyword alone differs from one that it passes by position")
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

		// inlineRead declares an interface whose Read takes an inline
		// body of one field typed by a package's Buf.
		inlineRead := func(host, field, pkg string) *node.Interface {
			i := iface(svcPath, host)
			m := coretest.Method(svcPath, host, "Read")
			m.Params = []*node.Param{{Name: "p", Type: &node.TypeRef{
				Spelling: "{" + field + ":Buf}", Form: symbol.FormInline,
				Fields: []*node.Field{{Name: field, Type: named(pkg, "Buf", symbol.KindStruct)}},
			}}}
			i.Methods = []*node.Method{m}
			return i
		}
		inlines := []struct {
			name  string
			field string
			pkg   string
			want  []rules.GapReason
		}{
			{
				name:  "folds two inline parameters of one spelling over one target into one member",
				field: "b", pkg: svcPath, want: []rules.GapReason{},
			},
			{
				name:  "reports GapConflict for inline parameters of one spelling that name two targets",
				field: "b", pkg: depPath, want: []rules.GapReason{rules.GapConflict},
			},
			{
				name:  "reports GapConflict for inline parameters of two spellings",
				field: "c", pkg: svcPath, want: []rules.GapReason{rules.GapConflict},
			},
		}
		for _, tt := range inlines {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				local, second := inlineRead("Local", "b", svcPath), inlineRead("Second", tt.field, tt.pkg)
				host := iface(svcPath, "Host")
				host.Embeds = []*node.Embed{
					{Ref: named(svcPath, "Local", symbol.KindInterface)},
					{Ref: named(svcPath, "Second", symbol.KindInterface)},
				}
				b, _, _ := boundOver(t, coretest.Frozen(t, coretest.Package(svcPath, local, second, host)))
				set, _ := b.MembersOf(host)
				assert.Equal(t, reasons(set), tt.want, "an inline body keys by its spelling and its members' targets")
			})
		}

		t.Run("reports GapConflict for inline parameters whose methods name two targets", func(t *testing.T) {
			t.Parallel()

			// getter declares an interface whose Read takes an inline
			// body of one method returning a package's Buf.
			getter := func(host, pkg string) *node.Interface {
				i := iface(svcPath, host)
				m := coretest.Method(svcPath, host, "Read")
				m.Params = []*node.Param{{Name: "p", Type: &node.TypeRef{
					Spelling: "{Get():Buf}", Form: symbol.FormInline,
					Methods: []*node.Method{{
						Name: "Get", Returns: []*node.Return{{Type: named(pkg, "Buf", symbol.KindStruct)}},
					}},
				}}}
				i.Methods = []*node.Method{m}
				return i
			}
			host := iface(svcPath, "Host")
			host.Embeds = []*node.Embed{
				{Ref: named(svcPath, "Local", symbol.KindInterface)},
				{Ref: named(svcPath, "Foreign", symbol.KindInterface)},
			}
			b, _, _ := boundOver(t, coretest.Frozen(t,
				coretest.Package(svcPath, getter("Local", svcPath), getter("Foreign", depPath), host)))
			set, _ := b.MembersOf(host)
			assert.Equal(t, reasons(set), []rules.GapReason{rules.GapConflict},
				"one spelling whose method returns two targets conflicts")
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
			assert.Equal(t, set.Gaps[0].Host, mid.ID, "the gap names Mid, whose list contains Other")
			assert.Equal(t, set.Gaps[0].Contributor, mid.Embeds[0].Ref, "and Mid's reference to it")
		})

		t.Run("gives each member a path of its own", func(t *testing.T) {
			t.Parallel()

			decls := make([]symbol.Symbol, 0, 4)
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
			var asked []symbol.Lang
			v, _, _ := viewOver(t, g)
			b := rules.NewBound(scripted(), v, func(lang symbol.Lang) rules.SourceRules {
				asked = append(asked, lang)
				return rules.Absent(lang)
			})
			set, _ := b.MembersOf(host)
			assert.Contains(t, asked, symbol.Lang("proto"), "the contributor's language is asked for its policy")
			assert.Equal(t, names(set), []string{"id"}, "and its declared members arrive under it")

			var viaView []symbol.Lang
			inner := rules.NewBound(scripted(), v.WithRules(func(lang symbol.Lang) rules.SourceRules {
				viaView = append(viaView, lang)
				return rules.Absent(lang)
			}), nil)
			_, _ = inner.MembersOf(host)
			assert.Contains(t, viaView, symbol.Lang("proto"),
				"a binding without a resolver of its own resolves through the resolver of the view")
		})

		t.Run("reports false for a symbol that is not a type", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, is := b.MembersOf(coretest.Function(svcPath, "F"))
			assert.False(t, is, "a function has no members")
		})

		t.Run("reports false for an alias", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			_, is := b.MembersOf(coretest.Alias(svcPath, keyName))
			assert.False(t, is, "an alias has no member lists of its own")
		})

		t.Run("counts depth from the host", func(t *testing.T) {
			t.Parallel()

			decls := make([]symbol.Symbol, 0, 4)
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

		t.Run("keeps a declared member over two promotions at one depth", func(t *testing.T) {
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
			assert.Equal(t, names(set), []string{"own"},
				"two arrivals at one depth cancel, and the declared member remains")
			assert.Length(t, set.Members, 1, "once")
			assert.Equal(t, set.Members[0].Owner, host.ID, "as the host's own")
			assert.True(t, set.Complete(), "a cancellation is the rule, not a gap")
		})

		t.Run("returns gaps that a later call leaves unchanged", func(t *testing.T) {
			t.Parallel()

			host := coretest.Struct(svcPath, "Host")
			host.Embeds = []*node.Embed{{Ref: named(svcPath, "Missing", symbol.KindStruct)}}
			other := coretest.Struct(svcPath, "Other")
			other.Embeds = []*node.Embed{{Ref: builtin("Elsewhere")}, embedding(svcPath, baseName)}
			pkg := coretest.Package(svcPath, host, other, coretest.Struct(svcPath, baseName))
			b, _, _ := boundOver(t, coretest.Frozen(t, pkg))
			first, _ := b.MembersOf(host)
			for range 8 {
				b.MembersOf(other)
			}
			assert.Equal(t, reasons(first), []rules.GapReason{rules.GapUnresolved},
				"the first call's gaps are its own, whatever the calls after it found")
			assert.Equal(t, first.Gaps[0].Host, host.ID, "and name its host")
		})

		t.Run("does not return a member of the call before", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			derived, _ := b.View().Lookup(coretest.ID(svcPath, derivedName, symbol.KindStruct))
			lone := coretest.Struct(svcPath, "Lone")
			for range 8 {
				b.MembersOf(derived)
				set, is := b.MembersOf(lone)
				assert.True(t, is, "a struct walks")
				assert.Empty(t, set.Members, "a struct without members has none of the call before")
				assert.True(t, set.Complete(), "and no gap of it")
			}
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

		t.Run("reports false for a typed nil", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			for _, sym := range []symbol.Symbol{(*node.Struct)(nil), (*node.Interface)(nil), (*node.Enum)(nil), (*node.Sum)(nil), nil} {
				_, is := b.MembersOf(sym)
				assert.False(t, is, "a nil declaration has no members")
			}
		})
	})

	t.Run("MemberSet.Complete", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.MemberSet
			want bool
		}{
			{name: "reports true for a set without a gap", give: rules.MemberSet{}, want: true},
			{
				name: "reports false for a set with a gap",
				give: rules.MemberSet{Gaps: []rules.Gap{{Reason: rules.GapCyclic}}},
				want: false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Complete(), tt.want, "Complete reports whether every contributor yielded")
			})
		}
	})

	t.Run("GapReason.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.GapReason
			want string
		}{
			{name: "returns unresolved for GapUnresolved", give: rules.GapUnresolved, want: "unresolved"},
			{name: "returns not-membered for GapNotMembered", give: rules.GapNotMembered, want: "not-membered"},
			{name: "returns cyclic for GapCyclic", give: rules.GapCyclic, want: "cyclic"},
			{name: "returns generic for GapGeneric", give: rules.GapGeneric, want: "generic"},
			{name: "returns conflict for GapConflict", give: rules.GapConflict, want: "conflict"},
			{name: "returns the number of an undeclared reason", give: rules.GapReason(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// A set reports its completeness, and a declared reason spells, without
// allocating in the ordinary run, which runs no benchmark.
func TestMembersAllocs(t *testing.T) {
	set, reason := rules.MemberSet{Gaps: []rules.Gap{{Reason: rules.GapCyclic}}}, rules.GapConflict
	var complete bool
	assert.MaxAllocs(t, func() { complete = set.Complete() }, 0, "MemberSet.Complete allocates nothing")
	assert.False(t, complete, "MemberSet.Complete reports false for a set with a gap")
	var got string
	assert.MaxAllocs(t, func() { got = reason.String() }, 0, "GapReason.String allocates nothing for a declared reason")
	assert.Equal(t, got, "conflict", "GapReason.String spells GapConflict")
}

// BenchmarkMembers measures the completeness of a member set, which a
// generator checks once per walk, and the spelling of a gap reason,
// which a finding prints once per gap.
func BenchmarkMembers(b *testing.B) {
	b.Run("MemberSet.Complete", func(b *testing.B) {
		set := rules.MemberSet{Gaps: []rules.Gap{{Reason: rules.GapCyclic}}}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		complete := true
		for c.Loop() {
			complete = set.Complete()
		}
		assert.False(b, complete, "MemberSet.Complete reports false for a set with a gap")
	})

	b.Run("GapReason.String", func(b *testing.B) {
		reason := rules.GapConflict
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = reason.String()
		}
		assert.Equal(b, got, "conflict", "GapReason.String spells GapConflict")
	})
}

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

// genericStruct returns a struct with one type parameter, and a
// reference that targets the parameter, as the resolution step writes a
// reference to a type parameter in scope.
func genericStruct(path, name, param string) (*node.Struct, *node.TypeRef) {
	s := coretest.Struct(path, name)
	id := coretest.MemberID(path, name, param, symbol.KindTypeParam)
	s.TypeParams = []*node.TypeParam{{ID: id, Name: param}}
	return s, &node.TypeRef{Spelling: param, Target: id}
}
