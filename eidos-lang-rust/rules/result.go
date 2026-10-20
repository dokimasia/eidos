// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// resultType is the name of the result type of Rust, and of the alias of the
// result type of a module, as io::Result.
const resultType = "Result"

// Result returns the reference of the value and the reference of the failure
// of a reference ref to a result type, and reports false for a reference of
// another type:
//
//   - Result<T, E> has the value T and the failure E, through the prelude
//     and through a path of any module, as std::result::Result<T, E>.
//   - A Result of a module with one type argument, as io::Result<T> and
//     anyhow::Result<T>, has the value T and the failure Error of the same
//     module, as io::Error.
//   - A Result of a module without a type argument, as fmt::Result, has no
//     value and the failure Error of the module.
//   - An alias of the crate has the value and the failure of its target,
//     with the type arguments of ref substituted, and the defaults of its
//     type parameters for the arguments that ref does not state.
//
// The value is nil where it is the unit type (). Result reads past the
// lifetimes among the type arguments, and follows at most [deriveDepth]
// aliases.
//
// # Allocation contract
//
// Result allocates the reference of the failure of a Result without a second
// type argument, and its spelling where ref is a path. An alias allocates
// what [rules.Substitute] allocates, and the list of its arguments where ref
// leaves out a default or states a lifetime. Each lookup allocates what the
// view allocates to record it.
func (Rules) Result(ref *node.TypeRef, v rules.View) (value, failure *node.TypeRef, ok bool) {
	for range deriveDepth {
		if ref == nil || ref.Form != symbol.FormNamed {
			return nil, nil, false
		}
		if !ref.Target.IsZero() {
			sym, _ := v.Lookup(ref.Target)
			alias, isAlias := sym.(*node.Alias)
			if !isAlias || alias.Target == nil {
				return nil, nil, false
			}
			ref = rules.Substitute(alias.Target, alias.TypeParams, typeArgsFor(alias.TypeParams, ref))
			continue
		}
		module, name, qualified := strings.CutLast(ref.Spelling, pathSep)
		if !qualified {
			module, name = "", ref.Spelling
		}
		if name != resultType {
			return nil, nil, false
		}
		value, failure = typeArg(ref, 0), typeArg(ref, 1)
		if failure == nil {
			failure = &node.TypeRef{Spelling: errorTrait, Pos: ref.Pos, Package: ref.Package}
			if qualified {
				failure.Spelling = module + pathSep + errorTrait
			}
		}
		if value != nil && value.Form == symbol.FormNamed && value.Spelling == unitType {
			value = nil
		}
		return value, failure, true
	}
	return nil, nil, false
}

// typeArgsFor returns the type arguments of a reference ref to a declaration
// with the type parameters params, for [rules.Substitute]: the arguments of
// ref without its lifetimes, and the default of each parameter after the
// arguments that ref states. It returns the arguments of ref where they
// match the parameters one for one, and allocates a list otherwise.
func typeArgsFor(params []*node.TypeParam, ref *node.TypeRef) []*node.TypeRef {
	lifetimes := 0
	for _, a := range ref.Args {
		if a != nil && strings.HasPrefix(a.Spelling, lifetimeMark) {
			lifetimes++
		}
	}
	if lifetimes == 0 && len(ref.Args) == len(params) {
		return ref.Args
	}
	out := make([]*node.TypeRef, 0, max(len(params), len(ref.Args)-lifetimes))
	for _, a := range ref.Args {
		if a == nil || !strings.HasPrefix(a.Spelling, lifetimeMark) {
			out = append(out, a)
		}
	}
	for _, p := range params[min(len(out), len(params)):] {
		var def *node.TypeRef
		if p != nil {
			def = p.Default
		}
		out = append(out, def)
	}
	return out
}
