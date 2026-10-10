// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The vocabulary names are constants, so a template and its helper
// spell one name.
const (
	// FuncDocs writes a declaration's documentation.
	FuncDocs = "docs"
	// FuncSpell writes a type reference.
	FuncSpell = "spell"
	// FuncTypeParams writes a type parameter list.
	FuncTypeParams = "typeparams"
	// FuncParams writes a parameter list.
	FuncParams = "params"
	// FuncResults writes a return type.
	FuncResults = "results"
	// FuncReturns writes a callable's return annotation.
	FuncReturns = "returns"
	// FuncMods writes a module-level declaration's keywords.
	FuncMods = "mods"
	// FuncMemberMods writes a class member's keywords.
	FuncMemberMods = "membermods"
	// FuncIndexMods writes a class index signature's keywords.
	FuncIndexMods = "indexmods"
	// FuncCtorMods writes a class constructor's keywords.
	FuncCtorMods = "ctormods"
	// FuncPropMods writes an interface property's keywords.
	FuncPropMods = "propmods"
	// FuncSigMods guards an interface method signature.
	FuncSigMods = "sigmods"
	// FuncBinding writes a module-level binding's keyword.
	FuncBinding = "binding"
	// FuncDecorators writes a declaration's decorator lines.
	FuncDecorators = "decorators"
	// FuncHeritage writes a type's heritage clauses.
	FuncHeritage = "heritage"
	// FuncAccessor writes a method's accessor keyword.
	FuncAccessor = "accessor"
	// FuncHard writes a member's hard-private prefix.
	FuncHard = "hard"
	// FuncIndexSig writes an index signature whole.
	FuncIndexSig = "indexsig"
	// FuncMethodKey writes a method's member key, quoted where the
	// name is no identifier.
	FuncMethodKey = "methodkey"
	// FuncPropKey writes a field's property key, quoted where the
	// name is not an identifier.
	FuncPropKey = "propkey"
	// FuncEnumKey writes an enum member's key, quoted where the name
	// is not an identifier.
	FuncEnumKey = "enumkey"
)

// Anonymous is what TypeScript writes where a declaration states
// no type: the type that admits everything while letting nothing
// through unchecked.
const Anonymous = "unknown"

// The brackets TypeScript writes an argument list in, and the
// separator of a qualified name.
const (
	argsOpener   = "<"
	argsCloser   = ">"
	qualifierSep = "."
)

// The keywords a module-level binding opens with.
const (
	constBinding = "const"
	letBinding   = "let"
)

// annotationSep, promiseOpen and promiseClose are the spellings an
// async callable's annotation joins, and voidType the result type of a
// callable that returns nothing.
const (
	annotationSep = ": "
	promiseType   = "Promise"
	promiseOpen   = promiseType + argsOpener
	promiseClose  = argsCloser
	voidType      = "void"
)

// The marks of a relative import specifier. currentDir opens a
// specifier of a module in the directory of the file under render or
// below it, parentDir climbs one directory, and pathSep separates the
// segments of a module path.
const (
	currentDir = "./"
	parentDir  = "../"
	pathSep    = "/"
)

// globalTypes lists the global types of TypeScript that a reference can
// write without an import. A declaration of the file with one of these
// names hides the global type.
var globalTypes = []string{
	"Array", "AsyncIterable", "Date", "Map", "Partial", promiseType, "ReadonlyArray", "Record", "Set", "Uint8Array",
}

// discardName is the name an unnamed parameter binds, numbered from
// the second on.
const discardName = "_"

// refusalPrefix opens every refusal the backend returns: the
// language's identity, as every backend's refusals open.
const refusalPrefix = string(typescript.Lang) + ": "

