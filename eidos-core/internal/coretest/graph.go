// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
)

// Frozen answers a graph holding pkgs, sealed.
//
// It fails the test rather than answering an error, because a case
// whose fixture would not load is not testing what it says it is.
func Frozen(tb assert.TB, pkgs ...*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	for _, pkg := range pkgs {
		assert.NoError(tb, g.AddPackage(pkg),
			"the fixture's packages load: a case whose fixture would not "+
				"load is not testing what it says it is")
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
	tb assert.TB, sc store.Scope, pkgs ...*node.Package,
) (*store.Reader, *store.ReadSet) {
	tb.Helper()

	reads := store.NewReadSet()
	r, err := Frozen(tb, pkgs...).Reader(reads, sc)
	assert.NoError(tb, err, "a sealed fixture graph hands out a reader")
	return r, reads
}
