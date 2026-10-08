// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/pathset"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// claimed is the first plan that routed a file to a path: its name and
// the position a collision with it relates.
type claimed struct {
	plan string
	at   position.Pos
}

// collide reports each pair of plans that route a file to one path,
// or to two paths one tree cannot contain beside each other, under the
// rule every output sink stages by: one path, two paths that differ
// only in case, and a path another needs as a directory. files lists the
// files of the plan of runs at the same index, in path order. The finding
// is at the second plan's file in composition order, with the first
// plan's as related. It reports whether any plans collided, because
// no plan of the run commits then.
func collide(runs []*planRun, files [][]stagedFile, sink *diag.Sink) bool {
	first := map[string]claimed{}
	var tree pathset.Set
	collided := false
	for i, p := range runs {
		for _, f := range files[i] {
			other := f.path
			c, taken := first[other]
			if !taken {
				var clash pathset.Clash
				clash, other = tree.Clash(f.path)
				c, taken = first[other], clash != pathset.ClashNone
			}
			if !taken {
				continue
			}
			collided = true
			sink.Report(diag.Diag{
				Code:     PlanCollision,
				Severity: diag.SeverityError,
				Pos:      f.at,
				Msg: fmt.Sprintf("plans %q and %q route files to %s and %s, which one tree cannot contain",
					c.plan, p.plan.name, other, f.path),
				Origin:  diag.PhaseClose,
				Related: []position.Pos{c.at},
			})
		}
		for _, f := range files[i] {
			if _, taken := first[f.path]; !taken {
				first[f.path] = claimed{plan: p.plan.name, at: f.at}
				tree.Add(f.path)
			}
		}
	}
	return collided
}

// collideWarm reports the collisions of a warm run. It looks up each path
// that a plan routes in the folded keys of the artifacts table. Where a
// path clashes with a file that another plan keeps, it compares every
// plan's routed and kept files in path order, as [collide] compares the
// files of a cold run, each kept file at the position that its artifact
// records. Otherwise it compares the routed files alone, because no kept
// file then clashes with a file. It reports whether any plans collided.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func collideWarm(runs []*planRun, phases *state.PhaseState, sink *diag.Sink) (bool, error) {
	byName := make(map[string]*planRun, len(runs))
	for _, p := range runs {
		byName[p.plan.name] = p
	}
	clashed := false
scan:
	for _, p := range runs {
		for _, f := range p.files {
			found, err := phases.Clashes(f.path)
			if err != nil {
				return false, fmt.Errorf("workspace: read the recorded paths: %w", err)
			}
			for _, c := range found {
				q := byName[c.Plan]
				if q == nil || q == p {
					continue
				}
				if slices.ContainsFunc(q.kept, func(e manifest.Entry) bool { return e.Path == c.Path }) {
					clashed = true
					break scan
				}
			}
		}
	}
	if !clashed {
		return collide(runs, routedFiles(runs), sink), nil
	}
	files := make([][]stagedFile, len(runs))
	for i, p := range runs {
		files[i] = slices.Clone(p.files)
		for _, e := range p.kept {
			a, _, err := phases.Artifact(e.Path)
			if err != nil {
				return false, fmt.Errorf("workspace: read the recorded paths: %w", err)
			}
			files[i] = append(files[i], stagedFile{path: e.Path, at: a.At})
		}
		slices.SortFunc(files[i], func(a, b stagedFile) int { return strings.Compare(a.path, b.path) })
	}
	return collide(runs, files, sink), nil
}

// routedFiles returns the files that each plan of runs routed, at the
// plan's index.
func routedFiles(runs []*planRun) [][]stagedFile {
	out := make([][]stagedFile, len(runs))
	for i, p := range runs {
		out[i] = p.files
	}
	return out
}

// swept is the staging of the outputs of plans the composition no
// longer declares: the sink, what its preparation found, and the
// previous entries it stages for removal. The zero value stages
// nothing.
type swept struct {
	out     output.Sink
	changes []output.Change
	entries []manifest.Entry
	// applies reports a sweep whose commit ran, or would under Dry:
	// the removed plans' entries then leave the record where their
	// files leave the destination.
	applies bool
}

