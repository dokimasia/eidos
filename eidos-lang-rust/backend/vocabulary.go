// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/naming"
	rust "go.dokimi.dev/eidos/lang/rust"
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
	// FuncTypeParams writes a type definition's parameter list.
	FuncTypeParams = "typeparams"
	// FuncImplParams writes an impl block's parameter list.
	FuncImplParams = "implparams"
	// FuncFnParams writes a function's, a method's or an associated
	// type's parameter list.
	FuncFnParams = "fnparams"
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
	// FuncConstType writes a constant's stated type, refusing an
	// unstated type and a missing value.
	FuncConstType = "consttype"
)

// The receiver spellings: the name every receiver binds, the
// shorthand forms of a borrowed receiver, the prefix of the typed
// form, and the three receiver types the shorthand forms stand for.
const (
	selfName       = "self"
	receiverRef    = "&self"
	receiverMutRef = "&mut self"
	receiverTyped  = "self: "
	selfType       = "Self"
	selfRefType    = "&Self"
	selfMutType    = "&mut Self"
)

// The punctuation the vocabulary writes: the brackets of an argument
// list, the unit type the shared speller takes as its stand-in, and
// the discard pattern an unnamed parameter binds.
const (
	genericsOpener = "<"
	genericsCloser = ">"
	unitSpelling   = "()"
	discardName    = "_"
)

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:        Docs,
		FuncSpell:       Spell,
		FuncTypeParams:  TypeParams,
		FuncImplParams:  ImplParams,
		FuncFnParams:    FnParams,
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
		FuncConstType:   ConstType,
	}
}

// ConstType writes a constant's stated type. It refuses a constant
// without a type, because rustc requires the annotation and no
// formatter catches its absence, and a constant without a value,
// because const X: T = ; declares nothing.
func ConstType(c *emit.Constant) (string, error) {
	switch {
	case c.Type == nil || c.Type.Spelling == "":
		return "", refuse("a constant states its type, and %s states none", c.Name)
	case c.Value == "":
		return "", refuse("a constant takes a value, and %s states none", c.Name)
	}
	return Spell(c.Type)
}

// Docs writes a declaration's documentation as outer doc
// comments, each line prefixed with the given indentation, so a
// member's doc is at its member's depth.
func Docs(lines []string, prefix ...string) string {
	return textfmt.LineDocs(lines, "/// ", prefix...)
}

// Spell writes a type reference: the source spelling verbatim, and
// behind a reference with arguments the argument list in angle
// brackets. A reference that states no type, itself or in an
// argument list at any depth, refuses: Rust has no spelling for an
// absent type, and the unit type in its place compiles and changes
// what the declaration states.
func Spell(t *emit.TypeRef) (string, error) {
	if !complete(t) {
		return "", refuse(unstatedType)
	}
	return spellref.Spell(t, genericsOpener, genericsCloser, unitSpelling), nil
}

// unstatedType is the reason a reference that states no type
// refuses.
const unstatedType = "a declaration states no type, and Rust spells none in its place"

// complete reports whether a reference states its type, itself and
// in every argument list at any depth.
func complete(t *emit.TypeRef) bool {
	if t == nil || t.Spelling == "" {
		return false
	}
	for _, a := range t.Args {
		if !complete(a) {
			return false
		}
	}
	return true
}

// paramsPosition is the position a type parameter list is written
// in. The positions differ in what Rust accepts: a type definition
// takes a default, an impl block restates the definition's
// parameters without their defaults, and a function, a method and an
// associated type take none.
type paramsPosition uint8

// The three positions.
const (
	definitionPosition paramsPosition = iota
	implPosition
	callablePosition
)

// TypeParams writes a type definition's parameter list in angle
// brackets, or nothing for a declaration stating none: bounds joined
// by plus signs behind a colon, the default behind an equals sign,
// and a value parameter in the const form with its value's type.
// Variance refuses, because Rust infers it from use and a
// declaration states none.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	return typeParams(ps, definitionPosition)
}

