// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// rulesAllocs is the budget of one projection pass over the fixture crate and
// its client crate. The pass allocated 644 times at 200 iterations. A memory
// profile attributes about 48% to TypeOf, 33% to SamplesOf, 14% to
// CallableOf and 4% to MembersOf. The budget is about 5% above the count.
const rulesAllocs = 680

// returnRolesAllocs is the list of the roles of the returns.
const returnRolesAllocs = 1

// The traits and the methods of the fixture that the member walk and the
// roles read.
var (
	namedID = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "Named", Kind: symbol.KindInterface}
	shelfID = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "Shelf", Kind: symbol.KindStruct}
	findID  = symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "find", Kind: symbol.KindFunction}
)

// TestRules pins the member policy and the roles of the rules, and checks the
// rules against the projection contract of the kernel.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules that meet the projection contract over the fixture crate", func(t *testing.T) {
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
			{name: "returns rules that project error types", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ErrorTypeRules)
				return is
			}},
			{name: "returns rules that split results", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ResultRules)
				return is
			}},
			{name: "returns rules that state ownership", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.OwnershipRules)
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
			{name: "returns rules that project constructors", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.ConstructRules)
				return is
			}},
			{name: "returns rules that build texts", give: func(r rules.SourceRules) bool {
				_, is := r.(rules.TextRules)
				return is
			}},
		}
		for _, tt := range capabilities {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, tt.give(rustrules.New()), "the rules have the capability")
			})
		}
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rust", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rustrules.New().Lang(), rust.Lang, "the language of the rules")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy under which the supertraits and the trait impls contribute", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rustrules.New().Members().Contributes,
				[]rules.Contribution{rules.ContributesExtends, rules.ContributesImplements},
				"a type has the methods of its supertraits and of its trait impls")
		})

		t.Run("returns promote as the shadowing rule", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rustrules.New().Members().Shadowing, rules.ShadowPromote,
				"two traits of one depth cancel a name")
		})

		tests := []struct {
			name string
			give symbol.Identity
			want []string
		}{
			{
				name: "returns a policy under which a struct has the methods of its trait impls after its own",
				give: memoryID,
				want: []string{"orders", "LIMIT", "new", "len", "get", "list", "ids", "save", "flush", "write"},
			},
			{
				name: "returns a policy under which a trait has the methods of its supertraits",
				give: namedID,
				want: []string{"name", "get", "list", "ids", "save", "flush", "write", "len"},
			},
			{
				name: "returns a policy under which two traits of one depth cancel a name",
				give: shelfID,
				want: nil,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				set, is := rules.NewBound(rustrules.New(), f.view, nil).MembersOf(f.decl(t, tt.give))
				assert.True(t, is, "a type has members")
				var names []string
				for _, m := range set.Members {
					decl, declares := m.Symbol.(node.Declaration)
					assert.True(t, declares, "a member is a declaration")
					names = append(names, decl.Identity().Name)
				}
				assert.Equal(t, names, tt.want, "the member names in their order")
			})
		}
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies each parameter as an input", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			c, is := rules.NewBound(rustrules.New(), f.view, nil).CallableOf(f.decl(t, findID))
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
			name      string
			give      string
			wantRoles []rules.ReturnRole
			wantModel rules.ErrorModel
		}{
			{
				name: "classifies a result of the alias of the crate as a value and a failure", give: "get",
				wantRoles: []rules.ReturnRole{rules.ReturnValue, rules.ReturnError}, wantModel: rules.ErrorsResultType,
			},
			{
				name: "classifies a Box of dyn Iterator as a stream", give: "list",
				wantRoles: []rules.ReturnRole{rules.ReturnStream}, wantModel: rules.ErrorsNone,
			},
			{
				name: "classifies impl Iterator as a stream", give: "ids",
				wantRoles: []rules.ReturnRole{rules.ReturnStream}, wantModel: rules.ErrorsNone,
			},
			{
				name: "classifies a result of the unit type as a failure alone", give: "save",
				wantRoles: []rules.ReturnRole{rules.ReturnError}, wantModel: rules.ErrorsResultType,
			},
			{
				name: "classifies io::Result of the unit type as a failure alone", give: "flush",
				wantRoles: []rules.ReturnRole{rules.ReturnError}, wantModel: rules.ErrorsResultType,
			},
			{
				name: "classifies fmt::Result as a failure alone", give: "write",
				wantRoles: []rules.ReturnRole{rules.ReturnError}, wantModel: rules.ErrorsResultType,
			},
			{
				name: "classifies a result of usize as a value", give: "len",
				wantRoles: []rules.ReturnRole{rules.ReturnValue}, wantModel: rules.ErrorsNone,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				c, is := rules.NewBound(rustrules.New(), f.view, nil).CallableOf(f.method(t, storeID, tt.give))
				assert.True(t, is, "the method is callable")
				roles := make([]rules.ReturnRole, 0, len(c.Returns))
				for _, r := range c.Returns {
					roles = append(roles, r.Role)
				}
				assert.Equal(t, roles, tt.wantRoles, "the roles of the views of the return")
				assert.Equal(t, c.Errors, tt.wantModel, "the error model")
			})
		}

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, model := rustrules.New().ReturnRoles([]*node.Return{nil, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "nothing to classify")
			assert.Equal(t, model, rules.ErrorsNone, "the error model")
		})

		t.Run("classifies a borrow of an iterator as a stream", func(t *testing.T) {
			t.Parallel()

			iter := &node.TypeRef{Spelling: "Iterator", Args: []*node.TypeRef{
				{Spelling: "Item=u8", Args: []*node.TypeRef{{Spelling: "u8"}}},
			}}
			dyn := &node.TypeRef{
				Spelling: "dyn Iterator<Item=u8>", Form: symbol.FormIntersection,
				Elems: []*node.TypeRef{iter},
			}
			borrow := &node.TypeRef{
				Spelling: "&mut dyn Iterator<Item=u8>", Form: symbol.FormBorrow,
				Elems: []*node.TypeRef{dyn},
			}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: borrow}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnStream}, "the borrow yields the stream")
		})

		t.Run("classifies an iterator as a stream", func(t *testing.T) {
			t.Parallel()

			iter := &node.TypeRef{Spelling: "Iterator", Args: []*node.TypeRef{
				{Spelling: "Item=u8", Args: []*node.TypeRef{{Spelling: "u8"}}},
			}}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: iter}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnStream}, "the iterator is a stream")
		})

		t.Run("classifies a stream nested past the budget of the walk as a value", func(t *testing.T) {
			t.Parallel()

			ref := &node.TypeRef{Spelling: "Iterator", Args: []*node.TypeRef{
				{Spelling: "Item=u8", Args: []*node.TypeRef{{Spelling: "u8"}}},
			}}
			for range 9 {
				ref = &node.TypeRef{Spelling: "&T", Form: symbol.FormBorrow, Elems: []*node.TypeRef{ref}}
			}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: ref}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue}, "the walk stops before the iterator")
		})

		t.Run("classifies a Box of a type that is no stream as a value", func(t *testing.T) {
			t.Parallel()

			boxed := &node.TypeRef{Spelling: "Box", Args: []*node.TypeRef{{Spelling: "u8"}}}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: boxed}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue}, "a Box of a number is a value")
		})

		t.Run("classifies a trait object of a trait of the crate as a value", func(t *testing.T) {
			t.Parallel()

			dyn := &node.TypeRef{
				Spelling: "dyn Store", Form: symbol.FormIntersection,
				Elems: []*node.TypeRef{{Spelling: "Store", Target: storeID}, nil},
			}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: dyn}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue}, "the trait is no iterator")
		})

		t.Run("classifies a tuple as a value", func(t *testing.T) {
			t.Parallel()

			tuple := &node.TypeRef{Spelling: "(u8,u8)", Form: symbol.FormTuple}
			roles, _ := rustrules.New().ReturnRoles([]*node.Return{{Type: tuple}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue}, "a tuple is a value")
		})
	})
}

