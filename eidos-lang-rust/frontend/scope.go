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

// qualifiedOpen opens the first segment of a qualified path, as in
// <T as Trait>::Item, which refers to an associated item of a type.
const qualifiedOpen = "<"

// scope is what one module records for the resolution step: the package
// paths of its crate and of the module, the child modules that its mod
// items declare, the path that each name of its use declarations binds,
// the paths of its glob imports, and the crate that each extern crate
// declaration binds under another name. The parse fills it before the
// items of the module lower, and it is read-only once the parse returns,
// so the resolution step reads it from any goroutine.
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

// bind records the names that a use declaration binds: each local name to
// its path, with the first binding of a name kept, and the path of each
// glob. An import as _ binds no name.
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

// candidates returns the tiers of a spelling in the module, as [resolve]
// describes them.
func (s *scope) candidates(spelling string) plugin.Candidates {
	return s.named(strings.Split(spelling, pathSeparator), 0)
}

// named returns the tiers of a path after the resolution followed hops use
// bindings. An acyclic chain of bindings follows each binding of the
// module at most once, so a chain longer than the bindings of the module
// is a cycle, which yields nothing more.
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
	return append(plugin.Zip(own, bound), s.globbed(name)...)
}

// globbed returns the tiers of a name through the glob imports of the
// module, with the tiers of the globs zipped. Two globs that both import a
// declaration of the name compete, which Rust reports as an ambiguity.
func (s *scope) globbed(name string) plugin.Candidates {
	sources := make([]plugin.Candidates, 0, len(s.globs))
	for _, g := range s.globs {
		sources = append(sources, candidatesIn(s.packages(g, 0), name))
	}
	return plugin.Zip(sources...)
}

// packages returns the packages of a module path, one tier each, after the
// resolution followed hops use bindings:
//
//   - a path from crate, self or super refers to a module below the crate
//     root, to the module itself and to its parent
//   - a path from a child module that a mod item of the module declares
//     refers to a module below that child
//   - a path from a name that a use declaration binds refers to the target
//     of the binding, and a path from a crate that an extern crate
//     declaration binds under another name refers to a module of that
//     crate
//   - a path from :: or from any other name refers to a module of an
//     external crate, and then to a child module that no mod item that the
//     parse read declares, such as one that a macro declares
//
// It returns nothing for a path from Self and for a qualified path, which
// refer to an associated item of a type, for super above the crate root,
// and for a cycle of use bindings.
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

// packageOf returns the package path of the module of the declaration of a
// spelling, for the Package of a reference. It is the module of a path,
// the module of the path that a use binding gives a bare name, and the
// empty string for any other bare name, which the module declares or no
// import binds.
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

// first returns the first package of a module path, and the empty string
// for the empty path and for a path without a package.
func (s *scope) first(module []string) string {
	if len(module) == 0 {
		return ""
	}
	if pkgs := s.packages(module, 0); len(pkgs) > 0 {
		return pkgs[0]
	}
	return ""
}

// resolve returns the candidates of a spelling in one module, in the scope
// order of Rust, through the bindings that the parse recorded. A file
// whose parse recorded no scope resolves nothing.
//
// A bare name refers to the own item of the module and to the target of
// the use binding of the name in the module in one tier, because Rust
// refuses a module that declares a name and imports it too. It then refers
// to the names that the glob imports of the module import. A path refers
// to its last segment in the module of its other segments, as
// [scope.packages] places a module path.
func resolve(s plugin.ImportScope, spelling string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	return sc.candidates(spelling)
}

// exports returns what a module publishes under a name that it does not
// declare: the target of its use binding of the name, in the tiers of the
// binding, and then the names that its glob imports import. A use
// declaration without pub publishes too, because the descendants of the
// module see a private binding, which super::Name and use super::* read.
// The compiler refuses a reference to it from anywhere else.
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
