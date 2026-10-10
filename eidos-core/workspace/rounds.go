// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// hashPrefix opens the digest a manifest entry states.
const hashPrefix = "sha256:"

// warmState is what the plans of a warm run read of the run so far: the
// generation's record of the phases, the dirty set that the shared phases
// left, the subjects whose matches may have changed, the generation's
// admission of each package whose files or module fact changed, the
// workspace files that the load found moved or vanished, the run's tree,
// and the previous record's entries of each plan. A cold run has none.
type warmState struct {
	phases     *state.PhaseState
	dirty      *dirtySet
	candidates []symbol.Identity
	prior      []admission
	// moved and vanished are the load's moved and vanished files, each
	// sorted by path.
	moved, vanished []string
	tree            fs.FS
	previous        map[string][]manifest.Entry
}

// changed reports whether the shared phases found any change: a dirty
// edge, or a subject whose matches may have changed.
func (ws *warmState) changed() bool { return len(ws.dirty.edges) > 0 || len(ws.candidates) > 0 }

// warmPlan is one plan of a warm run that executes in part: its dirty
// groups and invocations as the generation records them, the files of
// those groups, the edges the plan made dirty, and the records it read.
//
// The plan executes in rounds. Each round starts from a fresh store, runs
// the contributors of the dirty groups and the dirty invocations again,
// and evaluates the candidates. A round that finds another group dirty
// adds it and starts again. The dirty groups only grow, so the plan runs
// at most one round for each of its groups. Each of these makes a group
// dirty:
//
//   - a dirty or new invocation that touched one of its units, and a
//     listed invocation that no longer runs and touched one. The call of a
//     generator that journals no invocation of its own is one invocation
//     under [plugin.WholeCall], which is dirty whenever the run found a
//     change.
//   - a name that appeared, disappeared or settled apart in a scope where
//     the group declares a name, or that the group read
//   - a new path equal to the path of one of its files, or one that one
//     tree cannot contain beside one of them
//
// # Concurrency
//
// A warmPlan is not safe for concurrent use. The plan's goroutine runs it.
type warmPlan struct {
	w  *Workspace
	ws *warmState
	p  *planRun
	ix *plugin.Index
	// scope is the plan's scope over the run, nil where its sources admit
	// every package.
	scope store.Scope
	// groups are the dirty groups by key unit, and files the paths of their
	// files, each as the generation records it.
	groups map[plugin.UnitRef]state.Group
	files  map[string]struct{}
	// invocations are the dirty invocations by match, as the generation
	// records them.
	invocations map[plugin.MatchKey]state.Invocation
	// edges are the edges that the plan made dirty, and artifacts the
	// artifacts that it read, by path.
	edges     map[state.EdgeHash]struct{}
	artifacts map[string]artifactRead
}

// artifactRead is one artifact that a warm plan read, and whether the
// generation records it.
type artifactRead struct {
	a    state.Artifact
	held bool
}

// runWarm runs one plan of a warm run. A plan whose work the generation
// records as pending runs whole, and so does a plan whose scope decides a
// package otherwise than in the generation, as [Sources.rebinds] states.
// Any other plan executes in part: the groups that the run's changes make
// dirty execute again, and the plan keeps its other groups as the
// generation records them. It returns the stamped files of the groups it
// executed, and for a plan that runs whole, the routed files that
// rendered, as [Workspace.runPlan] does.
//
// Error modes: those of [Workspace.runPlan], and an error wrapping
// [state.ErrDamaged] for a record of the generation that does not read
// whole.
func (w *Workspace) runWarm(
	ctx context.Context, g *store.Graph, facts *meta.Facts, table plugin.Validated, src tree, p *planRun,
	deps []*planRun, rec *state.Recorder, ws *warmState,
) ([]stagedFile, []plugin.File, error) {
	pl := p.plan
	pending, err := ws.phases.Pending(pl.name)
	if err != nil {
		return nil, nil, fmt.Errorf("read the record of the phases: %w", err)
	}
	scope := pl.sources.bind(g, facts, w.kernel)
	if pending || pl.sources.rebinds(scope, ws.prior) {
		p.exportChanged = true
		return w.runPlan(ctx, g, facts, table, src, p, deps, rec)
	}
	ix, err := plugin.NewIndex(g, facts, table, scope)
	if err != nil {
		return nil, nil, err
	}
	wp := &warmPlan{
		w: w, ws: ws, p: p, ix: ix, scope: scope,
		groups:      map[plugin.UnitRef]state.Group{},
		files:       map[string]struct{}{},
		invocations: map[plugin.MatchKey]state.Invocation{},
		edges:       map[state.EdgeHash]struct{}{},
		artifacts:   map[string]artifactRead{},
	}
	p.selective, p.lane = true, rec.Lane(pl.name)
	rec.KeepPlan(pl.name)
	staged, err := wp.run(ctx, facts, src, deps, rec)
	if errors.Is(err, state.ErrDamaged) {
		err = fmt.Errorf("read the record of the phases: %w", err)
	}
	return staged, nil, err
}

