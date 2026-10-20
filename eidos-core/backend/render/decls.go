// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// lineBreak ends a line. One more line break before a declaration that
// follows another puts a blank line between them.
const lineBreak = '\n'

// clusterRun is the state of the clusters of one unit under render: the
// clusters that the language's Cluster gathers, the cluster of each
// clustered declaration by its index in the unit, and the clusters that
// rendered.
type clusterRun struct {
	clusters []Clustered
	memberOf map[int]int
	rendered map[int]bool
}

// declRun renders one unit's declarations in the order the flush
// fixed, as [frame.declAt] renders each. It reports whether any
// declaration rendered.
func (f *frame) declRun(u plugin.Unit, b *bound) bool {
	run := f.clustersOf(u)
	spelt := false
	for i := range u.Decls {
		spelt = f.declAt(u, &run, i, b) || spelt
	}
	return spelt
}

// orderedRun renders the declarations of the file g in order, as
// [frame.declAt] renders each, with the state of the clusters of its
// unit. It reports whether any declaration rendered.
func (f *frame) orderedRun(g *plugin.File, order []declRef, b *bound) bool {
	s := &f.ord
	s.runs = slices.Grow(s.runs[:0], len(g.Units))
	for _, u := range g.Units {
		s.runs = append(s.runs, f.clustersOf(u))
	}
	spelt := false
	for _, r := range order {
		u := g.Units[r.unit]
		f.plugin = u.Plugin
		spelt = f.declAt(u, &s.runs[r.unit], r.decl, b) || spelt
	}
	return spelt
}

// clustersOf returns the clusters of the unit u that the language's
// Cluster gathers, with the cluster of each declaration that one claims.
// A declaration that two clusters claim goes to the first, and a member
// that the unit does not contain is ignored. A language without a Cluster
// returns the zero state, which allocates nothing.
func (f *frame) clustersOf(u plugin.Unit) clusterRun {
	if f.pass.cluster == nil {
		return clusterRun{}
	}
	run := clusterRun{clusters: f.pass.cluster(u.Decls), memberOf: map[int]int{}}
	index := make(map[symbol.Symbol]int, len(u.Decls))
	for i, d := range u.Decls {
		index[d] = i
	}
	for ci, c := range run.clusters {
		for _, d := range c.Decls {
			i, held := index[d]
			if !held {
				continue
			}
			if _, claimed := run.memberOf[i]; claimed {
				continue
			}
			run.memberOf[i] = ci
		}
	}
	run.rendered = make(map[int]bool, len(run.clusters))
	return run
}

// declAt renders the declaration at index i of the unit u: what the
// language's Cluster gathers renders through the group template of its
// cluster, at the position of the first member of the cluster that comes
// up, and the rest renders through the kind templates. An emit.File is
// part of the header of the file, which the skeleton writes, so declAt
// skips it. It reports whether the declaration rendered.
func (f *frame) declAt(u plugin.Unit, run *clusterRun, i int, b *bound) bool {
	d := u.Decls[i]
	if _, header := d.(*emit.File); header {
		return false
	}
	ci, member := run.memberOf[i]
	if !member {
		return f.singleton(u, d, b)
	}
	if run.rendered[ci] {
		return false
	}
	run.rendered[ci] = true
	return f.clustered(u, run.clusters[ci], d, b)
}

// singleton renders one declaration through its kind template,
// into scratch first: a template that refuses mid-write must leave
// no fragment in the file and no import it recorded, because the
// finding reports the declaration as skipped and the file must
// match it. The import set names the declaration under render
// before the template runs, and the declaration hosts the bodies that
// its template places. It reports whether the declaration rendered.
func (f *frame) singleton(u plugin.Unit, d symbol.Symbol, b *bound) bool {
	t, templated := b.kinds[d.Kind()]
	if !templated {
		f.unspelt(d.Kind(), "the declaration")
		return false
	}
	if !f.guard(d) {
		return false
	}
	f.set.Enter(emit.DeclaredName(d))
	f.decl = d
	f.scratch.Reset()
	mark := f.set.mark()
	if err := t.Execute(&f.scratch, d); err != nil {
		f.set.rollback(mark)
		f.sink.Errorf(refusalCode(err), f.at, f.origin,
			"the %s template refused a declaration of %s: %v",
			d.Kind(), u.Plugin, err)
		return false
	}
	f.adopt()
	return true
}

// unspelt reports a declaration whose kind has no template, which
// the pass skips: under [RefusedKind] with the reason where the
// language declares the kind refused, and under [UnspeltKind] where
// the language declares nothing for it. The guard never reads a
// skipped file-level declaration, so none of its facts reports.
func (f *frame) unspelt(k symbol.Kind, what string) {
	if reason, refused := f.pass.refused[k]; refused {
		f.sink.Errorf(RefusedKind, f.at, f.origin,
			"%s refuses the %s kind, and %s is skipped: %s", f.pass.name, k, what, reason)
		return
	}
	f.sink.Errorf(UnspeltKind, f.at, f.origin,
		"%s declares no template for %s, and %s is skipped", f.pass.name, k, what)
}

