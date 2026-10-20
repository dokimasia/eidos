// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the vocabulary are constants, so a template and the map
// of its helpers use the same name. The names that every backend declares
// are the names of the render package.
const (
	// FuncDocs is the template name of [Docs].
	FuncDocs = render.HelperDocs
	// FuncMemberDocs is the template name of [MemberDocs].
	FuncMemberDocs = "memberdocs"
	// FuncWrapDocs is the template name of [WrapDocs].
	FuncWrapDocs = render.HelperWrapDocs
	// FuncSpell is the template name of [Speller.Spell].
	FuncSpell = render.HelperSpell
	// FuncSpellValue is the template name of [Speller.Spell] in a value
	// position. Java imports a class for its type and its value alike, so
	// the helper writes what the spell helper writes.
	FuncSpellValue = render.HelperSpellValue
	// FuncTypeParams is the template name of [Speller.TypeParams].
	FuncTypeParams = render.HelperTypeParams
	// FuncTypeArgs is the template name of [Speller.TypeArgs].
	FuncTypeArgs = render.HelperTypeArgs
	// FuncParams is the template name of [Speller.Params].
	FuncParams = render.HelperParams
	// FuncResults is the template name of [Speller.Results].
	FuncResults = render.HelperResults
	// FuncPackage is the template name of [PackageClause].
	FuncPackage = "package"
	// FuncTypeMods is the template name of [TypeMods].
	FuncTypeMods = "typemods"
	// FuncMemberType is the template name of [MemberType].
	FuncMemberType = "membertype"
	// FuncFieldMods is the template name of [FieldMods].
	FuncFieldMods = "fieldmods"
	// FuncConstantMods is the template name of [ConstantMods].
	FuncConstantMods = "constantmods"
	// FuncMethodMods is the template name of [MethodMods].
	FuncMethodMods = "methodmods"
	// FuncSigMods is the template name of [SigMods].
	FuncSigMods = "sigmods"
	// FuncAnnotate is the template name of [Annotate].
	FuncAnnotate = "annotate"
	// FuncMemberAnnotate is the template name of [MemberAnnotate].
	FuncMemberAnnotate = "memberannotate"
	// FuncHeritage is the template name of [Speller.Heritage].
	FuncHeritage = "heritage"
	// FuncThrows is the template name of [Speller.Throws].
	FuncThrows = "throws"
	// FuncEnumVariant is the template name of [EnumVariantName].
	FuncEnumVariant = "enumvariant"
)

// Anonymous is the type that the backend writes where a declaration has
// no type. Every reference type of Java descends from Object.
const Anonymous = "Object"

// argsOpener and argsCloser enclose an argument list in Java.
const (
	argsOpener = "<"
	argsCloser = ">"
)

// boundSep separates the bounds of a type parameter, and listSep
// separates the types of every other list.
const (
	boundSep = " & "
	listSep  = ", "
)

// packageKeyword and packageEnd enclose the path of a package statement.
// pathSep separates the segments of a package path, and Java writes each
// separator as a dot.
const (
	packageKeyword = "package "
	packageEnd     = ";\n\n"
	pathSep        = "/"
)

// These are the modifiers that Java writes before a declaration, each
// with the space after it.
const (
	publicKeyword    = "public "
	privateKeyword   = "private "
	protectedKeyword = "protected "
	abstractKeyword  = "abstract "
	staticKeyword    = "static "
	finalKeyword     = "final "
	sealedKeyword    = "sealed "
	defaultKeyword   = "default "
)

// unnamedParam is the start of the name of an unnamed parameter. The
// position of the parameter in the list follows it.
const unnamedParam = "arg"

// refusalPrefix is the start of every refusal of the backend. It is the
// identity of the language, as in the refusals of every backend.
const refusalPrefix = string(java.Lang) + ": "

// Speller writes the type references of one file. It imports a class of
// another package under its simple name where the file does not use that
// name yet. Where another import or a declaration of the file already
// uses the name, Speller writes the class fully qualified, as javac reads
// it. The package is the package of the target where the target is a
// Java declaration, and the package that the reference records where it
// has no target.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text that it writes and nothing else, so a
// builtin and a class imported under its simple name allocate nothing.
// The first claim of the name of a class allocates in the set. A method
// that joins more than two parts also allocates its list of parts,
// because Go places a list of at most two strings on the stack.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller of the file with the import set set.
// set must not be nil. NewSpeller allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes the type reference t and imports its class. It returns
// the source spelling where t does not refer to a class of another
// package, and [Anonymous] for a nil t. A reference with arguments has
// its bare name in Spelling, and Spell writes the argument list in angle
// brackets. A structural reference keeps its written spelling. Spell
// returns an error where a class inside a structural reference needs its
// fully qualified name, because the composite spelling of Java does not
// follow from its structure.
//
// Spell allocates nothing for a builtin or a class that it imports under
// its simple name. A fully qualified class and an argument list allocate
// their text. A refusal allocates its error.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	out, err := spellref.SpellWith(t, argsOpener, argsCloser, Anonymous, s.qualify)
	if err != nil {
		return "", refuse("%w", err)
	}
	return out, nil
}

