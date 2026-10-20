// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// constructorName is the name of a class's constructor, and of the
// implicit constructor too.
const constructorName = "constructor"

// The fields of an error class that the error contract reads: the message
// and the cause that ECMAScript defines on an error.
const (
	messageField = "message"
	causeField   = "cause"
)

// The parameters of the constructors of the global error classes beside
// the message: the options of an error, and the errors that an
// AggregateError collects.
const (
	optionsParam = "options"
	errorsParam  = "errors"
)

// errorConstructor and aggregateConstructor are the constructors that
// TypeScript's standard library declares for the global error classes.
// Error and the native error classes take an optional message and
// options. AggregateError takes its errors before them.
var (
	errorConstructor = &node.Method{
		Name: constructorName, Constructs: true, Visibility: symbol.VisibilityPublic,
		Params: []*node.Param{
			{Name: messageField, Type: &node.TypeRef{Spelling: typescript.TypeString}, Optional: true},
			{Name: optionsParam, Type: &node.TypeRef{Spelling: typescript.TypeErrorOptions}, Optional: true},
		},
	}
	aggregateConstructor = &node.Method{
		Name: constructorName, Constructs: true, Visibility: symbol.VisibilityPublic,
		Params: append([]*node.Param{{
			Name: errorsParam,
			Type: &node.TypeRef{
				Spelling: typescript.TypeIterable, Args: []*node.TypeRef{{Spelling: typescript.TypeAny}},
			},
		}}, errorConstructor.Params...),
	}
)

// ErrorContract returns the contract of a class whose extends chain
// reaches a global error class of TypeScript, such as Error or TypeError,
// through classes that the view declares. Message and Cause are the
// fields message and cause among the members of the class, which inherit
// the fields of the classes of the chain. Each is nil where no class of
// the chain declares it. TypeScript has no hooks, so Unwrap, Is and As
// are nil. ErrorContract reports false for every other declaration, and
// for a chain deeper than the kernel's default depth.
//
// It allocates the binding of the member walk and what the walk
// allocates.
func (r Rules) ErrorContract(decl symbol.Symbol, v rules.View) (rules.ErrorContract, bool) {
	s, isClass := decl.(*node.Struct)
	if !isClass || s == nil {
		return rules.ErrorContract{}, false
	}
	base, reached := s, false
	for depth := 0; depth < rules.DefaultDepth && !reached; depth++ {
		if len(base.Extends) == 0 || base.Extends[0] == nil {
			return rules.ErrorContract{}, false
		}
		ext := base.Extends[0]
		if ext.Target.IsZero() {
			reached = ext.Package == "" && typescript.ErrorClass(ext.Spelling)
			if !reached {
				return rules.ErrorContract{}, false
			}
			continue
		}
		sym, _ := v.Lookup(ext.Target)
		next, isNext := sym.(*node.Struct)
		if !isNext {
			return rules.ErrorContract{}, false
		}
		base = next
	}
	if !reached {
		return rules.ErrorContract{}, false
	}
	var c rules.ErrorContract
	set, _ := rules.NewBound(r, v, nil).MembersOf(s)
	for _, m := range set.Members {
		f, isField := m.Symbol.(*node.Field)
		switch {
		case !isField:
		case f.Name == messageField && c.Message == nil:
			c.Message = f
		case f.Name == causeField && c.Cause == nil:
			c.Cause = f
		}
	}
	return c, true
}

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
		return &node.TypeRef{Spelling: typescript.TypeUnknown}, true
	}
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
// clause substituted. A class whose chain ends at a global error class,
// such as Error, has the constructor that TypeScript's standard library
// declares for that class. Error and the native error classes take
// (message?: string, options?: ErrorOptions). AggregateError takes
// (errors: Iterable<any>, message?: string, options?: ErrorOptions). A
// class that does not extend a class has the implicit constructor, which
// has no parameters. The list is empty in these cases:
//
//   - every constructor of the class is private or protected;
//   - the class extends a type that the view does not declare as a class,
//     and that is not a global error class;
//   - the chain of classes is deeper than the kernel's default depth.
//
// # Allocation contract
//
// Constructors allocates the binding of the projection, the list of
// callables, sized to the class's methods, and the lists of each
// callable. An inherited constructor of a generic class also allocates
// the copies that substitution makes. The constructor of a global error
// class allocates the list of one callable and the list of its
// parameters.
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
					c.Params[i].Ref = rules.Substitute(c.Params[i].Ref, f.params, f.args)
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
		if ext := s.Extends[0]; ext.Target.IsZero() && ext.Package == "" && typescript.ErrorClass(ext.Spelling) {
			ctor := errorConstructor
			if ext.Spelling == typescript.TypeAggregateError {
				ctor = aggregateConstructor
			}
			c, _ := b.CallableOf(ctor)
			return []rules.Callable{c}
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
