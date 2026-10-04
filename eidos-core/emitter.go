// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Tag selects a declared output family; the zero value is the
// primary. Addressing a family the plugin never declared panics:
// the family set is the plugin's own declaration, so the mismatch
// is a defect and panics on the first matching test.
type Tag string

// Emitter is a handler's write surface, scoped to its subject.
//
// Each accessor returns a handle onto the one accumulator for its
// cardinality key and family, created on first touch and appended to
// thereafter, so two interfaces in one source file assemble one
// per-source unit and a package's matches assemble one registry. At
// most one tag per call; more is a defect and panics.
//
// Every write through the Emitter is buffered with the invocation and
// applies when the phase call's rules have run, in canonical match
// order, so the output is the same whether the phase call runs its
// invocations one after another or on several workers. A handler sees
// the plan's store and the slots as they were when its phase call
// began.
//
// # Concurrency
//
// An Emitter belongs to one invocation and is valid during its handler
// call. A handler does not share it with another goroutine.
//
// # Allocation contract
//
// The phase call's buffers and the invocation's handles are pooled, so
// an accessor, an append and a slot view allocate nothing per
// invocation once the call's state is warm. What a call's effects leave
// in the store allocates when they apply: the units, their lists of
// declarations and origins, and the slots' values.
type Emitter struct {
	rs *runState
	m  *match
}

// File returns the accumulator for the subject's source file and
// the family, keyed by the subject's position: a subject without
// one keys the empty string. It allocates nothing for an invocation's
// first four handles, and a handle each after them.
func (e *Emitter) File(tags ...Tag) *Out {
	return e.out(plugin.PerSource, e.m.pos.File, tags)
}

// PackageFile returns the accumulator for the subject's package and
// the family, keyed by the language and the package path the
// subject's identity names. It allocates nothing for an invocation's
// first four handles, and a handle each after them.
func (e *Emitter) PackageFile(tags ...Tag) *Out {
	return e.out(plugin.PerPackage, e.m.subject.Package, tags)
}

// PlanFile returns the accumulator for the plan and the family. A
// plan has one output per family, so its key is empty. It allocates
// nothing for an invocation's first four handles, and a handle each
// after them.
func (e *Emitter) PlanFile(tags ...Tag) *Out {
	return e.out(plugin.PerPlan, "", tags)
}

// Slot returns the invocation's write view of one slot: a slot of a
// value this plugin creates, or, through [OnEmit], a slot of a value
// an earlier bucket placed. Appends through the view apply when the
// phase call's rules have run, in canonical match order, and an append
// into a slot of an earlier bucket's value names the plugin among the
// contributors of the unit that contains the value. The view is valid
// during the handler call. A nil slot panics, because the view would
// append nowhere. Slot returns the view by value and allocates nothing.
func (e *Emitter) Slot[T any](s *emit.Slot[T]) SlotView[T] {
	if s == nil {
		panic("eidos: " + string(e.rs.plugin) + " asks for the view of a nil slot")
	}
	return SlotView[T]{rs: e.rs, seq: e.m.seq, host: e.m.host, slot: s}
}

// JoinName joins a family word onto a base name, the way a
// generated identifier derives from the declaration it serves: the
// base, then the word with its first letter raised, so the boundary
// between them survives. The result is in the neutral convention
// every emitted name takes, and the plan's backend spells it in the
// target's case when the plan settles: store and stub join as
// storeStub, which a Go plan spells StoreStub for a public
// declaration. An empty word returns the base, and an empty base
// returns the word, without allocating. A join allocates the joined
// name, one allocation.
func (*Emitter) JoinName(word, base string) string {
	if word == "" {
		return base
	}
	if base == "" {
		return word
	}
	first, size := utf8.DecodeRuneInString(word)
	var joined strings.Builder
	joined.Grow(len(base) + utf8.UTFMax + len(word) - size)
	joined.WriteString(base)
	joined.WriteRune(unicode.ToUpper(first))
	joined.WriteString(word[size:])
	return joined.String()
}

