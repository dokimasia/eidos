// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// View is what a projection reads through: the invocation's
// tracked declaration reader, the run's arbitrated facts and the
// read set both record into, and the kernel's own keys for the
// authored values the walks read first. The workspace mints one
// per invocation. The zero View reads nothing, and a projection
// handed one refuses rather than reading an untracked graph.
type View struct {
	Decls  *store.Reader
	Facts  *meta.Facts
	Reads  meta.Recorder
	Kernel meta.KernelKeys
}

// IsZero reports whether the view reads nothing.
func (v View) IsZero() bool { return v.Decls == nil }

// Lookup returns one declaration by identity, recorded. A zero
// view holds nothing.
func (v View) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	if v.Decls == nil || id.IsZero() {
		return nil, false
	}
	return v.Decls.Lookup(id)
}

// PackageOf returns the package holding a declaration, recorded.
func (v View) PackageOf(id symbol.Identity) (*node.Package, bool) {
	if v.Decls == nil || id.IsZero() {
		return nil, false
	}
	return v.Decls.PackageOf(id)
}

// Fact returns a subject's winning value for a key, recorded. A
// view without facts, or a zero key, reads nothing.
func Fact[T meta.FactValue](v View, id symbol.Identity, k meta.Key[T]) (T, bool) {
	var zero T
	if v.Facts == nil || k.IsZero() || id.IsZero() {
		return zero, false
	}
	if v.Reads == nil {
		return meta.Get(v.Facts, id, k)
	}
	return meta.Fact(v.Facts, v.Reads, id, k)
}

// sortLangs orders languages by spelling.
func sortLangs(langs []symbol.Lang) {
	slices.Sort(langs)
}