// ImplParams writes an impl block's parameter list: a type
// definition's parameters with their bounds and without their
// defaults, because the definition states the defaults and rustc
// refuses one on an impl.
func ImplParams(ps []*emit.TypeParam) (string, error) {
	return typeParams(ps, implPosition)
}

// FnParams writes a function's, a method's or an associated type's
// parameter list. A stated default refuses, because Rust takes a
// default on a type definition's parameter alone.
func FnParams(ps []*emit.TypeParam) (string, error) {
	return typeParams(ps, callablePosition)
}

// typeParams writes a parameter list in one position.
func typeParams(ps []*emit.TypeParam, at paramsPosition) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Variance != symbol.VarianceInvariant {
			return "", refuse("a type parameter states no variance, and %s states one: "+
				"variance is inferred from use", p.Name)
		}
		if at == callablePosition && (p.Default != nil || p.DefaultValue != "") {
			return "", refuse("a type parameter takes a default on a type definition alone, "+
				"and %s states one", p.Name)
		}
		part, err := typeParam(p, at == definitionPosition)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return genericsOpener + strings.Join(parts, ", ") + genericsCloser, nil
}

// TypeNames writes a parameter list's names alone, the form an
// impl block's target repeats: the impl's own parameter list states
// the bounds, and the target names the type they apply to.
func TypeNames(ps []*emit.TypeParam) string {
	if len(ps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, p.Name)
	}
	return genericsOpener + strings.Join(parts, ", ") + genericsCloser
}

// typeParam writes one parameter: the const form with its value
// type, or the name behind its bounds, each with its default where
// defaults is set.
func typeParam(p *emit.TypeParam, defaults bool) (string, error) {
	if p.Const {
		typ, err := Spell(p.Type)
		if err != nil {
			return "", err
		}
		part := "const " + p.Name + ": " + typ
		if defaults && p.DefaultValue != "" {
			part += " = " + p.DefaultValue
		}
		return part, nil
	}
	part := p.Name
	if len(p.Bounds) > 0 {
		bounds := make([]string, 0, len(p.Bounds))
		for _, b := range p.Bounds {
			spelled, err := Spell(b)
			if err != nil {
				return "", err
			}
			bounds = append(bounds, spelled)
		}
		part += ": " + strings.Join(bounds, " + ")
	}
	if defaults && p.Default != nil {
		spelled, err := Spell(p.Default)
		if err != nil {
			return "", err
		}
		part += " = " + spelled
	}
	return part, nil
}

// Binder writes the impl binder of a receiver's type parameters, or
// nothing for a receiver taking none. An argument binds where it
// names a type parameter: its target is one, or it has no target and
// is a bare identifier that names no [builtinTypes] entry. Every
// other argument is concrete and binds nothing: a declaration, an
// instantiation, a path, a structural form, a primitive or String.
// A method on Box<T> therefore opens impl<T> Box<T>, and a method on
// Wrapper<String> opens impl Wrapper<String>. The methods state the
// bounds, the way Rust practice bounds functions and not type
// definitions.
func Binder(t *emit.TypeRef) (string, error) {
	if t == nil || len(t.Args) == 0 {
		return "", nil
	}
	var params []string
	for _, a := range t.Args {
		if !complete(a) {
			return "", refuse(unstatedType)
		}
		if typeParameter(a) {
			params = append(params, a.Spelling)
		}
	}
	if len(params) == 0 {
		return "", nil
	}
	return genericsOpener + strings.Join(params, ", ") + genericsCloser, nil
}

// typeParameter reports whether a receiver argument names a type
// parameter.
func typeParameter(a *emit.TypeRef) bool {
	if !a.Target.IsZero() {
		return a.Target.Kind == symbol.KindTypeParam
	}
	// ponytail: an unresolved bare identifier reads as a parameter, so a
	// std type the resolution left open, such as PathBuf, binds as one.
	// A generator marks a concrete argument with its target.
	name := bareName(a)
	_, builtin := builtinTypes[name]
	return naming.IsIdentifier(name) && !builtin
}

