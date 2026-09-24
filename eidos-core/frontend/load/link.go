// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// link resolves every type reference in the graph, nested type
// arguments included.
//
// A bare name that spells a type parameter in scope targets that
// parameter: the parameters of every enclosing declaration are in
// scope, the innermost first, so a parameter takes its name from a
// package-level type of the same spelling the way every language
// with generics scopes it. Every other reference resolves through
// the bindings of its file's language: the frontend's Resolve names
// the candidates in shadowing tiers, and the first tier with a
// candidate the graph contains decides. That candidate is the
// target, and several such candidates in the tier report under
// [AmbiguousReference]. A reference no tier resolves keeps its
// spelling alone: degradation a reader can ask about, not a
// failure.
//
// A file without a recorded scope resolves nothing: there are no
// bindings to resolve through, and its references keep their
// spellings.
func link(packages []*spliced, scopes []scopeEntry, ix *index, sink *diag.Sink) {
	byFile := make(map[*node.File]scopeEntry, len(scopes))
	for _, s := range scopes {
		byFile[s.file] = s
	}
	for _, sp := range packages {
		for _, f := range sp.pkg.Files {
			entry, held := byFile[f]
			if !held {
				continue
			}
			linkUnder(f, symbol.Identity{}, nil, entry, f.ID, ix, sink)
		}
	}
}

// linkUnder resolves the references directly inside one
// declaration, then descends into each nested declaration under its
// own identity.
//
// The owner is what a lexically scoped language resolves against:
// a name written inside a message resolves to that message's own
// nested types before it resolves outward. A declaration without an
// identity, which a dropped duplicate is, keeps its parent's owner,
// because its references are written inside the parent. The type
// parameters in scope map each parameter's name to its identity.
func linkUnder(
	s symbol.Symbol, owner symbol.Identity, params map[string]symbol.Identity,
	entry scopeEntry, file symbol.Identity, ix *index, sink *diag.Sink,
) {
	scope := plugin.ImportScope{File: file, Owner: owner, Bindings: entry.bindings}
	node.Walk(s, func(child symbol.Symbol) bool {
		if child == s {
			return true
		}
		// A reference resolves here, under this owner. A structural
		// one has no target of its own: the walk descends into its
		// children, and the named ones resolve.
		if ref, is := child.(*node.TypeRef); is {
			if ref.Form == symbol.FormNamed && ref.Spelling != "" && ref.Target.IsZero() {
				if id, spells := params[ref.Spelling]; spells && len(ref.Args) == 0 {
					ref.Target = id
				} else {
					resolve(ref, entry.frontend, scope, ix, sink)
				}
			}
			return true
		}
		declared := typeParamsOf(child)
		if !scoped(child) && len(declared) == 0 {
			return true
		}
		inner := owner
		if scoped(child) {
			if decl, names := child.(node.Declaration); names && !decl.Identity().IsZero() {
				inner = decl.Identity()
			}
		}
		linkUnder(child, inner, withParams(params, declared), entry, file, ix, sink)
		return false // the recursion walks this subtree
	})
}

// scoped reports whether a declaration opens a lexical scope a
// reference inside it resolves against first: the kinds that nest
// types. A member nests none, so a field's own reference resolves
// under the type that declares the field and not under the field.
func scoped(s symbol.Symbol) bool {
	switch s.(type) {
	case *node.Struct, *node.Interface, *node.Enum, *node.Sum:
		return true
	default:
		return false
	}
}

// typeParamsOf returns the type parameters a declaration declares,
// and nil for a kind that declares none.
func typeParamsOf(s symbol.Symbol) []*node.TypeParam {
	switch d := s.(type) {
	case *node.Struct:
		return d.TypeParams
	case *node.Interface:
		return d.TypeParams
	case *node.Alias:
		return d.TypeParams
	case *node.Sum:
		return d.TypeParams
	case *node.Function:
		return d.TypeParams
	case *node.Method:
		return d.TypeParams
	default:
		return nil
	}
}

// withParams returns the type parameters in scope inside a
// declaration: the enclosing ones, shadowed by the declaration's
// own of the same name. A parameter without an identity, which a
// dropped duplicate is, names nothing a reference could target.
// The enclosing map is never written, so a sibling declaration sees
// what its parent saw.
func withParams(
	enclosing map[string]symbol.Identity, declared []*node.TypeParam,
) map[string]symbol.Identity {
	if len(declared) == 0 {
		return enclosing
	}
	out := make(map[string]symbol.Identity, len(enclosing)+len(declared))
	maps.Copy(out, enclosing)
	for _, tp := range declared {
		if tp != nil && tp.Name != "" && !tp.ID.IsZero() {
			out[tp.Name] = tp.ID
		}
	}
	return out
}

// resolve settles one reference: the first tier with a candidate
// the graph contains decides, that candidate is the target, and
// several such candidates in the tier report as an ambiguity.
func resolve(
	ref *node.TypeRef, f plugin.Frontend, scope plugin.ImportScope,
	ix *index, sink *diag.Sink,
) {
	for _, tier := range f.Resolve(scope, ref.Spelling) {
		var hits []symbol.Identity
		for _, c := range tier {
			for _, full := range ix.lookup(c) {
				if !slices.Contains(hits, full) {
					hits = append(hits, full)
				}
			}
		}
		if len(hits) == 0 {
			continue
		}
		ref.Target = hits[0]
		if len(hits) > 1 {
			names := make([]string, len(hits))
			for i, h := range hits {
				names[i] = h.String()
			}
			sink.Warnf(AmbiguousReference, ref.Pos, f.Name(),
				"%q resolves to %s, and the first is the target", ref.Spelling, strings.Join(names, " and "))
		}
		return
	}
}
