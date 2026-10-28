// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// RefusedConstruct is the code of a lowering hook that refuses a
// declaration. The target declares no idiom for the construct, and the
// settle withholds the declaration from the render.
var RefusedConstruct = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 29, Meaning: "the target declares no idiom for a construct",
})

// RefusedName is the code of a respell hook that refuses a declared
// name. The target cannot spell the name at its visibility. The settle
// withholds the declaration from the render, because a render under
// another access would misstate the declaration.
var RefusedName = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 30, Meaning: "the target cannot spell a declared name at its visibility",
})

// CollidingNames is the code of two declarations in one scope that
// settle to one name. The settle reports two members of one host, which
// keep their emitted names, and the references to either follow the
// emitted names. The layout reports two file-level declarations of one
// routed scope, which keep their settled names. The finding names the
// declarations to fix, and the Error fails the plan.
var CollidingNames = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 31, Meaning: "two declarations settle to one name in one scope",
})

// AmbiguousReference is the code of a bare reference that matches
// declarations with different settled names. The settle leaves the
// reference as written.
var AmbiguousReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 32, Meaning: "a bare reference matches declarations whose settled names diverge",
})

// VerbatimParams is the code of a verbatim body that pins the parameter
// and result names of its callable. The text of the body uses the
// emitted names, so the signature keeps them, and the respell would have
// changed one of them.
var VerbatimParams = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 33, Meaning: "a verbatim body pins parameter names one respell would have changed",
})

// Lower rewrites one declaration into the construct shapes of the
// target. Every returned declaration has the origin of the input. The
// output states the lowered fact in the shape of the target and not in
// the shape of the model, so a second settle changes nothing. An error
// means that the target refuses the construct.
//
// A nil list without an error keeps the declaration unchanged, so the
// common pass-through allocates nothing. A hook that reshaped the
// declaration in place returns nil as well, because the store already
// contains the declaration.
type Lower func(symbol.Symbol) ([]symbol.Symbol, error)

// Lowerer is the role of a backend that reshapes the constructs of its
// target. Every [Backend] implements it. A backend whose target keeps
// every construct returns nil and no error, which keeps the declaration.
type Lowerer interface {
	Lower(symbol.Symbol) ([]symbol.Symbol, error)
}

// Respell spells one declared name in the convention of the target.
// host is the kind of the declaration that declares a member, and
// [symbol.KindInvalid] at file level. A kind without visibility passes
// the zero visibility. An error means that the target cannot spell the
// name at that visibility.
type Respell func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error)

// Respeller is the role of a backend that respells declared names in the
// convention of its target. Every [Backend] implements it. A backend
// whose target keeps every name returns the name unchanged.
type Respeller interface {
	Respell(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error)
}

// NameKeeper is the role of a backend whose target keeps a declared name
// of its own language: every name that a source of the language declares is
// a name of the target, as in protobuf. The settle respells no name that an
// emitted declaration carries from an origin of the target's language. A
// target whose convention also governs names of its own language, as Go
// writes an exported name in upper case, does not keep them.
type NameKeeper interface {
	// KeepsOwnNames reports whether the target keeps a declared name of
	// its own language.
	KeepsOwnNames() bool
}

// Member is what the convention of a target reads of a declared member
// beside its name: the kind of its host, its own kind, its visibility, its
// level and its mutability. A method has the zero mutability.
type Member struct {
	Host       symbol.Kind
	Kind       symbol.Kind
	Visibility symbol.Visibility
	Level      symbol.Level
	Mutability symbol.Mutability
}

// MemberRespell spells the name of one member in the convention of the
// target, as [Respell] spells a declared name, by the level and the
// mutability of the member too. An error means that the target cannot
// spell the name.
type MemberRespell func(m Member, name string) (string, error)

// MemberRespeller is the role of a backend whose convention spells a member
// by its level and its mutability, as Python keeps the capitals of a
// constant of a class. The settle respells each field and each method of a
// declaration through it in place of [Respeller.Respell], and the lowering
// of a target names a member through it in the same way.
type MemberRespeller interface {
	RespellMember(m Member, name string) (string, error)
}

