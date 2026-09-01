// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
	"path"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The vocabulary names, so a template and its helper cannot drift
// apart on a spelling.
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
	// FuncMods writes a module-level declaration's keywords.
	FuncMods = "mods"
	// FuncMemberMods writes a class member's keywords.
	FuncMemberMods = "membermods"
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
)

// Anonymous is what TypeScript writes where a declaration states
// no type: the type that admits everything while letting nothing
// through unchecked.
const Anonymous = "unknown"

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      Spell,
		FuncTypeParams: TypeParams,
		FuncParams:     Params,
		FuncResults:    Results,
		FuncMods:       Mods,
		FuncMemberMods: MemberMods,
		FuncPropMods:   PropMods,
		FuncSigMods:    SigMods,
		FuncBinding:    Binding,
		FuncDecorators: Decorators,
		FuncHeritage:   Heritage,
	}
}

// Docs writes a declaration's documentation as a TSDoc block,
// each line prefixed with the given indentation, so a member's
// doc sits at its member's depth.
func Docs(lines []string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	at := strings.Join(prefix, "")
	var b strings.Builder
	b.WriteString(at)
	b.WriteString("/**\n")
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString(" * ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(at)
	b.WriteString(" */\n")
	return b.String()
}

// Spell writes a type reference. The source spelling passes
// through verbatim; a missing one spells [Anonymous]. A reference
// carrying arguments holds its bare name in Spelling, and the
// argument list spells here in angle brackets.
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return Anonymous
	}
	if len(t.Args) == 0 {
		return t.Spelling
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a))
	}
	return t.Spelling + "<" + strings.Join(args, ", ") + ">"
}

// TypeParams writes a type parameter list in angle brackets, or
// nothing for a declaration stating none: the declared variance
// before the name, bounds folded into an intersection behind
// extends, and the default behind an equals sign. A value
// parameter refuses: the model's Const takes a value argument,
// TypeScript's const modifier narrows inference on a type one,
// and spelling one as the other would misstate the declaration.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Const {
			return "", fmt.Errorf(
				"typescript: a type parameter takes a type, and %s takes a value",
				p.Name)
		}
		part := variance(p.Variance) + p.Name
		if len(p.Bounds) > 0 {
			bounds := make([]string, 0, len(p.Bounds))
			for _, b := range p.Bounds {
				bounds = append(bounds, Spell(b))
			}
			part += " extends " + strings.Join(bounds, " & ")
		}
		if p.Default != nil {
			part += " = " + Spell(p.Default)
		}
		parts = append(parts, part)
	}
	return "<" + strings.Join(parts, ", ") + ">", nil
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

// Mods writes a module-level declaration's leading keywords:
// export for a public or unstated visibility, nothing for a
// package-scoped one, abstract before an abstract class, and
// async before an asynchronous function. A protected, private or
// internal visibility refuses at module level, a final class
// refuses because TypeScript seals nothing, and an annotation
// list refuses on every declaration decorators cannot mark:
// TypeScript decorates classes and their members alone.
func Mods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		if t.Final {
			return "", fmt.Errorf(
				"typescript: a class admits no final, and %s states it", t.Name)
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
			return "", fmt.Errorf(
				"typescript: an alias is transparent, and %s states a "+
					"defined type", t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Enum:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case t.Fields.Len() > 0 || t.Methods.Len() > 0:
			return "", fmt.Errorf(
				"typescript: an enum carries values alone, and %s states "+
					"members", t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Constant:
		if len(t.Annotations) > 0 {
			return "", undecorated(t.Name)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Variable:
		if len(t.Annotations) > 0 {
			return "", undecorated(t.Name)
		}
		return exported(t.Visibility, t.Name)
	default:
		return "", fmt.Errorf(
			"typescript: no module-level keywords spell a %s", d.Kind())
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
		return "", fmt.Errorf(
			"typescript: a module-level declaration exports or stays "+
				"module-scoped, and %s states another scope", name)
	}
}

// MemberMods writes a class member's leading keywords, in the
// order TypeScript states them: accessibility, static, abstract,
// override, async on methods, and readonly on fields. A final or
// default-carrying method refuses, and so does a package or
// internal accessibility, which class members do not take.
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
			return "", fmt.Errorf(
				"typescript: a method admits no final, and %s states it", t.Name)
		case t.HasDefault:
			return "", fmt.Errorf(
				"typescript: a class method carries its body outright, and %s "+
					"states a default", t.Name)
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
		if t.Async {
			part += "async "
		}
		return part, nil
	default:
		return "", fmt.Errorf(
			"typescript: no member keywords spell a %s", d.Kind())
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
		return "", fmt.Errorf(
			"typescript: a class member states public, private or protected, "+
				"and %s states another scope", name)
	}
}

