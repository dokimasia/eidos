// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of grpc-go and the stream types of a server that
// protoc-gen-go-grpc writes into a handler.
const (
	grpcPackage      = "google.golang.org/grpc"
	grpcServerStream = "grpc.ServerStreamingServer"
	grpcClientStream = "grpc.ClientStreamingServer"
	grpcBidiStream   = "grpc.BidiStreamingServer"
)

// The suffix of the name of a stream type of protoc-gen-connect-go and of
// the name of its package, the names of the parameters of the context and
// of a stream, and the separator of an import path.
const (
	connectStream = "ServerStream"
	connectSuffix = "connect"
	contextParam  = "ctx"
	streamParam   = "stream"
	packageSep    = "/"
)

// handlerParams is the most parameters that a handler of a binding takes:
// the context, the request and the stream.
const handlerParams = 3

// typeSpoke serves [Type] as the type spoke of the Go target, which
// [plugin.CanonicalHandler] spells the failure through.
type typeSpoke struct{}

// SpellType implements [plugin.TypeSpeller] through [Type].
func (typeSpoke) SpellType(s rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	return Type(s, p)
}

// Handler returns the method that serves the rpc r in Go under the binding
// that [golang.GRPC] selects in p. translate spells a reference of the rpc
// through the spoke of the plan. Every method declares error in Throws,
// which the Go backend lowers into its error result:
//
//   - Under [golang.GRPCNone] the method is the method of
//     [plugin.CanonicalHandler] without its asynchrony, because Go has no
//     asynchronous method: the context, the request and the response of the
//     projection.
//   - Under [golang.GRPCGo] the method has the form of protoc-gen-go-grpc
//     v1.6.2. A unary rpc takes the context and the request, and returns
//     the response. A server stream takes the request and a
//     grpc.ServerStreamingServer of the response, a client stream a
//     grpc.ClientStreamingServer of the request and the response, and a bidi
//     stream a grpc.BidiStreamingServer of both.
//   - Under [golang.GRPCConnect] the method has the form of
//     protoc-gen-connect-go v2.0.1. Every rpc takes the context. A unary rpc
//     takes the request and returns the response. A server stream takes the
//     request and its stream, a client stream takes its stream and returns
//     the response, and a bidi stream takes its stream. The stream is the
//     struct <Service><Method>ServerStream, a bound reference to the
//     service, which [Home] places in the connect package.
//
// A message is a pointer, as the spoke spells it under a binding, and a
// stream type takes its messages without the pointer.
//
// Error modes: the error of translate, and an error for an rpc without a
// request or a response.
//
// # Allocation contract
//
// Handler allocates what translate allocates, and the method that it
// returns: the method, its failure and the list of its failure, each
// parameter and return with the reference that it adds, and the lists of
// the parameters and the returns. A stream type of grpc-go allocates the
// list of its arguments, and a stream type of connect-go allocates its
// name. A refusal allocates its error.
func Handler(r plugin.RPC, p plugin.Policy, translate plugin.Translate) (*emit.Method, error) {
	choice, _ := p.Lookup(golang.GRPC)
	if choice != golang.GRPCGo && choice != golang.GRPCConnect {
		m, err := plugin.CanonicalHandler(r, p, typeSpoke{}, translate)
		if err != nil {
			return nil, err
		}
		m.Async = false
		return m, nil
	}
	name := r.Callable.Name
	var request rules.ParamView
	for _, v := range r.Callable.Params {
		if v.Role != rules.ParamContext {
			request = v
		}
	}
	if request.Ref == nil || len(r.Callable.Returns) == 0 {
		return nil, fmt.Errorf("spell: the rpc %s has no request or no response, so no binding of gRPC serves it",
			name)
	}
	response := r.Callable.Returns[0].Ref
	in, err := side(request.Ref, translate)
	if err != nil {
		return nil, err
	}
	out, err := side(response, translate)
	if err != nil {
		return nil, err
	}
	m := &emit.Method{
		Origin: r.ID, Doc: r.Doc, Name: name, Visibility: r.Visibility,
		Throws: []*emit.TypeRef{{Spelling: errorType}},
	}
	clientStreams, serverStreams := request.Ref.Form == symbol.FormStream, response.Form == symbol.FormStream
	var stream *emit.TypeRef
	switch {
	case choice == golang.GRPCConnect && (clientStreams || serverStreams):
		spelling := make([]byte, 0, len(r.Service.Name)+len(name)+len(connectStream))
		spelling = append(appendProtoc(appendProtoc(spelling, r.Service.Name), name), connectStream...)
		stream = &emit.TypeRef{Spelling: string(spelling), Target: r.Service, Bound: true}
	case clientStreams && serverStreams:
		stream = &emit.TypeRef{
			Spelling: grpcBidiStream, Package: grpcPackage, Args: []*emit.TypeRef{in.message, out.message},
		}
	case clientStreams:
		stream = &emit.TypeRef{
			Spelling: grpcClientStream, Package: grpcPackage, Args: []*emit.TypeRef{in.message, out.message},
		}
	case serverStreams:
		stream = &emit.TypeRef{Spelling: grpcServerStream, Package: grpcPackage, Args: []*emit.TypeRef{out.message}}
	}
	params := make([]*emit.Param, 0, handlerParams)
	// Every form of connect-go and the unary form of grpc-go take the
	// context first.
	if choice == golang.GRPCConnect || stream == nil {
		params = append(params, &emit.Param{
			Name: contextParam, Type: &emit.TypeRef{Spelling: contextType, Package: contextPackage},
		})
	}
	if !clientStreams {
		params = append(params, &emit.Param{Name: request.Name, Type: in.pointer})
	}
	if stream != nil {
		params = append(params, &emit.Param{Name: streamParam, Type: stream})
	}
	m.Params = emit.SlotOf(params...)
	// A unary rpc returns its response, and so does a client stream under
	// connect-go.
	if !serverStreams && (!clientStreams || choice == golang.GRPCConnect) {
		m.Returns = emit.SlotOf(&emit.Return{Type: out.pointer})
	}
	return m, nil
}

