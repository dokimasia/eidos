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
// fixed. It reports whether any declaration rendered.
func (f *frame) declRun(u plugin.Unit, b *bound) bool {
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
	spelt := false
	for i, d := range u.Decls {
		ci, member := memberOf[i]
		if !member {
			spelt = f.singleton(u, d, b) || spelt
			continue
		}
		if rendered[ci] {
			continue
		}
		rendered[ci] = true
		spelt = f.clustered(u, clusters[ci], b) || spelt
	}
	return spelt
}

// singleton renders one declaration through its kind template,
// into scratch first: a template that refuses mid-write must leave
// no fragment in the file and no import it recorded, because the
// finding reports the declaration as skipped and the file must
// match it. It reports whether the declaration rendered.
func (f *frame) singleton(u plugin.Unit, d symbol.Symbol, b *bound) bool {
	t, templated := b.kinds[d.Kind()]
	if !templated {
		f.unspelt(d.Kind(), "the declaration")
		return false
	}
	f.guard(d)
	f.scratch.Reset()
	mark := f.set.mark()
	if err := t.Execute(&f.scratch, d); err != nil {
		f.set.rollback(mark)
		f.sink.Errorf(refusalCode(err), f.at, f.origin,
			"the %s template refused a declaration of %s: %v",
			d.Kind(), u.Plugin, err)
		return false
	}
	f.out.Write(f.scratch.Bytes())
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
// same reason. It reports whether the cluster rendered.
func (f *frame) clustered(u plugin.Unit, c Clustered, b *bound) bool {
	t, held := b.groups[c.Group]
	if !held {
		f.sink.Errorf(UnknownGroup, f.at, f.origin,
			"%s clusters %d declarations under %q, which has no group template, and they are skipped",
			f.pass.name, len(c.Decls), c.Group)
		return false
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
		return false
	}
	f.out.Write(f.scratch.Bytes())
	return true
}

// guard reports the stated facts the declared coverage refuses or
// misses, before a declaration's template runs: a refused fact is
// a warning that keeps the narrowing loud, and an undeclared one
// is an error naming a defect in the backend's own declaration.
// The template still runs, so the output is what the template
// writes: most declarations render without the refused fact, and a
// vocabulary helper refusing the combination outright reports
// beside the warning. An undeclared coverage leaves the guard off.
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
// kind without a template reports the way a file-level declaration
// of it does, and the member spells nothing. A template refusing the
// declaration propagates, so a host never renders around a
// half-spelt member. The block returns without its trailing line
// break, because the host's template supplies the separators its
// member layout uses.
func (f *frame) nested(indent string, s symbol.Symbol) (string, error) {
	t, spelt := f.bound.kinds[s.Kind()]
	if !spelt {
		f.unspelt(s.Kind(), "the nested declaration")
		return "", nil
	}
	var out strings.Builder
	if err := t.Execute(&out, s); err != nil {
		return "", err
	}
	return indented(out.String(), indent), nil
}

// declaredName returns the name a file-level declaration binds in
// the file's scope, and empty for a kind that binds none there: a
// method is scoped by its receiver, and a member by its host.
func declaredName(d symbol.Symbol) string {
	switch t := d.(type) {
	case *emit.Struct:
		return t.Name
	case *emit.Interface:
		return t.Name
	case *emit.Enum:
		return t.Name
	case *emit.Sum:
		return t.Name
	case *emit.Function:
		return t.Name
	case *emit.Alias:
		return t.Name
	case *emit.Constant:
		return t.Name
	case *emit.Variable:
		return t.Name
	default:
		return ""
	}
}

// indented prefixes every non-empty line and drops the trailing
// line break, keeping blank lines bare, so an indented block has
// no trailing spaces.
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
