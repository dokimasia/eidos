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
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The package of the satisfaction fixture, and the names of its
// declarations and members beside getName and putName, which the
// callable and resolve cases declare.
const (
	satPath       = "svc/sat"
	contractName  = "Contract"
	numberName    = "Number"
	boxedName     = "Boxed"
	fullName      = "Full"
	partialName   = "Partial"
	wrongName     = "Wrong"
	pointeredName = "Pointered"
	intBoxName    = "IntBox"
	valueName     = "Value"
	typeParamName = "T"
	labelledName  = "Labelled"
	labelName     = "Label"
	labelField    = "label"
)

// The names of the fixture of a method of package visibility. Sealed
// declares seal beside Get, and Sealer, Forger and Holder are the
// candidates that [sealing] describes.
const (
	sealedName = "Sealed"
	sealerName = "Sealer"
	forgerName = "Forger"
	holderName = "Holder"
	sealName   = "seal"
)

// satisfyAllocs is the comparison of the member set of Full with the
// members of Contract: the member lists of the two walks, the list of the
// members of Contract, and the signatures of the four methods that it
// compares.
const satisfyAllocs = 2 + 1 + 4

// refining is the scripted rules with a satisfaction that reads more than
// the member sets. The method Put of Pointered belongs to a pointer to
// Pointered alone, and the interface Number constrains a type set.
type refining struct{ rules.SourceRules }

var _ rules.SatisfactionRules = refining{}

// ValueReceiver reports that a value has every method but Put of
// Pointered.
func (refining) ValueReceiver(m *node.Method, _ rules.View) bool {
	return m.ID.Owner != pointeredName || m.Name != putName
}

// TypeSet reports that Number constrains a type set.
func (refining) TypeSet(i *node.Interface, _ rules.View) bool { return i.Name == numberName }

// The satisfaction compares the member sets that the member walk returns,
// so every case reads a frozen graph through a binding.
func TestSatisfy(t *testing.T) {
	t.Parallel()

	g := coretest.Frozen(t, satisfying())
	contract := named(satPath, contractName, symbol.KindInterface)
	sealedGraph := coretest.Frozen(t, sealing()...)
	sealed := named(satPath, sealedName, symbol.KindInterface)

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		pointer := &node.TypeRef{
			Form: symbol.FormOptional, Elems: []*node.TypeRef{named(satPath, pointeredName, symbol.KindStruct)},
		}
		tests := []struct {
			name           string
			give           *node.TypeRef
			wantMissing    []string
			wantMismatched []string
		}{
			{
				name: "returns no miss for a type that has every member",
				give: named(satPath, fullName, symbol.KindStruct),
			},
			{
				name: "returns the member that a type misses", give: named(satPath, partialName, symbol.KindStruct),
				wantMissing: []string{putName},
			},
			{
				name: "returns the member that a type has with another signature",
				give: named(satPath, wrongName, symbol.KindStruct), wantMismatched: []string{getName},
			},
			{
				name: "counts a method that only a pointer has as missing for a value",
				give: named(satPath, pointeredName, symbol.KindStruct), wantMissing: []string{putName},
			},
			{name: "returns no miss for a pointer to a type whose pointer has every member", give: pointer},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, compared := satisfier(t, g).Satisfies(tt.give, contract)
				assert.True(t, compared, "the reference and the interface name declarations")
				expect.Equal(t, memberNames(s.Missing), tt.wantMissing, "the missing members")
				expect.Equal(t, mismatchNames(s.Mismatched), tt.wantMismatched, "the mismatched members")
				expect.Empty(t, s.Gaps, "both walks are complete")
			})
		}

		scoped := []struct {
			name        string
			give        *node.TypeRef
			giveScoped  bool
			wantMissing []string
		}{
			{
				name:       "returns no miss for a type of the package of a method of package visibility",
				give:       named(satPath, sealerName, symbol.KindStruct),
				giveScoped: true,
			},
			{
				name:        "returns the method of package visibility that a type of another package declares as missing",
				give:        named(depPath, forgerName, symbol.KindStruct),
				giveScoped:  true,
				wantMissing: []string{sealName},
			},
			{
				name:       "returns no miss for a type of another package that embeds the interface",
				give:       named(depPath, holderName, symbol.KindStruct),
				giveScoped: true,
			},
			{
				name: "returns no miss for a type of another package without PackageScopedNames",
				give: named(depPath, forgerName, symbol.KindStruct),
			},
		}
		for _, tt := range scoped {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, compared := scopedSatisfier(t, sealedGraph, tt.giveScoped).Satisfies(tt.give, sealed)
				assert.True(t, compared, "the reference and the interface name declarations")
				expect.Equal(t, memberNames(s.Missing), tt.wantMissing, "the missing members")
				expect.Empty(t, s.Mismatched, "no member has another signature")
				expect.Empty(t, s.Gaps, "both walks are complete")
			})
		}

		t.Run("substitutes the type arguments of a generic interface", func(t *testing.T) {
			t.Parallel()

			boxed := named(satPath, boxedName, symbol.KindInterface)
			boxed.Args = []*node.TypeRef{builtin(intSpelling)}
			s, compared := satisfier(t, g).Satisfies(named(satPath, intBoxName, symbol.KindStruct), boxed)
			assert.True(t, compared, "the reference and the interface name declarations")
			assert.True(t, s.Satisfied(), "Value() T at int is the Value() int of IntBox")
		})

		t.Run("returns no miss for a type with the field of an interface", func(t *testing.T) {
			t.Parallel()

			s, compared := satisfier(t, g).Satisfies(named(satPath, labelName, symbol.KindStruct),
				named(satPath, labelledName, symbol.KindInterface))
			assert.True(t, compared, "the reference and the interface name declarations")
			expect.Empty(t, s.Missing, "Label has the field label of Labelled")
			expect.Empty(t, s.Mismatched, "the field label of Label has the type of the field of Labelled")
			expect.Empty(t, s.Gaps, "both walks are complete")
		})

		t.Run("reports false for an interface that constrains a type set", func(t *testing.T) {
			t.Parallel()

			_, compared := satisfier(t, g).Satisfies(named(satPath, fullName, symbol.KindStruct),
				named(satPath, numberName, symbol.KindInterface))
			assert.False(t, compared, "a comparison of members cannot prove a type set")
		})

		t.Run("reports false for a reference that names no declaration", func(t *testing.T) {
			t.Parallel()

			_, compared := satisfier(t, g).Satisfies(named(satPath, "Ghost", symbol.KindStruct), contract)
			assert.False(t, compared, "the view does not contain Ghost")
		})
	})

	t.Run("Implementers", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the types whose value or pointer satisfies the interface in identity order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, satisfier(t, g).Implementers(contract), []symbol.Identity{
				coretest.ID(satPath, fullName, symbol.KindStruct),
				coretest.ID(satPath, pointeredName, symbol.KindStruct),
			}, "Full and the pointer to Pointered satisfy Contract")
		})

		t.Run("returns the types that have the method of package visibility from its package", func(t *testing.T) {
			t.Parallel()

			assert.Permutation(t, scopedSatisfier(t, sealedGraph, true).Implementers(sealed), []symbol.Identity{
				coretest.ID(satPath, sealerName, symbol.KindStruct),
				coretest.ID(depPath, holderName, symbol.KindStruct),
			}, "Sealer declares seal in the package of Sealed, and Holder embeds Sealed")
		})

		t.Run("returns nil for an interface that constrains a type set", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, satisfier(t, g).Implementers(named(satPath, numberName, symbol.KindInterface)),
				"no type implements a type set by its members")
		})
	})

	t.Run("Satisfied", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for a satisfaction with a gap", func(t *testing.T) {
			t.Parallel()

			s := rules.Satisfaction{Gaps: []rules.Gap{{Reason: rules.GapUnresolved}}}
			assert.False(t, s.Satisfied(), "a gap leaves the comparison unproven")
		})
	})
}

