// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"slices"
	"strconv"
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
		id := a.derive(x, owner, x.Name, symbol.KindFunction, a.disc(x.Params))
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.signature(childOwner(owner, x.Name), id, dropping, x.TypeParams, x.Params, x.Returns)

	case *node.Method:
		at := owner
		if at == "" && x.Receives != nil {
			at = x.Receives.Spelling
		}
		id := a.derive(x, at, x.Name, symbol.KindMethod, a.disc(x.Params))
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		x.Host = host
		a.signature(childOwner(at, x.Name), id, dropping, x.TypeParams, x.Params, x.Returns)

	case *node.Struct:
		id := a.derive(x, owner, x.Name, symbol.KindStruct, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.typeParams(childOwner(owner, x.Name), id, dropping, x.TypeParams)
		a.members(childOwner(owner, x.Name), id, dropping, x.Fields, x.Methods, x.Types, x.Embeds)

	case *node.Interface:
		id := a.derive(x, owner, x.Name, symbol.KindInterface, "")
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.typeParams(childOwner(owner, x.Name), id, dropping, x.TypeParams)
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
		a.typeParams(childOwner(owner, x.Name), id, dropping, x.TypeParams)
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
		var dropping bool
		x.ID, dropping = a.claim(x, id, x.Pos, drop)
		a.typeParams(childOwner(owner, x.Name), id, dropping, x.TypeParams)

	case *node.Embed:
		// An embed is named by the embedded type's bare name, which
		// is the field name the language promotes it under and the
		// subject a directive on it attaches to.
		id := a.derive(x, owner, embedName(x.Ref), symbol.KindEmbed, "")
		x.ID, _ = a.claim(x, id, x.Pos, drop)
		x.Host = host

	default:
		panic(fmt.Sprintf(
			"load: %s built a %T in %s, which is not a node declaration",
			a.origin, s, a.pkg,
		))
	}
}

// signature assigns a callable's type parameters, parameters and
// returns under the callable's owner chain. A parameter or a return
// the language leaves unnamed is named by its position, so two
// unnamed ones spell apart and no written name collides with the
// spelling. A slot repeating a name an earlier slot of the same list
// wrote is named by its position too, because in valid source only
// a blank such as Go's or Rust's _ repeats. Each carries the
// callable's discriminator, so the parameters of two overloads
// spell apart too. [assigner.disc] refuses a nil parameter before
// this runs.
func (a *assigner) signature(
	owner string, host symbol.Identity, drop bool,
	tps []*node.TypeParam, params []*node.Param, returns []*node.Return,
) {
	a.typeParams(owner, host, drop, tps)
	for i, p := range params {
		name := p.Name
		for _, earlier := range params[:i] {
			if earlier.Name == name {
				name = ""
				break
			}
		}
		id := a.derive(p, owner, positional(name, i), symbol.KindParam, host.Disc)
		p.ID, _ = a.claim(p, id, p.Pos, drop)
	}
	for i, r := range returns {
		if r == nil {
			panic(a.nilSlot(symbol.KindReturn))
		}
		name := r.Name
		for _, earlier := range returns[:i] {
			if earlier.Name == name {
				name = ""
				break
			}
		}
		id := a.derive(r, owner, positional(name, i), symbol.KindReturn, host.Disc)
		r.ID, _ = a.claim(r, id, r.Pos, drop)
	}
}

// typeParams assigns a declaration's type parameters under its
// owner chain, carrying the host's discriminator.
func (a *assigner) typeParams(owner string, host symbol.Identity, drop bool, tps []*node.TypeParam) {
	for _, tp := range tps {
		if tp == nil {
			panic(a.nilSlot(symbol.KindTypeParam))
		}
		id := a.derive(tp, owner, tp.Name, symbol.KindTypeParam, host.Disc)
		tp.ID, _ = a.claim(tp, id, tp.Pos, drop)
	}
}

// nilSlot spells the panic a nil entry in a signature's lists
// raises, naming the frontend: a structural defect in what it
// built, because the store cannot index a nil declaration.
func (a *assigner) nilSlot(kind symbol.Kind) string {
	return fmt.Sprintf("load: %s built a nil %s in %s", a.origin, kind, a.pkg)
}

// positional returns a written name, or the position's spelling
// for an unnamed parameter or return: a hash and the index, which
// no language admits as an identifier.
func positional(name string, i int) string {
	if name != "" {
		return name
	}
	return positionalPrefix + strconv.Itoa(i)
}

// positionalPrefix leads the spelling of an unnamed parameter's or
// return's name.
const positionalPrefix = "#"

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

// embedName spells the name an embedded type contributes: the
// named reference under the form's decoration, its spelling after
// the last qualifier, which is the bare name for an instantiation
// and the whole spelling for a name. A frontend that leaves a
// decorated spelling on a Named reference strips the same way,
// the leading pointer or borrow marks first.
func embedName(ref *node.TypeRef) string {
	for ref != nil && ref.Form != symbol.FormNamed && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	if ref == nil {
		return ""
	}
	name := strings.TrimLeft(ref.Spelling, "*&")
	if at := strings.LastIndexByte(name, '.'); at >= 0 {
		name = name[at+1:]
	}
	if at := strings.IndexByte(name, '['); at >= 0 {
		name = name[:at]
	}
	return name
}

// childOwner extends the dotted owner chain by one type name.
func childOwner(owner, name string) string {
	if owner == "" {
		return name
	}
	return owner + "." + name
}

// disc spells a callable's discriminator: the parameter type
// spellings as written, comma-joined, so two overloads spell
// apart and a nullary callable spells empty. A nil parameter
// panics with [assigner.nilSlot] before anything reads it.
func (a *assigner) disc(params []*node.Param) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		if p == nil {
			panic(a.nilSlot(symbol.KindParam))
		}
		if p.Type != nil {
			parts[i] = p.Type.Spelling
		}
	}
	return strings.Join(parts, ",")
}
