// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The spellings a directory pattern is written with: the optional
// leading "./", the trailing wildcard that extends a directory to every
// directory below it, the wildcard itself, and the tree's root
// directory.
const (
	patternDot      = "./"
	patternSubtree  = "/..."
	patternWildcard = "..."
	rootDir         = "."
)

// directory is one parsed directory pattern: a workspace-relative,
// slash-separated directory, and whether the pattern extends to every
// directory below it.
type directory struct {
	dir     string
	subtree bool
}

// parsePattern parses one directory pattern, and reports false for a
// pattern that names no directory of a workspace tree: an empty one,
// one with a ".." element, a backslash or a leading slash, and one with
// "..." anywhere but at its end. "./..." and "..." name the whole tree.
func parsePattern(p string) (directory, bool) {
	rest := strings.TrimPrefix(p, patternDot)
	d := directory{dir: rest}
	switch {
	case rest == patternWildcard:
		d = directory{dir: rootDir, subtree: true}
	case strings.HasSuffix(rest, patternSubtree):
		d = directory{dir: strings.TrimSuffix(rest, patternSubtree), subtree: true}
	}
	if strings.Contains(d.dir, patternWildcard) || strings.ContainsRune(d.dir, '\\') || !fs.ValidPath(d.dir) {
		return directory{}, false
	}
	return d, true
}

// names reports whether the pattern names a workspace-relative
// directory: the pattern's own directory, and every directory below it
// for a pattern that extends to them.
func (d directory) names(dir string) bool {
	switch {
	case dir == d.dir:
		return true
	case !d.subtree:
		return false
	case d.dir == rootDir:
		return true
	default:
		return strings.HasPrefix(dir, d.dir+"/")
	}
}

// packageKey names a package by the two identity fields every
// declaration shares with its package: how the scope maps an identity
// to its verdict.
type packageKey struct {
	lang symbol.Lang
	pkg  string
}

// Sources is a plan's source scope: which packages its generators see,
// written in a closed vocabulary of three fields. A package is in scope
// when every field that is set matches it, and the zero value admits
// every package. Build validates the fields, and each run binds them to
// the frozen graph and its facts before the plan's first generator
// runs. The bound scope decides each package the first time a reader
// asks about it.
//
// The fields apply to every package of the graph. A dependency
// package, whose files a store provides, lies in no workspace
// directory, so no pattern matches it, and it matches a module only
// where its own gen.module fact names that module. A plan scoped by
// Lang alone admits the dependency packages of its language, as an
// unscoped plan admits every package.
type Sources struct {
	// Lang admits the packages of one source language. Build refuses a
	// language that no registered frontend loads and no registered
	// rules value declares. Empty admits every language.
	Lang symbol.Lang
	// Packages admits a package whose every file is in a directory one
	// of the patterns names. A pattern is a workspace-relative,
	// slash-separated directory with an optional leading "./", and a
	// trailing "/..." extends it to every directory below. Build
	// refuses an empty pattern, a ".." element, a backslash, an
	// absolute path, and "..." anywhere but at the end. Nil admits
	// every directory.
	Packages []string
	// Module admits a package whose gen.module fact is this module
	// path. Empty admits every module.
	Module string
}

// check returns the faults of one plan's sources: a language that
// langs does not contain, and each pattern that names no directory of a
// workspace tree, each fault naming the plan.
func (s Sources) check(plan string, langs map[symbol.Lang]bool) []error {
	var faults []error
	if s.Lang != "" && !langs[s.Lang] {
		faults = append(faults, fmt.Errorf(
			"workspace: plan %q scopes the language %q, which no frontend loads and no rules value declares",
			plan, s.Lang,
		))
	}
	for _, p := range s.Packages {
		if _, valid := parsePattern(p); !valid {
			faults = append(faults, fmt.Errorf(
				"workspace: plan %q scopes the pattern %q, which names no directory of a workspace tree",
				plan, p,
			))
		}
	}
	return faults
}

// fold spells the sources for the composition's fingerprint: the
// language, the patterns in sorted order and the module, each quoted,
// so two scopes that admit different packages spell apart.
func (s Sources) fold() string {
	return fmt.Sprintf("%q %q %q", s.Lang, slices.Sorted(slices.Values(s.Packages)), s.Module)
}

// bind returns the scope the sources admit over one run's frozen graph
// and facts, nil for sources that admit every package. The scope decides
// a package the first time a reader asks about it and keeps the verdict,
// so a run decodes the regions and restores the facts of the packages
// that its plan reads, and of no other. A question after the first costs
// one map probe under a read lock. The run binds the scope after the
// annotate phase, the last phase that stamps facts, so a verdict does not
// depend on when a reader first asks. A pattern Build would refuse names
// no directory, so a graph a caller handed over without Build's
// validation is scoped by the valid patterns alone.
//
// The scope is safe for concurrent use, because the lanes of a phase call
// use it from goroutines of their own. Lanes that ask about one new
// package concurrently each decide it and come to one verdict.
func (s Sources) bind(g *store.Graph, facts *meta.Facts, k meta.KernelKeys) store.Scope {
	if s.Lang == "" && len(s.Packages) == 0 && s.Module == "" {
		return nil
	}
	dirs := make([]directory, 0, len(s.Packages))
	for _, p := range s.Packages {
		if d, valid := parsePattern(p); valid {
			dirs = append(dirs, d)
		}
	}
	var mu sync.RWMutex
	decided := map[packageKey]bool{}
	return func(pkg symbol.Identity) bool {
		key := packageKey{lang: pkg.Lang, pkg: pkg.Package}
		mu.RLock()
		in, done := decided[key]
		mu.RUnlock()
		if done {
			return in
		}
		in = s.admits(g, pkg.PackageIdentity(), dirs, facts, k)
		mu.Lock()
		decided[key] = in
		mu.Unlock()
		return in
	}
}

// admits reports whether the graph contains a package and every field
// that is set matches it. dirs are the parsed patterns, and an empty list
// with patterns declared admits no package, because none of them names a
// directory.
func (s Sources) admits(
	g *store.Graph, id symbol.Identity, dirs []directory, facts *meta.Facts, k meta.KernelKeys,
) bool {
	if s.Lang != "" && id.Lang != s.Lang {
		return false
	}
	p, held := g.PackageOf(id)
	if !held {
		return false
	}
	if len(s.Packages) > 0 && !inDirectories(p, dirs) {
		return false
	}
	if s.Module != "" {
		module, named := meta.Get(facts, id, k.Module)
		if !named || module != s.Module {
			return false
		}
	}
	return true
}

// inDirectories reports whether a package has a file, and every file
// it has is in a directory one of the patterns names. A file a store
// provides is in no workspace directory.
func inDirectories(p *node.Package, dirs []directory) bool {
	seen := false
	for _, f := range p.Files {
		if f == nil {
			continue
		}
		if _, _, stored := plugin.CutStorePath(f.Path); stored {
			return false
		}
		dir := path.Dir(f.Path)
		if !slices.ContainsFunc(dirs, func(d directory) bool { return d.names(dir) }) {
			return false
		}
		seen = true
	}
	return seen
}
