// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// commitAll commits each plan that can, in the composition's commit
// order, then the sweep, and records each plan's status, changes and
// withheld changes in the report, in composition order. A skipped plan
// reports [PlanSkipped] and nothing else. A blocked run commits nothing,
// and neither does a dry one. A plan commits only where every plan it
// depends on commits, or would under Dry. One that does not fails, and
// unless the whole run is blocked, the run reports one FailedDependency
// for it at the failed plan's cause. A cancellation observed before a
// plan's commit skips that commit and every later one, the sweep's
// included, and the commit of every plan that depends on a skipped
// one. A plan's commit runs to its end once begun, and one that fails
// part-way fails its plan and keeps the files it wrote in its changes.
// It returns the sweep's commit error.
func commitAll(
	ctx context.Context, runs []*planRun, order []int, sw *swept, blocked, dry bool, report *Report,
) error {
	plans := make([]PlanReport, len(runs))
	cancelled := false
	for _, i := range order {
		p := runs[i]
		upstream := p.upstream
		if upstream == nil {
			upstream = uncommitted(p, runs)
		}
		switch {
		case p.skipped:
			p.status = PlanSkipped
		case p.cancelled:
			p.status = PlanCancelled
		case blocked:
			p.status = PlanFailed
		case upstream != nil && upstream.status == PlanCancelled:
			p.status = PlanCancelled
		case upstream != nil:
			p.status = PlanFailed
			failedDependency(report.Sink, p, upstream)
		case p.failed():
			p.status = PlanFailed
		case dry:
			p.status = PlanPrepared
		case cancelled || ctx.Err() != nil:
			cancelled = true
			p.status = PlanCancelled
		default:
			p.status = PlanCommitted
		}
		reached := p.finish()
		pr := PlanReport{Name: p.plan.name, Status: p.status, Refused: p.refused}
		if reached || p.status == PlanPrepared {
			pr.Changes, pr.Withheld = p.changes, p.withheld
		}
		plans[i] = pr
	}
	report.Plans = append(report.Plans, plans...)
	if sw.out == nil {
		return nil
	}
	if blocked || dry || cancelled || ctx.Err() != nil {
		sw.applies = dry
		if err := sw.out.Discard(); err != nil {
			return fmt.Errorf("workspace: sweep: discard the output: %w", err)
		}
		return nil
	}
	records, err := sw.out.Commit()
	sw.changes, sw.applies = reconcile(sw.changes, records), true
	for _, r := range records {
		if r.Action == output.ActionDeleted {
			report.Swept = append(report.Swept, r)
		}
	}
	if err != nil {
		return fmt.Errorf("workspace: sweep: commit the output: %w", err)
	}
	return nil
}

// uncommitted returns the first plan p depends on that does not commit,
// or would not under Dry, and nil where every one of them does. The
// commit order places each of them before p, so each has its status.
func uncommitted(p *planRun, runs []*planRun) *planRun {
	for _, d := range p.plan.deps {
		if !runs[d].commits() {
			return runs[d]
		}
	}
	return nil
}

// failedDependency reports that a plan commits nothing because a plan
// it depends on failed, at that plan's cause, and reports nothing where
// that plan failed on a returned error alone.
func failedDependency(sink *diag.Sink, p, upstream *planRun) {
	at, caused := upstream.cause()
	if !caused {
		return
	}
	sink.Report(diag.Diag{
		Code:     FailedDependency,
		Severity: diag.SeverityInfo,
		Pos:      at,
		Msg: fmt.Sprintf("plan %q depends on plan %q, which failed, and commits nothing",
			p.plan.name, upstream.plan.name),
		Origin: diag.PhaseClose,
	})
}

// finish commits the plan's staging where its status is committed and
// discards it otherwise, and reports whether the commit ran. A commit
// that fails part-way fails the plan, and its changes keep only the
// paths the commit wrote.
func (p *planRun) finish() bool {
	if p.out == nil {
		return false
	}
	out := p.out
	p.out = nil
	if p.status != PlanCommitted {
		if err := out.Discard(); err != nil {
			p.err = fmt.Errorf("discard the output: %w", err)
		}
		return false
	}
	records, err := out.Commit()
	p.changes = reconcile(p.changes, records)
	if err != nil {
		p.err = fmt.Errorf("commit the output: %w", err)
		p.status = PlanFailed
	}
	return true
}

