// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"bytes"
	"fmt"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// dependencies runs the dependency rounds of every [plugin.Dependent]
// frontend, in composition order, or keeps the rounds the history
// recorded where they would return the same units. It records each round
// in the frontend's door records, and returns the units the rounds
// returned, each keyed from the gate's digests at
// [plugin.DepthSignatures] and kept, restored or parsed. The workspace's
// units are classified before the first round, so their imports decide
// the first round's needs.
//
// Error modes: the error of a frontend's round or parse, of a round that
// breaks the dependency contract, and of a record that does not read
// whole.
func (l *loader) dependencies(claimed map[string]bool, units []*unit) ([]*unit, error) {
	loaded := make(map[string]bool, len(units))
	for _, u := range units {
		for _, ref := range u.files {
			loaded[ref.Path] = true
		}
	}
	var out []*unit
	for _, f := range l.cfg.Frontends {
		dependent, is := f.(plugin.Dependent)
		if !is {
			continue
		}
		r := &rounds{
			l: l, frontend: f, dependent: dependent,
			claimed: claimed, loaded: loaded, units: slices.Concat(units, out),
		}
		got, kept, err := r.reuse()
		if err == nil && !kept {
			got, err = r.run()
		}
		if err != nil {
			return nil, err
		}
		l.doors[f.Name()] = append(l.doors[f.Name()], r.records...)
		out = append(out, got...)
	}
	return out, nil
}

// importKey names one file's imports of one path.
type importKey struct {
	path string
	file string
}

// rounds is one frontend's dependency rounds: the load, its claimed and
// loaded files, the units classified so far, which the needs of each
// round are read from, and the record of each round.
type rounds struct {
	l         *loader
	frontend  plugin.Frontend
	dependent plugin.Dependent
	claimed   map[string]bool
	loaded    map[string]bool
	units     []*unit
	records   []DoorRecord
}

// run runs the rounds and returns the units they returned. A round
// after the first runs only when it has a need no earlier round
// passed. A round that returns no unit the load has not loaded adds no
// import, so the rounds end after it.
func (r *rounds) run() ([]*unit, error) {
	f := r.frontend
	config, version, err := r.identity()
	if err != nil {
		return nil, err
	}
	shared := r.shared()
	passed := map[string]bool{}
	var out []*unit
	for number := 1; ; number++ {
		needs, at := r.needs(passed)
		if number > 1 && len(needs) == 0 {
			return out, nil
		}
		for _, n := range needs {
			passed[n.Path] = true
		}
		reader := &recordingReader{fsys: r.l.tree, gate: r.l.gate}
		asked := &plugin.DependencyRound{Number: number, Needs: needs, Shared: shared}
		parts, err := r.dependent.Dependencies(r.l.ctx, asked, reader)
		if err != nil {
			return nil, fmt.Errorf("load: dependencies %s: %w", f.Name(), err)
		}
		found, stray := r.warnUnplaced(asked, at)
		if stray != nil {
			return nil, stray
		}
		fresh, err := r.admit(parts)
		if err != nil {
			return nil, err
		}
		record := DoorRecord{Round: number, Reads: reader.reads, Needs: needs, Units: fresh, Findings: found}
		r.records = append(r.records, record)
		round := r.unitsOf(record, config, version)
		r.load(round)
		if err := keyAll(round, r.l.gate, r.l.cfg.Brand); err != nil {
			return nil, err
		}
		if err := r.l.classify(round); err != nil {
			return nil, err
		}
		out = append(out, round...)
		r.units = append(r.units, round...)
	}
}

