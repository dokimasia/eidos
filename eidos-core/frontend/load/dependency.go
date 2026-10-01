// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/plugin"
)

// dependencies runs the dependency rounds of every [plugin.Dependent]
// frontend, in composition order, and returns the units the rounds
// returned, each parsed at [plugin.DepthSignatures]. The workspace's
// units have parsed before the first round, so their imports decide
// the first round's needs.
func dependencies(
	ctx context.Context, cfg Config, tree storeTree, claimed map[string]bool, units []*unit,
) ([]*unit, error) {
	loaded := make(map[string]bool, len(units))
	for _, u := range units {
		for _, ref := range u.files {
			loaded[ref.Path] = true
		}
	}
	var out []*unit
	for _, f := range cfg.Frontends {
		dependent, is := f.(plugin.Dependent)
		if !is {
			continue
		}
		r := &rounds{
			cfg: cfg, tree: tree, frontend: f, dependent: dependent,
			claimed: claimed, loaded: loaded, units: slices.Concat(units, out),
		}
		got, err := r.run(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// rounds is one frontend's dependency rounds: the load's claimed and
// loaded files, and the units parsed so far, which the needs of each
// round are read from.
type rounds struct {
	cfg       Config
	tree      storeTree
	frontend  plugin.Frontend
	dependent plugin.Dependent
	claimed   map[string]bool
	loaded    map[string]bool
	units     []*unit
}

// run runs the rounds and returns the units they returned. A round
// after the first runs only when it has a need no earlier round
// passed. A round that returns no unit the load has not loaded adds no
// import, so the rounds end after it.
func (r *rounds) run(ctx context.Context) ([]*unit, error) {
	f := r.frontend
	config, err := encodeOptions(f)
	if err != nil {
		return nil, err
	}
	versioned, _ := f.(plugin.Versioned) // asserted when the load began
	version := versioned.Version()
	shared := r.shared()
	passed := map[string]bool{}
	var out []*unit
	for number := 1; ; number++ {
		needs := r.needs(passed)
		if number > 1 && len(needs) == 0 {
			return out, nil
		}
		for _, n := range needs {
			passed[n.Path] = true
		}
		reader := &recordingReader{fsys: r.tree, reads: sha256.New()}
		parts, err := r.dependent.Dependencies(ctx, plugin.DependencyRound{
			Number: number, Needs: needs, Shared: shared,
		}, reader)
		if err != nil {
			return nil, fmt.Errorf("load: dependencies %s: %w", f.Name(), err)
		}
		fresh, err := r.admit(parts)
		if err != nil {
			return nil, err
		}
		sum := reader.reads.Sum(nil)
		round := make([]*unit, len(fresh))
		for i, part := range fresh {
			round[i] = &unit{
				frontend: f, files: part, depth: plugin.DepthSignatures,
				partition: sum, config: config, version: version, round: number,
			}
			for _, ref := range part {
				r.loaded[ref.Path] = true
			}
		}
		if err := parseAll(ctx, r.cfg, r.tree, round); err != nil {
			return nil, err
		}
		out = append(out, round...)
		r.units = append(r.units, round...)
	}
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

// needs returns the import paths the frontend's loaded files name
// that no loaded package of its language declares and no earlier
// round passed, sorted, each with the files that import it, sorted.
// What an import path means is the language's own, so the load
// compares it with package paths and nothing else.
func (r *rounds) needs(passed map[string]bool) []plugin.Need {
	lang := r.frontend.Lang()
	declared := map[string]bool{}
	for _, u := range r.units {
		if u.frontend.Lang() != lang {
			continue
		}
		for _, p := range u.src.Graph().Packages() {
			declared[p.ID.Package] = true
		}
	}
	from := map[string][]string{}
	for _, u := range r.units {
		if u.frontend.Name() != r.frontend.Name() {
			continue
		}
		for _, p := range u.src.Graph().Packages() {
			for _, file := range p.Files {
				for _, imp := range file.Imports {
					if declared[imp.Path] || passed[imp.Path] {
						continue
					}
					from[imp.Path] = append(from[imp.Path], file.Path)
				}
			}
		}
	}
	needs := make([]plugin.Need, 0, len(from))
	for _, path := range slices.Sorted(maps.Keys(from)) {
		files := from[path]
		slices.Sort(files)
		needs = append(needs, plugin.Need{Path: path, From: slices.Compact(files)})
	}
	return needs
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
