// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"strconv"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// RuleID is a rule's ordinal in its plugin's declaration order.
//
// It is stable exactly as long as the declaration is: reordering
// rules renumbers them. Anything durable keyed on one is
// invalidated by a reorder, which the conformance suite's
// declaration check makes visible.
type RuleID int

// Phase says when a subscription's rule runs. The zero Phase names
// no phase.
type Phase uint8

const (
	// PhaseAnnotate runs over the frozen graph, stamping facts.
	PhaseAnnotate Phase = iota + 1
	// PhaseGenerate runs over node subjects, within a plan.
	PhaseGenerate
	// PhaseEmit runs over emit values earlier buckets produced in
	// the same plan.
	PhaseEmit
)

// String answers the phase's spelling. Subscription records reach
// stats and faults, so the spelling is API. A phase nothing
// declares answers its number rather than a name.
func (p Phase) String() string {
	switch p {
	case PhaseAnnotate:
		return "annotate"
	case PhaseGenerate:
		return "generate"
	case PhaseEmit:
		return "emit"
	default:
		return "Phase(" + strconv.Itoa(int(p)) + ")"
	}
}

// Subscription is one gate tuple as data: what a rule watches,
// which is also what invalidation routes through. Ungated is the
// zero value throughout, so a bare rule's record carries only its
// rule, kind and phase.
type Subscription struct {
	// Rule names the rule the record belongs to. Several records
	// may share one, because a rule gated on two fact keys answers
	// two.
	Rule RuleID
	// Kind is the trigger's subject kind, zero for a graph-wide
	// rule.
	Kind symbol.Kind
	// Directive is the gating schema's name, "" when the rule has
	// no directive gate.
	Directive directive.Name
	// FactKey is the gating fact key, zero when the rule has no
	// fact gate.
	FactKey meta.KeyID
	// Phase says when the rule runs.
	Phase Phase
}

// Subscribed is a plugin that declares its gates, so the engine
// reads them as data without executing a handler.
//
// A hand-rolled plugin may skip it entirely, which reads as one
// implicit subscription to everything in scope: honest, and priced
// with full-graph dispatch.
type Subscribed interface {
	Plugin
	Subscriptions() []Subscription
}