// sweep stages the removal of every previous entry whose plan the
// composition no longer declares, except a path a plan routes a file
// to in this run, into a sink of its own, and prepares it. A stale
// file that drifted or lost the brand's frame is reported and kept. A
// blocked run stages nothing, and neither does a composition without
// output or a run with nothing to remove: each returns the zero sweep.
func (w *Workspace) sweep(rec *record, runs []*planRun, sink *diag.Sink, blocked bool) (*swept, error) {
	sw := &swept{}
	if blocked || w.open == nil {
		return sw, nil
	}
	declared := map[string]bool{}
	routed := map[string]bool{}
	for _, p := range runs {
		declared[p.plan.name] = true
		for _, f := range p.files {
			routed[f.path] = true
		}
	}
	for _, e := range rec.previous.Files {
		if !declared[e.Plan] && !routed[e.Path] {
			sw.entries = append(sw.entries, e)
		}
	}
	if len(sw.entries) == 0 {
		return sw, nil
	}
	out, err := w.openSink()
	if err != nil {
		return sw, fmt.Errorf("workspace: sweep: %w", err)
	}
	for _, e := range sw.entries {
		if derr := out.Delete(e.Path); derr != nil {
			return sw, fmt.Errorf("workspace: sweep: stage the removal of %s: %w", e.Path, discarding(out, derr))
		}
	}
	sw.changes, err = out.Prepare()
	if err != nil {
		return sw, fmt.Errorf("workspace: sweep: prepare the output: %w", discarding(out, err))
	}
	for _, c := range sw.changes {
		kept(sink, c, w.brand)
	}
	sw.out = out
	return sw, nil
}

// audit reports every declaration of a contract's kinds that lacks the
// contract's key, at the declaration's position and at the contract's
// severity, and reports whether any of them is an Error. It reads
// every declaration of a graph the caller handed over, and the
// declarations of the units the load parsed at full depth: a
// dependency's declarations are signatures no annotator is promised to
// stamp. It records each finding into rec under the contract's key and
// the declaration, where rec is set.
func (w *Workspace) audit(
	g *store.Graph, facts *meta.Facts, loaded *load.Report, sink *diag.Sink, rec *state.Recorder,
) bool {
	if len(w.contracts) == 0 {
		return false
	}
	var audited map[string]bool
	if loaded != nil {
		audited = fullDepth(loaded)
	}
	failed := false
	for _, c := range w.contracts {
		present := slices.Collect(facts.ByKey(c.key))
		for _, kind := range c.On {
			for s := range g.ByKind(kind) {
				decl, held := s.(node.Declaration)
				at := s.Position()
				if !held || audited != nil && !audited[at.File] {
					continue
				}
				id := decl.Identity()
				if _, stamped := slices.BinarySearchFunc(present, id, symbol.Identity.Compare); stamped {
					continue
				}
				failed = failed || c.Severity == diag.SeverityError
				d := unmetContract(c, id, at)
				sink.Report(d)
				if rec != nil {
					rec.Audit(c.name, id, d)
				}
			}
		}
	}
	return failed
}

