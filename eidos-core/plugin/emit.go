// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"fmt"
	"iter"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// kindSlots is the number of values of a [symbol.Kind], which is a
// uint8. An array of this length counts by kind without depending on the
// generated count of kinds.
const kindSlots = math.MaxUint8 + 1

// Cardinality is the number of outputs that a family produces. The zero
// value addresses nothing. Every declared family states its cardinality,
// and [Emit.Add] refuses a unit that does not.
type Cardinality uint8

const (
	// PerSource is one output per source file.
	PerSource Cardinality = iota + 1
	// PerPackage is one output per package.
	PerPackage
	// PerPlan is one output per plan.
	PerPlan
)

// String returns the spelling of the cardinality in a diagnostic, which
// is per-source, per-package or per-plan. A cardinality outside the three
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

// Output declares one file family that a generator emits.
//
// The target spells the join and the extension of Word at render, so one
// declaration serves store_stub.go and store.stub.ts alike.
type Output struct {
	// Tag selects the family, and "" selects the primary family.
	Tag string
	Per Cardinality
	// External reports that the files of the family test their package
	// through its exported API, as an importer does. The Go target writes
	// each test file of the family into the external test package of its
	// directory. The other targets write the files of the family as they
	// write the files of any family.
	External bool
	// Word is the word of the family, such as "stub".
	Word string
	// Template is the slash-separated path of the file template that
	// writes each file of the family, in the template tree of the plugin
	// for the target of the plan. A family with a template claims its
	// files. The template writes the declarations of each file, and the
	// kind templates of the target write none of them. The empty Template
	// leaves the files to the kind templates.
	Template string
}

// Unit is one accumulated output entity. It contains everything that one
// plugin contributed to one family and cardinality key in one phase
// call. It contains the full routing key, so no consumer derives any
// part of the key from the declarations again.
type Unit struct {
	// Plugin is the contributor. A manifest names it in the attribution,
	// and it is part of what makes the flush of one accumulator unique.
	Plugin ID
	// Tag selects the declared family, and "" selects the primary family.
	Tag string
	Per Cardinality
	// Route is how the layout placed the unit. It is zero for a unit that
	// the layout has not placed, such as a unit of the emit store.
	Route Route
	// Word is the word of the family. The target spells its join and its
	// extension at render.
	Word string
	// Key addresses the accumulator. It is the source file path of the
	// subject for PerSource, the package path for PerPackage, and "" for
	// PerPlan. The stem of a rendered filename derives from the
	// per-source key, so the key and the filename cannot disagree.
	Key string
	// Pkg is the identity of the package of a per-source or a per-package
	// unit, and zero for a plan unit. It is the namespace half of the
	// routing key.
	Pkg symbol.Identity
	// Decls are the emit declarations, ordered by the source position of
	// their origin, then by origin identity, then by instance order under
	// a repeatable directive, then by insertion. An origin without a
	// position orders after every origin with one. Contributions arrive in
	// the order of the source and never in dispatch order.
	Decls []symbol.Symbol
	// Origins are the node identities whose matches contributed, sorted
	// and deduplicated. A manifest records them as the provenance.
	Origins []symbol.Identity
	// Subjects maps the origin of each declaration that a match placed for
	// another subject to that subject, such as the double of a handle that
	// the handler of an interface writes. The layout routes such a
	// declaration with the routing of the subject where the origin has
	// none. Where two matches place declarations of one origin, the subject
	// of the first placement in the order of Decls applies. It is nil where
	// every declaration of the unit originates from the subject of its
	// match.
	Subjects map[symbol.Identity]symbol.Identity
	// Contributors are the plugins other than the plugin of the unit that
	// appended into slots of its declarations through their Emitter,
	// sorted. The frame of the file and its manifest entry name a weaver
	// from this list beside the plugin of the unit.
	Contributors []ID
	// Template is the file template of the family of the unit, from its
	// [Output.Template], and empty for a family that claims no file. The
	// declarations of a unit with a template are records. They are data
	// for the template, which the settle respells, and no reference,
	// collision check or export reads them as declarations of the file.
	Template string
	// Data contains the data that the plugin of the unit attached to the
	// declarations of each origin. The file template reads it beside each
	// record. It is nil for a unit whose plugin attached none.
	Data map[symbol.Identity]any
	// Inside are the origins whose declarations test the package of the
	// unit from inside, sorted and distinct, in a unit of a family that
	// states [Output.External], such as the check of an unexported
	// declaration. A file with such a declaration does not test from
	// outside, and a target that writes external test packages splits
	// the declarations into a file of their own. Inside is nil where
	// every declaration of the unit tests from outside.
	Inside []symbol.Identity
}

