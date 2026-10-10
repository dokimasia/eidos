// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// declaration is one identity-bearing declaration of a unit: its
// identity and the path of the file that contains it.
type declaration struct {
	id   symbol.Identity
	file string
}

// coupling decides which kept or restored units one pass of the load
// parses, because the units it parses or restores couple to them.
// members maps each package to the units of the load that contribute to
// it, recorded each package to the history's units that did, and first
// each unit by its first member. trial contains the identities the parsed
// units' declarations take, computed on first use.
type coupling struct {
	l        *loader
	units    []*unit
	removed  []*UnitRecord
	members  map[symbol.Identity][]*unit
	recorded map[symbol.Identity][]*UnitRecord
	first    map[string]*unit
	trial    map[*unit][]declaration
	marked   map[*unit]bool
}

// settle parses every kept or restored unit that a unit the load parses
// or restores couples to, until no unit joins. A unit that parses joins
// the units whose identities, names and links the load decides again,
// and a parse that couples to further units brings them in on the next
// pass. It counts each unit it parses.
//
// Error modes: the error of a frontend's parse, and of a record that
// does not read whole.
func (l *loader) settle(units []*unit) error {
	removed := l.removed(units)
	for {
		c := &coupling{l: l, units: units, removed: removed, marked: map[*unit]bool{}}
		marked, err := c.mark()
		if err != nil || len(marked) == 0 {
			return err
		}
		for _, u := range marked {
			u.from, u.region = FromParse, nil
			l.reparsed++
		}
		if err := parseAll(l.ctx, l.cfg, l.tree, marked); err != nil {
			return err
		}
	}
}

// removed returns the history's units whose first member begins no unit
// of the load, in the history's order.
func (l *loader) removed(units []*unit) []*UnitRecord {
	if l.history.cold() {
		return nil
	}
	begun := make(map[string]bool, len(units))
	for _, u := range units {
		begun[u.files[0].Path] = true
	}
	var out []*UnitRecord
	for i := range l.history.units {
		if rec := &l.history.units[i]; !begun[rec.Files[0].Path] {
			out = append(out, rec)
		}
	}
	return out
}

// mark returns the kept and restored units the parsed and restored
// units couple to, in splice order. For each package that a parsed,
// restored or removed unit contributes to, now or in the history, the
// kept and restored units of the package must parse:
//
//   - every one, where a unit of the package records a duplicate
//     declaration, because the first of two declarations is kept and the
//     second reports
//   - every one, where the package's units changed or a unit's name for
//     the package changed, and the package's units now spell more than
//     one name, because a later name reports against the first
//   - each one that declares an identity a change added or removed,
//     where another unit of the package declares it too
//
// A kept unit of a language whose frontend is a [plugin.Importer] must
// parse where its references name an identity that appeared,
// disappeared or moved to another file, or followed a re-export of a
// package a parsed or restored exporting unit contributes to, because
// the package its reference imports a target from is its parse's.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) mark() ([]*unit, error) {
	touched := c.touched()
	if len(touched) == 0 {
		return nil, nil
	}
	c.index()
	importers := c.importers()
	var bare []symbol.Identity
	for _, pkg := range touched {
		moved, err := c.markPackage(pkg, importers)
		if err != nil {
			return nil, err
		}
		bare = append(bare, moved...)
	}
	if importers {
		if err := c.markImporters(bare); err != nil {
			return nil, err
		}
	}
	var out []*unit
	for _, u := range c.units {
		if c.marked[u] {
			out = append(out, u)
		}
	}
	return out, nil
}

// touched returns the packages a parsed, restored or removed unit
// contributes to, now or in the history, sorted, and nothing where no
// kept or restored unit is left to couple to.
func (c *coupling) touched() []symbol.Identity {
	if !slices.ContainsFunc(c.units, func(u *unit) bool { return !u.parsed() }) {
		return nil
	}
	set := map[symbol.Identity]bool{}
	for _, u := range c.units {
		if u.from == FromGeneration {
			continue
		}
		for _, p := range u.packageIDs() {
			set[p] = true
		}
		if u.record != nil {
			for _, p := range u.record.Summary.Packages {
				set[p] = true
			}
		}
	}
	for _, rec := range c.removed {
		for _, p := range rec.Summary.Packages {
			set[p] = true
		}
	}
	return slices.SortedFunc(maps.Keys(set), symbol.Identity.Compare)
}

