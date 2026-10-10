// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// kindSlots spans every value of a [symbol.Kind]: the kind is a uint8,
// so an array this long counts by kind without depending on the
// generated kind count.
const kindSlots = math.MaxUint8 + 1

// Cardinality is how many outputs a family produces. The zero
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

// String returns the cardinality's spelling in a diagnostic:
// per-source, per-package or per-plan. A cardinality outside the three
// returns its number. String allocates nothing for a declared
// cardinality.
func (c Cardinality) String() string {
	switch c {
	case PerSource:
		return "per-source"
	case PerPackage:
		return "per-package"
	case PerPlan:
		return "per-plan"
	default:
		return "Cardinality(" + strconv.Itoa(int(c)) + ")"
	}
}

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
// It contains the full routing key, so no consumer re-derives any
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
	// Pkg is the identity of the unit's package for per-source and
	// per-package units, zero for a plan unit: the namespace half of
	// the routing key.
	Pkg symbol.Identity
	// Decls are the emit declarations, ordered by origin identity,
	// then instance order under a repeatable directive, then
	// insertion, so contributions arrive in canonical subject order
	// and never in dispatch order.
	Decls []symbol.Symbol
	// Origins are the node identities whose matches contributed,
	// sorted and deduplicated: the provenance a manifest records.
	Origins []symbol.Identity
	// Contributors are the plugins that appended into slots of the
	// unit's declarations through their Emitter, sorted, without the
	// unit's own plugin: the attribution a weaver gets in the file's
	// frame and its manifest entry beside the unit's plugin.
	Contributors []ID
}

// FileKey returns the routing key a unit's filename takes its stem
// from: the source path of a per-source unit, and the empty string for
// a per-package or per-plan unit, whose filename joins its family word
// and tag alone. A target's filename spelling reads the stem through
// it, so no target spells a package path into a filename. It allocates
// nothing.
func (u Unit) FileKey() string {
	if u.Per != PerSource {
		return ""
	}
	return u.Key
}

// Ref returns the unit's reference: the key one phase call flushes the
// unit under. It allocates nothing.
func (u Unit) Ref() UnitRef {
	return UnitRef{Plugin: u.Plugin, Tag: u.Tag, Pkg: u.Pkg, Key: u.Key}
}

// Emit contains one plan's accumulated units, plus a per-kind index
// over their declarations that is maintained at [Emit.Add]: each
// unit's tree is walked once when it arrives, so an emit-triggered
// rule enumerates its matches and not the whole emit graph.
//
// An Emit is not safe for concurrent use: annotators and generators
// run sequentially, and a unit arriving mid-enumeration would race
// the index it is being read from.
type Emit struct {
	units []Unit
	held  map[UnitRef]struct{}
	// byKind maps a kind to the declarations with an origin that each
	// unit's tree contains, in walk order, keyed by the unit's index
	// into units.
	byKind map[symbol.Kind]map[int][]symbol.Symbol
	// order caches the unit indexes in Units order; nil after a
	// unit arrived since it was built.
	order []int
	// holders maps each declaration the per-kind index lists to its
	// unit's index into units. The first [Emit.Contribute] after a
	// unit arrived builds it, so a plan without contributions never
	// builds it.
	holders map[symbol.Symbol]int
	// refs maps each declaration of the walked units' trees to its
	// reference, and walked counts those units, in arrival order. [Emit.Ref]
	// walks the units that arrived since its last call, so a plan whose
	// phase calls journal nothing never builds the map.
	refs   map[symbol.Symbol]EmitRef
	walked int
	// settled reports that the store passed through [Settle]: the backend's
	// lowering seams ran, and the declarations the readers see are
	// the ones that render.
	settled bool
	// translates reports that the settle found a type reference whose
	// target is a declaration of another language than the backend's
	// target.
	translates bool
	// emitted maps each declaration whose name the settle changed to the
	// name it was emitted under, which [NewExport] keys an export on. The
	// settle allocates it at its first change, so a store whose names all
	// kept their spelling has none.
	emitted map[symbol.Symbol]string
}

// NewEmit returns an empty emit store. It allocates three times: the
// store, its set of accumulator keys and its map of kinds.
func NewEmit() *Emit {
	return &Emit{
		held:   map[UnitRef]struct{}{},
		byKind: map[symbol.Kind]map[int][]symbol.Symbol{},
	}
}

// Settled reports whether the store passed through [Settle]. A
// renderer whose backend declares a lowering seam refuses an
// unsettled store, so a composition that skips the settle fails at
// the first render instead of writing the wrong bytes. It allocates
// nothing.
func (e *Emit) Settled() bool { return e.settled }

