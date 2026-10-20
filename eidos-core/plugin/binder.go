// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// Binder is the role of a backend whose target writes code against the
// code generator of a gRPC binding, such as the messages of protoc-gen-go.
// The emitter calls it for the handler of an rpc, and the layout calls it
// for the package of a bound reference, whose [emit.TypeRef.Bound] is set.
//
// # Concurrency
//
// An implementation is safe for concurrent use, as a TypeSpeller is.
type Binder interface {
	// Handler returns the method that serves the rpc r under the policy p.
	// translate spells a reference of r through the spoke of the plan. Under
	// a policy that selects no binding, Handler returns the method of
	// [CanonicalHandler] in the form of its target. It returns an error for
	// an rpc that the binding cannot serve, and the error of translate.
	Handler(r RPC, p Policy, translate Translate) (*emit.Method, error)
	// Home returns the package of the code that the code generator of a
	// binding writes for target. schema places the source file of target at
	// the path where the default placement of the plan writes the code of
	// the file. Home returns an error for a target whose code no binding of
	// the target generates.
	Home(target symbol.Identity, schema Placement) (symbol.Identity, error)
}

// Translate spells a reference of the graph in the target of a plan, as
// [eidos.Emitter.Type] translates a type, and returns the reason of a
// refusal. [eidos.Emitter.Handler] passes one to a [Binder] for the
// references of an rpc.
type Translate func(ref *node.TypeRef) (*emit.TypeRef, error)

// RPC is an rpc that a [Binder] serves: the identity, the documentation and
// the visibility of the rpc as the load placed it, the service that declares
// it, and its projection, which has its name.
type RPC struct {
	// ID is the identity of the rpc, and Service the identity of the
	// interface of its service.
	ID, Service symbol.Identity
	// Doc is the documentation of the rpc.
	Doc []string
	// Visibility is the visibility of the rpc.
	Visibility symbol.Visibility
	// Callable is the projection of the rpc under the rules of its
	// language, with the context of the call among its parameters.
	Callable rules.Callable
}

// CanonicalHandler returns the canonical method that serves the rpc r. The
// method has the origin, the documentation, the name and the visibility of
// the rpc, and the asynchrony of its projection. It has a parameter for each
// parameter of the projection and a return for each return, in their order,
// and translate spells each reference. Under [rules.ErrorsRaised] the method
// states its failure in Throws, as types spells the well-known error under
// the policy p. Another error model states the failure in the returns of the
// projection, and CanonicalHandler does not read types, so the caller may
// pass nil. A backend without a binder serves this method, and a binder
// starts from it under a policy that selects no binding.
//
// Error modes: the error of translate, and the error of types for a target
// without a spelling of the well-known error.
//
// # Allocation contract
//
// CanonicalHandler allocates the method, a parameter for each parameter
// view, a return for each return view, the storage of each slot that is not
// empty, and what translate allocates for each reference. A failure
// allocates its list and what types allocates for its reference.
func CanonicalHandler(r RPC, p Policy, types TypeSpeller, translate Translate) (*emit.Method, error) {
	c := r.Callable
	params := make([]*emit.Param, 0, len(c.Params))
	for _, v := range c.Params {
		t, err := translate(v.Ref)
		if err != nil {
			return nil, err
		}
		params = append(params, &emit.Param{Name: v.Name, Type: t})
	}
	returns := make([]*emit.Return, 0, len(c.Returns))
	for _, v := range c.Returns {
		t, err := translate(v.Ref)
		if err != nil {
			return nil, err
		}
		returns = append(returns, &emit.Return{Name: v.Name, Type: t})
	}
	out := &emit.Method{
		Origin: r.ID, Doc: r.Doc, Name: c.Name, Visibility: r.Visibility, Async: c.Async,
		Params: emit.SlotOf(params...), Returns: emit.SlotOf(returns...),
	}
	if c.Errors == rules.ErrorsRaised {
		failure, err := types.SpellType(rules.Reference(rules.WellKnownError.Name, rules.WellKnownError), p)
		if err != nil {
			return nil, err
		}
		out.Throws = []*emit.TypeRef{failure}
	}
	return out, nil
}