// bareName returns the name a reference spells where it is a bare
// name the resolution left open, and nothing for a declaration, an
// instantiation or a structural form.
func bareName(t *emit.TypeRef) string {
	if t == nil || !t.Target.IsZero() || t.Form != symbol.FormNamed || len(t.Args) > 0 {
		return ""
	}
	return t.Spelling
}

// builtinTypes maps each type name Rust declares without a
// declaration in the workspace to whether the type is numeric. The
// numeric primitives convert into one another by cast. bool, char,
// str and the prelude's String convert through traits and methods.
// No such name is a type parameter or a tuple struct.
var builtinTypes = map[string]bool{
	"bool": false, "char": false, "str": false, "String": false,
	"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
	"f32": true, "f64": true,
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
		return "", refuse("visibility scopes by module, and %s states a scope "+
			"modules cannot spell", name)
	}
}

// StructMods writes a struct's keywords: its visibility alone. An
// abstract struct refuses, because every Rust struct can be made. A
// final struct passes, because nothing subclasses. Supertypes and
// embeds refuse, because a struct neither inherits nor promotes.
func StructMods(s *emit.Struct) (string, error) {
	switch {
	case s.Abstract:
		return "", refuse("every struct can be made, and %s states abstract", s.Name)
	case len(s.Extends) > 0 || len(s.Implements) > 0 || len(s.Embeds) > 0:
		return "", refuse("a struct neither inherits nor promotes, and %s states supertypes", s.Name)
	}
	return Vis(s.Visibility, s.Name)
}

// EnumMods writes an enum's keywords: its visibility alone. An
// enum with fields or methods refuses, because Rust puts state in
// variants and behaviour in impl blocks.
func EnumMods(e *emit.Enum) (string, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return "", refuse("an enum has variants alone, and %s states members", e.Name)
	}
	return Vis(e.Visibility, e.Name)
}

// AssocType writes one associated type: the name behind the
// keyword, its parameter list in angle brackets where the type is
// generic, which is what a trait declares and an implementation
// supplies. Only a transparent alias without a target spells that
// way, so anything else nested in a trait refuses. A stated
// visibility refuses, because a trait item takes the trait's
// visibility, and so does a target or definedness, because an alias
// with a target is a default the stable language does not take.
func AssocType(s symbol.Symbol) (string, error) {
	a, is := s.(*emit.Alias)
	if !is {
		return "", refuse("a trait nests associated types alone, and it declares a %s", s.Kind())
	}
	switch {
	case a.Target != nil:
		return "", refuse("an associated type names what the implementation "+
			"supplies, and %s states a target", a.Name)
	case a.Defined:
		return "", refuse("an associated type defines nothing itself, and %s "+
			"states a defined type", a.Name)
	case a.Visibility != symbol.VisibilityUnknown &&
		a.Visibility != symbol.VisibilityPublic:
		return "", refuse("a trait item takes the trait's visibility, and %s states its own", a.Name)
	}
	params, err := FnParams(a.TypeParams)
	if err != nil {
		return "", err
	}
	return "type " + a.Name + params + ";", nil
}

// SumMods writes a data enum's keywords: its visibility alone. A
// sum with methods refuses, because Rust puts behaviour in impl
// blocks.
func SumMods(s *emit.Sum) (string, error) {
	if s.Methods.Len() > 0 {
		return "", refuse("a data enum has variants alone, and %s states methods", s.Name)
	}
	return Vis(s.Visibility, s.Name)
}

