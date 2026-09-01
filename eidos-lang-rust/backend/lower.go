// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings a lowered failure composes: the wrapper the value
// arrives in, and the unit type a callable without a result wraps.
const (
	resultType = "Result"
	unitType   = "()"
)

// Lower reshapes the constructs Rust states in other declarations:
// a callable announcing a failure type wraps its result in Result,
// in place, so a stated throw becomes the error the caller matches
// on. The lowering consumes the fact, so a second settle changes
// nothing, and everything else passes through as it stands.
//
// A callable announcing several failure types refuses, because a
// result carries one error and folding them into a union is a
// design the generator owns; one returning several values beside a
// throw refuses too, because a result wraps one value and the
// tuple fold would hide references inside a composite spelling.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	switch d := s.(type) {
	case *emit.Function:
		returns, err := wrapped(d.Name, d.Returns, d.Throws)
		if err != nil {
			return nil, err
		}
		d.Returns, d.Throws = returns, nil
	case *emit.Method:
		returns, err := wrapped(d.Name, d.Returns, d.Throws)
		if err != nil {
			return nil, err
		}
		d.Returns, d.Throws = returns, nil
	case *emit.Struct:
		if err := lowerMembers(d.Methods.Items()); err != nil {
			return nil, err
		}
	case *emit.Interface:
		if err := lowerMembers(d.Methods.Items()); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// lowerMembers rewrites a host's member methods the way the
// file-level callables rewrite, because the lowering receives the
// host whole.
func lowerMembers(methods []*emit.Method) error {
	for _, m := range methods {
		returns, err := wrapped(m.Name, m.Returns, m.Throws)
		if err != nil {
			return err
		}
		m.Returns, m.Throws = returns, nil
	}
	return nil
}

// wrapped folds a callable's result and its announced failure into
// the one Result return: the stated value type as the first
// argument, the unit type where none is stated, and the failure
// type as the second, each reference moved whole so the settle
// keeps following it.
func wrapped(
	name string, returns []*emit.Return, throws []*emit.TypeRef,
) ([]*emit.Return, error) {
	switch {
	case len(throws) == 0:
		return returns, nil
	case len(throws) > 1:
		return nil, fmt.Errorf(
			"rust: a result carries one failure type, and %s announces %d",
			name, len(throws),
		)
	case len(returns) > 1:
		return nil, fmt.Errorf(
			"rust: a result wraps one value, and %s returns %d beside a "+
				"throw", name, len(returns),
		)
	}
	value := &emit.TypeRef{Spelling: unitType}
	out := &emit.Return{}
	if len(returns) == 1 {
		out.Name = returns[0].Name
		if returns[0].Type != nil {
			value = returns[0].Type
		}
	}
	out.Type = &emit.TypeRef{
		Spelling: resultType,
		Args:     []*emit.TypeRef{value, throws[0]},
	}
	return []*emit.Return{out}, nil
}
