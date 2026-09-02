// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// index is what the resolution step reads: every assigned identity
// a type reference can target, under its bare spelling — kind and
// discriminator zeroed, because a resolver cannot know what kind a
// spelling declares — and the standing identity for every dropped
// duplicate.
//
// Only the kinds a type position can name are indexed: the type
// declarations and their variants. A language resolving a value
// position — TypeScript's typeof — widens the set when its
// frontend arrives, and the load bench prices the widening.
type index struct {
	byBare  map[symbol.Identity][]symbol.Identity
	dropped map[symbol.Symbol]symbol.Identity
}

// lookup returns the held identities a candidate names.
func (ix *index) lookup(c symbol.Identity) []symbol.Identity {
	return ix.byBare[bareOf(c)]
}

// add records one assigned identity under its bare spelling.
func (ix *index) add(full symbol.Identity) {
	bare := bareOf(full)
	ix.byBare[bare] = append(ix.byBare[bare], full)
}

// bareOf strips an identity to what a resolver can spell.
func bareOf(id symbol.Identity) symbol.Identity {
	return symbol.Identity{Lang: id.Lang, Package: id.Package, Owner: id.Owner, Name: id.Name}
}

// assign walks every package in splice order and gives each
// declaration its canonical identity, per the rules the model
// fixes:
//
//   - a package is lang:path, its identity's kind alone filled
//   - a file is lang:package/path, named by its whole
//     workspace-relative path, because two files of one base name
//     can meet in one package
//   - a top-level declaration is lang:package.Name
//   - a member is lang:package.Owner#Name, the owner the dotted
//     chain of enclosing type names; a top-level method's owner is
//     the spelling of the type it attaches to
//   - a callable's discriminator is its parameter type spellings,
//     comma-joined, so overloads spell apart
//
// Members get their Host filled beside the identity. A second
// declaration spelling one identity reports under
// [DuplicateDeclaration] and its subtree leaves every index, one
// finding for the subtree's root alone; the survivor's identity
// still stands for anything attached to the duplicate. Reparsing
// an unchanged file yields the same identities, which is what
// diff-by-identity later stands on.
func assign(packages []*spliced, sink *diag.Sink) *index {
	ix := &index{
		byBare:  map[symbol.Identity][]symbol.Identity{},
		dropped: map[symbol.Symbol]symbol.Identity{},
	}
	seen := map[symbol.Identity]symbol.Symbol{}
	for _, sp := range packages {
		a := &assigner{
			lang:   sp.lang,
			pkg:    sp.pkg.ID.Package,
			origin: sp.origin,
			seen:   seen,
			ix:     ix,
			sink:   sink,
		}
		for _, f := range sp.pkg.Files {
			a.file(f)
		}
	}
	for _, bucket := range ix.byBare {
		slices.SortFunc(bucket, symbol.Identity.Compare)
	}
	return ix
}

// assigner carries one package's assignment state.
type assigner struct {
	lang   symbol.Lang
	pkg    string
	origin diag.Origin
	seen   map[symbol.Identity]symbol.Symbol
	ix     *index
	sink   *diag.Sink
}

// file assigns a file's identity and descends into its
// declarations.
func (a *assigner) file(f *node.File) {
	if f.Path == "" {
		panic(fmt.Sprintf("load: %s built a file with no path in %s", a.origin, a.pkg))
	}
	id := symbol.Identity{Lang: a.lang, Package: a.pkg, Name: f.Path, Kind: symbol.KindFile}
	assigned, dropping := a.claim(f, id, f.Pos, false)
	f.ID = assigned
	for _, d := range f.Decls {
		a.decl(d, "", id, dropping)
	}
}

// claim settles one identity: recorded and returned, or — for a
// duplicate or a declaration inside a dropped subtree — zero, with
// the standing identity remembered for attachments. The boolean
// says whether the subtree continues dropping. Only a kind a type
// position can name enters the resolution index; everything enters
// the duplicate check.
func (a *assigner) claim(
	s symbol.Symbol, id symbol.Identity, at position.Pos, drop bool,
) (symbol.Identity, bool) {
	if drop {
		a.ix.dropped[s] = id
		return symbol.Identity{}, true
	}
	if first, held := a.seen[id]; held {
		a.sink.Warnf(DuplicateDeclaration, at, a.origin,
			"%s is declared twice; the first, at %s:%d, stands",
			id, first.Position().File, first.Position().Line)
		a.ix.dropped[s] = id
		return symbol.Identity{}, true
	}
	a.seen[id] = s
	switch id.Kind {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindEnum,
		symbol.KindSum, symbol.KindAlias,
		symbol.KindEnumVariant, symbol.KindSumVariant:
		a.ix.add(id)
	}
	return id, false
}