// FileKey returns the routing key from which the filename of a unit
// takes its stem. That is the source path of a per-source unit, and the
// empty string for a per-package or per-plan unit, whose filename joins
// its family word and its tag alone. The filename spelling of a target
// reads the stem through FileKey, so no target spells a package path into
// a filename. It allocates nothing.
func (u Unit) FileKey() string {
	if u.Per != PerSource {
		return ""
	}
	return u.Key
}

// Ref returns the reference of the unit, which is the key under which one
// phase call flushes the unit. It allocates nothing.
func (u Unit) Ref() UnitRef {
	return UnitRef{Plugin: u.Plugin, Tag: u.Tag, Pkg: u.Pkg, Key: u.Key}
}

// Emit contains the accumulated units of one plan, and a per-kind index
// over their declarations that [Emit.Add] maintains. Add walks the tree
// of each unit once when it arrives, so an emit-triggered rule
// enumerates its matches and not the whole emit graph.
//
// The store seals every slot of each declaration that [Emit.Add]
// receives, through [emit.Seal]. Until the settle starts, a direct
// append into such a slot panics, so a plugin writes into a value of the
// store through the slot view of its emitter alone. The settle lifts the
// seal, so the lowering hooks of the backend rewrite the declarations.
//
// # Concurrency
//
// An Emit is not safe for concurrent use. Annotators and generators run
// one after another, and a unit that arrives during an enumeration
// would race the index that the enumeration reads.
type Emit struct {
	units []Unit
	held  map[UnitRef]struct{}
	// byKind maps a kind to the declarations with an origin that the
	// tree of each unit contains, in walk order, keyed by the index of
	// the unit in units.
	byKind map[symbol.Kind]map[int][]symbol.Symbol
	// order caches the unit indexes in the order of Units. It is nil after
	// a unit arrived since the cache was built.
	order []int
	// holders maps each declaration that the per-kind index lists to the
	// index of its unit in units. The first [Emit.Contribute] after a
	// unit arrived builds it, so a plan without contributions never
	// builds it.
	holders map[symbol.Symbol]int
	// refs maps each declaration of the trees of the walked units to its
	// reference, and walked counts those units, in arrival order.
	// [Emit.Ref] walks the units that arrived since its last call, so a
	// plan whose phase calls journal nothing never builds the map.
	refs   map[symbol.Symbol]EmitRef
	walked int
	// sealed is the flag that every sealed slot of the store reads. It is
	// true from [NewEmit] until the settle starts.
	sealed bool
	// settled reports that the store passed through [Settle]. The
	// lowering seams of the backend ran, and the declarations that the
	// readers see are the declarations that render.
	settled bool
	// translates reports that the settle found a type reference whose
	// target is a declaration of another language than the target of the
	// backend.
	translates bool
	// emitted maps each declaration whose name the settle changed to the
	// name it was emitted under, which [NewExport] keys an export on. The
	// settle allocates it at its first change, so a store whose names all
	// kept their spelling has none.
	emitted map[symbol.Symbol]string
}

