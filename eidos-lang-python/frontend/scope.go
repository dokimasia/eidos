// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// builtinsModule is the module whose names every module reads without an
// import, the last scope of a name.
const builtinsModule = "builtins"

// spareCandidates is the number of candidates that a spelling can have
// beyond one for each separator of its owner chain, one for each dot of the
// spelling and one for each star import. The three are the innermost class
// of the chain, the module or the class that an import names, and builtins.
const spareCandidates = 3

// binding is what an import binds one local name to: a module, and the name
// of a declaration of the module where the import names one. An import of a
// module binds the module itself, and its name is empty.
type binding struct {
	module string
	name   string
}

// scope is what the File node of one module records for the resolution
// step: the package of the module, what its imports bind by the local name,
// the modules that its imports name, the modules of its star imports in
// statement order, and the names that it declares at its top level. It is
// immutable once the parse returns, and safe for concurrent use.
type scope struct {
	pkg      string
	imports  map[string]binding
	modules  map[string]bool
	stars    []string
	declared map[string]bool
}

// newScope returns the empty record of the module of a package. Its set of
// declared names is nil until the lowering sizes it.
func newScope(pkg string) *scope {
	return &scope{pkg: pkg, imports: map[string]binding{}, modules: map[string]bool{}}
}

// candidates returns the tiers of one spelling written in the class owner,
// in Python's order of the scopes of an annotation, and the zero owner at
// the top level of the module:
//
//   - The bodies of the classes that enclose the spelling come first,
//     innermost first, one tier each, because Python evaluates the
//     annotations of a class body in the namespace of the class.
//   - The module follows in one tier: a declaration of the module, or the
//     declaration that an import binds the name to.
//   - Each star import follows in a tier of its own, in statement order.
//   - The module builtins is the last tier of a name without a dot.
//
// A dotted name resolves to a member of the class that its first name is,
// and through a module that an import binds its first name to, to the name
// in the longest module of its names first. A name that from a import c
// binds is the declaration c of the module a, and then the module a/c.
func (s *scope) candidates(owner symbol.Identity, spelling string) plugin.Candidates {
	if spelling == "" {
		return nil
	}
	head, rest, dotted := strings.Cut(spelling, python.ModuleSep)
	inner, name, _ := strings.CutLast(spelling, python.ModuleSep)
	if !dotted {
		inner, name = "", spelling
	}
	chain := chainOf(owner)
	ids := make([]symbol.Identity, 0,
		strings.Count(chain, symbol.OwnerSep)+strings.Count(spelling, python.ModuleSep)+len(s.stars)+spareCandidates)
	for ; chain != ""; chain = outerChain(chain) {
		ids = append(ids, symbol.Identity{Lang: Lang, Package: s.pkg, Owner: joinOwner(chain, inner), Name: name})
	}
	switch b, bound := s.imports[head]; {
	case s.declared[head]:
		ids = append(ids, symbol.Identity{Lang: Lang, Package: s.pkg, Owner: inner, Name: name})
	case bound && !dotted && b.name != "":
		ids = append(ids, symbol.Identity{Lang: Lang, Package: b.module, Name: b.name})
	case bound && dotted:
		ids = through(ids, b, rest)
	}
	for _, star := range s.stars {
		ids = append(ids, symbol.Identity{Lang: Lang, Package: star, Owner: inner, Name: name})
	}
	if !dotted {
		ids = append(ids, symbol.Identity{Lang: Lang, Package: builtinsModule, Name: name})
	}
	return oneEach(ids)
}

// published returns the tiers of a name that the module publishes and does
// not declare: the declaration that an import binds the name to, and then
// the name in each star import, one tier each in statement order. An import
// binds a name of the module at run time, so a name of a package that its
// __init__.py imports resolves through the package.
func (s *scope) published(name string) plugin.Candidates {
	if s.declared[name] {
		return nil
	}
	ids := make([]symbol.Identity, 0, len(s.stars)+1)
	if b, bound := s.imports[name]; bound && b.name != "" {
		ids = append(ids, symbol.Identity{Lang: Lang, Package: b.module, Name: b.name})
	}
	for _, star := range s.stars {
		ids = append(ids, symbol.Identity{Lang: Lang, Package: star, Name: name})
	}
	return oneEach(ids)
}

