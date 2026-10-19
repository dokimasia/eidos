// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// rulesAllocs is one projection pass over the fixture tree: 346 at 2,000
// iterations. A memory profile attributes about 173 to TypeOf, 84 to
// SamplesOf, 24 to CallableOf, 20 to MembersOf and 2 to the fresh view, and
// it samples the tiny allocations of the rest only in part. The 2 more are
// for the runtime's own allocations in a run of one iteration.
const rulesAllocs = 346 + 2

// returnRolesAllocs is the list of the roles of the returns.
const returnRolesAllocs = 1

// TestRules pins the member policy and the roles of the rules, and checks
// the rules against the projection contract of the kernel.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the fixture tree", func(t *testing.T) {
			t.Parallel()

			rulestest.RunRulesSuite(t, setup)
		})

		t.Run("returns rules that project enums", func(t *testing.T) {
			t.Parallel()

			_, is := graphqlrules.New().(rules.EnumRules)
			assert.True(t, is, "the rules have the enum capability")
		})

		t.Run("returns rules that project the shapes of custom scalars", func(t *testing.T) {
			t.Parallel()

			_, is := graphqlrules.New().(rules.ShapeRules)
			assert.True(t, is, "the rules have the shape capability")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns graphql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphqlrules.New().Lang(), graphql.Lang, "the language of the rules")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy under which nothing contributes", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, graphqlrules.New().Members().Contributes,
				"a type restates the fields of its interfaces, so its members are its own")
		})

		t.Run("returns override as the shadowing rule", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphqlrules.New().Members().Shadowing, rules.ShadowOverride, "the policy is total")
		})

		t.Run("returns a policy under which a type has the members that it declares", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			id := symbol.Identity{Lang: graphql.Lang, Package: shopPkg, Name: "Author", Kind: symbol.KindStruct}
			set, is := rules.NewBound(graphqlrules.New(), f.view, nil).MembersOf(f.decl(t, id))
			assert.True(t, is, "an object type has members")
			names := make([]string, 0, len(set.Members))
			for _, m := range set.Members {
				decl, declares := m.Symbol.(node.Declaration)
				assert.True(t, declares, "a member is a declaration")
				names = append(names, decl.Identity().Name)
			}
			assert.Equal(t, names, []string{"id", "name", "books"}, "the fields of Author in their order")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
		}{
			{
				name: "classifies an argument of a field as an input",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: shopPkg, Owner: "Query", Name: "book", Kind: symbol.KindMethod,
				},
			},
			{
				name: "classifies a variable of an operation as an input",
				give: symbol.Identity{Lang: graphql.Lang, Package: docsPkg, Name: "GetBook", Kind: symbol.KindFunction},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				c, is := rules.NewBound(graphqlrules.New(), f.view, nil).CallableOf(f.decl(t, tt.give))
				assert.True(t, is, "the declaration is callable")
				assert.Length(t, c.Params, 1, "the callable has one parameter")
				assert.Equal(t, c.Params[0].Role, rules.ParamInput, "the role of the parameter")
			})
		}
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want rules.ReturnRole
		}{
			{
				name: "classifies the result of a field as a value",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: shopPkg, Owner: "Query", Name: "book", Kind: symbol.KindMethod,
				},
				want: rules.ReturnValue,
			},
			{
				name: "classifies the result of a query as a value",
				give: symbol.Identity{Lang: graphql.Lang, Package: docsPkg, Name: "GetBook", Kind: symbol.KindFunction},
				want: rules.ReturnValue,
			},
			{
				name: "classifies the result of a subscription as a stream",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: docsPkg, Name: "OnChanged", Kind: symbol.KindFunction,
				},
				want: rules.ReturnStream,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				c, is := rules.NewBound(graphqlrules.New(), f.view, nil).CallableOf(f.decl(t, tt.give))
				assert.True(t, is, "the declaration is callable")
				assert.Length(t, c.Returns, 1, "the callable has one result")
				assert.Equal(t, c.Returns[0].Role, tt.want, "the role of the result")
				assert.Equal(t, c.Errors, rules.ErrorsNone, "a schema states no failure in a signature")
			})
		}

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, model := graphqlrules.New().ReturnRoles([]*node.Return{nil, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "nothing to classify")
			assert.Equal(t, model, rules.ErrorsNone, "the error model")
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

	r := graphqlrules.New()
	param := &node.Param{Name: "id"}
	rets := []*node.Return{
		{Type: &node.TypeRef{Spelling: "Book"}},
		{Type: &node.TypeRef{Spelling: "stream Book", Form: symbol.FormStream}},
	}
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
			call:     func() { built = graphqlrules.New() },
			check:    func(tb assert.TB) { assert.Equal(tb, built.Lang(), graphql.Lang, "New returns the GraphQL rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, graphql.Lang, "Lang returns graphql") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowOverride, "Members returns the override policy")
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