// PropMods writes an interface property's keywords: readonly
// where the property is immutable, and nothing else, because an
// interface member takes no accessibility, no static level and no
// decorator.
func PropMods(f *emit.Field) (string, error) {
	switch {
	case len(f.Annotations) > 0:
		return "", undecorated(f.Name)
	case f.Visibility != symbol.VisibilityUnknown &&
		f.Visibility != symbol.VisibilityPublic:
		return "", fmt.Errorf(
			"typescript: an interface property is public by shape, and %s "+
				"states a scope", f.Name)
	case f.Level == symbol.LevelType:
		return "", fmt.Errorf(
			"typescript: an interface property has no static level, and %s "+
				"states one", f.Name)
	}
	if f.Mutability == symbol.MutabilityImmutable {
		return "readonly ", nil
	}
	return "", nil
}

// SigMods guards an interface method signature, which takes no
// keywords at all: a stated modifier refuses rather than
// dropping, and the signature spells bare.
func SigMods(m *emit.Method) (string, error) {
	switch {
	case len(m.Annotations) > 0:
		return "", undecorated(m.Name)
	case m.Visibility != symbol.VisibilityUnknown &&
		m.Visibility != symbol.VisibilityPublic:
		return "", fmt.Errorf(
			"typescript: an interface method is public by shape, and %s "+
				"states a scope", m.Name)
	case m.Level == symbol.LevelType || m.Abstract || m.Final ||
		m.Override || m.HasDefault || m.Async:
		return "", fmt.Errorf(
			"typescript: an interface method is a bare signature, and %s "+
				"states a modifier", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	}
	return "", nil
}

// Heritage writes a type's heritage clauses: one base behind
// extends and the contracts behind implements on a class, the
// widened contracts behind extends on an interface. A second
// class base refuses, because TypeScript extends one, and an
// embed refuses on either, because nothing promotes members.
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
			return "", fmt.Errorf(
				"typescript: a class extends one base, and %s states %d",
				t.Name, len(t.Extends))
		}
		if len(t.Implements) > 0 {
			part += " implements " + joined(t.Implements)
		}
		return part, nil
	case *emit.Interface:
		if len(t.Embeds) > 0 {
			return "", unembedded(t.Name)
		}
		if len(t.Extends) > 0 {
			return " extends " + joined(t.Extends), nil
		}
		return "", nil
	default:
		return "", fmt.Errorf(
			"typescript: no heritage clause spells a %s", d.Kind())
	}
}

// joined writes references as a comma-joined list.
func joined(ts []*emit.TypeRef) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, Spell(t))
	}
	return strings.Join(parts, ", ")
}

// unthrown is the refusal for declared failure types: a
// TypeScript signature declares none.
func unthrown(name string) error {
	return fmt.Errorf(
		"typescript: a signature declares no failure types, and %s states "+
			"throws", name)
}

// unembedded is the refusal for embeds: nothing promotes members.
func unembedded(name string) error {
	return fmt.Errorf(
		"typescript: nothing promotes members, and %s states embeds", name)
}

// Binding writes a module-level binding's keyword: const for an
// immutable binding, let otherwise.
func Binding(v *emit.Variable) string {
	if v.Mutability == symbol.MutabilityImmutable {
		return "const"
	}
	return "let"
}

// Decorators writes a declaration's decorator lines, one per
// annotation, each prefixed with the given indentation: the name
// behind its marker, and the argument spellings verbatim in
// parentheses where any are stated.
func Decorators(a emit.Annotations, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, an := range a {
		b.WriteString(at)
		b.WriteString("@")
		b.WriteString(an.Name)
		if len(an.Args) > 0 {
			b.WriteString("(")
			b.WriteString(strings.Join(an.Args, ", "))
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// undecorated is the refusal for an annotation list on a
// declaration decorators cannot mark.
func undecorated(name string) error {
	return fmt.Errorf(
		"typescript: decorators mark classes and their members, and %s "+
			"states annotations elsewhere", name)
}

// Params writes a parameter list, the rest marker included and a
// stated default behind its equals sign, verbatim the way the
// model carries it. A rest parameter stating a default refuses,
// because TypeScript initializes no rest.
func Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = "_"
		}
		if p.Variadic != symbol.VariadicNone {
			if p.Default != "" {
				return "", fmt.Errorf(
					"typescript: a rest parameter takes no default, and %s "+
						"states one", name)
			}
			parts = append(parts, "..."+name+": "+Spell(p.Type)+"[]")
			continue
		}
		part := name + ": " + Spell(p.Type)
		if p.Default != "" {
			part += " = " + p.Default
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", "), nil
}

// Results writes a return type annotation: void for none, the
// type for one, and a tuple for several, because TypeScript
// returns one value however many the delegate hands back.
func Results(rs []*emit.Return) string {
	switch len(rs) {
	case 0:
		return ": void"
	case 1:
		return ": " + Spell(rs[0].Type)
	default:
		parts := make([]string, 0, len(rs))
		for _, r := range rs {
			parts = append(parts, Spell(r.Type))
		}
		return ": [" + strings.Join(parts, ", ") + "]"
	}
}

// Module writes the module stem a file's owning package spells:
// the path's last element. TypeScript has no package clause, so
// the stem serves headers and diagnostics rather than a
// declaration.
func Module(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	return path.Base(id.Package)
}