// Ref returns a reference to one of this plugin's templates, for a
// body to claim: the render resolves the name in this plugin's tree
// for the plan's target, wherever the body is placed, a slot of
// another plugin's declaration included. Data is the payload the
// template executes over. Ref allocates the reference, which the body
// keeps, one allocation.
func (e *Emitter) Ref(name string, data any) *emit.TemplateRef {
	return &emit.TemplateRef{Name: name, Data: data, Owner: e.rs.plugin}
}

// out resolves the family, buffers the accumulator's touch, and
// returns the subject-bound handle.
func (e *Emitter) out(per plugin.Cardinality, key string, tags []Tag) *Out {
	tag := oneTag(tags)
	fam, declared := e.rs.b.outByTag[tag]
	if !declared {
		panic("eidos: " + string(e.rs.plugin) +
			" addresses undeclared output tag " + strconv.Quote(string(tag)))
	}
	if fam.Per != per {
		panic("eidos: " + string(e.rs.plugin) + " addresses family " +
			strconv.Quote(string(tag)) + " at the wrong cardinality")
	}
	k := accKey{tag: tag, per: per, key: key}
	if per == plugin.PerPackage {
		k.lang = e.m.subject.Lang
	}
	instance := 0
	if e.m.gate != nil {
		instance = e.m.gate.Instance
	}
	at := e.rs.fx.touch(e.m.seq, touch{key: k, fam: fam, subject: e.m.subject, instance: instance})
	if e.rs.minted < len(e.rs.handles) {
		h := &e.rs.handles[e.rs.minted]
		e.rs.minted++
		*h = Out{rs: e.rs, seq: e.m.seq, touch: at}
		return h
	}
	return &Out{rs: e.rs, seq: e.m.seq, touch: at}
}

// oneTag returns the selected family: none means the primary, and
// more than one is a defect.
func oneTag(tags []Tag) Tag {
	switch len(tags) {
	case 0:
		return Tag("")
	case 1:
		return tags[0]
	default:
		panic("eidos: at most one tag per emitter call")
	}
}

// Out is one accumulator seen from one match. The handle records the
// invocation's sequence and its accessor call, which records the
// match's subject and gating instance, so an append is attributed and
// ordered without shared mutable state: two matches have two handles
// onto one accumulator.
type Out struct {
	rs    *runState
	seq   int
	touch int
}

// Append places emit declarations under the handle's subject; with
// none it is a no-op and records nothing. The placements apply when
// the phase call's rules have run, and the flush orders a unit's
// declarations by origin identity, then by the gating instance's
// source order under a repeatable directive, then by canonical match
// order and the order of the appends, so output order is canonical
// and never schedule order. The call copies the declarations, so the
// caller may reuse the slice it passed. It buffers them in the phase
// call's pooled storage and allocates nothing once the call's state is
// warm. The flush allocates the unit's lists of declarations and
// origins.
func (o *Out) Append(decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	o.rs.fx.place(o.seq, o.touch, decls)
}

// SlotView is one invocation's write access to one slot, returned by
// [Emitter.Slot]. Its zero value has no invocation to buffer with,
// and appending through it panics.
type SlotView[T any] struct {
	rs   *runState
	seq  int
	host symbol.Symbol
	slot *emit.Slot[T]
}

// Append buffers values for the slot, in order. They apply when the
// phase call's rules have run, after the values of every invocation
// earlier in canonical match order. An append of no values is not
// recorded. The buffer is the phase call's pooled storage, so an append
// allocates nothing once the call's state is warm. The slot grows when
// the values apply, and the store records the plugin among the host
// unit's contributors.
func (v SlotView[T]) Append(values ...T) {
	if v.rs == nil {
		panic("eidos: a zero SlotView appends nowhere; Emitter.Slot returns the view to append through")
	}
	if len(values) == 0 {
		return
	}
	v.rs.fx.contribute(v.seq, v.host)
	appendSlot(&v.rs.fx, v.seq, v.slot, values)
}

// accKey addresses one accumulator: a family under one cardinality
// key. A per-package key includes the language, because two
// languages may spell one package path.
type accKey struct {
	tag  Tag
	per  plugin.Cardinality
	lang symbol.Lang
	key  string
}