// SumPayload writes one variant's payload: nothing for an empty
// variant, named fields in braces for a struct variant, bare
// types in parentheses for a tuple variant. A payload mixing
// named and unnamed entries refuses, and so does an entry stating
// anything an inline spelling cannot express.
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
			return "", refuse("a payload spells one way, and %s mixes named and "+
				"unnamed entries", v.Name)
		}
		typ, err := Spell(f.Type)
		if err != nil {
			return "", err
		}
		if named {
			parts = append(parts, f.Name+": "+typ)
		} else {
			parts = append(parts, typ)
		}
	}
	if named {
		return " { " + strings.Join(parts, ", ") + " }", nil
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

// inlineEntry refuses the payload facts an inline spelling cannot
// express: a payload entry spells as a name and a type alone, on
// the variant's own line.
func inlineEntry(variant string, f *emit.Field) error {
	switch {
	case len(f.Doc) > 0 || f.Comment != "" || len(f.Annotations) > 0:
		return refuse("a payload entry spells inline, and one in %s states "+
			"documentation, a comment or attributes", variant)
	case f.Visibility != symbol.VisibilityUnknown:
		return refuse("a payload follows its enum's visibility, and an entry in "+
			"%s states its own", variant)
	case f.Level == symbol.LevelType:
		return refuse("an enum has no statics, and a payload entry in %s "+
			"states type level", variant)
	case f.Mutability == symbol.MutabilityImmutable:
		return refuse("a payload's mutability follows its owning binding, and "+
			"an entry in %s states its own", variant)
	case f.Value != "":
		return refuse("a payload declares no defaults, and an entry in %s "+
			"states one", variant)
	}
	return nil
}

// AliasMods writes a type alias's keywords: its visibility alone.
// A defined type refuses, because a Rust alias is transparent.
func AliasMods(a *emit.Alias) (string, error) {
	if a.Defined {
		return "", refuse("an alias is transparent, and %s states a defined type", a.Name)
	}
	return Vis(a.Visibility, a.Name)
}

// Supertraits writes a trait's supertrait bounds behind a colon,
// joined by plus signs, or nothing where none are stated. An
// embed refuses, because a trait widens through supertraits
// alone.
func Supertraits(i *emit.Interface) (string, error) {
	if len(i.Embeds) > 0 {
		return "", refuse("a trait widens through supertraits alone, and %s states embeds", i.Name)
	}
	if len(i.Extends) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(i.Extends))
	for _, t := range i.Extends {
		spelled, err := Spell(t)
		if err != nil {
			return "", err
		}
		parts = append(parts, spelled)
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
// and nothing else. A trait item takes the trait's own visibility,
// so a stated scope refuses. An abstract method passes, because a
// trait method without a default is a signature, and a body on
// such a method refuses, because the signature cannot place it.
// Final refuses, because every implementation may override a trait
// method, and override refuses, because a trait method overrides
// nothing.
func TraitFn(m *emit.Method) (string, error) {
	switch {
	case !m.HasDefault && !m.Body.IsZero():
		return "", refuse("a trait method without a default is a signature, and %s states a body", m.Name)
	case m.Visibility != symbol.VisibilityUnknown &&
		m.Visibility != symbol.VisibilityPublic:
		return "", refuse("a trait item takes the trait's visibility, and %s states its own", m.Name)
	case m.Final:
		return "", refuse("every implementation may override a trait method, and %s states final", m.Name)
	case m.Override:
		return "", refuse("a method overrides nothing, and %s states it", m.Name)
	}
	if m.Async {
		return "async ", nil
	}
	return "", nil
}

// ImplFn writes an impl method's keywords: its visibility, then
// async where stated. Abstract and default refuse, because an impl
// method states its body outright, and override refuses, because an
// inherent method overrides nothing. Final passes, because nothing
// overrides an inherent method.
func ImplFn(m *emit.Method) (string, error) {
	switch {
	case m.Abstract:
		return "", refuse("an impl method states its body outright, and %s states abstract", m.Name)
	case m.HasDefault:
		return "", refuse("default bodies belong to traits, and %s is an impl method", m.Name)
	case m.Override:
		return "", refuse("a method overrides nothing, and %s states it", m.Name)
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

// SelfParams writes a method's parameter list with its receiver: the
// stated receiver where the method states one, the shared borrow
// &self where it states none, and the parameters alone for an
// associated function at type level, which refuses a stated
// receiver.
func SelfParams(m *emit.Method) (string, error) {
	params, err := Params(m.Params)
	if err != nil {
		return "", err
	}
	if m.Level == symbol.LevelType {
		if m.Receiver != nil {
			return "", refuse("an associated function takes no receiver, and %s states one", m.Name)
		}
		return params, nil
	}
	recv := receiverRef
	if m.Receiver != nil {
		if recv, err = receiver(m.Name, m.Receiver); err != nil {
			return "", err
		}
	}
	if params == "" {
		return recv, nil
	}
	return recv + ", " + params, nil
}

// receiver writes a stated receiver from its type: Self as self,
// &Self as &self, &mut Self as &mut self, and any other type, such
// as Box<Self> or &'a Self, in the typed form self: T. Its trailing
// comment follows as a block comment. A receiver binds self alone,
// so a stated name other than self refuses.
func receiver(method string, p *emit.Param) (string, error) {
	if p.Name != "" && p.Name != selfName {
		return "", refuse("a receiver binds self, and %s names its receiver %s", method, p.Name)
	}
	typ, err := Spell(p.Type)
	if err != nil {
		return "", err
	}
	var spelled string
	switch typ {
	case selfType:
		spelled = selfName
	case selfRefType:
		spelled = receiverRef
	case selfMutType:
		spelled = receiverMutRef
	default:
		spelled = receiverTyped + typ
	}
	return spelled + textfmt.Inline(p.Comment), nil
}

// FieldMods writes a field's keywords: its visibility alone. A
// type-level field refuses, because Rust declares statics outside
// types. An immutable field refuses, because mutability follows the
// owning binding. An initializer refuses, because a struct declares
// no field defaults.
func FieldMods(f *emit.Field) (string, error) {
	switch {
	case f.Level == symbol.LevelType:
		return "", refuse("a struct has no statics, and %s states type level", f.Name)
	case f.Mutability == symbol.MutabilityImmutable:
		return "", refuse("a field's mutability follows its owning binding, and %s "+
			"states its own", f.Name)
	case f.Value != "":
		return "", refuse("a struct declares no field defaults, and %s states one", f.Name)
	}
	return Vis(f.Visibility, f.Name)
}

// Attrs writes a declaration's attribute lines, one per
// annotation, each prefixed with the given indentation: the name
// in an outer attribute, its argument spellings verbatim in
// parentheses where any are stated.
func Attrs(a symbol.Annotations, prefix ...string) string {
	return textfmt.Marked(a, "#[", "]", prefix...)
}

// Params writes a parameter list. An unnamed parameter binds to
// the discard pattern, which is the spelling Rust accepts, and a
// parameter that states no type refuses.
func Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = discardName
		}
		typ, err := Spell(p.Type)
		if err != nil {
			return "", err
		}
		parts = append(parts, name+": "+typ+textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", "), nil
}

// Results writes a return annotation: nothing for none, the type
// for one, and a tuple for several, which is how Rust returns
// more than one value. A result that states no type refuses.
func Results(rs []*emit.Return) (string, error) {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		typ, err := Spell(r.Type)
		if err != nil {
			return "", err
		}
		parts = append(parts, typ+textfmt.Inline(r.Comment))
	}
	switch len(parts) {
	case 0:
		return "", nil
	case 1:
		return " -> " + parts[0], nil
	default:
		return " -> (" + strings.Join(parts, ", ") + ")", nil
	}
}

// refusalPrefix opens every refusal the backend returns: the
// language's identity, as every backend's refusals open.
const refusalPrefix = string(rust.Lang) + ": "

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
