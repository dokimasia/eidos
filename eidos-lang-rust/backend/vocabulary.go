// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/rust"
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
	// FuncSpell is the template name of [Speller.Spell].
	FuncSpell = render.HelperSpell
	// FuncTypeParams is the template name of [Speller.TypeParams].
	FuncTypeParams = render.HelperTypeParams
	// FuncImplParams is the template name of [Speller.ImplParams].
	FuncImplParams = "implparams"
	// FuncFnParams is the template name of [Speller.FnParams].
	FuncFnParams = "fnparams"
	// FuncBinder is the template name of [Binder].
	FuncBinder = "binder"
	// FuncParams is the template name of [Speller.Params].
	FuncParams = render.HelperParams
	// FuncResults is the template name of [Speller.Results].
	FuncResults = render.HelperResults
	// FuncVis is the template name of [Vis].
	FuncVis = "vis"
	// FuncStructMods is the template name of [StructMods].
	FuncStructMods = "structmods"
	// FuncFnMods is the template name of [FnMods].
	FuncFnMods = "fnmods"
	// FuncTraitFn is the template name of [TraitFn].
	FuncTraitFn = "traitfn"
	// FuncImplFn is the template name of [ImplFn].
	FuncImplFn = "implfn"

	// FuncTypeNames is the template name of [TypeNames].
	FuncTypeNames = "typenames"
	// FuncSelfParams is the template name of [Speller.SelfParams].
	FuncSelfParams = "selfparams"
	// FuncFieldMods is the template name of [FieldMods].
	FuncFieldMods = "fieldmods"
	// FuncAttrs is the template name of [Attrs].
	FuncAttrs = "attrs"
	// FuncMemberAttrs is the template name of [MemberAttrs].
	FuncMemberAttrs = "memberattrs"
	// FuncSupertraits is the template name of [Speller.Supertraits].
	FuncSupertraits = "supertraits"
	// FuncEnumMods is the template name of [EnumMods].
	FuncEnumMods = "enummods"
	// FuncAssocType is the template name of [Speller.AssocType].
	FuncAssocType = "assoctype"
	// FuncSumMods is the template name of [SumMods].
	FuncSumMods = "summods"
	// FuncSumPayload is the template name of [Speller.SumPayload].
	FuncSumPayload = "sumpayload"
	// FuncAliasMods is the template name of [AliasMods].
	FuncAliasMods = "aliasmods"
	// FuncConstType is the template name of [Speller.ConstType].
	FuncConstType = "consttype"
	// FuncRepr is the template name of [Speller.Repr].
	FuncRepr = "repr"
)

// Every receiver has the name selfName. A borrowed receiver has one of
// two shorthand forms, and every other receiver type follows the prefix
// of the typed form. The shorthand forms abbreviate the three receiver
// types selfType, selfRefType and selfMutType.
const (
	selfName       = "self"
	receiverRef    = "&self"
	receiverMutRef = "&mut self"
	receiverTyped  = "self: "
	selfType       = "Self"
	selfRefType    = "&Self"
	selfMutType    = "&mut Self"
)

// genericsOpener and genericsCloser enclose an argument list.
// unitSpelling is the spelling of a missing type in the shared speller,
// and discardName is the pattern of an unnamed parameter.
const (
	genericsOpener = "<"
	genericsCloser = ">"
	unitSpelling   = "()"
	discardName    = "_"
)

// pathSep separates the segments of a Rust path.
const pathSep = "::"

// docMark starts a line of an outer doc comment, and attrOpen and
// attrClose enclose an outer attribute. reprOpen and reprClose enclose the
// integer type of a repr attribute.
const (
	docMark   = "/// "
	attrOpen  = "#["
	attrClose = "]"
	reprOpen  = "repr("
	reprClose = ")"
)

// asyncKeyword is the keyword of an async function, with the space after
// it.
const asyncKeyword = "async "

// unstatedType is the refusal of a reference without a type.
const unstatedType = "a declaration states no type, and Rust spells none in its place"