// Settle takes the store of one plan through the hooks of the backend,
// once, after the schedule and before the render. Every declaration
// lowers into the construct shapes of the target, and every declared
// name respells into the convention of the target. The references follow
// their declarations, and the per-kind index rebuilds over the
// declarations that remain. Settle marks the store settled, and a second
// call on a settled store changes nothing.
//
// A declared name takes an override where facts contains one. An
// override is a value of the name key of the target, [Target.NameKey],
// such as golang.name, on the origin of the declaration. A directive or
// a higher authority writes it. The settle reads each override through
// a point read of the fact of the origin. The override replaces the
// spelling of the respell hook for the declaration that renders the
// origin. The hook spells the emitted name of that declaration in the
// same way as the name of the origin. A file-level declaration renders
// a nested origin under the flat name of the origin,
// [symbol.Identity.FlatName]. Another declaration with the same origin,
// such as a mock of an interface, keeps the spelling of the hook. The
// settle withholds a name that the hook refuses, also where the origin
// has an override. The stamp of a plugin on the key is not an override,
// because the hook spells the convention of the target. The settle does
// not apply an override where the fact store is nil, or where the
// composition did not register the name key.
//
// A field and a method respell through the [MemberRespeller] role of a
// backend that has it, with the level and the mutability of the member.
//
// A field that is a key of a wire format, [emit.Field.Wire], keeps the
// name that the generator wrote. The settle calls neither the hook nor the
// override for it, because a consumer matches the key by string and a
// renamed key breaks the consumer. Such a field joins the check of the
// members of its host as any field does.
//
// The settle respells the records of a family that claims its files.
// The units of such a family have a [Unit.Template], and the template
// reads the names of the target. A record that the lowering replaces
// passes its data in [Unit.Data] to each declaration that replaces it.
// The settle does not check a record for a collision, and it does not
// rewrite a reference to the name of a record. The layout qualifies a
// reference with a target into the file of a record.
//
// The settle records whether a declaration that renders has an emitted
// reference, or a type reference to a declaration of another language
// than the target of the backend. [Emit.Translates] reports the result.
//
// The settle reports its findings to the sink under the name of the
// backend. A refused construct or name withholds its declaration.
// Colliding members of one host keep their emitted spellings, and the
// settle leaves an ambiguous reference as written. The settle does not
// check the file-level names, which the layout checks in their routed
// packages. A returned error is a defect of the hooks of the backend, a
// lowering that drops the origin of its input. The error fails the whole
// plan, not one declaration.
//
// A nil backend marks the store settled and changes nothing in it.
//
// # Allocation contract
//
// Settle rebuilds the store's per-kind index, one list per unit and
// kind. The lowering allocates a new list for each unit in which the
// hook refuses or replaces a declaration, and nothing for a hook that
// keeps every declaration. A unit in which the hook replaces a record
// with data also allocates a copy of its data. The respell adds the plan
// of every declared name and the tables that the references follow, each
// sized once to the store. A nil backend settles the store without
// allocating.
func Settle(e *Emit, b Backend, facts *meta.Facts, sink *diag.Sink) error {
	return settle(e, b, facts, sink, nil, nil)
}

// SettleWith settles a store that contains part of a plan's units: the
// units of the files that a warm run generates again. It settles as
// [Settle] does, and resolves each reference that the store does not
// settle against others, the names of the plan's other files. A bare
// reference that both declare under different settled names is
// ambiguous, as it is where the store declares the plan whole. Members
// collide only with the members of their host, which the store contains.
// Settle is SettleWith with no others.
//
// SettleWith returns what each unit read. It lists every entry that the
// unit's references looked up, whether a file declares the entry or not.
// It also lists the name override of every origin that the unit's
// declarations render. It leaves out a lookup that the unit's own
// declarations settle with no entry in others, because the unit
// generates again whenever its own names change. A nil backend rewrites
// no reference, and SettleWith then lists the lookups of the bare type
// references alone. The errors are the errors of Settle.
//
// # Allocation contract
//
// SettleWith allocates what Settle allocates and its read log, which is
// all it adds for a store whose units read nothing. The first read adds
// the set that lists each unit's read once. The lists of reads grow by
// doubling. The settle allocates each unit's place in [Emit.Units] order
// once, and others adds what its methods allocate.
func SettleWith(e *Emit, b Backend, facts *meta.Facts, sink *diag.Sink, others Names) (Settled, error) {
	reads := &readLog{e: e}
	err := settle(e, b, facts, sink, others, reads)
	return reads.out, err
}

// settle is the settle that Settle and SettleWith share. others is nil
// for a store that contains the whole plan, and reads is nil where the
// caller does not keep the reads.
func settle(e *Emit, b Backend, facts *meta.Facts, sink *diag.Sink, others Names, reads *readLog) error {
	if e == nil || e.settled {
		return nil
	}
	// The lowering hooks rewrite the declarations of the store, so the
	// seal lifts before the first of them runs.
	e.sealed = false
	if b == nil {
		reads.typeRefs(e)
		reads.finish()
		e.settled = true
		return nil
	}
	by := b.Name()
	if err := lowerAll(e, b, by, sink); err != nil {
		// A store abandoned mid-lowering is not settled, so the flag
		// remains down and Settled reports false.
		return err
	}
	respellAll(e, b, b.Target(), overridesFor(facts, b.Target()), others, by, sink, reads)
	reads.finish()
	e.reindex()
	e.settled = true
	return nil
}

// overrides reads the name overrides of one target. An override is a
// name on an origin at directive authority or above. The reader reads
// each origin with one point read of the claim that ranks first. A stamp
// of the name at plugin authority therefore costs the settle nothing,
// although a lowering entry writes one on every declaration that a plan
// of the target renders. A store that a run restores does not restore
// the bag of an origin without the name key. The zero value reads no
// override, and the settle uses it for a nil fact store and for a name
// key that the composition did not register.
type overrides struct {
	facts *meta.Facts
	key   meta.Key[string]
	name  meta.KeyName
}

// overridesFor returns the reader of a target's name overrides in one
// fact store. It allocates the spelling of the target's name key.
func overridesFor(facts *meta.Facts, t Target) overrides {
	if facts == nil {
		return overrides{}
	}
	name := t.NameKey()
	key, registered := meta.Lookup[string](facts.Registry(), name)
	if !registered {
		return overrides{}
	}
	return overrides{facts: facts, key: key, name: name}
}

// on returns the override on one origin. The override is the value of
// the claim that ranks first, where that claim has directive authority
// or above. on reports false for a reader without a store, for the zero
// identity, for an origin without a claim at that authority and for an
// empty value. No declaration renders the zero identity. It allocates
// nothing.
func (o overrides) on(id symbol.Identity) (string, bool) {
	if o.facts == nil || id.IsZero() {
		return "", false
	}
	name, written := meta.GetAtLeast(o.facts, id, o.key, meta.AuthorityDirective)
	if !written || name == "" {
		return "", false
	}
	return name, true
}

