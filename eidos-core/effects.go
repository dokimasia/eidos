// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"math"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/grow"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// minGrowth is the capacity an empty buffer of a phase call takes on its
// first append.
const minGrowth = 64

// effectKind names what one buffered effect does when it applies.
type effectKind uint8

const (
	// effectTouch touches an accumulator through an accessor call.
	effectTouch effectKind = 1
	// effectPlace places one [Out.Append] call's declarations into the
	// accumulator its handle touched.
	effectPlace effectKind = 2
	// effectSlot appends one [SlotView.Append] call's values into a
	// slot.
	effectSlot effectKind = 3
	// effectFinding reports one finding.
	effectFinding effectKind = 4
	// effectHost names the plugin among the contributors of the unit
	// that contains an emit value whose slots the invocation appended
	// into.
	effectHost effectKind = 5
)

// effect is one buffered effect: the sequence of the invocation that
// made it, where its payload is in the lane's buffer of its kind, and
// what it does.
type effect struct {
	seq  int
	at   int
	kind effectKind
}

// effects is one lane's buffer: every effect its invocations made on
// state the phase call shares, in the order they made them. A lane
// runs its invocations in increasing sequence, so the stream is sorted
// by sequence and one invocation's effects are contiguous in it. The
// payloads are batched per call: an append of n values is one effect
// and n buffered values.
type effects struct {
	stream []effect
	// touches are the accessor calls, one per call.
	touches []touch
	// places are the [Out.Append] calls, each a range of decls.
	places []place
	decls  []symbol.Symbol
	// slotted are the [SlotView.Append] calls, each an index into the
	// lane's typed buffer of its element type.
	slotted []slotted
	found   []finding
	hosts   []symbol.Symbol
	// slots maps each slot element type to the lane's buffer of the
	// appends into slots of that type, keyed by a nil pointer to the
	// type.
	slots map[any]slotApplier
	// lastHost is the sequence of the last invocation that buffered a
	// host, so an invocation names its host once.
	lastHost int
}

// touch is one accessor call, in 96 bytes. It derives the family from
// the key's tag, and the subject from the rule and the matched value.
type touch struct {
	// key addresses the accumulator.
	key accKey
	// fr and value are the rule and the matched value from which the
	// subject of the call's handle derives.
	fr    *flatRule
	value symbol.Symbol
	// instance is the gating instance the handle places under.
	instance int
	// acc is the accumulator, set once the touch applies.
	acc *accumulator
}

// place is one [Out.Append] call: the touch of the handle it went
// through, and its declarations' range in the lane's decls.
type place struct {
	touch    int
	from, to int
}

// slotted is one [SlotView.Append] call: the lane's typed buffer of
// its element type, and the call's index into the buffer.
type slotted struct {
	buf slotApplier
	at  int
}

// finding is one buffered finding. lang names the language of an
// absent-rules warning, which the phase call reports once per
// language, and is empty for every other finding.
type finding struct {
	d    diag.Diag
	lang symbol.Lang
}

// slotApplier is a lane's typed buffer of slot appends, applied one
// append call at a time, and emptied for the lane's next phase call.
type slotApplier interface {
	apply(at int)
	reset()
}

// slotAppend is one buffered append call: the slot, and the range of
// its values in the buffer.
type slotAppend[T any] struct {
	slot     *emit.Slot[T]
	from, to int
}

// slotBuffer is a lane's buffer of the appends into slots of one
// element type.
type slotBuffer[T any] struct {
	values  []T
	appends []slotAppend[T]
}

// apply appends one buffered call's values into its slot, in order.
func (b *slotBuffer[T]) apply(at int) {
	a := b.appends[at]
	a.slot.Append(b.values[a.from:a.to]...)
}

// reset empties the buffer and keeps its storage. It zeroes the values
// and the slots it held, so the buffer retains nothing of the run.
func (b *slotBuffer[T]) reset() {
	clear(b.values)
	clear(b.appends)
	b.values, b.appends = b.values[:0], b.appends[:0]
}