// canonical returns the module and the name that a dotted name means in the
// module. Through an import that binds its first name, the module is the
// module of the import and the name is the name in it, as typing and
// Optional for t.Optional after import typing as t. A name that the module
// declares is in the module itself, and a name without a dot that no import
// binds is in builtins. typing_extensions reads as typing, because it
// declares the same forms. A dotted name whose first name neither an import
// nor the module binds has the empty module.
func (s *scope) canonical(dotted string) (module, name string) {
	head, rest, nested := strings.Cut(dotted, python.ModuleSep)
	b, bound := s.imports[head]
	switch {
	case s.declared[head]:
		module, name = s.pkg, dotted
	case !bound && !nested:
		module, name = builtinsModule, dotted
	case !bound:
		return "", dotted
	case b.name != "" && !nested:
		module, name = b.module, b.name
	case b.name != "":
		module, name = b.module, b.name+python.ModuleSep+rest
	default:
		module, name = b.module, rest
		if path, last, deeper := strings.CutLast(rest, python.ModuleSep); deeper {
			module = joinModule(b.module, strings.ReplaceAll(path, python.ModuleSep, python.PackageSep))
			name = last
		}
	}
	if module == typingExtensions {
		module = typingModule
	}
	return module, name
}

// packageOf returns the package that the import of a reference names: the
// module that an import binds the first name of the spelling to, and the
// longest module among the modules that the imports name for the names of a
// dotted spelling through it. A name that the module declares, and a name
// that no import binds, have the empty package.
func (s *scope) packageOf(spelling string) string {
	head, rest, dotted := strings.Cut(spelling, python.ModuleSep)
	b, bound := s.imports[head]
	if !bound || s.declared[head] {
		return ""
	}
	if b.name != "" || !dotted {
		return b.module
	}
	module := b.module
	for {
		segment, more, nested := strings.Cut(rest, python.ModuleSep)
		next := joinModule(module, segment)
		if !nested || !s.modules[next] {
			return module
		}
		module, rest = next, more
	}
}

// through appends to ids the candidates of the names after the first name
// of a dotted spelling, rest, where the import b binds the first name: the
// member of the class that the import names, and then the name in each
// module that the names can name, the longest module first.
func through(ids []symbol.Identity, b binding, rest string) []symbol.Identity {
	path, last, nested := strings.CutLast(rest, python.ModuleSep)
	if !nested {
		path, last = "", rest
	}
	module := b.module
	if b.name != "" {
		module = joinModule(b.module, b.name)
		ids = append(ids, symbol.Identity{Lang: Lang, Package: b.module, Owner: joinOwner(b.name, path), Name: last})
	}
	for prefix := path; ; {
		pkg := module
		if prefix != "" {
			pkg = joinModule(module, strings.ReplaceAll(prefix, python.ModuleSep, python.PackageSep))
		}
		owner := strings.TrimPrefix(strings.TrimPrefix(path, prefix), python.ModuleSep)
		ids = append(ids, symbol.Identity{Lang: Lang, Package: pkg, Owner: owner, Name: last})
		if prefix == "" {
			return ids
		}
		prefix = prefix[:max(strings.LastIndex(prefix, python.ModuleSep), 0)]
	}
}

// oneEach returns the tiers of a list of candidates, one candidate in each
// tier and in order, and nil for no candidate. The tiers share the list, so
// a resolution allocates the list and its tiers alone.
func oneEach(ids []symbol.Identity) plugin.Candidates {
	if len(ids) == 0 {
		return nil
	}
	tiers := make(plugin.Candidates, len(ids))
	for i := range ids {
		tiers[i] = ids[i : i+1 : i+1]
	}
	return tiers
}

// chainOf returns the owner chain of the members of a class owner, as
// Outer.Inner for the class Inner inside Outer, and the empty chain for the
// zero owner.
func chainOf(owner symbol.Identity) string {
	if owner.IsZero() {
		return ""
	}
	return joinOwner(owner.Owner, owner.Name)
}

// outerChain returns the owner chain of the class that encloses the last
// class of a chain, and the empty chain for a class at the top level.
func outerChain(chain string) string {
	outer, _, nested := strings.CutLast(chain, symbol.OwnerSep)
	if !nested {
		return ""
	}
	return outer
}

// joinOwner joins two parts of an owner chain, either of which can be
// empty.
func joinOwner(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + symbol.OwnerSep + b
	}
}

// joinModule returns the slash path of the module name inside the module
// parent, and name alone inside the empty module.
func joinModule(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + python.PackageSep + name
}
