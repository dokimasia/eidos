// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lower reshapes the constructs Java states in other declarations.
// A sum becomes the sealed principal interface keeping the sum's
// name and one final class per variant: each class joins the sum's
// name and its variant's in the neutral camel form, carries the
// variant's payload as its fields, restates the sum's type
// parameters, and implements the principal through a reference
// resolved to the sum's origin, so every name follows the settle
// the way the rest of the store does. The principal permits each
// class the same resolved way, because the split files every type
// apart and Java then demands the enumeration spelled.
//
// Every output carries the sum's origin and none restates the sum,
// so a second settle changes nothing. A sum stating methods
// refuses, because a variant class would owe bodies the model does
// not carry; a payload entry without a name refuses, because a
// field carries one. Everything else passes through unchanged.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	sum, held := s.(*emit.Sum)
	if !held {
		return nil, nil // the declaration stands
	}
	if sum.Methods.Len() > 0 {
		return nil, fmt.Errorf(
			"java: a variant class would owe method bodies the model does "+
				"not carry, and %s states methods", sum.Name,
		)
	}
	variants := sum.Variants.Items()
	permits := make([]*emit.TypeRef, 0, len(variants))
	for _, v := range variants {
		permits = append(permits, &emit.TypeRef{
			Target:   sum.Origin,
			Spelling: sum.Name + naming.Pascal(v.Name),
		})
	}
	out := make([]symbol.Symbol, 0, 1+len(variants))
	out = append(out, &emit.Interface{
		Origin:      sum.Origin,
		Doc:         sum.Doc,
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

// variantClass builds one variant's final implementing class.
func variantClass(sum *emit.Sum, v *emit.SumVariant) (*emit.Struct, error) {
	cls := &emit.Struct{
		Origin:      sum.Origin,
		Doc:         v.Doc,
		Name:        sum.Name + naming.Pascal(v.Name),
		Visibility:  sum.Visibility,
		Final:       true,
		TypeParams:  copyTypeParams(sum.TypeParams),
		Implements:  []*emit.TypeRef{principalRef(sum)},
		Annotations: v.Annotations,
	}
	for _, f := range v.Fields.Items() {
		if f.Name == "" {
			return nil, fmt.Errorf(
				"java: a field carries a name, and a payload entry in %s "+
					"states none", v.Name,
			)
		}
		cls.Fields.Append(f)
	}
	return cls, nil
}

// principalRef references the principal interface, resolved to the
// sum's origin so the settle follows it precisely, the sum's
// parameters restated as arguments.
func principalRef(sum *emit.Sum) *emit.TypeRef {
	t := &emit.TypeRef{Target: sum.Origin, Spelling: sum.Name}
	for _, p := range sum.TypeParams {
		t.Args = append(t.Args, &emit.TypeRef{Spelling: p.Name})
	}
	return t
}

// copyTypeParams restates a parameter list without sharing nodes,
// so the settle's walks visit each output's list once: fresh
// parameters, fresh bound and default references.
func copyTypeParams(ps []*emit.TypeParam) []*emit.TypeParam {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(ps))
	for _, p := range ps {
		c := *p
		c.Bounds = copyTypeRefs(p.Bounds)
		c.Default = copyTypeRef(p.Default)
		c.Type = copyTypeRef(p.Type)
		out = append(out, &c)
	}
	return out
}

// copyTypeRefs restates references without sharing nodes.
func copyTypeRefs(ts []*emit.TypeRef) []*emit.TypeRef {
	if len(ts) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, copyTypeRef(t))
	}
	return out
}

// copyTypeRef restates one reference tree without sharing nodes.
func copyTypeRef(t *emit.TypeRef) *emit.TypeRef {
	if t == nil {
		return nil
	}
	c := *t
	c.Args = copyTypeRefs(t.Args)
	return &c
}