// touch buffers an accessor call and returns its index, which the
// call's handle places under.
func (fx *effects) touch(seq int, t touch) int {
	at := len(fx.touches)
	fx.stream = append(grow.Room(fx.stream, 1, minGrowth), effect{seq: seq, at: at, kind: effectTouch})
	fx.touches = append(grow.Room(fx.touches, 1, minGrowth), t)
	return at
}

// place buffers one append call's declarations, in order, under the
// touch of the handle the call went through.
func (fx *effects) place(seq, touch int, decls []symbol.Symbol) {
	fx.stream = append(grow.Room(fx.stream, 1, minGrowth), effect{seq: seq, at: len(fx.places), kind: effectPlace})
	from := len(fx.decls)
	fx.decls = append(grow.Room(fx.decls, len(decls), minGrowth), decls...)
	fx.places = append(grow.Room(fx.places, 1, minGrowth), place{touch: touch, from: from, to: len(fx.decls)})
}

// report buffers one finding.
func (fx *effects) report(seq int, f finding) {
	fx.stream = append(grow.Room(fx.stream, 1, minGrowth), effect{seq: seq, at: len(fx.found), kind: effectFinding})
	fx.found = append(grow.Room(fx.found, 1, minGrowth), f)
}

// contribute buffers the emit value whose slots an invocation appended
// into, once per invocation. An invocation that matched no emit value
// names no host.
func (fx *effects) contribute(seq int, host symbol.Symbol) {
	if host == nil || seq == fx.lastHost {
		return
	}
	fx.lastHost = seq
	fx.stream = append(grow.Room(fx.stream, 1, minGrowth), effect{seq: seq, at: len(fx.hosts), kind: effectHost})
	fx.hosts = append(grow.Room(fx.hosts, 1, minGrowth), host)
}

// reset empties the lane's effects for its next phase call and keeps
// the storage of every buffer, the typed slot buffers included. It
// zeroes each element that refers into the run.
func (fx *effects) reset() {
	clear(fx.touches)
	clear(fx.decls)
	clear(fx.slotted)
	clear(fx.found)
	clear(fx.hosts)
	for _, buf := range fx.slots {
		buf.reset()
	}
	*fx = effects{
		stream:   fx.stream[:0],
		touches:  fx.touches[:0],
		places:   fx.places[:0],
		decls:    fx.decls[:0],
		slotted:  fx.slotted[:0],
		found:    fx.found[:0],
		hosts:    fx.hosts[:0],
		slots:    fx.slots,
		lastHost: -1,
	}
}

// appendSlot buffers one append call's values for one slot, in order.
func appendSlot[T any](fx *effects, seq int, s *emit.Slot[T], values []T) {
	buf := slotBufferOf[T](fx)
	from := len(buf.values)
	buf.values = append(grow.Room(buf.values, len(values), minGrowth), values...)
	fx.stream = append(grow.Room(fx.stream, 1, minGrowth), effect{seq: seq, at: len(fx.slotted), kind: effectSlot})
	fx.slotted = append(grow.Room(fx.slotted, 1, minGrowth), slotted{buf: buf, at: len(buf.appends)})
	buf.appends = append(grow.Room(buf.appends, 1, minGrowth), slotAppend[T]{slot: s, from: from, to: len(buf.values)})
}

// slotBufferOf returns the lane's buffer of the appends into slots of
// one element type, created on first use.
func slotBufferOf[T any](fx *effects) *slotBuffer[T] {
	key := any((*T)(nil))
	if buf, held := fx.slots[key]; held {
		typed, _ := buf.(*slotBuffer[T])
		return typed
	}
	buf := &slotBuffer[T]{}
	if fx.slots == nil {
		fx.slots = map[any]slotApplier{}
	}
	fx.slots[key] = buf
	return buf
}

