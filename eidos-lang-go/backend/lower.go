// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang/naming"
)

// underlying is the defined type an enum lowers over. A variant's
// stated value spells verbatim against it, so a generator stating
// values states int-shaped ones.
const underlying = "int"

// Lower reshapes the constructs Go states in other declarations.
// An enum becomes a defined type and one typed constant per
// variant: the type keeps the enum's name, each constant joins the
// type's name and its variant's in the neutral camel form, its
// type references the defined type, and its value is the variant's
// stated spelling or its ordinal, because a constant declared
// alone cannot count through iota. Every output carries the
// enum's origin and none restates the enum, so a second settle
// changes nothing.
//
// An enum carrying fields or methods refuses: a constant group
// holds no members, and Go declares no other closed value set.
// Everything else passes through unchanged, a sum included, whose
// unspelt kind the render reports.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	e, held := s.(*emit.Enum)
	if !held {
		return []symbol.Symbol{s}, nil
	}
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return nil, fmt.Errorf(
			"go: a constant group holds no members, and %s states some", e.Name)
	}
	variants := e.Variants.Items()
	out := make([]symbol.Symbol, 0, 1+len(variants))
	out = append(out, &emit.Alias{
		Origin:      e.Origin,
		Doc:         e.Doc,
		Name:        e.Name,
		Visibility:  e.Visibility,
		Defined:     true,
		Target:      &emit.TypeRef{Spelling: underlying},
		Annotations: e.Annotations,
	})
	for i, v := range variants {
		value := v.Value
		if value == "" {
			value = strconv.Itoa(i)
		}
		out = append(out, &emit.Constant{
			Origin:      e.Origin,
			Doc:         v.Doc,
			Name:        e.Name + naming.Pascal(v.Name),
			Visibility:  e.Visibility,
			Type:        &emit.TypeRef{Spelling: e.Name},
			Value:       value,
			Annotations: v.Annotations,
		})
	}
	return out, nil
}