// NewEmit returns an empty emit store, which seals the declarations that
// it receives. It allocates three times: the store, its set of
// accumulator keys and its map of kinds.
func NewEmit() *Emit {
	return &Emit{
		sealed: true,
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

// Add records one unit, indexes its tree and seals every slot of its
// declarations. A direct append into such a slot then panics until the
// settle starts.
//
// A second unit under the same (plugin, tag, package, key) is
// refused as the defect it is: one phase call flushes each
// accumulator once. A zero cardinality and an empty word are refused
// the same way, because a unit missing either cannot be routed, and so
// is a plan unit naming a key, because a plan has one output and its
// key is empty. A nil declaration is refused, and so is a nil pointer of
// a declaration kind, because no renderer could spell either.
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
		// A nil pointer of a declaration kind, such as a header that a
		// helper returns nil for, is no declaration either.
		if v := reflect.ValueOf(d); d == nil || v.Kind() == reflect.Pointer && v.IsNil() {
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
	for _, d := range u.Decls {
		emit.Seal(d, &e.sealed)
	}
	e.order, e.holders = nil, nil
	return nil
}

// Contribute records that a plugin appended into a slot of a declaration
// that the store contains. The unit whose tree contains host adds the
// plugin to its [Unit.Contributors]. Contribute reports false for a host
// that the per-kind index does not list, which is a declaration without
// an origin or a declaration that no unit contains. It does not record
// the plugin of the unit itself, and it records a plugin once.
//
// # Allocation contract
//
// The first contribution after a unit arrived allocates the map from
// each indexed declaration to its unit, sized to the declarations, so
// it does not grow while it fills. A contribution that records a plugin
// new to its unit grows the list of contributors of the unit, and any
// other contribution allocates nothing.
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

// Ref returns the reference of a declaration in a unit of the store. The
// reference is the reference of the unit and the index of the declaration
// in the depth-first walk of the declarations of the unit. Ref reports
// false for a declaration that no unit contains. A declaration that two
// units contain, or that the tree of one unit contains twice, returns the
// index that the walk visited first, in the order in which the units
// arrived.
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

// Units enumerates every unit by plugin, then by cardinality, key,
// package and tag. The order is total, so two runs agree whatever the
// order in which the units arrived.
func (e *Emit) Units() iter.Seq[Unit] {
	return func(yield func(Unit) bool) {
		for _, at := range e.sorted() {
			if !yield(e.units[at]) {
				return
			}
		}
	}
}

// ByKind enumerates the emit declarations of one kind across every unit,
// in the order of [Emit.Units], with the tree of each unit walked depth
// first.
//
// It returns only values with a nonzero origin, because the predicates,
// the skips and the reports of every emit trigger resolve through the
// origin. A value whose producer left the origin zero still arrives in
// its unit, but it is not a subject.
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

// index walks the declarations of one unit into the per-kind index. A
// first walk counts the indexed declarations of each kind, so the list of
// each kind of the unit is allocated once, at its final length.
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

// indexed reports whether the per-kind index lists a declaration. The
// index lists a declaration with a nonzero origin, because every emit
// trigger resolves through the origin.
func indexed(s symbol.Symbol) bool {
	origin, carries := emit.OriginOf(s)
	return carries && !origin.IsZero()
}

// reindex rebuilds the per-kind index over the settled units, so a
// reader after the settle never meets a kind that a lowering replaced.
func (e *Emit) reindex() {
	e.byKind = map[symbol.Kind]map[int][]symbol.Symbol{}
	for i := range e.units {
		e.index(i, e.units[i].Decls)
	}
	e.order, e.holders, e.refs, e.walked = nil, nil, nil, 0
}

// respelled records the name under which a declaration was emitted,
// where the settle replaced that name with another spelling. The first
// record allocates the map, and a later record of the same declaration
// replaces the earlier record.
func (e *Emit) respelled(d symbol.Symbol, emitted string) {
	if e.emitted == nil {
		e.emitted = map[symbol.Symbol]string{}
	}
	e.emitted[d] = emitted
}

// sorted returns the unit indexes in the order of Units. It rebuilds the
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
