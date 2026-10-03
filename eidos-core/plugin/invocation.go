// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// WholeCall is the rule of the one invocation a run records for a phase
// call that journals none of its own: the call of a plugin that
// implements its role directly.
const WholeCall RuleID = -1

// UnitRef names one accumulated unit of a plan by the key a phase call
// flushes it under: its plugin, its family's tag, its package and its
// key. The key includes the package, because two languages may spell
// one package path as two namespaces. [Unit.Ref] returns a unit's
// reference, and [Emit.Add] refuses a second unit under one reference.
//
// # Concurrency
//
// A UnitRef is a value, and any number of goroutines may read one.
//
// # Allocation contract
//
// [UnitRef.Compare] allocates nothing.
type UnitRef struct {
	Plugin ID
	Tag    string
	Pkg    symbol.Identity
	Key    string
}

// Compare orders two unit references by plugin, tag, package and key,
// and returns a negative number, zero or a positive number as r sorts
// before, with or after o. Strings compare bytewise and packages by
// [symbol.Identity.Compare]. It cannot fail.
func (r UnitRef) Compare(o UnitRef) int {
	return cmp.Or(
		strings.Compare(string(r.Plugin), string(o.Plugin)),
		strings.Compare(r.Tag, o.Tag),
		r.Pkg.Compare(o.Pkg),
		strings.Compare(r.Key, o.Key),
	)
}

// EmitRef names one emit value of a plan: the unit that contains it,
// and its place in the depth-first walk of the unit's declarations, each
// declaration's subtree in order, counted from zero. [Emit.Ref] returns a
// value's reference. The reference depends on the unit alone, so it is
// the same in every run that flushes the unit with the same
// declarations.
//
// # Concurrency
//
// An EmitRef is a value, and any number of goroutines may read one.
//
// # Allocation contract
//
// [EmitRef.Compare] allocates nothing.
type EmitRef struct {
	Unit  UnitRef
	Index int
}

// Compare orders two emit references by unit, then by place, and returns
// a negative number, zero or a positive number as r sorts before, with or
// after o. It cannot fail.
func (r EmitRef) Compare(o EmitRef) int {
	return cmp.Or(r.Unit.Compare(o.Unit), cmp.Compare(r.Index, o.Index))
}

// MatchKey identifies one invocation of a phase call across runs. Its
// order is the canonical match order: plugin, rule, subject identity,
// gating instance, then host.
//
// A rule's ordinal is its place in its plugin's declaration order, so a
// key is valid for one declaration order only. The rules are code, and
// a run that records keys runs cold over a record another executable
// wrote.
//
// # Concurrency
//
// A MatchKey is a value, and any number of goroutines may read one.
//
// # Allocation contract
//
// [MatchKey.Compare] allocates nothing.
type MatchKey struct {
	Plugin ID
	Rule   RuleID
	// Subject is the match's subject: an emit-phase match's origin, and
	// zero for a graph-wide rule and for a whole call.
	Subject symbol.Identity
	// Instance is the gating directive instance, and zero for a match no
	// directive gates.
	Instance int
	// Host is the emit value an emit-phase rule matched, and zero for
	// every other rule.
	Host EmitRef
}

// Compare orders two match keys in canonical match order, and returns a
// negative number, zero or a positive number as k sorts before, with or
// after o. It cannot fail.
func (k MatchKey) Compare(o MatchKey) int {
	return cmp.Or(
		strings.Compare(string(k.Plugin), string(o.Plugin)),
		cmp.Compare(k.Rule, o.Rule),
		k.Subject.Compare(o.Subject),
		cmp.Compare(k.Instance, o.Instance),
		k.Host.Compare(o.Host),
	)
}

// Selection restricts a phase call to what a warm run executes again.
//
// The authoring surface honours it. A rule that is not emit-phase runs
// the matches Matches lists that its gates still admit, and every match
// its gates admit on a candidate, and no other match. Emit-phase rules
// ignore the selection and run over every emit value of the plan's
// store. A plugin that implements its role directly may ignore the
// selection, and the run then records the whole call as one invocation
// under [WholeCall].
//
// # Concurrency
//
// A phase call reads the selection and writes nothing to it, so one
// selection may serve concurrent calls.
type Selection struct {
	// Matches are recorded matches to run again where their gates still
	// admit them, in canonical match order. A graph-wide rule runs only
	// where Matches lists it.
	Matches []MatchKey
	// Candidates are subjects whose matches may have changed, in identity
	// order. The call evaluates every rule's gates over each, runs each
	// match it finds, and reports each candidate's matches through
	// [Journal.Evaluated].
	Candidates []symbol.Identity
}

// Invocation is what one handler call read and touched: the record a
// warm run keys the call's re-execution on.
//
// # Concurrency
//
// A journal receives an Invocation on the goroutine that made the phase
// call, and the record's slices and read set are the call's own until
// [Journal.Invoked] returns.
type Invocation struct {
	Match MatchKey
	// Reads are the edges the call read through its match, nil for a call
	// that read nothing. The set lists no edge for the subject itself:
	// the match names the subject, and the call reads it without a
	// tracked read. The set is valid until Invoked returns, so a call may
	// reuse it for a later invocation.
	Reads *store.ReadSet
	// Exports are the plans whose export the call read, sorted.
	Exports []string
	// Units are the units the call placed declarations into or touched,
	// sorted.
	Units []UnitRef
	// Hosts are the emit values whose slots the call appended into,
	// sorted.
	Hosts []EmitRef
	// Claimed are the facts the call stamped, sorted.
	Claimed []meta.FactRef
	// Findings are the findings the call reported, in the order it
	// reported them.
	Findings []diag.Diag
}

// Journal receives a phase call's records after the call's effects
// apply: one [Invocation] for each invocation the call ran, in canonical
// match order, then one Evaluated record for each candidate of the
// call's [Selection], in identity order.
//
// # Concurrency
//
// A phase call makes every Journal call on the goroutine that made the
// phase call, one record at a time, so an implementation that serves one
// call at a time needs no lock.
type Journal interface {
	// Invoked records one invocation.
	Invoked(inv Invocation)
	// Evaluated records the matches a candidate subject has now, in
	// canonical match order, and nil where it has none. The slice is the
	// call's own until Evaluated returns, so a call may reuse it for the
	// next candidate.
	Evaluated(subject symbol.Identity, matches []MatchKey)
}
