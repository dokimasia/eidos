// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package callee

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/reference/middlewaregen"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The separator of a qualified name in the text that a weaver logs, and
// the separator of the elements of an import path.
const (
	selector = "."
	pathSep  = "/"
)

// Reporter is the part of a match that [Check] reads and reports at: the
// facts and the declarations of the load, and the subject of the match.
// Every match of the SDK satisfies it.
type Reporter interface {
	sdk.Matcher
	Rules() rules.Bound
	Errorf(c diag.Code, format string, a ...any)
}

// Options are the options of a plugin that calls a function of another
// package: the import path of the package and the name of the function. A
// config sets both, because the options contract has no required key.
type Options struct {
	Package string `opt:"package" doc:"the import path of the package that declares the function that the plugin calls"`
	Func    string `opt:"func"    doc:"the name of the function that the plugin calls"`
}

// Log are the options of a weaver: the import path of the package of the
// function that logs, the name of the function, and the format of the
// entry that it logs. The format takes the qualified name of the method.
type Log struct {
	Package string `opt:"package" doc:"the import path of the package that declares the function that logs"`
	Func    string `opt:"func"    doc:"the name of the function that logs a format and its arguments"`
	Format  string `opt:"format"  doc:"the format of the entry with one verb for the name of the method"`
}

// Chain returns the rule of a plugin of the middleware chain. On an emitted
// function with the named slot of the chain, it appends
// h = <package>.<func>(h) into the slot, where h is the first parameter of
// the function. A function without the slot, such as a validator, is left
// as it is.
func Chain(o *Options) func(*sdk.EmitMatch, *sdk.Emitter) error {
	return func(m *sdk.EmitMatch, e *sdk.Emitter) error {
		fn, _ := m.Value.(*emit.Function)
		slot, declared := fn.Body.Slot(middlewaregen.ChainSlot)
		if !declared {
			return nil
		}
		handler := emit.Expr{Kind: emit.ExprName, Name: fn.Params.Items()[0].Name}
		wrapped, set := call(m, o.Package, o.Func, handler)
		if !set {
			return nil
		}
		e.Slot(slot).Append(emit.Stmt{Kind: emit.StmtAssign, Names: []string{handler.Name}, Value: wrapped})
		return nil
	}
}

// Epilogue returns the rule of a plugin that calls a function of the
// request at the end of each request. On the method ServeHTTP of a handler,
// it appends <package>.<func>(w, r) into the epilogue of the method, with
// the parameters of the method. Every other method is left as it is.
func Epilogue(o *Options) func(*sdk.EmitMatch, *sdk.Emitter) error {
	return func(m *sdk.EmitMatch, e *sdk.Emitter) error {
		method, _ := m.Value.(*emit.Method)
		if method.Name != handlergen.ServeHTTP {
			return nil
		}
		args := make([]emit.Expr, 0, method.Params.Len())
		for _, p := range method.Params.Items() {
			args = append(args, emit.Expr{Kind: emit.ExprName, Name: p.Name})
		}
		c, set := call(m, o.Package, o.Func, args...)
		if !set {
			return nil
		}
		e.Slot(&method.Body.Epilogue).Append(emit.Stmt{Kind: emit.StmtExpr, Value: c})
		return nil
	}
}

// Weave returns the rule of a weaver. On an emitted method with a
// receiver, it appends <package>.<func>(<format>, "<T>.<M>") into the
// prologue, where T is the name of the origin of the method and M the name
// of the method. A method without a receiver, such as a method of an
// interface, renders no body, and is left as it is.
func Weave(o *Log) func(*sdk.EmitMatch, *sdk.Emitter) error {
	return func(m *sdk.EmitMatch, e *sdk.Emitter) error {
		method, _ := m.Value.(*emit.Method)
		if method.Receiver == nil {
			return nil
		}
		c, set := call(m, o.Package, o.Func,
			emit.ValueExpr(emit.Literal(emit.LiteralString, o.Format)),
			emit.ValueExpr(emit.Literal(emit.LiteralString, m.Origin().Name+selector+method.Name)),
		)
		if !set {
			return nil
		}
		e.Slot(&method.Body.Prologue).Append(emit.Stmt{Kind: emit.StmtExpr, Value: c})
		return nil
	}
}

// Check reports whether pkg and fn name a function to call from the
// declaration at, which the rule of m generates code for. It reports at the
// subject of m and returns false in three cases:
//
//   - [Missing] where pkg or fn is empty.
//   - [UnknownFunction] where the load contains the Go package pkg, and no
//     file of it declares a function named fn.
//   - [UnknownPackage] where pkg is in the module of the package of at, and
//     the load does not contain it.
//
// The load contains the packages of the workspace alone. So Check accepts
// the function of a package outside the module of at, such as a package of
// the standard library, as the options name it. Each read records on the
// invocation of m. Check allocates what the reads and the report allocate.
func Check(m Reporter, at symbol.Identity, pkg, fn string) bool {
	if pkg == "" || fn == "" {
		m.Errorf(Missing, "the plugin calls no function, because the option package or the option func is empty")
		return false
	}
	p, loaded := m.Rules().View().PackageOf(symbol.Identity{Lang: golang.Lang, Package: pkg, Kind: symbol.KindPackage})
	if loaded {
		if declares(p, fn) {
			return true
		}
		m.Errorf(UnknownFunction, "the package %s declares no function %s, which the options of the plugin name",
			pkg, fn)
		return false
	}
	module, known := sdk.FactOf(m, symbol.Identity{Lang: at.Lang, Package: at.Package, Kind: symbol.KindPackage},
		meta.Named[string](meta.ModuleKey))
	rest, inModule := strings.CutPrefix(pkg, module)
	if known && inModule && (rest == "" || strings.HasPrefix(rest, pathSep)) {
		m.Errorf(UnknownPackage, "the module %s contains no package %s, which the options of the plugin name",
			module, pkg)
		return false
	}
	return true
}

// declares reports whether a file of the package p declares a function
// named fn at its top level.
func declares(p *node.Package, fn string) bool {
	for _, f := range p.Files {
		if f == nil {
			continue
		}
		for _, d := range f.Decls {
			if fun, isFunction := d.(*node.Function); isFunction && fun.Name == fn {
				return true
			}
		}
	}
	return false
}

// call returns the call of the function fn of the Go package pkg with
// args. It returns false where [Check] returns false for the origin of m.
func call(m *sdk.EmitMatch, pkg, fn string, args ...emit.Expr) (emit.Expr, bool) {
	if !Check(m, m.Origin(), pkg, fn) {
		return emit.Expr{}, false
	}
	ref := emit.RefExpr(symbol.Identity{Lang: golang.Lang, Package: pkg, Name: fn, Kind: symbol.KindFunction})
	return emit.Expr{Kind: emit.ExprCall, Fn: &ref, Args: args}, true
}
