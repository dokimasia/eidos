// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// GraphMatch is the OnGraph subject: the whole scope, through the
// match's reader. It has no subject identity, so subject-bound fact
// reads return false and its reports demand the position they arrive
// at.
type GraphMatch struct{ match }

// Errorf reports at Error severity at the position the handler
// names: a graph match has no subject, and a positionless finding
// is a defect in whatever reported it. It allocates the finding's
// message, which the sink keeps, and what formatting its arguments
// allocates.
func (m *GraphMatch) Errorf(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityError, at, format, a...)
}

// Warnf reports at Warning severity at the position the handler
// names. It allocates as [GraphMatch.Errorf] states.
func (m *GraphMatch) Warnf(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityWarning, at, format, a...)
}

// Infof reports at Info severity at the position the handler names. It
// allocates as [GraphMatch.Errorf] states.
func (m *GraphMatch) Infof(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityInfo, at, format, a...)
}

// unbind zeroes the match for its lane's next phase call, and returns
// the key the lane keeps released graph matches under.
func (m *GraphMatch) unbind() any {
	*m = GraphMatch{}
	return (*GraphMatch)(nil)
}

// OnGraph runs the handler once per phase call: the pressure valve
// for logic that genuinely spans subjects. It takes the Emitter
// only. A stamper writes to its subject's bag and a graph rule
// names no subject; an annotator that wants facts on many
// declarations subscribes to their kinds. OnGraph allocates the rule's
// leaf and its invocation's closure over the handler, two allocations.
func OnGraph(h func(*GraphMatch, *Emitter) error) Rule {
	return Rule{leaf: &leaf{
		phase: plugin.PhaseGenerate,
		graph: true,
		invoke: func(inv invocation) error {
			m := bindMatch[GraphMatch](inv)
			return h(m, inv.rs.emitterFor(&m.match))
		},
	}}
}

// EmitMatch is the OnEmit subject: one emit value an earlier-bucket
// plugin produced in the same plan. Its subject is the value's
// origin, so fact reads, predicates and skip all resolve against
// the origin, and its reports arrive at the origin's position.
type EmitMatch struct {
	match
	// Value is the emit declaration; assert it to its kind.
	Value symbol.Symbol
}

// Origin returns the node identity the emit value derives from. It
// allocates nothing.
func (m *EmitMatch) Origin() symbol.Identity { return m.subject }

// unbind zeroes the match for its lane's next phase call, and returns
// the key the lane keeps released emit matches under.
func (m *EmitMatch) unbind() any {
	*m = EmitMatch{}
	return (*EmitMatch)(nil)
}

// OnEmit runs the handler once per emit value of one kind,
// wherever a slot contains it. It takes the Emitter only: emit is
// per plan and plans run in parallel, so a fact stamped from the emit
// side would be visible to one plan, and sibling plans would never
// read it. A fact about generated output is a fact on its origin,
// stamped during Annotate. OnEmit allocates the rule's leaf and its
// invocation's closure over the handler, two allocations.
func OnEmit(k symbol.Kind, h func(*EmitMatch, *Emitter) error) Rule {
	return Rule{leaf: &leaf{
		kind:  k,
		phase: plugin.PhaseEmit,
		invoke: func(inv invocation) error {
			m := bindMatch[EmitMatch](inv)
			m.Value = inv.value
			return h(m, inv.rs.emitterFor(&m.match))
		},
	}}
}