// Translates reports whether a declaration of the settled store has a
// translated reference: a type reference whose target is a declaration of
// another language than the target of the backend that settled the store.
// The layout qualifies such a reference by the file that declares its
// referent. An unsettled store, and a store that a settle without a
// backend settled, report false. It allocates nothing.
func (e *Emit) Translates() bool { return e.translates }

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
	k := u.Ref()
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
	e.order, e.holders = nil, nil
	return nil
}

// Contribute records that a plugin appended into a slot of a
// declaration the store contains: the unit whose tree contains host
// names the plugin among its [Unit.Contributors]. It reports false for
// a host the per-kind index does not list, which is a declaration
// without an origin or one no unit contains. The unit's own plugin is
// not recorded, and a plugin is recorded once.
//
// # Allocation contract
//
// The first contribution after a unit arrived allocates the map from
// each indexed declaration to its unit, sized to the declarations, so
// it does not grow while it fills. A contribution that records a plugin
// new to its unit grows the unit's list of contributors, and any other
// contribution allocates nothing.
func (e *Emit) Contribute(host symbol.Symbol, p ID) bool {
	if e.holders == nil {
		indexed := 0
		for _, per := range e.byKind {
			for _, decls := range per {
				indexed += len(decls)
			}
		}
		e.holders = make(map[symbol.Symbol]int, indexed)
		for _, per := range e.byKind {
			for at, decls := range per {
				for _, d := range decls {
					e.holders[d] = at
				}
			}
		}
	}
	at, held := e.holders[host]
	if !held {
		return false
	}
	u := &e.units[at]
	if p == u.Plugin {
		return true
	}
	if i, recorded := slices.BinarySearch(u.Contributors, p); !recorded {
		u.Contributors = slices.Insert(u.Contributors, i, p)
	}
	return true
}

// Ref returns the reference of a declaration in a unit of the store: the
// unit's reference, and the declaration's place in the depth-first walk
// of the unit's declarations. It reports false for a declaration no unit
// contains. A declaration that two units contain, or that one unit's
// tree contains twice, returns the place the walk visited first, in the
// order the units arrived.
//
// A call walks the trees of the units that arrived since the previous
// call, and the settle discards every reference, because it may replace
// declarations.
//
// # Allocation contract
//
// A call after no unit arrived allocates nothing. A call after units
// arrived grows the map by one entry for each declaration they contain.
func (e *Emit) Ref(d symbol.Symbol) (EmitRef, bool) {
	for ; e.walked < len(e.units); e.walked++ {
		if e.refs == nil {
			e.refs = map[symbol.Symbol]EmitRef{}
		}
		unit := e.units[e.walked].Ref()
		at := 0
		for _, root := range e.units[e.walked].Decls {
			for s := range emit.All(root) {
				if _, seen := e.refs[s]; !seen {
					e.refs[s] = EmitRef{Unit: unit, Index: at}
				}
				at++
			}
		}
	}
	r, held := e.refs[d]
	return r, held
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
// Only values with a nonzero origin are returned, because every
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

// index walks one unit's declarations into the per-kind index. A first
// walk counts the indexed declarations of each kind, so the unit's list
// of each kind is allocated once, at its final length.
func (e *Emit) index(at int, decls []symbol.Symbol) {
	var counts [kindSlots]int
	for _, d := range decls {
		for s := range emit.All(d) {
			if indexed(s) {
				counts[s.Kind()]++
			}
		}
	}
	for _, d := range decls {
		for s := range emit.All(d) {
			if !indexed(s) {
				continue
			}
			k := s.Kind()
			per := e.byKind[k]
			if per == nil {
				per = map[int][]symbol.Symbol{}
				e.byKind[k] = per
			}
			list := per[at]
			if list == nil {
				list = make([]symbol.Symbol, 0, counts[k])
			}
			per[at] = append(list, s)
		}
	}
}

// indexed reports whether the per-kind index lists a declaration: one
// that carries a nonzero origin, the one thing every emit trigger
// resolves through.
func indexed(s symbol.Symbol) bool {
	origin, carries := emit.OriginOf(s)
	return carries && !origin.IsZero()
}

// reindex rebuilds the per-kind index over the settled units, so a
// reader after the settle never meets a kind a lowering replaced.
func (e *Emit) reindex() {
	e.byKind = map[symbol.Kind]map[int][]symbol.Symbol{}
	for i := range e.units {
		e.index(i, e.units[i].Decls)
	}
	e.order, e.holders, e.refs, e.walked = nil, nil, nil, 0
}

// respelled records the name a declaration was emitted under, which the
// settle replaced with another spelling. The first record allocates the
// map, and a later record of the same declaration replaces the earlier.
func (e *Emit) respelled(d symbol.Symbol, emitted string) {
	if e.emitted == nil {
		e.emitted = map[symbol.Symbol]string{}
	}
	e.emitted[d] = emitted
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
