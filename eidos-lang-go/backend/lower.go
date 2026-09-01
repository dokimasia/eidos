// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// underlying is the defined type an enum lowers over. A variant's
// stated value spells verbatim against it, so a generator stating
// values states int-shaped ones.
const underlying = "int"

// errorType is the return an announced failure lowers into, which
// is how Go declares one.
const errorType = "error"

// Lower reshapes the constructs Go states in other declarations:
// an enum becomes a defined type and its constants, and a callable
// announcing failure types gains an error return, in place. Both
// consume their fact, so a second settle changes nothing, and
// everything else passes through as it stands, a sum included,
// whose unspelt kind the render reports.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	switch d := s.(type) {
	case *emit.Enum:
		return lowerEnum(d)
	case *emit.Function:
		d.Returns, d.Throws = thrown(d.Returns, d.Throws), nil
	case *emit.Method:
		d.Returns, d.Throws = thrown(d.Returns, d.Throws), nil
	case *emit.Struct:
		lowerMembers(d.Methods.Items())
	case *emit.Interface:
		lowerMembers(d.Methods.Items())
	}
	return nil, nil
}

// lowerMembers rewrites a host's member methods the way the
// file-level callables rewrite, because the lowering receives the
// host whole.
func lowerMembers(methods []*emit.Method) {
	for _, m := range methods {
		m.Returns, m.Throws = thrown(m.Returns, m.Throws), nil
	}
}

// thrown appends the error return a callable's announced failure
// types lower into: one error whatever the count, because Go's
// failures are values of one interface and the concrete types
// arrive through errors.As. A callable announcing none keeps its
// returns untouched.
func thrown(returns []*emit.Return, throws []*emit.TypeRef) []*emit.Return {
	if len(throws) == 0 {
		return returns
	}
	return append(returns, &emit.Return{
		Type: &emit.TypeRef{Spelling: errorType},
	})
}

// lowerEnum reshapes an enum into a defined type and one typed
// constant per variant: the type keeps the enum's name, each
// constant joins the type's name and its variant's in the neutral
// camel form, its type references the defined type, and its value
// is the variant's stated spelling or its ordinal, because a
// constant declared alone cannot count through iota. Every output
// carries the enum's origin and none restates the enum.
//
// An enum carrying fields or methods refuses: a constant group
// holds no members, and Go declares no other closed value set.
func lowerEnum(e *emit.Enum) ([]symbol.Symbol, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return nil, fmt.Errorf(
			"go: a constant group holds no members, and %s states some", e.Name,
		)
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
