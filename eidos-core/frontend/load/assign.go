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

// index is what the resolution step reads. It maps:
//
//   - every assigned identity a type reference can target, under its
//     bare spelling, with kind and discriminator zeroed, because a
//     resolver cannot know what kind a spelling declares
//   - every dropped duplicate, to the identity of the declaration
//     that is kept
//   - every indexed identity a [plugin.Importer] language declares,
//     to the workspace path of the file that declares it
//
// Only the kinds a type position can name are indexed: the type
// declarations and their variants. A value position, such as
// TypeScript's typeof, therefore resolves nothing.
//
// The index contains the declarations the load assigned. kept looks up the
// declarations of the units the load keeps or restores, and is nil
// where it keeps none.
type index struct {
	byBare  map[symbol.Identity][]symbol.Identity
	dropped map[symbol.Symbol]symbol.Identity
	files   map[symbol.Identity]string
	kept    *keptIndex
}

// lookup returns the declarations a candidate names, in identity order:
// the ones the load assigned and the ones of the units it keeps.
func (ix *index) lookup(c symbol.Identity) []symbol.Identity {
	bare := bareOf(c)
	assigned := ix.byBare[bare]
	if ix.kept == nil {
		return assigned
	}
	kept := ix.kept.lookup(bare)
	if len(kept) == 0 {
		return assigned
	}
	if len(assigned) == 0 {
		return kept
	}
	merged := slices.Concat(assigned, kept)
	slices.SortFunc(merged, symbol.Identity.Compare)
	return merged
}

// file returns the workspace path of the file that declares an indexed
// identity of an importing language, and the empty string for any other
// identity.
func (ix *index) file(id symbol.Identity) string {
	if f := ix.files[id]; f != "" || ix.kept == nil {
		return f
	}
	return ix.kept.file(id)
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
//   - a callable's discriminator is its parameters' spellings,
//     comma-joined, where the frontend reports that its language
//     overloads, so overloads spell apart, and empty where it does
//     not. [assigner.disc] states how one parameter spells
//
// Members get their Host filled beside the identity. A second
// declaration spelling one identity reports under
// [DuplicateDeclaration], one finding for the subtree's root alone,
// recorded as a finding of the unit that declares it, and its subtree
// leaves every index. Anything attached to the duplicate attaches to
// the identity of the declaration that is kept. Reparsing an unchanged
// file yields the same identities, which diff-by-identity depends on.
func assign(packages []*spliced, sink *diag.Sink) *index {
	ix := &index{
		byBare:  map[symbol.Identity][]symbol.Identity{},
		dropped: map[symbol.Symbol]symbol.Identity{},
		files:   map[symbol.Identity]string{},
	}
	seen := map[symbol.Identity]symbol.Symbol{}
	for _, sp := range packages {
		a := &assigner{
			lang:      sp.lang,
			pkg:       sp.pkg.ID.Package,
			origin:    sp.origin,
			importer:  sp.importer,
			overloads: sp.overloads,
			units:     sp.units,
			seen:      seen,
			ix:        ix,
			sink:      sink,
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

// assigner is one package's assignment state. Where importer is set,
// the language's frontend is a [plugin.Importer], and every indexed
// identity records path, the file under assignment. Where overloads
// is set, the language's frontend reports that it overloads, and a
// callable's discriminator spells its parameters. unit is the unit of
// the file under assignment, which a duplicate's finding is recorded
// on.
type assigner struct {
	lang      symbol.Lang
	pkg       string
	origin    diag.Origin
	importer  bool
	overloads bool
	units     map[*node.File]*unit
	unit      *unit
	path      string
	seen      map[symbol.Identity]symbol.Symbol
	ix        *index
	sink      *diag.Sink
}

// file assigns a file's identity and descends into its
// declarations.
func (a *assigner) file(f *node.File) {
	if f.Path == "" {
		panic(fmt.Sprintf("load: %s built a file with no path in %s", a.origin, a.pkg))
	}
	a.path = f.Path
	a.unit = a.units[f]
	id := symbol.Identity{Lang: a.lang, Package: a.pkg, Name: f.Path, Kind: symbol.KindFile}
	assigned, dropping := a.claim(f, id, f.Pos, false)
	f.ID = assigned
	for _, d := range f.Decls {
		a.decl(d, "", id, dropping)
	}
}

// claim settles one identity. It records and returns the identity,
// or returns zero for a duplicate or a declaration inside a dropped
// subtree and remembers the kept identity for attachments. The
// boolean reports whether the subtree continues dropping. Only a
// kind a type position can name enters the resolution index, and
// every kind enters the duplicate check.
func (a *assigner) claim(
	s symbol.Symbol, id symbol.Identity, at position.Pos, drop bool,
) (symbol.Identity, bool) {
	if drop {
		a.ix.dropped[s] = id
		return symbol.Identity{}, true
	}
	if first, held := a.seen[id]; held {
		a.unit.warnf(a.sink, DuplicateDeclaration, at, a.origin,
			"%s is declared twice, and the first, at %s:%d, is kept",
			id, first.Position().File, first.Position().Line)
		a.ix.dropped[s] = id
		return symbol.Identity{}, true
	}
	a.seen[id] = s
	if targetable(id.Kind) {
		a.ix.add(id)
		if a.importer {
			a.ix.files[id] = a.path
		}
	}
	return id, false
}

// decl assigns one declaration and its members.
//
// The owner is the dotted chain of enclosing type names, and host
// the identity of the enclosing declaration that is kept, so the
// members of a dropped duplicate point at the kept declaration.
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
// a blank such as Go's or Rust's _ repeats. Each takes the
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
// owner chain, each with the host's discriminator.
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

// disc spells a callable's discriminator. In a language that
// overloads it is the parameters' spellings, comma-joined, so two
// overloads spell apart and a nullary callable spells empty. A
// parameter spells its type as the frontend wrote it, with an
// instantiation's arguments in the angle brackets every language that
// overloads writes, a ... prefix where the parameter is positionally
// variadic, a ** prefix where it is variadic by keyword, and a ? suffix
// where it is optional: each changes which calls the callable accepts.
// In a language that cannot overload it is empty. A nil parameter
// panics with [assigner.nilSlot] in either language, before anything
// reads it.
func (a *assigner) disc(params []*node.Param) string {
	if slices.Contains(params, nil) {
		panic(a.nilSlot(symbol.KindParam))
	}
	if !a.overloads {
		return ""
	}
	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteString(discSeparator)
		}
		switch p.Variadic {
		case symbol.VariadicPositional:
			b.WriteString(discPositional)
		case symbol.VariadicKeyword:
			b.WriteString(discKeyword)
		}
		discType(&b, p.Type)
		if p.Optional {
			b.WriteString(discOptional)
		}
	}
	return b.String()
}

// The marks a discriminator spells between and around its parameters.
const (
	discSeparator  = ","
	discPositional = "..."
	discKeyword    = "**"
	discOptional   = "?"
	discArgsOpen   = "<"
	discArgsClose  = ">"
)

// discType spells one parameter type into a discriminator: the
// reference's spelling, and an instantiation's arguments after it in
// angle brackets, each spelled the same way. A nil type spells
// nothing.
func discType(b *strings.Builder, t *node.TypeRef) {
	if t == nil {
		return
	}
	b.WriteString(t.Spelling)
	if len(t.Args) == 0 {
		return
	}
	b.WriteString(discArgsOpen)
	for i, arg := range t.Args {
		if i > 0 {
			b.WriteString(discSeparator)
		}
		discType(b, arg)
	}
	b.WriteString(discArgsClose)
}