// auditWarm is the audit of a warm run. It evaluates the contracts again
// over the candidates of the shared phases, which are the declarations
// that appeared or changed and the subjects whose facts changed, and
// reports each unmet contract as [Workspace.audit] does. It reports every
// other finding of the generation's audit again, and records it again
// into rec where rec is set, so the commit writes only the rows that
// changed. It drops the finding of a declaration that disappeared. It
// reports whether any finding it reports is an Error.
//
// Error modes: an error wrapping [state.ErrDamaged] for an audit row of
// the generation that does not read whole.
func (w *Workspace) auditWarm(
	g *store.Graph, facts *meta.Facts, loaded *load.Report, phases *state.PhaseState,
	candidates []symbol.Identity, sink *diag.Sink, rec *state.Recorder,
) (bool, error) {
	if len(w.contracts) == 0 {
		return false, nil
	}
	prior, err := phases.Audits()
	if err != nil {
		return false, fmt.Errorf("workspace: read the audit: %w", err)
	}
	again := make(map[symbol.Identity]struct{}, len(candidates)+len(loaded.Changes.Disappeared))
	for _, id := range candidates {
		again[id] = struct{}{}
	}
	for _, id := range loaded.Changes.Disappeared {
		again[id] = struct{}{}
	}
	failed := false
	report := func(key meta.KeyName, id symbol.Identity, d diag.Diag) {
		failed = failed || d.Severity == diag.SeverityError
		sink.Report(d)
		if rec != nil {
			rec.Audit(key, id, d)
		}
	}
	for _, a := range prior {
		if _, evaluated := again[a.Subject]; !evaluated {
			report(a.Key, a.Subject, a.Finding)
		}
	}
	var audited map[string]bool
	for _, c := range w.contracts {
		for _, id := range candidates {
			if !slices.Contains(c.On, id.Kind) {
				continue
			}
			s, held := g.Lookup(id)
			if _, declared := s.(node.Declaration); !held || !declared {
				continue
			}
			if audited == nil {
				audited = fullDepth(loaded)
			}
			at := s.Position()
			if _, present := winner(facts, id, c.key); present || !audited[at.File] {
				continue
			}
			report(c.name, id, unmetContract(c, id, at))
		}
	}
	return failed, nil
}

// check runs the composition's workspace checks over the plans'
// records, one after another in registration order, and reports
// whether any of them reported an Error. Each check reads an index and
// a reader over the whole graph, the facts, and the records of the
// plans it reads: their manifest entries and their exports. A check
// that reads a plan that failed or was cancelled does not run, and the
// run reports one FailedDependency for it at the failed plan's cause,
// where the plan has one. A check's findings arrive in the run's sink
// in the order it reported them. A check's returned error stops the
// step and returns, wrapped with the check's name. Every check it
// calls counts into stats, and records its reader's reads and its
// findings into a lane of rec, where rec is set.
//
// ws is the state of a warm run, and nil on a cold run. A warm run calls
// a check only where [warmState.keptCheck] does not find a record to
// keep. For any other check it reports the recorded findings in their
// recorded order, and the commit keeps the record. check drops the record
// of every check that it calls, and of every check that a failed plan
// blocks.
//
// Error modes: an error wrapping [state.ErrDamaged] for the record of a
// check or the export of a plan that does not read whole, the error of an
// index that does not build, and a check's returned error, wrapped with
// the check's name.
func (w *Workspace) check(
	g *store.Graph, facts *meta.Facts, table plugin.Validated,
	runs []*planRun, sink *diag.Sink, stats *Stats, rec *state.Recorder, ws *warmState,
) (bool, error) {
	var (
		ix   *plugin.Index
		lane *state.Lane
	)
	failed := false
	for _, c := range w.checks {
		if p := blocking(c, runs); p != nil {
			rec.DropCheck(c.name)
			if at, caused := p.cause(); caused {
				sink.Report(diag.Diag{
					Code:     FailedDependency,
					Severity: diag.SeverityInfo,
					Pos:      at,
					Msg: fmt.Sprintf("check %s reads plan %q, which failed, and checks nothing",
						c.name, p.plan.name),
					Origin: diag.PhaseClose,
				})
			}
			continue
		}
		if ws != nil {
			kept, keep, err := ws.keptCheck(c, runs)
			if err != nil {
				return failed, fmt.Errorf("workspace: check %s: %w", c.name, err)
			}
			if keep {
				for _, d := range kept.Findings {
					failed = failed || d.Severity == diag.SeverityError
					sink.Report(d)
				}
				continue
			}
		}
		rec.DropCheck(c.name)
		plans, err := records(c.reads, runs)
		if err != nil {
			return failed, fmt.Errorf("workspace: check %s: %w", c.name, err)
		}
		if ix == nil {
			if ix, err = plugin.NewIndex(g, facts, table, nil); err != nil {
				return failed, fmt.Errorf("workspace: %w", err)
			}
			if rec != nil {
				lane = rec.Lane("")
			}
		}
		reads := store.NewReadSet()
		reader, err := ix.Reader(reads)
		if err != nil {
			return failed, fmt.Errorf("workspace: %w", err)
		}
		local := diag.NewSink()
		stats.Checked++
		err = c.run.Check(&plugin.CheckContext{
			Index:  ix,
			Reader: reader,
			Facts:  facts,
			Sink:   local,
			Rules:  w.rules,
			Kernel: w.kernel,
			Plugin: c.name,
			Plans:  plans,
		})
		lane.Check(c.name, reads, reported(local, sink))
		failed = failed || local.Failed()
		if err != nil {
			return true, fmt.Errorf("workspace: check %s: %w", c.name, err)
		}
	}
	return failed, nil
}

