// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// schemaLang is the language of the schema of the binder cases.
const schemaLang symbol.Lang = "protobuf"

// The schema of the binder cases: its package and its file, the module
// that contains it, and the file, the import path and the name of the Go
// package that protoc-gen-go writes beside the schema.
const (
	schemaPkg    = "acme.svc"
	schemaFile   = "svc/store.proto"
	schemaModule = "example.com/acme"
	messagesFile = "svc/store.pb.go"
	messagesPath = schemaModule + "/svc"
	messagesName = "svc"
)

// The rpc of the binder cases: the name of its service, its name and its
// documentation, the names of its context and its request, the names of
// its messages, and the stream type that protoc-gen-connect-go writes for
// it.
const (
	storeService  = "Store"
	getRPC        = "Get"
	getDoc        = "Get returns the session of a request."
	contextName   = "ctx"
	requestName   = "request"
	getRequestMsg = "GetRequest"
	sessionMsg    = "Session"
	storeStream   = "StoreGetServerStream"
)

// The package of grpc-go, and the stream types of a server that
// protoc-gen-go-grpc writes into a handler.
const (
	grpcPath     = "google.golang.org/grpc"
	serverStream = "grpc.ServerStreamingServer"
	clientStream = "grpc.ClientStreamingServer"
	bidiStream   = "grpc.BidiStreamingServer"
)

// The allocations of the handlers and the homes that the allocation test
// and the benchmark measure, with a translation that allocates nothing.
const (
	// unaryAllocs is a unary rpc under grpc-go: the method, its failure and
	// the list of its failure, the list of the parameters, the context with
	// its reference, the request, and the return with its list.
	unaryAllocs = 9
	// connectStreamAllocs is a server stream under connect-go: the method,
	// its failure and the list of its failure, the list of the parameters,
	// the context with its reference, the request, the stream with its
	// reference, and the name of the stream type.
	connectStreamAllocs = 10
	// canonicalAllocs is a unary rpc without a binding: the method, the
	// list of the parameters, the context and the request, the list of the
	// returns, the return, and the failure with its list.
	canonicalAllocs = 8
	// serviceHomeAllocs is the connect package of a service in a directory
	// with a Go package: its path, which ends in its name.
	serviceHomeAllocs = 1
)

// The declarations of the schema that the binder cases reference.
var (
	storeID      = symbol.Identity{Lang: schemaLang, Package: schemaPkg, Name: storeService, Kind: symbol.KindInterface}
	getRequestID = symbol.Identity{Lang: schemaLang, Package: schemaPkg, Name: getRequestMsg, Kind: symbol.KindStruct}
	sessionID    = symbol.Identity{Lang: schemaLang, Package: schemaPkg, Name: sessionMsg, Kind: symbol.KindStruct}
	stateID      = symbol.Identity{
		Lang: schemaLang, Package: schemaPkg, Owner: sessionMsg, Name: "State", Kind: symbol.KindEnum,
	}
)

// The references of the rpcs of the binder cases: the context, each
// message, and a stream of each message.
var (
	contextRef    = &node.TypeRef{Spelling: "context", Target: rules.WellKnownContext}
	getRequestRef = &node.TypeRef{Spelling: getRequestMsg, Target: getRequestID}
	sessionRef    = &node.TypeRef{Spelling: sessionMsg, Target: sessionID}
	requestStream = &node.TypeRef{Form: symbol.FormStream, Elems: []*node.TypeRef{getRequestRef}}
	sessionStream = &node.TypeRef{Form: symbol.FormStream, Elems: []*node.TypeRef{sessionRef}}
)

// The spellings of the messages under a binding: each message, which is
// bound, and the pointer to it. contextArg is the context parameter that a
// handler takes.
var (
	getRequestType    = &emit.TypeRef{Spelling: getRequestMsg, Target: getRequestID, Bound: true}
	getRequestPointer = &emit.TypeRef{
		Spelling: "*" + getRequestMsg, Form: symbol.FormOptional, Elems: []*emit.TypeRef{getRequestType},
	}
	sessionType    = &emit.TypeRef{Spelling: sessionMsg, Target: sessionID, Bound: true}
	sessionPointer = &emit.TypeRef{
		Spelling: "*" + sessionMsg, Form: symbol.FormOptional, Elems: []*emit.TypeRef{sessionType},
	}
	contextArg = &emit.Param{Name: contextName, Type: &emit.TypeRef{Spelling: "context.Context", Package: "context"}}
)

