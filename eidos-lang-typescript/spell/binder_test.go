// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// schemaLang is the language of the schema of the binder cases.
const schemaLang symbol.Lang = "protobuf"

// The schema of the binder cases: its package and its file, and the file
// that protoc-gen-es writes beside it.
const (
	schemaPkg    = "acme.svc"
	schemaFile   = "svc/store.proto"
	messagesFile = "svc/store_pb.ts"
)

// The rpc of the binder cases: the name of its service, its name and its
// documentation, the names of its context and its request, and the names
// of its messages.
const (
	storeService  = "Store"
	getRPC        = "Get"
	getDoc        = "Get returns the session of a request."
	contextName   = "ctx"
	requestName   = "request"
	getRequestMsg = "GetRequest"
	sessionMsg    = "Session"
)

// The allocations of the handlers and the homes that the allocation test
// and the benchmark measure, with a translation that allocates nothing.
const (
	// connectAllocs is a unary rpc under connect-es: the method, the
	// request and the context with their list, and the return with its
	// list.
	connectAllocs = 6
	// canonicalAllocs is a unary rpc without a binding: the method, the
	// list of the parameters, the context and the request, the list of the
	// returns and the return.
	canonicalAllocs = 6
	// homeAllocs is the module of a message: the path of its file.
	homeAllocs = 1
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
// message, and an asynchronous stream of each message.
var (
	contextRef    = &node.TypeRef{Spelling: "context", Target: rules.WellKnownContext}
	getRequestRef = &node.TypeRef{Spelling: getRequestMsg, Target: getRequestID}
	sessionRef    = &node.TypeRef{Spelling: sessionMsg, Target: sessionID}
	requestStream = &node.TypeRef{Form: symbol.FormStream, Async: true, Elems: []*node.TypeRef{getRequestRef}}
	sessionStream = &node.TypeRef{Form: symbol.FormStream, Async: true, Elems: []*node.TypeRef{sessionRef}}
)

// The spellings of the references under the binding connect-es: each
// message, which is bound, the AsyncIterable of each message, and the
// HandlerContext of Connect-ES.
var (
	getRequestType = &emit.TypeRef{Spelling: getRequestMsg, Target: getRequestID, Bound: true}
	sessionType    = &emit.TypeRef{Spelling: sessionMsg, Target: sessionID, Bound: true}
	getRequests    = &emit.TypeRef{Spelling: "AsyncIterable", Args: []*emit.TypeRef{getRequestType}}
	sessions       = &emit.TypeRef{Spelling: "AsyncIterable", Args: []*emit.TypeRef{sessionType}}
	handlerContext = &emit.TypeRef{Spelling: "HandlerContext", Package: "@connectrpc/connect"}
)

// messagesModule is the module of the file that protoc-gen-es writes for
// the schema.
var messagesModule = symbol.Identity{
	Lang: typescript.Lang, Package: "svc/store_pb", Name: "store_pb", Kind: symbol.KindPackage,
}

// getCall is the rpc Get of the service Store without its parameters and
// its returns. It raises its failure, as the rules of protobuf state.
var getCall = plugin.RPC{
	ID:         symbol.Identity{Lang: schemaLang, Package: schemaPkg, Owner: storeService, Name: getRPC},
	Service:    storeID,
	Doc:        []string{getDoc},
	Visibility: symbol.VisibilityPublic,
	Callable:   rules.Callable{Name: getRPC, Errors: rules.ErrorsRaised},
}

// errUntranslated is the reason of the translation of the cases that
// refuses a reference.
var errUntranslated = errors.New("spell_test: the translation refuses the reference")

// handlerCall is one handler that the allocation test and the benchmark
// measure: the policy, the ceiling and the name of the first parameter of
// the method.
type handlerCall struct {
	name   string
	policy plugin.Policy
	allocs uint64
	want   string
}

// A handler of Connect-ES serves an rpc in the form of a ServiceImpl, and
// the code of protobuf-es is in a module beside the schema.
func TestBinder(t *testing.T) {
	t.Parallel()

	connect := policy(t, map[plugin.PolicyKey]plugin.Choice{typescript.GRPC: typescript.GRPCConnect})

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()

		forms := []struct {
			name         string
			givePolicy   plugin.Policy
			giveRequest  *node.TypeRef
			giveResponse *node.TypeRef
			wantParams   []*emit.Param
			wantReturns  []*emit.Return
			wantAsync    bool
		}{
			{
				name:       "returns the canonical method of the projection for no binding",
				givePolicy: policy(t, nil), giveRequest: getRequestRef, giveResponse: sessionRef,
				wantParams: []*emit.Param{
					{Name: contextName, Type: &emit.TypeRef{Spelling: "AbortSignal"}},
					{Name: requestName, Type: &emit.TypeRef{Spelling: getRequestMsg, Target: getRequestID}},
				},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: sessionMsg, Target: sessionID}}},
			},
			{
				name:       "returns the asynchronous method of Connect-ES for a unary rpc",
				givePolicy: connect, giveRequest: getRequestRef, giveResponse: sessionRef,
				wantParams: []*emit.Param{
					{Name: requestName, Type: getRequestType}, {Name: "context", Type: handlerContext},
				},
				wantReturns: []*emit.Return{{Type: sessionType}}, wantAsync: true,
			},
			{
				name:       "returns the method of Connect-ES for a server stream",
				givePolicy: connect, giveRequest: getRequestRef, giveResponse: sessionStream,
				wantParams: []*emit.Param{
					{Name: requestName, Type: getRequestType}, {Name: "context", Type: handlerContext},
				},
				wantReturns: []*emit.Return{{Type: sessions}},
			},
			{
				name:       "returns the asynchronous method of Connect-ES for a client stream",
				givePolicy: connect, giveRequest: requestStream, giveResponse: sessionRef,
				wantParams: []*emit.Param{
					{Name: "requests", Type: getRequests}, {Name: "context", Type: handlerContext},
				},
				wantReturns: []*emit.Return{{Type: sessionType}}, wantAsync: true,
			},
			{
				name:       "returns the method of Connect-ES for a bidi stream",
				givePolicy: connect, giveRequest: requestStream, giveResponse: sessionStream,
				wantParams: []*emit.Param{
					{Name: "requests", Type: getRequests}, {Name: "context", Type: handlerContext},
				},
				wantReturns: []*emit.Return{{Type: sessions}},
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
				got, err := spell.Handler(r, tt.givePolicy, translation(tt.givePolicy, nil))
				assert.NoError(t, err, "the policy serves the rpc")
				expect.Equal(t, got.Params.Items(), tt.wantParams, "the parameters of the form")
				expect.Equal(t, got.Returns.Items(), tt.wantReturns, "the returns of the form")
				expect.Empty(t, got.Throws, "a TypeScript method declares no failure")
				expect.Equal(t, got.Async, tt.wantAsync, "the asynchrony of the form")
			})
		}

		t.Run("returns a method that derives from the rpc under connect-es", func(t *testing.T) {
			t.Parallel()

			r := getCall
			r.Callable.Params = []rules.ParamView{
				{Name: contextName, Ref: contextRef, Role: rules.ParamContext}, {Name: requestName, Ref: getRequestRef},
			}
			r.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
			got, err := spell.Handler(r, connect, translation(connect, nil))
			assert.NoError(t, err, "the binding serves the rpc")
			expect.Equal(t, got.Origin, r.ID, "the method derives from the rpc")
			expect.Equal(t, got.Doc, []string{getDoc}, "the method has the documentation of the rpc")
			expect.Equal(t, got.Name, getRPC, "the method has the name of the rpc")
			expect.Equal(t, got.Visibility, symbol.VisibilityPublic, "the method has the visibility of the rpc")
		})

		context := rules.ParamView{Name: contextName, Ref: contextRef, Role: rules.ParamContext}
		request := rules.ParamView{Name: requestName, Ref: getRequestRef}
		incomplete := []struct {
			name        string
			giveParams  []rules.ParamView
			giveReturns []rules.ReturnView
		}{
			{
				name:       "returns an error for an rpc without a request",
				giveParams: []rules.ParamView{context}, giveReturns: []rules.ReturnView{{Ref: sessionRef}},
			},
			{
				name:       "returns an error for an rpc without a context",
				giveParams: []rules.ParamView{request}, giveReturns: []rules.ReturnView{{Ref: sessionRef}},
			},
			{
				name:       "returns an error for an rpc without a response",
				giveParams: []rules.ParamView{context, request},
			},
		}
		for _, tt := range incomplete {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := getCall
				r.Callable.Params, r.Callable.Returns = tt.giveParams, tt.giveReturns
				_, err := spell.Handler(r, connect, translation(connect, nil))
				assert.HasError(t, err, "a handler of Connect-ES takes a request and a context")
				assert.Contains(t, err.Error(), "has no request, no context or no response",
					"the error names the missing part")
			})
		}

		untranslated := []struct {
			name        string
			givePolicy  plugin.Policy
			giveRefused *node.TypeRef
		}{
			{name: "returns the error of translate for the request", givePolicy: connect, giveRefused: getRequestRef},
			{name: "returns the error of translate for the context", givePolicy: connect, giveRefused: contextRef},
			{name: "returns the error of translate for the response", givePolicy: connect, giveRefused: sessionRef},
			{
				name:       "returns the error of translate for the canonical method",
				givePolicy: policy(t, nil), giveRefused: getRequestRef,
			},
		}
		for _, tt := range untranslated {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := getCall
				r.Callable.Params = []rules.ParamView{context, request}
				r.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
				_, err := spell.Handler(r, tt.givePolicy, translation(tt.givePolicy, tt.giveRefused))
				assert.ErrorIs(t, err, errUntranslated, "the error is the refusal of the translation")
			})
		}
	})

	t.Run("Home", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			giveTarget symbol.Identity
		}{
			{name: "returns the module of the file of protoc-gen-es for a message", giveTarget: getRequestID},
			{name: "returns the module of the file of protoc-gen-es for an enum", giveTarget: stateID},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Home(tt.giveTarget, plugin.Placement{Path: schemaFile})
				assert.NoError(t, err, "the code of protoc-gen-es has a module")
				want, err := spell.Package(plugin.Placement{Path: messagesFile})
				assert.NoError(t, err, "the file of protoc-gen-es has a module")
				assert.Equal(t, got, want, "the module of the file beside the schema")
			})
		}

		t.Run("returns an error for a service", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Home(storeID, plugin.Placement{Path: schemaFile})
			assert.HasError(t, err, "protoc-gen-es writes no type for a service")
			assert.Contains(t, err.Error(), "writes no TypeScript type", "the error has the reason")
		})
	})
}

