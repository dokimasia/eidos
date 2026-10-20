// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"slices"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
)

// regions is a [store.Source] over one region for each package, whose
// summaries mark the first dependencies regions as regions of a
// dependency round.
type regions struct {
	all          []*store.Region
	dependencies int
}

// Regions returns the summary of each region, as [store.Region.Info]
// computes it, with the mark of a dependency round on the first
// dependencies regions.
func (r regions) Regions() []store.RegionInfo {
	infos := make([]store.RegionInfo, len(r.all))
	for i, region := range r.all {
		infos[i] = region.Info()
		infos[i].Dependency = i < r.dependencies
	}
	return infos
}

// Region returns the i-th region, which decodes whole.
func (r regions) Region(i int) (*store.Region, error) { return r.all[i], nil }

// Frozen returns a graph holding pkgs, sealed.
//
// It fails the test rather than returning an error, because a case
// whose fixture would not load is not testing what it says it is.
func Frozen(tb assert.TB, pkgs ...*node.Package) *store.Graph {
	tb.Helper()

	g := store.New()
	assert.Total(tb, g.AddPackage, pkgs,
		"the fixture's packages load: a case whose fixture would not load is not testing what it says it is")
	g.Freeze()
	return g
}

// Dependent returns a sealed graph over one region for each package, in
// which a dependency round loaded the packages of deps and the partition
// loaded pkgs. [store.Graph.Dependency] then reports each package of deps.
func Dependent(deps []*node.Package, pkgs ...*node.Package) *store.Graph {
	src := regions{dependencies: len(deps)}
	for _, p := range slices.Concat(deps, pkgs) {
		src.all = append(src.all, &store.Region{Packages: []*node.Package{p}})
	}
	return store.Sealed(src)
}

// Reading returns a tracked reader over a graph holding pkgs,
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