// The Go package that protoc-gen-go writes the messages of the schema
// into, the connect package of protoc-gen-connect-go below it, and the
// placement of the schema in the directory of that package.
var (
	messagesPkg = symbol.Identity{
		Lang: golang.Lang, Package: messagesPath, Name: messagesName, Kind: symbol.KindPackage,
	}
	connectPkg = symbol.Identity{
		Lang: golang.Lang, Package: messagesPath + "/svcconnect", Name: "svcconnect", Kind: symbol.KindPackage,
	}
	messagesSchema = plugin.Placement{
		Path: schemaFile, Residents: []plugin.Resident{{File: messagesFile, Pkg: messagesPkg}},
	}
)

// getCall is the rpc Get of the service Store without its parameters and
// its returns. It is asynchronous, and it raises its failure, as the rules
// of protobuf state.
var getCall = plugin.RPC{
	ID:         symbol.Identity{Lang: schemaLang, Package: schemaPkg, Owner: storeService, Name: getRPC},
	Service:    storeID,
	Doc:        []string{getDoc},
	Visibility: symbol.VisibilityPublic,
	Callable:   rules.Callable{Name: getRPC, Errors: rules.ErrorsRaised, Async: true},
}

// errUntranslated is the reason of the translation of the cases that
// refuses a reference.
var errUntranslated = errors.New("spell_test: the translation refuses the reference")

// handlerCall is one handler that the allocation test and the benchmark
// measure: the rpc, the policy, the ceiling and the number of parameters
// of the method.
type handlerCall struct {
	name   string
	give   plugin.RPC
	policy plugin.Policy
	allocs uint64
	want   int
}