// decl assigns one declaration and its members.
//
// The owner is the dotted chain of enclosing type names, and host
// the enclosing declaration's identity — the one that stands, so a
// dropped duplicate's members still point at the survivor.
func (a *assigner) decl(s symbol.Symbol, owner string, host symbol.Identity, drop bool) {
	switch x := s.(type) {
	case *node.Function:
		id := a.derive(x, owner, x.Name, symbol.KindFunction, discOf(x.Params))
		x.ID, _ = a.claim(x, id, x.Pos, drop)

	case *node.Method:
		at := owner
		if at == "" && x.Receives != nil {
			at = x.Receives.Spelling
		}
		id := a.derive(x, at, x.Name, symbol.KindMethod, discOf(x.Params))
		x.ID, _ = a.claim(x, id, x.Pos, drop)
		x.Host = host

	case *node.Struct:
		id := a.derive(x, owner, x.Name, symbol.KindStruct, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.members(childOwner(owner, x.Name), id, dropping, x.Fields, x.Methods, x.Types, x.Embeds)

	case *node.Interface:
		id := a.derive(x, owner, x.Name, symbol.KindInterface, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.members(childOwner(owner, x.Name), id, dropping, x.Fields, x.Methods, x.Types, x.Embeds)

	case *node.Enum:
		id := a.derive(x, owner, x.Name, symbol.KindEnum, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.members(childOwner(owner, x.Name), id, dropping, x.Variants, x.Fields, x.Methods, nil)

	case *node.EnumVariant:
		id := a.derive(x, owner, x.Name, symbol.KindEnumVariant, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)
		x.Host = host

	case *node.Sum:
		id := a.derive(x, owner, x.Name, symbol.KindSum, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.members(childOwner(owner, x.Name), id, dropping, x.Variants, x.Methods, nil, nil)

	case *node.SumVariant:
		id := a.derive(x, owner, x.Name, symbol.KindSumVariant, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		x.Host = host
		a.members(childOwner(owner, x.Name), id, dropping, x.Fields, nil, nil, nil)

	case *node.Field:
		x.Host = host
		if x.Name == "" {
			return // positional: only its type names it, so nothing indexes it
		}
		id := a.derive(x, owner, x.Name, symbol.KindField, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)

	case *node.Variable:
		id := a.derive(x, owner, x.Name, symbol.KindVariable, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)

	case *node.Constant:
		id := a.derive(x, owner, x.Name, symbol.KindConstant, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)

	case *node.Alias:
		id := a.derive(x, owner, x.Name, symbol.KindAlias, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)

	case *node.Embed:
		x.Host = host // an embed is an edge; the reference names it

	case *node.Constraint:
		// The kind carries no name of its own, so nothing indexes it.

	default:
		panic(fmt.Sprintf(
			"load: %s built a %T in %s, which is not a node declaration",
			a.origin, s, a.pkg,
		))
	}
}

// members descends into up to four member lists, in order.
func (a *assigner) members(
	owner string, host symbol.Identity, drop bool, lists ...any,
) {
	for _, list := range lists {
		switch typed := list.(type) {
		case nil:
		case []*node.Field:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		case []*node.Method:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		case []*node.EnumVariant:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		case []*node.SumVariant:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		case []*node.Embed:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		case node.Symbols:
			for _, m := range typed {
				a.decl(m, owner, host, drop)
			}
		default:
			panic(fmt.Sprintf("load: a %T member list is not part of the model", list))
		}
	}
}

// derive spells one canonical identity, refusing a nameless
// declaration of a named kind as the structural defect it is.
func (a *assigner) derive(
	s symbol.Symbol, owner, name string, kind symbol.Kind, disc string,
) symbol.Identity {
	if name == "" {
		panic(fmt.Sprintf(
			"load: %s built a %s in %s that names nothing", a.origin, s.Kind(), a.pkg,
		))
	}
	return symbol.Identity{
		Lang:    a.lang,
		Package: a.pkg,
		Owner:   owner,
		Name:    name,
		Kind:    kind,
		Disc:    disc,
	}
}

// childOwner extends the dotted owner chain by one type name.
func childOwner(owner, name string) string {
	if owner == "" {
		return name
	}
	return owner + "." + name
}

// discOf spells a callable's discriminator: the parameter type
// spellings as written, comma-joined, so two overloads spell
// apart and a nullary callable spells empty.
func discOf(params []*node.Param) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		if p.Type != nil {
			parts[i] = p.Type.Spelling
		}
	}
	return strings.Join(parts, ",")
}
