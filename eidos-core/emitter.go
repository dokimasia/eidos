// Copyright Dokimasia B.V. 2026
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

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// Tag selects a declared output family. The zero value selects the
// primary family. An emitter call with a family that the plugin did not
// declare panics. The plugin declares its families itself, so the
// mismatch is a defect, and the first matching test panics.
type Tag string

// TagTest is the well-known tag of a test companion. A target spells a
// family under it as its ecosystem spells a test file, such as
// suite_test.go in Go and suite.test.ts in TypeScript. The constant is
// untyped, so it is a [Tag] and the tag of a [plugin.Output] alike.
const TagTest = "test"

// Emitter is the write surface of a handler for its subject.
//
// Each accessor returns a handle to the one accumulator for its
// cardinality key and family. The first touch creates the accumulator,
// and later touches append to it. Two interfaces in one source file
// therefore assemble one per-source unit, and the matches of a package
// assemble one registry. A call takes at most one tag, and more than one
// panics.
//
// Every write through the Emitter is buffered with the invocation. It
// applies in canonical match order when the rules of the phase call have
// run. The output is therefore the same whether the phase call runs its
// invocations one after another or on several workers. A handler sees
// the store of the plan and the slots as they were when its phase call
// began.
//
// # Concurrency
//
// An Emitter belongs to one invocation and is valid during its handler
// call. A handler does not share it with another goroutine.
//
// # Allocation contract
//
// The buffers of the phase call and the handles of the invocation are
// pooled, so an accessor, an append and a slot view allocate nothing per
// invocation once the state of the call is warm. When the effects of a
// call apply, they allocate the units, their lists of declarations and
// origins, and the values of the slots.
type Emitter struct {
	rs *runState
	m  *match
}

// File returns the accumulator for the source file of the subject and
// the family. The key is the file of the subject's position, and the
// empty string for a subject without a position. It allocates nothing
// for the first four handles of an invocation, and one handle for each
// later handle.
func (e *Emitter) File(tags ...Tag) *Out {
	return e.out(plugin.PerSource, e.m.pos.File, tags)
}

// PackageFile returns the accumulator for the package of the subject and
// the family. The key is the language and the package path of the
// subject's identity. It allocates nothing for the first four handles of
// an invocation, and one handle for each later handle.
func (e *Emitter) PackageFile(tags ...Tag) *Out {
	return e.out(plugin.PerPackage, e.m.subject.Package, tags)
}

// PlanFile returns the accumulator for the plan and the family. A plan
// has one output per family, so its key is empty. It allocates nothing
// for the first four handles of an invocation, and one handle for each
// later handle.
func (e *Emitter) PlanFile(tags ...Tag) *Out {
	return e.out(plugin.PerPlan, "", tags)
}

// Slot returns the write view of the invocation for one slot. The slot
// belongs to a value that this plugin creates. In an [OnEmit] handler,
// it can also belong to a value that a plugin of an earlier turn placed.
// Appends through the view apply in canonical match order when the rules
// of the phase call have run. An append into a slot of a value of an
// earlier turn adds the plugin to the contributors of the unit that contains
// the value. The view is valid during the handler call. A nil slot
// panics, because the view would append nowhere. Slot returns the view
// by value and allocates nothing.
func (e *Emitter) Slot[T any](s *emit.Slot[T]) SlotView[T] {
	if s == nil {
		panic("eidos: " + string(e.rs.plugin) + " asks for the view of a nil slot")
	}
	return SlotView[T]{rs: e.rs, seq: e.m.seq, host: e.m.host, slot: s}
}

