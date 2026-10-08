// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// dirtySet contains the dirty edges of a warm run, and the records of the
// generation that read them. The first time an edge becomes dirty, the
// set reads the edge's readers from the readers table. Each validation
// among them names a subject that the run validates again. Each annotator
// invocation joins the pending records of its plugin, and the plugin's
// phase call executes the invocation again or removes it. Each plan's
// invocation and group joins the dirty records of its plan, which the
// plan executes again when it runs. A check runs again in Close, so it
// does not join the set.
//
// A membership edge, of a declaration kind or a directive spelling, routes
// only its validations and annotator invocations, which read the whole
// graph. The set lists the packages in which the edge's membership
// changed, and each plan routes its own readers of the edge where its
// sources admit one of them.
//
// A probe of a candidate routes the invocations of the candidate and
// leaves its declaration edge clean. The run withdraws the claims of the
// annotator invocations before the call evaluates the candidate again.
//
// Edges become dirty in the order of the run's phases. The load's
// changes come first, then the subjects whose validated directives
// changed, and then each fact whose winner changed.
//
// # Concurrency
//
// A dirtySet is not safe for concurrent use. The run's goroutine changes
// it between phase calls.
//
// # Allocation contract
//
// Each edge, routed reader and pending record allocates its entries in
// the set's maps. Each readers row and record row that the set reads
// allocates its decode.
type dirtySet struct {
	phases *state.PhaseState
	edges  map[state.EdgeHash]struct{}
	// members lists, for each dirty membership edge, the packages in which
	// its membership changed.
	members map[state.EdgeHash]map[symbol.Identity]struct{}
	// routed lists the records that the set has routed whole, so it reads
	// each such record once.
	routed map[state.RecordRef]struct{}
	// validations lists the subjects whose recorded validation read a
	// dirty edge.
	validations map[symbol.Identity]struct{}
	// pending lists the recorded annotator invocations that a dirty edge
	// or a probe routed, by plugin and then by match.
	pending map[plugin.ID]map[plugin.MatchKey]state.Invocation
	// plans lists the recorded invocations and groups of each plan that a
	// dirty edge or a probe routed.
	plans map[string]*planDirt
	// probed lists the subjects whose invocations already joined pending.
	probed map[symbol.Identity]struct{}
}

// planDirt is the dirty records of one plan that the shared phases
// routed: its invocations by match, and its groups by key unit.
type planDirt struct {
	invocations map[plugin.MatchKey]state.Invocation
	groups      map[plugin.UnitRef]state.Group
}

// newDirtySet returns an empty dirty set over a generation's record of
// the phases.
func newDirtySet(phases *state.PhaseState) *dirtySet {
	return &dirtySet{
		phases:      phases,
		edges:       map[state.EdgeHash]struct{}{},
		members:     map[state.EdgeHash]map[symbol.Identity]struct{}{},
		routed:      map[state.RecordRef]struct{}{},
		validations: map[symbol.Identity]struct{}{},
		pending:     map[plugin.ID]map[plugin.MatchKey]state.Invocation{},
		plans:       map[string]*planDirt{},
		probed:      map[symbol.Identity]struct{}{},
	}
}

// plan returns the dirty records of a plan, which it creates empty on
// first use.
func (d *dirtySet) plan(name string) *planDirt {
	p := d.plans[name]
	if p == nil {
		p = &planDirt{
			invocations: map[plugin.MatchKey]state.Invocation{},
			groups:      map[plugin.UnitRef]state.Group{},
		}
		d.plans[name] = p
	}
	return p
}

// add makes an edge dirty and routes every record that read it. An edge
// that is already dirty routes nothing.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (d *dirtySet) add(e state.EdgeHash) error {
	if _, dirty := d.edges[e]; dirty {
		return nil
	}
	d.edges[e] = struct{}{}
	return d.route(e, true)
}

