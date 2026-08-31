// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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
	e.settled = true
	if b == nil {
		return nil
	}
	by := b.Name()

	if l, held := b.(Lowerer); held {
		if err := lowerAll(e, l, by, sink); err != nil {
			return err
		}
	}
	if r, held := b.(Respeller); held {
		respellAll(e, r, by, sink)
	}
	e.reindex()
	return nil
}

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
				sink.Errorf(RefusedConstruct, d.Position(), by, "%v", err)
				continue
			}
			origin, _ := emit.OriginOf(d)
			for _, o := range out {
				produced, _ := emit.OriginOf(o)
				if produced != origin {
					return fmt.Errorf(
						"plugin: %s lowers a declaration of origin %s into one carrying %s: "+
							"every output carries the input's origin",
						by, origin, produced)
				}
			}
			lowered = append(lowered, out...)
		}
		u.Decls = lowered
	}
	return nil
}

// planned is one name's visit: what the walk met, what the hook
// answered, and what the apply pass writes.
type planned struct {
	host    symbol.Symbol
	kind    symbol.Kind
	at      position.Pos
	emitted string
	settled string
	final   string
	err     error
}

// respellAll settles every declared name and rewrites the
// references that follow them: every visit plans first, collisions
// resolve per scope, then the plans apply and the references
// follow.
func respellAll(e *Emit, r Respeller, by diag.Origin, sink *diag.Sink) {
	plans := planNames(e, r)
	table, byOrigin := resolveTop(e, plans, by, sink)
	resolveMembers(e, plans, by, sink)
	applyNames(e, plans, by, sink)
	rewriteRefs(e, plans, table, byOrigin, by, sink)
}