// Speller spells type references for one file. A reference to a
// declaration of another module imports the declaration through the
// file's import set, under its own name or under a numbered one where
// another import or a declaration of the file takes that name. The
// spelling then uses the bound name. The import specifier follows these
// rules:
//
//   - A reference to a TypeScript declaration imports the module path of
//     the declaration relative to the file's own, as ./store or
//     ../api/user. Where its source wrote a bare specifier, such as react
//     for an ambient module, it imports that specifier as written.
//   - A translated reference imports the module path that the layout
//     recorded, relative to the file's own.
//   - A reference without a target imports the specifier that its source
//     wrote.
//
// A reference to a declaration of the file's own module imports nothing.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text it writes and nothing else, so a name
// spelled as written allocates nothing. A reference that imports a
// module path allocates its relative specifier, and binding a
// declaration's import for the first time allocates in the set. A
// method that joins more than two parts also allocates its list of
// parts, because Go places a list of at most two strings on the stack. A
// trailing comment allocates its block comment and the part it ends. A
// refusal allocates its error.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller for the file whose import set is
// set. The set is not nil. It allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes a type reference in a type position, and imports what
// it names for the type checker alone. The source spelling passes
// through where the reference names nothing another module declares,
// and a missing one spells [Anonymous]. A reference with arguments
// has its bare name in Spelling, and the argument list spells here in
// angle brackets. A structural reference keeps its written spelling,
// and Spell returns an error where a type inside it imports under
// another name, because TypeScript's composite spelling does not
// follow from its structure. Spell refuses a global type that a
// declaration of the file hides, because the spelling would refer to
// that declaration.
//
// Spell allocates nothing for a name spelled as written, a missing
// reference and a structural reference. An imported declaration
// allocates its relative specifier where it imports a module path. A
// reference with arguments writes into one buffer, one allocation where
// every bound name is as long as its written spelling. A member of an
// imported declaration allocates the member behind the bound name.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	return s.spell(t, true)
}

// IndexSig writes an index signature whole: the one key parameter
// in brackets, the element type behind the colon. It refuses what
// an index signature cannot state: more parameters, no result, type
// parameters or an accessor. It allocates the signature, one
// allocation, and what the key's and the element's spellings allocate.
func (s Speller) IndexSig(m *emit.Method) (string, error) {
	switch {
	case len(m.Params) != 1 || m.Params[0].Name == "" || m.Params[0].Type == nil:
		return "", refuse("an index signature takes one named, typed key, and %s does not", m.Name)
	case len(m.Returns) != 1:
		return "", refuse("an index signature states one element type, and %s does not", m.Name)
	case len(m.TypeParams) != 0 || m.Accessor != symbol.AccessorNone:
		return "", refuse("an index signature admits no type parameters and no accessor, "+
			"and %s states one", m.Name)
	}
	key, err := s.Spell(m.Params[0].Type)
	if err != nil {
		return "", err
	}
	elem, err := s.Spell(m.Returns[0].Type)
	if err != nil {
		return "", err
	}
	return "[" + m.Params[0].Name + ": " + key + "]: " + elem + ";", nil
}

// TypeParams writes a type parameter list in angle brackets, or
// nothing for a declaration stating none: the declared variance
// before the name, bounds folded into an intersection behind
// extends, and the default behind an equals sign. A value
// parameter refuses, because the model's Const takes a value
// argument and TypeScript's const modifier applies to a type
// argument.
//
// TypeParams allocates one spelling for each clause a parameter
// states, which is its variance with its name, its bounds or its
// default, and the joined bounds of more than one. It allocates the
// joined list and its brackets, and what each type's spelling
// allocates: four allocations for a parameter of two bounds beside an
// unbounded one.
func (s Speller) TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Const {
			return "", refuse("a type parameter takes a type, and %s takes a value", p.Name)
		}
		part := variance(p.Variance) + p.Name
		if len(p.Bounds) > 0 {
			bounds, err := s.all(p.Bounds, s.Spell)
			if err != nil {
				return "", err
			}
			part += " extends " + strings.Join(bounds, " & ")
		}
		if p.Default != nil {
			def, err := s.Spell(p.Default)
			if err != nil {
				return "", err
			}
			part += " = " + def
		}
		parts = append(parts, part)
	}
	return argsOpener + strings.Join(parts, ", ") + argsCloser, nil
}

