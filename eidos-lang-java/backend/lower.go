// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// variantSep separates the name of a sum from the name of its variant in
// the neutral name of a variant class, which the respell joins into one
// Pascal-case word.
const variantSep = "_"

// Lower reshapes the constructs Java states in other declarations,
// and refuses a file-level type that states what only a member type
// can spell. The settle lowers file-level declarations alone, so the
// refusal leaves member types to the kind templates.
//
// A sum becomes the sealed principal interface keeping the sum's
// name and one final class per variant: each class joins the sum's
// name and its variant's with a word separator, takes the
// variant's payload as its fields, restates the sum's type
// parameters, and implements the principal through a reference
// resolved to the sum's origin, so every name follows the settle
// the way the rest of the store does. The principal permits each
// class the same resolved way, because the split files every type
// apart and Java then demands the enumeration spelled.
//
// Every output has the sum's origin and none restates the sum, so a
// second settle changes nothing. A sum stating methods refuses,
// because a variant class would owe bodies the model does not
// state. A payload entry without a name refuses, because a field has
// one. Everything else passes through unchanged.
//
// # Allocation contract
//
// A declaration that passes through allocates nothing. A sum allocates
// the list of permits and per variant its reference and its joined
// name, the list of outputs and the principal, and per variant its
// class, its joined name, its implements list and reference, the copy of
// its type parameters, and its field list where it has fields. A refusal
// allocates its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if err := fileLevel(s); err != nil {
		return nil, err
	}
	sum, is := s.(*emit.Sum)
	if !is {
		return nil, nil // the declaration passes through
	}
	if sum.Methods.Len() > 0 {
		return nil, refuse("a variant class would owe method bodies the model does "+
			"not state, and %s states methods", sum.Name)
	}
	variants := sum.Variants.Items()
	if len(variants) == 0 {
		// javac rejects a sealed type with no permits clause, so a
		// variantless sum has no legal Java spelling.
		return nil, refuse("a sealed interface owes its permits clause, and %s "+
			"states no variants", sum.Name)
	}
	permits := make([]*emit.TypeRef, 0, len(variants))
	for _, v := range variants {
		permits = append(permits, &emit.TypeRef{
			Target:   sum.Origin,
			Spelling: sum.Name + variantSep + v.Name,
		})
	}
	out := make([]symbol.Symbol, 0, 1+len(variants))
	out = append(out, &emit.Interface{
		Origin:      sum.Origin,
		Doc:         sum.Doc,
		Comment:     sum.Comment,
		Name:        sum.Name,
		Visibility:  sum.Visibility,
		Sealed:      true,
		TypeParams:  sum.TypeParams,
		Permits:     permits,
		Annotations: sum.Annotations,
	})
	for _, v := range variants {
		cls, err := variantClass(sum, v)
		if err != nil {
			return nil, err
		}
		out = append(out, cls)
	}
	return out, nil
}

// fileLevel refuses what a file-level type states that javac rejects
// at file scope: a private or protected scope, and the type-level
// binding a member class spells as static.
func fileLevel(s symbol.Symbol) error {
	var name string
	var v symbol.Visibility
	switch t := s.(type) {
	case *emit.Struct:
		if t.Level == symbol.LevelType {
			return refuse("a file-level class takes no static, and %s states a type-level binding",
				t.Name)
		}
		name, v = t.Name, t.Visibility
	case *emit.Interface:
		name, v = t.Name, t.Visibility
	case *emit.Enum:
		name, v = t.Name, t.Visibility
	case *emit.Sum:
		name, v = t.Name, t.Visibility
	default:
		return nil
	}
	if v == symbol.VisibilityPrivate || v == symbol.VisibilityProtected {
		return refuse("a file-level type takes public or default access, and %s states a "+
			"narrower scope", name)
	}
	return nil
}

// variantClass returns the final class of the variant v, which implements
// the principal interface of the sum. The reference to the principal has
// the origin of the sum as its target, so the settle follows it, and the
// type parameters of the sum as its arguments. variantClass returns an
// error for a payload entry without a name.
func variantClass(sum *emit.Sum, v *emit.SumVariant) (*emit.Struct, error) {
	if err := lowering.NamedPayload(java.Lang, v); err != nil {
		return nil, err
	}
	cls := &emit.Struct{
		Origin:     sum.Origin,
		Doc:        v.Doc,
		Comment:    v.Comment,
		Name:       sum.Name + variantSep + v.Name,
		Visibility: sum.Visibility,
		Final:      true,
		TypeParams: lowering.CopyTypeParams(sum.TypeParams),
		Implements: []*emit.TypeRef{{
			Target: sum.Origin, Spelling: sum.Name, Args: lowering.ParamArgs(sum.TypeParams),
		}},
		Annotations: v.Annotations,
	}
	cls.Fields.Append(v.Fields.Items()...)
	return cls, nil
}