// A binding of gRPC serves an rpc in the form of the code generator of the
// binding, and the code of that generator is in a package beside the
// schema.
func TestBinder(t *testing.T) {
	t.Parallel()

	grpc, err := plugin.NewPolicy(golang.Target, golang.Policies(),
		map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCGo})
	assert.NoError(t, err, "the binding grpc-go is a choice of the policy")
	connect, err := plugin.NewPolicy(golang.Target, golang.Policies(),
		map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCConnect})
	assert.NoError(t, err, "the binding connect-go is a choice of the policy")

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()

		forms := []struct {
			name         string
			givePolicy   plugin.Policy
			giveRequest  *node.TypeRef
			giveResponse *node.TypeRef
			wantParams   []*emit.Param
			wantReturns  []*emit.Return
		}{
			{
				name:       "returns the canonical method of the projection for no binding",
				givePolicy: plugin.Policy{}, giveRequest: getRequestRef, giveResponse: sessionRef,
				wantParams: []*emit.Param{
					contextArg, {Name: requestName, Type: &emit.TypeRef{Spelling: getRequestMsg, Target: getRequestID}},
				},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: sessionMsg, Target: sessionID}}},
			},
			{
				name:       "returns the method of protoc-gen-go-grpc for a unary rpc",
				givePolicy: grpc, giveRequest: getRequestRef, giveResponse: sessionRef,
				wantParams:  []*emit.Param{contextArg, {Name: requestName, Type: getRequestPointer}},
				wantReturns: []*emit.Return{{Type: sessionPointer}},
			},
			{
				name:       "returns the method of protoc-gen-go-grpc for a server stream",
				givePolicy: grpc, giveRequest: getRequestRef, giveResponse: sessionStream,
				wantParams: []*emit.Param{
					{Name: requestName, Type: getRequestPointer},
					{Name: "stream", Type: &emit.TypeRef{
						Spelling: serverStream, Package: grpcPath, Args: []*emit.TypeRef{sessionType},
					}},
				},
			},
			{
				name:       "returns a stream type of the reference itself for a stream of an enum",
				givePolicy: grpc, giveRequest: getRequestRef,
				giveResponse: &node.TypeRef{
					Form: symbol.FormStream, Elems: []*node.TypeRef{{Spelling: "State", Target: stateID}},
				},
				wantParams: []*emit.Param{
					{Name: requestName, Type: getRequestPointer},
					{Name: "stream", Type: &emit.TypeRef{
						Spelling: serverStream, Package: grpcPath,
						Args: []*emit.TypeRef{{Spelling: "Session_State", Target: stateID, Bound: true}},
					}},
				},
			},
			{
				name:       "returns the method of protoc-gen-go-grpc for a client stream",
				givePolicy: grpc, giveRequest: requestStream, giveResponse: sessionRef,
				wantParams: []*emit.Param{{Name: "stream", Type: &emit.TypeRef{
					Spelling: clientStream, Package: grpcPath, Args: []*emit.TypeRef{getRequestType, sessionType},
				}}},
			},
			{
				name:       "returns the method of protoc-gen-go-grpc for a bidi stream",
				givePolicy: grpc, giveRequest: requestStream, giveResponse: sessionStream,
				wantParams: []*emit.Param{{Name: "stream", Type: &emit.TypeRef{
					Spelling: bidiStream, Package: grpcPath, Args: []*emit.TypeRef{getRequestType, sessionType},
				}}},
			},
			{
				name:       "returns the method of protoc-gen-connect-go for a unary rpc",
				givePolicy: connect, giveRequest: getRequestRef, giveResponse: sessionRef,
				wantParams:  []*emit.Param{contextArg, {Name: requestName, Type: getRequestPointer}},
				wantReturns: []*emit.Return{{Type: sessionPointer}},
			},
			{
				name:       "returns the method of protoc-gen-connect-go for a server stream",
				givePolicy: connect, giveRequest: getRequestRef, giveResponse: sessionStream,
				wantParams: []*emit.Param{
					contextArg,
					{Name: requestName, Type: getRequestPointer},
					{Name: "stream", Type: &emit.TypeRef{Spelling: storeStream, Target: storeID, Bound: true}},
				},
			},
			{
				name:       "returns the method of protoc-gen-connect-go for a client stream",
				givePolicy: connect, giveRequest: requestStream, giveResponse: sessionRef,
				wantParams: []*emit.Param{
					contextArg,
					{Name: "stream", Type: &emit.TypeRef{Spelling: storeStream, Target: storeID, Bound: true}},
				},
				wantReturns: []*emit.Return{{Type: sessionPointer}},
			},
			{
				name:       "returns the method of protoc-gen-connect-go for a bidi stream",
				givePolicy: connect, giveRequest: requestStream, giveResponse: sessionStream,
				wantParams: []*emit.Param{
					contextArg,
					{Name: "stream", Type: &emit.TypeRef{Spelling: storeStream, Target: storeID, Bound: true}},
				},
			},
		}
		for _, tt := range forms {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := getCall
				r.Callable.Params = []rules.ParamView{
					{Name: contextName, Ref: contextRef, Role: rules.ParamContext},
					{Name: requestName, Ref: tt.giveRequest},
				}
				r.Callable.Returns = []rules.ReturnView{{Ref: tt.giveResponse}}
				got, err := spell.Handler(r, tt.givePolicy, func(ref *node.TypeRef) (*emit.TypeRef, error) {
					return spell.Type(rules.Reference(ref.Spelling, ref.Target), tt.givePolicy)
				})
				assert.NoError(t, err, "the policy serves the rpc")
				expect.Equal(t, got.Params.Items(), tt.wantParams, "the parameters of the form")
				expect.Equal(t, got.Returns.Items(), tt.wantReturns, "the returns of the form", assert.EquateEmpty())
				expect.Equal(t, got.Throws, []*emit.TypeRef{{Spelling: "error"}}, "the method fails with an error")
				expect.False(t, got.Async, "Go has no asynchronous method")
			})
		}

		t.Run("returns a method that derives from the rpc under a binding", func(t *testing.T) {
			t.Parallel()

			r := getCall
			r.Callable.Params = []rules.ParamView{{Name: requestName, Ref: getRequestRef}}
			r.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
			got, err := spell.Handler(r, grpc, func(ref *node.TypeRef) (*emit.TypeRef, error) {
				return spell.Type(rules.Reference(ref.Spelling, ref.Target), grpc)
			})
			assert.NoError(t, err, "the binding serves the rpc")
			expect.Equal(t, got.Origin, r.ID, "the method derives from the rpc")
			expect.Equal(t, got.Doc, []string{getDoc}, "the method has the documentation of the rpc")
			expect.Equal(t, got.Name, getRPC, "the method has the name of the rpc")
			expect.Equal(t, got.Visibility, symbol.VisibilityPublic, "the method has the visibility of the rpc")
		})

		incomplete := []struct {
			name        string
			giveParams  []rules.ParamView
			giveReturns []rules.ReturnView
		}{
			{
				name:        "returns an error for an rpc without a request",
				giveParams:  []rules.ParamView{{Name: contextName, Ref: contextRef, Role: rules.ParamContext}},
				giveReturns: []rules.ReturnView{{Ref: sessionRef}},
			},
			{
				name:       "returns an error for an rpc without a response",
				giveParams: []rules.ParamView{{Name: requestName, Ref: getRequestRef}},
			},
		}
		for _, tt := range incomplete {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := getCall
				r.Callable.Params, r.Callable.Returns = tt.giveParams, tt.giveReturns
				_, err := spell.Handler(r, grpc, func(ref *node.TypeRef) (*emit.TypeRef, error) {
					return spell.Type(rules.Reference(ref.Spelling, ref.Target), grpc)
				})
				assert.HasError(t, err, "a binding serves an rpc with a request and a response")
				assert.Contains(t, err.Error(), "has no request or no response", "the error names the missing side")
			})
		}

		untranslated := []struct {
			name        string
			givePolicy  plugin.Policy
			giveRefused *node.TypeRef
		}{
			{name: "returns the error of translate for the request", givePolicy: grpc, giveRefused: getRequestRef},
			{name: "returns the error of translate for the response", givePolicy: grpc, giveRefused: sessionRef},
			{
				name:       "returns the error of translate for the canonical method",
				givePolicy: plugin.Policy{}, giveRefused: getRequestRef,
			},
		}
		for _, tt := range untranslated {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := getCall
				r.Callable.Params = []rules.ParamView{{Name: requestName, Ref: getRequestRef}}
				r.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
				_, err := spell.Handler(r, tt.givePolicy, func(ref *node.TypeRef) (*emit.TypeRef, error) {
					if ref == tt.giveRefused {
						return nil, errUntranslated
					}
					return spell.Type(rules.Reference(ref.Spelling, ref.Target), tt.givePolicy)
				})
				assert.ErrorIs(t, err, errUntranslated, "the error is the refusal of the translation")
			})
		}
	})

	t.Run("Home", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			giveTarget symbol.Identity
			giveSchema plugin.Placement
			want       symbol.Identity
		}{
			{
				name:       "returns the package of the directory of the schema for a message",
				giveTarget: getRequestID, giveSchema: messagesSchema, want: messagesPkg,
			},
			{
				name:       "returns the package of the directory of the schema for an enum",
				giveTarget: stateID, giveSchema: messagesSchema, want: messagesPkg,
			},
			{
				name:       "returns the connect package below the package of the directory for a service",
				giveTarget: storeID, giveSchema: messagesSchema, want: connectPkg,
			},
			{
				name:       "returns the package that the module derives for a directory without a Go package",
				giveTarget: getRequestID,
				giveSchema: plugin.Placement{
					Path: schemaFile, Modules: []plugin.Module{{Lang: golang.Lang, Path: schemaModule, Root: "."}},
				},
				want: messagesPkg,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Home(tt.giveTarget, tt.giveSchema)
				assert.NoError(t, err, "the code of protoc-gen-go has a package")
				assert.Equal(t, got, tt.want, "the package of the code")
			})
		}

		refusals := []struct {
			name       string
			giveTarget symbol.Identity
			giveSchema plugin.Placement
			want       string
		}{
			{
				name:       "returns an error for a target of another kind",
				giveTarget: symbol.Identity{Lang: schemaLang, Name: getRPC, Kind: symbol.KindFunction},
				giveSchema: messagesSchema, want: "writes no Go type",
			},
			{
				name:       "returns an error for a directory outside every module",
				giveTarget: getRequestID, giveSchema: plugin.Placement{Path: schemaFile}, want: "import base",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Home(tt.giveTarget, tt.giveSchema)
				assert.HasError(t, err, "the code of protoc-gen-go has no package")
				assert.Contains(t, err.Error(), tt.want, "the error has the reason")
			})
		}
	})
}

