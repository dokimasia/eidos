// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
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
	// FuncBinder writes an impl block's binder.
	FuncBinder = "binder"
	// FuncParams writes a parameter list.
	FuncParams = "params"
	// FuncResults writes a return annotation.
	FuncResults = "results"
	// FuncVis writes a declaration's visibility.
	FuncVis = "vis"
	// FuncStructMods writes a struct's keywords.
	FuncStructMods = "structmods"
	// FuncFnMods writes a free function's keywords.
	FuncFnMods = "fnmods"
	// FuncTraitFn writes a trait method's keywords.
	FuncTraitFn = "traitfn"
	// FuncImplFn writes an impl method's keywords.
	FuncImplFn = "implfn"

	// FuncTypeNames writes a parameter list's names alone, for an
	// impl block's target.
	FuncTypeNames = "typenames"
	// FuncSelfParams writes a method's parameter list with its
	// receiver.
	FuncSelfParams = "selfparams"
	// FuncFieldMods writes a field's keywords.
	FuncFieldMods = "fieldmods"
	// FuncAttrs writes a declaration's attribute lines.
	FuncAttrs = "attrs"
	// FuncSupertraits writes a trait's supertrait bounds.
	FuncSupertraits = "supertraits"
	// FuncEnumMods writes an enum's keywords.
	FuncEnumMods = "enummods"
	// FuncAssocType writes a trait's associated type.
	FuncAssocType = "assoctype"
	// FuncSumMods writes a data enum's keywords.
	FuncSumMods = "summods"
	// FuncSumPayload writes a data enum variant's payload.
	FuncSumPayload = "sumpayload"
	// FuncAliasMods writes a type alias's keywords.
	FuncAliasMods = "aliasmods"
)

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:        Docs,
		FuncSpell:       Spell,
		FuncTypeParams:  TypeParams,
		FuncBinder:      Binder,
		FuncParams:      Params,
		FuncResults:     Results,
		FuncVis:         Vis,
		FuncStructMods:  StructMods,
		FuncFnMods:      FnMods,
		FuncTraitFn:     TraitFn,
		FuncImplFn:      ImplFn,
		FuncTypeNames:   TypeNames,
		FuncSelfParams:  SelfParams,
		FuncFieldMods:   FieldMods,
		FuncAttrs:       Attrs,
		FuncSupertraits: Supertraits,
		FuncEnumMods:    EnumMods,
		FuncAssocType:   AssocType,
		FuncSumMods:     SumMods,
		FuncSumPayload:  SumPayload,
		FuncAliasMods:   AliasMods,
	}
}

// Docs writes a declaration's documentation as outer doc
// comments, each line prefixed with the given indentation, so a
// member's doc sits at its member's depth.
func Docs(lines []string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString("/// ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Spell writes a type reference: the source spelling verbatim. A
// declaration stating no type has no Rust spelling at all, so the
// unit type stands in where a template reaches one, and the
// compiler's refusal names the file. A reference carrying
// arguments holds its bare name in Spelling, and the argument
// list spells here in angle brackets.
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return "()"
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
// nothing for a declaration stating none: bounds joined by plus
// signs behind a colon, the default behind an equals sign, and a
// value parameter in the const form with its value's type.
// Variance refuses: Rust infers it from use, a declaration states
// none, and dropping it would misstate the declaration.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Variance != symbol.VarianceInvariant {
			return "", fmt.Errorf(
				"rust: a type parameter states no variance, and %s states one: "+
					"variance is inferred from use", p.Name,
			)
		}
		parts = append(parts, typeParam(p))
	}
	return "<" + strings.Join(parts, ", ") + ">", nil
}

// TypeNames writes a parameter list's names alone, the form an
// impl block's target repeats: the bounds stay on the impl's own
// parameter list, and the target names the type they apply to.
func TypeNames(ps []*emit.TypeParam) string {
	if len(ps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, p.Name)
	}
	return "<" + strings.Join(parts, ", ") + ">"
}