// applyEffects applies every lane's buffered effects in canonical
// order: the sequence of the invocation that made them, then the order
// it made them in. Two lanes never share a sequence, so the merge
// takes the next effect of the lane whose next effect has the lowest
// sequence, which applies one invocation's effects together. A call
// that journals fills each effect's record as the effect applies.
func (c *phaseCall) applyEffects() {
	c.merge(math.MaxInt, func(ln *runState, e effect) {
		if c.journal != nil {
			c.fill(ln, e.seq)
		}
		c.applyEffect(ln, e)
	})
}

// reportThrough reports the buffered findings of the invocations up
// to and including the one at seq, and drops every other effect: a
// failed phase call flushes no emit, and its findings up to the
// failure arrive in the sink, as they do when it runs sequentially.
func (c *phaseCall) reportThrough(seq int) {
	c.merge(seq, func(ln *runState, e effect) {
		if e.kind == effectFinding {
			c.applyEffect(ln, e)
		}
	})
}

// merge visits every lane's effects with a sequence up to through, in
// canonical order. The cursors are the call's own buffer.
func (c *phaseCall) merge(through int, visit func(*runState, effect)) {
	c.cursors = slices.Grow(c.cursors[:0], len(c.lanes))[:len(c.lanes)]
	cursors := c.cursors
	clear(cursors)
	for {
		best, lowest := -1, 0
		for i, ln := range c.lanes {
			if cursors[i] == len(ln.fx.stream) {
				continue
			}
			if seq := ln.fx.stream[cursors[i]].seq; seq <= through && (best < 0 || seq < lowest) {
				best, lowest = i, seq
			}
		}
		if best < 0 {
			return
		}
		ln := c.lanes[best]
		visit(ln, ln.fx.stream[cursors[best]])
		cursors[best]++
	}
}

// applyEffect applies one effect a lane buffered. A touch applies
// before every placement through its handle, because both are on one
// lane and the touch is earlier in its stream. Where a record fills, a
// touch, a host and a reported finding also enter it. The report of a
// failed call fills none.
func (c *phaseCall) applyEffect(ln *runState, e effect) {
	switch e.kind {
	case effectTouch:
		t := &ln.fx.touches[e.at]
		t.acc = c.accFor(t.key, subjectOf(t.fr, t.value))
		if c.filling != nil {
			c.units = append(grow.Room(c.units, 1, minGrowth), plugin.UnitRef{
				Plugin: c.plugin, Tag: string(t.key.tag), Pkg: t.acc.pkg, Key: t.acc.key,
			})
			c.filling.units.to = int32(len(c.units))
		}
	case effectPlace:
		p := ln.fx.places[e.at]
		t := &ln.fx.touches[p.touch]
		origin := subjectOf(t.fr, t.value)
		for _, d := range ln.fx.decls[p.from:p.to] {
			t.acc.places = append(t.acc.places, placed{
				origin:   origin,
				instance: t.instance,
				seq:      len(t.acc.places),
				decl:     d,
			})
		}
	case effectSlot:
		s := ln.fx.slotted[e.at]
		s.buf.apply(s.at)
	case effectFinding:
		f := ln.fx.found[e.at]
		if f.lang != "" {
			if c.reported[f.lang] {
				return
			}
			if c.reported == nil {
				c.reported = map[symbol.Lang]bool{}
			}
			c.reported[f.lang] = true
		}
		c.sink.Report(f.d)
		if c.filling != nil {
			c.findings = append(grow.Room(c.findings, 1, minGrowth), f.d)
			c.filling.findings.to = int32(len(c.findings))
		}
	case effectHost:
		if c.emit == nil {
			return
		}
		host := ln.fx.hosts[e.at]
		c.emit.Contribute(host, c.plugin)
		if c.filling == nil {
			return
		}
		if ref, held := c.emit.Ref(host); held {
			c.hosts = append(grow.Room(c.hosts, 1, minGrowth), ref)
			c.filling.hosts.to = int32(len(c.hosts))
		}
	}
}
