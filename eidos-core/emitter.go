// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
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

// FileOf returns the accumulator for the source file of the declaration
// id and the family. id is a declaration of the package of the subject,
// such as the first error value of a package match. The key is the file
// of the position of id. FileOf reads id through the tracked reader, so
// the invocation records the read, and a move of the declaration to
// another file runs the handler again.
//
// FileOf refuses an identity that the package of the subject does not
// declare, and an identity of another package is one. It reports an
// Error under [RefusedFile] at the position of the subject, and returns
// the accumulator that [Emitter.File] returns. The run then fails.
//
// FileOf allocates what File allocates. The lookup allocates only where
// the read set of the invocation grows, and the first tracked read of a
// phase call allocates the reader of the lane. A refusal allocates its
// message.
func (e *Emitter) FileOf(id symbol.Identity, tags ...Tag) *Out {
	m := e.m
	var decl symbol.Symbol
	held := false
	if id.Lang == m.subject.Lang && id.Package == m.subject.Package {
		decl, held = m.Reader().Lookup(id)
	}
	if !held {
		e.rs.reportf(m.seq, RefusedFile, diag.SeverityError, m.pos,
			"%s is not a declaration of the package of %s: a handler addresses the files of its own package",
			id, m.subject)
		return e.File(tags...)
	}
	return e.out(plugin.PerSource, decl.Position().File, tags)
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
// written, through [rules.EmitRef]. Every other reference folds into its
// shape under the rules of the language of owner, a reference to a type
// parameter included, so the spoke decides at every depth of a structural
// type. A spoke of a language with type parameters spells a parameter by
// its name and without a target. The fold reads through the view of the
// invocation, so each declaration that it reads records on the
// invocation. Where owner is a field of the graph and ref is the type of
// that field, the fold is [rules.Bound.FieldTypeOf], so the presence of
// the field translates as an optional. The spoke of the plan spells the
// shape under the policy of the plan. When the spoke reads the choice of a
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
// target's language. For any other reference it allocates what the fold
// allocates and the reference that the spoke returns. The lookup of a
// field owner allocates only where the read set of the invocation grows.
// The fold allocates the memo of the binding on the first fold of the
// invocation in the subject's language, and on every call where the
// language of owner is not the subject's. A refusal allocates its
// message. The match makes its reader of overrides on its first
// translation in a phase call, one allocation, and an override read
// allocates only where the read set of the invocation grows.
func (e *Emitter) Type(owner symbol.Identity, ref *node.TypeRef) (*emit.TypeRef, bool) {
	return e.translate(owner, ref, false)
}

// Supertype translates a type that the declaration owner extends,
// implements or embeds, or that bounds one of its type parameters. It
// translates as [Emitter.Type] does, and spells the shape through the
// supertype spelling of the plan's backend, [plugin.SupertypeSpeller]. A
// backend without one spells a supertype as a value. Its refusals, its
// reads and its allocations are those of Type.
//
// A target can spell the two positions apart. Rust writes the supertrait
// of a trait as the bare name Node, and a value of the trait as the boxed
// trait object Box<dyn Node>. Every other target spells both alike.
func (e *Emitter) Supertype(owner symbol.Identity, ref *node.TypeRef) (*emit.TypeRef, bool) {
	return e.translate(owner, ref, true)
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

// TypeParams translates the type parameters tps of the declaration owner
// into the plan's target. Each emit parameter keeps the name, the variance,
// the const mark and the spelling of the default value of its parameter.
// Its bounds translate as supertypes of owner, as [Emitter.Supertype]
// translates them, and its default and the type of its value translate as
// [Emitter.Type] translates a type. TypeParams returns nil and true for no
// parameter.
//
// TypeParams reports false for a bound, a default or a type of a value
// without a spelling in the target, after the translation reports the
// refusal, and then returns no parameters. The caller withholds its
// declaration, as it does after a refusal of [Emitter.Fields].
//
// # Allocation contract
//
// TypeParams allocates the list of the emit parameters at the length of
// tps, each emit parameter, the list of the bounds of each parameter with
// bounds, and what each translation allocates. It allocates nothing for no
// parameter.
func (e *Emitter) TypeParams(owner symbol.Identity, tps []*node.TypeParam) ([]*emit.TypeParam, bool) {
	if len(tps) == 0 {
		return nil, true
	}
	out := make([]*emit.TypeParam, 0, len(tps))
	for _, tp := range tps {
		p := &emit.TypeParam{Name: tp.Name, Variance: tp.Variance, Const: tp.Const, DefaultValue: tp.DefaultValue}
		if len(tp.Bounds) > 0 {
			p.Bounds = make([]*emit.TypeRef, 0, len(tp.Bounds))
		}
		for _, b := range tp.Bounds {
			bound, spelled := e.Supertype(owner, b)
			if !spelled {
				return nil, false
			}
			p.Bounds = append(p.Bounds, bound)
		}
		var spelled bool
		if p.Default, spelled = e.Type(owner, tp.Default); !spelled {
			return nil, false
		}
		if p.Type, spelled = e.Type(owner, tp.Type); !spelled {
			return nil, false
		}
		out = append(out, p)
	}
	return out, true
}

// Handler returns the method of the plan's target that serves rpc, in the
// form of the gRPC binding that the policy of the plan selects for it. It
// projects rpc under the rules of its language, with the context of the call
// among the parameters, and asks the [plugin.Binder] of the plan's backend.
// A backend without a binder serves the method of
// [plugin.CanonicalHandler]. The binder spells each reference of the rpc as
// [Emitter.Type] translates a type with rpc as the owner, so each
// declaration that a fold reads records on the invocation, and the policy
// reads the overrides of its keys on rpc first. The service of a
// [plugin.RPC] is the interface whose owner chain declares rpc.
//
// Handler reports false for a nil rpc. A refusal of the binder or of a
// spelling reports an Error under [RefusedHandler] at the position of rpc,
// and Handler reports false. The message contains the target, the rpc and
// the reason.
//
// # Allocation contract
//
// Handler allocates the parameter list and the return list of the
// projection, what each translation allocates as Type states it, the method
// that the binder returns, and the closure that spells the references. A
// refusal allocates its message.
func (e *Emitter) Handler(rpc *node.Method) (*emit.Method, bool) {
	if rpc == nil {
		return nil, false
	}
	rs, m := e.rs, e.m
	b := e.rulesOf(rpc.ID)
	// A method of the graph is a callable, so the projection reports true.
	c, _ := b.CallableOf(rpc)
	r := plugin.RPC{ID: rpc.ID, Service: serviceOf(rpc.ID), Doc: rpc.Doc, Visibility: rpc.Visibility, Callable: c}
	translate := plugin.Translate(func(ref *node.TypeRef) (*emit.TypeRef, error) {
		if ref == nil {
			return nil, errUntyped
		}
		return e.typeOf(rpc.ID, ref, false)
	})
	prev := m.translating
	m.translating = rpc.ID
	p := rs.policy.Overridden(m.overrides())
	var (
		method *emit.Method
		err    error
	)
	switch binder, binds := rs.types.(plugin.Binder); {
	case binds:
		method, err = binder.Handler(r, p, translate)
	case rs.types != nil:
		method, err = plugin.CanonicalHandler(r, p, rs.types, translate)
	default:
		err = errNoSpoke
	}
	m.translating = prev
	if err != nil {
		rs.reportf(m.seq, RefusedHandler, diag.SeverityError, rs.positionOf(rpc.ID),
			"the %s target cannot serve the rpc %s: %v", rs.target, rpc.ID, err)
		return nil, false
	}
	return method, true
}

// errNoSpoke is the reason of a refusal of a plan whose backend does not
// implement a type spoke.
var errNoSpoke = errors.New("eidos: the plan's backend does not implement a type spoke")

// errUntyped is the reason of a refusal of a parameter or a return of an rpc
// without a type.
var errUntyped = errors.New("eidos: a parameter or a return of the rpc has no type")

// translate is the translation of [Emitter.Type] and [Emitter.Supertype].
// It reports the refusal of [Emitter.typeOf] under [RefusedType] at the
// position of owner.
func (e *Emitter) translate(owner symbol.Identity, ref *node.TypeRef, supertype bool) (*emit.TypeRef, bool) {
	if ref == nil {
		return nil, true
	}
	spelled, err := e.typeOf(owner, ref, supertype)
	if err != nil {
		e.rs.reportf(e.m.seq, RefusedType, diag.SeverityError, e.rs.positionOf(owner),
			"the %s target cannot spell the %s type %s: %v", e.rs.target, owner.Lang, ref.Spelling, err)
		return nil, false
	}
	return spelled, true
}

// typeOf translates the reference ref of the declaration owner, as
// [Emitter.Type] states it, and returns the reason of a refusal. supertype
// selects the supertype spelling of the backend, where the backend
// implements [plugin.SupertypeSpeller]. ref is not nil.
func (e *Emitter) typeOf(owner symbol.Identity, ref *node.TypeRef, supertype bool) (*emit.TypeRef, error) {
	rs, m := e.rs, e.m
	switch {
	case owner.Lang == symbol.Lang(rs.target):
		return rules.EmitRef(ref), nil
	case rs.types == nil:
		return nil, errNoSpoke
	}
	b := e.rulesOf(owner)
	var shape rules.TypeShape
	if f, field := fieldOf(b.View(), owner); field && f.Type == ref {
		shape = b.FieldTypeOf(f)
	} else {
		shape = b.TypeOf(ref)
	}
	prev := m.translating
	m.translating = owner
	p := rs.policy.Overridden(m.overrides())
	var (
		spelled *emit.TypeRef
		err     error
	)
	if s, spells := rs.types.(plugin.SupertypeSpeller); supertype && spells {
		spelled, err = s.SpellSupertype(shape, p)
	} else {
		spelled, err = rs.types.SpellType(shape, p)
	}
	m.translating = prev
	return spelled, err
}

// rulesOf returns the rules of the language of owner, bound to the view of
// the invocation: the rules of the subject's language, or the rules of
// another language of the composition.
func (e *Emitter) rulesOf(owner symbol.Identity) rules.Bound {
	if owner.Lang == e.m.subject.Lang {
		return e.m.Rules()
	}
	return e.m.RulesFor(owner.Lang)
}

// serviceOf returns the identity of the interface that declares the rpc id:
// the last name of the owner chain of id, under the rest of the chain.
func serviceOf(id symbol.Identity) symbol.Identity {
	owner, name := "", id.Owner
	if at := strings.LastIndex(id.Owner, symbol.OwnerSep); at >= 0 {
		owner, name = id.Owner[:at], id.Owner[at+len(symbol.OwnerSep):]
	}
	return symbol.Identity{Lang: id.Lang, Package: id.Package, Owner: owner, Name: name, Kind: symbol.KindInterface}
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
	rs       *runState
	seq      int
	touch    int
	internal bool // [Out.Internal] marked the handle
}

// Internal marks the declarations that the handle places from now on as
// tests of their package from inside, such as the check of an unexported
// declaration in a family that states [plugin.Output.External]. The flush
// lists their origins in [plugin.Unit.Inside]. The Go target writes them
// into the internal test file of the family, which declares the package
// under test, as store_check_internal_test.go beside store_check_test.go.
// A family that is not External tests from inside already, and the mark
// does not change its units. Internal returns o, and it allocates nothing.
func (o *Out) Internal() *Out {
	o.internal = true
	return o
}

// Append places emit declarations under the subject of the handle. A
// call without declarations does nothing and records nothing. The
// placements apply when the rules of the phase call have run. The flush
// orders the declarations of a unit by the source position of their
// origin: the file, then the line, then the column. An origin without a
// position orders after every origin with one. Declarations of one
// position order by origin identity, then by the source order of the
// gating instance under a repeatable directive, then by canonical match
// order and the order of the appends. The output order is therefore the
// order of the source and never the order of the schedule. The call
// copies the declarations, so the caller may reuse the slice that it
// passed. It buffers them in the pooled storage of the phase call and
// allocates nothing once the state of the call is warm. The flush
// allocates the lists of declarations and origins of the unit. A nil
// declaration, or a nil pointer of a declaration kind, fails the phase
// call at the flush with the error of [plugin.Emit.Add].
func (o *Out) Append(decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	o.rs.fx.place(o.seq, o.touch, decls, nil, o.internal)
}

// Record places emit declarations under the subject of the handle as
// [Out.Append] does, and attaches data to each of them, for a family that
// claims its files through [plugin.Output.Template]. The file template of
// the family reads each declaration as a record, with the data of the
// call that placed it. Two records of one origin each keep the data of
// their own call. A declaration that two calls place keeps the data of
// the first call in canonical match order. A call without declarations
// does nothing and records nothing. It buffers as Append does, and the
// flush allocates the map of data of the unit.
func (o *Out) Record(data any, decls ...symbol.Symbol) {
	if len(decls) == 0 {
		return
	}
	o.rs.fx.place(o.seq, o.touch, decls, data, o.internal)
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
// that an [Out.Record] call attached to it. pos is the position of the
// origin in the graph, and zero for an origin without one. The data is
// nil for [Out.Append]. internal reports that the handle of the call was
// marked through [Out.Internal].
type placed struct {
	pos      position.Pos
	origin   symbol.Identity
	instance int
	seq      int
	decl     symbol.Symbol
	data     any
	internal bool
}

// originsOf returns the distinct nonzero origins of places, sorted by
// identity. The flush orders the places of one origin next to each other,
// so originsOf counts the runs of origins first, and the slice has its
// exact size. The sort allocates nothing.
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
	slices.SortFunc(out, symbol.Identity.Compare)
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
// nothing. A unit records in [plugin.Unit.Subjects] the subject of each
// declaration whose origin is another declaration. Where two placements of
// one origin have two subjects, the first placement in the order of the
// unit applies.
//
// # Allocation contract
//
// The keys sort in the buffer of the call. Each unit allocates its
// declarations and, where they have origins, its origins. A unit with a
// declaration whose origin is not its subject also allocates the map of
// subjects. The store keeps all three.
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
			if unplaced := a.pos.IsZero(); unplaced != b.pos.IsZero() {
				if unplaced {
					return 1
				}
				return -1
			}
			if c := a.pos.Compare(b.pos); c != 0 {
				return c
			}
			if c := a.origin.Compare(b.origin); c != 0 {
				return c
			}
			if c := cmp.Compare(a.instance, b.instance); c != 0 {
				return c
			}
			return cmp.Compare(a.seq, b.seq)
		})
		decls := make([]symbol.Symbol, 0, len(acc.places))
		var (
			data     map[symbol.Symbol]any
			subjects map[symbol.Identity]symbol.Identity
			inside   []symbol.Identity
		)
		for _, p := range acc.places {
			decls = append(decls, p.decl)
			// A nil pointer of a declaration kind has no origin, and Add
			// refuses the unit that contains it.
			if v := reflect.ValueOf(p.decl); v.Kind() == reflect.Pointer && v.IsNil() {
				continue
			}
			origin, _ := emit.OriginOf(p.decl)
			if _, placed := subjects[origin]; !placed && !origin.IsZero() && !p.origin.IsZero() && origin != p.origin {
				if subjects == nil {
					subjects = map[symbol.Identity]symbol.Identity{}
				}
				subjects[origin] = p.origin
			}
			if p.internal && acc.out.External && !origin.IsZero() {
				inside = append(inside, origin)
			}
			if p.data == nil {
				continue
			}
			if _, attached := data[p.decl]; !attached {
				if data == nil {
					data = map[symbol.Symbol]any{}
				}
				data[p.decl] = p.data
			}
		}
		origins := originsOf(acc.places)
		slices.SortFunc(inside, symbol.Identity.Compare)
		err := into.Add(plugin.Unit{
			Plugin:   c.plugin,
			Tag:      string(k.tag),
			Per:      k.per,
			Word:     acc.out.Word,
			Key:      acc.key,
			Pkg:      acc.pkg,
			Decls:    decls,
			Origins:  origins,
			Subjects: subjects,
			Template: acc.out.Template,
			Data:     data,
			Inside:   slices.Compact(inside),
		})
		if err != nil {
			return fmt.Errorf("eidos: %s flush: %w", c.plugin, err)
		}
	}
	return nil
}