// JoinName returns head followed by tail, with the first letter of tail
// raised, so the boundary between the two words survives.
// JoinName("stub", "store") returns storeStub. A generated identifier
// derives from the declaration that it serves this way. The result is
// in the neutral convention of every emitted name, and the backend of
// the plan writes it in the case of the target when the plan settles.
// A Go plan writes storeStub as StoreStub for a public declaration. An
// empty tail returns head, and an empty head returns tail, without
// allocating. A join allocates the joined name, one allocation.
func (*Emitter) JoinName(tail, head string) string {
	if tail == "" {
		return head
	}
	if head == "" {
		return tail
	}
	first, size := utf8.DecodeRuneInString(tail)
	var joined strings.Builder
	joined.Grow(len(head) + utf8.UTFMax + len(tail) - size)
	joined.WriteString(head)
	joined.WriteRune(unicode.ToUpper(first))
	joined.WriteString(tail[size:])
	return joined.String()
}

// Ref returns a reference to one of the templates of this plugin, for a
// body to claim. The render resolves the name in the tree of this plugin
// for the target of the plan, wherever the body is placed. That includes
// a slot of a declaration of another plugin. data is the payload of the
// template. Ref allocates the reference that the body keeps, one
// allocation.
func (e *Emitter) Ref(name string, data any) *emit.TemplateRef {
	return &emit.TemplateRef{Name: name, Data: data, Owner: e.rs.plugin}
}

// Type returns the reference of the plan's target for ref. owner is the
// declaration with ref, such as the field whose type the generator
// mirrors. Type reports false for a type without a spelling in the
// target, after it reports the refusal. A nil ref returns nil and true.
//
// A reference in the language of the plan's target translates as
// written, through [rules.EmitRef]. A reference to a type parameter
// translates to a reference that has the parameter's name and no target.
// Every other reference folds into its shape under the rules of the
// language of owner. The fold reads through the view of the invocation,
// so each declaration that it reads records on the invocation. Where
// owner is a field of the graph and ref is the type of that field, the
// fold is [rules.Bound.FieldTypeOf], so the presence of the field
// translates as an optional. The spoke of the plan spells the shape
// under the policy of the plan. When the spoke reads the choice of a
// policy key, the policy first reads the value of the key at directive
// authority on owner, then on the subject of the match, and then on the
// package of owner. The first value that it finds is the choice. Each of
// these reads records on the invocation, so the invocation records the
// overrides of the keys that the type depends on and no others.
//
// A refusal reports an Error under [RefusedType] at the position of
// owner. The message contains the target, the source spelling and the
// reason of the spoke. A plan whose backend does not implement a spoke
// refuses every reference of another language.
//
// # Allocation contract
//
// Type allocates what [rules.EmitRef] allocates for a reference in the
// target's language, and one reference for a type parameter. For any
// other reference it allocates what the fold allocates and the reference
// that the spoke returns. The lookup of a field owner allocates only
// where the read set of the invocation grows. The fold allocates the memo
// of the binding on the first fold of the invocation in the subject's
// language, and on every call where the language of owner is not the
// subject's. A refusal allocates its message. The match makes its reader
// of overrides on its first translation in a phase call, one allocation,
// and an override read allocates only where the read set of the
// invocation grows.
func (e *Emitter) Type(owner symbol.Identity, ref *node.TypeRef) (*emit.TypeRef, bool) {
	if ref == nil {
		return nil, true
	}
	rs, m := e.rs, e.m
	if owner.Lang == symbol.Lang(rs.target) {
		return rules.EmitRef(ref), true
	}
	if ref.Target.Kind == symbol.KindTypeParam {
		return &emit.TypeRef{Spelling: ref.Spelling}, true
	}
	if rs.types == nil {
		rs.reportf(m.seq, RefusedType, diag.SeverityError, rs.positionOf(owner),
			"the %s target cannot spell the %s type %s: the plan's backend does not implement a type spoke",
			rs.target, owner.Lang, ref.Spelling)
		return nil, false
	}
	var b rules.Bound
	if owner.Lang == m.subject.Lang {
		b = m.Rules()
	} else {
		b = m.RulesFor(owner.Lang)
	}
	var shape rules.TypeShape
	if f, field := fieldOf(b.View(), owner); field && f.Type == ref {
		shape = b.FieldTypeOf(f)
	} else {
		shape = b.TypeOf(ref)
	}
	m.translating = owner
	spelled, err := rs.types.SpellType(shape, rs.policy.Overridden(m.overrides()))
	m.translating = symbol.Identity{}
	if err != nil {
		rs.reportf(m.seq, RefusedType, diag.SeverityError, rs.positionOf(owner),
			"the %s target cannot spell the %s type %s: %v", rs.target, owner.Lang, ref.Spelling, err)
		return nil, false
	}
	return spelled, true
}

