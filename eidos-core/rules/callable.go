// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Callable is the normalized view of a function or a method: what a
// shape detector reads, and the only thing it may. [Bound.CallableOf]
// returns it. Params is nil for a signature without parameters, and
// Returns for one without returns.
type Callable struct {
	Receiver *ParamView   // nil for a free function or a type-level member
	Params   []ParamView  // in declaration order, without nil entries
	Returns  []ReturnView // in declaration order, without nil entries
	Errors   ErrorModel   // the failure channel the language names
	Async    bool         // whether the callable is asynchronous
}

// ParamView is one parameter: its name, its reference, the role the
// language gave it, and whether it collects the rest. It contains the
// reference and no shape. A consumer that needs the shape asks
// [Bound.TypeOf], which memoises, so a detector folds only the shapes it
// reads.
type ParamView struct {
	Name     string
	Ref      *node.TypeRef
	Role     ParamRole
	Variadic bool
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
		return b.callable(nil, symbol.LevelInstance, d.Params, d.Returns, d.Async), true
	case *node.Method:
		return b.callable(d.Receiver, d.Level, d.Params, d.Returns, d.Async), true
	default:
		return Callable{}, false
	}
}

// callable assembles the view from the signature's parts. It skips a
// nil parameter and a nil return, and gives a return the language left
// without a role [ReturnValue]. It allocates the receiver's view at
// instance level, and the parameter list and the return list where the
// signature has entries.
func (b Bound) callable(
	receiver *node.Param, level symbol.Level, params []*node.Param, returns []*node.Return, async bool,
) Callable {
	c := Callable{Async: async}
	if receiver != nil && level == symbol.LevelInstance {
		c.Receiver = &ParamView{Name: receiver.Name, Ref: receiver.Type}
	}
	if len(params) > 0 {
		c.Params = make([]ParamView, 0, len(params))
		for _, p := range params {
			if p == nil {
				continue
			}
			c.Params = append(c.Params, ParamView{
				Name:     p.Name,
				Ref:      p.Type,
				Role:     b.source.ParamRole(p, b.view),
				Variadic: p.Variadic != symbol.VariadicNone,
			})
		}
	}
	roles, model := b.source.ReturnRoles(returns, b.view)
	c.Errors = model
	if len(returns) > 0 {
		c.Returns = make([]ReturnView, 0, len(returns))
		for i, r := range returns {
			if r == nil {
				continue
			}
			role := ReturnValue
			if i < len(roles) {
				role = roles[i]
			}
			c.Returns = append(c.Returns, ReturnView{Name: r.Name, Ref: r.Type, Role: role})
		}
	}
	return c
}
