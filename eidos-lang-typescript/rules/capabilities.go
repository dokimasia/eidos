// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// unknownSpelling is TypeScript's top type, which is the witness of a
// type parameter without a bound. constructorName is the name of a
// class's constructor, and of the implicit constructor too.
const (
	unknownSpelling = "unknown"
	constructorName = "constructor"
)

// Derive returns the witness of a type parameter. The witness of a
// parameter with one bound is the bound, which every admitted type
// satisfies. The witness of a parameter without a bound is unknown.
// Derive reports false for a parameter with more than one bound, which
// TypeScript does not allow. It allocates the reference to unknown, one
// allocation, and nothing for a bound.
func (Rules) Derive(p *node.TypeParam, _ rules.View) (*node.TypeRef, bool) {
	switch {
	case p == nil || len(p.Bounds) > 1:
		return nil, false
	case len(p.Bounds) == 1 && p.Bounds[0] != nil:
		return p.Bounds[0], true
	default:
		return &node.TypeRef{Spelling: unknownSpelling}, true
	}
}

// Substitute restates a reference with each type parameter replaced by
// its argument. A named reference without arguments becomes a copy of
// the argument at a parameter's position when it resolved to the
// parameter. It also becomes the copy when it resolved to nothing and
// has the parameter's name. Substitute restates the children of a
// structural reference, the arguments of an instantiation, and the field
// types and method signatures of an inline object type. It copies what it
// rewrites, and returns a reference that does not refer to a parameter
// unchanged, with its children.
//
// # Allocation contract
//
// Substitute allocates a copy of each reference on the path to a
// replaced parameter, and a copy of each list that contains such a
// reference. It also allocates a copy of each member of an inline object
// type that refers to a parameter. A reference that does not refer to a
// parameter, and an empty parameter list, allocate nothing.
func (r Rules) Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef {
	if ref == nil || len(params) == 0 || len(params) != len(args) {
		return ref
	}
	if ref.Form == symbol.FormNamed && len(ref.Args) == 0 {
		for i, p := range params {
			if p != nil && args[i] != nil &&
				(!ref.Target.IsZero() && ref.Target == p.ID || ref.Target.IsZero() && ref.Spelling == p.Name) {
				c := *args[i]
				return &c
			}
		}
	}
	elems, elemsBound := r.substituteAll(ref.Elems, params, args)
	bound, argsBound := r.substituteAll(ref.Args, params, args)
	fields, fieldsBound := r.substituteFields(ref.Fields, params, args)
	methods, methodsBound := r.substituteMethods(ref.Methods, params, args)
	if !elemsBound && !argsBound && !fieldsBound && !methodsBound {
		return ref
	}
	c := *ref
	c.Elems, c.Args, c.Fields, c.Methods = elems, bound, fields, methods
	return &c
}

