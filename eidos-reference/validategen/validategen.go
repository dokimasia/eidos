// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package validategen

import (
	"fmt"

	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "validategen"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that validategen provides.
const Capability plugin.Capability = "http.validate"

// Word is the word of the family of the validators.
const Word = "validate"

// Prefix opens the name of each validator, before the name of the struct.
const Prefix = "Validate"

// The parameter and the result of a validator, the variable that the
// prologue of ServeHTTP binds the result of the validator to, the import
// path and the type of the request, and the format of the documentation of
// a validator. The format takes the names of the validator and of the
// struct.
const (
	requestParam = "r"
	requestType  = "http.Request"
	pointerMark  = "*"
	httpPackage  = "net/http"
	errorType    = "error"
	errVar       = "err"
	validatorDoc = "%s reports whether the request for %s is well-formed."
)

// New returns validategen. For each struct that handlergen serves, it
// writes the function Validate<T>(r *http.Request) error into the family
// [Word], beside the source of the struct. The function returns nil. On the
// method ServeHTTP of the handler of the struct, it appends the call of the
// validator and a guard into the prologue, so the handler returns before
// it calls Serve where the validator reports an error. Each call returns a
// new plugin, because a plugin instance belongs to one workspace.
func New() plugin.Generator {
	handled := sdk.HasKey(meta.Named[bool](handlergen.KeyHandler))
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Requires(handlergen.Capability).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word}).
		Handle(
			sdk.Where(handled, sdk.OnStruct(validator)),
			sdk.Where(handled, sdk.OnEmit(symbol.KindMethod, guard)),
		).
		Build().(plugin.Generator)
	return g
}

// validator writes the validator of one struct. The validator returns nil.
func validator(m *sdk.StructMatch, e *sdk.Emitter) error {
	s := m.Struct
	name := Prefix + s.Name
	fn := &emit.Function{
		Origin:     s.ID,
		Doc:        []string{fmt.Sprintf(validatorDoc, name, s.Name)},
		Name:       name,
		Visibility: symbol.VisibilityPublic,
		Params: []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{
			Spelling: pointerMark + requestType, Form: symbol.FormOptional,
			Elems: []*emit.TypeRef{{Spelling: requestType, Package: httpPackage}},
		}}},
		Returns: []*emit.Return{{Type: &emit.TypeRef{Spelling: errorType}}},
	}
	fn.Body.Stmts = []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Literal(emit.LiteralNil, ""))}}
	e.File().Append(fn)
	return nil
}

// guard appends into the prologue of the method ServeHTTP of a handler the
// call of the validator of the struct with the request, and a guard that
// returns where the validator reports an error. Every other method is left
// as it is. The call refers to the validator by its name in the package of
// the handler, and the settle follows the name.
func guard(m *sdk.EmitMatch, e *sdk.Emitter) error {
	method, _ := m.Value.(*emit.Method)
	if method.Name != handlergen.ServeHTTP {
		return nil
	}
	validate := emit.Expr{Kind: emit.ExprName, Name: Prefix + m.Origin().Name}
	request := emit.Expr{Kind: emit.ExprName, Name: method.Params[len(method.Params)-1].Name}
	e.Slot(&method.Body.Prologue).Append(
		emit.Stmt{
			Kind: emit.StmtAssign, Declare: true, Names: []string{errVar},
			Value: emit.Expr{Kind: emit.ExprCall, Fn: &validate, Args: []emit.Expr{request}},
		},
		emit.Stmt{Kind: emit.StmtGuard, Name: errVar, Then: []emit.Stmt{{Kind: emit.StmtReturn}}},
	)
	return nil
}
