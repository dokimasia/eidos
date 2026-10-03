// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Source is a sealed graph's regions: one for each frontend unit, in
// splice order. A warm run builds one over its last generation's
// regions and the units it parsed or restored.
//
// A Source states each region's summary without decoding the region,
// and decodes a region when [Sealed] first needs it. A summary lists
// every package its region's declarations, directives and stamps name,
// as [Region.Info] computes it: a region whose summary leaves out a
// package is unreachable through that package.
//
// # Concurrency
//
// [Sealed] calls Region at most once for each region, from the
// goroutine that first needs the region, and calls it for different
// regions at once.
type Source interface {
	// Regions returns every region's summary, in splice order.
	Regions() []RegionInfo
	// Region decodes the i-th region. An error makes the region read as
	// absent, and [Graph.Damaged] returns it.
	Region(i int) (*Region, error)
}

// RegionInfo is what a run knows of a region without decoding it.
type RegionInfo struct {
	// Packages are the packages the region contributes files to, and
	// the packages of the subjects its directives and stamps name,
	// sorted.
	Packages []symbol.Identity
	// Files are the region's files, each with its package, sorted by
	// path.
	Files []RegionFile
	// Kinds are the declaration kinds the region declares, sorted.
	Kinds []symbol.Kind
	// Directives are the spellings of the raw directives attached in
	// the region, sorted.
	Directives []directive.Name
}

// RegionFile is one file of a region and the package it declares.
type RegionFile struct {
	Path string
	Pkg  symbol.Identity
}

// Region is one unit's share of the graph: the unit's packages with
// their files and declarations, after identity assignment and link,
// the raw directives and classification stamps attached to them, the
// link record of its references, and the findings its parse and its
// link reported.
//
// A package that more than one unit contributes files to appears in
// each of those units' regions, each time with that unit's files alone.
// A sealed graph merges the parts in region order, as the load's splice
// merges units. Every declaration's identity names the package whose
// files contain it, and every directive and stamp subject names a
// package of the region. A sealed graph reads the packages, the
// directives and the stamps, and a warm load reads the links and the
// findings.
//
// # Concurrency
//
// A Region is not mutated once a [Source] returns it, so any number of
// goroutines read it at once.
type Region struct {
	Packages   []*node.Package
	Directives map[symbol.Identity][]directive.Raw
	Stamps     map[symbol.Identity][]meta.RawStamp
	// Links are the references the unit's frontend resolved, in the
	// order of the depth-first walk of Packages.
	Links []Link
	// Findings are what the unit's parse reported, and what the splice
	// and the identity assignment reported about its declarations. What
	// the link reported about a reference is in that reference's Link.
	Findings []diag.Diag
}

// Link is the record of one type reference a frontend resolved: the
// reference's place among the region's references, the candidates the
// frontend's Resolve returned, and the re-exports the selection
// followed. A run that keeps the region selects the reference's target
// again from the candidates, without the frontend.
type Link struct {
	// Ref is the reference's place among the type references of the
	// depth-first walk of the region's packages, counted from zero.
	Ref int
	// Tiers are the candidates in shadowing tiers, each a bare identity:
	// its language, package, owner and name.
	Tiers [][]symbol.Identity
	// Followed are the candidates whose package the selection asked for
	// a re-export, each a bare identity.
	Followed []symbol.Identity
	// Reached are the candidates the followed re-exports offered, each a
	// bare identity. A declaration that appears or disappears under one
	// of them can move the reference's target as one of its own
	// candidates can.
	Reached []symbol.Identity
	// Findings are what the selection reported about the reference: an
	// ambiguity where its first tier with a match matched more than one
	// declaration.
	Findings []diag.Diag
}

// Info returns the region's summary: the packages its declarations,
// directives and stamps name, its files, the kinds of its declarations
// and the spellings of its directives, each sorted and without
// repeats. A declaration whose identity is zero counts toward no kind,
// because no index lists it.
//
// # Allocation contract
//
// Info allocates each non-empty list of the summary once, at its size
// before repeats drop out, and walks every declaration of the region
// once.
func (r *Region) Info() RegionInfo {
	var (
		info                 RegionInfo
		kinds                [kindSlots]bool
		files, raws, counted int
	)
	for _, p := range r.Packages {
		files += len(p.Files)
		node.Walk(p, func(s symbol.Symbol) bool {
			if decl, names := s.(node.Declaration); names && !decl.Identity().IsZero() {
				kinds[decl.Kind()] = true
			}
			return true
		})
	}
	for _, attached := range r.Directives {
		raws += len(attached)
	}
	for _, declared := range kinds {
		if declared {
			counted++
		}
	}

	info.Packages = sized[symbol.Identity](len(r.Packages) + len(r.Directives) + len(r.Stamps))
	info.Files = sized[RegionFile](files)
	info.Kinds = sized[symbol.Kind](counted)
	info.Directives = sized[directive.Name](raws)
	for _, p := range r.Packages {
		info.Packages = append(info.Packages, p.ID)
		for _, f := range p.Files {
			info.Files = append(info.Files, RegionFile{Path: f.Path, Pkg: p.ID})
		}
	}
	for subject, attached := range r.Directives {
		info.Packages = append(info.Packages, owningPackage(subject))
		for _, raw := range attached {
			info.Directives = append(info.Directives, raw.Name)
		}
	}
	for subject := range r.Stamps {
		info.Packages = append(info.Packages, owningPackage(subject))
	}
	for k, declared := range kinds {
		if declared {
			info.Kinds = append(info.Kinds, symbol.Kind(k))
		}
	}
	slices.SortFunc(info.Packages, symbol.Identity.Compare)
	info.Packages = slices.Compact(info.Packages)
	slices.SortFunc(info.Files, func(a, b RegionFile) int { return strings.Compare(a.Path, b.Path) })
	slices.Sort(info.Directives)
	info.Directives = slices.Compact(info.Directives)
	return info
}

// sized returns an empty list with room for n elements, and nil for no
// room, so an empty list of a summary is nil whichever way the summary
// was made.
func sized[T any](n int) []T {
	if n == 0 {
		return nil
	}
	return make([]T, 0, n)
}