// substituteAll restates a list of references, and reports whether any
// of them refers to a parameter. It returns a list without such a
// reference unchanged. The first restated reference copies the list.
func (r Rules) substituteAll(
	refs []*node.TypeRef, params []*node.TypeParam, args []*node.TypeRef,
) ([]*node.TypeRef, bool) {
	var out []*node.TypeRef
	for i, ref := range refs {
		restated := r.Substitute(ref, params, args)
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

// substituteFields restates the fields of an inline object type, and
// reports whether the type of any field refers to a parameter. It
// returns a list without such a field unchanged. A restated field copies
// the field, and the first one also copies the list.
func (r Rules) substituteFields(
	fields []*node.Field, params []*node.TypeParam, args []*node.TypeRef,
) ([]*node.Field, bool) {
	var out []*node.Field
	for i, f := range fields {
		if f == nil {
			continue
		}
		restated := r.Substitute(f.Type, params, args)
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
// object type, and reports whether any signature refers to a parameter.
// It returns a list without such a method unchanged. A restated method
// copies the method, and the first one also copies the list.
func (r Rules) substituteMethods(
	methods []*node.Method, params []*node.TypeParam, args []*node.TypeRef,
) ([]*node.Method, bool) {
	var out []*node.Method
	for i, m := range methods {
		if m == nil {
			continue
		}
		var types []*node.TypeRef
		for _, p := range m.Params {
			if p != nil {
				types = append(types, p.Type)
			}
		}
		for _, ret := range m.Returns {
			if ret != nil {
				types = append(types, ret.Type)
			}
		}
		if _, named := r.substituteAll(types, params, args); !named {
			continue
		}
		if out == nil {
			out = slices.Clone(methods)
		}
		c := *m
		c.Params = make([]*node.Param, 0, len(m.Params))
		for _, p := range m.Params {
			if p != nil {
				pc := *p
				pc.Type = r.Substitute(p.Type, params, args)
				c.Params = append(c.Params, &pc)
			}
		}
		c.Returns = make([]*node.Return, 0, len(m.Returns))
		for _, ret := range m.Returns {
			if ret != nil {
				rc := *ret
				rc.Type = r.Substitute(ret.Type, params, args)
				c.Returns = append(c.Returns, &rc)
			}
		}
		out[i] = &c
	}
	if out == nil {
		return methods, false
	}
	return out, true
}

// Reified reports false, because TypeScript erases type arguments. A
// generic value has no reflection of its parameters at run time. Reified
// allocates nothing.
func (Rules) Reified() bool { return false }

// Properties returns the accessor properties of a class. Each instance
// getter is a property, in the order of the member walk, and the
// instance setter of the same name is its setter. The type of a property
// is the return type of its getter, or nil when the getter has no return
// type. A setter without a getter is not a property, because no code can
// read it.
//
// # Allocation contract
//
// Properties allocates the binding of the member walk, what the walk
// allocates, and the list of properties, sized to the walk's members. A
// class with one getter and its setter costs four allocations.
func (r Rules) Properties(s *node.Struct, v rules.View) []rules.Property {
	set, _ := rules.NewBound(r, v, nil).MembersOf(s)
	out := make([]rules.Property, 0, len(set.Members))
	for _, m := range set.Members {
		get, isMethod := m.Symbol.(*node.Method)
		if !isMethod || get.Accessor != symbol.AccessorGet || get.Level != symbol.LevelInstance {
			continue
		}
		p := rules.Property{Name: get.Name, Getter: get}
		if len(get.Returns) > 0 && get.Returns[0] != nil {
			p.Type = get.Returns[0].Type
		}
		for _, other := range set.Members {
			setter, isMethod := other.Symbol.(*node.Method)
			if isMethod && setter.Accessor == symbol.AccessorSet && setter.Level == symbol.LevelInstance &&
				setter.Name == get.Name {
				p.Setter = setter
				break
			}
		}
		out = append(out, p)
	}
	return out
}

// Constructors returns the public constructor signatures of a class, one
// per overload. A class without a constructor has the constructors of
// the class that it extends, with the type arguments of the extends
// clause substituted. A class that does not extend a class has the
// implicit constructor, which has no parameters. The list is empty in
// these cases:
//
//   - every constructor of the class is private or protected;
//   - the class extends a type that the view does not declare as a class;
//   - the chain of classes is deeper than the kernel's default depth.
//
// # Allocation contract
//
// Constructors allocates the binding of the projection, the list of
// callables, sized to the class's methods, and the lists of each
// callable. An inherited constructor of a generic class also allocates
// the copies that substitution makes.
func (r Rules) Constructors(s *node.Struct, v rules.View) []rules.Callable {
	b := rules.NewBound(r, v, nil)
	// A frame maps the type parameters of the class one level up to the
	// arguments of the extends clause that refers to that class.
	type frame struct {
		params []*node.TypeParam
		args   []*node.TypeRef
	}
	var frames []frame
	for range rules.DefaultDepth {
		var out []rules.Callable
		declared := false
		for _, m := range s.Methods {
			if m == nil || !m.Constructs {
				continue
			}
			declared = true
			if m.Visibility != symbol.VisibilityPublic && m.Visibility != symbol.VisibilityUnknown {
				continue
			}
			c, _ := b.CallableOf(m)
			for i := range c.Params {
				for _, f := range slices.Backward(frames) {
					c.Params[i].Ref = r.Substitute(c.Params[i].Ref, f.params, f.args)
				}
			}
			if out == nil {
				out = make([]rules.Callable, 0, len(s.Methods))
			}
			out = append(out, c)
		}
		switch {
		case declared:
			return out
		case len(s.Extends) == 0 || s.Extends[0] == nil:
			return []rules.Callable{{Name: constructorName}}
		}
		sym, _ := v.Lookup(s.Extends[0].Target)
		base, isClass := sym.(*node.Struct)
		if !isClass {
			return nil
		}
		frames = append(frames, frame{params: base.TypeParams, args: s.Extends[0].Args})
		s = base
	}
	return nil
}

// Comparable reports true for every TypeScript type, because === and a
// Map can compare any two values. It returns no problem and allocates
// nothing.
func (Rules) Comparable(*node.TypeRef, rules.View) (bool, []*node.TypeRef) { return true, nil }