// member makes a membership edge dirty for a package in which a
// declaration of the edge's kind, or a subject of its spelling, appeared
// or disappeared, and lists the package. The edge's first package routes
// the edge's validations and annotator invocations.
//
// Error modes: those of [dirtySet.add].
func (d *dirtySet) member(e state.EdgeHash, pkg symbol.Identity) error {
	pkgs, dirty := d.members[e]
	if !dirty {
		pkgs = map[symbol.Identity]struct{}{}
		d.members[e] = pkgs
	}
	pkgs[pkg] = struct{}{}
	if dirty {
		return nil
	}
	d.edges[e] = struct{}{}
	return d.route(e, false)
}

// route reads the records that read an edge, and routes each record that
// the set has not routed whole:
//
//   - a validation's subject joins the subjects to validate again
//   - an annotator invocation joins the pending records of its plugin
//   - a plan's group joins the dirty records of its plan
//   - a plan's invocation joins the dirty records of its plan where plans
//     is set, and is left to its plan otherwise
//
// A record that route leaves to a plan is not routed whole. A group reads
// the edges of names, scopes, directories, files, modules and units, and
// never a membership edge.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (d *dirtySet) route(e state.EdgeHash, plans bool) error {
	refs, err := d.phases.Readers(e)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, done := d.routed[ref]; done {
			continue
		}
		switch ref.Kind {
		case state.RecordValidation:
			d.routed[ref] = struct{}{}
			vs, err := d.phases.Validations(ref)
			if err != nil {
				return err
			}
			for _, v := range vs {
				d.validations[v.Subject] = struct{}{}
			}
		case state.RecordInvocation:
			invs, err := d.phases.Invocations(ref)
			if err != nil {
				return err
			}
			whole := true
			for _, inv := range invs {
				if inv.Plan != "" && !plans {
					whole = false
					continue
				}
				d.join(inv)
			}
			if whole {
				d.routed[ref] = struct{}{}
			}
		case state.RecordGroup:
			d.routed[ref] = struct{}{}
			groups, err := d.phases.Groups(ref)
			if err != nil {
				return err
			}
			for _, g := range groups {
				d.plan(g.Plan).groups[g.Key] = g
			}
		}
	}
	return nil
}

// declaration makes the declaration edge of a subject dirty. Every
// invocation lists the declaration edge of its subject among its reads,
// so the edge also routes the invocations of the subject. A later probe
// of the subject does not read the readers table again.
//
// Error modes: those of [dirtySet.add].
func (d *dirtySet) declaration(id symbol.Identity) error {
	d.probed[id] = struct{}{}
	return d.add(state.DeclarationEdge(id))
}

// probe routes the recorded annotator invocations whose match names a
// subject, and leaves the declaration edge of the subject clean. probe
// routes nothing for a subject that it probed before, or whose
// declaration edge is dirty.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (d *dirtySet) probe(id symbol.Identity) error {
	if _, done := d.probed[id]; done {
		return nil
	}
	d.probed[id] = struct{}{}
	refs, err := d.phases.Readers(state.DeclarationEdge(id))
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref.Kind != state.RecordInvocation {
			continue
		}
		invs, err := d.phases.Invocations(ref)
		if err != nil {
			return err
		}
		for _, inv := range invs {
			if inv.Match.Subject == id {
				d.join(inv)
			}
		}
	}
	return nil
}

// join adds an annotator invocation to the pending records of its
// plugin, and a plan's invocation to the dirty records of its plan.
func (d *dirtySet) join(inv state.Invocation) {
	if inv.Plan != "" {
		d.plan(inv.Plan).invocations[inv.Match] = inv
		return
	}
	byMatch := d.pending[inv.Match.Plugin]
	if byMatch == nil {
		byMatch = map[plugin.MatchKey]state.Invocation{}
		d.pending[inv.Match.Plugin] = byMatch
	}
	byMatch[inv.Match] = inv
}

// take returns the pending records of a plugin in canonical match order,
// and removes them from the set.
func (d *dirtySet) take(p plugin.ID) []state.Invocation {
	byMatch := d.pending[p]
	delete(d.pending, p)
	return slices.SortedFunc(maps.Values(byMatch), func(a, b state.Invocation) int {
		return a.Match.Compare(b.Match)
	})
}