// unitPos returns the position of a settle finding. An emit declaration
// has no source position, so the position is the routing key of the
// unit, the path of its output.
func unitPos(u *Unit) position.Pos { return position.Pos{File: u.Key} }

// lowerAll rewrites every declaration through the lowering hook. A
// refusal withholds the declaration under a positioned finding, and
// an output with another origin is a defect and returns. A record that
// the hook replaces passes its data to each declaration that replaces it.
// The data moves in a copy of the map of the unit, because the caller
// can hold the map of the unit beside the store. A unit whose
// declarations all lower to nil keeps its list, so a lowering that keeps
// every declaration allocates nothing. The first refusal or replacement
// in a unit allocates the unit's new list, with the declarations before
// it, and the first replacement of a record with data allocates the copy
// of the map.
func lowerAll(e *Emit, l Lowerer, by diag.Origin, sink *diag.Sink) error {
	for i := range e.units {
		u := &e.units[i]
		var (
			lowered []symbol.Symbol
			data    map[symbol.Symbol]any
		)
		for j, d := range u.Decls {
			out, err := l.Lower(d)
			if err == nil && out == nil {
				if lowered != nil {
					lowered = append(lowered, d)
				}
				continue
			}
			if lowered == nil {
				lowered = make([]symbol.Symbol, j, len(u.Decls)+len(out))
				copy(lowered, u.Decls[:j])
			}
			if err != nil {
				sink.Errorf(RefusedConstruct, unitPos(u), by, "%v", err)
				continue
			}
			origin, _ := emit.OriginOf(d)
			for _, o := range out {
				produced, _ := emit.OriginOf(o)
				if produced != origin {
					return fmt.Errorf(
						"plugin: %s lowers a declaration of origin %s into one of origin %s: "+
							"every output has the input's origin",
						by, origin, produced,
					)
				}
			}
			lowered = append(lowered, out...)
			if v, held := u.Data[d]; held {
				if data == nil {
					data = maps.Clone(u.Data)
				}
				delete(data, d)
				for _, o := range out {
					data[o] = v
				}
			}
		}
		if lowered != nil {
			u.Decls = lowered
		}
		if data != nil {
			u.Data = data
		}
	}
	return nil
}

// planned is the visit of one name. It records what the walk met, what
// the hook returned and what the apply pass writes. grouped marks a
// member that a collision group contains already.
type planned struct {
	host    symbol.Symbol
	at      position.Pos
	emitted string
	settled string
	final   string
	err     error
	kind    symbol.Kind
	grouped bool
}

// respellAll settles every declared name and rewrites the references to
// them. Every visit plans first, and the collisions of members resolve per
// host. The plans then apply, and the references follow the names of the
// store and then the names of others. The withheld declarations leave
// their units last, because every earlier pass addresses a plan by the
// index of its declaration. reads records the override reads and the
// lookups of each unit. The reference pass records whether a reference
// is emitted or has a target of another language than the target t.
func respellAll(
	e *Emit, r Respeller, t Target, over overrides, others Names, by diag.Origin, sink *diag.Sink, reads *readLog,
) {
	var own symbol.Lang
	if keeper, keeps := r.(NameKeeper); keeps && keeper.KeepsOwnNames() {
		own = symbol.Lang(t)
	}
	plans := planNames(e, r, own, over, reads)
	table, byOrigin := resolveTop(e, plans)
	resolveMembers(e, plans, by, sink)
	applyNames(e, plans, by, sink)
	res := &resolver{
		table: table, byOrigin: byOrigin, others: others, by: by, sink: sink, reads: reads, lang: symbol.Lang(t),
	}
	res.rewriteRefs(e, plans)
	dropWithheld(e)
}

// dropWithheld removes the declarations that applyNames withheld.
// applyNames marks each of them nil in place, so the reference pass
// finds the plan of each remaining declaration at its original index.
func dropWithheld(e *Emit) {
	for i := range e.units {
		u := &e.units[i]
		u.Decls = slices.DeleteFunc(u.Decls, func(d symbol.Symbol) bool { return d == nil })
	}
}

// span is the slice of the plan arena of one declaration.
type span struct{ lo, hi int }

// plan contains the visits of one settle. Every pass points into one
// arena with a span for each declaration, so the plan of the store costs
// one growing slice and not one slice per declaration.
type plan struct {
	spans map[[2]int]span
	all   []planned
}

// of returns one declaration's visits. The arena never grows after
// planning, so a pointer into the returned slice remains valid.
func (p *plan) of(i, j int) []planned {
	s := p.spans[[2]int{i, j}]
	return p.all[s.lo:s.hi]
}

