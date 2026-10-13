// Copyright Dokimasia B.V. 2026
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
	trueSpelling    = "true"
	falseSpelling   = "false"
	zeroInt         = "0"
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

// scriptedContributions is the one list the scripted language's members
// arrive through. Every policy shares it, and the walk only reads it, so
// a policy allocates nothing.
var scriptedContributions = []rules.Contribution{rules.ContributesEmbeds}

// Members walks embeds under promotion.
func (scripted) Members() rules.MemberPolicy {
	return rules.MemberPolicy{
		Contributes: scriptedContributions,
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

// DeriveSamples derives a pair for a builtin and refuses the rest: a
// named type the view does not contain as unresolved, and every
// other type with no literal, because the scripted language writes
// no composite. The declaration a named type targets decides the
// refusal, so the lookup records it on the view.
func (scripted) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	if ref == nil {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	switch ref.Spelling {
	case scriptedInt:
		return rules.Pair(emit.LiteralInt, sampleInt, alternateInt)
	case scriptedString:
		return rules.Pair(emit.LiteralString, samplePrefix+hint, alternatePrefix+hint)
	case scriptedBool:
		return rules.Pair(emit.LiteralBool, trueSpelling, falseSpelling)
	}
	if ref.Target.IsZero() {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	if _, held := v.Lookup(ref.Target); !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// ZeroValue spells a builtin's zero and reports false for the rest.
func (scripted) ZeroValue(ref *node.TypeRef, _ rules.View) (emit.Value, bool) {
	if ref == nil {
		return emit.Value{}, false
	}
	switch ref.Spelling {
	case scriptedInt:
		return emit.Literal(emit.LiteralInt, zeroInt), true
	case scriptedString:
		return emit.Literal(emit.LiteralString, ""), true
	case scriptedBool:
		return emit.Literal(emit.LiteralBool, falseSpelling), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor reads text as a string for a string, and as an integer for
// an int where the text parses. It reports false for every other text and
// type. The scope plays no part, because the scripted language has no
// names as values.
func (scripted) LiteralFor(_ rules.Scope, ref *node.TypeRef, text string, _ rules.View) (emit.Value, bool) {
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

// Derive returns an int witness for every parameter: the scripted
// language states no bounds, so every type set is open.
func (scripted) Derive(*node.TypeParam, rules.View) (*node.TypeRef, bool) {
	return &node.TypeRef{Spelling: scriptedInt}, true
}

// Reified reports that the scripted language keeps nothing at runtime,
// because it has no runtime.
func (scripted) Reified() bool { return false }
