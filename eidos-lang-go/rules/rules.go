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

// The spellings the signature rules read.
const (
	contextSpelling = "context.Context"
	errorSpelling   = "error"
	boolSpelling    = "bool"
	iterSeqPrefix   = "iter.Seq"
)

// Rules is the Go language's rules value. It holds no state, so
// one value serves every goroutine.
type Rules struct{}

// New returns the Go rules.
func New() rules.SourceRules { return Rules{} }

// Lang returns the language the rules answer for.
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
// convention; the spelling decides, so a context anywhere in the
// list reads as one.
func (Rules) ParamRole(p *node.Param, _ rules.View) rules.ParamRole {
	if p != nil && p.Type != nil && named(p.Type) == contextSpelling {
		return rules.ParamContext
	}
	return rules.ParamInput
}

// ReturnRoles classifies a callable's returns: a last return of
// type error is the error under the last-return model, the second
// of two returns of type bool is the ok flag, an iter.Seq or
// iter.Seq2 return is a stream, and the rest are values. A
// callable without an error return reports no error model.
func (Rules) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	model := rules.ErrorsNone
	for i, r := range rs {
		switch {
		case r == nil || r.Type == nil:
		case i == len(rs)-1 && named(r.Type) == errorSpelling:
			roles[i] = rules.ReturnError
			model = rules.ErrorsLastReturn
		case len(rs) == 2 && i == 1 && named(r.Type) == boolSpelling:
			roles[i] = rules.ReturnOkBool
		case strings.HasPrefix(named(r.Type), iterSeqPrefix):
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
