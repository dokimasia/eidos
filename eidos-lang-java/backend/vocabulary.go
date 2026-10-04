// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
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
	// FuncResults writes the one return type.
	FuncResults = "results"
	// FuncPackage writes the package clause.
	FuncPackage = "package"
	// FuncTypeMods writes a type's keywords.
	FuncTypeMods = "typemods"
	// FuncMemberType refuses what an interface's member type states
	// that Java cannot spell there.
	FuncMemberType = "membertype"
	// FuncFieldMods writes a field's keywords.
	FuncFieldMods = "fieldmods"
	// FuncConstantMods writes an interface field's keywords.
	FuncConstantMods = "constantmods"
	// FuncMethodMods writes a class method's keywords.
	FuncMethodMods = "methodmods"
	// FuncSigMods writes an interface method's keywords.
	FuncSigMods = "sigmods"
	// FuncAnnotate writes a declaration's annotation lines.
	FuncAnnotate = "annotate"
	// FuncHeritage writes a type's heritage clauses.
	FuncHeritage = "heritage"
	// FuncThrows writes a callable's throws clause.
	FuncThrows = "throws"
	// FuncEnumVariant writes one enum constant.
	FuncEnumVariant = "enumvariant"
)

// Anonymous is what Java writes where a declaration states no
// type: the root every reference type descends from.
const Anonymous = "Object"

// The brackets Java writes an argument list in.
const (
	argsOpener = "<"
	argsCloser = ">"
)

// The separators Java writes a list of types with: a bound list's
// ampersand, and the comma of every other list.
const (
	boundSep = " & "
	listSep  = ", "
)

// The package statement's keyword and its end, and the separator of a
// package path's segments, which Java writes as a dot.
const (
	packageKeyword = "package "
	packageEnd     = ";\n\n"
	pathSep        = "/"
)

// unnamedParam is the name an unnamed parameter takes, behind its
// position in the list.
const unnamedParam = "arg"

// refusalPrefix opens every refusal the backend returns: the
// language's identity, as every backend's refusals open.
const refusalPrefix = string(java.Lang) + ": "

// Speller spells type references for one file. A class a spelling
// names from another package is imported under its simple name
// where the file's name is free, and written fully qualified where
// another import or a declaration of the file takes it, the way
// javac reads the two. The package is the reference's target's where
// the target is a Java declaration, and the one the reference records
// where it has no target.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text it writes and nothing else, so a
// builtin and a class imported under its simple name allocate nothing.
// Claiming a class's name for the first time allocates in the set. A
// method that joins more than two parts also allocates its list of
// parts, because Go places a list of at most two strings on the stack.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller for the file whose import set is
// set. The set is not nil. It allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes a type reference and imports the class it names. The
// source spelling passes through where the reference names no class
// of another package, and a missing one spells [Anonymous]. A
// reference with arguments has its bare name in Spelling, and the
// argument list spells here in angle brackets. A structural reference
// keeps its written spelling, and Spell returns an error where a class
// inside it is written fully qualified, because Java's composite
// spelling does not follow from its structure.
//
// Spell allocates nothing for a builtin or a class it imports under its
// simple name. A fully qualified class and an argument list allocate
// their text, and a refusal allocates its error.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	out, err := spellref.SpellWith(t, argsOpener, argsCloser, Anonymous, s.qualify)
	if err != nil {
		return "", refuse("%w", err)
	}
	return out, nil
}

// TypeParams writes a type parameter list in angle brackets, or
// nothing for a declaration stating none, bounds joined by
// ampersands behind extends. Variance, defaults and value
// parameters refuse, because Java's variance is a use-site wildcard
// and its parameters take no default and no value.
//
// TypeParams allocates each bounded parameter's spelling, the joined
// list and its brackets, and what each bound's spelling allocates.
func (s Speller) TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		switch {
		case p.Variance != symbol.VarianceInvariant:
			return "", refuse("a type parameter states no variance, and %s states one: "+
				"the wildcard is use-site", p.Name)
		case p.Const:
			return "", refuse("a type parameter takes a type, and %s takes a value", p.Name)
		case p.Default != nil:
			return "", refuse("a type parameter takes no default, and %s states one", p.Name)
		}
		part := p.Name
		if len(p.Bounds) > 0 {
			bounds, err := s.joined(p.Bounds, boundSep)
			if err != nil {
				return "", err
			}
			part += " extends " + bounds
		}
		parts = append(parts, part)
	}
	return argsOpener + strings.Join(parts, ", ") + argsCloser, nil
}