// reuse returns the units of the frontend's rounds the history recorded,
// and reports true, where the rounds would return them again: the first
// round's needs and shared inputs are the recorded ones, everything
// every round read reads the same, no recorded member is now claimed or
// loaded, and every unit keeps its recorded key, so no later round's
// needs move either. It reports each round's recorded findings again.
//
// Error modes: the error of a record that does not read whole, and of a
// file a round read that fails to read for a cause other than its
// absence.
func (r *rounds) reuse() ([]*unit, bool, error) {
	doors, err := r.l.history.doorsOf(r.frontend.Name())
	if err != nil {
		return nil, false, err
	}
	var recorded []DoorRecord
	for _, d := range doors {
		if d.Round > 0 {
			recorded = append(recorded, d)
		}
	}
	if len(recorded) == 0 {
		return nil, false, nil
	}
	needs, _ := r.needs(map[string]bool{})
	if !slices.EqualFunc(needs, recorded[0].Needs, sameNeed) || !slices.Equal(r.shared(), r.recordedShared()) {
		return nil, false, nil
	}
	config, version, err := r.identity()
	if err != nil {
		return nil, false, err
	}
	var out []*unit
	for _, door := range recorded {
		for _, read := range door.Reads {
			if same, err := unchangedRead(r.l.gate, read); err != nil || !same {
				return nil, false, err
			}
		}
		for _, part := range door.Units {
			for _, ref := range part {
				if r.claimed[ref.Path] || r.loaded[ref.Path] {
					return nil, false, nil
				}
			}
		}
		out = append(out, r.unitsOf(door, config, version)...)
	}
	if err := keyAll(out, r.l.gate, r.l.cfg.Brand); err != nil {
		return nil, false, err
	}
	for _, u := range out {
		if rec := r.l.history.first(u.files[0].Path); rec == nil || !bytes.Equal(rec.Key, u.key) {
			return nil, false, nil
		}
	}
	for _, u := range out {
		u.record, u.from = r.l.history.first(u.files[0].Path), FromGeneration
	}
	r.load(out)
	for _, door := range recorded {
		for _, d := range door.Findings {
			r.l.cfg.Sink.Report(d)
		}
	}
	r.records = recorded
	return out, true, nil
}

// unitsOf returns the units a round's record returned, each loaded at
// [plugin.DepthSignatures] and keyed on the round's fold.
func (r *rounds) unitsOf(door DoorRecord, config []byte, version string) []*unit {
	fold := door.fold()
	out := make([]*unit, len(door.Units))
	for i, part := range door.Units {
		out[i] = &unit{
			frontend: r.frontend, files: part, depth: plugin.DepthSignatures,
			door: fold, config: config, version: version, round: door.Round,
		}
	}
	return out
}

// load marks the members of units loaded.
func (r *rounds) load(units []*unit) {
	for _, u := range units {
		for _, ref := range u.files {
			r.loaded[ref.Path] = true
		}
	}
}

// identity returns the frontend's options in their canonical encoding
// and its declared version, which every unit of its rounds keys on.
//
// Error modes: the error of options the canonical encoding cannot see
// whole.
func (r *rounds) identity() ([]byte, string, error) {
	config, err := encodeOptions(r.frontend)
	if err != nil {
		return nil, "", err
	}
	versioned, _ := r.frontend.(plugin.Versioned) // asserted when the load began
	return config, versioned.Version(), nil
}