// Home returns the Go package of the code that protoc-gen-go and
// protoc-gen-connect-go write for target with paths=source_relative. schema
// places the source file of target where the plan writes the code of the
// file. The package of a message and of an enum is the package of the
// directory of schema, by the rule of [Package]: the package that the Go
// files of the directory declare, then the module that contains the
// directory, then the import base of the plan. The package of a service is
// the connect package of protoc-gen-connect-go below it, whose path and name
// are the path and the name of the directory's package followed by connect,
// as svc/svcconnect.
//
// Error modes: the error of [Package] for a directory without a package,
// and an error for a target of another kind, which protoc-gen-go writes no
// type for.
//
// # Allocation contract
//
// Home allocates what [Package] allocates for the directory, and the path
// of the connect package of a service, one allocation. A refusal allocates
// its error.
func Home(target symbol.Identity, schema plugin.Placement) (symbol.Identity, error) {
	switch target.Kind {
	case symbol.KindStruct, symbol.KindEnum, symbol.KindInterface:
	default:
		return symbol.Identity{}, fmt.Errorf("spell: protoc-gen-go writes no Go type for the %s %s",
			target.Kind, target)
	}
	pkg, err := directory(schema)
	if err != nil || target.Kind != symbol.KindInterface {
		return pkg, err
	}
	// The name of the connect package is the last element of its path.
	connect := pkg.Package + packageSep + pkg.Name + connectSuffix
	return symbol.Identity{
		Lang: golang.Lang, Package: connect, Name: connect[len(pkg.Package)+len(packageSep):], Kind: symbol.KindPackage,
	}, nil
}

// handlerSide is one side of an rpc under a binding: the pointer to its
// message, which a unary handler takes or returns, and the message itself,
// which a stream type takes as a type argument.
type handlerSide struct {
	pointer *emit.TypeRef
	message *emit.TypeRef
}

// side spells the side ref of an rpc through translate: the message of a
// stream, or the reference itself. A pointer, which the spoke spells for a
// message under a binding, has the message as its one child.
func side(ref *node.TypeRef, translate plugin.Translate) (handlerSide, error) {
	if ref.Form == symbol.FormStream && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	t, err := translate(ref)
	if err != nil {
		return handlerSide{}, err
	}
	if t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		return handlerSide{pointer: t, message: t.Elems[0]}, nil
	}
	return handlerSide{pointer: t, message: t}, nil
}