// Handler allocates the method that it returns, and Home allocates the
// connect package of a service alone. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestBinderAllocs(t *testing.T) {
	for _, tt := range handlerCalls(t) {
		var (
			got *emit.Method
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Handler(tt.give, tt.policy, spelled) }, tt.allocs,
			"Handler of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Handler serves "+tt.name)
		assert.Equal(t, got.Params.Len(), tt.want, "Handler of "+tt.name+" returns the parameters of its form")
	}
	var (
		got symbol.Identity
		err error
	)
	assert.MaxAllocs(t, func() { got, err = spell.Home(getRequestID, messagesSchema) }, 0,
		"Home allocates nothing for a message in a directory with a Go package")
	assert.NoError(t, err, "Home derives the package of a message")
	assert.Equal(t, got, messagesPkg, "Home returns the package of the directory")
	assert.MaxAllocs(t, func() { got, err = spell.Home(storeID, messagesSchema) }, serviceHomeAllocs,
		"Home allocates the path of the connect package")
	assert.NoError(t, err, "Home derives the package of a service")
	assert.Equal(t, got, connectPkg, "Home returns the connect package")
}

// BenchmarkBinder measures the handler that a generator asks for once per
// rpc, and the package that the layout asks for once per bound reference.
func BenchmarkBinder(b *testing.B) {
	b.Run("Handler", func(b *testing.B) {
		for _, tt := range handlerCalls(b) {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got *emit.Method
					err error
				)
				for c.Loop() {
					got, err = spell.Handler(tt.give, tt.policy, spelled)
				}
				assert.NoError(b, err, "Handler serves "+tt.name)
				assert.Equal(b, got.Params.Len(), tt.want, "Handler of "+tt.name+" returns its parameters")
			})
		}
	})

	b.Run("Home", func(b *testing.B) {
		for _, tt := range []struct {
			name   string
			give   symbol.Identity
			allocs uint64
			want   symbol.Identity
		}{
			{name: "a message", give: getRequestID, want: messagesPkg},
			{name: "a service", give: storeID, allocs: serviceHomeAllocs, want: connectPkg},
		} {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got symbol.Identity
					err error
				)
				for c.Loop() {
					got, err = spell.Home(tt.give, messagesSchema)
				}
				assert.NoError(b, err, "Home derives the package")
				assert.Equal(b, got, tt.want, "Home returns the package")
			})
		}
	})
}

