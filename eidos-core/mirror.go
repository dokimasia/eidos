// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
)

// Mirror returns an emit method with the signature of the node method
// m. It copies the spelling of each type verbatim and sets the origin
// to the identity of m. The signature consists of the name, the
// visibility, the level, the type parameters, the parameters, the
// results, the async flag and the declared failure types. It also
// consists of the optional mark, the accessor, the index and the
// construct forms, the hard privacy, the mutability of an index
// signature and the computed mark. Each parameter keeps its label, its
// default and its optional and variadic forms. Mirror leaves out the
// facts of an implementation: abstract, final, override and a default
// body, because the generator writes the implementation. The Receives
// field of the result is host, so the settle scopes the method under
// host as it scopes a parsed method.
//
// Mirror leaves the receiver unset, because each target writes it in
// its own form. The Go satellite sets a pointer receiver on a stub
// through its own helper, and the Rust backend renders self. The render
// collects the imports when it renders the types, so Mirror adds no
// import.
//
// # Allocation contract
//
// Mirror allocates the method and its Receives type. For each list of
// the signature that has entries, it allocates the list, each entry and
// the restated type of each entry. A method with one parameter and one
// return allocates eight times.
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
			Passing:  p.Passing,
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
		Accessor:   m.Accessor,
		Indexer:    m.Indexer,
		Constructs: m.Constructs,
		Hard:       m.Hard,
		Optional:   m.Optional,
		Mutability: m.Mutability,
		Computed:   m.Computed,
		Receives:   &emit.TypeRef{Spelling: host},
		TypeParams: MirrorTypeParams(m.TypeParams),
		Params:     emit.SlotOf(params...),
		Returns:    emit.SlotOf(returns...),
		Throws:     throws,
	}
}

// MirrorTypeParams returns the emit form of the node type parameters tps,
// in order. Each result has the name, the variance, the bounds, the
// default, the value type and the default value of its parameter. It
// restates each type reference through [rules.EmitRef], and returns nil
// for no parameters. A generator copies the type parameters of a generic
// declaration to the declaration that it emits for it. The mock of a
// generic interface needs them, because its methods refer to the
// parameters.
//
// # Allocation contract
//
// MirrorTypeParams allocates nothing for no parameters. Otherwise it
// allocates the list, each parameter and the bound list of each
// parameter. It also allocates what [rules.EmitRef] allocates for each
// bound, default and value type. One parameter with one bound allocates
// four times.
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

// TypeArgs returns the type parameters tps, in order, as the type
// arguments of a reference inside the declaration that declares them.
// It returns nil for no parameters. Each argument has the name of its
// parameter and no target. [Emitter.Type] translates a reference to a
// type parameter the same way. A generator uses the arguments where a
// generic declaration that it emits refers to itself or to the
// declaration that it derives from. The mock of a generic interface
// uses them in the reference to the interface that it implements.
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

// mirrorTypeParam returns the emit form of the node type parameter tp,
// with its bounds, its default and its value type.
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
