// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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
		FuncAccessor:   AccessorKw,
		FuncHard:       Hard,
		FuncIndexSig:   IndexSig,
		FuncPropKey:    PropKey,
		FuncMethodKey:  MethodKey,
	}
}

// PropKey writes a field's property key: an identifier bare, and
// anything else — a wire name like content-type, a digit-led key —
// in single quotes, so the declared name reaches the type whole
// instead of rendering invalid TypeScript. A hard-private field
// must be an identifier, because # admits no quoted form.
func PropKey(f *emit.Field) (string, error) {
	if identifier(f.Name) {
		return f.Name, nil
	}
	if f.Hard {
		return "", fmt.Errorf(
			"typescript: a hard-private name admits no quoted form, and "+
				"%s is not an identifier", f.Name,
		)
	}
	return quote(f.Name), nil
}

// MethodKey spells a method's member key: the identifier bare, and
// anything else quoted, which TypeScript admits on classes and
// interfaces alike. A hard-private name refuses the quoted form
// the way a property's does.
func MethodKey(m *emit.Method) (string, error) {
	if identifier(m.Name) {
		return m.Name, nil
	}
	if m.Hard {
		return "", fmt.Errorf(
			"typescript: a hard-private name admits no quoted form, and "+
				"%s is not an identifier", m.Name,
		)
	}
	return quote(m.Name), nil
}

// identifier reports whether a name spells bare in TypeScript:
// a letter, underscore or dollar first, those and digits after.
func identifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		head := r == '_' || r == '$' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if head || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// AccessorKw writes a method's accessor keyword and holds the
// declaration to the accessor's shape: a getter takes nothing and
// returns one value, a setter takes one value and returns
// nothing, and neither declares type parameters, because
// TypeScript's accessors admit none.
func AccessorKw(m *emit.Method) (string, error) {
	switch m.Accessor {
	case symbol.AccessorNone:
		return "", nil
	case symbol.AccessorGet:
		switch {
		case len(m.Params) != 0:
			return "", fmt.Errorf("typescript: a getter takes nothing, and %s takes parameters", m.Name)
		case len(m.Returns) == 0:
			return "", fmt.Errorf("typescript: a getter returns its property, and %s returns nothing", m.Name)
		case len(m.TypeParams) != 0:
			return "", fmt.Errorf("typescript: an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "get ", nil
	default:
		switch {
		case len(m.Params) != 1:
			return "", fmt.Errorf("typescript: a setter takes its one value, and %s does not", m.Name)
		case len(m.Returns) != 0:
			return "", fmt.Errorf("typescript: a setter returns nothing, and %s returns", m.Name)
		case len(m.TypeParams) != 0:
			return "", fmt.Errorf("typescript: an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "set ", nil
	}
}

// Hard writes a member's hard-private prefix. A hard-private name
// is runtime privacy in the name itself, so a stated visibility
// beside it refuses: the two mechanisms cannot combine.
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
		return "", fmt.Errorf(
			"typescript: a hard-private name carries its privacy in the "+
				"name, and %s states a visibility beside it", name,
		)
	}
	return "#", nil
}

// IndexSig writes an index signature whole: the one key parameter
// in brackets, the element type behind the colon. Anything an
// index signature cannot state — more parameters, no result, type
// parameters, an accessor — refuses.
func IndexSig(m *emit.Method) (string, error) {
	switch {
	case len(m.Params) != 1 || m.Params[0].Name == "" || m.Params[0].Type == nil:
		return "", fmt.Errorf(
			"typescript: an index signature takes one named, typed key, "+
				"and %s does not", m.Name,
		)
	case len(m.Returns) != 1:
		return "", fmt.Errorf(
			"typescript: an index signature states one element type, "+
				"and %s does not", m.Name,
		)
	case len(m.TypeParams) != 0 || m.Accessor != symbol.AccessorNone:
		return "", fmt.Errorf(
			"typescript: an index signature admits no type parameters "+
				"and no accessor, and %s states one", m.Name,
		)
	}
	return "[" + m.Params[0].Name + ": " + Spell(m.Params[0].Type) + "]: " +
		Spell(m.Returns[0].Type) + ";", nil
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
				p.Name,
			)
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
				"typescript: a class admits no final, and %s states it", t.Name,
			)
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
					"defined type", t.Name,
			)
		}
		return exported(t.Visibility, t.Name)
	case *emit.Enum:
		switch {
		case len(t.Annotations) > 0:
			return "", undecorated(t.Name)
		case t.Fields.Len() > 0 || t.Methods.Len() > 0:
			return "", fmt.Errorf(
				"typescript: an enum carries values alone, and %s states "+
					"members", t.Name,
			)
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
			"typescript: no module-level keywords spell a %s", d.Kind(),
		)
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
				"module-scoped, and %s states another scope", name,
		)
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
				"typescript: a method admits no final, and %s states it", t.Name,
			)
		case t.HasDefault:
			return "", fmt.Errorf(
				"typescript: a class method carries its body outright, and %s "+
					"states a default", t.Name,
			)
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
			"typescript: no member keywords spell a %s", d.Kind(),
		)
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
				"and %s states another scope", name,
		)
	}
}

