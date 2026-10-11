// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"slices"
	"strings"

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
	// holds reports whether the graph contains a subject, so a source
	// resolves to its package without a parse of the identity grammar,
	// which a dotted package spells ambiguously.
	holds func(symbol.Identity) bool
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
// that the patterns admit, and true without patterns. A source of no
// package of the graph admits nothing, and neither does a package that a
// store provides.
func (n narrowing) admits(sources []string) bool {
	if n.scope == nil {
		return true
	}
	for _, s := range sources {
		if pkg, held := n.packageOf(s); held && n.scope(pkg) {
			return true
		}
	}
	return false
}

// packageOf returns the package of a source. A record stores a source as
// the string of an identity, which is the language, a colon and the rest.
// The package is the longest prefix of the rest that is a package of the
// graph and that the end of the source, a dot, a slash or the parenthesis
// of a discriminator follows. packageOf reports false where no prefix is a
// package of the graph. It does not allocate, and a graph that decodes its
// packages lazily decodes each package that it finds.
func (n narrowing) packageOf(source string) (symbol.Identity, bool) {
	lang, rest, found := strings.Cut(source, ":")
	if !found {
		return symbol.Identity{}, false
	}
	for end := len(rest); end > 0; end-- {
		if end < len(rest) && !strings.ContainsRune(packageEnds, rune(rest[end])) {
			continue
		}
		pkg := symbol.Identity{Lang: symbol.Lang(lang), Package: rest[:end], Kind: symbol.KindPackage}
		if n.holds(pkg) {
			return pkg, true
		}
	}
	return symbol.Identity{}, false
}

// packageEnds are the bytes that end the package in the string of an
// identity: the dot before a name, the slash before a file and the
// parenthesis before a discriminator.
const packageEnds = "./("

// narrowed returns the narrowing of an input's patterns and prune, with the
// patterns bound to the run's frozen graph and facts as a plan's sources
// bind, and the previous record's entries rec read.
func (w *Workspace) narrowed(in Input, g *store.Graph, facts *meta.Facts, rec *record) narrowing {
	n := narrowing{prune: in.Prune}
	if len(in.Patterns) > 0 {
		n.scope, n.recorded = Sources{Packages: in.Patterns}.bind(g, facts, w.kernel), rec.byPath
		n.holds = g.Holds
	}
	return n
}