// Heritage writes a type's heritage clauses: one base behind
// extends and the contracts behind implements on a class, the
// widened contracts behind extends on an interface. A class's base
// is a value at run time, so it imports as a value, and every
// contract imports for the type checker alone. A second class base
// refuses, because TypeScript extends one, and an embed refuses on
// either, because nothing promotes members.
//
// Heritage allocates each clause behind its keyword and the joined
// contracts of more than one, with what each type's spelling
// allocates: one allocation for a base alone, three for a base beside
// two contracts, and nothing for a type without heritage.
func (s Speller) Heritage(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		if len(t.Embeds) > 0 {
			return "", unembedded(t.Name)
		}
		var part string
		switch len(t.Extends) {
		case 0:
		case 1:
			base, err := s.spellValue(t.Extends[0])
			if err != nil {
				return "", err
			}
			part = " extends " + base
		default:
			return "", refuse("a class extends one base, and %s states %d", t.Name, len(t.Extends))
		}
		if len(t.Implements) > 0 {
			contracts, err := s.all(t.Implements, s.Spell)
			if err != nil {
				return "", err
			}
			part += " implements " + strings.Join(contracts, ", ")
		}
		return part, nil
	case *emit.Interface:
		if len(t.Embeds) > 0 {
			return "", unembedded(t.Name)
		}
		if len(t.Extends) == 0 {
			return "", nil
		}
		widened, err := s.all(t.Extends, s.Spell)
		if err != nil {
			return "", err
		}
		return " extends " + strings.Join(widened, ", "), nil
	default:
		return "", refuse("no heritage clause spells a %s", d.Kind())
	}
}

// Params writes a parameter list, the rest marker included and a
// stated default behind its equals sign, verbatim from the model. A
// rest parameter stating a default refuses, because TypeScript
// initializes no rest. An unnamed parameter binds the discard name,
// numbered from the second on.
//
// Params allocates each parameter's spelling and the joined list of
// more than one, with what each type's spelling allocates: three
// allocations for two parameters, and nothing for none. A numbered
// discard name, an optional marker and a default each allocate one
// more.
func (s Speller) Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	unnamed := 0
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = discardName
			if unnamed > 0 {
				name = discardName + strconv.Itoa(unnamed)
			}
			unnamed++
		} else if !spell.IsIdentifier(name) {
			return "", refuse("a parameter admits no quoted form, and %q is not an identifier", name)
		}
		typ, err := s.Spell(p.Type)
		if err != nil {
			return "", err
		}
		if p.Variadic != symbol.VariadicNone {
			switch {
			case p.Default != "":
				return "", refuse("a rest parameter takes no default, and %s states one", name)
			case p.Optional:
				return "", refuse("a rest parameter is optional by shape, and %s states it", name)
			}
			parts = append(parts, "..."+name+": "+typ+"[]"+textfmt.Inline(p.Comment))
			continue
		}
		if p.Optional && p.Default != "" {
			return "", refuse("a default already makes %s optional, and it states the marker "+
				"beside it", name)
		}
		part := name
		if p.Optional {
			part += "?"
		}
		part += ": " + typ
		if p.Default != "" {
			part += " = " + p.Default
		}
		parts = append(parts, part+textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", "), nil
}

// Returns writes a callable's return annotation. It writes the
// [Results] spelling, inside Promise for an async callable, because an
// async function returns a promise of its result. It writes nothing for
// a setter, because a TypeScript setter cannot have a return
// annotation. It refuses an async callable in a file whose declaration
// hides the global Promise. It allocates what [Speller.Results]
// allocates, the promise taking the annotation's place, and nothing for
// a setter or an async callable that returns nothing.
func (s Speller) Returns(d symbol.Symbol) (string, error) {
	var rs []*emit.Return
	async := false
	switch c := d.(type) {
	case *emit.Function:
		rs, async = c.Returns, c.Async
	case *emit.Method:
		if c.Accessor == symbol.AccessorSet {
			return "", nil
		}
		rs, async = c.Returns, c.Async
	}
	switch {
	case !async:
		return s.Results(rs)
	case s.set.Reserved(promiseType):
		return "", refuse("a declaration of the file hides the global type %s", promiseType)
	case len(rs) == 0:
		return annotationSep + promiseOpen + voidType + promiseClose, nil
	}
	return s.resultType(rs, annotationSep+promiseOpen, promiseClose)
}