// Fields translates the type of each field of fields that keep reports,
// with the field as the owner, as [Emitter.Type] does. It returns the emit
// fields in the order of fields, and a nil keep keeps every field. Each
// emit field has the origin, the documentation, the name, the visibility
// and the translated type of its field.
//
// Fields reports false for a field whose type has no spelling in the
// target, after Type reports the refusal, and then returns no fields. The
// caller withholds its declaration, as a generator withholds a declaration
// that it cannot write whole. A caller passes the result to [emit.SlotOf]
// in the composite literal of the declaration, which keeps the list as the
// storage of the slot.
//
// # Allocation contract
//
// Fields allocates the list of the emit fields at the length of fields,
// each emit field, and what Type allocates for each type. It allocates no
// list for no fields.
func (e *Emitter) Fields(fields []*node.Field, keep func(*node.Field) bool) ([]*emit.Field, bool) {
	out := make([]*emit.Field, 0, len(fields))
	for _, f := range fields {
		if keep != nil && !keep(f) {
			continue
		}
		t, spelled := e.Type(f.ID, f.Type)
		if !spelled {
			return nil, false
		}
		out = append(out, &emit.Field{Origin: f.ID, Doc: f.Doc, Name: f.Name, Visibility: f.Visibility, Type: t})
	}
	return out, true
}

// out resolves the family, buffers the touch of the accumulator, and
// returns the handle for the subject.
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
	at := e.rs.fx.touch(e.m.seq, touch{key: k, fr: e.m.fr, value: e.m.value, instance: instance})
	if e.rs.minted < len(e.rs.handles) {
		h := &e.rs.handles[e.rs.minted]
		e.rs.minted++
		*h = Out{rs: e.rs, seq: e.m.seq, touch: at}
		return h
	}
	return &Out{rs: e.rs, seq: e.m.seq, touch: at}
}

// fieldOf returns the field that owner identifies. It reads through the
// view v of the invocation, so the read records. It returns false for an
// owner of another kind and for an owner that the view does not contain.
// It allocates only where the read set grows.
func fieldOf(v rules.View, owner symbol.Identity) (*node.Field, bool) {
	if owner.Kind != symbol.KindField {
		return nil, false
	}
	decl, held := v.Lookup(owner)
	if !held {
		return nil, false
	}
	f, is := decl.(*node.Field)
	return f, is
}

// oneTag returns the selected family. No tag selects the primary
// family, and more than one tag panics.
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

// Out is one accumulator as one match sees it. The handle records the
// sequence of the invocation and its accessor call. The accessor call
// records the subject of the match and the gating instance. An append is
// therefore attributed and ordered without shared mutable state, and two
// matches have two handles to one accumulator.
type Out struct {
	rs    *runState
	seq   int
	touch int
}

// Append places emit declarations under the subject of the handle. A
// call without declarations does nothing and records nothing. The
// placements apply when the rules of the phase call have run. The flush
// orders the declarations of a unit by origin identity, then by the
// source order of the gating instance under a repeatable directive, then
// by canonical match order and the order of the appends. The output
// order is therefore canonical and never the order of the schedule. The
// call copies the declarations, so the caller may reuse the slice that it
// passed. It buffers them in the pooled storage of the phase call and
// allocates nothing once the state of the call is warm. The flush
// allocates the lists of declarations and origins of the unit.
func (o *Out) Append(decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	o.rs.fx.place(o.seq, o.touch, decls, nil)
}

