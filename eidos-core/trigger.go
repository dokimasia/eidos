// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// GraphMatch is the OnGraph subject: the whole scope, through the
// match's reader. It has no subject identity, so subject-bound fact
// reads answer false and its reports demand the position they land
// at.
type GraphMatch struct{ match }

// Errorf reports at Error severity at the position the handler
// names: a graph match has no subject, and a positionless finding
// is a defect in whatever reported it.
func (m *GraphMatch) Errorf(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.sink.Errorf(c, at, m.rs.plugin, format, a...)
}

// Warnf reports at Warning severity at the position the handler
// names.
func (m *GraphMatch) Warnf(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.sink.Warnf(c, at, m.rs.plugin, format, a...)
}

// Infof reports at Info severity at the position the handler names.
func (m *GraphMatch) Infof(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.sink.Infof(c, at, m.rs.plugin, format, a...)
}

// OnGraph fires the handler once per phase call: the pressure valve
// for logic that genuinely spans subjects. It takes the Emitter
// only. A stamper writes to its subject's bag and a graph rule
// names no subject; an annotator that wants facts on many
// declarations subscribes to their kinds.
func OnGraph(h func(*GraphMatch, *Emitter) error) Rule {
	return Rule{leaf: &leaf{
		phase: plugin.PhaseGenerate,
		graph: true,
		invoke: func(inv invocation) error {
			m := &GraphMatch{match: newMatch(inv)}
			return h(m, inv.rs.emitterFor(&m.match))
		},
	}}
}

// EmitMatch is the OnEmit subject: one emit value an earlier-bucket
// plugin produced in the same plan. Its subject is the value's
// origin, so fact reads, predicates and skip all resolve against
// the origin, and its reports land at the origin's position.
type EmitMatch struct {
	match
	// Value is the emit declaration; assert it to its kind.
	Value symbol.Symbol
}

// Origin answers the node identity the emit value derives from.
func (m *EmitMatch) Origin() symbol.Identity { return m.subject }

// OnEmit fires the handler once per emit value of one kind,
// wherever a slot holds it. It takes the Emitter only: emit is per
// plan and plans run in parallel, so a fact stamped from the emit
// side would live in a universe sibling plans never see. A fact
// about generated output is a fact on its origin, stamped during
// Annotate.
func OnEmit(k symbol.Kind, h func(*EmitMatch, *Emitter) error) Rule {
	return Rule{leaf: &leaf{
		kind:  k,
		phase: plugin.PhaseEmit,
		invoke: func(inv invocation) error {
			m := &EmitMatch{match: newMatch(inv), Value: inv.value}
			return h(m, inv.rs.emitterFor(&m.match))
		},
	}}
}
