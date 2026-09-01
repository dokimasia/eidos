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
// one plan wants and what a fixture uses.
type Scope func(pkg symbol.Identity) bool

// admits reports whether the scope admits a package, and holds the
// rule that a nil Scope admits every one.
func (sc Scope) admits(pkg symbol.Identity) bool { return sc == nil || sc(pkg) }

// Reader is a tracked, scope-filtered read handle over a frozen
// graph.
//
// A declaration outside scope is neither returned nor recorded. Both
// halves matter: returning it would let one plan observe another's
// sources, and recording it would let a change the plan could never
// have seen re-run it.
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
// It records a set-membership edge, so the reader runs again when a
// declaration of that kind enters or leaves the set, and never when
// one merely changes. Sensitivity to a change inside the set comes
// from the per-identity edges recorded for the declarations the
// caller actually reached, which is why this returns an iterator
// rather than a slice: it records what the caller reached, not what
// it might have.
func (r *Reader) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		r.reads.recordKind(k)

		// An unscoped enumeration records an edge per element, so the
		// set can size its map up front. Under a scope the admitted
		// count is unknown, and reserving the full set would hand a
		// narrow reader a map sized for the whole graph.
		held := r.graph.byKind[k]
		if r.scope == nil {
			r.reads.reserve(len(held))
		}

		for _, decl := range held {
			id := decl.Identity()
			if !r.scope.admits(owningPackage(id)) {
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
// returned nor recorded. It records a directive-membership edge,
// so the reader runs again when a subject gains or loses the
// directive, plus a per-identity edge for each declaration the
// caller reached.
func (r *Reader) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		r.reads.recordDirective(n)

		for _, decl := range r.graph.byDirective[n] {
			id := decl.Identity()
			if !r.scope.admits(owningPackage(id)) {
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
// It records a per-identity edge, so a change to that declaration
// alone re-runs the reader. A lookup that returned nothing records
// too: the reader asked, so it has to run again when a declaration
// appears under that identity.
func (r *Reader) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if !r.scope.admits(owningPackage(id)) {
		return nil, false
	}
	r.reads.recordIdentity(id)
	return r.graph.Lookup(id)
}

// PackageOf returns the package holding a declaration, recording a
// per-identity edge on the package.
func (r *Reader) PackageOf(id symbol.Identity) (*node.Package, bool) {
	pkg := owningPackage(id)
	if !r.scope.admits(pkg) {
		return nil, false
	}
	r.reads.recordIdentity(pkg)
	return r.graph.packageOf(id)
}

// owningPackage returns the identity of the package a declaration
// belongs to.
//
// It reads the identity rather than the graph, so scope is decided
// for a declaration the graph does not hold too. Deciding it from a
// lookup instead would let an out-of-scope caller learn whether a
// declaration exists.
func owningPackage(id symbol.Identity) symbol.Identity {
	return symbol.Identity{Lang: id.Lang, Package: id.Package, Kind: symbol.KindPackage}
}
