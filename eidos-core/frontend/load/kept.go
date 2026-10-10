// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// keptIndex looks up the declarations of the units a load keeps or
// restores, which the resolution step does not assign: a package's
// declarations a type position can name, by bare identity, and the file
// of each, built from the package's regions the first time a candidate
// names the package. It parses a kept unit of an exporting language for
// its bindings the first time a link follows a re-export through the
// unit's package.
//
// A lookup that fails to read a region or to parse a unit finds what
// the other units of the package declare, and the first failure is
// kept for [keptIndex.err], which the load returns after the link.
type keptIndex struct {
	l *loader
	// units maps each package to the kept and restored units that
	// contribute to it, in splice order.
	units    map[symbol.Identity][]*unit
	packages map[symbol.Identity]*keptPackage
	scoped   map[pkgKey][]exporterEntry
	failure  error
}

// keptPackage is one package's declarations in the kept and restored
// units: the ones a type position can name, by bare identity in
// identity order, and the file that declares each.
type keptPackage struct {
	byBare map[symbol.Identity][]symbol.Identity
	files  map[symbol.Identity]string
}

// newKeptIndex returns the index of the units the load keeps or
// restores, and nil where it parses every unit.
func newKeptIndex(l *loader, units []*unit) *keptIndex {
	k := &keptIndex{
		l:        l,
		units:    map[symbol.Identity][]*unit{},
		packages: map[symbol.Identity]*keptPackage{},
		scoped:   map[pkgKey][]exporterEntry{},
	}
	for _, u := range units {
		if u.parsed() {
			continue
		}
		for _, p := range u.packageIDs() {
			k.units[p] = append(k.units[p], u)
		}
	}
	if len(k.units) == 0 {
		return nil
	}
	return k
}

// lookup returns the kept declarations under a bare identity, in
// identity order.
func (k *keptIndex) lookup(bare symbol.Identity) []symbol.Identity {
	if p := k.pkg(bare.PackageIdentity()); p != nil {
		return p.byBare[bare]
	}
	return nil
}

// file returns the path of the file that declares a kept identity.
func (k *keptIndex) file(id symbol.Identity) string {
	if p := k.pkg(id.PackageIdentity()); p != nil {
		return p.files[id]
	}
	return ""
}

// pkg returns a package's kept declarations, built from the regions of
// its kept and restored units the first time, and nil for a package no
// such unit contributes to.
func (k *keptIndex) pkg(id symbol.Identity) *keptPackage {
	if p, built := k.packages[id]; built {
		return p
	}
	units := k.units[id]
	if len(units) == 0 {
		k.packages[id] = nil
		return nil
	}
	p := &keptPackage{byBare: map[symbol.Identity][]symbol.Identity{}, files: map[symbol.Identity]string{}}
	for _, u := range units {
		r, err := k.l.keptRegion(u)
		if err != nil {
			k.fail(err)
			continue
		}
		for _, d := range declarationsOf(r.Packages) {
			if d.id.PackageIdentity() != id || !targetable(d.id.Kind) {
				continue
			}
			bare := bareOf(d.id)
			p.byBare[bare] = append(p.byBare[bare], d.id)
			p.files[d.id] = d.file
		}
	}
	for _, bucket := range p.byBare {
		slices.SortFunc(bucket, symbol.Identity.Compare)
	}
	k.packages[id] = p
	return p
}

// exportersOf returns the files of a package that kept or restored
// units of an exporting language contribute, each with the bindings a
// parse of its unit recorded. It parses each such unit the first time.
func (k *keptIndex) exportersOf(key pkgKey) []exporterEntry {
	if entries, parsed := k.scoped[key]; parsed {
		return entries
	}
	var out []exporterEntry
	for _, u := range k.units[symbol.Identity{Lang: key.lang, Package: key.path, Kind: symbol.KindPackage}] {
		exp, exports := u.frontend.(plugin.Exporter)
		if !exports {
			continue
		}
		scopes, err := k.l.scopes(u)
		if err != nil {
			k.fail(err)
			continue
		}
		for _, s := range scopes {
			if s.file.ID.Lang == key.lang && s.file.ID.Package == key.path {
				out = append(out, exporterEntry{scope: s, exporter: exp})
			}
		}
	}
	k.scoped[key] = out
	return out
}

// fail keeps the first failure of a lookup.
func (k *keptIndex) fail(err error) {
	if k.failure == nil {
		k.failure = err
	}
}

// err returns the first failure of a lookup, and nil for an index the
// load keeps no unit in.
func (k *keptIndex) err() error {
	if k == nil {
		return nil
	}
	return k.failure
}

// scopes parses a kept or restored unit for the bindings its files
// recorded, each file carrying its identity, and counts the parse. The
// unit keeps its region: the parse serves a link that follows a
// re-export through the unit's files, and the region's findings are the
// unit's.
//
// Error modes: the frontend's error, wrapped.
func (l *loader) scopes(u *unit) ([]scopeEntry, error) {
	src := plugin.NewSourceUnit(
		u.files, l.tree, u.depth, u.frontend.Syntax(),
		string(l.cfg.Brand), diag.NewSink(), u.frontend.Name(),
	)
	if err := u.frontend.Parse(l.ctx, src); err != nil {
		return nil, fmt.Errorf("load: parse %s: %w", u.frontend.Name(), err)
	}
	l.reparsed++
	gb := src.Graph()
	lang := u.frontend.Lang()
	packageOf := map[*node.File]string{}
	for _, p := range gb.Packages() {
		for _, f := range p.Files {
			packageOf[f] = p.ID.Package
		}
	}
	recorded := gb.Scopes()
	out := make([]scopeEntry, 0, len(recorded))
	for _, s := range recorded {
		s.File.ID = symbol.Identity{Lang: lang, Package: packageOf[s.File], Name: s.File.Path, Kind: symbol.KindFile}
		out = append(out, scopeEntry{frontend: u.frontend, file: s.File, bindings: s.Bindings})
	}
	return out, nil
}
