// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Callable is the normalized view of a function or a method: what a
// shape detector reads, and the only thing it may. [Bound.CallableOf]
// returns it. Params is nil for a signature without parameters and
// without a context, and Returns for one without returns. The view of a
// context that the [ContextRules] of the language give comes first among
// the parameters. Under [ErrorsResultType], a return of a result type of a
// language with [ResultRules] has two views: its value, where it has one,
// and its failure with the role [ReturnError].
type Callable struct {
	Name     string       // the declared name of the function or the method
	Receiver *ParamView   // nil for a free function or a type-level member
	Params   []ParamView  // in declaration order, without nil entries
	Returns  []ReturnView // in declaration order, without nil entries
	Errors   ErrorModel   // the failure channel the language names
	Async    bool         // whether the callable is asynchronous
}

// ParamView is one parameter: its name, its reference, the role the
// language gave it, whether it collects the rest, whether a call can
// leave it out, and how a call passes it. It contains the reference and no
// shape. A consumer that needs the shape asks [Bound.TypeOf], which
// memoises, so a detector folds only the shapes it reads.
type ParamView struct {
	Name     string
	Ref      *node.TypeRef
	Role     ParamRole
	Variadic bool
	// Optional reports whether a call can leave the parameter out: the
	// parameter has a default, the optional mark of its language, or
	// collects the rest of the arguments. A callable whose parameters are
	// all optional takes a call without arguments.
	Optional bool
	// Passing is the way a call passes the argument: by position alone, by
	// keyword alone, or in the way of the language. A generator that
	// writes a delegating call passes a keyword parameter by its name.
	Passing symbol.Passing
}

// ReturnView is one return: its name where the language names
// results, its reference, and the role the language gave it.
type ReturnView struct {
	Name string
	Ref  *node.TypeRef
	Role ReturnRole
}

// callableOf maps a function or a method into the view, and asks the
// language for the roles. It reports false for any other symbol, nil
// included.
func (b Bound) callableOf(sym symbol.Symbol) (Callable, bool) {
	switch d := sym.(type) {
	case *node.Function:
		return b.callable(sym, d.Name, nil, symbol.LevelInstance, d.Params, d.Returns, d.Async), true
	case *node.Method:
		return b.callable(sym, d.Name, d.Receiver, d.Level, d.Params, d.Returns, d.Async), true
	default:
		return Callable{}, false
	}
}

// callable assembles the view of the callable sym from the name and the
// signature's parts. It puts the context that the [ContextRules] of the
// language give sym first among the parameters, skips a nil parameter and a
// nil return, and gives a return the language left without a role
// [ReturnValue]. Under [ErrorsResultType] it splits a return of a result
// type through the [ResultRules] of the language: the value with the role
// of the return, where the result has a value, and the failure with the
// role [ReturnError], both with the name of the return. It allocates the
// receiver's view at instance level, the parameter list where the signature
// has parameters or a context, and the return list where it has returns.
func (b Bound) callable(
	sym symbol.Symbol, name string, receiver *node.Param, level symbol.Level, params []*node.Param,
	returns []*node.Return, async bool,
) Callable {
	c := Callable{Name: name, Async: async}
	if receiver != nil && level == symbol.LevelInstance {
		c.Receiver = &ParamView{Name: receiver.Name, Ref: receiver.Type}
	}
	var ctx ParamView
	hasContext := false
	if contexts, implemented := b.source.(ContextRules); implemented {
		ctx, hasContext = contexts.Context(sym, b.view)
	}
	if hasContext {
		c.Params = append(make([]ParamView, 0, len(params)+1), ctx)
	}
	if len(params) > 0 {
		if !hasContext {
			c.Params = make([]ParamView, 0, len(params))
		}
		for _, p := range params {
			if p == nil {
				continue
			}
			variadic := p.Variadic != symbol.VariadicNone
			c.Params = append(c.Params, ParamView{
				Name:     p.Name,
				Ref:      p.Type,
				Role:     b.source.ParamRole(p, b.view),
				Variadic: variadic,
				Optional: variadic || p.Optional || p.Default != "",
				Passing:  p.Passing,
			})
		}
	}
	roles, model := b.source.ReturnRoles(returns, b.view)
	c.Errors = model
	if len(returns) == 0 {
		return c
	}
	results, implemented := b.source.(ResultRules)
	splits := implemented && model == ErrorsResultType
	size := len(returns)
	if splits {
		size *= 2
	}
	c.Returns = make([]ReturnView, 0, size)
	for i, r := range returns {
		if r == nil {
			continue
		}
		role := ReturnValue
		if i < len(roles) {
			role = roles[i]
		}
		if splits {
			if value, failure, ok := results.Result(r.Type, b.view); ok {
				if value != nil {
					c.Returns = append(c.Returns, ReturnView{Name: r.Name, Ref: value, Role: role})
				}
				c.Returns = append(c.Returns, ReturnView{Name: r.Name, Ref: failure, Role: ReturnError})
				continue
			}
		}
		c.Returns = append(c.Returns, ReturnView{Name: r.Name, Ref: r.Type, Role: role})
	}
	return c
}

// IsImplementation reports whether the method m is the implementation of
// an overloaded name among methods: m has no mark Overload, and another
// method of its name and its level has the mark. A reader of the
// signatures that callers see, such as the constructors of a class, leaves
// the implementation out, because TypeScript and Python show callers the
// overload signatures alone. It reports false for a nil method, and it
// allocates nothing.
func IsImplementation(m *node.Method, methods []*node.Method) bool {
	if m == nil || m.Overload {
		return false
	}
	return slices.ContainsFunc(methods, func(other *node.Method) bool {
		return other != nil && other.Overload && other.Name == m.Name && other.Level == m.Level
	})
}