// Results writes a return type annotation: void for none, the
// type for one, and a tuple for several, because TypeScript
// returns one value however many the delegate hands back. It allocates
// the annotation and the joined tuple of more than one result, with
// what each type's spelling allocates: one allocation for one result,
// two for a tuple of two, and nothing for none.
func (s Speller) Results(rs []*emit.Return) (string, error) {
	if len(rs) == 0 {
		return annotationSep + voidType, nil
	}
	return s.resultType(rs, annotationSep, "")
}

// resultType writes the type one or more results return as, between
// open and closer: the one result's type, and a tuple of several. The
// text joins in one concatenation, so one result allocates once.
func (s Speller) resultType(rs []*emit.Return, open, closer string) (string, error) {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		typ, err := s.Spell(r.Type)
		if err != nil {
			return "", err
		}
		parts = append(parts, typ+textfmt.Inline(r.Comment))
	}
	if len(parts) == 1 {
		return open + parts[0] + closer, nil
	}
	return open + "[" + strings.Join(parts, ", ") + "]" + closer, nil
}

// spellValue writes a reference in a value position, where the
// declaration must exist at run time, such as a class's base.
func (s Speller) spellValue(t *emit.TypeRef) (string, error) {
	return s.spell(t, false)
}

// spell writes a reference, importing what it names for the type
// checker alone where typeOnly is set.
func (s Speller) spell(t *emit.TypeRef, typeOnly bool) (string, error) {
	out, err := spellref.SpellWith(t, argsOpener, argsCloser, Anonymous, s.qualify(typeOnly))
	if err != nil {
		return "", refuse("%w", err)
	}
	return out, nil
}

// qualify returns the spelling of a named reference through its
// import. The first segment of a qualified spelling is the declaration
// that the module exports, and the rest is a member of it. A reference
// without a module and a reference of the file's own module import
// nothing. qualify refuses a global type that a declaration of the file
// hides.
func (s Speller) qualify(typeOnly bool) spellref.Qualify {
	return func(t *emit.TypeRef) (string, error) {
		head, rest, qualified := strings.Cut(t.Spelling, qualifierSep)
		module := spellref.PackageOf(t, typescript.Lang)
		switch {
		case module == "" && slices.Contains(globalTypes, head) && s.set.Reserved(head):
			return "", refuse("a declaration of the file hides the global type %s", head)
		case module == "" || module == s.set.Home():
			return t.Spelling, nil
		}
		local := s.set.BindItem(s.specifier(t, module), head, typeOnly)
		if !qualified {
			return local, nil
		}
		return local + qualifierSep + rest, nil
	}
}

// specifier returns the import specifier of the module of a named
// reference, by the rules of [Speller]. module is the reference's module
// path, or the specifier that its source wrote where it has no target.
func (s Speller) specifier(t *emit.TypeRef, module string) string {
	if t.Target.IsZero() ||
		t.Target.Lang == typescript.Lang && t.Package != "" && kindOf(t.Package) == packageSpecifier {
		return t.Package
	}
	return relativeTo(s.set.Home(), module)
}

// all writes each reference through spell, in order.
func (Speller) all(ts []*emit.TypeRef, spell func(*emit.TypeRef) (string, error)) ([]string, error) {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		part, err := spell(t)
		if err != nil {
			return nil, err
		}
		out = append(out, part)
	}
	return out, nil
}

// Funcs returns the shared template vocabulary the kind templates
// call, bound to one file's import set: every declaration a helper
// names from another module is imported there. Funcs allocates the map
// of twenty-one helpers, four allocations, and the speller's seven
// bound helpers.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      s.Spell,
		FuncTypeParams: s.TypeParams,
		FuncParams:     s.Params,
		FuncResults:    s.Results,
		FuncReturns:    s.Returns,
		FuncMods:       Mods,
		FuncMemberMods: MemberMods,
		FuncIndexMods:  IndexMods,
		FuncCtorMods:   CtorMods,
		FuncPropMods:   PropMods,
		FuncSigMods:    SigMods,
		FuncBinding:    Binding,
		FuncDecorators: Decorators,
		FuncHeritage:   s.Heritage,
		FuncAccessor:   AccessorKw,
		FuncHard:       Hard,
		FuncIndexSig:   s.IndexSig,
		FuncPropKey:    PropKey,
		FuncMethodKey:  MethodKey,
		FuncEnumKey:    EnumKey,
	}
}

