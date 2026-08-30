// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"errors"
	"iter"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Index is the dispatcher's routing surface over one frozen run:
// untracked, scope-filtered enumeration, plus the validated
// directive table and the skip table derived from it. Enumerating
// it records nothing, because dispatch is not a plugin's read; a
// plugin's own reads go through the [store.Reader] it is handed.
//
// Index wraps the graph rather than exposing it, and the wrapping
// is load-bearing twice over. Nothing reachable from a phase
// context can make a structural write, because the graph's write
// surface is not here; and nothing reachable can read a stranger's
// raw directives. What dispatch needs is exactly what is here.
//
// An Index is safe for concurrent reads: everything it holds is
// fixed at [NewIndex], and the graph beneath it is frozen.
type Index struct {
	graph     *store.Graph
	facts     *meta.Facts
	validated map[symbol.Identity][]directive.Directive
	skips     map[symbol.Identity]skipEntry
	scope     store.Scope
	// admitted holds the packages the scope admits, keyed by the
	// two identity fields ownership derives from. The scope runs
	// once per package here, at construction, so no enumeration
	// evaluates it per declaration. It is nil for a nil scope,
	// which admits everything.
	admitted map[pkgKey]struct{}
}

// pkgKey names a package by the identity fields a declaration
// shares with its owner: how a candidate maps to its package
// without a graph probe.
type pkgKey struct {
	lang symbol.Lang
	pkg  string
}

// skipEntry is one subject's skip ruling: every plugin, or a named
// few.
type skipEntry struct {
	all     bool
	plugins map[ID]struct{}
}

// NewIndex builds the routing surface for one run.
//
// It refuses a missing graph, a missing fact store and an unfrozen
// graph, because routing over any of those would answer partial
// results. validated holds each subject's typed instances in
// position order, as validation answered them; the index keeps the
// map, and the caller does not mutate it after handing it over. A
// nil scope admits everything.
func NewIndex(
	g *store.Graph, f *meta.Facts,
	validated map[symbol.Identity][]directive.Directive,
	sc store.Scope,
) (*Index, error) {
	if g == nil {
		return nil, errors.New("plugin: no graph to route over")
	}
	if f == nil {
		return nil, errors.New(
			"plugin: no fact store, so a fact gate would have nothing to evaluate",
		)
	}
	if !g.Frozen() {
		return nil, errors.New(
			"plugin: the graph is not frozen, so routing would answer partial results",
		)
	}
	return &Index{
		graph:     g,
		facts:     f,
		validated: validated,
		skips:     skipsOf(validated),
		scope:     sc,
		admitted:  admittedOf(g, sc),
	}, nil
}

// admittedOf evaluates the scope once per held package, and answers
// nil for a nil scope.
func admittedOf(g *store.Graph, sc store.Scope) map[pkgKey]struct{} {
	if sc == nil {
		return nil
	}
	admitted := map[pkgKey]struct{}{}
	for s := range g.ByKind(symbol.KindPackage) {
		decl, names := s.(node.Declaration)
		if !names {
			continue
		}
		id := decl.Identity()
		if sc(id) {
			admitted[pkgKey{lang: id.Lang, pkg: id.Package}] = struct{}{}
		}
	}
	return admitted
}

// skipsOf derives the skip table once, so a match pays one probe of
// a map holding only the subjects that carry skip.
func skipsOf(
	validated map[symbol.Identity][]directive.Directive,
) map[symbol.Identity]skipEntry {
	skips := map[symbol.Identity]skipEntry{}
	for id, ds := range validated {
		for _, d := range ds {
			if d.Name != directive.KernelSkip {
				continue
			}
			entry := skips[id]
			v, narrowed := d.Param(directive.SkipPlugin)
			if !narrowed {
				entry.all = true
				skips[id] = entry
				continue
			}
			if entry.plugins == nil {
				entry.plugins = map[ID]struct{}{}
			}
			entry.plugins[ID(v.Str)] = struct{}{}
			skips[id] = entry
		}
	}
	return skips
}

