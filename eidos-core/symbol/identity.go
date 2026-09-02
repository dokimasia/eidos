// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol

import (
	"cmp"
	"fmt"
	"strings"
)

// Lang is the source-language name as identities, metadata
// namespaces and the sealed state store it. It is a registered
// name: workspace Build validates every spelling against the
// language registry.
type Lang string

// Identity names a declaration across runs, machines and reparses.
// It is the join everything long-lived keys on: read sets, exports,
// manifests, drift and explain.
//
// Equality is the whole struct. Overloads differ only in Disc, and
// a language that cannot overload writes the empty string. Lang is
// part of the key because a mixed workspace can hold two languages'
// packages in one directory.
type Identity struct {
	Lang    Lang   `json:"lang,omitzero"` // "golang", "protobuf"
	Package string `json:"package,omitzero"`
	Owner   string `json:"owner,omitzero"` // enclosing type; "" at top level
	Name    string `json:"name,omitzero"`
	Kind    Kind   `json:"kind,omitzero"`
	Disc    string `json:"disc,omitzero"` // "" where overloads cannot exist
}

// IsZero reports whether id names nothing.
func (id Identity) IsZero() bool { return id == Identity{} }

// String renders the identity in the canonical grammar:
//
//   - package:  lang:package
//   - file:     lang:package/file
//   - toplevel: lang:package.Name
//   - member:   lang:package.Owner#Name
//
// Callable kinds ([KindFunction], [KindMethod]) always append the
// parenthesized discriminator, even when it is empty, so a field
// and a nullary method with one name spell differently.
func (id Identity) String() string {
	// The parts are written into one buffer rather than concatenated
	// in steps, because an identity is spelled on every ordering the
	// kernel makes deterministic.
	var out strings.Builder
	out.Grow(len(id.Lang) + len(id.Package) + len(id.Owner) + len(id.Name) + len(id.Disc) + 4)
	out.WriteString(string(id.Lang))
	out.WriteByte(':')
	out.WriteString(id.Package)

	if id.Kind == KindFile {
		out.WriteByte('/')
		out.WriteString(id.Name)
		return out.String()
	}
	if id.Name == "" {
		return out.String()
	}

	out.WriteByte('.')
	if id.Owner != "" {
		out.WriteString(id.Owner)
		out.WriteByte('#')
	}
	out.WriteString(id.Name)
	if id.Kind == KindFunction || id.Kind == KindMethod {
		out.WriteByte('(')
		out.WriteString(id.Disc)
		out.WriteByte(')')
	}
	return out.String()
}

// Compare orders two identities, returning a negative number, zero
// or a positive one as id sorts before, with, or after other.
//
// The order is field order: language, package, owner, name, kind,
// discriminator. It exists so that a caller sorting identities for a
// deterministic output compares the parts rather than
// [Identity.String], which builds a string per comparison.
func (id Identity) Compare(other Identity) int {
	return cmp.Or(
		cmp.Compare(id.Lang, other.Lang),
		cmp.Compare(id.Package, other.Package),
		cmp.Compare(id.Owner, other.Owner),
		cmp.Compare(id.Name, other.Name),
		cmp.Compare(id.Kind, other.Kind),
		cmp.Compare(id.Disc, other.Disc),
	)
}

// Parse reads an identity from the canonical grammar.
//
// It fills Kind only where the string form determines it: a bare
// path is [KindPackage], and a parenthesized discriminator makes a
// callable ([KindFunction] without an owner, [KindMethod] with
// one). Every other form leaves [KindInvalid], which no reader
// recovers: the store keys on the whole identity, so a parsed
// identity missing its kind matches nothing. A caller parsing a
// spelling supplies the kind it expects.
//
// Two forms do not round-trip. A file identity's string form is
// indistinguishable from a dotted top-level name and parses as
// one. So is a package whose last segment carries a dot —
// gopkg.in/yaml.v2 — because the grammar cuts a path at the first
// dot after the last slash: the package and the name it spells
// are recovered wrong, and String is not injective across that
// pair. Nothing in the framework parses identities back today;
// a boundary that starts to must state which half it holds.
func Parse(s string) (Identity, error) {
	lang, rest, found := strings.Cut(s, ":")
	if !found || lang == "" || rest == "" {
		return Identity{}, fmt.Errorf("symbol: parse %q: want \"lang:package...\"", s)
	}
	id := Identity{Lang: Lang(lang)}

	// Package, owner and name never contain '(', so the first one
	// starts the discriminator.
	callable := false
	if i := strings.IndexByte(rest, '('); i >= 0 {
		if !strings.HasSuffix(rest, ")") {
			return Identity{}, fmt.Errorf("symbol: parse %q: unclosed discriminator", s)
		}
		id.Disc = rest[i+1 : len(rest)-1]
		rest = rest[:i]
		callable = true
	}

	if left, member, isMember := strings.Cut(rest, "#"); isMember {
		pkg, owner, hasOwner := splitName(left)
		if !hasOwner || owner == "" || member == "" {
			return Identity{}, fmt.Errorf("symbol: parse %q: member form wants \"lang:package.Owner#Name\"", s)
		}
		id.Package, id.Owner, id.Name = pkg, owner, member
		if callable {
			id.Kind = KindMethod
		}
		return id, nil
	}

	pkg, name, hasName := splitName(rest)
	if !hasName {
		if callable {
			return Identity{}, fmt.Errorf("symbol: parse %q: discriminator without a name", s)
		}
		id.Package = pkg
		id.Kind = KindPackage
		return id, nil
	}
	if name == "" {
		return Identity{}, fmt.Errorf("symbol: parse %q: empty name after '.'", s)
	}
	id.Package, id.Name = pkg, name
	if callable {
		id.Kind = KindFunction
	}
	return id, nil
}

// splitName cuts a path at the first '.' after the last '/': the
// package half against the name half. It reports false when the
// final segment carries no dot, which is the bare package form.
func splitName(path string) (pkg, name string, found bool) {
	slash := strings.LastIndexByte(path, '/')
	dot := strings.IndexByte(path[slash+1:], '.')
	if dot < 0 {
		return path, "", false
	}
	dot += slash + 1
	return path[:dot], path[dot+1:], true
}