// planNames runs the recording walk over every declaration. Each visit
// calls the hook, takes the override of the declaration that renders its
// origin in place of the spelling of the hook, and stores the result by
// value. A field and a method spell through the [MemberRespeller] role
// where the backend has it. The visit of a wire field keeps the emitted
// name and reads no override. So does the visit of a declaration that
// carries the name of its origin where the origin is a declaration of
// lang: the language of a target that is a [NameKeeper], and empty for
// every other target. Nothing writes yet. The apply walk replays the visits in their
// order. reads records the override that each unit read for each origin
// that its declarations render.
func planNames(e *Emit, r Respeller, lang symbol.Lang, over overrides, reads *readLog) *plan {
	total := 0
	for i := range e.units {
		total += len(e.units[i].Decls)
	}
	plans := &plan{
		spans: make(map[[2]int]span, total),
		all:   make([]planned, 0, total*2),
	}
	members, _ := r.(MemberRespeller)
	var at position.Pos
	// One recording callback serves every walk. Its per-visit state
	// is declared beside it, so planning allocates the arena's
	// growth and nothing else. The callback returns no error. It
	// keeps a hook fault in the fault's entry, and the apply pass
	// judges the fault per declaration.
	record := func(
		host, carrier symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
	) (string, error) {
		p := planned{host: host, kind: kind, at: at, emitted: name}
		if origin, _ := emit.OriginOf(carrier); lang != "" && origin.Lang == lang && origin.Name == name {
			p.settled, p.final = name, name
			plans.all = append(plans.all, p)
			return name, nil
		}
		m := Member{Host: hostKind(host), Kind: kind, Visibility: v}
		member := false
		switch c := carrier.(type) {
		case *emit.Field:
			if c.Wire {
				p.settled, p.final = name, name
				plans.all = append(plans.all, p)
				return name, nil
			}
			m.Level, m.Mutability, member = c.Level, c.Mutability, true
		case *emit.Method:
			m.Level, member = c.Level, true
		}
		p.settled, p.err = respellName(r, members, m, member, name)
		// A carrier without an origin reads the zero identity, which
		// takes no override.
		origin, _ := emit.OriginOf(carrier)
		if over.facts != nil && !origin.IsZero() {
			reads.fact(origin, over.name)
		}
		if override, renders := overrideOf(r, members, over, host, origin, m, member, p.settled); renders {
			p.settled = override
		}
		p.final = p.settled
		if p.err != nil {
			p.final = name
		}
		plans.all = append(plans.all, p)
		return name, nil
	}
	for i := range e.units {
		// An emit declaration has no source position, so every
		// visit in a unit is positioned at the unit.
		at = unitPos(&e.units[i])
		reads.enter(i)
		for j, d := range e.units[i].Decls {
			lo := len(plans.all)
			_ = emit.RespellNames(d, record)
			plans.spans[[2]int{i, j}] = span{lo: lo, hi: len(plans.all)}
		}
	}
	return plans
}

// overrideOf returns the override of a carrier of origin that renders
// the origin. The override is on the origin. The carrier renders the
// origin where the hook spells the name of the origin as it spelled the
// name of the carrier, settled. A file-level carrier renders a nested
// origin under the flat name of the origin, [symbol.Identity.FlatName],
// because a target without nested types declares the origin in the
// file. The hook spells the name of the origin as [respellName] spells the
// name of the carrier m, a member where member is true. overrideOf reports
// false for an origin without an override, for a name that the hook refuses
// and for a name that the hook spells apart from the name of the origin. It
// allocates the flat name of a nested origin that has an override, and
// nothing otherwise.
func overrideOf(
	r Respeller, members MemberRespeller, over overrides, host symbol.Symbol, origin symbol.Identity,
	m Member, member bool, settled string,
) (string, bool) {
	override, written := over.on(origin)
	if !written {
		return "", false
	}
	name := origin.Name
	if host == nil {
		name = origin.FlatName()
	}
	own, err := respellName(r, members, m, member, name)
	if err != nil || own != settled {
		return "", false
	}
	return override, true
}

// respellName spells a declared name through the hook of r: a member, which
// member reports, through members, the [MemberRespeller] role of r, where r
// has the role, and every other name through r.
func respellName(r Respeller, members MemberRespeller, m Member, member bool, name string) (string, error) {
	if member && members != nil {
		return members.RespellMember(m, name)
	}
	return r.Respell(m.Host, m.Kind, m.Visibility, name)
}

// hostKind returns the kind of a host for the hook, and the invalid kind
// at file level.
func hostKind(host symbol.Symbol) symbol.Kind {
	if host == nil {
		return symbol.KindInvalid
	}
	return host.Kind()
}

// tableEntry is one scope's settled spelling for an emitted name, and
// the arrival index of the unit that declares the name, -1 where more
// than one unit does.
type tableEntry struct {
	settled   string
	ambiguous bool
	unit      int
}

// originEntry is the settled spelling of a declaration derived from an
// origin, and the arrival index of the unit that declares it, -1 where
// more than one unit declares the origin's emitted name.
type originEntry struct {
	settled string
	unit    int
}

// pkgName keys one emitted spelling in one package's reference
// table.
type pkgName struct {
	pkg     string
	emitted string
}

// originName keys one emitted spelling under one origin identity.
type originName struct {
	id      symbol.Identity
	emitted string
}

// resolveTop builds the tables of the file-level names that the
// references follow. table records the settled name of each emitted name
// of each package, and byOrigin records the same keyed by the identity of
// the origin, for resolved references. Both record the unit that declares
// each name. The records of a claimed family join no table, because the
// settle does not rewrite a reference to a record. It does not check
// whether two file-level names collide. The layout checks them in the
// packages of their routed files, which the settle does not know. It
// allocates the two tables, each sized once to the store.
func resolveTop(e *Emit, plans *plan) (map[pkgName]tableEntry, map[originName]originEntry) {
	table := make(map[pkgName]tableEntry, len(plans.spans))
	byOrigin := make(map[originName]originEntry, len(plans.spans))
	for i := range e.units {
		if e.units[i].Template != "" {
			continue
		}
		pkg := e.units[i].Pkg.Package
		for j, d := range e.units[i].Decls {
			origin, known := emit.OriginOf(d)
			known = known && !origin.IsZero()
			list := plans.of(i, j)
			for k := range list {
				p := &list[k]
				if p.host != nil || p.err != nil {
					continue
				}
				// A method's name never spells a bare reference, and
				// neither does the name of a field that another type
				// receives: a type or a callable does, a member of one
				// does not, so the table records everything else.
				if p.kind != symbol.KindMethod && emit.ReceivesOf(d) == nil {
					at := pkgName{pkg: pkg, emitted: p.emitted}
					if entry, taken := table[at]; taken {
						entry.ambiguous = entry.ambiguous || entry.settled != p.final
						if entry.unit != i {
							entry.unit = -1
						}
						table[at] = entry
					} else {
						table[at] = tableEntry{settled: p.final, unit: i}
					}
				}
				if known {
					at := originName{id: origin, emitted: p.emitted}
					entry := originEntry{settled: p.final, unit: i}
					if prior, taken := byOrigin[at]; taken && prior.unit != i {
						entry.unit = -1
					}
					byOrigin[at] = entry
				}
			}
		}
	}
	return table, byOrigin
}