// refusalPrefix is the start of every refusal of the backend. It is the
// identity of the language, as in the refusals of every backend.
const refusalPrefix = string(rust.Lang) + ": "

// builtinTypes maps each type name that Rust declares without a
// declaration in the workspace to whether the type is numeric. The
// numeric primitives convert into one another by a cast. bool, char, str
// and the String of the prelude convert through traits and methods. None
// of the names is a type parameter or a tuple struct.
var builtinTypes = map[string]bool{
	"bool": false, "char": false, "str": false, "String": false,
	"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
	"f32": true, "f64": true,
}

// reprTypes are the integer types of Rust, which a repr attribute of an
// enum takes.
var reprTypes = []string{"i8", "i16", "i32", "i64", "i128", "isize", "u8", "u16", "u32", "u64", "u128", "usize"}

// Speller writes the type references of one file. For an item of
// another module, it adds a use declaration to the import set of the
// file. The use binds the name of the item, or a numbered name where
// another use or a declaration of the file already has that name. The
// written reference then uses the bound name. The module is the module
// of the target where the target is a Rust declaration, and the module
// that the reference records where it has no target.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text that it writes and nothing else, so a
// name in its written form and an item with a bound use allocate
// nothing. The first use of an item allocates in the set. A method that
// joins more than two parts also allocates its list of parts, because Go
// places a list of at most two strings on the stack. A trailing comment
// allocates its block comment and the part that it ends. A refusal
// allocates its error.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller of the file with the import set set.
// set must not be nil. NewSpeller allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes the type reference t and adds a use of its item. It
// returns the source spelling where t does not refer to an item of
// another module. A reference with arguments gets the argument list in
// angle brackets after the name. Spell returns an error for a reference
// without a type, at any depth of its argument lists, because Rust has
// no spelling for a missing type. The unit type in its place compiles
// and changes the declaration. A structural reference keeps its written
// spelling. Spell returns an error where an item inside a structural
// reference has a use under another name, because the composite spelling
// of Rust does not follow from its structure.
//
// Spell allocates nothing for a name in its written form, an item with a
// use and a structural reference. A reference with arguments writes into
// one buffer, which is one allocation where every bound name is as long
// as its written spelling. A path inside an item with a use allocates
// the path after the bound name.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	if !complete(t) {
		return "", refuse(unstatedType)
	}
	out, err := spellref.SpellWith(t, genericsOpener, genericsCloser, unitSpelling, s.qualify)
	if err != nil {
		return "", refuse("%w", err)
	}
	return out, nil
}

// ConstType writes the type of the constant c. It returns an error for a
// constant without a type, because rustc requires the annotation and a
// formatter does not detect its absence. It returns an error for a
// constant without a value, because const X: T = ; declares nothing. It
// allocates what [Speller.Spell] allocates for the type.
func (s Speller) ConstType(c *emit.Constant) (string, error) {
	switch {
	case c.Type == nil || c.Type.Spelling == "":
		return "", refuse("a constant states its type, and %s states none", c.Name)
	case c.Value == "":
		return "", refuse("a constant takes a value, and %s states none", c.Name)
	}
	return s.Spell(c.Type)
}

// Repr writes the repr attribute of the enum e on a line of its own, such
// as #[repr(u8)], where e has an underlying type, and nothing otherwise.
// It returns an error for an underlying type that is not an integer type
// of Rust, because a repr attribute of an enum takes an integer type. It
// allocates the attribute and what [Speller.Spell] allocates, and nothing
// for an enum without an underlying type. A refusal allocates its error.
func (s Speller) Repr(e *emit.Enum) (string, error) {
	if e.Underlying == nil {
		return "", nil
	}
	spelled, err := s.Spell(e.Underlying)
	if err != nil {
		return "", err
	}
	if !slices.Contains(reprTypes, spelled) {
		return "", refuse("a repr takes an integer type, and %s states %s", e.Name, spelled)
	}
	return attrOpen + reprOpen + spelled + reprClose + attrClose + "\n", nil
}