// run seeds the plan's dirty groups, executes them in rounds, and
// records what the last round executed. It returns the stamped files of
// the dirty groups. A plan without a dirty group, a dirty invocation or a
// candidate executes nothing and keeps every file. Such a plan does not
// read the exports of the plans it depends on.
func (wp *warmPlan) run(
	ctx context.Context, facts *meta.Facts, src tree, deps []*planRun, rec *state.Recorder,
) ([]stagedFile, error) {
	p, pl := wp.p, wp.p.plan
	if err := wp.seed(deps); err != nil {
		return nil, err
	}
	if len(wp.groups) == 0 && len(wp.invocations) == 0 && len(wp.ws.candidates) == 0 {
		return nil, wp.keep(nil, nil, nil, rec)
	}
	exports, err := exportsOf(deps)
	if err != nil {
		return nil, err
	}
	for {
		p.emit, p.sink, p.invoked = plugin.NewEmit(), p.policy.sink(), nil
		p.lane.Reset()
		touches := &touchJournal{next: p.lane}
		sel := wp.selection()
		if err := wp.w.generate(ctx, wp.ix, facts, p, exports, sel, touches); err != nil {
			return nil, err
		}
		if grew, err := wp.placed(touches.touches, sel.Matches); err != nil || grew {
			if err != nil {
				return nil, err
			}
			continue
		}
		filtered, err := wp.filter(p.emit)
		if err != nil {
			return nil, err
		}
		grew, err := wp.renamed(filtered)
		if err != nil {
			return nil, err
		}
		if grew {
			continue
		}
		names := wp.ws.phases.Names(pl.name, func(file string) bool {
			_, again := wp.files[file]
			return again
		})
		settled, err := plugin.SettleWith(filtered, pl.backend, facts, p.sink, names)
		if err != nil {
			return nil, fmt.Errorf("settle: %w", err)
		}
		staged, rendered, err := wp.w.write(pl, wp.ix, src, filtered, p.sink, names)
		if err == nil {
			err = names.Err()
		}
		if err != nil {
			return nil, err
		}
		if grew, err = wp.respelled(rendered, filtered, names); err != nil {
			return nil, err
		}
		if grew {
			continue
		}
		p.emit = filtered
		if !p.sink.Failed() {
			_, placed := pl.backend.(plugin.Packager)
			pg := planGroups{
				plan: pl.name, emit: filtered, files: rendered, staged: staged,
				touches: touches.touches, settled: settled, others: names, placed: placed,
			}
			pg.record(p.lane)
		}
		for _, m := range sel.Matches {
			rec.DropInvocation(pl.name, m)
		}
		for _, t := range touches.touches {
			rec.DropInvocation(pl.name, t.match)
		}
		return staged, wp.keep(rendered, filtered, names, rec)
	}
}