// Params writes a parameter list, the variadic marker included. It
// allocates each parameter's spelling and the joined list, and what each
// type's spelling allocates.
func (s Speller) Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	for i, p := range ps {
		name := p.Name
		if name == "" {
			name = unnamedParam + strconv.Itoa(i)
		}
		spelling, err := s.Spell(p.Type)
		if err != nil {
			return "", err
		}
		if p.Variadic != symbol.VariadicNone {
			spelling += "..."
		}
		parts = append(parts, spelling+" "+name+textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", "), nil
}

// Results writes the return type: void for none, the type for
// one, and an error for several, because a Java callable returns
// one value and a second arrives thrown, not returned. It allocates
// what the type's spelling allocates, and the type joined to its
// trailing comment where one is stated.
func (s Speller) Results(rs []*emit.Return) (string, error) {
	switch len(rs) {
	case 0:
		return "void", nil
	case 1:
		typ, err := s.Spell(rs[0].Type)
		if err != nil {
			return "", err
		}
		return typ + textfmt.Inline(rs[0].Comment), nil
	default:
		return "", refuse("a callable returns one value, and this one states %d: "+
			"a second result arrives thrown, not returned", len(rs))
	}
}

// Heritage writes a type's heritage clauses: one superclass
// behind extends and the contracts behind implements on a class,
// the widened contracts behind extends on an interface, and the
// enumerated subtypes behind permits on either, last the way Java
// states them. A second superclass refuses, because Java extends
// one, and an embed refuses on either, because nothing promotes
// members. The permits clause and the sealed keyword go together:
// javac rejects a sealed type in a file of its own without the
// clause, and the clause on a type that is not sealed. It allocates the
// clauses it writes, and what each type's spelling allocates.
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
			base, err := s.Spell(t.Extends[0])
			if err != nil {
				return "", err
			}
			part = " extends " + base
		default:
			return "", refuse("a class extends one superclass, and %s states %d",
				t.Name, len(t.Extends))
		}
		if len(t.Implements) > 0 {
			contracts, err := s.joined(t.Implements, listSep)
			if err != nil {
				return "", err
			}
			part += " implements " + contracts
		}
		permits, err := s.permitted(t.Name, t.Sealed, t.Permits)
		if err != nil {
			return "", err
		}
		return part + permits, nil
	case *emit.Interface:
		if len(t.Embeds) > 0 {
			return "", unembedded(t.Name)
		}
		var part string
		if len(t.Extends) > 0 {
			widened, err := s.joined(t.Extends, listSep)
			if err != nil {
				return "", err
			}
			part = " extends " + widened
		}
		permits, err := s.permitted(t.Name, t.Sealed, t.Permits)
		if err != nil {
			return "", err
		}
		return part + permits, nil
	default:
		return "", refuse("no heritage clause spells a %s", d.Kind())
	}
}

// Throws writes a callable's throws clause: the declared failure
// types comma-joined behind the keyword, or nothing where none
// are stated. It allocates the clause, the joined list of several
// types, and what each type's spelling allocates.
func (s Speller) Throws(ts []*emit.TypeRef) (string, error) {
	if len(ts) == 0 {
		return "", nil
	}
	failures, err := s.joined(ts, listSep)
	if err != nil {
		return "", err
	}
	return " throws " + failures, nil
}

// qualify returns the spelling of a named reference: as written where
// the file claims the simple name of the class it names, and fully
// qualified where the file cannot. The first segment of a qualified
// spelling is the class the package declares, and the rest names a
// member class of it.
func (s Speller) qualify(t *emit.TypeRef) (string, error) {
	pkg := spellref.PackageOf(t, java.Lang)
	if pkg == "" {
		return t.Spelling, nil
	}
	class, _, _ := strings.Cut(t.Spelling, memberSep)
	if s.set.Claim(pkg, class) {
		return t.Spelling, nil
	}
	return javaPackage(pkg) + memberSep + t.Spelling, nil
}

// permitted writes the permits clause, or nothing where no subtypes
// are enumerated. A sealed type without one refuses, and so does the
// clause on a type that is not sealed.
func (s Speller) permitted(name string, sealed bool, ts []*emit.TypeRef) (string, error) {
	switch {
	case sealed && len(ts) == 0:
		return "", refuse("a sealed type names its permitted subtypes, and %s names none", name)
	case !sealed && len(ts) > 0:
		return "", refuse("a permits clause belongs to a sealed type, and %s is not sealed", name)
	case len(ts) == 0:
		return "", nil
	default:
		subtypes, err := s.joined(ts, listSep)
		if err != nil {
			return "", err
		}
		return " permits " + subtypes, nil
	}
}

// joined writes references through the speller, in order, joined by
// sep. The parts do not escape the call, so a list of up to two
// references allocates no slice.
func (s Speller) joined(ts []*emit.TypeRef, sep string) (string, error) {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		part, err := s.Spell(t)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, sep), nil
}