// Handler allocates the method that it returns, and Home allocates the
// path of the module. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestBinderAllocs(t *testing.T) {
	unary := unaryCall()
	for _, tt := range handlerCalls(t) {
		var (
			got *emit.Method
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Handler(unary, tt.policy, spelled) }, tt.allocs,
			"Handler of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Handler serves "+tt.name)
		assert.Equal(t, got.Params.Items()[0].Name, tt.want, "Handler of "+tt.name+" takes the parameters of its form")
	}
	var (
		got symbol.Identity
		err error
	)
	schema := plugin.Placement{Path: schemaFile}
	assert.MaxAllocs(t, func() { got, err = spell.Home(getRequestID, schema) }, homeAllocs,
		"Home allocates the path of the module")
	assert.NoError(t, err, "Home derives the module of a message")
	assert.Equal(t, got, messagesModule, "Home returns the module of the file of protoc-gen-es")
}

// BenchmarkBinder measures the handler that a generator asks for once per
// rpc, and the module that the layout asks for once per bound reference.
func BenchmarkBinder(b *testing.B) {
	b.Run("Handler", func(b *testing.B) {
		unary := unaryCall()
		for _, tt := range handlerCalls(b) {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got *emit.Method
					err error
				)
				for c.Loop() {
					got, err = spell.Handler(unary, tt.policy, spelled)
				}
				assert.NoError(b, err, "Handler serves "+tt.name)
				assert.Equal(b, got.Params.Items()[0].Name, tt.want, "Handler of "+tt.name+" takes its parameters")
			})
		}
	})

	b.Run("Home", func(b *testing.B) {
		schema := plugin.Placement{Path: schemaFile}
		c := bench.Start(b).MaxAllocs(homeAllocs)
		defer c.End()
		var (
			got symbol.Identity
			err error
		)
		for c.Loop() {
			got, err = spell.Home(getRequestID, schema)
		}
		assert.NoError(b, err, "Home derives the module")
		assert.Equal(b, got, messagesModule, "Home returns the module")
	})
}

