// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
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
// follow the emitted names, and the finding says what to fix.
var CollidingNames = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 31, Meaning: "two declarations settle to one name in one scope",
})

// AmbiguousReference reports a bare reference matching declarations
// whose settled names diverge: the reference stands as written.
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
// declaration carries the input's origin, and the output states
// the lowered fact in the target's shape rather than the model's,
// so a second settle changes nothing. An error is the target
// refusing the construct.
//
// A nil list without an error keeps the declaration as it stands,
// so the common pass-through spends no allocation; a hook that
// reshaped the declaration in place returns nil the same way,
// because the store already holds it.
type Lower func(symbol.Symbol) ([]symbol.Symbol, error)

// Lowerer is the provider a backend implements when its target
// reshapes constructs. A backend without it renders declarations
// as emitted.
type Lowerer interface {
	Lower(symbol.Symbol) ([]symbol.Symbol, error)
}

// Respell spells one declared name in the target's own convention.
// Host is the kind of the declaration a member sits in,
// [symbol.KindInvalid] at file level; a kind without visibility
// passes the zero value. An error means the target cannot spell
// the name at that visibility.
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
// Findings attach to the sink under the backend's name: a refused
// construct or name withholds its declaration, colliding names
// keep their emitted spellings, and an ambiguous reference stands
// as written. A returned error is a defect in the backend's own
// hooks, a lowering that drops its input's origin, and fails the
// plan rather than one declaration.
func Settle(e *Emit, b Backend, sink *diag.Sink) error {
	if e == nil || e.settled {
		return nil
	}
	if b == nil {
		e.settled = true
		return nil
	}
	by := b.Name()

	if l, held := b.(Lowerer); held {
		if err := lowerAll(e, l, by, sink); err != nil {
			// The flag stays down: a store abandoned mid-lowering is
			// not settled, and Settled must not say it is.
			return err
		}
	}
	if r, held := b.(Respeller); held {
		respellAll(e, r, by, sink)
	}
	e.reindex()
	e.settled = true
	return nil
}

// unitPos positions a settle finding: an emit declaration carries
// no source position, so the unit's routing key names the output
// the declaration was bound for.
func unitPos(u *Unit) position.Pos { return position.Pos{File: u.Key} }

// lowerAll rewrites every declaration through the lowering hook. A
// refusal withholds the declaration under a positioned finding; an
// output carrying another origin is a defect and returns.
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
						"plugin: %s lowers a declaration of origin %s into one carrying %s: "+
							"every output carries the input's origin",
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
// answered, and what the apply pass writes. grouped marks a
// member a collision group already anchored.
type planned struct {
	host    symbol.Symbol
	kind    symbol.Kind
	at      position.Pos
	emitted string
	settled string
	final   string
	err     error
	grouped bool
}

