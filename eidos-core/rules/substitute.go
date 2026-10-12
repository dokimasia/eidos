// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Substitute returns ref with each reference to a parameter of params
// replaced by a copy of its argument in args. A reference to a parameter
// is a named reference without type arguments whose target is the
// identity of the parameter, because the resolution step targets a type
// parameter in scope. Substitute restates the children of a structural
// reference, the arguments of an instantiation, and the field types and
// method signatures of an inline body. It copies what it rewrites, so ref
// does not change. A restated structural reference has an empty
// spelling, because its written spelling contains the parameter, and a
// speller restates it from its children. A restated named reference
// keeps its spelling, which is its name, because its arguments spell
// apart. It returns ref unchanged where ref refers to no
// parameter, for a nil ref, for no parameters, and for a count of
// arguments that differs from the count of parameters. A nil parameter
// or a nil argument replaces nothing.
//
// # Allocation contract
//
// Substitute allocates a copy of each reference on the path to a
// replaced parameter, and a copy of each list that contains such a
// reference. It also allocates a copy of each member of an inline body
// that refers to a parameter, with the lists of a copied method. A
// reference that does not refer to a parameter, and an empty parameter
// list, allocate nothing.
func Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef {
	if ref == nil || len(params) == 0 || len(params) != len(args) {
		return ref
	}
	if ref.Form == symbol.FormNamed && len(ref.Args) == 0 && !ref.Target.IsZero() {
		for i, p := range params {
			if p != nil && args[i] != nil && ref.Target == p.ID {
				c := *args[i]
				return &c
			}
		}
	}
	elems, elemsBound := substituteAll(ref.Elems, params, args)
	bound, argsBound := substituteAll(ref.Args, params, args)
	fields, fieldsBound := substituteFields(ref.Fields, params, args)
	methods, methodsBound := substituteMethods(ref.Methods, params, args)
	if !elemsBound && !argsBound && !fieldsBound && !methodsBound {
		return ref
	}
	c := *ref
	c.Elems, c.Args, c.Fields, c.Methods = elems, bound, fields, methods
	if c.Form != symbol.FormNamed {
		c.Spelling = ""
	}
	return &c
}

// substituteAll restates a list of references, and reports whether any
// of them refers to a parameter. It returns a list without such a
// reference unchanged. The first restated reference copies the list.
func substituteAll(refs []*node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) ([]*node.TypeRef, bool) {
	var out []*node.TypeRef
	for i, ref := range refs {
		restated := Substitute(ref, params, args)
		if restated != ref && out == nil {
			out = slices.Clone(refs)
		}
		if out != nil {
			out[i] = restated
		}
	}
	if out == nil {
		return refs, false
	}
	return out, true
}

// substituteFields restates the fields of an inline body, and reports
// whether the type of any field refers to a parameter. It returns a list
// without such a field unchanged. A restated field copies the field, and
// the first one also copies the list.
func substituteFields(fields []*node.Field, params []*node.TypeParam, args []*node.TypeRef) ([]*node.Field, bool) {
	var out []*node.Field
	for i, f := range fields {
		if f == nil {
			continue
		}
		restated := Substitute(f.Type, params, args)
		if restated == f.Type {
			continue
		}
		if out == nil {
			out = slices.Clone(fields)
		}
		c := *f
		c.Type = restated
		out[i] = &c
	}
	if out == nil {
		return fields, false
	}
	return out, true
}

// substituteMethods restates the signatures of the methods of an inline
// body, and reports whether any signature refers to a parameter. It
// returns a list without such a method unchanged. A restated method
// copies the method and its parameter and return lists, and the first
// one also copies the list of methods.
func substituteMethods(methods []*node.Method, params []*node.TypeParam, args []*node.TypeRef) ([]*node.Method, bool) {
	var out []*node.Method
	for i, m := range methods {
		if m == nil || !signatureRefers(m, params, args) {
			continue
		}
		if out == nil {
			out = slices.Clone(methods)
		}
		c := *m
		c.Params = substituteParams(m.Params, params, args)
		c.Returns = substituteReturns(m.Returns, params, args)
		out[i] = &c
	}
	if out == nil {
		return methods, false
	}
	return out, true
}

// signatureRefers reports whether the type of a parameter or a return of
// m refers to a parameter of params that has an argument in args. It
// allocates nothing.
func signatureRefers(m *node.Method, params []*node.TypeParam, args []*node.TypeRef) bool {
	for _, p := range m.Params {
		if p != nil && refers(p.Type, params, args) {
			return true
		}
	}
	for _, r := range m.Returns {
		if r != nil && refers(r.Type, params, args) {
			return true
		}
	}
	return false
}

// refers reports whether ref, its children, its arguments or the members
// of its inline body refer to a parameter of params that has an argument
// in args, as [Substitute] matches one. It allocates nothing.
func refers(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) bool {
	if ref == nil {
		return false
	}
	if ref.Form == symbol.FormNamed && len(ref.Args) == 0 && !ref.Target.IsZero() {
		for i, p := range params {
			if p != nil && args[i] != nil && ref.Target == p.ID {
				return true
			}
		}
	}
	for _, e := range ref.Elems {
		if refers(e, params, args) {
			return true
		}
	}
	for _, a := range ref.Args {
		if refers(a, params, args) {
			return true
		}
	}
	for _, f := range ref.Fields {
		if f != nil && refers(f.Type, params, args) {
			return true
		}
	}
	for _, m := range ref.Methods {
		if m != nil && signatureRefers(m, params, args) {
			return true
		}
	}
	return false
}

// substituteParams restates a parameter list with each type restated. It
// copies each parameter and drops a nil one, and returns nil for an
// empty list.
func substituteParams(ps []*node.Param, params []*node.TypeParam, args []*node.TypeRef) []*node.Param {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*node.Param, 0, len(ps))
	for _, p := range ps {
		if p == nil {
			continue
		}
		c := *p
		c.Type = Substitute(p.Type, params, args)
		out = append(out, &c)
	}
	return out
}

// substituteReturns restates a return list with each type restated. It
// copies each return and drops a nil one, and returns nil for an empty
// list.
func substituteReturns(rs []*node.Return, params []*node.TypeParam, args []*node.TypeRef) []*node.Return {
	if len(rs) == 0 {
		return nil
	}
	out := make([]*node.Return, 0, len(rs))
	for _, r := range rs {
		if r == nil {
			continue
		}
		c := *r
		c.Type = Substitute(r.Type, params, args)
		out = append(out, &c)
	}
	return out
}
