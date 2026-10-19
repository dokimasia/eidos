// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/core/emit"
)

// Mentioned returns the type parameters of tps that the references refs
// mention, in the order of tps. It adds each type parameter that the
// bounds, the default or the value type of a returned parameter mention,
// so a declaration of the result declares every parameter that its own
// parameters use. Mentioned returns the parameters of tps themselves, and
// nil where no reference mentions one.
//
// A named reference mentions a parameter where its spelling is the name of
// the parameter, or where a type argument of it mentions the parameter. A
// qualified spelling, such as pkg.V, names a declaration of another
// package and mentions no parameter. A structural reference mentions a
// parameter where one of its children mentions it. The children of an
// inline body are the types of its members, so the name of a member is no
// mention. A nil reference mentions nothing.
//
// # Allocation contract
//
// Mentioned allocates the list that it returns, one allocation, and
// nothing where it returns nil. A walk over more than 32 type parameters
// also allocates its flags, one allocation, because Go places a list of
// variable length on the stack up to 32 bytes.
func Mentioned(tps []*emit.TypeParam, refs ...*emit.TypeRef) []*emit.TypeParam {
	in := make([]bool, len(tps))
	for i, tp := range tps {
		in[i] = slices.ContainsFunc(refs, func(t *emit.TypeRef) bool { return mentions(t, tp.Name) })
	}
	for grew := true; grew; {
		grew = false
		for i, tp := range tps {
			named := func(t *emit.TypeRef) bool { return mentions(t, tp.Name) }
			for j, user := range tps {
				if !in[i] && in[j] &&
					(slices.ContainsFunc(user.Bounds, named) || named(user.Default) || named(user.Type)) {

					in[i], grew = true, true
				}
			}
		}
	}
	n := 0
	for _, used := range in {
		if used {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, n)
	for i, tp := range tps {
		if in[i] {
			out = append(out, tp)
		}
	}
	return out
}

// mentions reports whether the reference t mentions the type parameter
// named name: its spelling is the name, or a type argument or a child of
// it mentions the name.
func mentions(t *emit.TypeRef, name string) bool {
	if t == nil {
		return false
	}
	if t.Spelling == name {
		return true
	}
	byChild := func(c *emit.TypeRef) bool { return mentions(c, name) }
	return slices.ContainsFunc(t.Args, byChild) || slices.ContainsFunc(t.Elems, byChild)
}