// clustered renders one cluster through the group template its
// name selects, into the same scratch singleton uses and for the
// same reason. first is the member that the cluster renders at, its
// first member in the order of the unit. The import set names it as the
// declaration under render, and it hosts the bodies that the group
// template places. It reports whether the cluster rendered.
func (f *frame) clustered(u plugin.Unit, c Clustered, first symbol.Symbol, b *bound) bool {
	t, held := b.groups[c.Group]
	if !held {
		f.sink.Errorf(UnknownGroup, f.at, f.origin,
			"%s clusters %d declarations under %q, which has no group template, and they are skipped",
			f.pass.name, len(c.Decls), c.Group)
		return false
	}
	renders := true
	for _, d := range c.Decls {
		renders = f.guard(d) && renders
	}
	if !renders {
		return false
	}
	f.set.Enter(emit.DeclaredName(first))
	f.decl = first
	f.scratch.Reset()
	mark := f.set.mark()
	if err := t.Execute(&f.scratch, c); err != nil {
		f.set.rollback(mark)
		f.sink.Errorf(refusalCode(err), f.at, f.origin,
			"the %s group template refused a cluster of %s: %v",
			c.Group, u.Plugin, err)
		return false
	}
	f.adopt()
	return true
}

// adopt writes the declaration that scratch contains into the file. A
// declaration that follows another starts after a blank line, so the
// declarations of a file are separated by one blank line. A formatter
// that collapses blank lines keeps one, and the language's own printer
// decides where it prints its own.
func (f *frame) adopt() {
	if f.out.Len() > 0 {
		f.out.WriteByte(lineBreak)
	}
	f.out.Write(f.scratch.Bytes())
}

// guard reports the stated facts that the declared coverage refuses,
// rejects or misses, before a declaration's template runs, and reports
// whether the declaration renders. A refused fact is a warning that keeps
// the narrowing loud. A rejected fact is an error, and the declaration
// does not render, because the target can write it neither with the fact
// nor without it. An undeclared fact is an error that names a defect in
// the backend's own declaration. The guard reports every stated fact
// before it answers, so one declaration reports all its refusals and
// rejections at once. Where no fact is rejected, the template runs:
// most declarations render without the refused fact, and a vocabulary
// helper that refuses the combination outright reports beside the
// warning. An undeclared coverage leaves the guard off.
func (f *frame) guard(d symbol.Symbol) bool {
	if !f.pass.coverage.Declared() {
		return true
	}
	renders := true
	emit.Facts(d, func(_ symbol.Symbol, kind symbol.Kind, fact symbol.Fact) {
		switch f.pass.coverage.Of(kind, fact) {
		case Refuses:
			f.sink.Warnf(RefusedFact, f.at, f.origin,
				"%s declares no spelling for %s stated on a %s",
				f.pass.name, fact, kind)
		case Rejects:
			renders = false
			f.sink.Errorf(RejectedFact, f.at, f.origin,
				"%s cannot render the declaration with or without %s stated on a %s",
				f.pass.name, fact, kind)
		case VerdictUndeclared:
			f.sink.Errorf(UndeclaredFact, f.at, f.origin,
				"%s takes no stance on %s stated on a %s: the coverage "+
					"declaration is incomplete", f.pass.name, fact, kind)
		case Renders, Holds:
		}
	})
	return renders
}

// nested renders one nested declaration through its kind template,
// every line behind the given indentation, so a host places its
// inner declarations at member depth; it is the nested builtin. A
// kind without a template reports the way a file-level declaration
// of it does, and the member spells nothing. A template refusing the
// declaration propagates, so a host never renders around a
// half-spelt member. The block returns without its trailing line
// break, because the host's template supplies the separators its
// member layout uses. The template writes into the frame's buffer for
// the depth of the call, which every later call at that depth reuses.
// The nested declaration hosts the bodies that its template places, and
// the host of the call hosts the bodies after it. A call allocates a copy
// of the text, and a second copy behind a non-empty indent. It allocates
// no growth once its buffer has grown.
func (f *frame) nested(indent string, s symbol.Symbol) (string, error) {
	t, spelt := f.bound.kinds[s.Kind()]
	if !spelt {
		f.unspelt(s.Kind(), "the nested declaration")
		return "", nil
	}
	if len(f.nestOut) == f.depth {
		f.nestOut = append(f.nestOut, new(bytes.Buffer))
	}
	out := f.nestOut[f.depth]
	out.Reset()
	host := f.decl
	f.decl = s
	f.depth++
	err := t.Execute(out, s)
	f.depth--
	f.decl = host
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return strings.TrimSuffix(indented(out.String(), indent), string(lineBreak)), nil
}

// indented prefixes every line of text that is not blank with indent,
// and keeps the line breaks, so a blank line has no trailing spaces. It
// allocates the indented text once, and nothing for an empty indent or
// a text without a line that is not blank.
func indented(text, indent string) string {
	lines := 0
	for line := range strings.Lines(text) {
		if line != string(lineBreak) {
			lines++
		}
	}
	if lines == 0 || indent == "" {
		return text
	}
	var b strings.Builder
	b.Grow(len(text) + lines*len(indent))
	for line := range strings.Lines(text) {
		if line != string(lineBreak) {
			b.WriteString(indent)
		}
		b.WriteString(line)
	}
	return b.String()
}
