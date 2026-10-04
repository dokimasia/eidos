// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/symbol"
)

// unregistered is a language no registry of the specs registers rules
// for.
const unregistered symbol.Lang = "mars"

// The allocations of a registry and of the absent rules, which
// TestContractAllocs checks in the ordinary run and BenchmarkContract in
// a benchmark run. A function without a constant allocates nothing.
const (
	// newRegistryAllocs is the registry and its map of languages.
	newRegistryAllocs = 2
	// registerAllocs is the first group of the map of languages, which
	// a registry's first language fills.
	registerAllocs = 1
	// absentAllocs is the absent rules of a language, which the
	// interface holds out of line.
	absentAllocs = 1
	// languagesAllocs is the sorted list of languages.
	languagesAllocs = 1
)

// The contract's registry and its refusing default are what a
// composition composes rules through.
func TestContract(t *testing.T) {
	t.Parallel()

	t.Run("NewRegistry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a registry without a language", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, rules.NewRegistry().Languages(), "a new registry lists no language")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("records the rules of a language", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			source := rulestest.Scripted()
			assert.NoError(t, r.Register(source), "a language registers")
			got, registered := r.For(source.Lang())
			assert.True(t, registered, "For finds the language")
			assert.Equal(t, got, source, "For returns the registered rules")
		})

		t.Run("returns an error for nil rules", func(t *testing.T) {
			t.Parallel()

			err := rules.NewRegistry().Register(nil)
			assert.HasError(t, err, "nil rules name no language")
			assert.HasPrefix(t, err.Error(), "rules: ", "under the package prefix")
		})

		t.Run("returns an error for rules of the zero language", func(t *testing.T) {
			t.Parallel()

			err := rules.NewRegistry().Register(rules.Absent(""))
			assert.HasError(t, err, "the zero language names no language")
			assert.HasPrefix(t, err.Error(), "rules: ", "under the package prefix")
		})

		t.Run("returns an error for a second value of one language", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			assert.NoError(t, r.Register(rulestest.Scripted()), "the first value registers")
			err := r.Register(rulestest.Scripted())
			assert.HasError(t, err, "a language registers once")
			assert.HasPrefix(t, err.Error(), "rules: ", "under the package prefix")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the rules registered for a language", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			source := rulestest.Scripted()
			assert.NoError(t, r.Register(source), "a language registers")
			got, registered := r.For(source.Lang())
			assert.True(t, registered, "For reports the language registered")
			assert.False(t, rules.IsAbsent(got), "For returns the registered rules")
		})

		t.Run("returns the absent rules for an unregistered language", func(t *testing.T) {
			t.Parallel()

			got, registered := rules.NewRegistry().For(unregistered)
			assert.False(t, registered, "For reports the language unregistered")
			assert.True(t, rules.IsAbsent(got), "For returns the absent rules")
			assert.Equal(t, got.Lang(), unregistered, "of the language asked for")
		})
	})

	t.Run("Languages", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the registered languages in spelling order", func(t *testing.T) {
			t.Parallel()

			r := rules.NewRegistry()
			assert.NoError(t, r.Register(otherLang{rulestest.Scripted()}), "the other language registers")
			assert.NoError(t, r.Register(rulestest.Scripted()), "the scripted language registers")
			assert.NoError(t, r.Register(scripted()), "the fixture's language registers")
			assert.Equal(t, r.Languages(),
				[]symbol.Lang{rulestest.Scripted().Lang(), coretest.Lang, otherLang{}.Lang()},
				"fake, golang and other, whatever order they registered in")
		})
	})

	t.Run("Absent", func(t *testing.T) {
		t.Parallel()

		t.Run("returns rules of the language asked for", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Absent(unregistered).Lang(), unregistered, "the absent rules name the language")
		})

		t.Run("returns rules that contribute no member", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Absent(unregistered).Members(), rules.MemberPolicy{},
				"the zero policy contributes nothing")
		})

		t.Run("returns rules that classify every parameter as an input", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Absent(unregistered).ParamRole(&node.Param{}, rules.View{}), rules.ParamInput,
				"a parameter is an input")
		})

		t.Run("returns rules that classify every return as a value", func(t *testing.T) {
			t.Parallel()

			roles, _ := rules.Absent(unregistered).ReturnRoles([]*node.Return{{}, {}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue},
				"one value role per return")
		})

		t.Run("returns rules that state ErrorsNone", func(t *testing.T) {
			t.Parallel()

			_, model := rules.Absent(unregistered).ReturnRoles([]*node.Return{{}}, rules.View{})
			assert.Equal(t, model, rules.ErrorsNone, "the absent rules state no failure channel")
		})

		t.Run("returns rules that fold every builtin to FormOpaque", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Absent(unregistered).Builtin(builtin(intSpelling), rules.View{}).Form,
				symbol.FormOpaque, "a builtin is opaque")
		})

		t.Run("returns rules that resolve no spelling", func(t *testing.T) {
			t.Parallel()

			_, err := rules.Absent(unregistered).Resolve(
				rules.Scope{}, rowName, directive.ResolveCallableInScope, rules.View{})
			assert.HasError(t, err, "a spelling names nothing")
			assert.HasPrefix(t, err.Error(), "rules: ", "under the package prefix")
		})

		t.Run("returns rules that refuse both samples with RefusedNoRules", func(t *testing.T) {
			t.Parallel()

			sample, alternate := rules.Absent(unregistered).SamplesOf(builtin(intSpelling), countField, rules.View{})
			assert.Equal(t, [2]rules.Sample{sample, alternate},
				[2]rules.Sample{rules.Refused(rules.RefusedNoRules), rules.Refused(rules.RefusedNoRules)},
				"both halves refuse for want of rules")
		})

		t.Run("returns rules without a zero value", func(t *testing.T) {
			t.Parallel()

			_, held := rules.Absent(unregistered).ZeroValue(builtin(intSpelling), rules.View{})
			assert.False(t, held, "ZeroValue reports false")
		})

		t.Run("returns rules without a literal", func(t *testing.T) {
			t.Parallel()

			_, held := rules.Absent(unregistered).LiteralFor(nil, builtin(intSpelling), "1", rules.View{})
			assert.False(t, held, "LiteralFor reports false")
		})

		t.Run("returns rules that append the word to the base", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Absent(unregistered).TypeName("Builder", rowName), "RowBuilder",
				"the join is a concatenation")
		})

		t.Run("returns rules without an optional capability", func(t *testing.T) {
			t.Parallel()

			absent := rules.Absent(unregistered)
			for name, held := range map[string]bool{
				"EnumRules":       satisfies[rules.EnumRules](absent),
				"ErrorValueRules": satisfies[rules.ErrorValueRules](absent),
				"TagRules":        satisfies[rules.TagRules](absent),
				"GenericsRules":   satisfies[rules.GenericsRules](absent),
				"PropertyRules":   satisfies[rules.PropertyRules](absent),
				"ConstructRules":  satisfies[rules.ConstructRules](absent),
				"ThrowsRules":     satisfies[rules.ThrowsRules](absent),
				"OwnershipRules":  satisfies[rules.OwnershipRules](absent),
				"PromotionRules":  satisfies[rules.PromotionRules](absent),
				"EqualityRules":   satisfies[rules.EqualityRules](absent),
			} {
				assert.False(t, held, "the absent rules satisfy no "+name)
			}
		})
	})

	t.Run("IsAbsent", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.SourceRules
			want bool
		}{
			{name: "reports true for the absent rules", give: rules.Absent(unregistered), want: true},
			{name: "reports false for the rules of a language", give: rulestest.Scripted(), want: false},
			{name: "reports false for nil rules", give: nil, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rules.IsAbsent(tt.give), tt.want, "IsAbsent reports whether Absent returned the rules")
			})
		}
	})

	t.Run("ParamRole.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.ParamRole
			want string
		}{
			{name: "returns input for ParamInput", give: rules.ParamInput, want: "input"},
			{name: "returns context for ParamContext", give: rules.ParamContext, want: "context"},
			{name: "returns the number of an undeclared role", give: rules.ParamRole(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("ReturnRole.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.ReturnRole
			want string
		}{
			{name: "returns value for ReturnValue", give: rules.ReturnValue, want: "value"},
			{name: "returns ok-bool for ReturnOkBool", give: rules.ReturnOkBool, want: "ok-bool"},
			{name: "returns stream for ReturnStream", give: rules.ReturnStream, want: "stream"},
			{name: "returns error for ReturnError", give: rules.ReturnError, want: "error"},
			{name: "returns the number of an undeclared role", give: rules.ReturnRole(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("ErrorModel.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.ErrorModel
			want string
		}{
			{name: "returns none for ErrorsNone", give: rules.ErrorsNone, want: "none"},
			{name: "returns last-return for ErrorsLastReturn", give: rules.ErrorsLastReturn, want: "last-return"},
			{name: "returns result-type for ErrorsResultType", give: rules.ErrorsResultType, want: "result-type"},
			{name: "returns thrown for ErrorsThrown", give: rules.ErrorsThrown, want: "thrown"},
			{name: "returns raised for ErrorsRaised", give: rules.ErrorsRaised, want: "raised"},
			{name: "returns the number of an undeclared model", give: rules.ErrorModel(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("Contribution.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.Contribution
			want string
		}{
			{name: "returns embeds for ContributesEmbeds", give: rules.ContributesEmbeds, want: "embeds"},
			{name: "returns extends for ContributesExtends", give: rules.ContributesExtends, want: "extends"},
			{
				name: "returns implements for ContributesImplements",
				give: rules.ContributesImplements,
				want: "implements",
			},
			{name: "returns the number of the zero contribution", give: rules.Contribution(0), want: "0"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("Shadowing.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.Shadowing
			want string
		}{
			{name: "returns promote for ShadowPromote", give: rules.ShadowPromote, want: "promote"},
			{name: "returns override for ShadowOverride", give: rules.ShadowOverride, want: "override"},
			{name: "returns merge for ShadowMerge", give: rules.ShadowMerge, want: "merge"},
			{name: "returns linearise for ShadowLinearise", give: rules.ShadowLinearise, want: "linearise"},
			{name: "returns the number of the zero rule", give: rules.Shadowing(0), want: "0"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// A registry, a language's registration, a lookup, a listing and the
// absent rules allocate within their ceilings in the ordinary run, which
// runs no benchmark. Each registration takes a registry built before the
// count, because a registry registers a language once. The check runs
// alone, because AllocsPerRun counts every goroutine's allocations and
// refuses to run beside parallel tests.
func TestContractAllocs(t *testing.T) {
	var registry *rules.Registry
	assert.MaxAllocs(t, func() { registry = rules.NewRegistry() }, newRegistryAllocs,
		"NewRegistry allocates the registry and its map")
	assert.Empty(t, registry.Languages(), "NewRegistry returns a registry without a language")

	source := rulestest.Scripted()
	fresh := make([]*rules.Registry, allocRuns)
	for i := range fresh {
		fresh[i] = rules.NewRegistry()
	}
	at := 0
	assert.MaxAllocs(t, func() {
		if err := fresh[at].Register(source); err != nil {
			t.Fatalf("Register: unexpected error: %v", err)
		}
		at++
	}, registerAllocs, "Register allocates the map's first group for a first language")

	lang := source.Lang()
	assert.MaxAllocs(t, func() {
		if _, registered := registry.For(lang); registered {
			t.Fatal("For found a language the registry does not register")
		}
	}, absentAllocs, "For allocates the absent rules for an unregistered language")
	assert.NoError(t, registry.Register(source), "the scripted language registers")
	assert.MaxAllocs(t, func() {
		if _, registered := registry.For(lang); !registered {
			t.Fatal("For missed the registered language")
		}
	}, 0, "For allocates nothing for a registered language")
	assert.MaxAllocs(t, func() {
		if len(registry.Languages()) != 1 {
			t.Fatal("Languages listed another number of languages")
		}
	}, languagesAllocs, "Languages allocates the list")

	var absent rules.SourceRules
	assert.MaxAllocs(t, func() { absent = rules.Absent(unregistered) }, absentAllocs,
		"Absent allocates the rules the interface holds")
	var got bool
	assert.MaxAllocs(t, func() { got = rules.IsAbsent(absent) }, 0, "IsAbsent allocates nothing")
	assert.True(t, got, "IsAbsent reports true for the absent rules")

	for _, tt := range spellings(t) {
		msg := tt.name + " allocates nothing for a declared value"
		assert.MaxAllocs(t, tt.call, tt.allocs, msg)
	}
}

// BenchmarkContract measures a registry's construction, a first
// registration, a lookup that finds the language and one that does not,
// a listing, the absent rules, the check for them, and the spelling of
// each vocabulary.
func BenchmarkContract(b *testing.B) {
	source := rulestest.Scripted()

	b.Run("NewRegistry", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newRegistryAllocs)
		defer c.End()
		var r *rules.Registry
		for c.Loop() {
			r = rules.NewRegistry()
		}
		assert.Empty(b, r.Languages(), "NewRegistry returns a registry without a language")
	})

	b.Run("Register", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(registerAllocs)
		defer c.End()
		var (
			r   *rules.Registry
			err error
		)
		for c.Loop() {
			c.Excluding(func() { r = rules.NewRegistry() })
			err = r.Register(source)
		}
		assert.NoError(b, err, "Register records the first language")
	})

	b.Run("For", func(b *testing.B) {
		b.Run("a registered language", func(b *testing.B) {
			r := rules.NewRegistry()
			assert.NoError(b, r.Register(source), "the scripted language registers")
			lang := source.Lang()
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			registered := false
			for c.Loop() {
				_, registered = r.For(lang)
			}
			assert.True(b, registered, "For finds the registered language")
		})

		b.Run("an unregistered language", func(b *testing.B) {
			r := rules.NewRegistry()
			c := bench.Start(b).MaxAllocs(absentAllocs)
			defer c.End()
			var got rules.SourceRules
			for c.Loop() {
				got, _ = r.For(unregistered)
			}
			assert.True(b, rules.IsAbsent(got), "For returns the absent rules")
		})
	})

	b.Run("Languages", func(b *testing.B) {
		r := rules.NewRegistry()
		assert.NoError(b, r.Register(source), "the scripted language registers")
		c := bench.Start(b).MaxAllocs(languagesAllocs)
		defer c.End()
		var got []symbol.Lang
		for c.Loop() {
			got = r.Languages()
		}
		assert.Equal(b, got, []symbol.Lang{source.Lang()}, "Languages lists the registered language")
	})

	b.Run("Absent", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(absentAllocs)
		defer c.End()
		var got rules.SourceRules
		for c.Loop() {
			got = rules.Absent(unregistered)
		}
		assert.Equal(b, got.Lang(), unregistered, "Absent returns the rules of the language asked for")
	})

	b.Run("IsAbsent", func(b *testing.B) {
		absent := rules.Absent(unregistered)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := false
		for c.Loop() {
			got = rules.IsAbsent(absent)
		}
		assert.True(b, got, "IsAbsent reports true for the absent rules")
	})

	for _, tt := range spellings(b) {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
		})
	}
}

// satisfies reports whether rules satisfy the capability C, the
// assertion a consumer makes on [rules.Bound.Source].
func satisfies[C any](r rules.SourceRules) bool {
	_, held := r.(C)
	return held
}

// spellings returns a call of the String method of each vocabulary the
// contract declares, on a declared value. Each call checks the spelling
// it returned.
func spellings(tb assert.TB) []allocCall {
	tb.Helper()

	param, ret, model := rules.ParamContext, rules.ReturnError, rules.ErrorsResultType
	contribution, shadowing := rules.ContributesImplements, rules.ShadowLinearise
	return []allocCall{
		{name: "ParamRole.String", call: func() {
			if param.String() != "context" {
				tb.Fatalf("ParamRole.String returned another spelling")
			}
		}},
		{name: "ReturnRole.String", call: func() {
			if ret.String() != "error" {
				tb.Fatalf("ReturnRole.String returned another spelling")
			}
		}},
		{name: "ErrorModel.String", call: func() {
			if model.String() != "result-type" {
				tb.Fatalf("ErrorModel.String returned another spelling")
			}
		}},
		{name: "Contribution.String", call: func() {
			if contribution.String() != "implements" {
				tb.Fatalf("Contribution.String returned another spelling")
			}
		}},
		{name: "Shadowing.String", call: func() {
			if shadowing.String() != "linearise" {
				tb.Fatalf("Shadowing.String returned another spelling")
			}
		}},
	}
}