// A comparison allocates the two member walks and the key of each member
// that it compares. The ordinary run, which runs no benchmark, checks the
// ceiling here.
func TestSatisfyAllocs(t *testing.T) {
	b := satisfier(t, coretest.Frozen(t, satisfying()))
	full, contract := named(satPath, fullName, symbol.KindStruct), named(satPath, contractName, symbol.KindInterface)
	var s rules.Satisfaction
	assert.MaxAllocs(t, func() { s, _ = b.Satisfies(full, contract) }, satisfyAllocs,
		"Satisfies allocates the member walks and the keys")
	assert.True(t, s.Satisfied(), "Full satisfies Contract")
}

// BenchmarkSatisfy measures the comparison that a plugin makes for each
// candidate of an interface.
func BenchmarkSatisfy(b *testing.B) {
	bound := satisfier(b, coretest.Frozen(b, satisfying()))
	full, contract := named(satPath, fullName, symbol.KindStruct), named(satPath, contractName, symbol.KindInterface)
	c := bench.Start(b).Warmup(1).MaxAllocs(satisfyAllocs)
	defer c.End()
	var s rules.Satisfaction
	for c.Loop() {
		s, _ = bound.Satisfies(full, contract)
	}
	assert.True(b, s.Satisfied(), "Full satisfies Contract")
}

// satisfier binds the refining rules to a view over the graph.
func satisfier(tb assert.TB, g *store.Graph) rules.Bound {
	tb.Helper()

	v, _, _ := viewOver(tb, g)
	return rules.NewBound(refining{scripted()}, v, nil)
}

