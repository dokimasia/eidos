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
// phase call executes the invocation again or removes it. A plan's
// invocation runs again with its plan, and a check runs again in Close,
// so neither joins the set.
//
// A probe of a candidate routes the annotator invocations of the
// candidate and leaves its declaration edge clean. The run withdraws the
// claims of these invocations before the call evaluates the candidate
// again.
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
	// routed lists the records that the set has read, so it reads each
	// record once.
	routed map[state.RecordRef]struct{}
	// validations lists the subjects whose recorded validation read a
	// dirty edge.
	validations map[symbol.Identity]struct{}
	// pending lists the recorded annotator invocations that a dirty edge
	// or a probe routed, by plugin and then by match.
	pending map[plugin.ID]map[plugin.MatchKey]state.Invocation
	// probed lists the subjects whose invocations already joined pending.
	probed map[symbol.Identity]struct{}
}

// newDirtySet returns an empty dirty set over a generation's record of
// the phases.
func newDirtySet(phases *state.PhaseState) *dirtySet {
	return &dirtySet{
		phases:      phases,
		edges:       map[state.EdgeHash]struct{}{},
		routed:      map[state.RecordRef]struct{}{},
		validations: map[symbol.Identity]struct{}{},
		pending:     map[plugin.ID]map[plugin.MatchKey]state.Invocation{},
		probed:      map[symbol.Identity]struct{}{},
	}
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
	refs, err := d.phases.Readers(e)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, done := d.routed[ref]; done {
			continue
		}
		d.routed[ref] = struct{}{}
		switch ref.Kind {
		case state.RecordValidation:
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
			for _, inv := range invs {
				d.join(inv)
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
// plugin. It ignores the invocation of a plan.
func (d *dirtySet) join(inv state.Invocation) {
	if inv.Plan != "" {
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
