// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// resultType is the wrapper type of a folded result, and unitType is the
// value type of a folded callable without a result.
const (
	resultType = "Result"
	unitType   = "()"
)

// Lower rewrites the constructs that Rust expresses through other
// declarations. It wraps the result of a callable with a declared
// failure type in Result, in place, so the declared failure becomes the
// error that the caller matches on. The rewrite removes the failure
// types, so a second settle changes nothing. Every other declaration
// passes through unchanged.
//
// Lower returns an error for these callables:
//
//   - a callable with two or more failure types, because a Result has
//     one error type, and a union of the types is a decision of the
//     generator;
//   - a callable with two or more results and a failure type, because a
//     Result wraps one value, and a tuple would hide references inside a
//     composite spelling;
//   - a callable whose result has no type, because the unit type in its
//     place changes what the callable returns;
//   - a callable with a failure type whose body has verbatim text,
//     statements or a return statement in a slot, because the fold
//     changes the result and does not rewrite the body;
//   - a file-level method without a Receives type, because Rust renders
//     a method inside the impl block of its type.
//
// # Allocation contract
//
// A declaration that passes through allocates nothing. A fold allocates
// the folded return, its Result reference and the list of its two
// arguments. A fold of a callable without a result also allocates the
// unit type and the storage of one return. A refusal allocates its
// error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if m, is := s.(*emit.Method); is && (m.Receives == nil || m.Receives.Spelling == "") {
		return nil, refuse("a method renders inside the impl block of its type, and %s attaches to none", m.Name)
	}
	return nil, lowering.Callables(rust.Lang, s, fold)
}

// fold folds the results and the failure type of the callable name into
// one Result return, in place. [lowering.Callables] calls it for each
// callable with failure types and clears the failure types. The first
// argument of the Result is the value type, or the unit type for a
// callable without a result. The second argument is the failure type.
// fold moves each reference whole, so the settle keeps following it. The
// name and the trailing comment of the result move to the folded return.
// fold returns an error for a callable that [Lower] refuses.
func fold(name string, returns *emit.Slot[*emit.Return], ts []*emit.TypeRef) error {
	rs := returns.Items()
	switch {
	case len(ts) > 1:
		return refuse("a result has one failure type, and %s announces %d", name, len(ts))
	case len(rs) > 1:
		return refuse("a result wraps one value, and %s returns %d beside a throw", name, len(rs))
	}
	out := &emit.Return{}
	var value *emit.TypeRef
	switch {
	case len(rs) == 0:
		value = &emit.TypeRef{Spelling: unitType}
	case rs[0].Type == nil:
		return refuse("a result wraps a stated type, and %s returns one that states none", name)
	default:
		value = rs[0].Type
		out.Name, out.Comment = rs[0].Name, rs[0].Comment
	}
	out.Type = &emit.TypeRef{
		Spelling: resultType,
		Args:     []*emit.TypeRef{value, ts[0]},
	}
	returns.Replace(out)
	return nil
}