// TypeParams writes the type parameter list ps of a type definition in
// angle brackets, and nothing for an empty list. The bounds of a
// parameter follow a colon, joined by plus signs, and its default
// follows an equals sign. A value parameter has the const form with the
// type of its value. TypeParams returns an error for a variance, because
// Rust infers variance from use and a declaration cannot declare it.
//
// Each clause of a parameter allocates one spelling. The clauses are the
// bounds, the default and the value type. TypeParams also allocates the
// joined bounds of more than one bound, the joined list with its
// brackets, and what the spelling of each type allocates. A parameter
// with two bounds and a parameter without a bound allocate four times.
func (s Speller) TypeParams(ps []*emit.TypeParam) (string, error) {
	return s.typeParams(ps, definitionPosition)
}

// ImplParams writes the type parameter list ps of an impl block. It
// writes the parameters of the type definition with their bounds and
// without their defaults, because the definition declares the defaults
// and rustc refuses a default on an impl. It allocates what
// [Speller.TypeParams] allocates for the list without its defaults.
func (s Speller) ImplParams(ps []*emit.TypeParam) (string, error) {
	return s.typeParams(ps, implPosition)
}

// FnParams writes the type parameter list ps of a function, a method or
// an associated type. It returns an error for a default, because Rust
// allows a default only on a parameter of a type definition. It
// allocates what [Speller.TypeParams] allocates.
func (s Speller) FnParams(ps []*emit.TypeParam) (string, error) {
	return s.typeParams(ps, callablePosition)
}

// AssocType writes the associated type d of a trait. The name follows
// the keyword type, and a generic type has its parameter list in angle
// brackets. A trait declares the associated type, and an implementation
// supplies it. Only a transparent alias without a target has this form,
// so AssocType returns an error for any other declaration in a trait. It
// returns an error for a visibility, because a trait item has the
// visibility of the trait. It also returns an error for a target and for
// a defined type, because an alias with a target is a default that
// stable Rust does not allow. AssocType allocates the declaration once,
// and what its parameter list allocates.
func (s Speller) AssocType(d symbol.Symbol) (string, error) {
	a, is := d.(*emit.Alias)
	if !is {
		return "", refuse("a trait nests associated types alone, and it declares a %s", d.Kind())
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
	params, err := s.FnParams(a.TypeParams)
	if err != nil {
		return "", err
	}
	return "type " + a.Name + params + ";", nil
}

// SumPayload writes the payload of the variant v. It writes nothing for
// an empty variant, named fields in braces for a struct variant, and
// bare types in parentheses for a tuple variant. The outer attributes of
// an entry precede it. It returns an error for a payload with named and
// unnamed entries, and for an entry with a fact that an inline entry
// cannot express.
//
// SumPayload allocates the spelling of each named entry, the joined
// entries of more than one entry, the payload in its delimiters, the
// attributes of an entry that has any, and what the spelling of each
// type allocates. One named entry allocates twice, one unnamed entry
// allocates once, and an empty payload allocates nothing.
func (s Speller) SumPayload(v *emit.SumVariant) (string, error) {
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
		typ, err := s.Spell(f.Type)
		if err != nil {
			return "", err
		}
		attrs := textfmt.MarkedInline(f.Annotations, attrOpen, attrClose)
		if named {
			parts = append(parts, attrs+f.Name+": "+typ)
		} else {
			parts = append(parts, attrs+typ)
		}
	}
	if named {
		return " { " + strings.Join(parts, ", ") + " }", nil
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

// Supertraits writes the supertrait bounds of the trait i after a colon,
// joined by plus signs, and nothing for a trait without supertraits. A
// trait extends other traits only through supertraits, so the coverage
// refuses an embed, and Supertraits writes none. Supertraits allocates the
// joined bounds of more than one bound, the bounds after their colon, and
// what the spelling of each bound allocates. Two supertraits allocate
// twice, one allocates once, and none allocate nothing.
func (s Speller) Supertraits(i *emit.Interface) (string, error) {
	if len(i.Extends) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(i.Extends))
	for _, t := range i.Extends {
		spelled, err := s.Spell(t)
		if err != nil {
			return "", err
		}
		parts = append(parts, spelled)
	}
	return ": " + strings.Join(parts, " + "), nil
}

// SelfParams writes the parameter list of the method m with its
// receiver. It writes the receiver of m where m has one, and the shared
// borrow &self where it has none. An associated function at type level
// gets its parameters alone, and SelfParams returns an error for its
// receiver.
//
// SelfParams allocates what [Speller.Params] allocates, and the receiver
// joined before the parameters. Two parameters allocate four times, and
// the shared borrow alone allocates nothing. A receiver in the typed form
// allocates its spelling.
func (s Speller) SelfParams(m *emit.Method) (string, error) {
	params, err := s.Params(&m.Params)
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
		if recv, err = s.receiver(m.Name, m.Receiver); err != nil {
			return "", err
		}
	}
	if params == "" {
		return recv, nil
	}
	return recv + ", " + params, nil
}