// resolveMembers reverts member collisions. Members of one host whose
// different emitted names settle to one name keep their emitted
// spellings under one finding. Members with one emitted name are
// overloads, or duplicates that the lowering of the language refuses,
// and the respell did not join them. The members of a record declare
// nothing, so the scan leaves out the units of a claimed family. Plans
// are short, so the scan compares pairs and allocates only where a
// collision exists.
func resolveMembers(
	e *Emit, plans *plan, by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
		if e.units[i].Template != "" {
			continue
		}
		for j := range e.units[i].Decls {
			list := plans.of(i, j)
			for a := range list {
				pa := &list[a]
				if pa.host == nil || pa.err != nil || pa.grouped || !collides(list, a) {
					continue
				}
				settled := pa.final
				var names []string
				for b := a; b < len(list); b++ {
					pb := &list[b]
					if pb.host != pa.host || pb.err != nil || pb.grouped ||
						pb.final != settled {

						continue
					}
					names = append(names, pb.emitted)
					pb.final, pb.grouped = pb.emitted, true
				}
				slices.Sort(names)
				sink.Errorf(CollidingNames, pa.at, by,
					"%s settle to %q in one host, and every one keeps its emitted name",
					strings.Join(names, " and "), settled)
			}
		}
	}
}

// collides reports whether a later member of the host of list[a]
// settles to the name of list[a] from a different emitted name.
func collides(list []planned, a int) bool {
	pa := &list[a]
	for b := a + 1; b < len(list); b++ {
		pb := &list[b]
		if pb.host == pa.host && pb.err == nil && !pb.grouped &&
			pb.final == pa.final && pb.emitted != pa.emitted {

			return true
		}
	}
	return false
}

// applyNames replays every plan over its declaration and writes the
// final spellings back. It records the emitted name of each declaration
// whose spelling changed, because an export uses that name as its key.
// applyNames withholds a declaration whose plan contains a hook refusal
// whole, under a positioned finding, because a half-respelt render
// would misstate the declaration. It sets the entry of the declaration
// to nil, and [dropWithheld] removes the entry after the rewrite of the
// references. A verbatim body pins the parameter and result names of its
// callable, under a finding where one of them would have changed.
func applyNames(
	e *Emit, plans *plan, by diag.Origin, sink *diag.Sink,
) {
	var list []planned
	var at int
	var warned map[symbol.Symbol]bool
	// One applying callback serves every walk, replaying the plan
	// positionally; the warning set exists only where a verbatim
	// pin met a rename.
	apply := func(
		host, carrier symbol.Symbol, kind symbol.Kind, _ symbol.Visibility, name string,
	) (string, error) {
		if at >= len(list) {
			return name, nil
		}
		p := &list[at]
		at++
		if pinnedByVerbatim(host, kind) {
			if p.final != p.emitted && !warned[host] {
				if warned == nil {
					warned = map[symbol.Symbol]bool{}
				}
				warned[host] = true
			}
			p.final = p.emitted
		}
		if p.final != p.emitted {
			e.respelled(carrier, p.emitted)
		}
		return p.final, nil
	}
	for i := range e.units {
		u := &e.units[i]
		for j, d := range u.Decls {
			list = plans.of(i, j)
			if refused := firstErr(list); refused != nil {
				sink.Errorf(RefusedName, unitPos(u), by, "%v", refused.err)
				u.Decls[j] = nil
				continue
			}
			at, warned = 0, nil
			_ = emit.RespellNames(d, apply)
			if len(warned) > 0 {
				// Sorted for one finding order; the collect runs only
				// where a pin met a rename, so the clean path allocates
				// nothing here.
				hosts := slices.SortedFunc(maps.Keys(warned), func(a, b symbol.Symbol) int {
					return strings.Compare(pinnedName(a), pinnedName(b))
				})
				for _, host := range hosts {
					sink.Errorf(VerbatimParams, unitPos(u), by,
						"a verbatim body pins its parameter names, and one on %s "+
							"would have respelled", pinnedName(host))
				}
			}
		}
	}
}

// pinnedName returns the name of the callable that a verbatim pin
// reports.
func pinnedName(host symbol.Symbol) string {
	switch c := host.(type) {
	case *emit.Function:
		return c.Name
	case *emit.Method:
		return c.Name
	default:
		return "a callable"
	}
}

// firstErr returns the first refused visit in a plan, nil when
// every name spelled.
func firstErr(list []planned) *planned {
	for k := range list {
		if list[k].err != nil {
			return &list[k]
		}
	}
	return nil
}