// ByKind enumerates the declarations of one kind under the scope,
// in the graph's own order, untracked.
func (ix *Index) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return ix.filtered(ix.graph.ByKind(k))
}

// ByDirective enumerates the declarations carrying a spelling under
// the scope, in identity order, untracked. The store's index is
// keyed by the name as written, so the dispatcher queries each
// spelling a schema answers to.
func (ix *Index) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return ix.filtered(ix.graph.ByDirective(n))
}

// ByFactKey enumerates the subjects a key presently reads present
// on, under the scope, in identity order, untracked. The fact
// store maintains the index at stamp time, which is what lets a
// fact-gated rule visit its matches rather than the graph.
func (ix *Index) ByFactKey(id meta.KeyID) iter.Seq[symbol.Identity] {
	if ix.admitted == nil {
		return ix.facts.ByKey(id)
	}
	return func(yield func(symbol.Identity) bool) {
		for subject := range ix.facts.ByKey(id) {
			if !ix.admits(subject) {
				continue
			}
			if !yield(subject) {
				return
			}
		}
	}
}

// Lookup answers one declaration under the scope, untracked: how a
// fact-gated candidate resolves to its declaration and kind. A
// declaration outside scope is not answered.
func (ix *Index) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if !ix.admits(id) {
		return nil, false
	}
	return ix.graph.Lookup(id)
}

// DirectivesOf answers a subject's validated instances, in position
// order, and nil for a subject carrying none. This is the gate's
// read, not a plugin's: a handler is handed only the one instance
// that caused its call. The answered slice is the table's own
// storage; do not mutate it.
func (ix *Index) DirectivesOf(id symbol.Identity) []directive.Directive {
	return ix.validated[id]
}

// Skipped reports whether the kernel skip directive excludes a
// subject from a plugin's bare and fact-gated rules: every plugin
// under a bare skip, the named one under skip plugin=<name>. The
// table is computed once at [NewIndex], so a match pays one probe
// of a map holding only the subjects that carry skip.
func (ix *Index) Skipped(id symbol.Identity, p ID) bool {
	if len(ix.skips) == 0 {
		return false
	}
	entry, held := ix.skips[id]
	if !held {
		return false
	}
	if entry.all {
		return true
	}
	_, named := entry.plugins[p]
	return named
}

// PackageOf answers the package holding a declaration, under the
// scope, untracked: how a flush resolves a unit's namespace once
// per accumulator.
func (ix *Index) PackageOf(id symbol.Identity) (*node.Package, bool) {
	if !ix.admits(id) {
		return nil, false
	}
	return ix.graph.PackageOf(id)
}

// Reader mints a tracked handle under the index's scope, recording
// into reads: how dispatch gives each handler invocation its own
// read grain, and how a hand-rolled plugin's phase call gets its
// one.
func (ix *Index) Reader(reads *store.ReadSet) (*store.Reader, error) {
	return ix.graph.Reader(reads, ix.scope)
}

// admits reports whether the scope admits a declaration, through
// the package its identity names.
func (ix *Index) admits(id symbol.Identity) bool {
	if ix.admitted == nil {
		return true
	}
	_, held := ix.admitted[pkgKey{lang: id.Lang, pkg: id.Package}]
	return held
}

// filtered narrows an enumeration of held declarations to the
// scope. A nil scope answers the enumeration untouched, so the
// common case pays nothing. Under a scope, the verdict is cached
// per package run: the graph's indexes group declarations by
// package, so consecutive candidates usually share one answer.
func (ix *Index) filtered(seq iter.Seq[symbol.Symbol]) iter.Seq[symbol.Symbol] {
	if ix.admitted == nil {
		return seq
	}
	return func(yield func(symbol.Symbol) bool) {
		var last pkgKey
		var admitted, cached bool
		for s := range seq {
			decl, names := s.(node.Declaration)
			if !names {
				continue
			}
			id := decl.Identity()
			key := pkgKey{lang: id.Lang, pkg: id.Package}
			if !cached || key != last {
				_, admitted = ix.admitted[key]
				last, cached = key, true
			}
			if !admitted {
				continue
			}
			if !yield(s) {
				return
			}
		}
	}
}