// blocking returns the first plan a check reads that failed or was
// cancelled, and nil where every one of them staged cleanly.
func blocking(c compiledCheck, runs []*planRun) *planRun {
	for _, i := range c.reads {
		if p := runs[i]; p.failed() || p.cancelled {
			return p
		}
	}
	return nil
}

// records returns the records of the plans at the indexes, in their
// order: each plan's manifest entries, those of the files it routed and
// those of the files a warm run kept, sorted by path, and its export.
//
// Error modes: an error wrapping [state.ErrDamaged] for an export that
// reads an artifact of the generation that does not read whole.
func records(at []int, runs []*planRun) ([]plugin.PlanRecord, error) {
	out := make([]plugin.PlanRecord, 0, len(at))
	for _, i := range at {
		p := runs[i]
		export, err := p.export()
		if err != nil {
			return nil, fmt.Errorf("read the export of plan %q: %w", p.plan.name, err)
		}
		files := slices.Concat(p.entries(), p.kept)
		slices.SortFunc(files, func(a, b manifest.Entry) int { return strings.Compare(a.Path, b.Path) })
		out = append(out, plugin.PlanRecord{Name: p.plan.name, Files: files, Export: export})
	}
	return out, nil
}

// fullDepth returns the files of the units that a load parsed at full
// depth, whose declarations the audit evaluates. A dependency's
// declarations are signatures that no annotator is promised to stamp.
func fullDepth(loaded *load.Report) map[string]bool {
	out := map[string]bool{}
	for _, u := range loaded.Units {
		if u.Depth == plugin.DepthFull {
			for _, f := range u.Files {
				out[f.Path] = true
			}
		}
	}
	return out
}

// unmetContract returns the finding of a declaration that lacks a
// contract's key, at the declaration's position and at the contract's
// severity.
func unmetContract(c contract, id symbol.Identity, at position.Pos) diag.Diag {
	return diag.Diag{
		Code:     UnmetContract,
		Severity: c.Severity,
		Pos:      at,
		Msg: fmt.Sprintf("%s %s lacks %s, whose contract promises it on every %s by the end of %s",
			id.Kind, id.Name, c.name, kinds(c.On), c.By),
		Origin: diag.PhaseClose,
	}
}

// kinds spells a contract's kinds in a finding.
func kinds(ks []symbol.Kind) string {
	spelled := make([]string, len(ks))
	for i, k := range ks {
		spelled[i] = k.String()
	}
	return strings.Join(spelled, " and ")
}

// keptCheck returns the generation's record of a check that the warm run
// keeps, and false for a check that the run calls again:
//
//   - The run found a change in the graph or the facts.
//   - A plan that the check reads rendered or removed a file, or changed
//     its export.
//   - The generation has no record of the check.
//
// A check reads through an index and a fact store that do not record its
// reads, as a plugin that implements its role directly does, so any change
// of the graph or the facts calls it again.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that does
// not read whole.
func (ws *warmState) keptCheck(c compiledCheck, runs []*planRun) (state.Check, bool, error) {
	if ws.changed() {
		return state.Check{}, false, nil
	}
	for _, i := range c.reads {
		if p := runs[i]; len(p.files) > 0 || len(p.stale) > 0 || p.exportChanged {
			return state.Check{}, false, nil
		}
	}
	return ws.phases.Check(c.name)
}
