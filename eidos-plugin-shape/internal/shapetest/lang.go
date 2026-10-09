// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest

import (
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the language of the declarations of a fixture.
const Lang symbol.Lang = "shapetest"

// The spellings of the types that the rules of [Lang] classify.
const (
	// Context is the spelling of a parameter with the context role.
	Context = "context"
	// Error is the spelling of the last return of a callable with an error
	// model.
	Error = "error"
	// Bool is the spelling of the ok flag where it is the second of two
	// returns, and of a truth value anywhere else.
	Bool = "bool"
	// Int is the spelling of a number.
	Int = "int"
	// Byte is the spelling of an eight-bit unsigned number, so a list of
	// it folds to the bytes form.
	Byte = "byte"
	// String is the spelling of a text.
	String = "string"
)

// byteBits is the width of [Byte].
const byteBits = 8

// Rules returns the rules of [Lang]. A parameter of the spelling [Context]
// has the context role, and every other parameter is an input. A last
// return of the spelling [Error] is the error under the last-return model,
// the second of two returns of the spelling [Bool] is the ok flag, a
// return of the stream form is a stream, and every other return is a
// value. [Byte] folds to an eight-bit unsigned scalar. The other rules are
// the scripted rules of the kernel's test kit, which fold [Bool], [Int]
// and [String] into their leaf shapes.
func Rules() rules.SourceRules { return langRules{SourceRules: rulestest.Scripted()} }

// langRules are the rules of [Lang]. The embedded rules are the scripted
// rules, and the methods of langRules replace the language, the roles and
// the fold of [Byte].
type langRules struct {
	rules.SourceRules
}

// Lang returns [Lang].
func (langRules) Lang() symbol.Lang { return Lang }

// Builtin folds [Byte] into an eight-bit unsigned scalar, and every other
// spelling as the scripted rules fold it.
func (l langRules) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	if ref.Spelling == Byte {
		return rules.Scalar(Byte, rules.ScalarUint, byteBits)
	}
	return l.SourceRules.Builtin(ref, v)
}

// ParamRole returns the context role for a parameter of the spelling
// [Context], and the input role for every other parameter.
func (langRules) ParamRole(p *node.Param, _ rules.View) rules.ParamRole {
	if p.Type.Spelling == Context {
		return rules.ParamContext
	}
	return rules.ParamInput
}

// ReturnRoles returns the role of each return, and the last-return model
// for a callable whose last return has the spelling [Error]. It allocates
// the list of roles.
func (langRules) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	model := rules.ErrorsNone
	for i, r := range rs {
		switch {
		case i == len(rs)-1 && r.Type.Spelling == Error:
			roles[i], model = rules.ReturnError, rules.ErrorsLastReturn
		case len(rs) == 2 && i == 1 && r.Type.Spelling == Bool:
			roles[i] = rules.ReturnOkBool
		case r.Type.Form == symbol.FormStream:
			roles[i] = rules.ReturnStream
		}
	}
	return roles, model
}
