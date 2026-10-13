// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package handlergen

import (
	"fmt"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "handlergen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that handlergen provides. A plugin that
// extends the handler or the struct of a handler requires it.
const Capability plugin.Capability = "http.handler"

// HandlerDirective is the directive that puts a struct under handlergen.
const HandlerDirective directive.Name = "handler"

// KeyHandler is the key of the fact that handlergen stamps on each struct
// that it serves over HTTP. A plugin that extends a handler selects its
// subjects through the fact, with a named handle.
const KeyHandler meta.KeyName = "handlergen.handler"

// Word is the word of the family of the handlers.
const Word = "handler"

// The method that handlergen generates on each handler, and the method of
// the struct that the handler calls.
const (
	ServeHTTP = "ServeHTTP"
	Serve     = "Serve"
)

// The parameters of ServeHTTP and of Serve, the import path of package
// net/http with the names and the spellings of its two types, and the
// selector of Go between a value and its member.
const (
	writerParam  = "w"
	requestParam = "r"
	httpPackage  = "net/http"
	writerName   = "ResponseWriter"
	requestName  = "Request"
	writerType   = "http." + writerName
	requestType  = "http." + requestName
	pointerMark  = "*"
	selector     = "."
)

// serveParams is the number of parameters of Serve.
const serveParams = 2

// The documentation of a handler and of its method ServeHTTP. The format of
// the documentation of a handler takes the names of the handler and of the
// struct.
const (
	handlerDoc = "%s serves %s over HTTP."
	serveDoc   = "ServeHTTP calls Serve between the statements that other plugins contribute."
)

// schema is the directive of handlergen.
var schema = directive.Schema{
	Name: HandlerDirective,
	Doc:  "generates a handler that serves the struct over HTTP through its method Serve",
}

// conclusion is the state of one instance of handlergen: the handle of its
// key, which the registration sets and the annotator stamps through.
//
// # Concurrency
//
// The composition registers the keys before the first phase call, and the
// handlers read the handle afterwards, so the workers of a phase call read
// it concurrently.
type conclusion struct {
	handler meta.Key[bool]
}

// New returns handlergen. Its annotator stamps [KeyHandler] on each struct
// under [HandlerDirective] that declares the method Serve(w
// http.ResponseWriter, r *http.Request), and reports [MissingServe] at each
// other struct under the directive. Its generator writes a handler for each
// struct with the fact. The handler is the struct <T>Handler, with the
// field <T> that points at the struct, and the method ServeHTTP, which
// calls Serve of the field. A plugin that extends a handler appends into
// the prologue or the epilogue of ServeHTTP. Each call returns a new
// plugin, because a plugin instance belongs to one workspace.
func New() sdk.Dual {
	c := &conclusion{}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word}).
		Keys(c.register).
		Handle(
			sdk.Directive(schema, sdk.OnStruct(c.conclude)),
			sdk.Where(sdk.HasKey(meta.Named[bool](KeyHandler)), sdk.OnStruct(generate)),
		).
		Dual()
}

// register claims the namespace of handlergen and registers [KeyHandler].
//
// Error modes: the error of the claim, such as a namespace that another
// registrant claimed, and the error of the registration.
func (c *conclusion) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(KeyHandler.Namespace()); err != nil {
		return err
	}
	var err error
	c.handler, err = meta.Register[bool](r, meta.KeySpec{
		Name:  KeyHandler,
		Kinds: []symbol.Kind{symbol.KindStruct},
		Doc:   "marks a struct that handlergen serves over HTTP through its method Serve",
	})
	return err
}

// conclude stamps [KeyHandler] on a struct under the directive that
// declares the method Serve(w http.ResponseWriter, r *http.Request). It
// reports [MissingServe] at a struct without one, and stamps nothing on it.
func (c *conclusion) conclude(m *sdk.StructMatch, st *sdk.Stamper) error {
	if method, declared := node.MemberNamed(m.Struct.Methods, Serve); declared && serves(method.Params) {
		sdk.Stamp(st, c.handler, true)
		return nil
	}
	m.Errorf(
		MissingServe,
		"%s is under the directive handler, and declares no method Serve(w %s, r *%s) for its handler to call",
		m.Struct.Name, writerType, requestType,
	)
	return nil
}

// serves reports whether params are the parameters that ServeHTTP passes
// to Serve: an http.ResponseWriter, then a pointer to an http.Request.
func serves(params []*node.Param) bool {
	if len(params) != serveParams || !httpType(params[0].Type, writerName) {
		return false
	}
	request := params[1].Type
	return request.Form == symbol.FormOptional && httpType(request.Elems[0], requestName)
}

// httpType reports whether ref is the type name of package net/http, as a Go
// source file names it through a qualified identifier. The Go frontend
// records the import path that the qualifier binds, so an import under
// another name does not change the result.
func httpType(ref *node.TypeRef, name string) bool {
	return ref.Form == symbol.FormNamed && ref.Package == httpPackage && strings.HasSuffix(ref.Spelling, selector+name)
}

// generate writes the handler of one struct into the family of the
// handlers, beside the source of the struct.
func generate(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	name := e.JoinName(Word, s.Name)
	handler := &emit.Struct{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(handlerDoc, name, s.Name)},
		Name:       name,
		Visibility: symbol.VisibilityPublic,
	}
	handler.Fields.Append(&emit.Field{
		Origin:     s.ID,
		Name:       s.Name,
		Visibility: symbol.VisibilityPublic,
		Type: &emit.TypeRef{
			Spelling: pointerMark + s.Name, Form: symbol.FormOptional,
			Elems: []*emit.TypeRef{{Spelling: s.Name, Target: s.ID}},
		},
	})
	handler.Methods.Append(serveHTTP(s, name))
	e.File().Append(handler)
	return nil
}

// serveHTTP returns the method ServeHTTP of the handler named name of the
// struct s, on a pointer receiver. Its content calls Serve of the field of
// the handler with the writer and the request.
func serveHTTP(s *node.Struct, name string) *emit.Method {
	method := golang.PointerReceiver(&emit.Method{
		Origin:     s.ID,
		Doc:        []string{serveDoc},
		Name:       ServeHTTP,
		Visibility: symbol.VisibilityPublic,
		Receives:   &emit.TypeRef{Spelling: name},
		Params: emit.SlotOf(
			&emit.Param{Name: writerParam, Type: &emit.TypeRef{Spelling: writerType, Package: httpPackage}},
			&emit.Param{Name: requestParam, Type: &emit.TypeRef{
				Spelling: pointerMark + requestType, Form: symbol.FormOptional,
				Elems: []*emit.TypeRef{{Spelling: requestType, Package: httpPackage}},
			}},
		),
	})
	serve := emit.Expr{Kind: emit.ExprName, Name: method.Receiver.Name + selector + s.Name + selector + Serve}
	method.Body.Stmts = []emit.Stmt{{
		Kind: emit.StmtExpr,
		Value: emit.Expr{Kind: emit.ExprCall, Fn: &serve, Args: []emit.Expr{
			{Kind: emit.ExprName, Name: writerParam},
			{Kind: emit.ExprName, Name: requestParam},
		}},
	}}
	return method
}
