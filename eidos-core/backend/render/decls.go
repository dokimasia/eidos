// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// declRun renders one unit's declarations: what the language's
// Cluster gathers renders through the group templates, each
// cluster at the position of its first member, and the rest
// renders through the kind templates, in the order the flush
// fixed.
func (f *frame) declRun(u plugin.Unit, b *bound) {
	memberOf := map[int]int{}
	var clusters []Clustered
	if f.pass.cluster != nil {
		clusters = f.pass.cluster(u.Decls)
		index := make(map[symbol.Symbol]int, len(u.Decls))
		for i, d := range u.Decls {
			index[d] = i
		}
		for ci, c := range clusters {
			for _, d := range c.Decls {
				i, held := index[d]
				if !held {
					continue
				}
				if _, claimed := memberOf[i]; claimed {
					continue
				}
				memberOf[i] = ci
			}
		}
	}
	rendered := make(map[int]bool, len(clusters))
	for i, d := range u.Decls {
		ci, member := memberOf[i]
		if !member {
			f.singleton(u, d, b)
			continue
		}
		if rendered[ci] {
			continue
		}
		rendered[ci] = true
		f.clustered(u, clusters[ci], b)
	}
}

// singleton renders one declaration through its kind template,
// into scratch first: a template that refuses mid-write must leave
// no fragment in the file and no import it recorded, because the
// finding reports the declaration as skipped and the file must
// match it.
func (f *frame) singleton(u plugin.Unit, d symbol.Symbol, b *bound) {
	t, spelt := b.kinds[d.Kind()]
	if !spelt {
		f.sink.Errorf(UnspeltKind, f.at, f.origin,
			"%s holds no template for %s, and the declaration is skipped",
			f.pass.name, d.Kind())
		return
	}
	f.guard(d)
	f.scratch.Reset()
	mark := f.set.mark()
	if err := t.Execute(&f.scratch, d); err != nil {
		f.set.rollback(mark)
		f.sink.Errorf(refusalCode(err), f.at, f.origin,
			"the %s template refused a declaration of %s: %v",
			d.Kind(), u.Plugin, err)
		return
	}
	f.out.Write(f.scratch.Bytes())
}

// clustered renders one cluster through the group template its
// name selects, into the same scratch singleton uses and for the
// same reason.
func (f *frame) clustered(u plugin.Unit, c Clustered, b *bound) {
	t, held := b.groups[c.Group]
	if !held {
		f.sink.Errorf(UnknownGroup, f.at, f.origin,
			"%s clusters %d declarations under %q, which has no group template, and they are skipped",
			f.pass.name, len(c.Decls), c.Group)
		return
	}
	for _, d := range c.Decls {
		f.guard(d)
	}
	f.scratch.Reset()
	mark := f.set.mark()
	if err := t.Execute(&f.scratch, c); err != nil {
		f.set.rollback(mark)
		f.sink.Errorf(refusalCode(err), f.at, f.origin,
			"the %s group template refused a cluster of %s: %v",
			c.Group, u.Plugin, err)
		return
	}
	f.out.Write(f.scratch.Bytes())
}

// guard reports the stated facts the declared coverage refuses or
// misses, before a declaration's template runs: a refused fact is
// a warning that keeps the narrowing loud, and an undeclared one
// is an error naming a defect in the backend's own declaration.
// The template still runs, so what the output holds stays the
// template's own answer: most declarations render without the
// refused fact, and a vocabulary helper refusing the combination
// outright reports beside the warning. An undeclared coverage
// leaves the guard off.
func (f *frame) guard(d symbol.Symbol) {
	if !f.pass.coverage.Declared() {
		return
	}
	emit.Facts(d, func(_ symbol.Symbol, kind symbol.Kind, fact symbol.Fact) {
		switch f.pass.coverage.Of(kind, fact) {
		case Refuses:
			f.sink.Warnf(RefusedFact, f.at, f.origin,
				"%s declares no spelling for %s stated on a %s",
				f.pass.name, fact, kind)
		case VerdictUndeclared:
			f.sink.Errorf(UndeclaredFact, f.at, f.origin,
				"%s takes no stance on %s stated on a %s: the coverage "+
					"declaration is incomplete", f.pass.name, fact, kind)
		case Renders, Holds:
		}
	})
}

// nested renders one nested declaration through its kind template,
// every line behind the given indentation, so a host places its
// inner declarations at member depth; it is the nested builtin. A
// kind without a template reports and spells nothing, the way an
// unspelt file-level declaration does; a template refusing the
// declaration propagates, so a host never renders around a
// half-spelt member. The block returns without its trailing line
// break, because the host's template supplies the separators its
// member layout uses.
func (f *frame) nested(indent string, s symbol.Symbol) (string, error) {
	t, spelt := f.bound.kinds[s.Kind()]
	if !spelt {
		f.sink.Errorf(UnspeltKind, f.at, f.origin,
			"%s holds no template for %s, and the nested declaration is skipped",
			f.pass.name, s.Kind())
		return "", nil
	}
	var out strings.Builder
	if err := t.Execute(&out, s); err != nil {
		return "", err
	}
	return indented(out.String(), indent), nil
}

// indented prefixes every non-empty line and drops the trailing
// line break, keeping blank lines bare, so an indented block
// carries no trailing spaces.
func indented(text, indent string) string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if line != "" {
			b.WriteString(indent)
			b.WriteString(line)
		}
	}
	return b.String()
}