// seed finds the plan's first dirty invocations and groups:
//
//   - those that the shared phases' dirty edges or candidates routed
//   - each invocation that read a membership edge whose membership changed
//     in a package that the plan's scope admits
//   - each invocation that read the export of a plan it depends on whose
//     export changed
//   - the call of each generator that journaled no invocation of its own,
//     where the run found a change
//   - the groups of every dirty invocation
//   - the group of each file that changed on disk since the commit that
//     wrote it
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole, and the error of a moved file that does not read.
func (wp *warmPlan) seed(deps []*planRun) error {
	name := wp.p.plan.name
	if d := wp.ws.dirty.plans[name]; d != nil {
		for _, g := range d.groups {
			wp.addGroup(g)
		}
		for _, inv := range d.invocations {
			wp.invocations[inv.Match] = inv
		}
	}
	members := wp.ws.dirty.members
	for _, e := range slices.Sorted(maps.Keys(members)) {
		for pkg := range members[e] {
			if wp.scope != nil && !wp.scope(pkg) {
				continue
			}
			if _, err := wp.dirtyEdge(e); err != nil {
				return err
			}
			break
		}
	}
	exported := false
	for _, d := range deps {
		if !d.exportChanged {
			continue
		}
		exported = true
		if _, err := wp.dirtyEdge(state.ExportEdge(d.plan.name)); err != nil {
			return err
		}
	}
	if wp.ws.changed() || exported {
		for _, s := range wp.p.plan.entries {
			inv, whole, err := wp.ws.phases.Invocation(name, plugin.MatchKey{Plugin: s.name, Rule: plugin.WholeCall})
			if err != nil {
				return err
			}
			if whole {
				wp.invocations[inv.Match] = inv
			}
		}
	}
	for _, inv := range wp.invocations {
		if _, err := wp.groupsOf(inv); err != nil {
			return err
		}
	}
	for _, e := range wp.ws.previous[name] {
		if _, err := wp.changedOnDisk(e); err != nil {
			return err
		}
	}
	return nil
}

// changedOnDisk makes the group of a file of the plan dirty where the
// file vanished, or where it moved and its bytes differ from the digest
// that the plan's commit wrote. It reports whether the file's group
// joined the dirty groups. A file without an artifact in the generation
// is a stale file that a removal kept, and the plan's staging removes it
// again.
//
// Error modes: an error wrapping [state.ErrDamaged] for an artifact that
// does not read whole, and the error of a moved file that does not read.
func (wp *warmPlan) changedOnDisk(e manifest.Entry) (bool, error) {
	_, vanished := slices.BinarySearch(wp.ws.vanished, e.Path)
	_, moved := slices.BinarySearch(wp.ws.moved, e.Path)
	if !vanished && !moved {
		return false, nil
	}
	if moved {
		b, err := plugin.ReadFile(wp.ws.tree, e.Path)
		if err != nil {
			return false, fmt.Errorf("workspace: read the generated file %s: %w", e.Path, err)
		}
		sum := sha256.Sum256(b)
		if hashPrefix+hex.EncodeToString(sum[:]) == e.Hash {
			return false, nil
		}
	}
	return wp.groupOfFile(e.Path)
}

// groupOfFile makes the group of a file of the plan dirty, and reports
// whether the group joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) groupOfFile(path string) (bool, error) {
	a, err := wp.artifact(path)
	if err != nil || !a.held || a.a.Entry.Plan != wp.p.plan.name {
		return false, err
	}
	g, held, err := wp.ws.phases.Group(wp.p.plan.name, a.a.Group)
	if err != nil || !held {
		return false, err
	}
	return wp.addGroup(g), nil
}

// artifact returns the generation's artifact of a path, which it reads
// once.
//
// Error modes: an error wrapping [state.ErrDamaged] for an artifact that
// does not read whole.
func (wp *warmPlan) artifact(path string) (artifactRead, error) {
	if a, read := wp.artifacts[path]; read {
		return a, nil
	}
	a, held, err := wp.ws.phases.Artifact(path)
	if err != nil {
		return artifactRead{}, err
	}
	wp.artifacts[path] = artifactRead{a: a, held: held}
	return wp.artifacts[path], nil
}

