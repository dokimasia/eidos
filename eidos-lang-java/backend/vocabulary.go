// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"
	"text/template"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
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

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:         Docs,
		FuncSpell:        Spell,
		FuncTypeParams:   TypeParams,
		FuncParams:       Params,
		FuncResults:      Results,
		FuncPackage:      PackageClause,
		FuncTypeMods:     TypeMods,
		FuncMemberType:   MemberType,
		FuncFieldMods:    FieldMods,
		FuncConstantMods: ConstantMods,
		FuncMethodMods:   MethodMods,
		FuncSigMods:      SigMods,
		FuncAnnotate:     Annotate,
		FuncHeritage:     Heritage,
		FuncThrows:       Throws,
		FuncEnumVariant:  EnumVariantName,
	}
}

// Docs writes a declaration's documentation as a Javadoc block,
// each line prefixed with the given indentation, so a member's
// doc is at its member's depth.
func Docs(lines []string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	return textfmt.BlockDocs(lines, "/**", " * ", " */", prefix...)
}

// Spell writes a type reference. The source spelling passes
// through verbatim, and a missing one spells [Anonymous]. A
// reference with arguments has its bare name in Spelling, and the
// argument list spells here in angle brackets.
func Spell(t *emit.TypeRef) string {
	return spellref.Spell(t, "<", ">", Anonymous)
}

// TypeParams writes a type parameter list in angle brackets, or
// nothing for a declaration stating none, bounds joined by
// ampersands behind extends. Variance, defaults and value
// parameters refuse, because Java's variance is a use-site wildcard
// and its parameters take no default and no value.
func TypeParams(ps []*emit.TypeParam) (string, error) {
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
			bounds := make([]string, 0, len(p.Bounds))
			for _, b := range p.Bounds {
				bounds = append(bounds, Spell(b))
			}
			part += " extends " + strings.Join(bounds, " & ")
		}
		parts = append(parts, part)
	}
	return "<" + strings.Join(parts, ", ") + ">", nil
}

// Params writes a parameter list, the variadic marker included.
func Params(ps []*emit.Param) string {
	parts := make([]string, 0, len(ps))
	for i, p := range ps {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("arg%d", i)
		}
		spelling := Spell(p.Type)
		if p.Variadic != symbol.VariadicNone {
			spelling += "..."
		}
		parts = append(parts, spelling+" "+name+textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", ")
}

// Results writes the return type: void for none, the type for
// one, and an error for several, because a Java callable returns
// one value and a second arrives thrown, not returned.
func Results(rs []*emit.Return) (string, error) {
	switch len(rs) {
	case 0:
		return "void", nil
	case 1:
		return Spell(rs[0].Type) + textfmt.Inline(rs[0].Comment), nil
	default:
		return "", refuse("a callable returns one value, and this one states %d: "+
			"a second result arrives thrown, not returned", len(rs))
	}
}

// PackageClause writes the package statement from the identity's
// package path, dots for slashes, followed by a blank line. An
// identity naming no package spells nothing, which is the default
// package.
func PackageClause(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	return "package " + strings.ReplaceAll(id.Package, "/", ".") + ";\n\n"
}

// TypeMods writes a type's keywords in Java's stated order: access,
// then abstract, static, final and sealed on a class where stated,
// sealed alone on an interface, and access alone on an enum. The
// keywords are a member type's, because the kind templates render
// both file-level and member types: the lowering refuses a file-level
// type stating what only a member type can spell. A class both
// abstract and final refuses, and so does one both final and
// sealed, because javac rejects either pair.
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
// public.
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
// the constructor form these templates do not spell.
func EnumVariantName(v *emit.EnumVariant) (string, error) {
	if v.Value != "" {
		return "", refuse("an enum constant spells its name alone, and %s states a value", v.Name)
	}
	return v.Name, nil
}

// FieldMods writes a field's keywords, in Java's stated order:
// access, static for a type-level field, final for an immutable
// one.
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
// neither on an interface field.
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
// body refuses, because an abstract method is a signature.
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
// type-level method, and default for one with a default body at
// instance level. An abstract method passes, because an interface
// signature is abstract by shape. A body without a default refuses,
// because the template places only a default body, and final,
// override and asynchrony refuse.
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
	switch {
	case m.Level == symbol.LevelType:
		part += "static "
	case m.HasDefault:
		part += "default "
	}
	return part, nil
}

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

// Heritage writes a type's heritage clauses: one superclass
// behind extends and the contracts behind implements on a class,
// the widened contracts behind extends on an interface, and the
// enumerated subtypes behind permits on either, last the way Java
// states them. A second superclass refuses, because Java extends
// one, and an embed refuses on either, because nothing promotes
// members. The permits clause and the sealed keyword go together:
// javac rejects a sealed type in a file of its own without the
// clause, and the clause on a type that is not sealed.
func Heritage(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		if len(t.Embeds) > 0 {
			return "", unembedded(t.Name)
		}
		var part string
		switch len(t.Extends) {
		case 0:
		case 1:
			part = " extends " + Spell(t.Extends[0])
		default:
			return "", refuse("a class extends one superclass, and %s states %d",
				t.Name, len(t.Extends))
		}
		if len(t.Implements) > 0 {
			part += " implements " + joined(t.Implements)
		}
		permits, err := permitted(t.Name, t.Sealed, t.Permits)
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
			part = " extends " + joined(t.Extends)
		}
		permits, err := permitted(t.Name, t.Sealed, t.Permits)
		if err != nil {
			return "", err
		}
		return part + permits, nil
	default:
		return "", refuse("no heritage clause spells a %s", d.Kind())
	}
}

// permitted writes the permits clause, or nothing where no subtypes
// are enumerated. A sealed type without one refuses, and so does the
// clause on a type that is not sealed.
func permitted(name string, sealed bool, ts []*emit.TypeRef) (string, error) {
	switch {
	case sealed && len(ts) == 0:
		return "", refuse("a sealed type names its permitted subtypes, and %s names none", name)
	case !sealed && len(ts) > 0:
		return "", refuse("a permits clause belongs to a sealed type, and %s is not sealed", name)
	case len(ts) == 0:
		return "", nil
	default:
		return " permits " + joined(ts), nil
	}
}

// Throws writes a callable's throws clause: the declared failure
// types comma-joined behind the keyword, or nothing where none
// are stated.
func Throws(ts []*emit.TypeRef) string {
	if len(ts) == 0 {
		return ""
	}
	return " throws " + joined(ts)
}

// joined writes references as a comma-joined list.
func joined(ts []*emit.TypeRef) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, Spell(t))
	}
	return strings.Join(parts, ", ")
}

// unembedded is the refusal for embeds: nothing promotes members.
func unembedded(name string) error {
	return refuse("nothing promotes members, and %s states embeds", name)
}

// Annotate writes a declaration's annotation lines, one per
// annotation, each prefixed with the given indentation: the name
// behind its marker, and the argument spellings verbatim in
// parentheses where any are stated.
func Annotate(a symbol.Annotations, prefix ...string) string {
	return textfmt.Marked(a, "@", "", prefix...)
}

// refusalPrefix opens every refusal the backend returns: the
// language's identity, as every backend's refusals open.
const refusalPrefix = string(java.Lang) + ": "

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