// index fills the units of each package, now and in the history, and
// the units by their first members.
func (c *coupling) index() {
	c.members = map[symbol.Identity][]*unit{}
	c.first = make(map[string]*unit, len(c.units))
	for _, u := range c.units {
		c.first[u.files[0].Path] = u
		for _, p := range u.packageIDs() {
			c.members[p] = append(c.members[p], u)
		}
	}
	c.recorded = map[symbol.Identity][]*UnitRecord{}
	for i := range c.l.history.units {
		rec := &c.l.history.units[i]
		for _, p := range rec.Summary.Packages {
			c.recorded[p] = append(c.recorded[p], rec)
		}
	}
}

// importers reports whether a kept unit's frontend is a
// [plugin.Importer], the one case that probes the history's references.
func (c *coupling) importers() bool {
	return slices.ContainsFunc(c.units, func(u *unit) bool {
		_, imports := u.frontend.(plugin.Importer)
		return imports && u.from == FromGeneration
	})
}

// markPackage marks the kept and restored units of one package that
// must parse. Where probe is set, it returns the bare identities of the
// package's declarations a type position can name that appeared,
// disappeared or moved to another file.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) markPackage(pkg symbol.Identity, probe bool) ([]symbol.Identity, error) {
	var stable []*unit
	for _, u := range c.members[pkg] {
		if !u.parsed() {
			stable = append(stable, u)
		}
	}
	if len(stable) == 0 && !probe {
		return nil, nil
	}
	before, now, err := c.identities(pkg)
	if err != nil {
		return nil, err
	}
	if len(stable) > 0 {
		whole, err := c.wholePackage(pkg)
		if err != nil {
			return nil, err
		}
		if whole {
			for _, u := range stable {
				c.marked[u] = true
			}
		} else if err := c.markCollisions(pkg, stable, before, now); err != nil {
			return nil, err
		}
	}
	if !probe {
		return nil, nil
	}
	return moved(before, now), nil
}

// wholePackage reports whether every unit of the package parses: a unit
// of it, now or removed, records a duplicate declaration, or its units
// or a unit's name for it changed and its units now spell more than one
// name.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) wholePackage(pkg symbol.Identity) (bool, error) {
	for _, u := range c.members[pkg] {
		if u.record != nil && hasDuplicate(u.record.Findings) {
			return true, nil
		}
		if u.from == FromMemo && hasDuplicate(u.region.Findings) {
			return true, nil
		}
	}
	for _, rec := range c.removed {
		if slices.Contains(rec.Summary.Packages, pkg) && hasDuplicate(rec.Findings) {
			return true, nil
		}
	}
	renamed, err := c.renamed(pkg)
	if err != nil || !renamed {
		return false, err
	}
	names := map[string]bool{}
	for _, u := range c.members[pkg] {
		name, err := c.l.nameOf(u, pkg)
		if err != nil {
			return false, err
		}
		if name != "" {
			names[name] = true
		}
	}
	return len(names) > 1, nil
}

// renamed reports whether the units of the package changed, or a parsed
// or restored unit spells another name for it than its record did.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) renamed(pkg symbol.Identity) (bool, error) {
	if !sameFirsts(c.members[pkg], c.recorded[pkg]) {
		return true, nil
	}
	for _, u := range c.members[pkg] {
		if u.from == FromGeneration || u.record == nil {
			continue
		}
		before, err := c.l.recordedName(u.record, pkg)
		if err != nil {
			return false, err
		}
		now, err := c.l.nameOf(u, pkg)
		if err != nil {
			return false, err
		}
		if before != now {
			return true, nil
		}
	}
	return false, nil
}