// pinnedByVerbatim reports whether a name is a parameter or a result of
// a callable whose body is literal text.
func pinnedByVerbatim(host symbol.Symbol, kind symbol.Kind) bool {
	if kind != symbol.KindParam && kind != symbol.KindReturn {
		return false
	}
	switch c := host.(type) {
	case *emit.Function:
		return c.Body.Verbatim != ""
	case *emit.Method:
		return c.Body.Verbatim != ""
	default:
		return false
	}
}

// resolver follows one store's references to the names that the
// settle left the store's declarations with, and then to the names of
// the plan's other files. It records each lookup of the unit under
// resolution into reads, except a lookup that the unit's own
// declarations settle with nothing in the other files.
type resolver struct {
	table    map[pkgName]tableEntry
	byOrigin map[originName]originEntry
	others   Names
	by       diag.Origin
	sink     *diag.Sink
	reads    *readLog
	// unit is the arrival index of the unit under resolution, pkg the
	// unit's package path, and at the position of its findings.
	unit int
	pkg  string
	at   position.Pos
	// lang is the language of the backend's target, which tells a
	// translated reference apart.
	lang symbol.Lang
}

// rewriteRefs rewrites the references of the store to the settled
// names. A resolved reference follows its origin where its spelling is
// the emitted bare name of the referent. A bare reference follows the
// table of its own package. The spelling of a structural reference
// follows the names beneath it. The structured names of a body follow
// the locals first, then the package. The pass leaves a composite
// spelling without elements, a verbatim body and the text of a template
// as written. It marks the store where a reference of a declaration that
// renders is an emitted reference, or has a target of another language
// than the language of the resolver.
func (r *resolver) rewriteRefs(e *Emit, plans *plan) {
	for i := range e.units {
		u := &e.units[i]
		r.unit, r.pkg, r.at = i, u.Pkg.Package, unitPos(u)
		r.reads.enter(i)
		for j, d := range u.Decls {
			if d == nil {
				continue
			}
			renames := paramNames(plans.of(i, j))
			for s := range emit.All(d) {
				switch t := s.(type) {
				case *emit.TypeRef:
					if t.Emitted || (!t.Target.IsZero() && t.Target.Lang != r.lang) {
						e.translates = true
					}
					r.rewriteRef(t)
				case *emit.Function:
					r.rewriteBody(&t.Body, maps.Clone(renames[t]))
				case *emit.Method:
					r.rewriteBody(&t.Body, maps.Clone(renames[t]))
				}
			}
		}
	}
}

// paramNames collects the parameter and result names of each callable
// from its plan, from the emitted spelling to the final one, so the
// references of its body follow them. Every such name is in the result,
// renamed or not, because a parameter shadows the package scope either
// way. A plan without a parameter or a result returns nil.
func paramNames(list []planned) map[symbol.Symbol]map[string]string {
	var out map[symbol.Symbol]map[string]string
	for k := range list {
		p := &list[k]
		if p.host == nil ||
			(p.kind != symbol.KindParam && p.kind != symbol.KindReturn) {

			continue
		}
		if out == nil {
			out = map[symbol.Symbol]map[string]string{}
		}
		m := out[p.host]
		if m == nil {
			m = map[string]string{}
			out[p.host] = m
		}
		m[p.emitted] = p.final
	}
	return out
}

// rewriteRef follows one type reference: a named one to the spelling
// [resolver.spelling] returns, and a structural one through the names
// beneath it. An ambiguous match reports and is left as written.
func (r *resolver) rewriteRef(t *emit.TypeRef) {
	if t.Form != symbol.FormNamed {
		r.followElements(t, t.Elems)
		return
	}
	settled, ambiguous := r.spelling(t)
	if ambiguous {
		r.sink.Errorf(AmbiguousReference, r.at, r.by,
			"%q matches declarations whose settled names diverge, and the "+
				"reference is left as written", t.Spelling)
		return
	}
	t.Spelling = settled
}

// spelling returns the spelling a named reference settles to: by
// origin where the reference is resolved and its spelling is the
// referent's emitted bare name, by the package's table where it is
// bare, and as written otherwise. A reference without a target that
// names another package is qualified, so no declaration of this
// package binds it, and it is left as written. A bound reference keeps
// the name that its binding gives it, because the plan does not emit its
// referent. It reports true for a spelling the table matches to
// declarations whose settled names diverge, which is left as written too.
// It reads two maps and allocates nothing but what others and the read
// log allocate.
func (r *resolver) spelling(t *emit.TypeRef) (string, bool) {
	if t.Bound {
		return t.Spelling, false
	}
	if !t.Target.IsZero() {
		if settled, match := r.ofOrigin(t.Target, t.Spelling); match {
			return settled, false
		}
		return t.Spelling, false
	}
	if t.Package != "" && t.Package != r.pkg {
		return t.Spelling, false
	}
	ent, held := r.inPackage(t.Spelling)
	switch {
	case !held:
		return t.Spelling, false
	case ent.ambiguous:
		return t.Spelling, true
	default:
		return ent.settled, false
	}
}

// inPackage returns the entry of a bare name of the unit's package: the
// store's own entry merged with the entry of others, ambiguous where the
// two settle the name apart. It records the lookup unless the unit's own
// declarations alone settle the name.
func (r *resolver) inPackage(emitted string) (tableEntry, bool) {
	ent, held := r.table[pkgName{pkg: r.pkg, emitted: emitted}]
	var kept NameEntry
	found := false
	if r.others != nil {
		kept, found = r.others.InPackage(r.pkg, emitted)
	}
	if found || !held || ent.unit != r.unit {
		r.reads.name(NameKey{Package: r.pkg, Emitted: emitted})
	}
	switch {
	case !found:
		return ent, held
	case !held:
		return tableEntry{settled: kept.Settled, ambiguous: kept.Ambiguous, unit: -1}, true
	default:
		ent.ambiguous = ent.ambiguous || kept.Ambiguous || kept.Settled != ent.settled
		return ent, true
	}
}

