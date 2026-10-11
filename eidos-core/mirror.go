// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
)

// Mirror returns an emit method mirroring a node method's
// signature, type spellings verbatim, origin set. The signature is
// the name, the visibility, the level, the type parameters, the
// parameters with their labels, defaults, optional and variadic
// forms, the results, the async flag and the announced failure
// types. Receives names host, so the settle scopes the method under
// its host as it scopes a parsed method.
//
// The receiver is left unset, because its spelling is the target's:
// a Go stub states a pointer receiver through the Go satellite's
// helper, and Rust spells self. Imports are collected at render as a
// side effect of spelling types, so Mirror infers none.
//
// # Allocation contract
//
// Mirror allocates the method and its receiving type, and for each
// list of the signature that has entries the list, each entry and each
// entry's restated type. A method of one parameter and one return
// allocates eight times.
func Mirror(host string, m *node.Method) *emit.Method {
	params := make([]*emit.Param, 0, len(m.Params))
	for _, p := range m.Params {
		params = append(params, &emit.Param{
			Name:     p.Name,
			Label:    p.Label,
			Type:     rules.EmitRef(p.Type),
			Default:  p.Default,
			Optional: p.Optional,
			Variadic: p.Variadic,
		})
	}
	returns := make([]*emit.Return, 0, len(m.Returns))
	for _, r := range m.Returns {
		returns = append(returns, &emit.Return{
			Name: r.Name,
			Type: rules.EmitRef(r.Type),
		})
	}
	var throws []*emit.TypeRef
	if len(m.Throws) > 0 {
		throws = make([]*emit.TypeRef, 0, len(m.Throws))
	}
	for _, t := range m.Throws {
		throws = append(throws, rules.EmitRef(t))
	}
	return &emit.Method{
		Origin:     m.ID,
		Name:       m.Name,
		Visibility: m.Visibility,
		Level:      m.Level,
		Async:      m.Async,
		Receives:   &emit.TypeRef{Spelling: host},
		TypeParams: MirrorTypeParams(m.TypeParams),
		Params:     params,
		Returns:    returns,
		Throws:     throws,
	}
}

// MirrorTypeParams returns the emit form of the node type parameters tps,
// in order: each parameter's name, variance, bounds, default, value type
// and default value, with each type reference restated through
// [rules.EmitRef]. It returns nil for no parameters. A generator lifts the
// type parameters of a generic declaration onto the declaration that it
// emits for it, such as the mock of a generic interface, whose methods
// name the parameters.
//
// # Allocation contract
//
// MirrorTypeParams allocates nothing for no parameters. Otherwise it
// allocates the list, each parameter, the list of each parameter's bounds,
// and what [rules.EmitRef] allocates for each bound, default and value
// type. One parameter with one bound allocates four times.
func MirrorTypeParams(tps []*node.TypeParam) []*emit.TypeParam {
	if len(tps) == 0 {
		return nil
	}
	out := make([]*emit.TypeParam, 0, len(tps))
	for _, tp := range tps {
		out = append(out, mirrorTypeParam(tp))
	}
	return out
}

// TypeArgs returns the type parameters tps as the arguments of a
// reference inside the declaration that declares them, in order, and nil
// for no parameters. Each argument has the name of its parameter and no
// target. [Emitter.Type] translates a reference to a type parameter the
// same way. A generator writes the arguments where a generic declaration
// that it emits refers to itself or to the declaration that it derives
// from, such as the interface that the mock of a generic interface
// implements.
//
// # Allocation contract
//
// TypeArgs allocates nothing for no parameters. Otherwise it allocates the
// list and each argument. Two parameters allocate three times.
func TypeArgs(tps []*emit.TypeParam) []*emit.TypeRef {
	if len(tps) == 0 {
		return nil
	}
	out := make([]*emit.TypeRef, 0, len(tps))
	for _, tp := range tps {
		out = append(out, &emit.TypeRef{Spelling: tp.Name})
	}
	return out
}

// mirrorTypeParam restates a node type parameter in the emit model,
// bounds, default and value type included.
func mirrorTypeParam(tp *node.TypeParam) *emit.TypeParam {
	out := &emit.TypeParam{
		Name:         tp.Name,
		Variance:     tp.Variance,
		Default:      rules.EmitRef(tp.Default),
		Const:        tp.Const,
		Type:         rules.EmitRef(tp.Type),
		DefaultValue: tp.DefaultValue,
	}
	if len(tp.Bounds) > 0 {
		out.Bounds = make([]*emit.TypeRef, 0, len(tp.Bounds))
	}
	for _, b := range tp.Bounds {
		out.Bounds = append(out.Bounds, rules.EmitRef(b))
	}
	return out
}
