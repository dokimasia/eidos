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

// RefusedConstruct reports a lowering hook refusing a declaration:
// the target declares no idiom for the construct, and the
// declaration is withheld from render.
var RefusedConstruct = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 29, Meaning: "the target declares no idiom for a construct",
})

// RefusedName reports a respell hook refusing a declared name: the
// target cannot spell it at its visibility, and the declaration is
// withheld from render, because rendering it under another access
// would misstate the declaration.
var RefusedName = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 30, Meaning: "the target cannot spell a declared name at its visibility",
})

// CollidingNames reports two declarations in one scope settling to
// one name: both keep their emitted names, references to either
// follow the emitted names, and the finding names what to fix.
var CollidingNames = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 31, Meaning: "two declarations settle to one name in one scope",
})

// AmbiguousReference reports a bare reference matching declarations
// whose settled names diverge: the reference is left as written.
var AmbiguousReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 32, Meaning: "a bare reference matches declarations whose settled names diverge",
})

// VerbatimParams reports a verbatim body pinning its callable's
// parameter and result names: the body's text reads them as
// emitted, so the signature keeps them as emitted too, and one
// would have respelled differently.
var VerbatimParams = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 33, Meaning: "a verbatim body pins parameter names one respell would have changed",
})

// Lower rewrites one declaration into the target's own construct
// shape: the same fact, the target's declarations. Every returned
// declaration has the input's origin, and the output states the
// lowered fact in the target's shape and not in the model's, so a
// second settle changes nothing. An error is the target refusing
// the construct.
//
// A nil list without an error keeps the declaration unchanged, so
// the common pass-through spends no allocation. A hook that
// reshaped the declaration in place returns nil the same way,
// because the store already contains it.
type Lower func(symbol.Symbol) ([]symbol.Symbol, error)

// Lowerer is the provider a backend implements when its target
// reshapes constructs. A backend without it renders declarations
// as emitted.
type Lowerer interface {
	Lower(symbol.Symbol) ([]symbol.Symbol, error)
}

// Respell spells one declared name in the target's own convention.
// Host is the kind of the declaration a member is declared in,
// [symbol.KindInvalid] at file level. A kind without visibility
// passes the zero value. An error means the target cannot spell the
// name at that visibility.
type Respell func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error)

// Respeller is the provider a backend implements when its target
// respells declared names. A backend without it renders names as
// emitted.
type Respeller interface {
	Respell(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error)
}

// Settle takes one plan's store through the backend's lowering
// seams, once, after the schedule and before the render: every
// declaration lowers into the target's construct shapes, every
// declared name respells into the target's convention, references
// follow their declarations, and the per-kind index rebuilds over
// what remains. The store is marked settled when it returns, and a
// settled store settles to itself, so a second call changes
// nothing.
//
// A declared name takes an override where facts contains one. An
// override is a value of the target's [Target.NameKey], such as
// golang.name, on the declaration's origin, written at directive
// authority or above. The settle reads each override through a point
// read of the origin's fact. The override replaces the respell hook's
// spelling for the declaration that renders the origin. The hook spells
// the emitted name of that declaration the same way as the origin's own
// name. A file-level declaration renders a nested origin under the
// origin's flat name, [symbol.Identity.FlatName]. Another declaration
// with the same origin, such as a mock of an interface, keeps the hook's
// spelling. The settle withholds a name that the hook refuses, also
// where the origin has an override. A plugin's stamp on the key is not
// an override, because the hook spells the target's convention. The
// settle does not apply an override where the fact store is nil, where
// the composition did not register the name key, or where the backend
// has no respell hook.
//
// The settle records whether a declaration that renders has a type
// reference whose target is a declaration of another language than the
// backend's target, which [Emit.Translates] reports.
//
// Findings attach to the sink under the backend's name: a refused
// construct or name withholds its declaration, colliding names
// keep their emitted spellings, and an ambiguous reference is left
// as written. A returned error is a defect in the backend's own
// hooks, a lowering that drops its input's origin, and fails the
// whole plan, not one declaration.
//
// # Allocation contract
//
// A backend without a lowering hook and a respell hook settles the
// store without allocating, because nothing in it changes. Otherwise
// Settle rebuilds the store's per-kind index, one list per unit and
// kind, and a respell hook adds the plan of every declared name and the
// tables the references follow, each sized once to the store.
func Settle(e *Emit, b Backend, facts *meta.Facts, sink *diag.Sink) error {
	return settle(e, b, facts, sink, nil, nil)
}