// translation returns a translation through the spoke of TypeScript under
// p. It spells a reference to the referent of the reference, and an
// asynchronous stream of such a reference. It refuses the reference
// refused with errUntranslated.
func translation(p plugin.Policy, refused *node.TypeRef) plugin.Translate {
	return func(ref *node.TypeRef) (*emit.TypeRef, error) {
		if ref == refused {
			return nil, errUntranslated
		}
		if ref.Form == symbol.FormStream {
			elem := rules.Reference(ref.Elems[0].Spelling, ref.Elems[0].Target)
			return spell.Type(rules.TypeShape{Form: symbol.FormStream, Async: true, Elems: []rules.TypeShape{elem}}, p)
		}
		return spell.Type(rules.Reference(ref.Spelling, ref.Target), p)
	}
}

// spelled is a translation that allocates nothing: it returns the session
// for the session, the context for the context, and the request for every
// other reference.
func spelled(ref *node.TypeRef) (*emit.TypeRef, error) {
	switch ref {
	case sessionRef:
		return sessionType, nil
	case contextRef:
		return handlerContext, nil
	default:
		return getRequestType, nil
	}
}

// unaryCall returns the rpc Get with its context and its request, which
// returns the session.
func unaryCall() plugin.RPC {
	r := getCall
	r.Callable.Params = []rules.ParamView{
		{Name: contextName, Ref: contextRef, Role: rules.ParamContext}, {Name: requestName, Ref: getRequestRef},
	}
	r.Callable.Returns = []rules.ReturnView{{Ref: sessionRef}}
	return r
}

// handlerCalls returns the handlers that the allocation test and the
// benchmark measure: a unary rpc under connect-es and without a binding.
func handlerCalls(tb testing.TB) []handlerCall {
	tb.Helper()

	return []handlerCall{
		{
			name:   "a unary rpc under connect-es",
			policy: policy(tb, map[plugin.PolicyKey]plugin.Choice{typescript.GRPC: typescript.GRPCConnect}),
			allocs: connectAllocs, want: requestName,
		},
		{name: "a unary rpc without a binding", policy: policy(tb, nil), allocs: canonicalAllocs, want: contextName},
	}
}