// Docs writes a declaration's documentation as a TSDoc block,
// each line prefixed with the given indentation, so a member's
// doc is at its member's depth. It allocates what [textfmt.BlockDocs]
// allocates: the block, sized once, and nothing for no lines.
func Docs(lines []string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	return textfmt.BlockDocs(lines, "/**", " * ", " */", prefix...)
}

// PropKey writes a field's property key: an identifier bare, and any
// other name, such as the wire name content-type or a digit-led key,
// in single quotes, so the type declares the name whole. A
// hard-private field must be an identifier, because # admits no
// quoted form. PropKey allocates a quoted key, one allocation, and
// nothing for an identifier. A refusal allocates its error.
func PropKey(f *emit.Field) (string, error) {
	if spell.IsIdentifier(f.Name) {
		return f.Name, nil
	}
	if f.Hard {
		return "", refuse("a hard-private name admits no quoted form, and %s is not an identifier",
			f.Name)
	}
	return quote(f.Name), nil
}

// MethodKey spells a method's member key: the identifier bare, and
// anything else quoted, which TypeScript admits on classes and
// interfaces alike. A hard-private name refuses the quoted form
// the way a property's does. MethodKey allocates a quoted key, one
// allocation, and nothing for an identifier. A refusal allocates its
// error.
func MethodKey(m *emit.Method) (string, error) {
	if spell.IsIdentifier(m.Name) {
		return m.Name, nil
	}
	if m.Hard {
		return "", refuse("a hard-private name admits no quoted form, and %s is not an identifier",
			m.Name)
	}
	return quote(m.Name), nil
}

// EnumKey writes an enum member's key: an identifier bare, and any
// other name in single quotes, which TypeScript admits on an enum
// member. It allocates a quoted key, one allocation, and nothing for an
// identifier.
func EnumKey(v *emit.EnumVariant) string {
	if spell.IsIdentifier(v.Name) {
		return v.Name
	}
	return quote(v.Name)
}