// Params writes the parameter list in the slot ps. A template passes the
// slot by address, as in {{params .Params}}, so the call reads the slot
// without a reflective method call. The outer attributes of a parameter
// precede its pattern. An unnamed parameter gets the discard pattern,
// because Rust requires a pattern. Params returns an error for a
// parameter without a type. It allocates the spelling of each parameter,
// the joined list of more than one parameter, the attributes of a
// parameter that has any, and what the spelling of each type allocates.
// Two parameters without attributes allocate three times, and an empty
// list allocates nothing.
func (s Speller) Params(ps *emit.Slot[*emit.Param]) (string, error) {
	items := ps.Items()
	parts := make([]string, 0, len(items))
	for _, p := range items {
		name := p.Name
		if name == "" {
			name = discardName
		}
		typ, err := s.Spell(p.Type)
		if err != nil {
			return "", err
		}
		parts = append(parts, textfmt.MarkedInline(p.Annotations, attrOpen, attrClose)+name+": "+typ+
			textfmt.Inline(p.Comment))
	}
	return strings.Join(parts, ", "), nil
}

// Results writes the return annotation of the results in the slot
// results. It writes nothing for no result, the type for one result, and
// a tuple for two or more, because Rust returns two or more values as a
// tuple. A template passes the slot by address, as Params describes.
// Results returns an error for a result without a type. It allocates the
// annotation, the joined tuple of more than one result, and what the
// spelling of each type allocates. One result allocates once, a tuple of
// two allocates twice, and no result allocates nothing.
func (s Speller) Results(results *emit.Slot[*emit.Return]) (string, error) {
	rs := results.Items()
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		typ, err := s.Spell(r.Type)
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

// qualify returns [Speller.use] of t in the form of [spellref.Qualify].
// It never returns an error.
func (s Speller) qualify(t *emit.TypeRef) (string, error) { return s.use(t), nil }

// use returns the spelling of the named reference t through its use. The
// first segment of a path is the item that the module declares, and the
// rest is a path inside that item.
func (s Speller) use(t *emit.TypeRef) string {
	module := spellref.PackageOf(t, rust.Lang)
	if module == "" {
		return t.Spelling
	}
	head, rest, qualified := strings.Cut(t.Spelling, pathSep)
	local := s.set.BindItem(module, head, false)
	if !qualified {
		return local
	}
	return local + pathSep + rest
}

// typeParams writes the type parameter list ps for the position at.
func (s Speller) typeParams(ps []*emit.TypeParam, at paramsPosition) (string, error) {
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
		part, err := s.typeParam(p, at == definitionPosition)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return genericsOpener + strings.Join(parts, ", ") + genericsCloser, nil
}

// typeParam writes the type parameter p. A value parameter has the const
// form with its value type, and any other parameter has its name and its
// bounds. Where defaults is true, typeParam adds the default.
func (s Speller) typeParam(p *emit.TypeParam, defaults bool) (string, error) {
	if p.Const {
		typ, err := s.Spell(p.Type)
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
			spelled, err := s.Spell(b)
			if err != nil {
				return "", err
			}
			bounds = append(bounds, spelled)
		}
		part += ": " + strings.Join(bounds, " + ")
	}
	if defaults && p.Default != nil {
		spelled, err := s.Spell(p.Default)
		if err != nil {
			return "", err
		}
		part += " = " + spelled
	}
	return part, nil
}

