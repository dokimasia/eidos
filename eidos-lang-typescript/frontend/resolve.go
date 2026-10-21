// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// globalThisName is the name of the global scope of TypeScript, which a
// qualified name such as globalThis.Error opens with.
const globalThisName = "globalThis"

// resolve returns what a spelling could mean in one file, in the scope
// order of TypeScript, through the bindings that the parse recorded. A
// file whose parse recorded no scope resolves nothing.
//
// A bare name probes the enclosing namespaces innermost first and then
// the package of the file, one tier each. A name that a named or a
// default import binds then probes the module of the import, and a name
// that a namespace import binds probes no further tier. An import hides
// the global scope from the name that it binds, also where the load does
// not contain the module. Any other bare name probes each package of the
// global scope last: the global package of the workspace and the package
// of the library. A qualified
// name probes the module that a namespace import binds its first name to,
// a namespace that a named import binds, or a namespace of each enclosing
// package and of each package of the global scope. A name that opens with
// globalThis probes the rest of the name in the packages of the global
// scope alone. An import alias resolves as the entity that it refers to.
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
// compete. Only the File of the module itself publishes, and export *
// publishes no default.
func exports(s plugin.ImportScope, name string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil || sc.own != sc.module.pkg {
		return nil
	}
	return sc.module.published(name)
}

// candidates returns the tiers of one spelling. followed contains the
// import aliases that the resolution already followed, so an alias that
// refers to itself, or a cycle of aliases, resolves to nothing.
func (s *scope) candidates(spelling string, followed map[string]bool) plugin.Candidates {
	if spelling == "" {
		return nil
	}
	m := s.module
	segments := strings.Split(spelling, namespaceSeparator)
	head, name := segments[0], segments[len(segments)-1]
	chain := s.chain()
	if len(segments) == 1 {
		tiers := make(plugin.Candidates, 0, len(chain)+1+len(m.globals))
		for _, pkg := range chain {
			tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: pkg, Name: name}})
		}
		if b, bound := m.imports[name]; bound {
			if b.name != "" {
				tiers = append(tiers, m.from(b.specifier, "", b.name)...)
			}
			return tiers
		}
		for _, global := range m.globals {
			if !slices.Contains(chain, global) {
				tiers = append(tiers, []symbol.Identity{{Lang: Lang, Package: global, Name: name}})
			}
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
		return m.from(b.specifier, typescript.JoinPackage(b.name, inner), name)
	}
	if head == globalThisName {
		tiers := make(plugin.Candidates, 0, len(m.globals))
		for _, global := range m.globals {
			tiers = append(tiers, []symbol.Identity{
				{Lang: Lang, Package: typescript.JoinPackage(global, inner), Name: name},
			})
		}
		return tiers
	}
	namespace := strings.Join(segments[:len(segments)-1], "/")
	tiers := make(plugin.Candidates, 0, len(chain)+len(m.globals))
	for _, pkg := range chain {
		tiers = append(tiers, []symbol.Identity{
			{Lang: Lang, Package: typescript.JoinPackage(pkg, namespace), Name: name},
		})
	}
	for _, global := range m.globals {
		if !slices.Contains(chain, global) {
			tiers = append(tiers, []symbol.Identity{
				{Lang: Lang, Package: typescript.JoinPackage(global, namespace), Name: name},
			})
		}
	}
	return tiers
}

// from returns the tiers of a name inside a module: each candidate
// package of the module specifier, with the namespace path inner inside
// it, in the tiers of the specifier.
func (m *module) from(specifier, inner, name string) plugin.Candidates {
	pkgs := m.packages(specifier)
	out := make(plugin.Candidates, 0, len(pkgs))
	for _, tier := range pkgs {
		ids := make([]symbol.Identity, 0, len(tier))
		for _, pkg := range tier {
			ids = append(ids, symbol.Identity{Lang: Lang, Package: typescript.JoinPackage(pkg, inner), Name: name})
		}
		out = append(out, ids)
	}
	return out
}

// local returns the tiers of a name of the scope of the file: the module
// that an import binds it from, and the package of the file otherwise.
func (m *module) local(name string) plugin.Candidates {
	if b, bound := m.imports[name]; bound && b.name != "" {
		return m.from(b.specifier, "", b.name)
	}
	return plugin.Candidates{{{Lang: Lang, Package: m.pkg, Name: name}}}
}

// published returns the tiers of a name that the module publishes and
// does not declare: the default export and the export clauses that
// publish it, then the export-star modules.
func (m *module) published(name string) plugin.Candidates {
	var explicit []plugin.Candidates
	if name == typescript.DefaultExport && m.defaultName != "" && m.defaultName != typescript.DefaultExport {
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
	if name != typescript.DefaultExport {
		for _, spec := range m.stars {
			stars = append(stars, m.from(spec, "", name))
		}
	}
	return append(plugin.Zip(explicit...), plugin.Zip(stars...)...)
}