// addGroup adds a group of the plan to the dirty groups, with its files,
// and reports whether it was not dirty before.
func (wp *warmPlan) addGroup(g state.Group) bool {
	if _, dirty := wp.groups[g.Key]; dirty {
		return false
	}
	wp.groups[g.Key] = g
	for _, f := range g.Files {
		wp.files[f] = struct{}{}
	}
	return true
}

// groupsOf makes dirty the groups of each unit that a recorded invocation
// touched, and reports whether a group joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) groupsOf(inv state.Invocation) (bool, error) {
	grew := false
	units := slices.Concat(inv.Units, []plugin.UnitRef{inv.Match.Host.Unit})
	for _, h := range inv.Hosts {
		units = append(units, h.Unit)
	}
	for _, u := range units {
		joined, err := wp.unitGroups(u)
		if err != nil {
			return grew, err
		}
		grew = grew || joined
	}
	return grew, nil
}

// unitGroups makes dirty the recorded groups of the plan that contain a
// unit, and reports whether a group joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) unitGroups(u plugin.UnitRef) (bool, error) {
	if u == (plugin.UnitRef{}) {
		return false, nil
	}
	groups, err := wp.recordedGroups(u)
	grew := false
	for _, g := range groups {
		grew = wp.addGroup(g) || grew
	}
	return grew, err
}

// recordedGroups returns the recorded groups of the plan that contain a
// unit: the readers of the unit's edge, which only a group reads.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) recordedGroups(u plugin.UnitRef) ([]state.Group, error) {
	name := wp.p.plan.name
	refs, err := wp.ws.phases.Readers(state.UnitEdge(name, u))
	if err != nil {
		return nil, err
	}
	var out []state.Group
	for _, ref := range refs {
		groups, err := wp.ws.phases.Groups(ref)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.Plan == name && slices.Contains(g.Units, u) {
				out = append(out, g)
			}
		}
	}
	return out, nil
}

// dirtyEdge makes an edge dirty for the plan: each group of the plan
// among the edge's readers joins the dirty groups, and each invocation of
// the plan among them joins the dirty invocations with its groups. It
// reports whether a group joined the dirty groups. An edge that is
// already dirty routes nothing.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) dirtyEdge(e state.EdgeHash) (bool, error) {
	if _, dirty := wp.edges[e]; dirty {
		return false, nil
	}
	wp.edges[e] = struct{}{}
	name := wp.p.plan.name
	refs, err := wp.ws.phases.Readers(e)
	if err != nil {
		return false, err
	}
	grew := false
	for _, ref := range refs {
		switch ref.Kind {
		case state.RecordGroup:
			groups, err := wp.ws.phases.Groups(ref)
			if err != nil {
				return grew, err
			}
			for _, g := range groups {
				if g.Plan == name {
					grew = wp.addGroup(g) || grew
				}
			}
		case state.RecordInvocation:
			invs, err := wp.ws.phases.Invocations(ref)
			if err != nil {
				return grew, err
			}
			for _, inv := range invs {
				if inv.Plan != name {
					continue
				}
				wp.invocations[inv.Match] = inv
				joined, err := wp.groupsOf(inv)
				if err != nil {
					return grew, err
				}
				grew = grew || joined
			}
		case state.RecordValidation, state.RecordCheck:
			// The run routes the validations before the plans, and the close
			// step decides on each check.
		}
	}
	return grew, nil
}

// selection returns the round's selection: the contributors of the dirty
// groups and the dirty invocations, sorted in canonical match order, and
// the candidates.
func (wp *warmPlan) selection() *plugin.Selection {
	listed := map[plugin.MatchKey]struct{}{}
	for _, g := range wp.groups {
		for _, m := range g.Contributors {
			listed[m] = struct{}{}
		}
	}
	for m := range wp.invocations {
		listed[m] = struct{}{}
	}
	return &plugin.Selection{
		Matches:    slices.SortedFunc(maps.Keys(listed), plugin.MatchKey.Compare),
		Candidates: wp.ws.candidates,
	}
}