// shared returns the shared inputs the frontend's partition declared
// on the workspace's units, sorted and without repeats.
func (r *rounds) shared() []string {
	set := map[string]bool{}
	for _, u := range r.units {
		if u.frontend.Name() != r.frontend.Name() {
			continue
		}
		for _, ref := range u.files {
			for _, s := range ref.Shared {
				set[s] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// recordedShared returns the shared inputs the frontend's partition
// declared on the workspace's units of the history, sorted and without
// repeats.
func (r *rounds) recordedShared() []string {
	set := map[string]bool{}
	for _, rec := range r.l.history.units {
		if rec.Frontend != r.frontend.Name() || rec.Round != 0 {
			continue
		}
		for _, ref := range rec.Files {
			for _, s := range ref.Shared {
				set[s] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// needs returns the import paths the frontend's loaded files name
// that no loaded package of its language declares and no earlier
// round passed, sorted, each with the files that import it, sorted.
// What an import path means is the language's own, so the load
// compares it with package paths and nothing else. It returns beside
// them where each need's first file imports it: the position of the
// file's first import of the path. A kept unit states its packages and
// its imports through its record.
func (r *rounds) needs(passed map[string]bool) ([]plugin.Need, map[string]position.Pos) {
	lang := r.frontend.Lang()
	declared := map[string]bool{}
	for _, u := range r.units {
		if u.frontend.Lang() != lang {
			continue
		}
		for _, p := range u.packageIDs() {
			declared[p.Package] = true
		}
	}
	from := map[string][]string{}
	first := map[string]string{}
	at := map[string]position.Pos{}
	for _, u := range r.units {
		if u.frontend.Name() != r.frontend.Name() {
			continue
		}
		for _, imp := range u.importList() {
			if declared[imp.Path] || passed[imp.Path] {
				continue
			}
			from[imp.Path] = append(from[imp.Path], imp.Files...)
			if f, met := first[imp.Path]; !met || imp.Files[0] < f {
				first[imp.Path], at[imp.Path] = imp.Files[0], imp.At
			}
		}
	}
	needs := make([]plugin.Need, 0, len(from))
	for _, path := range slices.Sorted(maps.Keys(from)) {
		files := from[path]
		slices.Sort(files)
		needs = append(needs, plugin.Need{Path: path, From: slices.Compact(files)})
	}
	return needs, at
}

// warnUnplaced reports each need the frontend reported placed nowhere,
// once, under [UnplacedNeed]: at the need's import in the first file
// that imports it, naming how many files import it and quoting the
// frontend's reason. It returns the findings for the round's record. A
// reported path that is no need of the round is fatal, naming the
// frontend.
func (r *rounds) warnUnplaced(round *plugin.DependencyRound, at map[string]position.Pos) ([]diag.Diag, error) {
	importers := make(map[string]int, len(round.Needs))
	for _, n := range round.Needs {
		importers[n.Path] = len(n.From)
	}
	warned := map[string]bool{}
	var found []diag.Diag
	for _, u := range round.Unplaced() {
		files, need := importers[u.Path]
		if !need {
			return nil, fmt.Errorf("load: dependencies %s: round %d reports %s placed nowhere, "+
				"and it is no need of the round", r.frontend.Name(), round.Number, u.Path)
		}
		if warned[u.Path] {
			continue
		}
		warned[u.Path] = true
		d := diag.Diag{
			Code:     UnplacedNeed,
			Severity: diag.SeverityWarning,
			Pos:      at[u.Path],
			Msg: fmt.Sprintf("%s places in no dependency unit: %s. It is imported by %d of the load's files, "+
				"and every reference into it keeps its spelling", u.Path, u.Reason, files),
			Origin: r.frontend.Name(),
		}
		r.l.cfg.Sink.Report(d)
		found = append(found, d)
	}
	return found, nil
}

// admit checks a round's units against the dependency contract and
// returns the units the load has not loaded. It drops a unit whose
// every member a loaded unit already has, so a language that returns
// its declared dependencies in every round loads them once. An empty
// unit, a member some frontend's selection claims, a member the round
// returns twice, and a unit that shares only some of its members with
// loaded units are each fatal, naming the frontend.
func (r *rounds) admit(parts [][]plugin.SourceRef) ([][]plugin.SourceRef, error) {
	name := r.frontend.Name()
	seen := map[string]bool{}
	var fresh [][]plugin.SourceRef
	for _, part := range parts {
		if len(part) == 0 {
			return nil, fmt.Errorf("load: dependencies %s: a unit with no members has nothing to parse", name)
		}
		loaded := 0
		for _, ref := range part {
			if r.claimed[ref.Path] {
				return nil, fmt.Errorf("load: dependencies %s: %s is a member the selection claims, "+
					"and a dependency is never the workspace's source", name, ref.Path)
			}
			if seen[ref.Path] {
				return nil, fmt.Errorf("load: dependencies %s: %s is a member twice in one round", name, ref.Path)
			}
			seen[ref.Path] = true
			if r.loaded[ref.Path] {
				loaded++
			}
		}
		switch loaded {
		case 0:
			fresh = append(fresh, part)
		case len(part):
		default:
			return nil, fmt.Errorf("load: dependencies %s: the unit of %s shares %d of its %d members "+
				"with loaded units, and every file is in exactly one", name, part[0].Path, loaded, len(part))
		}
	}
	return fresh, nil
}

// importList returns the imports a unit's files name: computed from a
// parsed unit's graph or a restored unit's region, once, and a kept
// unit's record's.
func (u *unit) importList() []Import {
	switch {
	case u.imports != nil:
	case u.parsed():
		u.imports = importsOf(u.src.Graph().Packages())
	case u.from == FromMemo:
		u.imports = importsOf(u.region.Packages)
	default:
		return u.record.Imports
	}
	return u.imports
}

// importsOf returns the imports the files of packages name, sorted by
// path: each path with the files that import it, sorted, and the
// position of the first import of the path in the first of those files.
func importsOf(packages []*node.Package) []Import {
	at := map[importKey]position.Pos{}
	files := map[string][]string{}
	for _, p := range packages {
		for _, f := range p.Files {
			for _, imp := range f.Imports {
				key := importKey{path: imp.Path, file: f.Path}
				if _, met := at[key]; met {
					continue
				}
				at[key] = imp.Pos
				files[imp.Path] = append(files[imp.Path], f.Path)
			}
		}
	}
	out := make([]Import, 0, len(files))
	for _, path := range slices.Sorted(maps.Keys(files)) {
		importers := files[path]
		slices.Sort(importers)
		out = append(out, Import{Path: path, Files: importers, At: at[importKey{path: path, file: importers[0]}]})
	}
	return out
}

// sameNeed reports whether two needs state one path imported by the
// same files.
func sameNeed(a, b plugin.Need) bool {
	return a.Path == b.Path && slices.Equal(a.From, b.From)
}
