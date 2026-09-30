// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The types the signature rules read: the two predeclared spellings,
// and the path and name of context.Context.
const (
	errorSpelling  = "error"
	boolSpelling   = "bool"
	contextPackage = "context"
	contextName    = "Context"
)

// The types a signature's parameters and returns classify by.
var (
	boolType    = typeName{name: boolSpelling}
	contextType = typeName{pkg: contextPackage, name: contextName}
	iterSeq     = typeName{pkg: golang.IterPackage, name: golang.IterSeq}
	iterSeq2    = typeName{pkg: golang.IterPackage, name: golang.IterSeq2}
)

// Rules is the Go language's rules value. It has no state, so one
// value serves every goroutine.
type Rules struct{}

// New returns the Go rules.
func New() rules.SourceRules { return Rules{} }

// Lang returns the language the rules apply to.
func (Rules) Lang() symbol.Lang { return golang.Lang }

// Members walks embedded fields under promotion, to the kernel's
// default depth: Go has no extends and no implements list, and an
// embedded type's members promote unless a shallower one shadows
// them, two at one depth cancelling both. A struct's embedded field
// is itself a member, named by its type's bare name, so it shadows
// a deeper member of that name.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{
		Contributes:     []rules.Contribution{rules.ContributesEmbeds},
		Shadowing:       rules.ShadowPromote,
		EmbedsAreFields: true,
	}
}

// ParamRole classifies a context.Context parameter as the context
// and every other as input. Go states the context first by
// convention, and a context anywhere in the list reads as one. The
// package the reference's import names and the name decide, so an
// aliased import of context classifies too.
func (Rules) ParamRole(p *node.Param, _ rules.View) rules.ParamRole {
	if p != nil && p.Type != nil && nameOf(p.Type) == contextType {
		return rules.ParamContext
	}
	return rules.ParamInput
}

// ReturnRoles classifies a callable's returns: a last return of
// type error is the error under the last-return model, the second
// of two returns of type bool is the ok flag, an iter.Seq or
// iter.Seq2 return is a stream, under any alias of the import, and
// the rest are values. A callable without an error return reports no
// error model.
func (Rules) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	model := rules.ErrorsNone
	for i, r := range rs {
		if r == nil || r.Type == nil {
			continue
		}
		switch t := nameOf(r.Type); {
		case i == len(rs)-1 && t == errorType:
			roles[i] = rules.ReturnError
			model = rules.ErrorsLastReturn
		case len(rs) == 2 && i == 1 && t == boolType:
			roles[i] = rules.ReturnOkBool
		case t == iterSeq || t == iterSeq2:
			roles[i] = rules.ReturnStream
		}
	}
	return roles, model
}

// TypeName joins a generator's word onto an author's name the way
// Go spells a derived type: PascalCase, keeping the base's export
// by its first rune, so "check" on Row is CheckRow and on row is
// checkRow.
func (Rules) TypeName(word, base string) string {
	if exported(base) {
		return naming.Pascal(word) + base
	}
	return naming.Camel(word) + naming.Pascal(base)
}

// named returns the spelling a reference names, its whitespace
// trimmed.
func named(ref *node.TypeRef) string {
	if ref == nil {
		return ""
	}
	return strings.TrimSpace(ref.Spelling)
}

// exported reports Go's visibility rule for a name: an upper-case
// first rune exports.
func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
