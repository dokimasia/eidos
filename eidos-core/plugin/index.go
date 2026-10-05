// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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
// it records nothing, because dispatch is not a plugin's read. A
// plugin's own reads go through the [store.Reader] it is handed.
//
// Index wraps the graph and does not expose it, for two reasons.
// Nothing reachable from a phase context can make a structural
// write, because the graph's write surface is not here. Nothing
// reachable can read another plugin's raw directives either. What
// dispatch needs is exactly what is here.
//
// An Index is safe for concurrent reads: [NewIndex] fixes every
// field, and the graph beneath it is frozen.
type Index struct {
	graph     *store.Graph
	facts     *meta.Facts
	validated map[symbol.Identity][]directive.Directive
	skips     map[symbol.Identity]skipEntry
	scope     store.Scope
	// admitted is the set of packages the scope admits, keyed by the
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
// graph, because routing over any of those would return partial
// results. validated maps each subject to its typed instances in
// position order, as validation returned them. The index keeps the
// map, and the caller does not mutate it after handing it over. A
// nil scope admits everything.
//
// # Allocation contract
//
// NewIndex allocates the index. A run with a skip adds the skip table,
// and a scope adds the set of packages it admits, each sized by what it
// contains.
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
			"plugin: the graph is not frozen, so routing would return partial results",
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

// admittedOf evaluates the scope once per package in the graph, and
// returns nil for a nil scope.
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

// skipsOf derives the skip table once, so a match costs one probe of
// a map of only the subjects that opt out of something. The table
// combines the kernel skip directive with every negated instance: a
// negated instance opts its subject out of the plugin that
// registered its schema, as skip plugin=<that plugin> does. A run
// without a skip has a nil table and allocates none.
func skipsOf(
	validated map[symbol.Identity][]directive.Directive,
) map[symbol.Identity]skipEntry {
	var skips map[symbol.Identity]skipEntry
	for id, ds := range validated {
		for _, d := range ds {
			var plugin ID
			switch {
			case d.Negated:
				plugin = ID(d.Name.Plugin())
			case d.Name == directive.KernelSkip:
				v, narrowed := d.Param(directive.SkipPlugin)
				if narrowed {
					plugin = ID(v.Str)
					break
				}
				if skips == nil {
					skips = map[symbol.Identity]skipEntry{}
				}
				entry := skips[id]
				entry.all = true
				skips[id] = entry
				continue
			default:
				continue
			}
			if skips == nil {
				skips = map[symbol.Identity]skipEntry{}
			}
			entry := skips[id]
			if entry.plugins == nil {
				entry.plugins = map[ID]struct{}{}
			}
			entry.plugins[plugin] = struct{}{}
			skips[id] = entry
		}
	}
	return skips
}

// ByKind enumerates the declarations of one kind under the scope,
// in identity order, untracked.
//
// # Allocation contract
//
// A range over the enumeration allocates nothing: the enumeration is a
// call of a method that takes the loop's body, which does not keep it.
func (ix *Index) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) { ix.eachOfKind(k, yield) }
}

// ByDirective enumerates the declarations with a directive of one
// spelling under the scope, in identity order, untracked. The
// store's index is keyed by the name as written, so the dispatcher
// queries each spelling a schema recognises.
//
// # Allocation contract
//
// A range over the enumeration allocates nothing, as one over
// [Index.ByKind] does.
func (ix *Index) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) { ix.eachWithDirective(n, yield) }
}

// ByFactKey enumerates the subjects on which a key reads present,
// under the scope, in identity order, untracked. The fact store
// maintains the index at stamp time, so a fact-gated rule visits its
// matches, not the whole graph.
//
// # Allocation contract
//
// A range over the enumeration allocates nothing, as one over
// [Index.ByKind] does.
func (ix *Index) ByFactKey(id meta.KeyID) iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) { ix.eachWithKey(id, yield) }
}

// Lookup returns one declaration under the scope, untracked: how a
// fact-gated candidate resolves to its declaration and kind. A
// declaration outside scope is not returned.
func (ix *Index) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if !ix.admits(id) {
		return nil, false
	}
	return ix.graph.Lookup(id)
}

// DirectivesOf returns a subject's validated instances, in position
// order, and nil for a subject with none. This is the gate's read,
// not a plugin's: a handler is handed only the one instance that
// caused its call. The returned slice is the table's own storage.
// Do not mutate it. It allocates nothing.
func (ix *Index) DirectivesOf(id symbol.Identity) []directive.Directive {
	return ix.validated[id]
}

// Skipped reports whether a subject is excluded from a plugin's bare
// and fact-gated rules: every plugin under a bare skip, the named
// one under skip plugin=<name>, and the plugin that registered a
// negated directive's schema. The table is computed once at
// [NewIndex], so a match costs one probe of a map of only the
// subjects that opt out of something.
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

// PackageOf returns the package that contains a declaration, under
// the scope, untracked: how a flush resolves a unit's namespace once
// per accumulator.
func (ix *Index) PackageOf(id symbol.Identity) (*node.Package, bool) {
	if !ix.admits(id) {
		return nil, false
	}
	return ix.graph.PackageOf(id)
}

// Reader returns a new tracked handle under the index's scope, which
// records into reads. Dispatch takes one for each lane of a phase call,
// and a plugin that implements its role directly takes the one its phase
// call hands it. It allocates the handle, one allocation, and returns
// the graph's error for a nil read set.
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

// eachOfKind calls yield with each declaration of one kind under the
// scope, in identity order, until yield returns false. It ranges
// over the graph's enumeration in place, so the compiler inlines it,
// and it does not keep yield.
func (ix *Index) eachOfKind(k symbol.Kind, yield func(symbol.Symbol) bool) {
	var run packageRun
	for s := range ix.graph.ByKind(k) {
		if run.admits(ix, s) && !yield(s) {
			return
		}
	}
}

// eachWithDirective calls yield with each declaration with a directive
// of one spelling under the scope, in identity order, until yield
// returns false, as [Index.eachOfKind] does.
func (ix *Index) eachWithDirective(n directive.Name, yield func(symbol.Symbol) bool) {
	var run packageRun
	for s := range ix.graph.ByDirective(n) {
		if run.admits(ix, s) && !yield(s) {
			return
		}
	}
}

// eachWithKey calls yield with each subject on which a key reads
// present under the scope, in identity order, until yield returns
// false, as [Index.eachOfKind] does.
func (ix *Index) eachWithKey(id meta.KeyID, yield func(symbol.Identity) bool) {
	for subject := range ix.facts.ByKey(id) {
		if ix.admits(subject) && !yield(subject) {
			return
		}
	}
}

// packageRun is the scope's verdict on the package of the last
// declaration an enumeration admitted or refused. The graph's indexes
// group declarations by package, so consecutive candidates usually
// share one verdict, and the scope's set is probed once per run of a
// package. The zero packageRun has no verdict.
type packageRun struct {
	last     pkgKey
	admitted bool
	cached   bool
}

// admits reports whether the index's scope admits a declaration of an
// enumeration, and refuses a symbol without an identity under a
// scope. A nil scope admits every symbol without a probe.
func (r *packageRun) admits(ix *Index, s symbol.Symbol) bool {
	if ix.admitted == nil {
		return true
	}
	decl, names := s.(node.Declaration)
	if !names {
		return false
	}
	id := decl.Identity()
	key := pkgKey{lang: id.Lang, pkg: id.Package}
	if !r.cached || key != r.last {
		_, r.admitted = ix.admitted[key]
		r.last, r.cached = key, true
	}
	return r.admitted
}