// planNames runs the recording walk over every declaration: each
// visit calls the hook once and stores its answer, and nothing
// writes yet. Plans key by unit and declaration index, in visit
// order, which the apply walk replays.
func planNames(e *Emit, r Respeller) map[[2]int][]*planned {
	plans := make(map[[2]int][]*planned)
	for i := range e.units {
		for j, d := range e.units[i].Decls {
			var list []*planned
			at := d.Position()
			// The recording walk never errors: a hook fault is kept
			// in its entry and judged per declaration.
			_ = emit.RespellNames(d, func(
				host symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
			) (string, error) {
				p := &planned{host: host, kind: kind, at: at, emitted: name}
				p.settled, p.err = r.Respell(hostKind(host), kind, v, name)
				p.final = p.settled
				if p.err != nil {
					p.final = name
				}
				list = append(list, p)
				return name, nil
			})
			plans[[2]int{i, j}] = list
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

// resolveTop settles the file-level names: collisions per package
// revert to the emitted spellings under one finding each, and the
// survivors form the table references follow. byOrigin carries the
// same decisions keyed by origin identity, for resolved
// references.
func resolveTop(
	e *Emit, plans map[[2]int][]*planned, by diag.Origin, sink *diag.Sink,
) (map[string]map[string]tableEntry, map[symbol.Identity]map[string]string) {
	perScope := map[string]map[string][]*planned{}
	for i := range e.units {
		pkg := e.units[i].Pkg.Package
		for j := range e.units[i].Decls {
			for _, p := range plans[[2]int{i, j}] {
				if p.host != nil || p.err != nil {
					continue
				}
				scope := perScope[pkg]
				if scope == nil {
					scope = map[string][]*planned{}
					perScope[pkg] = scope
				}
				scope[p.settled] = append(scope[p.settled], p)
			}
		}
	}
	for _, pkg := range slices.Sorted(maps.Keys(perScope)) {
		for _, settled := range slices.Sorted(maps.Keys(perScope[pkg])) {
			group := perScope[pkg][settled]
			if len(group) < 2 {
				continue
			}
			names := make([]string, 0, len(group))
			for _, p := range group {
				names = append(names, p.emitted)
				p.final = p.emitted
			}
			sink.Errorf(CollidingNames, group[0].at, by,
				"%s settle to %q in %s, and every one keeps its emitted name",
				strings.Join(names, " and "), settled, pkg)
		}
	}

	table := map[string]map[string]tableEntry{}
	byOrigin := map[symbol.Identity]map[string]string{}
	for i := range e.units {
		pkg := e.units[i].Pkg.Package
		for j, d := range e.units[i].Decls {
			for _, p := range plans[[2]int{i, j}] {
				if p.host != nil || p.err != nil {
					continue
				}
				scope := table[pkg]
				if scope == nil {
					scope = map[string]tableEntry{}
					table[pkg] = scope
				}
				if held, taken := scope[p.emitted]; taken {
					if held.settled != p.final {
						held.ambiguous = true
						scope[p.emitted] = held
					}
				} else {
					scope[p.emitted] = tableEntry{settled: p.final}
				}
				if origin, carries := emit.OriginOf(d); carries && !origin.IsZero() {
					m := byOrigin[origin]
					if m == nil {
						m = map[string]string{}
						byOrigin[origin] = m
					}
					m[p.emitted] = p.final
				}
			}
		}
	}
	return table, byOrigin
}

// resolveMembers reverts member collisions: within one host, two
// members settling to one name keep their emitted spellings under
// one finding.
func resolveMembers(
	e *Emit, plans map[[2]int][]*planned, by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
		for j := range e.units[i].Decls {
			var hosts []symbol.Symbol
			perHost := map[symbol.Symbol]map[string][]*planned{}
			for _, p := range plans[[2]int{i, j}] {
				if p.host == nil || p.err != nil {
					continue
				}
				members := perHost[p.host]
				if members == nil {
					members = map[string][]*planned{}
					perHost[p.host] = members
					hosts = append(hosts, p.host)
				}
				members[p.final] = append(members[p.final], p)
			}
			for _, host := range hosts {
				members := perHost[host]
				for _, settled := range slices.Sorted(maps.Keys(members)) {
					group := members[settled]
					if len(group) < 2 {
						continue
					}
					names := make([]string, 0, len(group))
					for _, p := range group {
						names = append(names, p.emitted)
						p.final = p.emitted
					}
					slices.Sort(names)
					sink.Errorf(CollidingNames, group[0].at, by,
						"%s settle to %q in one host, and every one keeps its emitted name",
						strings.Join(names, " and "), settled)
				}
			}
		}
	}
}

// applyNames replays every plan over its declaration, writing the
// final spellings back. A declaration whose plan holds a hook
// refusal is withheld whole under a positioned finding, because
// rendering it half-respelt would misstate it. A verbatim body
// pins its callable's parameter and result names, under a finding
// where one would have changed.
func applyNames(
	e *Emit, plans map[[2]int][]*planned, by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
		u := &e.units[i]
		kept := make([]symbol.Symbol, 0, len(u.Decls))
		for j, d := range u.Decls {
			list := plans[[2]int{i, j}]
			if refused := firstErr(list); refused != nil {
				sink.Errorf(RefusedName, d.Position(), by, "%v", refused.err)
				continue
			}
			warned := map[symbol.Symbol]bool{}
			at := 0
			_ = emit.RespellNames(d, func(
				host symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
			) (string, error) {
				if at >= len(list) {
					return name, nil
				}
				p := list[at]
				at++
				if pinnedByVerbatim(host, kind) {
					if p.final != p.emitted && !warned[host] {
						warned[host] = true
						sink.Errorf(VerbatimParams, d.Position(), by,
							"a verbatim body pins %q, which would have respelled to %q",
							p.emitted, p.final)
					}
					p.final = p.emitted
				}
				return p.final, nil
			})
			kept = append(kept, d)
		}
		u.Decls = kept
	}
}

// firstErr returns the first refused visit in a plan, nil when
// every name spelled.
func firstErr(list []*planned) *planned {
	for _, p := range list {
		if p.err != nil {
			return p
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
	e *Emit, plans map[[2]int][]*planned,
	table map[string]map[string]tableEntry,
	byOrigin map[symbol.Identity]map[string]string,
	by diag.Origin, sink *diag.Sink,
) {
	for i := range e.units {
		u := &e.units[i]
		scope := table[u.Pkg.Package]
		for j, d := range u.Decls {
			renames := paramNames(plans[[2]int{i, j}])
			for s := range emit.All(d) {
				switch t := s.(type) {
				case *emit.TypeRef:
					rewriteRef(t, scope, byOrigin, d, by, sink)
				case *emit.Function:
					rewriteBody(&t.Body, maps.Clone(renames[t]), scope, d, by, sink)
				case *emit.Method:
					rewriteBody(&t.Body, maps.Clone(renames[t]), scope, d, by, sink)
				}
			}
		}
	}
}

// paramNames collects each callable's parameter and result names
// from its plan, emitted spelling to final, so its body's
// references follow them. Every such name arrives, renamed or
// not, because an unrenamed parameter still shadows the package
// scope.
func paramNames(list []*planned) map[symbol.Symbol]map[string]string {
	out := map[symbol.Symbol]map[string]string{}
	for _, p := range list {
		if p.host == nil ||
			(p.kind != symbol.KindParam && p.kind != symbol.KindReturn) {
			continue
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
	t *emit.TypeRef, scope map[string]tableEntry,
	byOrigin map[symbol.Identity]map[string]string,
	d symbol.Symbol, by diag.Origin, sink *diag.Sink,
) {
	if !t.Target.IsZero() {
		if m, held := byOrigin[t.Target]; held {
			if settled, match := m[t.Spelling]; match {
				t.Spelling = settled
			}
		}
		return
	}
	ent, held := scope[t.Spelling]
	if !held {
		return
	}
	if ent.ambiguous {
		sink.Errorf(AmbiguousReference, d.Position(), by,
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
	b *emit.Body, locals map[string]string, table map[string]tableEntry,
	d symbol.Symbol, by diag.Origin, sink *diag.Sink,
) {
	if b.Verbatim != "" {
		return
	}
	if locals == nil {
		locals = map[string]string{}
	}
	rewriteStmts(b.Prologue.Items(), locals, table, d, by, sink)
	rewriteStmts(b.Stmts, locals, table, d, by, sink)
	for _, s := range b.Slots {
		if s != nil {
			rewriteStmts(s.Slot.Items(), locals, table, d, by, sink)
		}
	}
	rewriteStmts(b.Epilogue.Items(), locals, table, d, by, sink)
}

// rewriteStmts follows one statement run, accumulating declared
// locals in statement order so a later reference resolves against
// them.
func rewriteStmts(
	stmts []emit.Stmt, locals map[string]string, table map[string]tableEntry,
	d symbol.Symbol, by diag.Origin, sink *diag.Sink,
) {
	for i := range stmts {
		s := &stmts[i]
		rewriteExpr(&s.Value, locals, table, d, by, sink)
		for _, n := range s.Names {
			locals[n] = n
		}
		if s.Name != "" {
			if renamed, held := locals[s.Name]; held {
				s.Name = renamed
			}
		}
		rewriteStmts(s.Then, locals, table, d, by, sink)
	}
}

// rewriteExpr follows one expression tree's names.
func rewriteExpr(
	x *emit.Expr, locals map[string]string, table map[string]tableEntry,
	d symbol.Symbol, by diag.Origin, sink *diag.Sink,
) {
	if x == nil {
		return
	}
	switch x.Kind {
	case emit.ExprName:
		if renamed, held := locals[x.Name]; held {
			x.Name = renamed
			return
		}
		ent, held := table[x.Name]
		if !held {
			return
		}
		if ent.ambiguous {
			sink.Errorf(AmbiguousReference, d.Position(), by,
				"%q matches declarations whose settled names diverge, and the "+
					"reference stands as written", x.Name)
			return
		}
		x.Name = ent.settled
	case emit.ExprCall:
		rewriteExpr(x.Fn, locals, table, d, by, sink)
		for i := range x.Args {
			rewriteExpr(&x.Args[i], locals, table, d, by, sink)
		}
	}
}
