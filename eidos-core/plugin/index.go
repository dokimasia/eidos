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

// Validated is the table of validated directives that a run routes by.
// It returns the typed instances of each subject in position order, as
// validation returned them. A warm run does not load the whole table. It
// reads the instances of a subject that it did not validate again from
// the sealed state, the first time a route reads them.
//
// # Concurrency
//
// An implementation must be safe for concurrent use, because the lanes
// of a phase call route concurrently.
type Validated interface {
	// DirectivesOf returns the validated instances of a subject in
	// position order, and nil for a subject without instances. The caller
	// must not modify the returned slice.
	DirectivesOf(id symbol.Identity) []directive.Directive
}

// ValidatedMap is a table of validated directives that is kept whole in
// memory, keyed by subject. A cold run's validation returns one, and a
// test can pass one to [NewIndex].
//
// # Concurrency
//
// A ValidatedMap is safe for concurrent reads. The caller must not
// modify it after passing it to [NewIndex].
//
// # Allocation contract
//
// DirectivesOf allocates nothing.
type ValidatedMap map[symbol.Identity][]directive.Directive

var _ Validated = ValidatedMap(nil)

// DirectivesOf returns the instances of a subject, and nil for a subject
// that the map does not contain. The returned slice shares the storage of
// the map.
func (m ValidatedMap) DirectivesOf(id symbol.Identity) []directive.Directive { return m[id] }

// Index is the dispatcher's routing surface over one frozen run. It
// enumerates the declarations under the run's scope without tracking the
// reads, and it reads the validated directive table and the skip rulings
// that the table states. An enumeration does not record its reads,
// because dispatch is not a plugin's read. A plugin's own reads go
// through the [store.Reader] that it receives.
//
// Index wraps the graph and does not expose it, for two reasons.
// Nothing reachable from a phase context can make a structural
// write, because the graph's write surface is not here. Nothing
// reachable can read another plugin's raw directives either. What
// dispatch needs is exactly what is here.
//
// An Index is safe for concurrent reads. [NewIndex] fixes every field,
// the graph is frozen, and the validated table is safe for concurrent
// use.
type Index struct {
	graph     *store.Graph
	facts     *meta.Facts
	validated Validated
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

// NewIndex builds the routing surface for one run.
//
// It refuses a missing graph, a missing fact store and an unfrozen
// graph, because routing over any of those would return partial
// results. validated is the run's table of validated directives. The
// index reads it one subject at a time, when a directive gate or a skip
// ruling routes that subject. A nil table contains no directive, and a
// nil scope admits every declaration.
//
// # Allocation contract
//
// NewIndex allocates the index. With a scope, it also allocates the set
// of packages that the scope admits.
func NewIndex(g *store.Graph, f *meta.Facts, validated Validated, sc store.Scope) (*Index, error) {
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
// Do not mutate it. It allocates what the table's DirectivesOf
// allocates, which is nothing for a [ValidatedMap].
func (ix *Index) DirectivesOf(id symbol.Identity) []directive.Directive {
	if ix.validated == nil {
		return nil
	}
	return ix.validated.DirectivesOf(id)
}

// Skipped reports whether a subject is excluded from the bare and
// fact-gated rules of a plugin. A bare skip excludes the subject from
// every plugin, and skip plugin=<name> excludes it from that plugin.
// A negated directive excludes the subject from the plugin that
// registered the directive's schema. Skipped reads the validated
// instances of the subject once, and it allocates what
// [Index.DirectivesOf] allocates.
func (ix *Index) Skipped(id symbol.Identity, p ID) bool {
	for _, d := range ix.DirectivesOf(id) {
		switch {
		case d.Negated:
			if ID(d.Name.Plugin()) == p {
				return true
			}
		case d.Name == directive.KernelSkip:
			if v, narrowed := d.Param(directive.SkipPlugin); !narrowed || ID(v.Str) == p {
				return true
			}
		}
	}
	return false
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
