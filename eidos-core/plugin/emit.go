// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// Cardinality says how many outputs a family produces. The zero
// value addresses nothing: every declared family states its
// cardinality, and [Emit.Add] refuses a unit that does not.
type Cardinality uint8

const (
	// PerSource is one output per source file.
	PerSource Cardinality = iota + 1
	// PerPackage is one output per package.
	PerPackage
	// PerPlan is one output per plan.
	PerPlan
)

// Output declares one file family a generator emits.
//
// The target spells the join and extension of Word at render, which
// is why neither appears here: one declaration serves store_stub.go
// and store.stub.ts alike.
type Output struct {
	// Tag selects the family; "" is the primary.
	Tag string
	Per Cardinality
	// Word is the family's word, e.g. "stub".
	Word string
}

// Unit is one accumulated output entity: everything one plugin
// contributed to one (family, cardinality key) in one phase call.
// It carries the full routing key, so no consumer re-derives any
// part of it from the declarations.
type Unit struct {
	// Plugin is the contributor: the attribution a manifest names,
	// and part of what makes one accumulator's flush unique.
	Plugin ID
	// Tag selects the declared family; "" is the primary.
	Tag string
	Per Cardinality
	// Word is the family's word; the target spells its join and
	// extension at render.
	Word string
	// Key addresses the accumulator: the subject's source file path
	// for PerSource, the package path for PerPackage, and "" for
	// PerPlan. The per-source key is the same field a rendered
	// filename's stem derives from, so the key and the filename
	// cannot disagree.
	Key string
	// Pkg is the owning package's identity for per-source and
	// per-package units, zero for a plan unit: the namespace half of
	// the routing key.
	Pkg symbol.Identity
	// Decls holds the emit declarations, ordered by origin identity,
	// then instance order under a repeatable directive, then
	// insertion, so contributions arrive in canonical subject order
	// and never in dispatch order.
	Decls []symbol.Symbol
	// Origins holds the node identities whose matches contributed,
	// sorted and deduplicated: the provenance a manifest carries.
	Origins []symbol.Identity
}

// unitKey addresses one accumulator: what one phase call may flush
// exactly once. The key includes the package, because two
// languages may spell one package path as two namespaces.
type unitKey struct {
	plugin ID
	tag    string
	pkg    symbol.Identity
	key    string
}

// Emit holds one plan's accumulated units, plus a per-kind index
// over their declarations that is maintained at [Emit.Add]: each
// unit's tree is walked once when it arrives, so an emit-triggered
// rule enumerates its matches rather than the emit graph.
//
// An Emit is not safe for concurrent use: annotators and generators
// run sequentially, and a unit arriving mid-enumeration would race
// the index it is being read from.
type Emit struct {
	units []Unit
	held  map[unitKey]struct{}
	// byKind maps a kind to the origin-carrying declarations each
	// unit's tree holds, in walk order, keyed by the unit's index
	// into units.
	byKind map[symbol.Kind]map[int][]symbol.Symbol
	// order caches the unit indexes in Units order; nil after a
	// unit arrived since it was built.
	order []int
	// settled says the store passed through [Settle]: the backend's
	// lowering seams ran, and the declarations the readers see are
	// the ones that render.
	settled bool
}

// NewEmit returns an emit store holding nothing.
func NewEmit() *Emit {
	return &Emit{
		held:   map[unitKey]struct{}{},
		byKind: map[symbol.Kind]map[int][]symbol.Symbol{},
	}
}

// Settled reports whether the store passed through [Settle]. A
// renderer whose backend declares a lowering seam refuses an
// unsettled store, so a composition that skips the settle fails at
// the first render instead of writing the wrong bytes.
func (e *Emit) Settled() bool { return e.settled }