// respellAll settles every declared name and rewrites the
// references that follow them: every visit plans first, collisions
// resolve per scope, then the plans apply and the references
// follow. Withheld declarations leave their units last, because
// every pass before that addresses a plan by the declaration index
// it was made under.
func respellAll(e *Emit, r Respeller, by diag.Origin, sink *diag.Sink) {
	plans := planNames(e, r)
	table, byOrigin := resolveTop(e, plans, by, sink)
	resolveMembers(e, plans, by, sink)
	applyNames(e, plans, by, sink)
	rewriteRefs(e, plans, table, byOrigin, by, sink)
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

// plan holds one settle's visits: one arena every pass points
// into, with a span per declaration, so the store's plan costs one
// growing slice rather than one per declaration.
type plan struct {
	spans map[[2]int]span
	all   []planned
}

// of returns one declaration's visits. The arena never grows after
// planning, so a pointer into the returned slice stays valid.
func (p *plan) of(i, j int) []planned {
	s := p.spans[[2]int{i, j}]
	return p.all[s.lo:s.hi]
}

// planNames runs the recording walk over every declaration: each
// visit calls the hook once and stores its answer by value, and
// nothing writes yet. Visit order is what the apply walk replays.
func planNames(e *Emit, r Respeller) *plan {
	total := 0
	for i := range e.units {
		total += len(e.units[i].Decls)
	}
	plans := &plan{
		spans: make(map[[2]int]span, total),
		all:   make([]planned, 0, total*2),
	}
	var at position.Pos
	// One recording callback serves every walk: the per-visit
	// state lives beside it, so planning allocates the arena's
	// growth and nothing else. The walk never errors: a hook fault
	// is kept in its entry and judged per declaration.
	record := func(
		host symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
	) (string, error) {
		p := planned{host: host, kind: kind, at: at, emitted: name}
		p.settled, p.err = r.Respell(hostKind(host), kind, v, name)
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
		for j, d := range e.units[i].Decls {
			lo := len(plans.all)
			_ = emit.RespellNames(d, record)
			plans.spans[[2]int{i, j}] = span{lo: lo, hi: len(plans.all)}
		}
	}
	return plans
}

// hostKind reads a host's kind for the hook, with the invalid kind
// at file level.
func hostKind(host symbol.Symbol) symbol.Kind {
	if host == nil {
		return symbol.KindInvalid
	}
	return host.Kind()
}

// tableEntry is one scope's settled spelling for an emitted name.
type tableEntry struct {
	settled   string
	ambiguous bool
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
// everywhere. byOrigin carries the same decisions keyed by origin
// identity, for resolved references. The clean path allocates
// per name and never per group: group storage exists only where a
// second occupant arrives.
func resolveTop(
	e *Emit, plans *plan, by diag.Origin, sink *diag.Sink,
) (map[pkgName]tableEntry, map[originName]string) {
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
				held, taken := first[at]
				if !taken {
					first[at] = p
					continue
				}
				if groups == nil {
					groups = map[scopeName][]*planned{}
				}
				if len(groups[at]) == 0 {
					groups[at] = append(groups[at], held)
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
	byOrigin := make(map[originName]string, len(plans.spans))
	for i := range e.units {
		pkg := e.units[i].Pkg.Package
		for j, d := range e.units[i].Decls {
			origin, carries := emit.OriginOf(d)
			carries = carries && !origin.IsZero()
			list := plans.of(i, j)
			for k := range list {
				p := &list[k]
				if p.host != nil || p.err != nil {
					continue
				}
				// A method's name never spells a bare reference:
				// a type or a callable does, a member of one does
				// not, so the table carries everything else.
				if p.kind != symbol.KindMethod {
					at := pkgName{pkg: pkg, emitted: p.emitted}
					if held, taken := table[at]; taken {
						if held.settled != p.final {
							held.ambiguous = true
							table[at] = held
						}
					} else {
						table[at] = tableEntry{settled: p.final}
					}
				}
				if carries {
					byOrigin[originName{id: origin, emitted: p.emitted}] = p.final
				}
			}
		}
	}
	return table, byOrigin
}

// scopeSep separates a package from a receiver inside one
// collision-scope key: a byte no spelling carries, so a joined
// key never collides with a plain one.
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
// final spellings back. A declaration whose plan holds a hook
// refusal is withheld whole under a positioned finding, because
// rendering it half-respelt would misstate it. Its entry in the
// unit is set to nil, and [dropWithheld] removes it once the
// references are rewritten. A verbatim body pins its callable's
// parameter and result names, under a finding where one would have
// changed.
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
		host symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
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

// pinnedByVerbatim reports whether a name sits in the signature a
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

// rewriteRefs follows the settled names through the store's
// references: a resolved reference follows its origin where its
// spelling is the referent's emitted bare name, a bare reference
// follows its own package's table, and structured body names
// follow locals first, then the package. Composite spellings,
// verbatim bodies and template text stand as written.
func rewriteRefs(
	e *Emit, plans *plan,
	table map[pkgName]tableEntry,
	byOrigin map[originName]string,
	by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
		u := &e.units[i]
		pkg := u.Pkg.Package
		at := unitPos(u)
		for j, d := range u.Decls {
			if d == nil {
				continue
			}
			renames := paramNames(plans.of(i, j))
			for s := range emit.All(d) {
				switch t := s.(type) {
				case *emit.TypeRef:
					rewriteRef(t, pkg, table, byOrigin, at, by, sink)
				case *emit.Function:
					rewriteBody(&t.Body, maps.Clone(renames[t]), pkg, table, at, by, sink)
				case *emit.Method:
					rewriteBody(&t.Body, maps.Clone(renames[t]), pkg, table, at, by, sink)
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

// rewriteRef follows one type reference: resolved by origin where
// the spelling is the referent's emitted bare name, bare by the
// package's table otherwise. An ambiguous match reports and
// stands.
func rewriteRef(
	t *emit.TypeRef, pkg string, table map[pkgName]tableEntry,
	byOrigin map[originName]string,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) {
	if !t.Target.IsZero() {
		if settled, match := byOrigin[originName{id: t.Target, emitted: t.Spelling}]; match {
			t.Spelling = settled
		}
		return
	}
	ent, held := table[pkgName{pkg: pkg, emitted: t.Spelling}]
	if !held {
		return
	}
	if ent.ambiguous {
		sink.Errorf(AmbiguousReference, at, by,
			"%q matches declarations whose settled names diverge, and the "+
				"reference stands as written", t.Spelling)
		return
	}
	t.Spelling = ent.settled
}

// rewriteBody follows a body's structured names: statement and
// expression names resolve against the callable's locals first,
// its parameters, results and declaring assignments in statement
// order, then the package's table. A verbatim body stands as
// written.
func rewriteBody(
	b *emit.Body, locals map[string]string, pkg string,
	table map[pkgName]tableEntry,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) {
	if b.Verbatim != "" {
		return
	}
	if locals == nil {
		locals = map[string]string{}
	}
	rewriteStmts(b.Prologue.Items(), locals, pkg, table, at, by, sink)
	rewriteStmts(b.Stmts, locals, pkg, table, at, by, sink)
	for _, s := range b.Slots {
		if s != nil {
			rewriteStmts(s.Slot.Items(), locals, pkg, table, at, by, sink)
		}
	}
	rewriteStmts(b.Epilogue.Items(), locals, pkg, table, at, by, sink)
}

// rewriteStmts follows one statement run, accumulating declared
// locals in statement order so a later reference resolves against
// them. An assignment that declares nothing targets names already
// in scope, so its targets resolve the way a reference does, and a
// guard's block is a scope of its own.
func rewriteStmts(
	stmts []emit.Stmt, locals map[string]string, pkg string,
	table map[pkgName]tableEntry,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) {
	for i := range stmts {
		s := &stmts[i]
		rewriteExpr(&s.Value, locals, pkg, table, at, by, sink)
		for k, n := range s.Names {
			if s.Declare {
				locals[n] = n
				continue
			}
			s.Names[k] = follow(n, locals, pkg, table, at, by, sink)
		}
		if s.Name != "" {
			s.Name = follow(s.Name, locals, pkg, table, at, by, sink)
		}
		if len(s.Then) > 0 {
			rewriteBlock(s.Then, locals, pkg, table, at, by, sink)
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
func rewriteBlock(
	stmts []emit.Stmt, locals map[string]string, pkg string,
	table map[pkgName]tableEntry,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) {
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
	rewriteStmts(stmts, locals, pkg, table, at, by, sink)
	for _, h := range slices.Backward(hidden) {
		if h.had {
			locals[h.name] = h.prev
		} else {
			delete(locals, h.name)
		}
	}
}

// rewriteExpr follows one expression tree's names.
func rewriteExpr(
	x *emit.Expr, locals map[string]string, pkg string,
	table map[pkgName]tableEntry,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) {
	if x == nil {
		return
	}
	switch x.Kind {
	case emit.ExprName:
		x.Name = follow(x.Name, locals, pkg, table, at, by, sink)
	case emit.ExprCall:
		rewriteExpr(x.Fn, locals, pkg, table, at, by, sink)
		for i := range x.Args {
			rewriteExpr(&x.Args[i], locals, pkg, table, at, by, sink)
		}
	}
}

// follow resolves one structured name: the callable's locals
// first, then the package's table. An ambiguous match reports and
// returns the name as written.
func follow(
	name string, locals map[string]string, pkg string,
	table map[pkgName]tableEntry,
	at position.Pos, by diag.Origin, sink *diag.Sink,
) string {
	if renamed, held := locals[name]; held {
		return renamed
	}
	ent, held := table[pkgName{pkg: pkg, emitted: name}]
	if !held {
		return name
	}
	if ent.ambiguous {
		sink.Errorf(AmbiguousReference, at, by,
			"%q matches declarations whose settled names diverge, and the "+
				"reference stands as written", name)
		return name
	}
	return ent.settled
}