// Funcs returns the shared template vocabulary the kind templates
// call, bound to one file's import set: every class a helper names
// from another package is imported there. Funcs allocates the map of
// sixteen helpers, four allocations, and the speller's six bound
// helpers.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:         Docs,
		FuncSpell:        s.Spell,
		FuncTypeParams:   s.TypeParams,
		FuncParams:       s.Params,
		FuncResults:      s.Results,
		FuncPackage:      PackageClause,
		FuncTypeMods:     TypeMods,
		FuncMemberType:   MemberType,
		FuncFieldMods:    FieldMods,
		FuncConstantMods: ConstantMods,
		FuncMethodMods:   MethodMods,
		FuncSigMods:      SigMods,
		FuncAnnotate:     Annotate,
		FuncHeritage:     s.Heritage,
		FuncThrows:       s.Throws,
		FuncEnumVariant:  EnumVariantName,
	}
}

// Docs writes a declaration's documentation as a Javadoc block,
// each line prefixed with the given indentation, so a member's
// doc is at its member's depth. It allocates what [textfmt.BlockDocs]
// allocates: the block, sized once, and nothing for no lines.
func Docs(lines []string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	return textfmt.BlockDocs(lines, "/**", " * ", " */", prefix...)
}

// PackageClause writes the package statement from the identity's
// package path, dots for slashes, followed by a blank line. An
// identity naming no package spells nothing, which is the default
// package. It sizes the statement once and allocates it, one
// allocation, and nothing for the default package.
func PackageClause(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(packageKeyword) + len(id.Package) + len(packageEnd))
	b.WriteString(packageKeyword)
	for i := range len(id.Package) {
		if c := id.Package[i]; c == pathSep[0] {
			b.WriteString(memberSep)
		} else {
			b.WriteByte(c)
		}
	}
	b.WriteString(packageEnd)
	return b.String()
}

// TypeMods writes a type's keywords in Java's stated order: access,
// then abstract, static, final and sealed on a class where stated,
// sealed alone on an interface, and access alone on an enum. The
// keywords are a member type's, because the kind templates render
// both file-level and member types: the lowering refuses a file-level
// type stating what only a member type can spell. A class both
// abstract and final refuses, and so does one both final and
// sealed, because javac rejects either pair. TypeMods allocates one
// join per keyword it writes behind the access keyword, and a refusal
// allocates its error.
func TypeMods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		switch {
		case t.Abstract && t.Final:
			return "", refuse("a class is abstract or final, and %s states both", t.Name)
		case t.Final && t.Sealed:
			return "", refuse("a sealed class has subclasses, and %s states final too", t.Name)
		}
		part, err := access(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Abstract {
			part += "abstract "
		}
		if t.Level == symbol.LevelType {
			part += "static "
		}
		if t.Final {
			part += "final "
		}
		if t.Sealed {
			part += "sealed "
		}
		return part, nil
	case *emit.Interface:
		part, err := access(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Sealed {
			part += "sealed "
		}
		return part, nil
	case *emit.Enum:
		return access(t.Visibility, t.Name)
	default:
		return "", refuse("no type keywords spell a %s", d.Kind())
	}
}

// MemberType writes nothing and refuses what an interface's member
// type states that Java cannot spell there: a private, protected or
// package scope, because every member type of an interface is
// public. It allocates nothing for a type it passes, and a refusal
// allocates its error.
func MemberType(s symbol.Symbol) (string, error) {
	var name string
	var v symbol.Visibility
	switch t := s.(type) {
	case *emit.Struct:
		name, v = t.Name, t.Visibility
	case *emit.Interface:
		name, v = t.Name, t.Visibility
	case *emit.Enum:
		name, v = t.Name, t.Visibility
	default:
		return "", nil
	}
	if v != symbol.VisibilityUnknown && v != symbol.VisibilityPublic {
		return "", refuse("a member type of an interface is public, and %s states "+
			"another scope", name)
	}
	return "", nil
}

// EnumVariantName writes one enum constant's spelling: the name
// alone. A stated value refuses, because a valued constant takes
// the constructor form these templates do not spell. It allocates
// nothing, and a refusal allocates its error.
func EnumVariantName(v *emit.EnumVariant) (string, error) {
	if v.Value != "" {
		return "", refuse("an enum constant spells its name alone, and %s states a value", v.Name)
	}
	return v.Name, nil
}

// FieldMods writes a field's keywords, in Java's stated order:
// access, static for a type-level field, final for an immutable
// one. It allocates one join per keyword it writes behind the access
// keyword, and a refusal allocates its error.
func FieldMods(f *emit.Field) (string, error) {
	part, err := access(f.Visibility, f.Name)
	if err != nil {
		return "", err
	}
	if f.Level == symbol.LevelType {
		part += "static "
	}
	if f.Mutability == symbol.MutabilityImmutable {
		part += "final "
	}
	return part, nil
}