// AccessorKw writes a method's accessor keyword and refuses a
// declaration outside the accessor's shape: a getter takes nothing
// and returns one value, a setter takes one value and returns
// nothing, and neither declares type parameters, because
// TypeScript's accessors admit none. AccessorKw returns a constant
// keyword and allocates nothing, and a refusal allocates its error.
func AccessorKw(m *emit.Method) (string, error) {
	switch m.Accessor {
	case symbol.AccessorNone:
		return "", nil
	case symbol.AccessorGet:
		switch {
		case len(m.Params) != 0:
			return "", refuse("a getter takes nothing, and %s takes parameters", m.Name)
		case len(m.Returns) == 0:
			return "", refuse("a getter returns its property, and %s returns nothing", m.Name)
		case len(m.TypeParams) != 0:
			return "", refuse("an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "get ", nil
	default:
		switch {
		case len(m.Params) != 1:
			return "", refuse("a setter takes its one value, and %s does not", m.Name)
		case len(m.Returns) != 0:
			return "", refuse("a setter returns nothing, and %s returns", m.Name)
		case len(m.TypeParams) != 0:
			return "", refuse("an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "set ", nil
	}
}

// Hard writes a member's hard-private prefix. A hard-private name
// is runtime privacy in the name itself, so a stated visibility
// beside it refuses: the two mechanisms cannot combine. Hard allocates
// nothing, and a refusal allocates its error.
func Hard(s symbol.Symbol) (string, error) {
	name, hard, vis := "", false, symbol.VisibilityUnknown
	switch d := s.(type) {
	case *emit.Field:
		name, hard, vis = d.Name, d.Hard, d.Visibility
	case *emit.Method:
		name, hard, vis = d.Name, d.Hard, d.Visibility
	}
	switch {
	case !hard:
		return "", nil
	case vis != symbol.VisibilityUnknown:
		return "", refuse("a hard-private name states its privacy in the name, and %s "+
			"states a visibility beside it", name)
	}
	return "#", nil
}

// Mods writes a module-level declaration's leading keywords:
// export for a public or unstated visibility, nothing for a
// package-scoped one, abstract before an abstract class, and
// async before an asynchronous function. A protected, private or
// internal visibility refuses at module level, a final class
// refuses because TypeScript seals nothing, and a constant without a
// value refuses, because const X = declares nothing. An annotation
// list refuses on every declaration decorators cannot mark:
// TypeScript decorates classes and their members alone. Mods allocates
// the keywords of an abstract class or an async function, one
// allocation, and nothing for every other declaration. A refusal
// allocates its error.
func Mods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		if t.Final {
			return "", refuse("a class admits no final, and %s states it", t.Name)
		}
		part, err := exported(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Abstract {
			part += "abstract "
		}
		return part, nil
	case *emit.Interface:
		if len(t.Annotations) > 0 {
			return "", undecorated(t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Function:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case len(t.Throws) > 0:
			return "", unthrown(t.Name)
		}
		part, err := exported(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Async {
			part += "async "
		}
		return part, nil
	case *emit.Alias:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case t.Defined:
			return "", refuse("an alias is transparent, and %s states a defined type", t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Enum:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case t.Fields.Len() > 0 || t.Methods.Len() > 0:
			return "", refuse("an enum has values alone, and %s states members", t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Constant:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case t.Value == "":
			return "", refuse("a constant takes a value, and %s states none", t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Variable:
		if len(t.Annotations) > 0 {
			return "", undecorated(t.Name)
		}
		return exported(t.Visibility, t.Name)
	default:
		return "", refuse("no module-level keywords spell a %s", d.Kind())
	}
}

// MemberMods writes a class member's leading keywords in the order of
// TypeScript's grammar: accessibility, static, abstract, override, async
// on methods, and readonly on fields. TypeScript allows async only on a
// method with a body that is not an accessor. An async abstract method
// and an async accessor keep their asynchrony in the promise of their
// annotation. MemberMods refuses these members:
//
//   - a final method, because TypeScript has no final methods;
//   - a method with a default, because TypeScript has no default
//     methods;
//   - an abstract method with a body, because an abstract method is a
//     signature;
//   - a member with package or internal accessibility, because class
//     members do not take one.
//
// MemberMods allocates one join per keyword that it writes after the
// first, and nothing for a member of one keyword or none. A refusal
// allocates its error.
func MemberMods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Field:
		part, err := accessibility(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Level == symbol.LevelType {
			part += "static "
		}
		if t.Mutability == symbol.MutabilityImmutable {
			part += "readonly "
		}
		return part, nil
	case *emit.Method:
		switch {
		case t.Final:
			return "", refuse("a method admits no final, and %s states it", t.Name)
		case t.HasDefault:
			return "", refuse("a class method states its body outright, and %s states a default",
				t.Name)
		case t.Abstract && !t.Body.IsZero():
			return "", refuse("an abstract method is a signature, and %s states a body", t.Name)
		case len(t.Throws) > 0:
			return "", unthrown(t.Name)
		}
		part, err := accessibility(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Level == symbol.LevelType {
			part += "static "
		}
		if t.Abstract {
			part += "abstract "
		}
		if t.Override {
			part += "override "
		}
		if t.Async && !t.Abstract && t.Accessor == symbol.AccessorNone {
			part += "async "
		}
		return part, nil
	default:
		return "", refuse("no member keywords spell a %s", d.Kind())
	}
}

// IndexMods writes a class index signature's one keyword: static for a
// type-level signature. An index signature is a
// declaration without a body, so a body refuses, and so do an
// accessibility, abstract, override, final, a default, asynchrony,
// a hard-private name, a throws clause and decorators, because
// TypeScript spells none of them on an index signature. IndexMods
// allocates nothing, and a refusal allocates its error.
func IndexMods(m *emit.Method) (string, error) {
	switch {
	case !m.Body.IsZero():
		return "", refuse("an index signature is a declaration without a body, and %s states one",
			m.Name)
	case m.Visibility != symbol.VisibilityUnknown && m.Visibility != symbol.VisibilityPublic:
		return "", refuse("an index signature takes no accessibility, and %s states one", m.Name)
	case m.Abstract || m.Override || m.Final || m.HasDefault || m.Async || m.Hard:
		return "", refuse("an index signature takes static alone, and %s states another modifier",
			m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	case len(m.Annotations) > 0:
		return "", refuse("decorators mark no index signature, and %s states annotations", m.Name)
	}
	if m.Level == symbol.LevelType {
		return "static ", nil
	}
	return "", nil
}

// CtorMods writes a class constructor's keywords: its accessibility
// alone. A constructor belongs to instances and takes no type
// parameters, so static and type parameters refuse, and so do
// abstract, override, final, a default, asynchrony, an accessor, a
// hard-private name, a throws clause and decorators, because
// TypeScript spells none of them on a constructor. CtorMods allocates
// nothing, and a refusal allocates its error.
func CtorMods(m *emit.Method) (string, error) {
	switch {
	case m.Level == symbol.LevelType:
		return "", refuse("a constructor belongs to instances, and %s states type level", m.Name)
	case len(m.TypeParams) > 0:
		return "", refuse("a constructor takes no type parameters, and %s states %d",
			m.Name, len(m.TypeParams))
	case m.Abstract || m.Override || m.Final || m.HasDefault || m.Async ||
		m.Hard || m.Accessor != symbol.AccessorNone:
		return "", refuse("a constructor takes an accessibility alone, and %s states another "+
			"modifier", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	case len(m.Annotations) > 0:
		return "", refuse("decorators mark no constructor, and %s states annotations", m.Name)
	}
	return accessibility(m.Visibility, m.Name)
}

// PropMods writes an interface property's keywords: readonly
// where the property is immutable. An interface member takes no
// accessibility, no static level and no decorator, so each of them
// refuses. PropMods allocates nothing, and a refusal allocates its
// error.
func PropMods(f *emit.Field) (string, error) {
	switch {
	case f.Hard:
		return "", refuse("an interface property has no runtime, and %s states a hard-private name",
			f.Name)
	case len(f.Annotations) > 0:
		return "", undecorated(f.Name)
	case f.Visibility != symbol.VisibilityUnknown &&
		f.Visibility != symbol.VisibilityPublic:
		return "", refuse("an interface property is public by shape, and %s states a scope", f.Name)
	case f.Level == symbol.LevelType:
		return "", refuse("an interface property has no static level, and %s states one", f.Name)
	case f.Value != "":
		return "", refuse("an interface property takes no initializer, and %s states one", f.Name)
	}
	if f.Mutability == symbol.MutabilityImmutable {
		return "readonly ", nil
	}
	return "", nil
}

// SigMods guards an interface member signature, which takes no
// keywords and no body: a method signature, an index signature and
// a construct signature alike. A stated modifier or a body is
// refused, and the signature spells bare. An async method signature
// keeps its asynchrony in the promise of its annotation, and an async
// index or construct signature is refused, because neither returns a
// promise. SigMods allocates nothing, and a refusal allocates its error.
func SigMods(m *emit.Method) (string, error) {
	switch {
	case !m.Body.IsZero():
		return "", refuse("an interface method is a signature, and %s states a body", m.Name)
	case len(m.Annotations) > 0:
		return "", undecorated(m.Name)
	case m.Visibility != symbol.VisibilityUnknown &&
		m.Visibility != symbol.VisibilityPublic:
		return "", refuse("an interface method is public by shape, and %s states a scope", m.Name)
	case m.Accessor != symbol.AccessorNone || m.Hard:
		return "", refuse("an interface states properties, not accessors or hard-private names, "+
			"and %s states one", m.Name)
	case m.Level == symbol.LevelType || m.Abstract || m.Final || m.Override || m.HasDefault:
		return "", refuse("an interface method is a bare signature, and %s states a modifier", m.Name)
	case m.Async && (m.Indexer || m.Constructs):
		return "", refuse("an index or construct signature returns no promise, and %s is async", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	}
	return "", nil
}

// Binding writes a module-level binding's keyword: const for an
// immutable binding, let otherwise. An immutable binding without an
// initializer refuses, because TypeScript requires a const to be
// initialized where it is declared. Binding returns a constant keyword
// and allocates nothing, and a refusal allocates its error.
func Binding(v *emit.Variable) (string, error) {
	if v.Mutability != symbol.MutabilityImmutable {
		return letBinding, nil
	}
	if v.Value == "" {
		return "", refuse("a const binding takes its value where it is declared, and %s "+
			"states none", v.Name)
	}
	return constBinding, nil
}

// Decorators writes a declaration's decorator lines, one per
// annotation, each prefixed with the given indentation: the name
// behind its marker, and the argument spellings verbatim in
// parentheses where any are stated. It allocates what [textfmt.Marked]
// allocates: the lines, sized once, and nothing for no annotations.
func Decorators(a symbol.Annotations, prefix ...string) string {
	return textfmt.Marked(a, "@", "", prefix...)
}

// variance writes the declaration-site keyword TypeScript places
// before a parameter's name: in for contravariant, out for
// covariant, nothing for invariant.
func variance(v symbol.Variance) string {
	switch v {
	case symbol.VarianceIn:
		return "in "
	case symbol.VarianceOut:
		return "out "
	default:
		return ""
	}
}

// exported writes the module-level visibility: export for public
// or unstated, nothing for package scope, which is the module
// itself. The other scopes have no module-level spelling.
func exported(v symbol.Visibility, name string) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return "export ", nil
	case symbol.VisibilityPackage:
		return "", nil
	default:
		return "", refuse("a module-level declaration is exported or module-scoped, and %s "+
			"states another scope", name)
	}
}

// accessibility writes a class member's accessibility: nothing
// for public or unstated, the keyword for private and protected.
// Package and internal scopes have no member spelling.
func accessibility(v symbol.Visibility, name string) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return "", nil
	case symbol.VisibilityPrivate:
		return "private ", nil
	case symbol.VisibilityProtected:
		return "protected ", nil
	default:
		return "", refuse("a class member states public, private or protected, and %s "+
			"states another scope", name)
	}
}

// relativeTo returns the relative specifier of a module path from the
// directory of the module path home. The specifier opens with ./ where
// the module is in that directory or below it, and otherwise with one
// ../ for each directory that the path climbs. It allocates the
// specifier.
func relativeTo(home, module string) string {
	dir, _, nested := strings.CutLast(home, pathSep)
	if !nested {
		dir = ""
	}
	// shared is the length of the directories that dir and the module
	// path open with, with the separator behind them.
	shared := 0
	for shared < len(dir) {
		segment := dir[shared:]
		if i := strings.Index(segment, pathSep); i >= 0 {
			segment = segment[:i]
		}
		end := shared + len(segment)
		if end >= len(module) || module[shared:end] != segment || module[end:end+len(pathSep)] != pathSep {
			break
		}
		shared = end + len(pathSep)
	}
	climbs := 0
	if shared < len(dir) {
		climbs = strings.Count(dir[shared:], pathSep) + 1
	}
	var b strings.Builder
	b.Grow(len(currentDir) + climbs*len(parentDir) + len(module) - shared)
	if climbs == 0 {
		b.WriteString(currentDir)
	}
	for range climbs {
		b.WriteString(parentDir)
	}
	b.WriteString(module[shared:])
	return b.String()
}

// unthrown is the refusal for declared failure types: a
// TypeScript signature declares none.
func unthrown(name string) error {
	return refuse("a signature declares no failure types, and %s states throws", name)
}

// unembedded is the refusal for embeds: nothing promotes members.
func unembedded(name string) error {
	return refuse("nothing promotes members, and %s states embeds", name)
}

// undecorated is the refusal for an annotation list on a
// declaration decorators cannot mark.
func undecorated(name string) error {
	return refuse("decorators mark classes and their members, and %s states annotations "+
		"elsewhere", name)
}

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