// typeParam writes one parameter: the const form with its value
// type and default spelling, or the name behind its bounds with
// its default type.
func typeParam(p *emit.TypeParam) string {
	if p.Const {
		part := "const " + p.Name + ": " + Spell(p.Type)
		if p.DefaultValue != "" {
			part += " = " + p.DefaultValue
		}
		return part
	}
	part := p.Name
	if len(p.Bounds) > 0 {
		bounds := make([]string, 0, len(p.Bounds))
		for _, b := range p.Bounds {
			bounds = append(bounds, Spell(b))
		}
		part += ": " + strings.Join(bounds, " + ")
	}
	if p.Default != nil {
		part += " = " + Spell(p.Default)
	}
	return part
}

// Binder writes the impl binder restating a receiver's type
// arguments, or nothing for a receiver taking none. Each argument
// restates as the name the receiver references, bare: a bound
// stays on the methods the way Rust's own practice bounds
// functions rather than type definitions, and a receiver
// instantiated at a const argument has no restatable binder,
// which stays a declared limit.
func Binder(t *emit.TypeRef) string {
	if t == nil || len(t.Args) == 0 {
		return ""
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a))
	}
	return "<" + strings.Join(args, ", ") + ">"
}

// Vis writes a declaration's visibility: pub for public or
// unstated, because a generated API is consumed, pub(crate) for
// internal, nothing for package scope, which is Rust's
// module-private default. Private and protected refuse: Rust
// scopes by module, never by type or by subclass.
func Vis(v symbol.Visibility, name string) (string, error) {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return "pub ", nil
	case symbol.VisibilityInternal:
		return "pub(crate) ", nil
	case symbol.VisibilityPackage:
		return "", nil
	default:
		return "", fmt.Errorf(
			"rust: visibility scopes by module, and %s states a scope "+
				"modules cannot spell", name,
		)
	}
}

// StructMods writes a struct's keywords: its visibility alone. An
// abstract struct refuses, because every Rust struct can be made;
// a final one holds, because nothing subclasses; supertypes and
// embeds refuse, because a struct neither inherits nor promotes.
func StructMods(s *emit.Struct) (string, error) {
	switch {
	case s.Abstract:
		return "", fmt.Errorf(
			"rust: every struct can be made, and %s states abstract", s.Name,
		)
	case len(s.Extends) > 0 || len(s.Implements) > 0 || len(s.Embeds) > 0:
		return "", fmt.Errorf(
			"rust: a struct neither inherits nor promotes, and %s states "+
				"supertypes", s.Name,
		)
	}
	return Vis(s.Visibility, s.Name)
}

// EnumMods writes an enum's keywords: its visibility alone. An
// enum carrying fields or methods refuses, because Rust holds
// state in variants and behaviour in impl blocks.
func EnumMods(e *emit.Enum) (string, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return "", fmt.Errorf(
			"rust: an enum holds variants alone, and %s states members", e.Name,
		)
	}
	return Vis(e.Visibility, e.Name)
}

// AssocType writes one associated type: the name behind the
// keyword, its parameter list in angle brackets where the type is
// generic, which is what a trait declares and an implementation
// supplies. Only a transparent alias without a target spells that
// way, so anything else nested in a trait refuses, and so does a
// stated visibility or definedness: a trait item carries the
// trait's visibility, and an alias with a target is a default the
// stable language does not take.
func AssocType(s symbol.Symbol) (string, error) {
	a, held := s.(*emit.Alias)
	if !held {
		return "", fmt.Errorf(
			"rust: a trait nests associated types alone, and it holds a %s",
			s.Kind(),
		)
	}
	switch {
	case a.Target != nil:
		return "", fmt.Errorf(
			"rust: an associated type names what the implementation "+
				"supplies, and %s states a target", a.Name,
		)
	case a.Defined:
		return "", fmt.Errorf(
			"rust: an associated type defines nothing itself, and %s "+
				"states a defined type", a.Name,
		)
	case a.Visibility != symbol.VisibilityUnknown &&
		a.Visibility != symbol.VisibilityPublic:
		return "", fmt.Errorf(
			"rust: a trait item carries the trait's visibility, and %s "+
				"states its own", a.Name,
		)
	}
	params, err := TypeParams(a.TypeParams)
	if err != nil {
		return "", err
	}
	return "type " + a.Name + params + ";", nil
}