// spelled is a translation that allocates nothing: it returns the pointer
// to the session for the session and for a stream of it, the context for
// the context, and the pointer to the request for every other reference.
func spelled(ref *node.TypeRef) (*emit.TypeRef, error) {
	switch ref {
	case sessionRef, sessionStream:
		return sessionPointer, nil
	case contextRef:
		return contextArg.Type, nil
	default:
		return getRequestPointer, nil
	}
}

// handlerCalls returns the handlers that the allocation test and the
// benchmark measure: a unary rpc under grpc-go, a server stream under
// connect-go, and a unary rpc without a binding.
func handlerCalls(tb testing.TB) []handlerCall {
	tb.Helper()

	grpc, err := plugin.NewPolicy(golang.Target, golang.Policies(),
		map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCGo})
	assert.NoError(tb, err, "the binding grpc-go is a choice of the policy")
	connect, err := plugin.NewPolicy(golang.Target, golang.Policies(),
		map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCConnect})
	assert.NoError(tb, err, "the binding connect-go is a choice of the policy")
	unary, stream := getCall, getCall
	unary.Callable.Params = []rules.ParamView{
		{Name: contextName, Ref: contextRef, Role: rules.ParamContext}, {Name: requestName, Ref: getRequestRef},
	}
	unary.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
	stream.Callable.Params = unary.Callable.Params
	stream.Callable.Returns = []rules.ReturnView{{Ref: sessionStream}}
	return []handlerCall{
		{name: "a unary rpc under grpc-go", give: unary, policy: grpc, allocs: unaryAllocs, want: 2},
		{
			name: "a server stream under connect-go", give: stream, policy: connect, allocs: connectStreamAllocs,
			want: 3,
		},
		{name: "a unary rpc without a binding", give: unary, policy: plugin.Policy{}, allocs: canonicalAllocs, want: 2},
	}
}