// placed makes dirty the groups that the round's invocations newly touch:
// each unit that a dirty or new invocation touched, and each unit that a
// listed invocation touched in the generation where it no longer runs. A
// clean invocation touches the units it touched before. It reports
// whether a group joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) placed(touches []touch, listed []plugin.MatchKey) (bool, error) {
	name := wp.p.plan.name
	grew := false
	ran := make(map[plugin.MatchKey]struct{}, len(touches))
	for _, t := range touches {
		ran[t.match] = struct{}{}
		if _, dirty := wp.invocations[t.match]; !dirty {
			_, recorded, err := wp.ws.phases.Invocation(name, t.match)
			if err != nil {
				return grew, err
			}
			if recorded {
				continue
			}
		}
		for _, u := range append(slices.Clip(t.units), t.match.Host.Unit) {
			joined, err := wp.unitGroups(u)
			if err != nil {
				return grew, err
			}
			grew = grew || joined
		}
	}
	for _, m := range listed {
		if _, runs := ran[m]; runs {
			continue
		}
		inv, recorded := wp.invocations[m]
		if !recorded {
			var err error
			if inv, recorded, err = wp.ws.phases.Invocation(name, m); err != nil {
				return grew, err
			}
		}
		if !recorded {
			continue
		}
		joined, err := wp.groupsOf(inv)
		if err != nil {
			return grew, err
		}
		grew = grew || joined
	}
	return grew, nil
}

// filter returns a store of the round's units that belong to a dirty
// group, and of the units outside every recorded group, which a dirty or
// new invocation created. A unit of a clean group received only the
// contributions of clean invocations that ran for a dirty group. Those
// contributions are unchanged, so the plan leaves the unit out.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole, and the error of a unit the store refuses.
func (wp *warmPlan) filter(e *plugin.Emit) (*plugin.Emit, error) {
	dirty := map[plugin.UnitRef]struct{}{}
	for _, g := range wp.groups {
		for _, u := range g.Units {
			dirty[u] = struct{}{}
		}
	}
	out := plugin.NewEmit()
	for u := range e.Units() {
		ref := u.Ref()
		if _, in := dirty[ref]; !in {
			groups, err := wp.recordedGroups(ref)
			if err != nil {
				return nil, err
			}
			if len(groups) > 0 {
				continue
			}
		}
		if err := out.Add(u); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// emittedName is one file-level name as a scope compares it: the package
// and receiver of its scope, its emitted name, its origin and its kind.
type emittedName struct {
	pkg      string
	receiver string
	emitted  string
	origin   symbol.Identity
	kind     symbol.Kind
}

// nameOf returns the emitted name of a name entry.
func nameOf(e plugin.NameEntry) emittedName {
	return emittedName{pkg: e.Package, receiver: e.Receiver, emitted: e.Emitted, origin: e.Origin, kind: e.Kind}
}

// renamed compares the file-level names that the round's store emits with
// the names that the generation records for the files of the dirty
// groups, before the settle. Each name that appeared or disappeared makes
// its scope and its entries dirty. It reports whether a group joined the
// dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) renamed(e *plugin.Emit) (bool, error) {
	counts := map[emittedName]int{}
	for u := range e.Units() {
		for _, d := range u.Decls {
			_ = emit.RespellNames(d, func(
				host, carrier symbol.Symbol, kind symbol.Kind, _ symbol.Visibility, name string,
			) (string, error) {
				if host != nil {
					return name, nil
				}
				n := emittedName{pkg: u.Pkg.Package, emitted: name, kind: kind}
				n.origin, _ = emit.OriginOf(carrier)
				if m, attached := carrier.(*emit.Method); attached && m.Receives != nil {
					n.receiver = m.Receives.Spelling
				}
				counts[n]++
				return name, nil
			})
		}
	}
	prior, err := wp.priorNames()
	if err != nil {
		return false, err
	}
	for _, entry := range prior {
		counts[nameOf(entry)]--
	}
	grew := false
	for _, n := range slices.SortedFunc(maps.Keys(counts), compareEmitted) {
		if counts[n] == 0 {
			continue
		}
		joined, err := wp.nameChanged(n)
		if err != nil {
			return grew, err
		}
		grew = grew || joined
	}
	return grew, nil
}

// respelled compares the names that the round's routed files declare with
// the names that the generation records for the files of the dirty
// groups, after the settle and the layout: a name that settled apart, or
// moved to another file or package, makes its scope and its entries
// dirty. It also makes dirty the group of each recorded file of the plan
// that a new path equals or clashes with. It reports whether a group
// joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) respelled(files []plugin.File, settled *plugin.Emit, others plugin.Names) (bool, error) {
	grew := false
	for _, f := range files {
		clashes, err := wp.ws.phases.Clashes(f.Path)
		if err != nil {
			return grew, err
		}
		for _, c := range clashes {
			if _, again := wp.files[c.Path]; again || c.Plan != wp.p.plan.name {
				continue
			}
			joined, err := wp.groupOfFile(c.Path)
			if err != nil {
				return grew, err
			}
			grew = grew || joined
		}
	}
	type spelling struct {
		settled, file string
		pkg           symbol.Identity
	}
	spelled := map[emittedName][]spelling{}
	for _, e := range plugin.NamesOf(files, settled, others) {
		spelled[nameOf(e)] = append(spelled[nameOf(e)], spelling{settled: e.Settled, file: e.File, pkg: e.FilePkg})
	}
	prior, err := wp.priorNames()
	if err != nil {
		return grew, err
	}
	changed := map[emittedName]struct{}{}
	for _, e := range prior {
		n := nameOf(e)
		was := spelling{settled: e.Settled, file: e.File, pkg: e.FilePkg}
		at := slices.Index(spelled[n], was)
		if at < 0 {
			changed[n] = struct{}{}
			continue
		}
		spelled[n] = slices.Delete(spelled[n], at, at+1)
	}
	for n, rest := range spelled {
		if len(rest) > 0 {
			changed[n] = struct{}{}
		}
	}
	for _, n := range slices.SortedFunc(maps.Keys(changed), compareEmitted) {
		joined, err := wp.nameChanged(n)
		if err != nil {
			return grew, err
		}
		grew = grew || joined
	}
	return grew, nil
}

