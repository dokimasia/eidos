// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest

import (
	"testing"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
)

// Frozen answers a graph holding pkgs, sealed.
//
// It fails the test rather than answering an error, because a case
// whose fixture would not load is not testing what it says it is.
func Frozen(tb testing.TB, pkgs ...*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	for _, pkg := range pkgs {
		if err := g.AddPackage(pkg); err != nil {
			tb.Fatalf("AddPackage(%v): unexpected error: %v", pkg.ID, err)
		}
	}
	g.Freeze()
	return g
}

// Reading answers a tracked reader over a graph holding pkgs,
// together with the read set it records into.
//
// A nil Scope admits every package, which is what a case not about
// scope wants.
func Reading(
	tb testing.TB, sc store.Scope, pkgs ...*node.Package,
) (*store.Reader, *store.ReadSet) {
	tb.Helper()

	reads := store.NewReadSet()
	r, err := Frozen(tb, pkgs...).Reader(reads, sc)
	if err != nil {
		tb.Fatalf("Reader: unexpected error: %v", err)
	}
	return r, reads
}