// ConstantMods writes an interface field's keywords, which are
// none: Java reads every interface field as a public, static and
// final constant. A field without an initializer refuses, because a
// constant takes its value where it is declared, and so do a scope
// other than public and a mutable field, because Java spells
// neither on an interface field. It allocates nothing, and a refusal
// allocates its error.
func ConstantMods(f *emit.Field) (string, error) {
	switch {
	case f.Value == "":
		return "", refuse("an interface field is a constant, and %s states no initializer", f.Name)
	case f.Visibility != symbol.VisibilityUnknown && f.Visibility != symbol.VisibilityPublic:
		return "", refuse("an interface field is public, and %s states another scope", f.Name)
	case f.Mutability == symbol.MutabilityMutable:
		return "", refuse("an interface field is final, and %s states a mutable one", f.Name)
	}
	return "", nil
}

// MethodMods writes a class method's keywords, in Java's stated
// order: access, static, abstract, final. An asynchronous method
// refuses, because Java marks no signature asynchronous, and a
// default refuses outside an interface. An abstract method with a
// body refuses, because an abstract method is a signature. MethodMods
// allocates one join per keyword it writes behind the access keyword,
// and a refusal allocates its error.
func MethodMods(m *emit.Method) (string, error) {
	switch {
	case m.Abstract && !m.Body.IsZero():
		return "", refuse("an abstract method is a signature, and %s states a body", m.Name)
	case m.Async:
		return "", refuse("a signature states no asynchrony, and %s states it", m.Name)
	case m.HasDefault:
		return "", refuse("default belongs to interface methods, and %s is a class member", m.Name)
	}
	part, err := access(m.Visibility, m.Name)
	if err != nil {
		return "", err
	}
	if m.Level == symbol.LevelType {
		part += "static "
	}
	if m.Abstract {
		part += "abstract "
	}
	if m.Final {
		part += "final "
	}
	return part, nil
}

// SigMods writes an interface method's keywords: nothing for the
// implicitly public signature, private where stated, static for a
// type-level method, and default for a public method with a default
// body at instance level. A private or a static method places its body
// the way a default does, and javac rejects either without a body, so
// such a method without a default body refuses. An abstract method
// passes, because an interface signature is abstract by shape. A body
// without a default refuses, because the template places only a
// default body, and final, override and asynchrony refuse. SigMods
// allocates the keywords of a private static method, one allocation,
// and nothing for every other signature. A refusal allocates its
// error.
func SigMods(m *emit.Method) (string, error) {
	switch {
	case !m.HasDefault && !m.Body.IsZero():
		return "", refuse("an interface method without a default is a signature, and %s states a body",
			m.Name)
	case m.Final:
		return "", refuse("an interface method admits no final, and %s states it", m.Name)
	case m.Override:
		return "", refuse("an interface method overrides nothing, and %s states it", m.Name)
	case m.Async:
		return "", refuse("a signature states no asynchrony, and %s states it", m.Name)
	}
	var part string
	switch m.Visibility {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		part = ""
	case symbol.VisibilityPrivate:
		part = "private "
	default:
		return "", refuse("an interface method is public or private, and %s states another scope",
			m.Name)
	}
	private := m.Visibility == symbol.VisibilityPrivate
	switch {
	case !m.HasDefault && (private || m.Level == symbol.LevelType):
		return "", refuse("a private or a static interface method states its body, and %s states none",
			m.Name)
	case m.Level == symbol.LevelType:
		part += "static "
	case !private && m.HasDefault:
		part = "default "
	}
	return part, nil
}

// Annotate writes a declaration's annotation lines, one per
// annotation, each prefixed with the given indentation: the name
// behind its marker, and the argument spellings verbatim in
// parentheses where any are stated. It allocates what [textfmt.Marked]
// allocates: the lines, sized once, and nothing for no annotations.
func Annotate(a symbol.Annotations, prefix ...string) string {
	return textfmt.Marked(a, "@", "", prefix...)
}

// javaPackage spells a package path the way Java writes it: dots for
// slashes.
func javaPackage(path string) string { return strings.ReplaceAll(path, pathSep, memberSep) }

// access writes a member's or a member type's access keyword. An
// unstated scope spells public, because a generated API exists to be
// called, and package scope spells Java's default access. An
// internal scope has no Java spelling at all.
func access(v symbol.Visibility, name string) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return "public ", nil
	case symbol.VisibilityPackage:
		return "", nil
	case symbol.VisibilityPrivate:
		return "private ", nil
	case symbol.VisibilityProtected:
		return "protected ", nil
	default:
		return "", refuse("no access keyword spells the scope %s states", name)
	}
}

// unembedded is the refusal for embeds: nothing promotes members.
func unembedded(name string) error {
	return refuse("nothing promotes members, and %s states embeds", name)
}

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