// PropMods writes an interface property's keywords: readonly
// where the property is immutable, and nothing else, because an
// interface member takes no accessibility, no static level and no
// decorator.
func PropMods(f *emit.Field) (string, error) {
	switch {
	case f.Hard:
		return "", fmt.Errorf(
			"typescript: an interface property has no runtime, and %s "+
				"states a hard-private name", f.Name,
		)
	case len(f.Annotations) > 0:
		return "", undecorated(f.Name)
	case f.Visibility != symbol.VisibilityUnknown &&
		f.Visibility != symbol.VisibilityPublic:
		return "", fmt.Errorf(
			"typescript: an interface property is public by shape, and %s "+
				"states a scope", f.Name,
		)
	case f.Level == symbol.LevelType:
		return "", fmt.Errorf(
			"typescript: an interface property has no static level, and %s "+
				"states one", f.Name,
		)
	case f.Value != "":
		return "", fmt.Errorf(
			"typescript: an interface property carries no initializer, and "+
				"%s states one", f.Name,
		)
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
				"states a scope", m.Name,
		)
	case m.Accessor != symbol.AccessorNone || m.Hard:
		return "", fmt.Errorf(
			"typescript: an interface states properties, not accessors or "+
				"hard-private names, and %s states one", m.Name,
		)
	case m.Level == symbol.LevelType || m.Abstract || m.Final ||
		m.Override || m.HasDefault || m.Async:
		return "", fmt.Errorf(
			"typescript: an interface method is a bare signature, and %s "+
				"states a modifier", m.Name,
		)
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
				t.Name, len(t.Extends),
			)
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
			"typescript: no heritage clause spells a %s", d.Kind(),
		)
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
			"throws", name,
	)
}

// unembedded is the refusal for embeds: nothing promotes members.
func unembedded(name string) error {
	return fmt.Errorf(
		"typescript: nothing promotes members, and %s states embeds", name,
	)
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
func Decorators(a symbol.Annotations, prefix ...string) string {
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
			"states annotations elsewhere", name,
	)
}

// Params writes a parameter list, the rest marker included and a
// stated default behind its equals sign, verbatim the way the
// model carries it. A rest parameter stating a default refuses,
// because TypeScript initializes no rest.
func Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	unnamed := 0
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = "_"
			if unnamed > 0 {
				name = fmt.Sprintf("_%d", unnamed)
			}
			unnamed++
		} else if !identifier(name) {
			return "", fmt.Errorf(
				"typescript: a parameter admits no quoted form, and %q is "+
					"not an identifier", name,
			)
		}
		if p.Variadic != symbol.VariadicNone {
			switch {
			case p.Default != "":
				return "", fmt.Errorf(
					"typescript: a rest parameter takes no default, and %s "+
						"states one", name,
				)
			case p.Optional:
				return "", fmt.Errorf(
					"typescript: a rest parameter is optional by shape, and "+
						"%s states it", name,
				)
			}
			parts = append(parts, "..."+name+": "+Spell(p.Type)+"[]")
			continue
		}
		if p.Optional && p.Default != "" {
			return "", fmt.Errorf(
				"typescript: a default already makes %s optional, and it "+
					"states the marker beside it", name,
			)
		}
		part := name
		if p.Optional {
			part += "?"
		}
		part += ": " + Spell(p.Type)
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
