// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"context"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// workspace is the configurations of the directories above the claimed
// files, in the order of their directories. A directory has at most one
// configuration of graphql-config and one of gqlgen, in that order.
type workspace struct {
	configs []config
}

// discover reads the configurations of the directory of each claimed file
// and of every directory above it, up to the root of the workspace. files
// are in path order. Each directory is read once through [probe], and
// every read folds into the keys of the units.
func discover(r plugin.FileReader, files []string) workspace {
	probed := map[string]bool{}
	var w workspace
	for _, p := range files {
		for dir := path.Dir(p); !probed[dir]; dir = path.Dir(dir) {
			probed[dir] = true
			w.configs = append(w.configs, probe(r, dir, p)...)
			if dir == rootDir {
				break
			}
		}
	}
	slices.SortStableFunc(w.configs, func(a, b config) int {
		return cmp.Or(strings.Compare(path.Dir(a.path), path.Dir(b.path)), cmp.Compare(a.tool, b.tool))
	})
	return w
}

// readShared reads the shared inputs of a unit: the configurations, and the
// files of the other units that the schema of the unit claims. It reports a
// configuration that the frontend does not read under [UnreadConfig], at
// the file. It returns the workspace of the configurations that it reads,
// in the order of [discover], and the paths of the shared files.
//
// Error modes: readShared returns the error of a shared input that does not
// read.
func readShared(u *plugin.SourceUnit, shared []string) (workspace, []string, error) {
	var w workspace
	var files []string
	for _, p := range shared {
		name := path.Base(p)
		if !slices.Contains(gqlgenNames[:], name) && !slices.Contains(graphqlConfigNames[:], name) {
			files = append(files, p)
			continue
		}
		b, err := u.Read(p)
		if err != nil {
			return workspace{}, nil, err
		}
		c, is := readConfig(p, b)
		if !is {
			continue
		}
		if c.err != nil {
			at := position.Pos{File: p, Line: 1, Col: 1}
			u.Errorf(UnreadConfig, at, "%v. The configuration defines no unit", c.err)
			continue
		}
		w.configs = append(w.configs, c)
	}
	slices.SortStableFunc(w.configs, func(a, b config) int {
		return cmp.Or(strings.Compare(path.Dir(a.path), path.Dir(b.path)), cmp.Compare(a.tool, b.tool))
	})
	return w, files, nil
}

// claimants returns the schemas that claim the file at the workspace path
// p, in the order in which a file joins a schema. The configurations of the
// directory of the file come first, and then those of each directory above
// it, up to the root. The configurations of every other directory follow,
// in the order of their paths, because a pattern can name a file outside
// the directory of its configuration. The schemas of one configuration are
// in their order.
func (w workspace) claimants(p string) []*schema {
	var out []*schema
	visit := func(c *config) {
		for i := range c.schemas {
			if c.schemas[i].matches(p) {
				out = append(out, &c.schemas[i])
			}
		}
	}
	chain := map[string]bool{}
	for dir := path.Dir(p); ; dir = path.Dir(dir) {
		chain[dir] = true
		for i := range w.configs {
			if path.Dir(w.configs[i].path) == dir {
				visit(&w.configs[i])
			}
		}
		if dir == rootDir {
			break
		}
	}
	for i := range w.configs {
		if !chain[path.Dir(w.configs[i].path)] {
			visit(&w.configs[i])
		}
	}
	return out
}

// packageOf returns the package of the unit of the file at p: the package
// of the first schema that claims it, and the package of its directory
// where no schema claims it.
func (w workspace) packageOf(p string) string {
	if claimed := w.claimants(p); len(claimed) > 0 {
		return claimed[0].pkg
	}
	return packageOf(path.Dir(p))
}

// partition groups the claimed files into units. A unit is one schema, and
// its package is the schema:
//
//   - A file joins the unit of the first schema that claims it, in the
//     order of [workspace.claimants].
//   - A file that no schema claims joins the unit of the files of its
//     directory.
//   - Every other schema that claims a file reads the package of the unit
//     of the file as a shared package, so the file is a shared input of the
//     unit of each of those schemas.
//
// Every configuration that the frontend reads is a shared input of every
// unit, so the parse of a unit decides its schema and its shared packages
// from the same configurations. A configuration that the frontend does not
// read is a shared input of one unit alone: the unit of the first claimed
// file in or below its directory. The parse of that unit reports it. The
// units are in the order of their first files, and the files of a unit are
// in path order.
//
// partition returns no error. A configuration that does not read is a
// finding of a parse, and a schema that claims no file has no unit.
func partition(_ context.Context, refs []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	files := make([]string, 0, len(refs))
	for _, ref := range refs {
		files = append(files, ref.Path)
	}
	slices.Sort(files)
	w := discover(r, files)

	var read []string
	for _, c := range w.configs {
		if c.err == nil {
			read = append(read, c.path)
		}
	}
	owned := map[*schema][]string{}
	shares := map[*schema][]string{}
	byDir := map[string][]string{}
	var owners []*schema
	var dirs []string
	for _, p := range files {
		claimed := w.claimants(p)
		if len(claimed) == 0 {
			dir := path.Dir(p)
			if len(byDir[dir]) == 0 {
				dirs = append(dirs, dir)
			}
			byDir[dir] = append(byDir[dir], p)
			continue
		}
		if len(owned[claimed[0]]) == 0 {
			owners = append(owners, claimed[0])
		}
		owned[claimed[0]] = append(owned[claimed[0]], p)
		for _, s := range claimed[1:] {
			shares[s] = append(shares[s], p)
		}
	}

	units := make([][]plugin.SourceRef, 0, len(owners)+len(dirs))
	for _, s := range owners {
		units = append(units, members(owned[s], slices.Concat(read, shares[s])))
	}
	for _, dir := range dirs {
		units = append(units, members(byDir[dir], slices.Clone(read)))
	}
	slices.SortFunc(units, func(a, b []plugin.SourceRef) int { return strings.Compare(a[0].Path, b[0].Path) })

	unitOf := make(map[string]int, len(files))
	for i, u := range units {
		for _, ref := range u {
			unitOf[ref.Path] = i
		}
	}
	for _, c := range w.configs {
		if c.err == nil {
			continue
		}
		u := units[unitOf[c.first]]
		shared := append(slices.Clip(u[0].Shared), c.path)
		slices.Sort(shared)
		for i := range u {
			u[i].Shared = shared
		}
	}
	return units, nil
}

// members returns the members of one unit: the files, each with the shared
// inputs of the unit in path order. Every member has the one list of
// shared inputs.
func members(files, shared []string) []plugin.SourceRef {
	slices.Sort(shared)
	out := make([]plugin.SourceRef, 0, len(files))
	for _, p := range files {
		out = append(out, plugin.SourceRef{Path: p, Shared: shared})
	}
	return out
}