// ofOrigin returns the settled name of the declaration that derives from
// an origin under an emitted name: the store's own where it declares
// one, and otherwise the entry of others. It records the lookup unless
// the unit's own declarations settle the name.
func (r *resolver) ofOrigin(origin symbol.Identity, emitted string) (string, bool) {
	ent, held := r.byOrigin[originName{id: origin, emitted: emitted}]
	if held && ent.unit == r.unit {
		return ent.settled, true
	}
	r.reads.name(NameKey{Origin: origin, Emitted: emitted})
	switch {
	case held:
		return ent.settled, true
	case r.others == nil:
		return "", false
	}
	kept, found := r.others.OfOrigin(origin, emitted)
	return kept.Settled, found
}

// followElements rewrites the spelling of a structural reference for the
// named references beneath it, its elements and their elements and type
// arguments at any depth. Each emitted name that settles to another name
// is replaced with its settled name wherever it is a whole, unqualified
// name of the spelling. The other text of the spelling remains as
// written, such as a modifier, a label, a lifetime or a bracket. The
// walk visits a reference before its elements, so the elements still
// have their emitted spellings when their parent follows them. It
// allocates only where it replaces a name.
func (r *resolver) followElements(parent *emit.TypeRef, refs []*emit.TypeRef) {
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if ref.Form == symbol.FormNamed {
			if settled, _ := r.spelling(ref); settled != ref.Spelling {
				parent.Spelling = replaceName(parent.Spelling, ref.Spelling, settled)
			}
		}
		r.followElements(parent, ref.Elems)
		r.followElements(parent, ref.Args)
	}
}

// qualifierMarks are the characters that, written before a name in a
// spelling, make it something other than a bare name of the unit's
// package: a qualifier separator, the dot of Go, Java and TypeScript
// and the colon of Rust's path, or a quote that opens a literal type.
const qualifierMarks = ".:'\"`"

// replaceName returns s with each occurrence of the name old that
// [wholeName] accepts replaced with now. It returns s itself where it
// replaces nothing, and builds the result in one buffer where it does.
func replaceName(s, old, now string) string {
	if old == "" {
		return s
	}
	var out strings.Builder
	written, from := 0, 0
	for {
		i := strings.Index(s[from:], old)
		if i < 0 {
			break
		}
		at, end := from+i, from+i+len(old)
		from = at + 1
		if !wholeName(s, at, end) {
			continue
		}
		if written == 0 {
			out.Grow(len(s) - len(old) + len(now))
		}
		out.WriteString(s[written:at])
		out.WriteString(now)
		written, from = end, end
	}
	if written == 0 {
		return s
	}
	out.WriteString(s[written:])
	return out.String()
}

// wholeName reports whether s[at:end] is a whole, unqualified name: no
// character of a name adjoins it, and no character of [qualifierMarks]
// precedes it.
func wholeName(s string, at, end int) bool {
	if before, size := utf8.DecodeLastRuneInString(s[:at]); size > 0 &&
		(nameRune(before) || strings.ContainsRune(qualifierMarks, before)) {

		return false
	}
	after, size := utf8.DecodeRuneInString(s[end:])
	return size == 0 || !nameRune(after)
}

// nameRune reports whether r continues a name in a target language: a
// letter, a digit, an underscore or a dollar sign.
func nameRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// rewriteBody rewrites the structured names of a body. A name resolves
// against the locals of the callable first, and then against the table
// of the package. The locals are the parameters, the results and the
// declaring assignments in statement order. A verbatim body remains as
// written.
func (r *resolver) rewriteBody(b *emit.Body, locals map[string]string) {
	if b.Verbatim != "" {
		return
	}
	if locals == nil {
		locals = map[string]string{}
	}
	r.rewriteStmts(b.Prologue.Items(), locals)
	r.rewriteStmts(b.Stmts, locals)
	for _, s := range b.Slots {
		if s != nil {
			r.rewriteStmts(s.Slot.Items(), locals)
		}
	}
	r.rewriteStmts(b.Epilogue.Items(), locals)
}

// rewriteStmts rewrites one run of statements. It adds the declared
// locals in statement order, so a later reference resolves against them.
// An assignment that declares nothing targets names in scope, so its
// targets resolve as a reference does. The block of a guard is a scope
// of its own.
func (r *resolver) rewriteStmts(stmts []emit.Stmt, locals map[string]string) {
	for i := range stmts {
		s := &stmts[i]
		r.rewriteExpr(&s.Value, locals)
		for k, n := range s.Names {
			if s.Declare {
				locals[n] = n
				continue
			}
			s.Names[k] = r.follow(n, locals)
		}
		if s.Name != "" {
			s.Name = r.follow(s.Name, locals)
		}
		if len(s.Then) > 0 {
			r.rewriteBlock(s.Then, locals)
		}
	}
}

// shadowed is an enclosing binding that a declaration of a block hides.
// It has the name, and the spelling of the name before the block, or
// none where the block introduced the name.
type shadowed struct {
	name string
	prev string
	had  bool
}