// SumMods writes a data enum's keywords: its visibility alone. A
// sum carrying methods refuses, because Rust holds behaviour in
// impl blocks.
func SumMods(s *emit.Sum) (string, error) {
	if s.Methods.Len() > 0 {
		return "", fmt.Errorf(
			"rust: a data enum holds variants alone, and %s states methods",
			s.Name,
		)
	}
	return Vis(s.Visibility, s.Name)
}

// SumPayload writes one variant's payload: nothing for an empty
// variant, named fields in braces for a struct variant, bare
// types in parentheses for a tuple variant. A payload mixing
// named and unnamed entries refuses, and so does an entry stating
// anything an inline spelling cannot carry.
func SumPayload(v *emit.SumVariant) (string, error) {
	fields := v.Fields.Items()
	if len(fields) == 0 {
		return "", nil
	}
	named := fields[0].Name != ""
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if err := inlineEntry(v.Name, f); err != nil {
			return "", err
		}
		if (f.Name != "") != named {
			return "", fmt.Errorf(
				"rust: a payload spells one way, and %s mixes named and "+
					"unnamed entries", v.Name,
			)
		}
		if named {
			parts = append(parts, f.Name+": "+Spell(f.Type))
		} else {
			parts = append(parts, Spell(f.Type))
		}
	}
	if named {
		return " { " + strings.Join(parts, ", ") + " }", nil
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

// inlineEntry refuses the payload facts an inline spelling cannot
// carry: a payload entry spells as a name and a type alone, on the
// variant's own line.
func inlineEntry(variant string, f *emit.Field) error {
	switch {
	case len(f.Doc) > 0 || f.Comment != "" || len(f.Annotations) > 0:
		return fmt.Errorf(
			"rust: a payload entry spells inline, and one in %s states "+
				"documentation, a comment or attributes", variant,
		)
	case f.Visibility != symbol.VisibilityUnknown:
		return fmt.Errorf(
			"rust: a payload follows its enum's visibility, and an entry in "+
				"%s states its own", variant,
		)
	case f.Level == symbol.LevelType:
		return fmt.Errorf(
			"rust: an enum holds no statics, and a payload entry in %s "+
				"states type level", variant,
		)
	case f.Mutability == symbol.MutabilityImmutable:
		return fmt.Errorf(
			"rust: a payload's mutability follows its owning binding, and "+
				"an entry in %s states its own", variant,
		)
	case f.Value != "":
		return fmt.Errorf(
			"rust: a payload declares no defaults, and an entry in %s "+
				"states one", variant,
		)
	}
	return nil
}

// AliasMods writes a type alias's keywords: its visibility alone.
// A defined type refuses, because a Rust alias is transparent.
func AliasMods(a *emit.Alias) (string, error) {
	if a.Defined {
		return "", fmt.Errorf(
			"rust: an alias is transparent, and %s states a defined type",
			a.Name,
		)
	}
	return Vis(a.Visibility, a.Name)
}

// Supertraits writes a trait's supertrait bounds behind a colon,
// joined by plus signs, or nothing where none are stated. An
// embed refuses, because a trait widens through supertraits
// alone.
func Supertraits(i *emit.Interface) (string, error) {
	if len(i.Embeds) > 0 {
		return "", fmt.Errorf(
			"rust: a trait widens through supertraits alone, and %s states "+
				"embeds", i.Name,
		)
	}
	if len(i.Extends) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(i.Extends))
	for _, t := range i.Extends {
		parts = append(parts, Spell(t))
	}
	return ": " + strings.Join(parts, " + "), nil
}

// FnMods writes a free function's keywords: its visibility, then
// async where the declaration states it.
func FnMods(f *emit.Function) (string, error) {
	part, err := Vis(f.Visibility, f.Name)
	if err != nil {
		return "", err
	}
	if f.Async {
		part += "async "
	}
	return part, nil
}