// accumulator gathers one output entity's contributions until the
// phase call's effects have applied and the flush orders them. It
// keeps no origin set of its own: the flush reads the unique origins
// off the sorted contributions, so a placement costs no map entry.
type accumulator struct {
	out    plugin.Output
	key    string
	pkg    symbol.Identity
	places []placed
}

// placed is one appended declaration and its ordering key.
type placed struct {
	origin   symbol.Identity
	instance int
	seq      int
	decl     symbol.Symbol
}

// originsOf returns the distinct nonzero origins of places sorted
// in canonical order: counted first, so the slice is allocated at
// its exact size.
func originsOf(places []placed) []symbol.Identity {
	distinct := 0
	prev := symbol.Identity{}
	for _, p := range places {
		if !p.origin.IsZero() && p.origin != prev {
			distinct++
			prev = p.origin
		}
	}
	if distinct == 0 {
		return nil
	}
	out := make([]symbol.Identity, 0, distinct)
	prev = symbol.Identity{}
	for _, p := range places {
		if !p.origin.IsZero() && p.origin != prev {
			out = append(out, p.origin)
			prev = p.origin
		}
	}
	return out
}

// accFor returns the accumulator for one key, bound on first touch
// with its namespace resolved once. It binds an accumulator an earlier
// call released where the call's state has one, so its placements
// append into storage that call grew, and allocates one otherwise. The
// first touch of a state creates the map, so a call that touches
// nothing allocates none.
func (c *phaseCall) accFor(k accKey, fam plugin.Output, subject symbol.Identity) *accumulator {
	if acc, held := c.accs[k]; held {
		return acc
	}
	if c.accs == nil {
		c.accs = map[accKey]*accumulator{}
	}
	var acc *accumulator
	if n := len(c.spare); n > 0 {
		acc, c.spare = c.spare[n-1], c.spare[:n-1]
		acc.out, acc.key = fam, k.key
	} else {
		acc = &accumulator{out: fam, key: k.key}
	}
	if k.per != plugin.PerPlan && !subject.IsZero() {
		if pkg, held := c.index.PackageOf(subject); held {
			acc.pkg = pkg.ID
		}
	}
	c.accs[k] = acc
	return acc
}

// flush turns every touched accumulator into a unit, contributions
// in canonical order, and arrives them in the plan's store. The
// accumulators flush in key order, so refusals arrive in one order. A
// call that touched nothing flushes nothing and allocates nothing.
//
// # Allocation contract
//
// The keys sort in the call's own buffer. Each unit allocates its
// declarations and, where they have origins, its origins, both of which
// the store keeps.
func (c *phaseCall) flush(into *plugin.Emit) error {
	if len(c.accs) == 0 {
		return nil
	}
	c.accKeys = slices.AppendSeq(c.accKeys[:0], maps.Keys(c.accs))
	slices.SortFunc(c.accKeys, func(a, b accKey) int {
		if c := cmp.Compare(a.per, b.per); c != 0 {
			return c
		}
		if c := cmp.Compare(a.key, b.key); c != 0 {
			return c
		}
		if c := cmp.Compare(a.lang, b.lang); c != 0 {
			return c
		}
		return cmp.Compare(a.tag, b.tag)
	})
	for _, k := range c.accKeys {
		acc := c.accs[k]
		slices.SortStableFunc(acc.places, func(a, b placed) int {
			if c := a.origin.Compare(b.origin); c != 0 {
				return c
			}
			if c := cmp.Compare(a.instance, b.instance); c != 0 {
				return c
			}
			return cmp.Compare(a.seq, b.seq)
		})
		decls := make([]symbol.Symbol, 0, len(acc.places))
		for _, p := range acc.places {
			decls = append(decls, p.decl)
		}
		origins := originsOf(acc.places)
		err := into.Add(plugin.Unit{
			Plugin:  c.plugin,
			Tag:     string(k.tag),
			Per:     k.per,
			Word:    acc.out.Word,
			Key:     acc.key,
			Pkg:     acc.pkg,
			Decls:   decls,
			Origins: origins,
		})
		if err != nil {
			return fmt.Errorf("eidos: %s flush: %w", c.plugin, err)
		}
	}
	return nil
}
