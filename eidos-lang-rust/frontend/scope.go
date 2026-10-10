// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// pathSeparator joins the segments of a Rust path, as in std::io::Read.
const pathSeparator = "::"

// qualifiedOpen opens a qualified path's first segment, as in
// <T as Trait>::Item, which names an associated item of a type.
const qualifiedOpen = "<"

// scope is what one module records for the resolution step: the
// package paths of its crate and of itself, the child modules its mod
// items declare, the path each name its use declarations bind names,
// the paths its glob imports name, and the crate each extern crate
// declaration binds under another name. The parse fills it before the
// module's items lower, and it is read-only once the parse returns, so
// the resolution step reads it from any goroutine.
type scope struct {
	crate    string
	module   string
	children map[string]bool
	uses     map[string][]string
	globs    [][]string
	crates   map[string]string
}

// newScope returns the empty scope of one module of a crate.
func newScope(crate, module string) *scope {
	return &scope{
		crate: crate, module: module,
		children: map[string]bool{}, uses: map[string][]string{}, crates: map[string]string{},
	}
}

// bind records the names a use declaration binds: each local name to the
// path it names, a name's first binding kept, and the path of each glob.
// An import as _ binds no name.
func (s *scope) bind(entries []useEntry) {
	for _, e := range entries {
		switch {
		case e.glob:
			s.globs = append(s.globs, e.path)
		case e.local != discardName:
			if _, met := s.uses[e.local]; !met {
				s.uses[e.local] = e.path
			}
		}
	}
}

// candidates returns the tiers a spelling names in the module, as
// [resolve] describes them.
func (s *scope) candidates(spelling string) plugin.Candidates {
	return s.named(strings.Split(spelling, pathSeparator), 0)
}

// named returns the tiers a path names after hops use bindings were
// followed. An acyclic chain of bindings follows each binding of the
// module at most once, so a chain longer than the module's bindings is
// a cycle, which names nothing more.
func (s *scope) named(segments []string, hops int) plugin.Candidates {
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		return candidatesIn(s.packages(segments[:len(segments)-1], hops), name)
	}
	own := plugin.Candidates{{{Lang: Lang, Package: s.module, Name: name}}}
	var bound plugin.Candidates
	if p, met := s.uses[name]; met && hops < len(s.uses) {
		bound = s.named(p, hops+1)
	}
	return append(zip([]plugin.Candidates{own, bound}), s.globbed(name)...)
}

// globbed returns the tiers a name has through the module's glob imports,
// each glob's tiers zipped, so two globs that both name a declaration
// compete, which Rust reports as an ambiguity.
func (s *scope) globbed(name string) plugin.Candidates {
	sources := make([]plugin.Candidates, 0, len(s.globs))
	for _, g := range s.globs {
		sources = append(sources, candidatesIn(s.packages(g, 0), name))
	}
	return zip(sources)
}

// packages returns the packages a module path names, one tier each,
// after hops use bindings were followed:
//
//   - a path from crate, self or super names a module below the crate
//     root, the module itself and its parent
//   - a path from a child module that a mod item of the module declares
//     names a module below that child
//   - a path from a name a use declaration binds names what the binding
//     names, and one from a crate an extern crate declaration binds
//     under another name names that crate's module
//   - a path from :: or from any other name names an external crate's
//     module, and then a child module that no mod item the parse read
//     declares, such as one a macro declares
//
// It returns nothing for a path from Self and for a qualified path,
// which name an associated item of a type, for super above the crate
// root, and for a cycle of use bindings.
func (s *scope) packages(segments []string, hops int) []string {
	head, rest := segments[0], strings.Join(segments[1:], "/")
	switch {
	case head == "":
		return []string{rest}
	case head == keywordCrate:
		return []string{join(s.crate, rest)}
	case head == keywordSelf:
		return []string{join(s.module, rest)}
	case head == keywordSuper:
		at := s.module
		for len(segments) > 0 && segments[0] == keywordSuper {
			if at == s.crate {
				return nil
			}
			at, segments = path.Dir(at), segments[1:]
		}
		return []string{join(at, strings.Join(segments, "/"))}
	case head == keywordSelfType || strings.HasPrefix(head, qualifiedOpen):
		return nil
	case s.children[head]:
		return []string{join(s.module, strings.Join(segments, "/"))}
	}
	if p, met := s.uses[head]; met {
		if hops == len(s.uses) {
			return nil
		}
		return s.packages(append(slices.Clone(p), segments[1:]...), hops+1)
	}
	if crate, met := s.crates[head]; met {
		return []string{join(crate, rest)}
	}
	external := strings.Join(segments, "/")
	return []string{external, join(s.module, external)}
}

// packageOf returns the package path of the module a reference's
// spelling names its declaration in, for the reference's Package: the
// module a path names, the module of the path a use binding gives a bare
// name, and empty for any other bare name, which the module declares or
// no import binds.
func (s *scope) packageOf(spelling string) string {
	segments := strings.Split(spelling, pathSeparator)
	if len(segments) == 1 {
		bound, met := s.uses[spelling]
		if !met {
			return ""
		}
		segments = bound
	}
	return s.first(segments[:len(segments)-1])
}

// first returns the first package a module path names, and empty for
// the empty path and for a path that names none.
func (s *scope) first(module []string) string {
	if len(module) == 0 {
		return ""
	}
	if pkgs := s.packages(module, 0); len(pkgs) > 0 {
		return pkgs[0]
	}
	return ""
}

// resolve returns what a spelling could mean in one module, in Rust's
// scope order, through the bindings the parse recorded. A file whose
// parse recorded no scope resolves nothing.
//
// A bare name names the module's own item and what the module's use
// binding of it names in one tier, because Rust refuses a module that
// declares a name and imports it too, and then what the module's glob
// imports name. A path names its last segment in the module its other
// segments name, as [scope.packages] places a module path.
func resolve(s plugin.ImportScope, spelling string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	return sc.candidates(spelling)
}

// exports returns what a module publishes under a name it does not
// declare: what its use binding of the name names, in the binding's
// tiers, and then what its glob imports name. A use declaration without
// pub publishes too, because a private binding is visible to the
// module's descendants, which super::Name and use super::* read, and
// the compiler refuses a reference to it from anywhere else.
func exports(s plugin.ImportScope, name string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	var bound plugin.Candidates
	if p, met := sc.uses[name]; met {
		bound = sc.named(p, 1)
	}
	return append(bound, sc.globbed(name)...)
}

// candidatesIn returns the candidates of a name in each package, one tier
// per package.
func candidatesIn(pkgs []string, name string) plugin.Candidates {
	out := make(plugin.Candidates, 0, len(pkgs))
	for _, pkg := range pkgs {
		out = append(out, []symbol.Identity{{Lang: Lang, Package: pkg, Name: name}})
	}
	return out
}

// zip merges several sources' tiers by position: the first tier of
// each, then the second of each, so the sources compete within a tier
// and each source's own order applies across tiers.
func zip(sources []plugin.Candidates) plugin.Candidates {
	var out plugin.Candidates
	for _, src := range sources {
		for i, tier := range src {
			for len(out) <= i {
				out = append(out, nil)
			}
			out[i] = append(out[i], tier...)
		}
	}
	return out
}
