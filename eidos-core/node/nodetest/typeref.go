// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest

import (
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Named returns a reference to a type that the fixture does not declare,
// such as a builtin, with args as its type arguments. The load leaves it
// without a target, unless it spells a type parameter in scope, which the
// load resolves as it resolves one in source. The reference keeps args as
// its arguments.
func Named(spelling string, args ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Args: args}
}

// External returns a reference to a type of the package pkg outside the
// fixture, with args as its type arguments. pkg is what an import of the
// language names, such as an import path of Go. The load leaves the
// reference without a target. The reference keeps args as its arguments.
func External(pkg, spelling string, args ...*node.TypeRef) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Package: pkg, Args: args}
}

// Optional returns an optional of elem. A nil elem is an empty reference.
func Optional(elem *node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormOptional, elem)
}

// List returns a list of elem. A nil elem is an empty reference.
func List(elem *node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormList, elem)
}

// Array returns an array of length elements of elem. A nil elem is an empty
// reference.
func Array(length int, elem *node.TypeRef) *node.TypeRef {
	ref := plugin.Composite("", position.Pos{}, symbol.FormArray, elem)
	ref.Length = length
	return ref
}

// Map returns a map from key to value. A nil key or value is an empty
// reference.
func Map(key, value *node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormMap, key, value)
}

// Stream returns a stream of elem. A nil elem is an empty reference. A test
// sets Async on the result for a stream whose elements arrive
// asynchronously.
func Stream(elem *node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormStream, elem)
}

// Borrow returns a borrow of elem. A nil elem is an empty reference.
func Borrow(elem *node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormBorrow, elem)
}

// Func returns a function type with params as its parameters and results as
// its results. Its Split is the index of the first result. A nil parameter
// or result is an empty reference. A function type without results keeps
// params as its children.
func Func(params, results []*node.TypeRef) *node.TypeRef {
	return plugin.FuncType("", position.Pos{}, params, results)
}

// Tuple returns a tuple of the members, in order. A nil member is an empty
// reference.
func Tuple(members ...*node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormTuple, members...)
}

// Union returns an untagged union of the members, in order. A nil member is
// an empty reference.
func Union(members ...*node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormUnion, members...)
}

// Intersection returns an intersection of the members, in order. A nil
// member is an empty reference.
func Intersection(members ...*node.TypeRef) *node.TypeRef {
	return plugin.Composite("", position.Pos{}, symbol.FormIntersection, members...)
}

// Wildcard returns a wildcard with variance as its variance and bound as its
// bound. A nil bound is an unbounded wildcard without a child. Every
// frontend records an unbounded wildcard as invariant, so a test passes
// [symbol.VarianceInvariant] with a nil bound.
func Wildcard(variance symbol.Variance, bound *node.TypeRef) *node.TypeRef {
	var ref *node.TypeRef
	if bound == nil {
		ref = plugin.Composite("", position.Pos{}, symbol.FormWildcard)
	} else {
		ref = plugin.Composite("", position.Pos{}, symbol.FormWildcard, bound)
	}
	ref.Variance = variance
	return ref
}

// Inline returns an inline body with fields as its fields and methods as its
// methods. The reference keeps both lists. Its children are copies of the
// types of the members, as a frontend records them for a body whose fields
// come before its methods: the type of each field, then for each method the
// bounds and the default of each type parameter, the type of its receiver,
// its parameter types and its result types. A member without a type has
// no child, and each child is a copy that [node.CloneRef] returns, because
// a node of a fixture has one place.
func Inline(fields []*node.Field, methods []*node.Method) *node.TypeRef {
	ref := &node.TypeRef{Form: symbol.FormInline, Fields: fields, Methods: methods}
	for _, f := range fields {
		if f.Type != nil {
			ref.Elems = append(ref.Elems, node.CloneRef(f.Type))
		}
	}
	for _, m := range methods {
		for _, tp := range m.TypeParams {
			for _, b := range tp.Bounds {
				ref.Elems = append(ref.Elems, node.CloneRef(b))
			}
			if tp.Default != nil {
				ref.Elems = append(ref.Elems, node.CloneRef(tp.Default))
			}
		}
		if m.Receiver != nil && m.Receiver.Type != nil {
			ref.Elems = append(ref.Elems, node.CloneRef(m.Receiver.Type))
		}
		for _, p := range m.Params {
			if p.Type != nil {
				ref.Elems = append(ref.Elems, node.CloneRef(p.Type))
			}
		}
		for _, r := range m.Returns {
			if r.Type != nil {
				ref.Elems = append(ref.Elems, node.CloneRef(r.Type))
			}
		}
	}
	return ref
}
