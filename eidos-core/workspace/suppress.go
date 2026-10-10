// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Suppression is one diag directive of a run's graph and what it removed.
type Suppression struct {
	// Subject is the declaration that the directive annotates, and At the
	// directive's position.
	Subject symbol.Identity
	At      position.Pos
	// Code is the code that the directive names.
	Code diag.Code
	// Count is the number of findings that the directive removed. A
	// directive that names the code of an earlier directive on the same
	// declaration removes none.
	Count int
}

// suppressed is one diag directive of a run, with the position of the
// declaration that it annotates, which keys the suppression table.
type suppressed struct {
	Suppression
	decl position.Pos
}

// policy is what a run applies to the sinks that decide an outcome: the
// table of the codes that its diag directives suppress at each
// declaration, and the promotion of every Warning under [Input.Strict].
// The zero value applies nothing.
type policy struct {
	table  map[position.Pos][]diag.Code
	strict bool
	// listed are the run's diag directives, in position order.
	listed []suppressed
}

// policyOf returns the policy of a run over a frozen graph, under strict:
// the table of the validated diag directives of every declaration that the
// graph's directive index lists. The index decodes on a warm run only the
// regions whose summary lists the spelling, and table returns each
// declaration's validated instances. It allocates nothing for a graph
// without a diag directive.
func policyOf(g *store.Graph, table plugin.Validated, strict bool) policy {
	p := policy{strict: strict}
	for s := range g.ByDirective(directive.KernelDiag) {
		// The index lists declarations alone, so the assertion succeeds.
		decl, _ := s.(node.Declaration)
		id, at := decl.Identity(), s.Position()
		for _, d := range table.DirectivesOf(id) {
			if d.Name != directive.KernelDiag {
				continue
			}
			v, _ := d.Param(directive.DiagOff)
			// Validation resolved the spelling to a registered code, so it
			// parses.
			code, _ := diag.ParseCode(v.Ref)
			if p.table == nil {
				p.table = map[position.Pos][]diag.Code{}
			}
			if !slices.Contains(p.table[at], code) {
				p.table[at] = append(p.table[at], code)
			}
			p.listed = append(p.listed, suppressed{Subject: id, At: d.Pos, Code: code, decl: at})
		}
	}
	slices.SortFunc(p.listed, func(a, b suppressed) int { return a.At.Compare(b.At) })
	return p
}

// sink returns a fresh sink under the policy: the table installed, and
// every Warning promoted under Strict.
func (p policy) sink() *diag.Sink {
	s := diag.NewSink()
	s.Suppress(p.table)
	if p.strict {
		s.Promote()
	}
	return s
}

// counted returns the run's suppressions, each with the number of findings
// that it removed from the run's sink and from the sinks of the plans of
// runs, and the number of removed findings for each code. It returns nil
// for both where the run has no diag directive, and a nil map where no
// directive removed a finding.
func (p policy) counted(sink *diag.Sink, runs []*planRun) ([]Suppression, map[diag.Code]int) {
	if len(p.listed) == 0 {
		return nil, nil
	}
	removed := sink.Removed()
	for _, r := range runs {
		for at, counts := range r.sink.Removed() {
			if removed == nil {
				removed = map[position.Pos]map[diag.Code]int{}
			}
			if removed[at] == nil {
				removed[at] = map[diag.Code]int{}
			}
			for code, n := range counts {
				removed[at][code] += n
			}
		}
	}
	out := make([]Suppression, 0, len(p.listed))
	var byCode map[diag.Code]int
	for _, l := range p.listed {
		s := l.Suppression
		s.Count = removed[l.decl][s.Code]
		delete(removed[l.decl], s.Code)
		if s.Count > 0 {
			if byCode == nil {
				byCode = map[diag.Code]int{}
			}
			byCode[s.Code] += s.Count
		}
		out = append(out, s)
	}
	return out, byCode
}

// unused reports each suppression that removed no finding as an Info under
// [UnusedSuppression], at the directive.
func unused(sink *diag.Sink, suppressions []Suppression) {
	for _, s := range suppressions {
		if s.Count == 0 {
			sink.Infof(UnusedSuppression, s.At, diag.PhaseClose,
				"the suppression of %s on %s removed no finding", s.Code, s.Subject)
		}
	}
}