// receiver writes the receiver p of method from its type. It writes Self
// as self, &Self as &self and &mut Self as &mut self. It writes any
// other type, such as Box<Self> or &'a Self, in the typed form self: T.
// The trailing comment follows as a block comment. A receiver has the
// name self alone, so receiver returns an error for another name.
func (s Speller) receiver(method string, p *emit.Param) (string, error) {
	if p.Name != "" && p.Name != selfName {
		return "", refuse("a receiver binds self, and %s names its receiver %s", method, p.Name)
	}
	typ, err := s.Spell(p.Type)
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

// paramsPosition is the position of a type parameter list. Rust accepts
// a different list in each position. A type definition takes defaults,
// an impl block restates the parameters of the definition without their
// defaults, and a function, a method and an associated type take no
// defaults.
type paramsPosition uint8

// definitionPosition, implPosition and callablePosition are the three
// positions.
const (
	definitionPosition paramsPosition = iota
	implPosition
	callablePosition
)

// Funcs returns the template vocabulary that the kind templates call. The
// helpers add a use of every item of another module that they write to
// set, the import set of one file. Every helper takes a
// fixed list of arguments, because text/template allocates the list of a
// variadic call and a value for each string argument in a template.
// Funcs allocates the map of twenty-seven helpers in four allocations,
// and the twelve bound helpers of the speller.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:        Docs,
		FuncMemberDocs:  MemberDocs,
		FuncSpell:       s.Spell,
		FuncTypeParams:  s.TypeParams,
		FuncImplParams:  s.ImplParams,
		FuncFnParams:    s.FnParams,
		FuncBinder:      Binder,
		FuncParams:      s.Params,
		FuncResults:     s.Results,
		FuncVis:         Vis,
		FuncStructMods:  StructMods,
		FuncFnMods:      FnMods,
		FuncTraitFn:     TraitFn,
		FuncImplFn:      ImplFn,
		FuncTypeNames:   TypeNames,
		FuncSelfParams:  s.SelfParams,
		FuncFieldMods:   FieldMods,
		FuncAttrs:       Attrs,
		FuncMemberAttrs: MemberAttrs,
		FuncSupertraits: s.Supertraits,
		FuncEnumMods:    EnumMods,
		FuncAssocType:   s.AssocType,
		FuncSumMods:     SumMods,
		FuncSumPayload:  s.SumPayload,
		FuncAliasMods:   AliasMods,
		FuncConstType:   s.ConstType,
		FuncRepr:        s.Repr,
	}
}

// Docs writes the documentation lines of a declaration as outer doc
// comments at the top level. It allocates what [textfmt.LineDocs]
// allocates. That is the lines, sized once, and nothing for no lines.
func Docs(lines []string) string {
	return textfmt.LineDocs(lines, docMark)
}

// MemberDocs writes the documentation lines of a member as the outer doc
// comments of [Docs]. Each line starts with the member indent of a
// block. It allocates what Docs allocates.
func MemberDocs(lines []string) string {
	return textfmt.LineDocs(lines, docMark, memberIndent)
}

// TypeNames writes the names of the type parameters ps alone, in the
// form that the target of an impl block repeats. The parameter list of
// the impl declares the bounds, and the target is the type that the
// bounds apply to.
//
// TypeNames allocates the joined names and the list in its brackets. Two
// parameters allocate twice, one allocates once, and none allocate
// nothing. More than two names also allocate their list, because Go
// places a list of at most two strings on the stack.
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

