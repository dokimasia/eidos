// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
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
	// FuncResults writes the one return type.
	FuncResults = "results"
	// FuncPackage writes the package clause.
	FuncPackage = "package"
	// FuncTypeMods writes a file-level type's keywords.
	FuncTypeMods = "typemods"
	// FuncFieldMods writes a field's keywords.
	FuncFieldMods = "fieldmods"
	// FuncMethodMods writes a class method's keywords.
	FuncMethodMods = "methodmods"
	// FuncSigMods writes an interface method's keywords.
	FuncSigMods = "sigmods"
	// FuncAnnotate writes a declaration's annotation lines.
	FuncAnnotate = "annotate"
)

// Anonymous is what Java writes where a declaration states no
// type: the root every reference type descends from.
const Anonymous = "Object"

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      Spell,
		FuncTypeParams: TypeParams,
		FuncParams:     Params,
		FuncResults:    Results,
		FuncPackage:    PackageClause,
		FuncTypeMods:   TypeMods,
		FuncFieldMods:  FieldMods,
		FuncMethodMods: MethodMods,
		FuncSigMods:    SigMods,
		FuncAnnotate:   Annotate,
	}
}

// Docs writes a declaration's documentation as a Javadoc block,
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
// nothing for a declaration stating none, bounds joined by
// ampersands behind extends. Variance, defaults and value
// parameters refuse: Java's variance is a use-site wildcard, its
// parameters take no default and no value, and dropping any of
// the three would misstate the declaration.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		switch {
		case p.Variance != symbol.VarianceInvariant:
			return "", fmt.Errorf(
				"java: a type parameter states no variance, and %s states one: "+
					"the wildcard is use-site", p.Name)
		case p.Const:
			return "", fmt.Errorf(
				"java: a type parameter takes a type, and %s takes a value", p.Name)
		case p.Default != nil:
			return "", fmt.Errorf(
				"java: a type parameter takes no default, and %s states one", p.Name)
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
		parts = append(parts, spelling+" "+name)
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
		return Spell(rs[0].Type), nil
	default:
		return "", fmt.Errorf(
			"java: a callable returns one value, and this one states %d: "+
				"a second result arrives thrown, not returned", len(rs))
	}
}

// PackageClause writes the package statement from the owning
// package's path, dots for slashes, followed by a blank line. An
// identity naming no package spells nothing, which is the default
// package.
func PackageClause(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	return "package " + strings.ReplaceAll(id.Package, "/", ".") + ";\n\n"
}

// TypeMods writes a file-level type's keywords: public for a
// public or unstated visibility, nothing for a package-scoped
// one, then abstract or final on a class where stated. A private,
// protected or internal visibility refuses, because Java's
// file-level types take public or default access alone.
func TypeMods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		part, err := access(t.Visibility, t.Name, true)
		if err != nil {
			return "", err
		}
		if t.Abstract {
			part += "abstract "
		}
		if t.Final {
			part += "final "
		}
		return part, nil
	case *emit.Interface:
		return access(t.Visibility, t.Name, true)
	default:
		return "", fmt.Errorf(
			"java: no file-level keywords spell a %s", d.Kind())
	}
}

// FieldMods writes a field's keywords, in Java's stated order:
// access, static for a type-level field, final for an immutable
// one.
func FieldMods(f *emit.Field) (string, error) {
	part, err := access(f.Visibility, f.Name, false)
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

// MethodMods writes a class method's keywords, in Java's stated
// order: access, static, abstract, final. An asynchronous method
// refuses, because Java marks no signature asynchronous, and a
// default refuses outside an interface.
func MethodMods(m *emit.Method) (string, error) {
	switch {
	case m.Async:
		return "", fmt.Errorf(
			"java: a signature carries no asynchrony, and %s states it", m.Name)
	case m.HasDefault:
		return "", fmt.Errorf(
			"java: default belongs to interface methods, and %s is a class "+
				"member", m.Name)
	}
	part, err := access(m.Visibility, m.Name, false)
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
// implicitly public signature, private where stated, static for
// a type-level method, and default for one carrying a body at
// instance level. Abstract holds, because an interface signature
// is abstract by shape; final, override and asynchrony refuse.
func SigMods(m *emit.Method) (string, error) {
	switch {
	case m.Final:
		return "", fmt.Errorf(
			"java: an interface method admits no final, and %s states it", m.Name)
	case m.Override:
		return "", fmt.Errorf(
			"java: an interface method overrides nothing, and %s states it",
			m.Name)
	case m.Async:
		return "", fmt.Errorf(
			"java: a signature carries no asynchrony, and %s states it", m.Name)
	}
	var part string
	switch m.Visibility {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		part = ""
	case symbol.VisibilityPrivate:
		part = "private "
	default:
		return "", fmt.Errorf(
			"java: an interface method is public or private, and %s states "+
				"another scope", m.Name)
	}
	switch {
	case m.Level == symbol.LevelType:
		part += "static "
	case m.HasDefault:
		part += "default "
	}
	return part, nil
}

// access writes a member's or type's access keyword. A member
// unstated spells public, because a generated API exists to be
// called; package scope spells Java's default access. A
// file-level type takes public or default access alone, and an
// internal scope has no Java spelling at all.
func access(v symbol.Visibility, name string, fileLevel bool) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return "public ", nil
	case symbol.VisibilityPackage:
		return "", nil
	case symbol.VisibilityPrivate:
		if !fileLevel {
			return "private ", nil
		}
	case symbol.VisibilityProtected:
		if !fileLevel {
			return "protected ", nil
		}
	}
	return "", fmt.Errorf(
		"java: no access keyword spells the scope %s states", name)
}

// Annotate writes a declaration's annotation lines, one per
// annotation, each prefixed with the given indentation: the name
// behind its marker, and the argument spellings verbatim in
// parentheses where any are stated.
func Annotate(a emit.Annotations, prefix ...string) string {
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