// TypeParams writes the type parameter list ps in angle brackets, and
// nothing for an empty list. The bounds of a parameter follow extends,
// joined by ampersands. TypeParams returns an error for a variance, a
// default and a value parameter. Java expresses variance as a wildcard
// at the use site, and its type parameters take no default and no value.
//
// TypeParams allocates the spelling of each bounded parameter, the joined
// list with its brackets, and what the spelling of each bound allocates.
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

// TypeArgs writes a list of type parameters or of type references as the
// type arguments of a reference in angle brackets, and nothing for an
// empty list: the parameters of a generic declaration as the arguments of
// a reference to it, as in PageBuilder<T>, and the witnesses of a test, as
// in <Long>. Each reference spells through [Speller.Spell], which imports
// its class. TypeArgs returns an error for a list of another type. It
// allocates what [spellref.TypeArgs] allocates. A template passes the
// list as a value of type any, which text/template boxes, one allocation.
func (s Speller) TypeArgs(list any) (string, error) {
	return spellref.TypeArgs(list, argsOpener, argsCloser, s.Spell)
}

// Params writes the parameter list in the slot ps with the variadic
// marker. A template passes the slot by address, as in
// {{params .Params}}, so the call reads the slot without a reflective
// method call. The annotations of a parameter precede its type, and an
// assigned one returns the error of [Annotate]. An unnamed parameter gets
// [unnamedParam] and its position as its name. Params allocates the
// spelling of each parameter, the joined list, the annotations of a
// parameter that has any, and what the spelling of each type allocates.
func (s Speller) Params(ps *emit.Slot[*emit.Param]) (string, error) {
	items := ps.Items()
	parts := make([]string, 0, len(items))
	for i, p := range items {
		name := p.Name
		if name == "" {
			name = unnamedParam + strconv.Itoa(i)
		}
		if err := unassigned(p.Annotations); err != nil {
			return "", err
		}
		spelling, err := s.Spell(p.Type)
		if err != nil {
			return "", err
		}
		if p.Variadic != symbol.VariadicNone {
			spelling += "..."
		}
		parts = append(parts, textfmt.MarkedInline(p.Annotations, "@", "")+spelling+" "+name+
			textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", "), nil
}

// Results writes the return type of the results in the slot results. It
// writes void for no result and the type for one result. It returns an
// error for two or more results, because a Java callable returns one
// value and reports a failure by a throw. A template passes the slot by
// address, as Params describes. Results allocates what the spelling of
// the type allocates, and the type joined to its trailing comment where
// the result has one.
func (s Speller) Results(results *emit.Slot[*emit.Return]) (string, error) {
	rs := results.Items()
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

// Heritage writes the heritage clauses of the type d. A class has one
// superclass after extends and its contracts after implements. An
// interface has the interfaces that it extends after extends. Either
// kind ends with its permitted subtypes after permits, as Java orders the
// clauses. Heritage returns an error for a second superclass, because a
// Java class extends one class. Java does not promote members, so the
// coverage refuses an embed, and Heritage writes none. The permits clause
// requires the sealed keyword, and the sealed keyword requires the
// clause. javac rejects a sealed type in a file of its own without the
// clause, and the clause on a type that is not sealed. Heritage allocates
// the clauses that it writes, and what the spelling of each type
// allocates.
func (s Speller) Heritage(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
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

// Throws writes the throws clause of the failure types ts, joined by
// commas after the keyword. It writes nothing for no failure type. It
// allocates the clause, the joined list of two or more types, and what
// the spelling of each type allocates.
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

// qualify returns the spelling of the named reference t. It returns the
// written spelling where the file claims the simple name of the class,
// and the fully qualified name where the file cannot claim it. The first
// segment of a qualified spelling is the class that the package
// declares, and the rest is a path to a member class of it.
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

// permitted writes the permits clause of the subtypes ts of the type
// name, and nothing for no subtype. It returns an error for a sealed type
// without subtypes, and for subtypes of a type that is not sealed.
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

// joined writes the references ts through the speller, in order, joined
// by sep. The parts do not escape the call, so a list of up to two
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

// Funcs returns the template vocabulary that the kind templates call. The
// helpers import every class of another package that they write into set,
// the import set of one file. Every helper takes a
// fixed list of arguments, because text/template allocates the list of a
// variadic call and a value for each string argument in a template.
// Funcs allocates the map of twenty-one helpers in four allocations, and
// the seven bound helpers of the speller. The spell helper and the
// spellvalue helper share one bound helper.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	spell := s.Spell
	return template.FuncMap{
		FuncDocs:           Docs,
		FuncMemberDocs:     MemberDocs,
		FuncWrapDocs:       WrapDocs,
		FuncSpell:          spell,
		FuncSpellValue:     spell,
		FuncTypeParams:     s.TypeParams,
		FuncTypeArgs:       s.TypeArgs,
		FuncParams:         s.Params,
		FuncResults:        s.Results,
		FuncPackage:        PackageClause,
		FuncTypeMods:       TypeMods,
		FuncMemberType:     MemberType,
		FuncFieldMods:      FieldMods,
		FuncConstantMods:   ConstantMods,
		FuncMethodMods:     MethodMods,
		FuncSigMods:        SigMods,
		FuncAnnotate:       Annotate,
		FuncMemberAnnotate: MemberAnnotate,
		FuncHeritage:       s.Heritage,
		FuncThrows:         s.Throws,
		FuncEnumVariant:    EnumVariantName,
	}
}

// Docs writes the documentation lines of a declaration as a Javadoc block
// at the top level. It allocates what [textfmt.BlockDocs] allocates. That
// is one block, sized once, and nothing for no lines.
func Docs(lines []string) string {
	return textfmt.BlockDocs(lines, "/**", " * ", " */")
}

// MemberDocs writes the documentation lines of a member as the Javadoc
// block of [Docs]. Each line starts with the member indent of a type
// body. It allocates what Docs allocates.
func MemberDocs(lines []string) string {
	return textfmt.BlockDocs(lines, "/**", " * ", " */", memberIndent)
}

// WrapDocs wraps text at [textfmt.DocsWidth] into the lines that [Docs]
// and [MemberDocs] write. A Javadoc block tag appears at the beginning of a
// line, so the wrap starts no line with a tag. The rest of the comment is
// HTML, which reads no word at the start of a line as markup. It allocates
// what [textfmt.Wrap] allocates.
func WrapDocs(text string) []string {
	return textfmt.Wrap(text, textfmt.DocsWidth, textfmt.MarkupTags)
}

// PackageClause writes the package statement of the package path of id,
// with dots for slashes, followed by a blank line. It writes nothing for
// an identity without a package, because Java puts such a declaration in
// the default package. It sizes the statement once and allocates it once,
// and allocates nothing for the default package.
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

// TypeMods writes the keywords of the type d in the order of the Java
// grammar. A class has its access, then abstract, static, final and
// sealed where the model sets them. An interface has its access and
// sealed, and an enum has its access alone. TypeMods writes the keywords
// of a member type, because the kind templates render file-level types
// and member types. The lowering refuses a file-level type with a
// keyword that only a member type can have. TypeMods returns an error
// for a class that is abstract and final, and for a class that is final
// and sealed, because javac rejects either pair. TypeMods allocates one
// join for each keyword after the access keyword. A refusal allocates
// its error.
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
			part += abstractKeyword
		}
		if t.Level == symbol.LevelType {
			part += staticKeyword
		}
		if t.Final {
			part += finalKeyword
		}
		if t.Sealed {
			part += sealedKeyword
		}
		return part, nil
	case *emit.Interface:
		part, err := access(t.Visibility, t.Name)
		if err != nil {
			return "", err
		}
		if t.Sealed {
			part += sealedKeyword
		}
		return part, nil
	case *emit.Enum:
		return access(t.Visibility, t.Name)
	default:
		return "", refuse("no type keywords spell a %s", d.Kind())
	}
}