// TraitFn writes a trait method's keywords: async where stated,
// and nothing else. A trait item carries the trait's own
// visibility, so a stated scope refuses; abstract holds, because
// a bodiless signature is the trait's shape; final and override
// refuse, because Rust seals and overrides nothing.
func TraitFn(m *emit.Method) (string, error) {
	switch {
	case m.Visibility != symbol.VisibilityUnknown &&
		m.Visibility != symbol.VisibilityPublic:
		return "", fmt.Errorf(
			"rust: a trait item carries the trait's visibility, and %s "+
				"states its own", m.Name,
		)
	case m.Final:
		return "", fmt.Errorf(
			"rust: a method admits no final, and %s states it", m.Name,
		)
	case m.Override:
		return "", fmt.Errorf(
			"rust: a method overrides nothing, and %s states it", m.Name,
		)
	}
	if m.Async {
		return "async ", nil
	}
	return "", nil
}

// ImplFn writes an impl method's keywords: its visibility, then
// async where stated. Abstract and default refuse, because an
// impl method carries its body outright; final and override
// refuse the way every Rust method refuses them.
func ImplFn(m *emit.Method) (string, error) {
	switch {
	case m.Abstract:
		return "", fmt.Errorf(
			"rust: an impl method carries its body outright, and %s states "+
				"abstract", m.Name,
		)
	case m.HasDefault:
		return "", fmt.Errorf(
			"rust: default bodies belong to traits, and %s is an impl "+
				"method", m.Name,
		)
	case m.Final:
		return "", fmt.Errorf(
			"rust: a method admits no final, and %s states it", m.Name,
		)
	case m.Override:
		return "", fmt.Errorf(
			"rust: a method overrides nothing, and %s states it", m.Name,
		)
	}
	part, err := Vis(m.Visibility, m.Name)
	if err != nil {
		return "", err
	}
	if m.Async {
		part += "async "
	}
	return part, nil
}

// SelfParams writes a method's parameter list with its receiver:
// the reference receiver first at instance level, the parameters
// alone for an associated function at type level.
func SelfParams(m *emit.Method) string {
	if m.Level == symbol.LevelType {
		return Params(m.Params)
	}
	if len(m.Params) == 0 {
		return "&self"
	}
	return "&self, " + Params(m.Params)
}

// FieldMods writes a field's keywords: its visibility alone. A
// type-level field refuses, because Rust holds statics outside
// types; an immutable field refuses, because mutability follows
// the owning binding; an initializer refuses, because a struct
// declares no field defaults.
func FieldMods(f *emit.Field) (string, error) {
	switch {
	case f.Level == symbol.LevelType:
		return "", fmt.Errorf(
			"rust: a struct holds no statics, and %s states type level", f.Name,
		)
	case f.Mutability == symbol.MutabilityImmutable:
		return "", fmt.Errorf(
			"rust: a field's mutability follows its owning binding, and %s "+
				"states its own", f.Name,
		)
	case f.Value != "":
		return "", fmt.Errorf(
			"rust: a struct declares no field defaults, and %s states one",
			f.Name,
		)
	}
	return Vis(f.Visibility, f.Name)
}

// Attrs writes a declaration's attribute lines, one per
// annotation, each prefixed with the given indentation: the name
// in an outer attribute, its argument spellings verbatim in
// parentheses where any are stated.
func Attrs(a emit.Annotations, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, an := range a {
		b.WriteString(at)
		b.WriteString("#[")
		b.WriteString(an.Name)
		if len(an.Args) > 0 {
			b.WriteString("(")
			b.WriteString(strings.Join(an.Args, ", "))
			b.WriteString(")")
		}
		b.WriteString("]\n")
	}
	return b.String()
}

// Params writes a parameter list. An unnamed parameter binds to
// the discard pattern, which is the spelling Rust accepts.
func Params(ps []*emit.Param) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = "_"
		}
		parts = append(parts, name+": "+Spell(p.Type))
	}
	return strings.Join(parts, ", ")
}

// Results writes a return annotation: nothing for none, the type
// for one, and a tuple for several, which is how Rust returns
// more than one value.
func Results(rs []*emit.Return) string {
	switch len(rs) {
	case 0:
		return ""
	case 1:
		return " -> " + Spell(rs[0].Type)
	default:
		parts := make([]string, 0, len(rs))
		for _, r := range rs {
			parts = append(parts, Spell(r.Type))
		}
		return " -> (" + strings.Join(parts, ", ") + ")"
	}
}
