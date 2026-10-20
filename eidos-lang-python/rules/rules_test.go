// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// rulesAllocs is the budget of one projection pass over the fixture tree
// and the 53 modules of typeshed that its load reads: 34,781 declarations,
// 34,261 of them in typeshed. The pass allocated 41,321 times at 200
// iterations. A memory profile attributes about 49% to TypeOf, 23% to
// MembersOf, 22% to CallableOf and 8% to SamplesOf. The budget is about 5%
// above the count.
const rulesAllocs = 43_500

// returnRolesAllocs is the list of the roles of the returns.
const returnRolesAllocs = 1

// totalID is the function total of the fixture, whose discriminator spells
// its positional-only parameter and its float.
var totalID = symbol.Identity{
	Lang: python.Lang, Package: modelsPkg, Name: "total", Kind: symbol.KindFunction, Disc: "Order/,float",
}

// TestRules pins the member policy and the roles of the rules, and checks the
// rules against the projection contract of the kernel.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the fixture tree", func(t *testing.T) {
			t.Parallel()

			rulestest.RunRulesSuite(t, setup)
		})

		capabilities := []struct {
			name string
			give func(rules.SourceRules) bool
		}{
			{name: "returns rules that project enums", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.EnumRules)
				return is
			}},
			{name: "returns rules that bless well-known types", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.WellKnownRules)
				return is
			}},
			{name: "returns rules that shape the classes of typeshed", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ShapeRules)
				return is
			}},
			{name: "returns rules that project error types", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ErrorTypeRules)
				return is
			}},
			{name: "returns rules that project properties", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.PropertyRules)
				return is
			}},
			{name: "returns rules that project constructors", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ConstructRules)
				return is
			}},
			{name: "returns rules that project equality", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.EqualityRules)
				return is
			}},
			{name: "returns rules that reason about generics", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.GenericsRules)
				return is
			}},
		}
		for _, tt := range capabilities {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, tt.give(pythonrules.New()), "the rules have the capability")
			})
		}
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns python", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pythonrules.New().Lang(), python.Lang, "the language of the rules")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy under which the bases contribute", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pythonrules.New().Members().Contributes, []rules.Contribution{rules.ContributesExtends},
				"a class inherits the members of its bases")
		})

		t.Run("returns linearise as the shadowing rule", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pythonrules.New().Members().Shadowing, rules.ShadowLinearise,
				"the nearest arrival of a name hides the others")
		})

		t.Run("returns a policy under which a class has the members of its bases after its own", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			rush := symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "Rush", Kind: symbol.KindStruct}
			set, is := rules.NewBound(pythonrules.New(), f.view, nil).MembersOf(f.decl(t, rush))
			assert.True(t, is, "a class has members")
			names := make([]string, 0, len(set.Members))
			for _, m := range set.Members {
				decl, declares := m.Symbol.(node.Declaration)
				assert.True(t, declares, "a member is a declaration")
				names = append(names, decl.Identity().Name)
			}
			assert.Equal(t, names, []string{"deadline", "id", "customer", "items", "_", "status", "note"},
				"the field of Rush, then the fields of Order")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies each parameter as an input", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, is := rules.NewBound(pythonrules.New(), f.view, nil).CallableOf(f.decl(t, totalID))
			assert.True(t, is, "the function is callable")
			assert.Length(t, c.Params, 2, "the function has two parameters")
			for _, p := range c.Params {
				assert.Equal(t, p.Role, rules.ParamInput, "the role of "+p.Name)
			}
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want rules.ReturnRole
		}{
			{
				name: "classifies a result of Iterator as a stream",
				give: symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "stream", Kind: symbol.KindFunction},
				want: rules.ReturnStream,
			},
			{
				name: "classifies a result of Generator as a stream",
				give: symbol.Identity{
					Lang:    python.Lang,
					Package: modelsPkg,
					Name:    "produce",
					Kind:    symbol.KindFunction,
				},
				want: rules.ReturnStream,
			},
			{
				name: "classifies a result of AsyncIterator as a stream",
				give: symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "watch", Kind: symbol.KindFunction},
				want: rules.ReturnStream,
			},
			{name: "classifies a result of int as a value", give: totalID, want: rules.ReturnValue},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				c, is := rules.NewBound(pythonrules.New(), f.view, nil).CallableOf(f.decl(t, tt.give))
				assert.True(t, is, "the function is callable")
				assert.Length(t, c.Returns, 1, "the function has one result")
				assert.Equal(t, c.Returns[0].Role, tt.want, "the role of the result")
				assert.Equal(t, c.Errors, rules.ErrorsRaised, "Python raises without declaring")
			})
		}

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, model := pythonrules.New().ReturnRoles([]*node.Return{nil, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "nothing to classify")
			assert.Equal(t, model, rules.ErrorsRaised, "the error model")
		})

		t.Run("classifies a return of a type outside the view as a value", func(t *testing.T) {
			t.Parallel()

			roles, _ := pythonrules.New().ReturnRoles(
				[]*node.Return{{Type: &node.TypeRef{Spelling: goneID.Name, Target: goneID}}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue}, "the view does not contain the type")
		})
	})
}

// TestRulesAllocs checks the ceilings of the methods of rules.go in the
// ordinary run, which does not run benchmarks.
func TestRulesAllocs(t *testing.T) {
	checkAllocs(t, rulesCalls(t))
}

// BenchmarkRules measures each method of rules.go, and the four hot
// projections of the kernel over the fixture tree under their budget. Only
// the benchmark checks the budget of the projections, because the contract
// leaves out the fresh view that each iteration binds.
func BenchmarkRules(b *testing.B) {
	benchCalls(b, append(rulesCalls(b), allocCall{
		name: "New", caseName: "the projections of the fixture tree",
		bench: func(b *testing.B) {
			b.Helper()
			rulestest.BenchRules(b, setup, rulestest.Budget{MaxAllocs: rulesAllocs})
		},
	}))
}

// rulesCalls returns a call of the constructor and of every method of
// rules.go.
func rulesCalls(tb testing.TB) []allocCall {
	tb.Helper()

	r := pythonrules.New()
	param := &node.Param{Name: "id"}
	iterator := &node.TypeRef{
		Spelling: "Iterator",
		Package:  "collections/abc",
		Args:     []*node.TypeRef{{Spelling: "int"}},
	}
	rets := []*node.Return{{Type: &node.TypeRef{Spelling: "int"}}, {Type: iterator}}
	var (
		built  rules.SourceRules
		lang   symbol.Lang
		policy rules.MemberPolicy
		role   rules.ParamRole
		roles  []rules.ReturnRole
	)
	return []allocCall{
		{
			name:     "New",
			caseName: "the rules",
			call:     func() { built = pythonrules.New() },
			check:    func(tb assert.TB) { assert.Equal(tb, built.Lang(), python.Lang, "New returns the Python rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, python.Lang, "Lang returns python") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowLinearise, "Members returns the linearised policy")
			},
		},
		{
			name:  "ParamRole",
			call:  func() { role = r.ParamRole(param, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, role, rules.ParamInput, "ParamRole classifies an input") },
		},
		{
			name: "ReturnRoles", allocs: returnRolesAllocs,
			call: func() { roles, _ = r.ReturnRoles(rets, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnStream},
					"ReturnRoles classifies the value and the stream")
			},
		},
	}
}
