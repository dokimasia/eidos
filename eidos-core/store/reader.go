// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"iter"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Scope decides which packages a reader may see.
//
// A nil Scope admits everything, which is what a composition with
// one plan wants and what a fixture uses. An enumeration asks a
// scope once per run of declarations in one package, not once per
// declaration.
type Scope func(pkg symbol.Identity) bool

// admits reports whether the scope admits a package, under the rule
// that a nil Scope admits every one.
func (sc Scope) admits(pkg symbol.Identity) bool { return sc == nil || sc(pkg) }

// verdicts asks a scope once per package run. The graph's indexes
// group declarations by package, so an enumeration calls the scope
// once per package it crosses, never once per declaration.
type verdicts struct {
	scope    Scope
	lang     symbol.Lang
	pkg      string
	admitted bool
	asked    bool
}

// admits reports whether the scope admits the package a
// declaration belongs to, reusing the previous verdict while the
// package repeats. A package is its language and its path, the two
// identity fields ownership derives from.
func (v *verdicts) admits(id symbol.Identity) bool {
	if v.scope == nil {
		return true
	}
	if !v.asked || id.Package != v.pkg || id.Lang != v.lang {
		v.lang, v.pkg, v.asked = id.Lang, id.Package, true
		v.admitted = v.scope(owningPackage(id))
	}
	return v.admitted
}

// Reader is a tracked, scope-filtered read handle over a frozen
// graph.
//
// A declaration outside scope is neither returned nor recorded.
// Returning it would let one plan observe another's sources, and
// recording it would let a change the plan could never have seen run
// the plan again.
//
// # Concurrency
//
// A Reader is not safe for concurrent use, because the [ReadSet] it
// records into is not. The graph beneath it is.
type Reader struct {
	graph *Graph
	reads *ReadSet
	scope Scope
}

// ByKind enumerates the declarations of one kind.
//
// It records a membership edge, so the reader runs again when a
// declaration of that kind enters or leaves the set, and not when one
// only changes. A change inside the set runs the reader again through
// the declaration edge recorded for each declaration the caller met.
// The iterator records what the caller met, so a caller that stops
// early records only the declarations it saw.
func (r *Reader) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		r.reads.recordKind(k)

		// An unscoped enumeration records an edge per element, so the
		// set can size its map up front. Under a scope the admitted
		// count is unknown, and reserving the full set would hand a
		// narrow reader a map sized for the whole graph.
		held := r.graph.kind(k)
		if r.scope == nil {
			r.reads.reserve(len(held))
		}

		scope := verdicts{scope: r.scope}
		for _, decl := range held {
			id := decl.Identity()
			if !scope.admits(id) {
				continue
			}
			r.reads.recordIdentity(id)
			if !yield(decl) {
				return
			}
		}
	}
}

// ByDirective enumerates the declarations carrying a spelling,
// under the reader's scope: a subject outside it is neither
// returned nor recorded. It records a membership edge on the
// spelling, so the reader runs again when a subject gains or loses
// the directive, plus a declaration edge for each declaration the
// caller met.
func (r *Reader) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		r.reads.recordDirective(n)

		scope := verdicts{scope: r.scope}
		for _, decl := range r.graph.carriers(n) {
			id := decl.Identity()
			if !scope.admits(id) {
				continue
			}
			r.reads.recordIdentity(id)
			if !yield(decl) {
				return
			}
		}
	}
}

// Lookup returns one declaration by identity.
//
// It records a declaration edge, so a change to that declaration,
// anywhere in its subtree, runs the reader again. A lookup of a
// package's own identity returns the package whole, whose members the
// reader can walk without another tracked read, so it records a package
// edge, which any member's change dirties. A lookup that returned
// nothing records too: the reader asked, so it has to run again when a
// declaration appears under that identity.
func (r *Reader) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if !r.scope.admits(owningPackage(id)) {
		return nil, false
	}
	if id.Kind == symbol.KindPackage {
		r.reads.recordPackage(id)
	} else {
		r.reads.recordIdentity(id)
	}
	return r.graph.Lookup(id)
}

// PackageOf returns the package that contains a declaration, and
// records a package edge: the reader can walk every member of the
// package it returns without another tracked read, so a change to any
// member runs the reader again.
func (r *Reader) PackageOf(id symbol.Identity) (*node.Package, bool) {
	pkg := owningPackage(id)
	if !r.scope.admits(pkg) {
		return nil, false
	}
	r.reads.recordPackage(pkg)
	return r.graph.packageOf(id)
}

// owningPackage returns the identity of the package a declaration
// belongs to.
//
// It reads the identity, not the graph, so scope is decided for a
// declaration the graph does not contain too. Deciding it from a lookup
// would let an out-of-scope caller learn whether a declaration exists.
func owningPackage(id symbol.Identity) symbol.Identity { return id.PackageIdentity() }