// markCollisions marks each kept or restored unit of the package that
// declares an identity a change added or removed, where another unit of
// the package declares it too.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) markCollisions(pkg symbol.Identity, stable []*unit, before, now map[symbol.Identity]string) error {
	delta := map[symbol.Identity]bool{}
	for id := range now {
		if _, held := before[id]; !held {
			delta[id] = true
		}
	}
	for id := range before {
		if _, held := now[id]; !held {
			delta[id] = true
		}
	}
	if len(delta) == 0 {
		return nil
	}
	declarers := map[symbol.Identity][]*unit{}
	for _, u := range c.members[pkg] {
		decls, err := c.declared(u)
		if err != nil {
			return err
		}
		for _, d := range decls {
			if delta[d.id] {
				declarers[d.id] = append(declarers[d.id], u)
			}
		}
	}
	for _, us := range declarers {
		if len(us) < 2 {
			continue
		}
		for _, u := range us {
			if slices.Contains(stable, u) {
				c.marked[u] = true
			}
		}
	}
	return nil
}

// identities returns the package's declarations that its parsed,
// restored and removed units declared in the history, and the ones its
// parsed and restored units declare now, each by identity with its
// file.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) identities(pkg symbol.Identity) (before, now map[symbol.Identity]string, err error) {
	before, now = map[symbol.Identity]string{}, map[symbol.Identity]string{}
	for _, u := range c.members[pkg] {
		if u.from == FromGeneration {
			continue
		}
		decls, derr := c.declared(u)
		if derr != nil {
			return nil, nil, derr
		}
		within(now, decls, pkg)
	}
	for _, rec := range c.recorded[pkg] {
		if u := c.first[rec.Files[0].Path]; u != nil && u.from == FromGeneration {
			continue
		}
		r, rerr := c.l.history.region(rec)
		if rerr != nil {
			return nil, nil, rerr
		}
		within(before, declarationsOf(r.Packages), pkg)
	}
	return before, now, nil
}

// markImporters marks each kept unit of an importing language whose
// references named one of the bare identities as a candidate, or
// followed a re-export of a package a parsed or restored exporting unit
// contributes to.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) markImporters(bare []symbol.Identity) error {
	probed, err := c.l.probed(c.units, bare)
	if err != nil {
		return err
	}
	for _, u := range probed {
		if _, imports := u.frontend.(plugin.Importer); imports {
			c.marked[u] = true
		}
	}
	return nil
}

// declared returns a unit's declarations as the load sees them now: a
// parsed unit's trial identities, and a kept or restored unit's region's.
//
// Error modes: the error of a record that does not read whole.
func (c *coupling) declared(u *unit) ([]declaration, error) {
	if u.parsed() {
		if c.trial == nil {
			c.trial = trial(c.units)
		}
		return c.trial[u], nil
	}
	r, err := c.l.keptRegion(u)
	if err != nil {
		return nil, err
	}
	return declarationsOf(r.Packages), nil
}

// trial assigns the parsed units' declarations the identities the load's
// assignment gives them, every finding discarded, and returns each
// parsed unit's declarations. The assignment that follows overwrites
// every identity the trial wrote.
func trial(units []*unit) map[*unit][]declaration {
	var parsed, copies []*unit
	for _, u := range units {
		if !u.parsed() {
			continue
		}
		c := *u
		c.linked = nil
		parsed, copies = append(parsed, u), append(copies, &c)
	}
	scratch := diag.NewSink()
	packages, _ := splice(copies, scratch)
	assign(packages, scratch)
	out := make(map[*unit][]declaration, len(parsed))
	for _, u := range parsed {
		out[u] = declarationsOf(u.src.Graph().Packages())
	}
	return out
}

// moved returns the bare identities of the declarations a type position
// can name that appeared, disappeared or moved to another file between
// before and now.
func moved(before, now map[symbol.Identity]string) []symbol.Identity {
	var out []symbol.Identity
	for id, file := range now {
		if was, held := before[id]; (!held || was != file) && targetable(id.Kind) {
			out = append(out, bareOf(id))
		}
	}
	for id := range before {
		if _, held := now[id]; !held && targetable(id.Kind) {
			out = append(out, bareOf(id))
		}
	}
	return out
}

