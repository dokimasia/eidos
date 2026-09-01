// Package eidos is the mini kernel the facade tests generate
// from: every re-export form in one small surface.
package eidos

import "go.dokimi.dev/eidos/core/symbol"

// Rule is one trigger bound to one handler.
type Rule struct{ kind symbol.Kind }

// Weight orders rules.
const Weight = 3

// Default is the rule every fixture starts from.
var Default = Rule{}

// On builds a rule for one kind; the kind vocabulary is
// [go.dokimi.dev/eidos/core/symbol.Kind].
func On(k symbol.Kind, hs ...func(r *Rule) error) (Rule, error) {
	return Rule{kind: k}, nil
}
