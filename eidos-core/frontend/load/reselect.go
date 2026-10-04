// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// relink selects the references of kept and restored units again where
// a change can move a target: every reference of a restored unit, whose
// region the memo recorded against another graph, and every reference
// of a kept unit whose references named a declaration that appeared or
// disappeared, found one through a re-export, or followed a re-export
// of a package a parsed or restored exporting unit contributes to. A
// unit whose selections changed is relinked, and its region is the one
// the selection rewrote. It returns the first failure of a lookup into
// the kept units, the parsed units' link's included.
//
// Error modes: the error of a record that does not read whole, of a
// parse the link needed, and of [linker.reselect].
func (l *loader) relink(w *linker, units []*unit, delta []symbol.Identity) error {
	probed, err := l.probed(units, delta)
	if err != nil {
		return err
	}
	for _, u := range units {
		if u.parsed() || u.from == FromGeneration && !slices.Contains(probed, u) {
			continue
		}
		r, err := l.keptRegion(u)
		if err != nil {
			return err
		}
		changed, err := w.reselect(u, r)
		if err != nil {
			return err
		}
		if changed {
			u.relinked, u.region = true, r
		}
	}
	return w.ix.kept.err()
}

// probed returns the kept units, in splice order, whose references named
// one of the bare identities as a candidate or found one through a
// re-export, or followed a re-export of a package a parsed or restored
// exporting unit contributes to, now or in the history.
//
// Error modes: the error of a record that does not read whole.
func (l *loader) probed(units []*unit, bare []symbol.Identity) ([]*unit, error) {
	prior := l.history.prior
	if prior == nil {
		return nil, nil
	}
	slices.SortFunc(bare, symbol.Identity.Compare)
	var numbers []int
	for _, b := range slices.Compact(bare) {
		named, err := prior.Probed(b)
		if err != nil {
			return nil, fmt.Errorf("load: read the probes of %s: %w", b, err)
		}
		numbers = append(numbers, named...)
	}
	for _, u := range units {
		if u.from == FromGeneration {
			continue
		}
		if _, exports := u.frontend.(plugin.Exporter); !exports {
			continue
		}
		packages := u.packageIDs()
		if u.record != nil {
			packages = append(slices.Clip(packages), u.record.Summary.Packages...)
		}
		for _, p := range packages {
			followed, err := prior.Followed(p)
			if err != nil {
				return nil, fmt.Errorf("load: read the follows of %s: %w", p, err)
			}
			numbers = append(numbers, followed...)
		}
	}
	marked := map[string]bool{}
	for _, n := range numbers {
		if rec := l.history.number(n); rec != nil {
			marked[rec.Files[0].Path] = true
		}
	}
	var out []*unit
	for _, u := range units {
		if u.from == FromGeneration && marked[u.files[0].Path] {
			out = append(out, u)
		}
	}
	return out, nil
}

// reselect selects the targets of a kept or restored unit's references
// again from their recorded candidates, against the declarations the
// load assigned and keeps, and reports whether anything a link records
// changed: a target, a followed re-export, a candidate a re-export
// offered, or a finding. It writes each changed selection into the region in place.
//
// Error modes: an error for a link whose reference the region lacks, and
// for a reference of an importing language whose target moves, because
// only the unit's parse names the import the new target needs.
func (w *linker) reselect(u *unit, r *store.Region) (bool, error) {
	if len(r.Links) == 0 {
		return false, nil
	}
	refs := refsOf(r.Packages)
	_, imports := u.frontend.(plugin.Importer)
	changed := false
	for i := range r.Links {
		link := &r.Links[i]
		if link.Ref < 0 || link.Ref >= len(refs) {
			return false, fmt.Errorf("load: the region of %s links reference %d of %d",
				u.files[0].Path, link.Ref, len(refs))
		}
		ref := refs[link.Ref]
		w.followed, w.reached = nil, nil
		hits := w.selected(plugin.Candidates(link.Tiers))
		var target symbol.Identity
		if len(hits) > 0 {
			target = hits[0]
		}
		findings := ambiguity(ref, u.frontend, hits)
		if ref.Target == target && slices.Equal(link.Followed, w.followed) &&
			slices.Equal(link.Reached, w.reached) && sameFindings(link.Findings, findings) {
			continue
		}
		if imports && ref.Target != target {
			return false, fmt.Errorf("load: %s selects another target for %q without the parse its import needs",
				u.files[0].Path, ref.Spelling)
		}
		ref.Target, link.Followed, link.Reached, link.Findings = target, w.followed, w.reached, findings
		changed = true
	}
	return changed, nil
}

// refsOf returns the type references of a region's packages in the
// order of the depth-first walk, which a link's Ref counts in.
func refsOf(packages []*node.Package) []*node.TypeRef {
	var out []*node.TypeRef
	for _, p := range packages {
		node.Walk(p, func(s symbol.Symbol) bool {
			if ref, is := s.(*node.TypeRef); is {
				out = append(out, ref)
			}
			return true
		})
	}
	return out
}

// sameFindings reports whether two lists of findings state the same
// findings in the same order.
func sameFindings(a, b []diag.Diag) bool {
	return slices.EqualFunc(a, b, func(x, y diag.Diag) bool { return x.Compare(y) == 0 })
}
