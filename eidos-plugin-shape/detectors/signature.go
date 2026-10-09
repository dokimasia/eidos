// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors

import (
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// signature is the part of a callable that the detectors read. It has the
// number of inputs and values of the callable, the first input and the
// first value, and whether the callable takes a context and has an error
// model. An input is a parameter with the input role, and a value is a
// return with the value or the stream role.
type signature struct {
	// inputs is the number of inputs, and input is the first of them.
	inputs int
	input  rules.ParamView
	// values is the number of values, and value is the first of them. The
	// error and the ok flag are no values.
	values int
	value  rules.ReturnView
	// context is true where a parameter has the context role.
	context bool
	// fails is true where the callable has an error model.
	fails bool
}

// signatureOf reads the signature of a callable. It allocates nothing.
func signatureOf(c rules.Callable) signature {
	s := signature{fails: c.Errors != rules.ErrorsNone}
	for _, p := range c.Params {
		if p.Role == rules.ParamContext {
			s.context = true
			continue
		}
		if s.inputs == 0 {
			s.input = p
		}
		s.inputs++
	}
	for _, r := range c.Returns {
		if r.Role != rules.ReturnValue && r.Role != rules.ReturnStream {
			continue
		}
		if s.values == 0 {
			s.value = r
		}
		s.values++
	}
	return s
}

// isInterface reports whether a reference refers to an interface. An
// inline body with a method is an interface, and so is a declaration of
// the interface kind that the view contains. An inline body without a
// method is a record. A type parameter and a type that the view does not
// contain are no interfaces.
func isInterface(b rules.Bound, ref *node.TypeRef) bool {
	switch {
	case ref == nil:
		return false
	case ref.Form == symbol.FormInline:
		return len(ref.Methods) > 0
	case ref.Form == symbol.FormNamed:
		sym, held := b.View().Lookup(ref.Target)
		return held && sym.Kind() == symbol.KindInterface
	default:
		return false
	}
}

// declared returns the identity of the declaration that a reference
// refers to. An optional reference counts as the reference that it wraps.
// A builtin and a structural form refer to no declaration, so declared
// returns the zero identity for them.
func declared(ref *node.TypeRef) symbol.Identity {
	for ref != nil && ref.Form == symbol.FormOptional && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	if ref == nil || ref.Form != symbol.FormNamed {
		return symbol.Identity{}
	}
	return ref.Target
}