// reconcile returns the changes a commit made: each staged path with
// the action and the digest its record states, a staged removal the
// commit left in place as unchanged, and no entry for a staged write
// the commit refused. A write's change always states a digest and a
// removal's never does, which tells the two apart.
func reconcile(changes []output.Change, records []output.Written) []output.Change {
	byPath := make(map[string]output.Written, len(records))
	for _, r := range records {
		byPath[r.Path] = r
	}
	out := make([]output.Change, 0, len(changes))
	for _, c := range changes {
		r, wrote := byPath[c.Path]
		switch {
		case wrote:
			c.Action, c.Hash = r.Action, r.Hash
		case c.Hash == "":
			c.Action = output.ActionUnchanged
		default:
			continue
		}
		out = append(out, c)
	}
	return out
}

// merged returns the run's record: per plan that commits, or would
// under Dry, the files it routed, the files a warm run kept, and the
// stale outputs it kept because they drifted or outlived a removal, per
// plan that does not, its previous entries, and per plan the composition
// no longer declares, the entries whose files the sweep did not remove.
// A path a committing plan routes a file to takes that plan's entry
// whatever another plan's previous entry listed there. A path whose
// change the run withheld keeps the previous record's entry, and has none
// where the previous record has none. It names the workspace the run
// records under.
func merged(rec *record, runs []*planRun, sw *swept) manifest.Manifest {
	entries := map[string]manifest.Entry{}
	keep := func(es ...manifest.Entry) {
		for _, e := range es {
			entries[e.Path] = e
		}
	}
	declared := map[string]bool{}
	for _, p := range runs {
		declared[p.plan.name] = true
	}
	if sw.applies {
		keep(survivors(sw.entries, sw.changes)...)
	} else {
		for _, e := range rec.previous.Files {
			if !declared[e.Plan] {
				keep(e)
			}
		}
	}
	for _, p := range runs {
		if !p.commits() {
			keep(rec.byPlan[p.plan.name]...)
		}
	}
	for _, p := range runs {
		if !p.commits() {
			continue
		}
		keep(survivors(slices.Collect(maps.Values(p.stale)), p.changes)...)
		keep(p.kept...)
		keep(p.entries()...)
		for _, c := range p.withheld {
			if e, recorded := rec.byPath[c.Path]; recorded {
				entries[c.Path] = e
			} else {
				delete(entries, c.Path)
			}
		}
	}
	files := slices.SortedFunc(maps.Values(entries), func(a, b manifest.Entry) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return manifest.Manifest{Version: manifest.Version, Workspace: rec.workspace, Files: files}
}

// commits reports whether the plan commits, or would under Dry.
func (p *planRun) commits() bool { return p.status == PlanCommitted || p.status == PlanPrepared }

// entries returns the manifest entries of the files the plan routed, in
// path order, each with the digest its changes state: the bytes the
// plan staged, or the bytes its commit wrote once it committed. A file
// whose change the commit refused states no digest.
func (p *planRun) entries() []manifest.Entry {
	hashes := make(map[string]string, len(p.changes))
	for _, c := range p.changes {
		hashes[c.Path] = c.Hash
	}
	out := make([]manifest.Entry, 0, len(p.files))
	for _, f := range p.files {
		out = append(out, manifest.Entry{
			Path: f.path, Plan: p.plan.name, Hash: hashes[f.path], Plugins: f.plugins, Sources: f.sources,
		})
	}
	return out
}

// survivors returns the stale entries whose files remain after their
// removal: a file edited since its stamp, and the brand's intact output
// the commit did not remove because it changed after the preparation.
// A stale path that contains nothing, contains a file without the
// brand's frame, or whose file the commit removed leaves the record.
func survivors(stale []manifest.Entry, changes []output.Change) []manifest.Entry {
	found := make(map[string]output.Change, len(changes))
	for _, c := range changes {
		found[c.Path] = c
	}
	var out []manifest.Entry
	for _, e := range stale {
		c := found[e.Path]
		if c.Found == output.FoundDrifted || c.Found == output.FoundIntact && c.Action != output.ActionDeleted {
			out = append(out, e)
		}
	}
	return out
}

// commitRecord records the merged manifest strictly after the last
// commit, where the run committed a plan or swept a file: it writes the
// documents whose bytes differ from the previous record's and removes
// those the manifest leaves empty. It records under a context without
// the run's cancellation, because the record has to match the
// destination once a commit wrote to it. A dry run and a composition
// without a ledger record nothing.
func commitRecord(
	ctx context.Context, rec *record, runs []*planRun, sw *swept, dry bool, m manifest.Manifest,
) error {
	if rec.ledger == nil || dry {
		return nil
	}
	committed := slices.ContainsFunc(runs, func(p *planRun) bool { return p.status == PlanCommitted })
	if !committed && !sw.applies {
		return nil
	}
	if _, err := state.WriteManifest(context.WithoutCancel(ctx), rec.ledger, m, rec.digests); err != nil {
		return fmt.Errorf("workspace: record the run: %w", err)
	}
	return nil
}
