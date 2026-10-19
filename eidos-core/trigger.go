// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// GraphMatch is the OnGraph subject. It reads the whole scope through
// the reader of the match. It has no subject identity, so a fact read
// of the subject returns false. Its reports take a position, through
// [GraphMatch.ErrorfAt], [GraphMatch.WarnfAt] and [GraphMatch.InfofAt],
// because a finding without a position is a defect of its reporter.
type GraphMatch struct{ match }

// unbind zeroes the match for the next phase call of its lane, and
// returns the key under which the lane keeps released graph matches.
func (m *GraphMatch) unbind() any {
	*m = GraphMatch{}
	return (*GraphMatch)(nil)
}

// OnGraph runs the handler once per phase call, for logic that spans
// subjects. It takes the Emitter only. A stamper writes to the bag of
// its subject, and a graph rule has no subject. An annotator that needs
// facts on many declarations subscribes to their kinds. OnGraph
// allocates the leaf of the rule and the closure of its invocation over
// the handler, two allocations.
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

// EmitMatch is the OnEmit subject. It is one emit value that a plugin
// of an earlier turn produced in the same plan. Its subject is the
// origin of the value, so fact reads, predicates and the skip directive
// resolve against the origin. Its reports have the position of the
// origin.
type EmitMatch struct {
	subjectMatch
	// Value is the emit declaration. A handler asserts it to its kind.
	Value symbol.Symbol
}

var _ SubjectMatch = (*EmitMatch)(nil)

// Origin returns the node identity that the emit value derives from. It
// allocates nothing.
func (m *EmitMatch) Origin() symbol.Identity { return m.subject }

// unbind zeroes the match for the next phase call of its lane, and
// returns the key under which the lane keeps released emit matches.
func (m *EmitMatch) unbind() any {
	*m = EmitMatch{}
	return (*EmitMatch)(nil)
}

// OnEmit runs the handler once for each emit value of one kind,
// wherever a slot contains it. It takes the Emitter only. Emit values
// belong to one plan, and plans run in parallel, so a fact stamped from
// the emit side would be visible to one plan alone. An annotator stamps
// a fact about generated output on its origin. OnEmit allocates the leaf
// of the rule and the closure of its invocation over the handler, two
// allocations.
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

// PackageMatch is the OnPackage subject. It is one package of the scope
// with its files and their declarations, so a handler checks the
// declarations of a package as one set. Its reports have the position of
// the package.
type PackageMatch struct {
	subjectMatch
	// Package is the package, with its files and their declarations.
	Package *node.Package
}

var _ SubjectMatch = (*PackageMatch)(nil)

// unbind zeroes the match for the next phase call of its lane, and
// returns the key under which the lane keeps released package matches.
func (m *PackageMatch) unbind() any {
	*m = PackageMatch{}
	return (*PackageMatch)(nil)
}

// OnPackage runs the handler once for each package in scope, or once for
// each gating instance of a directive on a package under a Directive
// wrapper. The effect parameter selects the role, as it does for the
// triggers of the declaration kinds. A handler walks the declarations of
// the package without a tracked read, so the invocation records a package
// edge, and a change of any member of the package runs the handler again.
//
// # Allocation contract
//
// OnPackage allocates the leaf of the rule and the closure of its
// invocation over the handler, two allocations. The first invocation of a
// phase call allocates the tracked reader of the lane, which records the
// package edge.
func OnPackage[E Effect](h func(*PackageMatch, E) error) Rule {
	return Rule{leaf: &leaf{
		kind:  symbol.KindPackage,
		phase: phaseOf[E](),
		invoke: func(inv invocation) error {
			pkg, held := inv.value.(*node.Package)
			if !held {
				return nil
			}
			m := bindMatch[PackageMatch](inv)
			m.Package = pkg
			// A lookup of the identity of a package records its package edge.
			m.Reader().Lookup(pkg.ID)
			return h(m, effectFor[E](inv.rs, &m.match))
		},
	}}
}