// Record places emit declarations under the subject of the handle as
// [Out.Append] does, and attaches data to the origin of each of them,
// for a family that claims its files through [plugin.Output.Template].
// The file template of the family reads each declaration as a record,
// with the data of its origin beside it. Where two calls attach data to
// one origin of a unit, the data of the first call in canonical match
// order applies. A call without declarations does nothing and records
// nothing. It buffers as Append does, and the flush allocates the map of
// data of the unit.
func (o *Out) Record(data any, decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	o.rs.fx.place(o.seq, o.touch, decls, data)
}

// SlotView is the write access of one invocation to one slot.
// [Emitter.Slot] returns it. Its zero value has no invocation, so an
// append through it panics.
type SlotView[T any] struct {
	rs   *runState
	seq  int
	host symbol.Symbol
	slot *emit.Slot[T]
}

// Append buffers values for the slot, in order. They apply when the
// rules of the phase call have run, after the values of every invocation
// earlier in canonical match order. An append of no values is not
// recorded. The buffer is the pooled storage of the phase call, so an
// append allocates nothing once the state of the call is warm. The slot
// grows when the values apply, and the store adds the plugin to the
// contributors of the host unit.
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

// accKey is the key of one accumulator. It is a family under one
// cardinality key. A per-package key includes the language, because two
// languages can have the same package path.
type accKey struct {
	tag  Tag
	per  plugin.Cardinality
	lang symbol.Lang
	key  string
}

// accumulator gathers the contributions of one output entity until the
// effects of the phase call have applied and the flush orders them. It
// keeps no origin set of its own. The flush reads the unique origins
// from the sorted contributions, so a placement costs no map entry.
type accumulator struct {
	out    plugin.Output
	key    string
	pkg    symbol.Identity
	places []placed
}

// placed is one appended declaration with its ordering key and the data
// that an [Out.Record] call attached to it. The data is nil for
// [Out.Append].
type placed struct {
	origin   symbol.Identity
	instance int
	seq      int
	decl     symbol.Symbol
	data     any
}

// originsOf returns the distinct nonzero origins of places in canonical
// order. It counts them first, so the slice has its exact size.
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

// accFor returns the accumulator for one key. The first touch binds it
// to the family of the tag of the key and resolves its namespace once.
// It reuses an accumulator that an earlier call released where the state
// of the call has one, so its placements append into storage that the
// earlier call grew. Otherwise it allocates one. The first touch of a
// state creates the map, so a call that touches nothing allocates none.
func (c *phaseCall) accFor(k accKey, subject symbol.Identity) *accumulator {
	if acc, held := c.accs[k]; held {
		return acc
	}
	if c.accs == nil {
		c.accs = map[accKey]*accumulator{}
	}
	fam := c.b.outByTag[k.tag]
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

// flush turns every touched accumulator into a unit with its
// contributions in canonical order, and adds each unit to the store of
// the plan. The accumulators flush in key order, so refusals arrive in
// one order. A call that touched nothing flushes nothing and allocates
// nothing.
//
// # Allocation contract
//
// The keys sort in the buffer of the call. Each unit allocates its
// declarations and, where they have origins, its origins. The store
// keeps both.
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
		var data map[symbol.Identity]any
		for _, p := range acc.places {
			decls = append(decls, p.decl)
			if p.data == nil {
				continue
			}
			origin, _ := emit.OriginOf(p.decl)
			if _, attached := data[origin]; !attached {
				if data == nil {
					data = map[symbol.Identity]any{}
				}
				data[origin] = p.data
			}
		}
		origins := originsOf(acc.places)
		err := into.Add(plugin.Unit{
			Plugin:   c.plugin,
			Tag:      string(k.tag),
			Per:      k.per,
			Word:     acc.out.Word,
			Key:      acc.key,
			Pkg:      acc.pkg,
			Decls:    decls,
			Origins:  origins,
			Template: acc.out.Template,
			Data:     data,
		})
		if err != nil {
			return fmt.Errorf("eidos: %s flush: %w", c.plugin, err)
		}
	}
	return nil
}
