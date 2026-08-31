// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"

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
// Each accessor returns the one accumulator for its cardinality key
// and family, created on first touch and appended to thereafter, so
// two interfaces in one source file assemble one per-source unit
// and a package's matches assemble one registry. At most one tag
// per call; more is a defect and panics.
type Emitter struct {
	rs *runState
	m  *match
}

// File returns the accumulator for the subject's source file and
// the family, keyed by the subject's position: a subject without
// one keys the empty string.
func (e *Emitter) File(tags ...Tag) *Out {
	return e.out(plugin.PerSource, e.m.pos.File, tags)
}

// PackageFile returns the accumulator for the subject's package and
// the family, keyed by the package path the subject's identity
// names.
func (e *Emitter) PackageFile(tags ...Tag) *Out {
	return e.out(plugin.PerPackage, e.m.subject.Package, tags)
}

// PlanFile returns the accumulator for the plan and the family. A
// plan has one output per family, so its key is empty.
func (e *Emitter) PlanFile(tags ...Tag) *Out {
	return e.out(plugin.PerPlan, "", tags)
}

// out resolves the family and returns the subject-bound handle.
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
	acc := e.rs.accFor(accKey{tag: tag, per: per, key: key}, fam, e.m.subject)
	instance := 0
	if e.m.gate != nil {
		instance = e.m.gate.Instance
	}
	if e.rs.minted < len(e.rs.handles) {
		h := &e.rs.handles[e.rs.minted]
		e.rs.minted++
		*h = Out{acc: acc, subject: e.m.subject, instance: instance}
		return h
	}
	return &Out{acc: acc, subject: e.m.subject, instance: instance}
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

// Out is one accumulator seen from one match. The handle carries
// the match's subject and gating instance, so an append is
// attributed without shared mutable state: two matches hold two
// handles onto one accumulator.
type Out struct {
	acc      *accumulator
	subject  symbol.Identity
	instance int
}

// Append places emit declarations under the handle's subject; with
// none it is a no-op and records nothing. The flush orders a unit's
// declarations by origin identity, then by the gating instance's
// source order under a repeatable directive, then by insertion, so
// output order is canonical and never match order. A slot append
// inside an already-placed declaration needs no Out at all: the
// value is placed, and slots are the composition seam.
func (o *Out) Append(decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	for _, d := range decls {
		o.acc.places = append(o.acc.places, placed{
			origin:   o.subject,
			instance: o.instance,
			seq:      len(o.acc.places),
			decl:     d,
		})
	}
}

// accKey addresses one accumulator: a family under one cardinality
// key.
type accKey struct {
	tag Tag
	per plugin.Cardinality
	key string
}

// accumulator gathers one output entity's contributions until the
// phase call returns and the flush orders them. It keeps no origin
// set of its own: the flush reads the unique origins off the
// sorted contributions, so an append costs no map entry.
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

// accFor returns the accumulator for one key, created on first
// touch with its namespace resolved once.
func (rs *runState) accFor(k accKey, fam plugin.Output, subject symbol.Identity) *accumulator {
	if acc, held := rs.accs[k]; held {
		return acc
	}
	acc := &accumulator{out: fam, key: k.key}
	if k.per != plugin.PerPlan && !subject.IsZero() {
		if pkg, held := rs.index.PackageOf(subject); held {
			acc.pkg = pkg.ID
		}
	}
	rs.accs[k] = acc
	return acc
}

// flush turns every touched accumulator into a unit, contributions
// in canonical order, and arrives them in the plan's store. The
// accumulators flush in key order, so refusals arrive in one order.
func (rs *runState) flush(into *plugin.Emit) error {
	keys := slices.SortedFunc(maps.Keys(rs.accs), func(a, b accKey) int {
		if c := cmp.Compare(a.per, b.per); c != 0 {
			return c
		}
		if c := cmp.Compare(a.key, b.key); c != 0 {
			return c
		}
		return cmp.Compare(a.tag, b.tag)
	})
	for _, k := range keys {
		acc := rs.accs[k]
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
			Plugin:  rs.plugin,
			Tag:     string(k.tag),
			Per:     k.per,
			Word:    acc.out.Word,
			Key:     acc.key,
			Pkg:     acc.pkg,
			Decls:   decls,
			Origins: origins,
		})
		if err != nil {
			return fmt.Errorf("eidos: %s flush: %w", rs.plugin, err)
		}
	}
	return nil
}
