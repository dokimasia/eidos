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
	var typeParams []*emit.TypeParam
	if len(m.TypeParams) > 0 {
		typeParams = make([]*emit.TypeParam, 0, len(m.TypeParams))
	}
	for _, tp := range m.TypeParams {
		typeParams = append(typeParams, mirrorTypeParam(tp))
	}
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
		TypeParams: typeParams,
		Params:     params,
		Returns:    returns,
		Throws:     throws,
	}
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
	for _, b := range tp.Bounds {
		out.Bounds = append(out.Bounds, rules.EmitRef(b))
	}
	return out
}
