// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// resolve returns what a spelling could mean in one file, in
// TypeScript's scope order, through the bindings the parse recorded. A
// file whose parse recorded no scope resolves nothing.
//
// A bare name probes the enclosing namespaces innermost first and then
// the file's own package, one tier each, then the module an import
// binds it from, and then the global package. A qualified name probes
// the module a namespace import binds its first name to, a namespace
// a named import binds, or a namespace of each enclosing package and of
// the global package. An import alias resolves as the entity it names.
func resolve(s plugin.ImportScope, spelling string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	return sc.candidates(spelling, nil)
}

// exports returns what a file publishes under a name and does not
// declare: the explicit re-exports and the default export in one pair of
// tiers, the file before the directory index, and then the export-star
// modules in another pair, where two modules that both publish the name
// compete. Only the module's own File publishes, and export * publishes
// no default.
func exports(s plugin.ImportScope, name string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil || sc.own != sc.module.pkg {
		return nil
	}
	return sc.module.published(name)
}

// candidates returns the tiers one spelling names. followed contains the
// import aliases the resolution already followed, so an alias that
// names itself, or a cycle of aliases, names nothing.
func (s *scope) candidates(spelling string, followed map[string]bool) plugin.Candidates {
	if spelling == "" {
		return nil
	}
	m := s.module
	segments := strings.Split(spelling, namespaceSeparator)
	head, name := segments[0], segments[len(segments)-1]
	chain := s.chain()
	if len(segments) == 1 {
		tiers := make(plugin.Candidates, 0, len(chain)+2)
		for _, pkg := range chain {
			tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: pkg, Name: name}})
		}
		if b, bound := m.imports[name]; bound && b.name != "" {
			tiers = append(tiers, m.from(b.specifier, "", b.name)...)
		}
		if !slices.Contains(chain, "") {
			tiers = append(tiers, []symbol.Identity{{Lang: Lang, Name: name}})
		}
		return tiers
	}
	if entity, isAlias := m.aliases[head]; isAlias {
		if followed[head] {
			return nil
		}
		if followed == nil {
			followed = map[string]bool{}
		}
		followed[head] = true
		return s.candidates(entity+spelling[len(head):], followed)
	}
	inner := strings.Join(segments[1:len(segments)-1], "/")
	if b, bound := m.imports[head]; bound {
		if b.name == "" {
			return m.from(b.specifier, inner, name)
		}
		return m.from(b.specifier, join(b.name, inner), name)
	}
	namespace := strings.Join(segments[:len(segments)-1], "/")
	tiers := make(plugin.Candidates, 0, len(chain)+1)
	for _, pkg := range chain {
		tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: join(pkg, namespace), Name: name}})
	}
	if !slices.Contains(chain, "") {
		tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: namespace, Name: name}})
	}
	return tiers
}

// from returns the tiers a name inside a module names: each candidate
// package of the module specifier, a namespace path inside it where
// inner names one, in the specifier's tiers.
func (m *module) from(specifier, inner, name string) plugin.Candidates {
	pkgs := m.packages(specifier)
	out := make(plugin.Candidates, 0, len(pkgs))
	for _, tier := range pkgs {
		ids := make([]symbol.Identity, 0, len(tier))
		for _, pkg := range tier {
			if inner != "" {
				pkg += "/" + inner
			}
			ids = append(ids, symbol.Identity{Lang: Lang, Package: pkg, Name: name})
		}
		out = append(out, ids)
	}
	return out
}

// local returns the tiers a name of the file's own scope names: the
// module an import binds it from, and the file's package otherwise.
func (m *module) local(name string) plugin.Candidates {
	if b, bound := m.imports[name]; bound && b.name != "" {
		return m.from(b.specifier, "", b.name)
	}
	return plugin.Candidates{{{Lang: Lang, Package: m.pkg, Name: name}}}
}

// published returns the tiers a name the module publishes and does not
// declare names: the default export and the export clauses that publish
// it, then the export-star modules.
func (m *module) published(name string) plugin.Candidates {
	var explicit []plugin.Candidates
	if name == defaultExport && m.defaultName != "" && m.defaultName != defaultExport {
		explicit = append(explicit, m.local(m.defaultName))
	}
	for _, r := range m.reexports {
		if r.published != name {
			continue
		}
		if r.specifier != "" {
			explicit = append(explicit, m.from(r.specifier, "", r.name))
		} else {
			explicit = append(explicit, m.local(r.name))
		}
	}
	var stars []plugin.Candidates
	if name != defaultExport {
		for _, spec := range m.stars {
			stars = append(stars, m.from(spec, "", name))
		}
	}
	return append(zip(explicit), zip(stars)...)
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