// Add records one unit and indexes its tree.
//
// A second unit under the same (plugin, tag, package, key) is
// refused as the defect it is: one phase call flushes each
// accumulator once. A
// zero cardinality and an empty word are refused the same way,
// because a unit missing either cannot be routed, and so is a plan
// unit naming a key, because a plan has one output and its key is
// empty.
func (e *Emit) Add(u Unit) error {
	if u.Per == 0 {
		return fmt.Errorf("plugin: %s flushes a unit with no cardinality", u.Plugin)
	}
	if u.Word == "" {
		return fmt.Errorf("plugin: %s flushes a unit with no word", u.Plugin)
	}
	if u.Per == PerPlan && u.Key != "" {
		return fmt.Errorf(
			"plugin: %s flushes a plan unit keyed %q: a plan has one output and its key is empty",
			u.Plugin, u.Key,
		)
	}
	for _, d := range u.Decls {
		if d == nil {
			return fmt.Errorf(
				"plugin: %s flushes a nil declaration, which no renderer could spell",
				u.Plugin,
			)
		}
	}
	k := unitKey{plugin: u.Plugin, tag: u.Tag, pkg: u.Pkg, key: u.Key}
	if _, taken := e.held[k]; taken {
		return fmt.Errorf(
			"plugin: %s flushes (%q, %q) twice: one phase call flushes each accumulator once",
			u.Plugin, u.Tag, u.Key,
		)
	}
	e.held[k] = struct{}{}

	at := len(e.units)
	e.units = append(e.units, u)
	e.index(at, u.Decls)
	e.order = nil
	return nil
}

// Units enumerates every unit: by plugin, then cardinality, then
// key, then package, then tag. The order is total, so two runs
// agree whatever order the units arrived in.
func (e *Emit) Units() iter.Seq[Unit] {
	return func(yield func(Unit) bool) {
		for _, at := range e.sorted() {
			if !yield(e.units[at]) {
				return
			}
		}
	}
}

// ByKind enumerates the emit declarations of one kind across every
// unit, in [Emit.Units] order, each unit's tree walked depth first.
//
// Only values carrying a nonzero origin are returned, because every
// emit-trigger mechanism resolves through the origin: predicates,
// skip and reporting alike. A value whose producer left the origin
// zero still arrives in its unit; it is just not a subject.
func (e *Emit) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		per := e.byKind[k]
		if len(per) == 0 {
			return
		}
		for _, at := range e.sorted() {
			for _, s := range per[at] {
				if !yield(s) {
					return
				}
			}
		}
	}
}

// index walks one unit's declarations into the per-kind index.
func (e *Emit) index(at int, decls []symbol.Symbol) {
	for _, d := range decls {
		for s := range emit.All(d) {
			origin, carries := emit.OriginOf(s)
			if !carries || origin.IsZero() {
				continue
			}
			per := e.byKind[s.Kind()]
			if per == nil {
				per = map[int][]symbol.Symbol{}
				e.byKind[s.Kind()] = per
			}
			per[at] = append(per[at], s)
		}
	}
}

// reindex rebuilds the per-kind index over the settled units, so a
// reader after the settle never meets a kind a lowering replaced.
func (e *Emit) reindex() {
	e.byKind = map[symbol.Kind]map[int][]symbol.Symbol{}
	for i := range e.units {
		e.index(i, e.units[i].Decls)
	}
	e.order = nil
}

// sorted returns the unit indexes in Units order, rebuilding the
// cache after a unit arrived.
func (e *Emit) sorted() []int {
	if e.order != nil {
		return e.order
	}
	order := make([]int, len(e.units))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		ua, ub := &e.units[a], &e.units[b]
		if c := strings.Compare(string(ua.Plugin), string(ub.Plugin)); c != 0 {
			return c
		}
		if c := cmp.Compare(ua.Per, ub.Per); c != 0 {
			return c
		}
		if c := strings.Compare(ua.Key, ub.Key); c != 0 {
			return c
		}
		if c := ua.Pkg.Compare(ub.Pkg); c != 0 {
			return c
		}
		return strings.Compare(ua.Tag, ub.Tag)
	})
	e.order = order
	return order
}