// SettleWith settles a store that contains part of a plan's units: the
// units of the files that a warm run generates again. It settles as
// [Settle] does, and resolves each reference that the store does not
// settle against others, the names of the plan's other files. A bare
// reference that both declare under different settled names is
// ambiguous, as it is where the store declares the plan whole. Names
// collide only with the store's own names, so the plan places the units
// of every file with a name in a changed scope into the store before it
// settles. Settle is SettleWith with no others.
//
// SettleWith returns what each unit read. It lists every entry that the
// unit's references looked up, whether a file declares the entry or not.
// It also lists the name override of every origin that the unit's
// declarations render. It leaves out a lookup that the unit's own
// declarations settle with no entry in others, because the unit
// generates again whenever its own names change. A backend without a
// respell hook rewrites no reference, and SettleWith then lists the
// lookups of the bare type references alone. The errors are the errors
// of Settle.
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
	l, lowers := b.(Lowerer)
	r, respells := b.(Respeller)
	if !lowers && !respells {
		reads.typeRefs(e)
		reads.finish()
		e.translates = b != nil && translates(e, b.Target())
		e.settled = true
		return nil
	}
	by := b.Name()

	if lowers {
		if err := lowerAll(e, l, by, sink); err != nil {
			// A store abandoned mid-lowering is not settled, so the
			// flag remains down and Settled reports false.
			return err
		}
	}
	if respells {
		respellAll(e, r, b.Target(), overridesFor(facts, b.Target()), others, by, sink, reads)
	} else {
		reads.typeRefs(e)
		e.translates = translates(e, b.Target())
	}
	reads.finish()
	e.reindex()
	e.settled = true
	return nil
}

// translates reports whether a declaration of the store has a type
// reference whose target is a declaration of another language than the
// target t. It visits every node of the store, and allocates nothing.
func translates(e *Emit, t Target) bool {
	lang := symbol.Lang(t)
	for i := range e.units {
		for _, d := range e.units[i].Decls {
			for s := range emit.All(d) {
				if ref, is := s.(*emit.TypeRef); is && !ref.Target.IsZero() && ref.Target.Lang != lang {
					return true
				}
			}
		}
	}
	return false
}

// overrides reads one target's name overrides: the name written on an
// origin at directive authority or above. It reads each origin with one
// point read of the claim that ranks first, so a stamp of the name at
// plugin authority, which a lowering entry writes on every declaration
// that a plan of the target renders, costs the settle nothing. A store
// that a run restores does not restore the bag of an origin on which
// the name key is absent. The zero value, for a nil fact store or a
// name key the composition did not register, reads none.
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

// on returns the override written on one origin: the value of the claim
// that ranks first, where that claim has directive authority or above.
// It reports false for a reader of no store, for the zero identity,
// which no declaration renders, for an origin without a claim at that
// authority and for an empty value. It allocates nothing.
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

// unitPos positions a settle finding: an emit declaration has no
// source position, so the unit's routing key names the output the
// declaration was bound for.
func unitPos(u *Unit) position.Pos { return position.Pos{File: u.Key} }

