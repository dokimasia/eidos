// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// discriminant is the property every lowered variant interface
// leads with, typed to the variant's own name as a string literal,
// which is what narrows a union member back to its variant.
const discriminant = "kind"

// variantSep separates the name of a sum from the name of its variant in
// the name of a variant interface, which the convention of TypeScript
// joins into one Pascal-case word.
const variantSep = "_"

// Lower reshapes the constructs TypeScript states in other
// declarations. A sum becomes one interface per variant and a
// union alias keeping the sum's name: each interface leads with
// the discriminant property typed to the variant's literal name,
// takes the variant's payload as its properties behind it, extends
// the contracts of the sum, and restates the sum's type parameters.
// Each interface has its own copy of the contracts, because the
// settle rewrites a reference in place. The alias's target joins the
// interfaces with the union bar, restating the parameters as
// arguments.
//
// The union target is a composite spelling, which the settle
// leaves as written, so the variant interfaces cannot wait for the
// respell: the lowering joins the sum's name and each variant's with a
// word separator, spells the join final through the
// module's own convention, and writes the same spelling into the
// interface and the union both, consistent by construction. The
// alias keeps the sum's name untouched, so a reference to the sum
// follows the settle the way any reference does. The discriminant
// literal keeps the variant's declared name, quoted in TypeScript's
// grammar, because the property is data and a respell must not move
// what a wire may send. The lowering writes the literal in the double
// quote, and the render writes it in the quote of the policy of its plan.
//
// Every output has the sum's origin and none restates the sum, so a
// second settle changes nothing. A sum stating methods refuses,
// because a union has no members. One stating no variants refuses,
// because a union joins at least one. A decorated sum or variant
// refuses, because TypeScript decorates classes alone. A payload entry
// without a name refuses, because a property has one. Lower also returns
// an error for a payload entry named like the discriminant, because the
// variant interface would declare that property twice. A class puts the
// overload signatures of each method name right before the implementation
// of the name, through [lowering.GroupOverloads], because TypeScript
// requires the implementation right after its signatures. A class or an
// interface passes through otherwise unchanged unless [members] returns an
// error for it. A declaration with the deleter of a property refuses
// through [lowering.DeleteAccessor], because TypeScript writes a getter
// and a setter alone. Everything else passes through unchanged.
//
// TypeScript admits a variance annotation on a type parameter of a class,
// of an interface, and of an alias of an object type or a function type.
// Lower therefore gives a function, a method and an alias of any other
// target a copy of its type parameters without the variance, as it gives
// the union alias of a sum. An object type opens with a brace, as an inline
// type, an index signature and a mapped type of TypeScript do. TypeScript
// infers the variance of every generic type from its structure, so each
// declaration keeps its meaning.
//
// # Allocation contract
//
// A declaration that passes through allocates nothing, apart from the list
// that [lowering.GroupOverloads] allocates for a class whose overloads
// stand apart, and the copy of the type parameters that states a variance
// where TypeScript admits none. A sum allocates
// the list of outputs, the union alias, its target and the joined
// union: four allocations. Each variant adds seven: its interface, the
// two steps of its name (the join and the final spelling), its list of
// fields, the discriminant field, its type and the quoted literal. A sum
// of two variants allocates 18 times. A generic sum adds, per variant,
// the copy of its type parameters and the variant's spelling with its
// type arguments. A sum with contracts adds, per variant, the copy of its
// contracts. A refusal allocates its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if err := lowering.DeleteAccessor(typescript.Lang, s); err != nil {
		return nil, err
	}
	var sum *emit.Sum
	switch d := s.(type) {
	case *emit.Struct:
		if methods := d.Methods.Items(); len(methods) > 1 {
			d.Methods.Replace(lowering.GroupOverloads(methods)...)
		}
		for _, m := range d.Methods.Items() {
			m.TypeParams = invariant(m.TypeParams)
		}
		return nil, members(d.Name, d.Fields.Items(), d.Methods.Items(), true)
	case *emit.Interface:
		for _, m := range d.Methods.Items() {
			m.TypeParams = invariant(m.TypeParams)
		}
		return nil, members(d.Name, d.Fields.Items(), d.Methods.Items(), false)
	case *emit.Function:
		d.TypeParams = invariant(d.TypeParams)
		return nil, nil
	case *emit.Alias:
		if d.Target == nil || d.Target.Form != symbol.FormFunc && !strings.HasPrefix(d.Target.Spelling, objectOpen) {
			d.TypeParams = invariant(d.TypeParams)
		}
		return nil, nil
	case *emit.Sum:
		sum = d
	default:
		return nil, nil // the declaration passes through
	}
	switch {
	case sum.Methods.Len() > 0:
		return nil, refuse("a union has no members, and %s states methods", sum.Name)
	case sum.Variants.Len() == 0:
		return nil, refuse("a union joins at least one variant, and %s states none", sum.Name)
	case len(sum.Annotations) > 0:
		return nil, refuse("decorators apply to classes, and the sum %s states some", sum.Name)
	}
	variants := sum.Variants.Items()
	out := make([]symbol.Symbol, 0, 1+len(variants))
	parts := make([]string, 0, len(variants))
	for _, v := range variants {
		iface, part, err := variantInterface(sum, v)
		if err != nil {
			return nil, err
		}
		out = append(out, iface)
		parts = append(parts, part)
	}
	return append(out, &emit.Alias{
		Origin:     sum.Origin,
		Doc:        sum.Doc,
		Comment:    sum.Comment,
		Name:       sum.Name,
		Visibility: sum.Visibility,
		TypeParams: invariant(sum.TypeParams),
		Target:     &emit.TypeRef{Spelling: strings.Join(parts, " | ")},
	}), nil
}

