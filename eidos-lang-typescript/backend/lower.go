// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// discriminant is the property every lowered variant interface
// leads with, typed to the variant's own name as a string literal,
// which is what narrows a union member back to its variant.
const discriminant = "kind"

// Lower reshapes the constructs TypeScript states in other
// declarations. A sum becomes one interface per variant and a
// union alias keeping the sum's name: each interface leads with
// the discriminant property typed to the variant's literal name,
// carries the variant's payload behind it, and restates the sum's
// type parameters; the alias's target joins the interfaces with
// the union bar, restating the parameters as arguments.
//
// The union target is a composite spelling, which the settle
// leaves as written, so the variant interfaces cannot wait for the
// respell: the lowering joins the sum's name and each variant's in
// the neutral camel form, spells the join final through the
// module's own convention, and writes the same spelling into the
// interface and the union both, consistent by construction. The
// alias keeps the sum's name untouched, so a reference to the sum
// follows the settle the way any reference does. The discriminant
// literal keeps the variant's declared name, because the property
// is data and a respell must not move what a wire may carry.
//
// Every output carries the sum's origin and none restates the sum,
// so a second settle changes nothing. A sum stating methods
// refuses, because a union carries no members; one stating no
// variants refuses, because a union joins at least one; a
// decorated sum or variant refuses, because TypeScript decorates
// classes alone; and a payload entry without a name refuses,
// because a property carries one. Everything else passes through
// unchanged.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	sum, held := s.(*emit.Sum)
	if !held {
		return nil, nil // the declaration stands
	}
	switch {
	case sum.Methods.Len() > 0:
		return nil, fmt.Errorf(
			"typescript: a union carries no members, and %s states methods",
			sum.Name,
		)
	case sum.Variants.Len() == 0:
		return nil, fmt.Errorf(
			"typescript: a union joins at least one variant, and %s states "+
				"none", sum.Name,
		)
	case len(sum.Annotations) > 0:
		return nil, fmt.Errorf(
			"typescript: decorators apply to classes, and the sum %s states "+
				"some", sum.Name,
		)
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
		Name:       sum.Name,
		Visibility: sum.Visibility,
		TypeParams: sum.TypeParams,
		Target:     &emit.TypeRef{Spelling: strings.Join(parts, " | ")},
	}), nil
}

// variantInterface builds one variant's interface and the spelling
// the union restates it by, the type arguments included.
func variantInterface(
	sum *emit.Sum, v *emit.SumVariant,
) (*emit.Interface, string, error) {
	if len(v.Annotations) > 0 {
		return nil, "", fmt.Errorf(
			"typescript: decorators apply to classes, and the variant %s "+
				"states some", v.Name,
		)
	}
	name, err := spell.Name(symbol.KindInvalid, symbol.KindInterface,
		sum.Visibility, sum.Name+naming.Pascal(v.Name))
	if err != nil {
		return nil, "", err
	}
	iface := &emit.Interface{
		Origin:     sum.Origin,
		Doc:        v.Doc,
		Name:       name,
		Visibility: sum.Visibility,
		TypeParams: lowering.CopyTypeParams(sum.TypeParams),
	}
	iface.Fields.Append(&emit.Field{
		Origin: v.Origin,
		Name:   discriminant,
		Type:   &emit.TypeRef{Spelling: strconv.Quote(v.Name)},
	})
	for _, f := range v.Fields.Items() {
		if f.Name == "" {
			return nil, "", fmt.Errorf(
				"typescript: a property carries a name, and a payload entry "+
					"in %s states none", v.Name,
			)
		}
		iface.Fields.Append(f)
	}
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