// priorNames returns the names that the generation records for the files
// of the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for an artifact that
// does not read whole.
func (wp *warmPlan) priorNames() ([]plugin.NameEntry, error) {
	var out []plugin.NameEntry
	for _, f := range slices.Sorted(maps.Keys(wp.files)) {
		a, err := wp.artifact(f)
		if err != nil {
			return nil, err
		}
		out = append(out, a.a.Names...)
	}
	return out, nil
}

// nameChanged makes dirty the scope of a name and the entries under which
// a reference looks it up: by its package for a declaration that is not
// a method, and by its origin for a declaration with one. It reports
// whether a group joined the dirty groups.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) nameChanged(n emittedName) (bool, error) {
	name := wp.p.plan.name
	edges := []state.EdgeHash{
		state.ScopeEdge(name, n.pkg, n.receiver),
		state.NameEdge(name, plugin.NameKey{Package: n.pkg, Emitted: n.emitted}),
	}
	if !n.origin.IsZero() {
		edges = append(edges, state.NameEdge(name, plugin.NameKey{Origin: n.origin, Emitted: n.emitted}))
	}
	grew := false
	for _, e := range edges {
		joined, err := wp.dirtyEdge(e)
		if err != nil {
			return grew, err
		}
		grew = grew || joined
	}
	return grew, nil
}