// MemberType checks the member type s of an interface and writes
// nothing. It returns an error for a private, protected or package
// visibility, because every member type of an interface is public. It
// allocates nothing for a type that it accepts. A refusal allocates its
// error.
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

// EnumVariantName writes the name of the enum constant v. It returns an
// error for a value, because a constant with a value needs the
// constructor form, which the templates do not render. It allocates
// nothing. A refusal allocates its error.
func EnumVariantName(v *emit.EnumVariant) (string, error) {
	if v.Value != "" {
		return "", refuse("an enum constant spells its name alone, and %s states a value", v.Name)
	}
	return v.Name, nil
}

// FieldMods writes the keywords of the field f in the order of the Java
// grammar. They are the access keyword, static for a type-level field
// and final for an immutable field. FieldMods allocates one join for each
// keyword after the access keyword. A refusal allocates its error.
func FieldMods(f *emit.Field) (string, error) {
	part, err := access(f.Visibility, f.Name)
	if err != nil {
		return "", err
	}
	if f.Level == symbol.LevelType {
		part += staticKeyword
	}
	if f.Mutability == symbol.MutabilityImmutable {
		part += finalKeyword
	}
	return part, nil
}

// ConstantMods checks the interface field f and writes no keyword,
// because every interface field of Java is a public, static and final
// constant. It returns an error for a field without an initializer,
// because a constant has its value in its declaration. It returns an
// error for a visibility other than public and for a mutable field,
// because an interface field cannot have either. It allocates nothing. A
// refusal allocates its error.
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

