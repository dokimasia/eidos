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
// only in case, and a path another needs as a directory. The finding
// is at the second plan's file in composition order, with the first
// plan's as related. It reports whether any plans collided, because
// no plan of the run commits then.
func collide(runs []*planRun, sink *diag.Sink) bool {
	first := map[string]claimed{}
	var tree pathset.Set
	collided := false
	for _, p := range runs {
		for _, f := range p.files {
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
		for _, f := range p.files {
			if _, taken := first[f.path]; !taken {
				first[f.path] = claimed{plan: p.plan.name, at: f.at}
				tree.Add(f.path)
			}
		}
	}
	return collided
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
// stamp.
func (w *Workspace) audit(g *store.Graph, facts *meta.Facts, loaded *load.Report, sink *diag.Sink) bool {
	if len(w.contracts) == 0 {
		return false
	}
	var audited map[string]bool
	if loaded != nil {
		audited = map[string]bool{}
		for _, u := range loaded.Units {
			if u.Depth == plugin.DepthFull {
				for _, f := range u.Files {
					audited[f] = true
				}
			}
		}
	}
	failed := false
	for _, c := range w.contracts {
		present := slices.Collect(facts.ByKey(c.key))
		for _, kind := range c.On {
			for s := range g.ByKind(kind) {
				decl, held := s.(node.Declaration)
				if !held {
					continue
				}
				at := s.Position()
				id := decl.Identity()
				if audited != nil && !audited[at.File] {
					continue
				}
				if _, stamped := slices.BinarySearchFunc(present, id, symbol.Identity.Compare); stamped {
					continue
				}
				failed = failed || c.Severity == diag.SeverityError
				sink.Report(diag.Diag{
					Code:     UnmetContract,
					Severity: c.Severity,
					Pos:      at,
					Msg: fmt.Sprintf("%s %s lacks %s, whose contract promises it on every %s by the end of %s",
						kind, id.Name, c.name, kinds(c.On), c.By),
					Origin: diag.PhaseClose,
				})
			}
		}
	}
	return failed
}

// kinds spells a contract's kinds in a finding.
func kinds(ks []symbol.Kind) string {
	spelled := make([]string, len(ks))
	for i, k := range ks {
		spelled[i] = k.String()
	}
	return strings.Join(spelled, " and ")
}