// scopedSatisfier binds the refining rules under the member policy of Go to
// a view over the graph. scoped sets PackageScopedNames of the policy.
func scopedSatisfier(tb assert.TB, g *store.Graph, scoped bool) rules.Bound {
	tb.Helper()

	v, _, _ := viewOver(tb, g)
	return rules.NewBound(refining{policy{scripted(), rules.MemberPolicy{
		Contributes: []rules.Contribution{rules.ContributesEmbeds}, Shadowing: rules.ShadowPromote,
		EmbedsAreFields: true, PackageScopedNames: scoped,
	}}}, v, nil)
}

// satisfying builds the satisfaction fixture. Contract has the methods Get
// and Put. Full has both, Partial has Get alone, Wrong has a Get that
// returns an int, and Pointered has a Put that only a pointer has. Number
// constrains a type set. Boxed[T] has Value() T, and IntBox has Value()
// int. Labelled has the string field label, and so does Label.
func satisfying() *node.Package {
	contract := coretest.Interface(satPath, contractName)
	contract.Methods = []*node.Method{method(satPath, contractName, getName), method(satPath, contractName, putName)}
	full := coretest.Struct(satPath, fullName)
	full.Methods = []*node.Method{method(satPath, fullName, getName), method(satPath, fullName, putName)}
	partial := coretest.Struct(satPath, partialName)
	partial.Methods = []*node.Method{method(satPath, partialName, getName)}
	wrongGet := method(satPath, wrongName, getName)
	wrongGet.Returns = []*node.Return{{Type: builtin(intSpelling)}}
	wrong := coretest.Struct(satPath, wrongName)
	wrong.Methods = []*node.Method{wrongGet, method(satPath, wrongName, putName)}
	pointered := coretest.Struct(satPath, pointeredName)
	pointered.Methods = []*node.Method{method(satPath, pointeredName, getName), method(satPath, pointeredName, putName)}
	number := coretest.Interface(satPath, numberName)
	param := &node.TypeParam{
		ID:   coretest.MemberID(satPath, boxedName, typeParamName, symbol.KindTypeParam),
		Name: typeParamName,
	}
	value := coretest.Method(satPath, boxedName, valueName)
	value.Returns = []*node.Return{{Type: &node.TypeRef{Spelling: typeParamName, Target: param.ID}}}
	boxed := coretest.Interface(satPath, boxedName)
	boxed.TypeParams = []*node.TypeParam{param}
	boxed.Methods = []*node.Method{value}
	intValue := coretest.Method(satPath, intBoxName, valueName)
	intValue.Returns = []*node.Return{{Type: builtin(intSpelling)}}
	intBox := coretest.Struct(satPath, intBoxName)
	intBox.Methods = []*node.Method{intValue}
	labelled := iface(satPath, labelledName)
	labelled.Fields = []*node.Field{field(satPath, labelledName, labelField, builtin(strSpelling))}
	label := coretest.Struct(satPath, labelName)
	label.Fields = []*node.Field{field(satPath, labelName, labelField, builtin(strSpelling))}
	return coretest.Package(satPath, contract, full, partial, wrong, pointered, number, boxed, intBox, labelled, label)
}

// sealing builds the fixture of a method of package visibility. Sealed
// has the methods Get and seal. Sealer of the package of Sealed declares
// both. Forger of another package declares both, and Holder of the other
// package embeds Sealed. The embedded field Sealed of Holder has the
// identity that the load assigns to it.
func sealing() []*node.Package {
	sealed := coretest.Interface(satPath, sealedName)
	sealed.Methods = []*node.Method{method(satPath, sealedName, getName), sealOf(satPath, sealedName)}
	sealer := coretest.Struct(satPath, sealerName)
	sealer.Methods = []*node.Method{method(satPath, sealerName, getName), sealOf(satPath, sealerName)}
	forger := coretest.Struct(depPath, forgerName)
	forger.Methods = []*node.Method{method(depPath, forgerName, getName), sealOf(depPath, forgerName)}
	embed := embedOf(satPath, sealedName, symbol.KindInterface)
	embed.ID = coretest.MemberID(depPath, holderName, sealedName, symbol.KindEmbed)
	holder := coretest.Struct(depPath, holderName)
	holder.Embeds = []*node.Embed{embed}
	return []*node.Package{coretest.Package(satPath, sealed, sealer), coretest.Package(depPath, forger, holder)}
}

// sealOf returns the method seal of package visibility on a host.
func sealOf(path, host string) *node.Method {
	m := method(path, host, sealName)
	m.Visibility = symbol.VisibilityPackage
	return m
}

// memberNames returns the names of members, and nil for none.
func memberNames(members []rules.Member) []string {
	var out []string
	for _, m := range members {
		if method, is := m.Symbol.(*node.Method); is {
			out = append(out, method.Name)
		}
	}
	return out
}

// mismatchNames returns the names of the members of the interface that
// mismatches pair, and nil for none.
func mismatchNames(mismatches []rules.Mismatch) []string {
	var out []string
	for _, m := range mismatches {
		if method, is := m.Want.Symbol.(*node.Method); is {
			out = append(out, method.Name)
		}
	}
	return out
}
