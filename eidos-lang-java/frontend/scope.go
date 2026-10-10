// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// javaLang is the package every compilation unit imports on demand.
const javaLang = "java/lang"

// scope is what one file records for the resolution step: its package's
// path, the canonical name each single-type import binds its simple name
// to, and the packages and types its on-demand imports, static ones
// included, import from, each as name segments. The parse fills it
// before the file's types lower, and it is read-only once the parse
// returns, so the resolution step reads it from any goroutine.
type scope struct {
	pkg      string
	single   map[string][]string
	onDemand [][]string
}

// newScope returns the empty scope of one file of a package.
func newScope(pkg string) *scope {
	return &scope{pkg: pkg, single: map[string][]string{}}
}

// simple returns the tiers a simple type name names, in the shadowing
// order of the Java Language Specification, one tier per scope: the
// member types of each type that encloses the reference, innermost
// first, then the single-type import, then the types of the file's
// package, and then the on-demand imports with java.lang, which compete.
// The file's own top-level types are among its package's, because Java
// refuses a file that imports a type of the name one of them has.
func (s *scope) simple(owner symbol.Identity, name string) plugin.Candidates {
	var tiers plugin.Candidates
	if !owner.IsZero() {
		for chain := join(owner.Owner, owner.Name); chain != ""; chain = parentOf(chain) {
			tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: owner.Package, Owner: chain, Name: name}})
		}
	}
	if segments, met := s.single[name]; met {
		tiers = append(tiers, flat(canonical(segments)))
	}
	tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: s.pkg, Name: name}})
	var demand []symbol.Identity
	for _, d := range s.onDemand {
		demand = append(demand, flat(canonical(append(slices.Clone(d), name)))...)
	}
	return append(tiers, append(demand, symbol.Identity{Lang: Lang, Package: javaLang, Name: name}))
}

// qualified returns the tiers a qualified type name names: the member
// type its first name names where that name is a type in scope, which
// the Java Language Specification prefers over a package of that name,
// and then the canonical name's readings, a type of a package before a
// type nested in a type.
func (s *scope) qualified(owner symbol.Identity, segments []string) plugin.Candidates {
	name, middle := segments[len(segments)-1], segments[1:len(segments)-1]
	var tiers plugin.Candidates
	for _, tier := range s.simple(owner, segments[0]) {
		nested := make([]symbol.Identity, 0, len(tier))
		for _, c := range tier {
			chain := join(c.Owner, strings.Join(append([]string{c.Name}, middle...), nameSeparator))
			nested = append(nested, symbol.Identity{Lang: Lang, Package: c.Package, Owner: chain, Name: name})
		}
		tiers = append(tiers, nested)
	}
	return append(tiers, canonical(segments)...)
}

// packageOf returns the package path a reference's spelling records as
// its Package: the package a single-type import imports a simple name
// from, the one a qualified name's first name is imported from, and
// otherwise the qualified name's prefix. A simple name no single-type
// import binds records none.
func (s *scope) packageOf(spelling string) string {
	segments := strings.Split(spelling, nameSeparator)
	if imported, met := s.single[segments[0]]; met {
		return slashed(imported[:len(imported)-1])
	}
	return slashed(segments[:len(segments)-1])
}

// resolve returns what a type name could mean in one file, through the
// scope the parse recorded and the type declaration that encloses the
// reference. A class file's scope names one candidate per spelling, and
// a file whose parse recorded no scope resolves nothing.
func resolve(s plugin.ImportScope, spelling string) plugin.Candidates {
	switch sc := s.Bindings.(type) {
	case *scope:
		segments := strings.Split(spelling, nameSeparator)
		if len(segments) == 1 {
			return sc.simple(s.Owner, spelling)
		}
		return sc.qualified(s.Owner, segments)
	case classScope:
		if id, met := sc[spelling]; met {
			return plugin.Candidates{{id}}
		}
	}
	return nil
}

// canonical returns the tiers a canonical name names: the type of its
// last name in the package its other names spell, and then the type
// nested in each type its other names can spell, the longest package
// first, one tier each.
func canonical(segments []string) plugin.Candidates {
	name := segments[len(segments)-1]
	tiers := plugin.Candidates{{{Lang: Lang, Package: slashed(segments[:len(segments)-1]), Name: name}}}
	for k := len(segments) - 2; k >= 1; k-- {
		owner := strings.Join(segments[k:len(segments)-1], nameSeparator)
		tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: slashed(segments[:k]), Owner: owner, Name: name}})
	}
	return tiers
}

// flat returns every candidate of the tiers in one tier.
func flat(tiers plugin.Candidates) []symbol.Identity {
	var out []symbol.Identity
	for _, tier := range tiers {
		out = append(out, tier...)
	}
	return out
}

// join returns an owner chain extended by a name: the name alone below
// no owner.
func join(chain, name string) string {
	if chain == "" {
		return name
	}
	return chain + nameSeparator + name
}

// parentOf returns an owner chain without its last name, and empty for a
// chain of one name.
func parentOf(chain string) string {
	if parent, _, found := strings.CutLast(chain, nameSeparator); found {
		return parent
	}
	return ""
}