// Binder writes the impl binder of the type parameters of the receiver
// type t, and nothing for a receiver without type arguments. An argument
// binds a parameter where its target is a type parameter. Every other
// argument is concrete and binds nothing. Concrete arguments are a
// declaration, an instantiation, a path, a structural form, a primitive,
// String, and a bare name without a target, such as a standard type that
// the resolution left open. Link sets the target of every reference to a
// type parameter that it resolves. A generator that builds such a
// reference by hand sets its target to the parameter. A method on Box<T>
// therefore opens impl<T> Box<T>, and a method on Wrapper<String> opens
// impl Wrapper<String>. The methods declare the bounds, because Rust code
// bounds functions and not type definitions.
//
// Binder allocates once for one bound parameter, and nothing for a
// receiver that binds none. More than one bound parameter also allocates
// the joined names, and more than two allocate their list. A refusal
// allocates its error.
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

// Vis writes the visibility v of the declaration name. It writes pub for
// a public or unknown visibility, because other code uses a generated
// API. It writes pub(crate) for internal. It writes nothing for package
// scope, because the default of Rust is private to the module. It
// returns an error for private and protected, because Rust scopes by
// module and not by type or subclass. Vis returns a constant spelling
// and allocates nothing. A refusal allocates its error.
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

// StructMods writes the keywords of the struct s, which consist of its
// visibility alone. It returns an error for an abstract struct, because
// code can construct every Rust struct. It accepts a final struct,
// because Rust has no subclasses. It returns an error for supertypes,
// because a struct does not inherit. A struct does not promote either, so
// the coverage refuses an embed. StructMods allocates nothing. A refusal
// allocates its error.
func StructMods(s *emit.Struct) (string, error) {
	switch {
	case s.Abstract:
		return "", refuse("every struct can be made, and %s states abstract", s.Name)
	case len(s.Extends) > 0 || len(s.Implements) > 0:
		return "", refuse("a struct does not inherit, and %s states supertypes", s.Name)
	}
	return Vis(s.Visibility, s.Name)
}

// EnumMods writes the keywords of the enum e, which consist of its
// visibility alone. It returns an error for an enum with fields or
// methods, because Rust puts state in variants and behaviour in impl
// blocks. EnumMods allocates nothing. A refusal allocates its error.
func EnumMods(e *emit.Enum) (string, error) {
	if e.Fields.Len() > 0 || e.Methods.Len() > 0 {
		return "", refuse("an enum has variants alone, and %s states members", e.Name)
	}
	return Vis(e.Visibility, e.Name)
}

// SumMods writes the keywords of the data enum s, which consist of its
// visibility alone. It returns an error for a sum with methods, because
// Rust puts behaviour in impl blocks. SumMods allocates nothing. A
// refusal allocates its error.
func SumMods(s *emit.Sum) (string, error) {
	if s.Methods.Len() > 0 {
		return "", refuse("a data enum has variants alone, and %s states methods", s.Name)
	}
	return Vis(s.Visibility, s.Name)
}

// AliasMods writes the keywords of the type alias a, which consist of its
// visibility alone. It returns an error for a defined type, because a
// Rust alias is transparent. AliasMods allocates nothing. A refusal
// allocates its error.
func AliasMods(a *emit.Alias) (string, error) {
	if a.Defined {
		return "", refuse("an alias is transparent, and %s states a defined type", a.Name)
	}
	return Vis(a.Visibility, a.Name)
}

// FnMods writes the visibility of the free function f, followed by async
// for an async function. FnMods allocates once for an async function and
// nothing otherwise. A refusal allocates its error.
func FnMods(f *emit.Function) (string, error) {
	part, err := Vis(f.Visibility, f.Name)
	if err != nil {
		return "", err
	}
	if f.Async {
		part += asyncKeyword
	}
	return part, nil
}