// keep completes the plan's run. It drops the generation's records of the
// dirty groups, of their files and of the files that they render now, so
// the commit reads the prior rows of each path that the plan writes. It
// lists the plan's files that the run keeps: each previous file outside the
// dirty groups that the generation records. For a plan marked exported, it
// reports whether the export changed, from the export rows of the files
// that it rendered and the rows that the generation records for the
// previous files of the dirty groups. It sets the plan's export, which
// reads the kept files' rows from the generation on its first call. It
// reports the findings of the plan's records that the run keeps. rendered
// are the routed files that the last round rendered, over the settled
// store, and others the names of the kept files. A plan that executed
// nothing passes none of them.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) keep(rendered []plugin.File, settled *plugin.Emit, others plugin.Names, rec *state.Recorder) error {
	p, name := wp.p, wp.p.plan.name
	for k := range wp.groups {
		rec.DropGroup(name, k)
	}
	for f := range wp.files {
		rec.DropFile(f)
	}
	for _, f := range rendered {
		rec.DropFile(f.Path)
	}
	var before []plugin.ExportedSymbol
	for _, e := range wp.ws.previous[name] {
		if _, again := wp.files[e.Path]; again {
			if p.plan.exported {
				a, err := wp.artifact(e.Path)
				if err != nil {
					return err
				}
				before = append(before, a.a.Export...)
			}
			continue
		}
		recorded, err := wp.ws.phases.Recorded(e.Path)
		if err != nil {
			return err
		}
		if recorded {
			p.kept = append(p.kept, e)
		}
	}
	if p.plan.exported {
		fresh := plugin.NewExport(name, rendered, settled, others).Symbols
		phases, kept := wp.ws.phases, p.kept
		p.fresh, p.exportChanged = fresh, !slices.Equal(sortedSymbols(before), fresh)
		p.export = sync.OnceValues(func() (plugin.ExportDoc, error) {
			rows := slices.Clone(fresh)
			for _, e := range kept {
				a, _, err := phases.Artifact(e.Path)
				if err != nil {
					return plugin.ExportDoc{}, err
				}
				rows = append(rows, a.Export...)
			}
			return plugin.ExportDoc{Plan: name, Symbols: sortedSymbols(rows)}, nil
		})
	}
	return wp.replay()
}

// replay reports into the plan's sink the findings of each of the plan's
// records that the run keeps: an invocation that it did not run again,
// and a group that is not dirty.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (wp *warmPlan) replay() error {
	name := wp.p.plan.name
	refs, err := wp.ws.phases.Readers(state.FindingsEdge)
	if err != nil {
		return err
	}
	ran := map[plugin.MatchKey]struct{}{}
	for _, m := range wp.selection().Matches {
		ran[m] = struct{}{}
	}
	for _, ref := range refs {
		switch ref.Kind {
		case state.RecordInvocation:
			invs, err := wp.ws.phases.Invocations(ref)
			if err != nil {
				return err
			}
			for _, inv := range invs {
				if _, again := ran[inv.Match]; inv.Plan == name && !again {
					wp.report(inv.Findings)
				}
			}
		case state.RecordGroup:
			groups, err := wp.ws.phases.Groups(ref)
			if err != nil {
				return err
			}
			for _, g := range groups {
				if _, again := wp.groups[g.Key]; g.Plan == name && !again {
					wp.report(g.Findings)
				}
			}
		case state.RecordValidation, state.RecordCheck:
			// The run replays the validations, and the close step reports
			// the findings of each check that it keeps.
		}
	}
	return nil
}

// report reports the findings of a kept record into the plan's sink.
func (wp *warmPlan) report(found []diag.Diag) {
	for _, d := range found {
		wp.p.sink.Report(d)
	}
}

// sortedSymbols sorts export rows in place in the order an export lists
// them, by key, then by file, and returns them.
func sortedSymbols(rows []plugin.ExportedSymbol) []plugin.ExportedSymbol {
	slices.SortFunc(rows, func(a, b plugin.ExportedSymbol) int {
		return cmp.Or(a.Compare(b.ExportKey), strings.Compare(a.File, b.File))
	})
	return rows
}

// compareEmitted orders two emitted names by package, receiver, emitted
// name, origin and kind.
func compareEmitted(a, b emittedName) int {
	return cmp.Or(
		strings.Compare(a.pkg, b.pkg),
		strings.Compare(a.receiver, b.receiver),
		strings.Compare(a.emitted, b.emitted),
		a.origin.Compare(b.origin),
		cmp.Compare(a.kind, b.kind),
	)
}
