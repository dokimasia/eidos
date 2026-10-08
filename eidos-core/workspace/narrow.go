// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"slices"

	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// narrowing decides which of a plan's changes a run commits and which it
// withholds for a later run: under [Input.Patterns] the changes inside the
// patterns, under [Input.Prune] the removals alone, and otherwise every
// change. The zero value withholds nothing.
type narrowing struct {
	// scope admits the packages that the patterns name, and is nil
	// without patterns.
	scope store.Scope
	// prune withholds every write.
	prune bool
	// recorded are the previous record's entries by path, whose sources
	// place a write inside the patterns as the file's own sources do.
	recorded map[string]manifest.Entry
}

// writes reports whether the run commits the write of a file at a path
// that derives from sources: never under a prune, and under patterns where
// a pattern admits a source of the file, as the run rendered it or as the
// previous record lists it.
func (n narrowing) writes(path string, sources []string) bool {
	return !n.prune && (n.admits(sources) || n.admits(n.recorded[path].Sources))
}

// removes reports whether the run commits the removal of a stale entry of
// a plan: where a pattern admits a source of the entry, and where the plan
// runs none of the entry's plugins any more, because the producer of the
// file left the plan.
func (n narrowing) removes(e manifest.Entry, pl *compiledPlan) bool {
	if n.admits(e.Sources) {
		return true
	}
	for _, p := range e.Plugins {
		if slices.ContainsFunc(pl.entries, func(g genEntry) bool { return g.name == p }) {
			return false
		}
	}
	return true
}

// admits reports whether a source of a file is a declaration of a package
// that the patterns admit, and true without patterns. A source that does
// not parse as an identity admits nothing. A package whose last path
// segment contains a dot parses as another package and is refused, as is
// every package that a store provides.
func (n narrowing) admits(sources []string) bool {
	if n.scope == nil {
		return true
	}
	for _, s := range sources {
		if id, err := symbol.Parse(s); err == nil && n.scope(id) {
			return true
		}
	}
	return false
}

// narrowed returns the narrowing of an input's patterns and prune, with the
// patterns bound to the run's frozen graph and facts as a plan's sources
// bind, and the previous record's entries rec read.
func (w *Workspace) narrowed(in Input, g *store.Graph, facts *meta.Facts, rec *record) narrowing {
	n := narrowing{prune: in.Prune}
	if len(in.Patterns) > 0 {
		n.scope, n.recorded = Sources{Packages: in.Patterns}.bind(g, facts, w.kernel), rec.byPath
	}
	return n
}