// TestRulesAllocs checks the ceilings of the methods of rules.go in the
// ordinary run, which does not run benchmarks.
func TestRulesAllocs(t *testing.T) {
	checkAllocs(t, rulesCalls(t))
}

// BenchmarkRules measures each method of rules.go, and the four hot
// projections of the kernel over the fixture crate under their budget. Only
// the benchmark checks the budget of the projections, because the contract
// leaves out the fresh view that each iteration binds.
func BenchmarkRules(b *testing.B) {
	benchCalls(b, append(rulesCalls(b), allocCall{
		name: "New", caseName: "the projections of the fixture crate",
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

	r := rustrules.New()
	param := &node.Param{Name: "id"}
	stream := &node.TypeRef{Spelling: "dyn Iterator<Item=u8>", Form: symbol.FormIntersection, Elems: []*node.TypeRef{
		{Spelling: "Iterator", Args: []*node.TypeRef{{Spelling: "Item=u8", Args: []*node.TypeRef{{Spelling: "u8"}}}}},
	}}
	rets := []*node.Return{{Type: &node.TypeRef{Spelling: "u8"}}, {Type: stream}}
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
			call:     func() { built = rustrules.New() },
			check:    func(tb assert.TB) { assert.Equal(tb, built.Lang(), rust.Lang, "New returns the Rust rules") },
		},
		{
			name:  "Lang",
			call:  func() { lang = r.Lang() },
			check: func(tb assert.TB) { assert.Equal(tb, lang, rust.Lang, "Lang returns rust") },
		},
		{
			name: "Members",
			call: func() { policy = r.Members() },
			check: func(tb assert.TB) {
				assert.Equal(tb, policy.Shadowing, rules.ShadowPromote, "Members returns the promoting policy")
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