// invariant returns the type parameters ps where none of them states a
// variance, and a copy of ps without the variance otherwise, for a
// declaration on which TypeScript admits no variance annotation. The copy
// leaves ps unchanged, because another declaration can share the list.
func invariant(ps []*emit.TypeParam) []*emit.TypeParam {
	if !slices.ContainsFunc(ps, func(p *emit.TypeParam) bool { return p.Variance != symbol.VarianceInvariant }) {
		return ps
	}
	out := lowering.CopyTypeParams(ps)
	for _, p := range out {
		p.Variance = symbol.VarianceInvariant
	}
	return out
}

// members returns an error for two members of one name in a class or an
// interface host, where both members are static or neither is. TypeScript
// allows one property, one getter and one setter of each name, and a
// property and a method cannot share a name. In a class, members also
// returns an error for two methods of one name that are no overload
// signatures, unless both are abstract, and for the overload signatures
// of a name without an implementation. The backend renders a body for
// each method that is neither abstract nor an overload signature, and
// TypeScript allows one implementation of each name. TypeScript reads two
// method signatures of one name in an interface as overloads. An index
// signature has no name, so members skips it. host is the name of the
// class or the interface, and each error contains it. members does not
// allocate, and an error allocates its message.
func members(host string, fields []*emit.Field, methods []*emit.Method, class bool) error {
	for i, f := range fields {
		if slices.ContainsFunc(fields[:i], func(g *emit.Field) bool { return g.Name == f.Name && g.Level == f.Level }) {
			return refuse("%s declares two properties named %s, and TypeScript declares one", host, f.Name)
		}
	}
	for i, m := range methods {
		if m.Indexer {
			continue
		}
		if slices.ContainsFunc(fields, func(f *emit.Field) bool { return f.Name == m.Name && f.Level == m.Level }) {
			return refuse("%s declares a property and a method named %s, and TypeScript declares one of them",
				host, m.Name)
		}
		for _, earlier := range methods[:i] {
			if earlier.Indexer || earlier.Name != m.Name || earlier.Level != m.Level || earlier.Accessor != m.Accessor {
				continue
			}
			switch {
			case m.Accessor == symbol.AccessorGet:
				return refuse("%s declares two getters named %s, and TypeScript declares one", host, m.Name)
			case m.Accessor == symbol.AccessorSet:
				return refuse("%s declares two setters named %s, and TypeScript declares one", host, m.Name)
			case class && !m.Overload && !earlier.Overload && (!m.Abstract || !earlier.Abstract):
				return refuse("%s declares two methods named %s, and a method of a class has one implementation",
					host, m.Name)
			}
		}
		if class && m.Overload && !m.Abstract && !slices.ContainsFunc(methods, func(other *emit.Method) bool {
			return !other.Overload && !other.Indexer && other.Name == m.Name && other.Level == m.Level
		}) {

			return refuse("%s declares the overload signatures of %s and no implementation", host, m.Name)
		}
	}
	return nil
}

// variantInterface builds one variant's interface and the spelling
// the union restates it by, the type arguments included.
func variantInterface(
	sum *emit.Sum, v *emit.SumVariant,
) (*emit.Interface, string, error) {
	if len(v.Annotations) > 0 {
		return nil, "", refuse("decorators apply to classes, and the variant %s states some", v.Name)
	}
	if err := lowering.NamedPayload(typescript.Lang, v); err != nil {
		return nil, "", err
	}
	name, err := spell.Name(symbol.KindInvalid, symbol.KindInterface,
		sum.Visibility, sum.Name+variantSep+v.Name)
	if err != nil {
		return nil, "", err
	}
	iface := &emit.Interface{
		Origin:     sum.Origin,
		Doc:        v.Doc,
		Comment:    v.Comment,
		Name:       name,
		Visibility: sum.Visibility,
		TypeParams: lowering.CopyTypeParams(sum.TypeParams),
		Extends:    lowering.CopyTypeRefs(sum.Implements),
	}
	payload := v.Fields.Items()
	fields := make([]*emit.Field, 0, 1+len(payload))
	fields = append(fields, &emit.Field{
		Origin: v.Origin,
		Name:   discriminant,
		Type:   &emit.TypeRef{Spelling: quote(v.Name, typescript.DoubleQuote)},
	})
	for _, f := range payload {
		if f.Name == discriminant {
			return nil, "", refuse("the discriminant property is named %s, and so is a payload entry in %s",
				discriminant, v.Name)
		}
		fields = append(fields, f)
	}
	iface.Fields.Append(fields...)
	part := name
	if len(sum.TypeParams) > 0 {
		args := make([]string, 0, len(sum.TypeParams))
		for _, p := range sum.TypeParams {
			args = append(args, p.Name)
		}
		part += "<" + strings.Join(args, ", ") + ">"
	}
	return iface, part, nil
}