// rewriteBlock rewrites the block of a guard as a nested scope. The
// names that the block declares resolve inside it, and the enclosing
// bindings that they hid return when it ends. The restore list exists
// only where the block declares a name.
func (r *resolver) rewriteBlock(stmts []emit.Stmt, locals map[string]string) {
	var hidden []shadowed
	for i := range stmts {
		if !stmts[i].Declare {
			continue
		}
		for _, n := range stmts[i].Names {
			prev, had := locals[n]
			hidden = append(hidden, shadowed{name: n, prev: prev, had: had})
		}
	}
	r.rewriteStmts(stmts, locals)
	for _, h := range slices.Backward(hidden) {
		if h.had {
			locals[h.name] = h.prev
		} else {
			delete(locals, h.name)
		}
	}
}

// rewriteExpr rewrites the names of one expression tree.
func (r *resolver) rewriteExpr(x *emit.Expr, locals map[string]string) {
	if x == nil {
		return
	}
	switch x.Kind {
	case emit.ExprName:
		x.Name = r.follow(x.Name, locals)
	case emit.ExprCall:
		r.rewriteExpr(x.Fn, locals)
		for i := range x.Args {
			r.rewriteExpr(&x.Args[i], locals)
		}
	case emit.ExprValue:
		// A value has no name to follow.
	case emit.ExprRef:
		// A reference identifies a declaration of another package,
		// and the target spells it from the identity.
	}
}

// follow resolves one structured name: the callable's locals
// first, then the package's table. An ambiguous match reports and
// returns the name as written.
func (r *resolver) follow(name string, locals map[string]string) string {
	if renamed, held := locals[name]; held {
		return renamed
	}
	ent, held := r.inPackage(name)
	if !held {
		return name
	}
	if ent.ambiguous {
		r.sink.Errorf(AmbiguousReference, r.at, r.by,
			"%q matches declarations whose settled names diverge, and the "+
				"reference is left as written", name)
		return name
	}
	return ent.settled
}

// readLog records what the units of one settle read, for a caller that
// keeps the reads: each entry that a unit's references looked up and
// each override that the settle read for a unit, each once per unit. A
// nil log records nothing, so a settle without a caller of the reads
// spends nothing on them.
//
// # Allocation contract
//
// A log allocates each of its two sets on its first read. The lists of
// [Settled] grow by doubling. The log allocates the place of each unit in
// [Emit.Units] order once when it finishes. A settle whose units read
// nothing allocates none of them.
type readLog struct {
	e   *Emit
	out Settled
	// unit is the arrival index of the unit under the settle's current
	// pass, and names and facts list what that unit read in the pass.
	unit  int
	names map[NameKey]struct{}
	facts map[symbol.Identity]struct{}
}

// enter starts what the unit at arrival index i reads in a pass.
func (l *readLog) enter(i int) {
	if l == nil {
		return
	}
	l.unit = i
	clear(l.names)
	clear(l.facts)
}

// name records that the current unit looked up k.
func (l *readLog) name(k NameKey) {
	if l == nil {
		return
	}
	if _, seen := l.names[k]; seen {
		return
	}
	if l.names == nil {
		l.names = map[NameKey]struct{}{}
	}
	l.names[k] = struct{}{}
	l.out.Read = append(l.out.Read, NameRead{Unit: l.unit, Key: k})
}

// fact records that the settle read the override of an origin under
// the key named key for the current unit.
func (l *readLog) fact(origin symbol.Identity, key meta.KeyName) {
	if l == nil {
		return
	}
	if _, seen := l.facts[origin]; seen {
		return
	}
	if l.facts == nil {
		l.facts = map[symbol.Identity]struct{}{}
	}
	l.facts[origin] = struct{}{}
	l.out.Facts = append(l.out.Facts, FactRead{Unit: l.unit, Fact: meta.FactRef{Subject: origin, Key: key}})
}

// typeRefs records the lookups of the bare type references of every
// unit, for a settle that rewrites no reference: what the layout
// resolves across files. A reference that a target resolves, and one
// qualified with another package, looks nothing up.
func (l *readLog) typeRefs(e *Emit) {
	if l == nil {
		return
	}
	for i := range e.units {
		l.enter(i)
		u := &e.units[i]
		for _, d := range u.Decls {
			for s := range emit.All(d) {
				t, ref := s.(*emit.TypeRef)
				if !ref || t.Form != symbol.FormNamed || !t.Target.IsZero() ||
					(t.Package != "" && t.Package != u.Pkg.Package) {

					continue
				}
				l.name(NameKey{Package: u.Pkg.Package, Emitted: t.Spelling})
			}
		}
	}
}

// finish numbers each read's unit by its place in [Emit.Units] order,
// and sorts the reads by unit, then by key. A log without a read has
// nothing to number.
func (l *readLog) finish() {
	if l == nil || len(l.out.Read)+len(l.out.Facts) == 0 {
		return
	}
	order := l.e.sorted()
	at := make([]int, len(order))
	for k, i := range order {
		at[i] = k
	}
	for i := range l.out.Read {
		l.out.Read[i].Unit = at[l.out.Read[i].Unit]
	}
	for i := range l.out.Facts {
		l.out.Facts[i].Unit = at[l.out.Facts[i].Unit]
	}
	slices.SortFunc(l.out.Read, func(a, b NameRead) int {
		return cmp.Or(cmp.Compare(a.Unit, b.Unit), a.Key.Compare(b.Key))
	})
	slices.SortFunc(l.out.Facts, func(a, b FactRead) int {
		return cmp.Or(cmp.Compare(a.Unit, b.Unit), a.Fact.Subject.Compare(b.Fact.Subject))
	})
}