// within adds the declarations of one package to a map by identity,
// each with its file.
func within(into map[symbol.Identity]string, decls []declaration, pkg symbol.Identity) {
	for _, d := range decls {
		if d.id.PackageIdentity() == pkg {
			into[d.id] = d.file
		}
	}
}

// declarationsOf returns every identity-bearing declaration under the
// packages' files, each file included, in traversal order, a dropped
// duplicate left out.
func declarationsOf(packages []*node.Package) []declaration {
	var out []declaration
	for _, p := range packages {
		for _, f := range p.Files {
			node.Walk(f, func(s symbol.Symbol) bool {
				if d, names := s.(node.Declaration); names && !d.Identity().IsZero() {
					out = append(out, declaration{id: d.Identity(), file: f.Path})
				}
				return true
			})
		}
	}
	return out
}

// hasDuplicate reports whether findings contain one under
// [DuplicateDeclaration].
func hasDuplicate(findings []diag.Diag) bool {
	return slices.ContainsFunc(findings, func(d diag.Diag) bool { return d.Code == DuplicateDeclaration })
}

// sameFirsts reports whether the units of a package now and the units
// that listed it in the history begin at the same first members.
func sameFirsts(now []*unit, before []*UnitRecord) bool {
	if len(now) != len(before) {
		return false
	}
	firsts := make(map[string]bool, len(before))
	for _, rec := range before {
		firsts[rec.Files[0].Path] = true
	}
	for _, u := range now {
		if !firsts[u.files[0].Path] {
			return false
		}
	}
	return true
}

// targetable reports whether a type position can name a declaration of
// a kind: the type declarations and their variants, which the
// resolution step indexes.
func targetable(k symbol.Kind) bool {
	switch k {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindEnum,
		symbol.KindSum, symbol.KindAlias,
		symbol.KindEnumVariant, symbol.KindSumVariant:
		return true
	default:
		return false
	}
}

// packageIDs returns the identities of the packages a unit contributes
// to: a parsed unit's graph's, and a kept or restored unit's summary's.
func (u *unit) packageIDs() []symbol.Identity {
	if u.parsed() {
		packages := u.src.Graph().Packages()
		out := make([]symbol.Identity, len(packages))
		for i, p := range packages {
			out[i] = symbol.Identity{Lang: u.frontend.Lang(), Package: p.ID.Package, Kind: symbol.KindPackage}
		}
		return out
	}
	return u.summary().Packages
}

// summary returns a kept or restored unit's region summary: the
// history's record of a kept unit's, and a restored region's own.
func (u *unit) summary() store.RegionInfo {
	if u.from == FromGeneration {
		return u.record.Summary
	}
	return u.region.Info()
}

// nameOf returns the name a unit's files spell for a package, and the
// empty string where the unit's part of the package names none.
//
// Error modes: the error of a kept unit's region that does not read.
func (l *loader) nameOf(u *unit, pkg symbol.Identity) (string, error) {
	if u.parsed() {
		for _, p := range u.src.Graph().Packages() {
			if p.ID.Package == pkg.Package && u.frontend.Lang() == pkg.Lang {
				return p.Name, nil
			}
		}
		return "", nil
	}
	r, err := l.keptRegion(u)
	if err != nil {
		return "", err
	}
	return partName(r, pkg), nil
}

// recordedName returns the name a recorded unit's files spelled for a
// package.
//
// Error modes: the error of a region that does not read whole.
func (l *loader) recordedName(rec *UnitRecord, pkg symbol.Identity) (string, error) {
	r, err := l.history.region(rec)
	if err != nil {
		return "", err
	}
	return partName(r, pkg), nil
}

// keptRegion returns a kept or restored unit's region: the history's,
// decoded once, and a restored unit's own.
//
// Error modes: the error of a record that does not read whole.
func (l *loader) keptRegion(u *unit) (*store.Region, error) {
	if u.region != nil {
		return u.region, nil
	}
	return l.history.region(u.record)
}

// partName returns the name a region's part of a package spells.
func partName(r *store.Region, pkg symbol.Identity) string {
	for _, p := range r.Packages {
		if p.ID == pkg {
			return p.Name
		}
	}
	return ""
}