// lowerAll rewrites every declaration through the lowering hook. A
// refusal withholds the declaration under a positioned finding, and
// an output with another origin is a defect and returns.
func lowerAll(e *Emit, l Lowerer, by diag.Origin, sink *diag.Sink) error {
	for i := range e.units {
		u := &e.units[i]
		lowered := make([]symbol.Symbol, 0, len(u.Decls))
		for _, d := range u.Decls {
			out, err := l.Lower(d)
			if err != nil {
				sink.Errorf(RefusedConstruct, unitPos(u), by, "%v", err)
				continue
			}
			if out == nil {
				lowered = append(lowered, d)
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
		}
		u.Decls = lowered
	}
	return nil
}

// planned is one name's visit: what the walk met, what the hook
// returned, and what the apply pass writes. grouped marks a member
// a collision group already anchored.
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

// respellAll settles every declared name and rewrites the
// references that follow them: every visit plans first, collisions
// resolve per scope, then the plans apply and the references
// follow, against the store's names and then against others.
// Withheld declarations leave their units last, because every pass
// before that addresses a plan by the declaration index it was made
// under. reads records each unit's override reads and lookups. The
// reference pass records whether a reference has a target of another
// language than the target t.
func respellAll(
	e *Emit, r Respeller, t Target, over overrides, others Names, by diag.Origin, sink *diag.Sink, reads *readLog,
) {
	plans := planNames(e, r, over, reads)
	table, byOrigin := resolveTop(e, plans, by, sink)
	resolveMembers(e, plans, by, sink)
	applyNames(e, plans, by, sink)
	res := &resolver{
		table: table, byOrigin: byOrigin, others: others, by: by, sink: sink, reads: reads, lang: symbol.Lang(t),
	}
	res.rewriteRefs(e, plans)
	dropWithheld(e)
}

// dropWithheld removes the declarations applyNames withheld, which
// it marks nil in place so the reference pass still finds each
// survivor's plan at its original index.
func dropWithheld(e *Emit) {
	for i := range e.units {
		u := &e.units[i]
		u.Decls = slices.DeleteFunc(u.Decls, func(d symbol.Symbol) bool { return d == nil })
	}
}

// span is one declaration's slice of the plan arena.
type span struct{ lo, hi int }

// plan contains one settle's visits: one arena every pass points
// into, with a span per declaration, so the store's plan costs one
// growing slice and not one per declaration.
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

// planNames runs the recording walk over every declaration: each
// visit calls the hook, takes the override of the declaration that
// renders its origin in place of the hook's spelling, and stores the
// result by value, and nothing writes yet. Visit order is what the
// apply walk replays. reads records the override that each unit read
// for each origin that its declarations render.
func planNames(e *Emit, r Respeller, over overrides, reads *readLog) *plan {
	total := 0
	for i := range e.units {
		total += len(e.units[i].Decls)
	}
	plans := &plan{
		spans: make(map[[2]int]span, total),
		all:   make([]planned, 0, total*2),
	}
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
		p.settled, p.err = r.Respell(hostKind(host), kind, v, name)
		// A carrier without an origin reads the zero identity, which
		// takes no override.
		origin, _ := emit.OriginOf(carrier)
		if over.facts != nil && !origin.IsZero() {
			reads.fact(origin, over.name)
		}
		if override, renders := overrideOf(r, over, host, origin, kind, v, p.settled); renders {
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
// the origin: an override is written on the origin, and the carrier
// renders it where the hook spells the origin's own name as it
// spelled the carrier's, settled. A file-level carrier renders a nested
// origin under the origin's flat name, [symbol.Identity.FlatName],
// because a target without nested types declares the origin in the
// file. An origin without an override, a name the hook refuses and a
// name the hook spells apart from the origin's report false. It
// allocates the flat name of a nested origin that has an override, and
// nothing otherwise.
func overrideOf(
	r Respeller, over overrides, host symbol.Symbol, origin symbol.Identity,
	kind symbol.Kind, v symbol.Visibility, settled string,
) (string, bool) {
	override, written := over.on(origin)
	if !written {
		return "", false
	}
	name := origin.Name
	if host == nil {
		name = origin.FlatName()
	}
	own, err := r.Respell(hostKind(host), kind, v, name)
	if err != nil || own != settled {
		return "", false
	}
	return override, true
}

// hostKind reads a host's kind for the hook, with the invalid kind
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

// scopeName keys one settled spelling in one collision scope.
type scopeName struct {
	scope   string
	settled string
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

// resolveTop settles the file-level names: collisions per scope
// revert to the emitted spellings under one finding each, and the
// survivors form the table references follow. A declaration's
// scope is its package; a receiver-attached method's is its
// receiver, because two types declare one method name legally
// everywhere. byOrigin records the same decisions keyed by origin
// identity, for resolved references. Both record the unit that
// declares each name. The clean path allocates per name and never
// per group: group storage exists only where a second occupant
// arrives.
func resolveTop(
	e *Emit, plans *plan, by diag.Origin, sink *diag.Sink,
) (map[pkgName]tableEntry, map[originName]originEntry) {
	first := make(map[scopeName]*planned, len(plans.spans))
	var groups map[scopeName][]*planned
	for i := range e.units {
		pkg := e.units[i].Pkg.Package
		for j, d := range e.units[i].Decls {
			key := scopeKey(pkg, d)
			list := plans.of(i, j)
			for k := range list {
				p := &list[k]
				if p.host != nil || p.err != nil {
					continue
				}
				at := scopeName{scope: key, settled: p.settled}
				occupant, taken := first[at]
				if !taken {
					first[at] = p
					continue
				}
				if groups == nil {
					groups = map[scopeName][]*planned{}
				}
				if len(groups[at]) == 0 {
					groups[at] = append(groups[at], occupant)
				}
				groups[at] = append(groups[at], p)
			}
		}
	}
	if len(groups) > 0 {
		keys := slices.SortedFunc(maps.Keys(groups), func(a, b scopeName) int {
			if c := strings.Compare(a.scope, b.scope); c != 0 {
				return c
			}
			return strings.Compare(a.settled, b.settled)
		})
		for _, at := range keys {
			group := groups[at]
			names := make([]string, 0, len(group))
			for _, p := range group {
				names = append(names, p.emitted)
				p.final = p.emitted
			}
			sink.Errorf(CollidingNames, group[0].at, by,
				"%s settle to %q in %s, and every one keeps its emitted name",
				strings.Join(names, " and "), at.settled, at.scope)
		}
	}

	table := make(map[pkgName]tableEntry, len(plans.spans))
	byOrigin := make(map[originName]originEntry, len(plans.spans))
	for i := range e.units {
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
				// A method's name never spells a bare reference:
				// a type or a callable does, a member of one does
				// not, so the table records everything else.
				if p.kind != symbol.KindMethod {
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

// scopeSep separates a package from a receiver inside one
// collision-scope key: a byte no spelling contains, so a joined key
// never collides with a plain one.
const scopeSep = "\x00"

// scopeKey names the collision scope one file-level declaration
// settles in: the package, and behind it the receiver a method
// attaches to, so two receivers declare one method name without
// meeting.
func scopeKey(pkg string, d symbol.Symbol) string {
	m, held := d.(*emit.Method)
	if !held || m.Receives == nil {
		return pkg
	}
	return pkg + scopeSep + m.Receives.Spelling
}

// resolveMembers reverts member collisions: within one host,
// members whose distinct emitted names settle to one name keep
// their emitted spellings under one finding. Members sharing one
// emitted name are overloads, or duplicates the language's own
// lowering refuses, and the respell did not bring them together.
// Plans are short, so the scan compares pairs and allocates only
// where a collision exists.
func resolveMembers(
	e *Emit, plans *plan, by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
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

// collides reports whether a later member of list[a]'s host
// settles to list[a]'s name from a different emitted name.
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

// applyNames replays every plan over its declaration, writing the
// final spellings back, and records the emitted name of each
// declaration whose spelling changed, which an export keys on. A
// declaration whose plan contains a hook refusal is withheld whole
// under a positioned finding, because rendering it half-respelt would
// misstate it. Its entry in the unit is set to nil, and
// [dropWithheld] removes it once the references are rewritten. A
// verbatim body pins its callable's parameter and result names, under
// a finding where one would have changed.
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

// pinnedName reads the callable's name a verbatim pin reports.
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

// pinnedByVerbatim reports whether a name is in the signature a
// verbatim body reads: the parameters and results of a callable
// whose body is literal text.
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

// rewriteRefs follows the settled names through the store's
// references: a resolved reference follows its origin where its
// spelling is the referent's emitted bare name, a bare reference
// follows its own package's table, a structural reference's spelling
// follows the names beneath it, and structured body names follow
// locals first, then the package. A composite spelling without
// elements, verbatim bodies and template text are left as written. The
// pass marks the store where a reference of a declaration that renders
// has a target of another language than the resolver's.
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
					if !t.Target.IsZero() && t.Target.Lang != r.lang {
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

// paramNames collects each callable's parameter and result names
// from its plan, emitted spelling to final, so its body's
// references follow them. Every such name arrives, renamed or
// not, because an unrenamed parameter still shadows the package
// scope; a plan without one returns nothing.
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
// package binds it, and it is left as written. It reports true for a
// spelling the table matches to declarations whose settled names
// diverge, which is left as written too. It reads two maps and
// allocates nothing but what others and the read log allocate.
func (r *resolver) spelling(t *emit.TypeRef) (string, bool) {
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

// followElements rewrites a structural reference's spelling for the
// named references beneath it: its elements, and their elements and
// type arguments at any depth. Each emitted name that settles apart is
// replaced with its settled name wherever it is a whole, unqualified
// name of the spelling, so the spelling's other text, a modifier, a
// label, a lifetime or a bracket, remains as written. The walk visits a
// reference before its elements, so the elements still have their
// emitted spellings when their parent follows them. It allocates only
// where it replaces a name.
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

// rewriteBody follows a body's structured names: statement and
// expression names resolve against the callable's locals first,
// its parameters, results and declaring assignments in statement
// order, then the package's table. A verbatim body is left as
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

// rewriteStmts follows one statement run, accumulating declared
// locals in statement order so a later reference resolves against
// them. An assignment that declares nothing targets names already
// in scope, so its targets resolve the way a reference does, and a
// guard's block is a scope of its own.
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

// shadowed is one enclosing binding a block's declaration hides:
// the name, and the spelling it resolved to before the block, or
// none where the block introduced it.
type shadowed struct {
	name string
	prev string
	had  bool
}

// rewriteBlock follows a guard's block as a nested scope: the names
// the block declares resolve inside it, and the enclosing bindings
// they hid return when it ends. The restore list exists only where
// the block declares a name.
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

// rewriteExpr follows one expression tree's names.
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
