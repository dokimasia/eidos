// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the parameters of a handler of Connect-ES: the request of
// a unary rpc and of a server stream, the requests of a client stream and
// of a bidi stream, and the context.
const (
	requestParam  = "request"
	requestsParam = "requests"
	contextParam  = "context"
)

// moduleSuffix ends the name of the file that protoc-gen-es writes for a
// schema file, as store_pb.ts for store.proto.
const moduleSuffix = "_pb" + typescript.Extension

// Handler returns the method that serves the rpc r in TypeScript under the
// binding that [typescript.GRPC] selects in p. translate spells a reference
// of the rpc through the spoke of the plan.
//
//   - Under [typescript.GRPCNone] the method is the method of
//     [plugin.CanonicalHandler] without a failure, because a TypeScript
//     method throws its failure and declares none.
//   - Under [typescript.GRPCConnect] the method has the form of the
//     ServiceImpl of Connect-ES v2.2.0. It takes the request, or the
//     requests of a client stream, and then the context, which the spoke
//     spells as the HandlerContext of Connect-ES. It returns the response.
//     A unary rpc and a client stream are asynchronous, so they return a
//     Promise of the response. A server stream and a bidi stream return
//     the AsyncIterable of their responses.
//
// Error modes: the error of translate, and an error for an rpc without a
// request, a response or a context.
//
// # Allocation contract
//
// Handler allocates what translate allocates, and the method that it
// returns: under [typescript.GRPCConnect] the method, the request and the
// context with their list, and the return with its list. Under
// [typescript.GRPCNone] it allocates what [plugin.CanonicalHandler]
// allocates. A refusal allocates its error.
func Handler(r plugin.RPC, p plugin.Policy, translate plugin.Translate) (*emit.Method, error) {
	if binding, _ := p.Lookup(typescript.GRPC); binding != typescript.GRPCConnect {
		// The method states no failure, so the canonical handler spells
		// none.
		r.Callable.Errors = rules.ErrorsNone
		return plugin.CanonicalHandler(r, p, nil, translate)
	}
	c := r.Callable
	var request, ctx rules.ParamView
	for _, v := range c.Params {
		if v.Role == rules.ParamContext {
			ctx = v
		} else {
			request = v
		}
	}
	if request.Ref == nil || ctx.Ref == nil || len(c.Returns) == 0 {
		return nil, fmt.Errorf("spell: the rpc %s has no request, no context or no response, "+
			"so no handler of Connect-ES serves it", c.Name)
	}
	response := c.Returns[0].Ref
	in, err := translate(request.Ref)
	if err != nil {
		return nil, err
	}
	context, err := translate(ctx.Ref)
	if err != nil {
		return nil, err
	}
	out, err := translate(response)
	if err != nil {
		return nil, err
	}
	name := requestParam
	if request.Ref.Form == symbol.FormStream {
		name = requestsParam
	}
	return &emit.Method{
		Origin: r.ID, Doc: r.Doc, Name: c.Name, Visibility: r.Visibility,
		Async:   response.Form != symbol.FormStream,
		Params:  emit.SlotOf(&emit.Param{Name: name, Type: in}, &emit.Param{Name: contextParam, Type: context}),
		Returns: emit.SlotOf(&emit.Return{Type: out}),
	}, nil
}

// Home returns the module of the code that protoc-gen-es writes for
// target: the module of the file <stem>_pb.ts beside the source file that
// schema places, as svc/store_pb for svc/store.proto. protoc-gen-es writes
// a type for a message and an enum alone.
//
// Error modes: an error for a target of another kind.
//
// # Allocation contract
//
// Home allocates the path of the file of the module, one allocation. A
// refusal allocates its error.
func Home(target symbol.Identity, schema plugin.Placement) (symbol.Identity, error) {
	if target.Kind != symbol.KindStruct && target.Kind != symbol.KindEnum {
		return symbol.Identity{}, fmt.Errorf("spell: protoc-gen-es writes no TypeScript type for the %s %s",
			target.Kind, target)
	}
	stem := strings.TrimSuffix(schema.Path, path.Ext(schema.Path))
	return Package(plugin.Placement{Path: stem + moduleSuffix})
}