// MethodMods writes the keywords of the class method m in the order of
// the Java grammar: access, static, abstract and final. It returns an
// error for an async method, because Java does not mark a signature as
// async. It returns an error for a default outside an interface, and for
// an abstract method with a body, because an abstract method is a
// signature. MethodMods allocates one join for each keyword after the
// access keyword. A refusal allocates its error.
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
		part += staticKeyword
	}
	if m.Abstract {
		part += abstractKeyword
	}
	if m.Final {
		part += finalKeyword
	}
	return part, nil
}

// SigMods writes the keywords of the interface method m. It writes
// nothing for a public signature, because an interface method is public
// by default. It writes private for a private method, static for a
// type-level method, and default for a public instance method with a
// default body. A private or a static method has a body as a default
// method does, and javac rejects either without a body, so SigMods
// returns an error for such a method without a default body. It accepts
// an abstract method, because an interface signature is abstract. It
// returns an error for a body without a default, because the template
// renders only a default body. It also returns an error for final,
// override and async. SigMods allocates once for the keywords of a
// private static method, and nothing for every other signature. A
// refusal allocates its error.
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
		part = privateKeyword
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
		part += staticKeyword
	case !private && m.HasDefault:
		part = defaultKeyword
	}
	return part, nil
}

// Annotate writes one annotation line at the top level for each
// annotation of a. A line has the name after the marker, and the
// arguments verbatim in parentheses where the annotation has any. It
// returns an error for an assigned annotation, because Java passes the
// arguments of an annotation in parentheses alone. It allocates what
// [textfmt.Marked] allocates. That is the lines, sized once, and nothing
// for no annotations.
func Annotate(a symbol.Annotations) (string, error) {
	if err := unassigned(a); err != nil {
		return "", err
	}
	return textfmt.Marked(a, "@", ""), nil
}

// MemberAnnotate writes the annotation lines of a member as [Annotate]
// writes them. Each line starts with the member indent of a type body.
// It returns the error of Annotate, and allocates what Annotate allocates.
func MemberAnnotate(a symbol.Annotations) (string, error) {
	if err := unassigned(a); err != nil {
		return "", err
	}
	return textfmt.Marked(a, "@", "", memberIndent), nil
}

// unassigned returns an error for the first assigned annotation of as,
// because Java passes the arguments of an annotation in parentheses
// alone, and nil where no annotation is assigned. It allocates nothing,
// and an error allocates its message.
func unassigned(as symbol.Annotations) error {
	for _, a := range as {
		if a.Assigned {
			return refuse("an annotation passes its arguments in parentheses, and %s assigns its argument", a.Name)
		}
	}
	return nil
}

// javaPackage returns the Java form of a slash-separated package path,
// with dots for slashes.
func javaPackage(path string) string { return strings.ReplaceAll(path, pathSep, memberSep) }

// access writes the access keyword of the visibility v of a member or a
// member type. It writes public for an unknown visibility, because other
// code calls a generated API. It writes nothing for package scope, which
// is the default access of Java. It returns an error for internal,
// because Java has no keyword for it.
func access(v symbol.Visibility, name string) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return publicKeyword, nil
	case symbol.VisibilityPackage:
		return "", nil
	case symbol.VisibilityPrivate:
		return privateKeyword, nil
	case symbol.VisibilityProtected:
		return protectedKeyword, nil
	default:
		return "", refuse("no access keyword spells the scope %s states", name)
	}
}

// refuse returns a refusal that starts with [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