// TraitFn writes the keywords of the trait method m. It writes async for
// an async method and nothing otherwise. A trait item has the visibility
// of the trait, so TraitFn returns an error for a visibility. It accepts
// an abstract method, because a trait method without a default is a
// signature. It returns an error for a body on such a method, because a
// signature has no body. It returns an error for final, because every
// implementation can override a trait method, and for override, because
// a trait method overrides nothing. TraitFn allocates nothing. A refusal
// allocates its error.
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
		return asyncKeyword, nil
	}
	return "", nil
}

// ImplFn writes the visibility of the impl method m, followed by async
// for an async method. It returns an error for abstract and for a
// default, because an impl method has its body. It returns an error for
// override, because an inherent method overrides nothing. It accepts
// final, because nothing overrides an inherent method. ImplFn allocates
// once for an async method and nothing otherwise. A refusal allocates
// its error.
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
		part += asyncKeyword
	}
	return part, nil
}

// FieldMods writes the keywords of the field f, which consist of its
// visibility alone. It returns an error for a type-level field, because
// Rust declares statics outside types. It returns an error for an
// immutable field, because the binding of the struct value decides
// mutability. It returns an error for an initializer, because a struct
// does not declare field defaults. FieldMods allocates nothing. A
// refusal allocates its error.
func FieldMods(f *emit.Field) (string, error) {
	switch {
	case f.Level == symbol.LevelType:
		return "", refuse("a struct has no statics, and %s states type level", f.Name)
	case f.Mutability == symbol.MutabilityImmutable:
		return "", refuse("a field's mutability follows the binding of its struct value, and %s "+
			"states its own", f.Name)
	case f.Value != "":
		return "", refuse("a struct declares no field defaults, and %s states one", f.Name)
	}
	return Vis(f.Visibility, f.Name)
}

// Attrs writes one attribute line at the top level for each annotation
// of a. A line has the name in an outer attribute, and the arguments
// verbatim in parentheses where the annotation has any. It allocates
// what [textfmt.Marked] allocates. That is the lines, sized once, and
// nothing for no annotations.
func Attrs(a symbol.Annotations) string {
	return textfmt.Marked(a, attrOpen, attrClose)
}

// MemberAttrs writes the attribute lines of a member as [Attrs] writes
// them. Each line starts with the member indent of a block. It allocates
// what Attrs allocates.
func MemberAttrs(a symbol.Annotations) string {
	return textfmt.Marked(a, attrOpen, attrClose, memberIndent)
}

// complete reports whether the reference t and every reference in its
// argument lists, at any depth, have a type.
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

// typeParameter reports whether the target of the receiver argument a is
// a type parameter.
func typeParameter(a *emit.TypeRef) bool {
	return a.Target.Kind == symbol.KindTypeParam
}

// bareName returns the spelling of t where t is a bare name that the
// resolution left open. It returns nothing for a declaration, an
// instantiation or a structural form.
func bareName(t *emit.TypeRef) string {
	if t == nil || !t.Target.IsZero() || t.Form != symbol.FormNamed || len(t.Args) > 0 {
		return ""
	}
	return t.Spelling
}

// inlineEntry returns an error for a fact of the payload entry f that an
// inline entry cannot express. An inline entry consists of its attributes,
// a name and a type alone, on the line of its variant.
func inlineEntry(variant string, f *emit.Field) error {
	switch {
	case len(f.Doc) > 0 || f.Comment != "":
		return refuse("a payload entry spells inline, and one in %s states "+
			"documentation or a comment", variant)
	case f.Visibility != symbol.VisibilityUnknown:
		return refuse("a payload follows its enum's visibility, and an entry in "+
			"%s states its own", variant)
	case f.Level == symbol.LevelType:
		return refuse("an enum has no statics, and a payload entry in %s "+
			"states type level", variant)
	case f.Mutability == symbol.MutabilityImmutable:
		return refuse("a payload's mutability follows the binding of its enum value, and "+
			"an entry in %s states its own", variant)
	case f.Value != "":
		return refuse("a payload declares no defaults, and an entry in %s "+
			"states one", variant)
	}
	return nil
}

// refuse returns a refusal that starts with [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
