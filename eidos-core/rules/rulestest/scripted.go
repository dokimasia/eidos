// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The scripted language's builtin spellings, and the values its
// samples derive from them.
const (
	scriptedInt    = "int"
	scriptedString = "string"
	scriptedBool   = "bool"

	sampleInt       = "42"
	alternateInt    = "7"
	samplePrefix    = "test-"
	alternatePrefix = "other-"
)

// Scripted returns the rules of the scripted language: a member
// walk over embeds under promotion, every parameter an input, an
// error model of none, three builtins, resolution in the subject's
// package, and samples for the builtins alone. It is the language
// the kernel proves its walks on.
func Scripted() rules.SourceRules { return scripted{} }

// scripted is the implementation.
type scripted struct{}

// Lang returns the scripted language.
func (scripted) Lang() symbol.Lang { return frontendtest.ScriptedLang }

// Members walks embeds under promotion.
func (scripted) Members() rules.MemberPolicy {
	return rules.MemberPolicy{
		Contributes: []rules.Contribution{rules.ContributesEmbeds},
		Shadowing:   rules.ShadowPromote,
	}
}

// ParamRole classifies every parameter as input.
func (scripted) ParamRole(*node.Param, rules.View) rules.ParamRole { return rules.ParamInput }

// ReturnRoles classifies every return as a value under no error
// model.
func (scripted) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return make([]rules.ReturnRole, len(rs)), rules.ErrorsNone
}

// Builtin classifies the three builtins and calls everything else
// opaque.
func (scripted) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	switch ref.Spelling {
	case scriptedInt:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 0)
	case scriptedString:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case scriptedBool:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	default:
		return rules.Opaque(ref)
	}
}

// Resolve looks a bare name up in the subject's package, at every
// resolution kind alike: the scripted language has one namespace.
func (scripted) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	for _, k := range []symbol.Kind{symbol.KindStruct, symbol.KindMethod, symbol.KindConstant} {
		id := symbol.Identity{Lang: frontendtest.ScriptedLang, Package: scope.Subject.Package, Name: name, Kind: k}
		if decl, held := v.Lookup(id); held {
			return decl, nil
		}
	}
	return nil, fmt.Errorf("rulestest: %q names nothing in %s at %v", name, scope.Subject.Package, kind)
}

// SamplesOf derives a pair for a builtin and refuses the rest.
func (scripted) SamplesOf(ref *node.TypeRef, hint string, _ rules.View) (rules.Sample, rules.Sample) {
	if ref == nil {
		return rules.Refused(rules.RefusedNoLiteral), rules.Refused(rules.RefusedNoLiteral)
	}
	switch ref.Spelling {
	case scriptedInt:
		return rules.Of(emit.Literal(emit.LiteralInt, sampleInt)),
			rules.Of(emit.Literal(emit.LiteralInt, alternateInt))
	case scriptedString:
		return rules.Of(emit.Literal(emit.LiteralString, samplePrefix+hint)),
			rules.Of(emit.Literal(emit.LiteralString, alternatePrefix+hint))
	case scriptedBool:
		return rules.Of(emit.Literal(emit.LiteralBool, "true")),
			rules.Of(emit.Literal(emit.LiteralBool, "false"))
	}
	if ref.Target.IsZero() {
		return rules.Refused(rules.RefusedNoLiteral), rules.Refused(rules.RefusedNoLiteral)
	}
	return rules.Refused(rules.RefusedUnresolved), rules.Refused(rules.RefusedUnresolved)
}

// ZeroValue spells a builtin's zero and reports false for the rest.
func (scripted) ZeroValue(ref *node.TypeRef, _ rules.View) (emit.Value, bool) {
	if ref == nil {
		return emit.Value{}, false
	}
	switch ref.Spelling {
	case scriptedInt:
		return emit.Literal(emit.LiteralInt, "0"), true
	case scriptedString:
		return emit.Literal(emit.LiteralString, ""), true
	case scriptedBool:
		return emit.Literal(emit.LiteralBool, "false"), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor admits text as a string, an integer where it parses,
// and refuses the rest.
func (scripted) LiteralFor(_ *node.File, ref *node.TypeRef, text string, _ rules.View) (emit.Value, bool) {
	if ref == nil {
		return emit.Value{}, false
	}
	switch ref.Spelling {
	case scriptedString:
		return emit.Literal(emit.LiteralString, text), true
	case scriptedInt:
		if _, err := strconv.Atoi(text); err != nil {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralInt, text), true
	default:
		return emit.Value{}, false
	}
}

// TypeName joins by concatenation, the base first.
func (scripted) TypeName(word, base string) string { return base + word }

// Derive returns an int witness for every parameter: the scripted
// language states no bounds, so every type set is open.
func (scripted) Derive(*node.TypeParam, rules.View) (*node.TypeRef, bool) {
	return &node.TypeRef{Spelling: scriptedInt}, true
}

// Substitute rewrites a reference naming a parameter into the
// argument at its position, copying what it rewrites.
func (scripted) Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef {
	if ref == nil || len(params) != len(args) {
		return ref
	}
	for i, p := range params {
		if p != nil && ref.Spelling == p.Name && args[i] != nil {
			c := *args[i]
			return &c
		}
	}
	if len(ref.Elems) == 0 && len(ref.Args) == 0 {
		return ref
	}
	c := *ref
	c.Elems = substituteAll(scripted{}, ref.Elems, params, args)
	c.Args = substituteAll(scripted{}, ref.Args, params, args)
	return &c
}

// substituteAll rewrites every reference in a list.
func substituteAll(s scripted, refs []*node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) []*node.TypeRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, s.Substitute(r, params, args))
	}
	return out
}

// Reified reports that the scripted language keeps nothing at
// runtime, there being no runtime.
func (scripted) Reified() bool { return false }
