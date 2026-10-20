// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
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
	// FuncSpellValue is the template name of [Speller.SpellValue].
	FuncSpellValue = render.HelperSpellValue
	// FuncTypeParams is the template name of [Speller.TypeParams].
	FuncTypeParams = render.HelperTypeParams
	// FuncTypeArgs is the template name of [Speller.TypeArgs].
	FuncTypeArgs = render.HelperTypeArgs
	// FuncParams is the template name of [Speller.Params].
	FuncParams = render.HelperParams
	// FuncResults is the template name of [Speller.Results].
	FuncResults = render.HelperResults
	// FuncReturns is the template name of [Speller.Returns].
	FuncReturns = "returns"
	// FuncMods is the template name of [Mods].
	FuncMods = "mods"
	// FuncMemberMods is the template name of [MemberMods].
	FuncMemberMods = "membermods"
	// FuncIndexMods is the template name of [IndexMods].
	FuncIndexMods = "indexmods"
	// FuncCtorMods is the template name of [CtorMods].
	FuncCtorMods = "ctormods"
	// FuncPropMods is the template name of [PropMods].
	FuncPropMods = "propmods"
	// FuncSigMods is the template name of [SigMods].
	FuncSigMods = "sigmods"
	// FuncBinding is the template name of [Binding].
	FuncBinding = "binding"
	// FuncDecorators is the template name of [Decorators].
	FuncDecorators = "decorators"
	// FuncMemberDecorators is the template name of [MemberDecorators].
	FuncMemberDecorators = "memberdecorators"
	// FuncHeritage is the template name of [Speller.Heritage].
	FuncHeritage = "heritage"
	// FuncAccessor is the template name of [AccessorKw].
	FuncAccessor = "accessor"
	// FuncHard is the template name of [Hard].
	FuncHard = "hard"
	// FuncIndexSig is the template name of [Speller.IndexSig].
	FuncIndexSig = "indexsig"
	// FuncMethodKey is the template name of [MethodKey].
	FuncMethodKey = "methodkey"
	// FuncPropKey is the template name of [PropKey].
	FuncPropKey = "propkey"
	// FuncEnumKey is the template name of [EnumKey].
	FuncEnumKey = "enumkey"
)

// Anonymous is the type that the backend writes where a declaration has
// no type. Every value is assignable to it, and TypeScript checks every
// use of a value of this type.
const Anonymous = typescript.TypeUnknown

// argsOpener and argsCloser enclose an argument list in TypeScript, and
// qualifierSep separates the parts of a qualified name.
const (
	argsOpener   = "<"
	argsCloser   = ">"
	qualifierSep = "."
)

// groupOpen and groupClose enclose the element of a list where
// [typescript.Grouped] reports true for it. listMark follows the element
// of a list.
const (
	groupOpen  = "("
	groupClose = ")"
	listMark   = "[]"
)

// constBinding and letBinding are the keywords that open a module-level
// binding.
const (
	constBinding = "const"
	letBinding   = "let"
)

// staticKeyword and readonlyKeyword are modifiers of a class member, each
// with the space after it. TypeScript writes them before the name of the
// member.
const (
	staticKeyword   = "static "
	readonlyKeyword = "readonly "
)

// hardMark starts the name of a hard-private member.
const hardMark = "#"

// annotationSep, promiseOpen and promiseClose are the parts of the return
// annotation of an async callable.
const (
	annotationSep = ": "
	promiseOpen   = typescript.TypePromise + argsOpener
	promiseClose  = argsCloser
)

// These are the parts of a relative import specifier. currentDir starts
// the specifier of a module in the directory of the rendered file or
// below it. parentDir moves up one directory, and pathSep separates the
// segments of a module path.
const (
	currentDir = "./"
	parentDir  = "../"
	pathSep    = "/"
)

// discardName is the name of an unnamed parameter. The second unnamed
// parameter and every later one get a number after it.
const discardName = "_"

// refusalPrefix is the start of every refusal of the backend. It is the
// identity of the language, as in the refusals of every backend.
const refusalPrefix = string(typescript.Lang) + ": "

// Speller writes the type references of one file. For a reference to a
// declaration of another module, it imports the declaration through the
// import set of the file. The import uses the name of the declaration,
// or a numbered name where another import or a declaration of the file
// already uses that name. The written reference then uses the bound
// name. The import specifier follows these rules:
//
//   - A reference to a TypeScript declaration imports the module path of
//     the declaration relative to the path of the file, such as ./store
//     or ../api/user. Where the source has a bare specifier, such as react
//     for an ambient module, the reference imports that specifier as
//     written.
//   - A translated reference imports the module path that the layout
//     recorded, relative to the path of the file.
//   - A reference without a target imports the specifier of its source.
//
// A reference to a declaration of the module of the file imports nothing.
// A reference to a declaration of a library of TypeScript imports nothing
// either, because a library declares globals.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text that it writes and nothing else, so a
// name in its written form allocates nothing. A reference that imports a
// module path allocates its relative specifier. The first import of a
// declaration allocates in the set. A method that joins more than two
// parts also allocates its list of parts, because Go places a list of at
// most two strings on the stack. A trailing comment allocates its block
// comment and the part that it ends. A refusal allocates its error.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller of the file with the import set set.
// set must not be nil. NewSpeller allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes the type reference t in a type position. It imports the
// declaration of t for the type checker alone. Spell returns the source
// spelling where t does not refer to a declaration of another module,
// and [Anonymous] for a nil t. A reference with arguments has its bare
// name in Spelling, and Spell writes the argument list in angle
// brackets. A structural reference keeps its written spelling. Spell
// returns an error where a type inside a structural reference imports
// under another name, because the composite spelling of TypeScript does
// not follow from its structure. Spell also returns an error where a
// declaration of the file hides the global type that t names, because the
// spelling would refer to that declaration. Every declaration of a
// library of TypeScript is global. A reference without a module names a
// global type where [typescript.GlobalType] reports true for its name.
//
// Spell allocates nothing for a name in its written form, a nil
// reference and a structural reference. An imported declaration
// allocates its relative specifier where it imports a module path. A
// reference with arguments writes into one buffer, which is one
// allocation where every bound name is as long as its written spelling.
// A member of an imported declaration allocates the member after the
// bound name.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	return s.spell(t, true)
}

// SpellValue writes the reference t in a value position, where the
// declaration must exist at run time, such as the class of a construction
// or the base of a class. It imports the declaration of t for its value,
// so tsc reads new Address() without the refusal of a type-only import.
// The type arguments of t spell in a type position, as [Speller.Spell]
// writes them, so a construction of Map<string, Address> imports Address
// for the type checker alone. SpellValue returns the errors of
// [Speller.Spell], and allocates what Spell allocates for the reference
// and for each type argument, and the joined arguments.
func (s Speller) SpellValue(t *emit.TypeRef) (string, error) {
	if t == nil || t.Form != symbol.FormNamed || len(t.Args) == 0 {
		return s.spell(t, false)
	}
	head := *t
	head.Args = nil
	name, err := s.spell(&head, false)
	if err != nil {
		return "", err
	}
	args, err := s.all(t.Args, s.Spell)
	if err != nil {
		return "", err
	}
	return name + argsOpener + strings.Join(args, ", ") + argsCloser, nil
}

// IndexSig writes the whole index signature of m. The one key parameter
// is in brackets, and the element type follows the colon. IndexSig
// returns an error for a method that an index signature cannot express:
// a method with more parameters, without a result, with type parameters
// or with an accessor. It allocates the signature once, and what the
// spellings of the key and the element allocate.
func (s Speller) IndexSig(m *emit.Method) (string, error) {
	ps, rs := m.Params.Items(), m.Returns.Items()
	switch {
	case len(ps) != 1 || ps[0].Name == "" || ps[0].Type == nil:
		return "", refuse("an index signature takes one named, typed key, and %s does not", m.Name)
	case len(rs) != 1:
		return "", refuse("an index signature states one element type, and %s does not", m.Name)
	case len(m.TypeParams) != 0 || m.Accessor != symbol.AccessorNone:
		return "", refuse("an index signature admits no type parameters and no accessor, "+
			"and %s states one", m.Name)
	}
	key, err := s.Spell(ps[0].Type)
	if err != nil {
		return "", err
	}
	elem, err := s.Spell(rs[0].Type)
	if err != nil {
		return "", err
	}
	return "[" + ps[0].Name + ": " + key + "]: " + elem + ";", nil
}

// TypeParams writes the type parameter list ps in angle brackets, and
// nothing for an empty list. Each parameter has its declared variance
// before the name, its bounds as an intersection after extends, and its
// default after an equals sign. TypeParams returns an error for a value
// parameter. Const in the model takes a value argument, and the const
// modifier of TypeScript applies to a type argument.
//
// Each clause of a parameter allocates one spelling. The clauses are the
// variance with the name, the bounds and the default. TypeParams also
// allocates the joined bounds of more than one bound, the joined list
// with its brackets, and what the spelling of each type allocates. A
// parameter with two bounds and a parameter without a bound allocate
// four times.
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

// TypeArgs writes a list of type parameters or of type references as the
// type arguments of a reference in angle brackets, and nothing for an
// empty list: the parameters of a generic declaration as the arguments of
// a reference to it, as in PageBuilder<T>, and the witnesses of a test, as
// in <number>. Each reference spells through [Speller.Spell], which
// imports its declaration for the type checker. TypeArgs returns an error
// for a list of another type. It allocates what [spellref.TypeArgs]
// allocates. A template passes the list as a value of type any, which
// text/template boxes, one allocation.
func (s Speller) TypeArgs(list any) (string, error) {
	return spellref.TypeArgs(list, argsOpener, argsCloser, s.Spell)
}

// Heritage writes the heritage clauses of the type d. A class has one
// base after extends and its contracts after implements. An interface
// has the interfaces that it extends after extends. The base of a class
// is a value at run time, so Heritage writes it through
// [Speller.SpellValue]. It imports every contract for the type checker
// alone. Heritage returns an error
// for a second base of a class, because a TypeScript class extends one
// class. TypeScript does not promote members, so the coverage refuses an
// embed, and Heritage writes none.
//
// Heritage allocates each clause with its keyword, the joined contracts
// of more than one contract, and what the spelling of each type
// allocates. A base alone allocates once, a base with two contracts
// allocates three times, and a type without heritage allocates nothing.
func (s Speller) Heritage(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		var part string
		switch len(t.Extends) {
		case 0:
		case 1:
			base, err := s.SpellValue(t.Extends[0])
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

// Params writes the parameter list in the slot ps with the rest marker,
// and with each default after an equals sign, verbatim from the model. A
// template passes the slot by address, as in {{params .Params}}, so the
// call reads the slot without a reflective method call. Params returns
// an error for a rest parameter with a default, because TypeScript does
// not initialize a rest parameter. An unnamed parameter gets the discard
// name, with a number from the second unnamed parameter on.
//
// The type of a rest parameter is the type of one argument, and Params
// writes a list of that element. The element is in parentheses where
// [typescript.Grouped] reports true for it. A rest parameter of the union
// string|number reads ...parts: (string|number)[].
//
// Params allocates the spelling of each parameter, the joined list of
// more than one parameter, and what the spelling of each type allocates.
// Two parameters allocate three times, and an empty list allocates
// nothing. A numbered discard name, an optional marker and a default
// each allocate once more.
func (s Speller) Params(ps *emit.Slot[*emit.Param]) (string, error) {
	items := ps.Items()
	parts := make([]string, 0, len(items))
	unnamed := 0
	for _, p := range items {
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
			open, closer := "", ""
			if typescript.Grouped(symbol.FormList, p.Type) {
				open, closer = groupOpen, groupClose
			}
			parts = append(parts, "..."+name+": "+open+typ+closer+listMark+textfmt.Inline(p.Comment))
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

// Returns writes the return annotation of the callable d. It writes the
// annotation of [Speller.Results], inside Promise for an async callable,
// because an async function returns a promise of its result. It writes
// nothing for a setter, because a TypeScript setter cannot have a return
// annotation. It returns an error for an async callable in a file with a
// declaration that hides the global Promise. It allocates what
// [Speller.Results] allocates, and nothing for a setter or an async
// callable without results.
func (s Speller) Returns(d symbol.Symbol) (string, error) {
	var none emit.Slot[*emit.Return]
	results, async := &none, false
	switch c := d.(type) {
	case *emit.Function:
		results, async = &c.Returns, c.Async
	case *emit.Method:
		if c.Accessor == symbol.AccessorSet {
			return "", nil
		}
		results, async = &c.Returns, c.Async
	}
	switch {
	case !async:
		return s.Results(results)
	case s.set.Reserved(typescript.TypePromise):
		return "", refuse("a declaration of the file hides the global type %s", typescript.TypePromise)
	case results.Len() == 0:
		return annotationSep + promiseOpen + typescript.TypeVoid + promiseClose, nil
	}
	return s.resultType(results.Items(), annotationSep+promiseOpen, promiseClose)
}

// Results writes the return type annotation of the results in the slot
// results. It writes void for no result, the type for one result, and a
// tuple for two or more, because a TypeScript function returns one
// value. A template passes the slot by address, as Params describes. It
// allocates the annotation, the joined tuple of more than one result,
// and what the spelling of each type allocates. One result allocates
// once, a tuple of two allocates twice, and no result allocates nothing.
func (s Speller) Results(results *emit.Slot[*emit.Return]) (string, error) {
	if results.Len() == 0 {
		return annotationSep + typescript.TypeVoid, nil
	}
	return s.resultType(results.Items(), annotationSep, "")
}

// resultType writes the type of one or more results between open and
// closer. It writes the type of one result, and a tuple for two or more.
// It joins the text in one concatenation, so one result allocates once.
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

// spell writes the reference t. Where typeOnly is true, it imports the
// declaration of t for the type checker alone. A composite whose child
// imports under another name, and a composite that the rules substituted,
// restate through [typescript.Restate].
func (s Speller) spell(t *emit.TypeRef, typeOnly bool) (string, error) {
	out, err := spellref.SpellRestated(t, argsOpener, argsCloser, Anonymous, s.qualify(typeOnly), typescript.Restate)
	if err != nil {
		return "", refuse("%w", err)
	}
	return out, nil
}

// qualify returns a function that writes a named reference through its
// import. The first segment of a qualified spelling is the declaration
// that the module exports, and the rest is a member of it. A reference
// without a module, a reference into a library of TypeScript, whose
// declarations are global, and a reference into the module of the file
// import nothing. The function returns an error where a declaration of
// the file hides the global that the first segment names. Every
// declaration of a library is such a global. A reference without a module
// names one where [typescript.GlobalType] reports true for the segment.
func (s Speller) qualify(typeOnly bool) spellref.Qualify {
	return func(t *emit.TypeRef) (string, error) {
		head, rest, qualified := strings.Cut(t.Spelling, qualifierSep)
		module := spellref.PackageOf(t, typescript.Lang)
		library := typescript.Library(module)
		switch {
		case (library || module == "" && typescript.GlobalType(head)) && s.set.Reserved(head):
			return "", refuse("a declaration of the file hides the global type %s", head)
		case library || module == "" || module == s.set.Home():
			return t.Spelling, nil
		}
		local := s.set.BindItem(s.specifier(t, module), head, typeOnly)
		if !qualified {
			return local, nil
		}
		return local + qualifierSep + rest, nil
	}
}

// specifier returns the import specifier of the module of the named
// reference t, by the rules of [Speller]. module is the module path of
// t, or the specifier of its source where t has no target.
func (s Speller) specifier(t *emit.TypeRef, module string) string {
	if t.Target.IsZero() ||
		t.Target.Lang == typescript.Lang && t.Package != "" && kindOf(t.Package) == packageSpecifier {

		return t.Package
	}
	return relativeTo(s.set.Home(), module)
}

// all writes each reference of ts through spell, in order.
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

// Funcs returns the template vocabulary that the kind templates call. The
// helpers import every declaration of another module that they write into
// set, the import set of one file. Every helper takes
// a fixed list of arguments, because text/template allocates the list of
// a variadic call and a value for each string argument in a template.
// Funcs allocates the map of twenty-six helpers in four allocations,
// and the nine bound helpers of the speller.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:             Docs,
		FuncMemberDocs:       MemberDocs,
		FuncWrapDocs:         WrapDocs,
		FuncSpell:            s.Spell,
		FuncSpellValue:       s.SpellValue,
		FuncTypeParams:       s.TypeParams,
		FuncTypeArgs:         s.TypeArgs,
		FuncParams:           s.Params,
		FuncResults:          s.Results,
		FuncReturns:          s.Returns,
		FuncMods:             Mods,
		FuncMemberMods:       MemberMods,
		FuncIndexMods:        IndexMods,
		FuncCtorMods:         CtorMods,
		FuncPropMods:         PropMods,
		FuncSigMods:          SigMods,
		FuncBinding:          Binding,
		FuncDecorators:       Decorators,
		FuncMemberDecorators: MemberDecorators,
		FuncHeritage:         s.Heritage,
		FuncAccessor:         AccessorKw,
		FuncHard:             Hard,
		FuncIndexSig:         s.IndexSig,
		FuncPropKey:          PropKey,
		FuncMethodKey:        MethodKey,
		FuncEnumKey:          EnumKey,
	}
}

// Docs writes the documentation lines of a declaration as a TSDoc block
// at the top level. It allocates what [textfmt.BlockDocs] allocates. That
// is one block, sized once, and nothing for no lines.
func Docs(lines []string) string {
	return textfmt.BlockDocs(lines, "/**", " * ", " */")
}

// MemberDocs writes the documentation lines of a member as the TSDoc
// block of [Docs]. Each line starts with the member indent of a class,
// an interface or an enum. It allocates what Docs allocates.
func MemberDocs(lines []string) string {
	return textfmt.BlockDocs(lines, "/**", " * ", " */", memberIndent)
}

// WrapDocs wraps text at [textfmt.DocsWidth] into the lines that [Docs]
// and [MemberDocs] write. A TSDoc block tag is the first element of its
// line, and the text of a comment may contain Markdown, so the wrap starts
// no line with a tag or with a block opener of CommonMark. It allocates
// what [textfmt.Wrap] allocates.
func WrapDocs(text string) []string {
	return textfmt.Wrap(text, textfmt.DocsWidth, textfmt.MarkupTags|textfmt.MarkupMarkdown)
}

// PropKey writes the property key of the field f, as [MethodKey] writes
// the key of a method. A quoted key is a name such as the wire name
// content-type or a key that starts with a digit, and the type then
// declares the whole name. It allocates once for a quoted key and nothing
// otherwise. A refusal allocates its error.
func PropKey(f *emit.Field) (string, error) {
	return memberKey(f.Name, f.Hard, f.Computed)
}

// MethodKey writes the member key of the method m. It writes an
// identifier as it is, a computed name as written, and any other name in
// single quotes. TypeScript allows a quoted member key on classes and on
// interfaces. MethodKey returns an error for a hard-private name that is
// not an identifier after its #, because # takes no quoted form. It
// allocates once for a quoted key and nothing otherwise. A refusal
// allocates its error.
func MethodKey(m *emit.Method) (string, error) {
	return memberKey(m.Name, m.Hard, m.Computed)
}

// EnumKey writes the key of the enum member v. It writes an identifier
// as it is and any other name in single quotes. TypeScript allows a
// quoted key on an enum member. EnumKey allocates once for a quoted key
// and nothing for an identifier.
func EnumKey(v *emit.EnumVariant) string {
	if spell.IsIdentifier(v.Name) {
		return v.Name
	}
	return quote(v.Name)
}

// AccessorKw writes the accessor keyword of the method m. It returns an
// error for an accessor of another shape. A getter takes no parameter
// and returns one value. A setter takes one value and returns nothing.
// Neither declares type parameters or a ?, because a TypeScript accessor
// cannot have them. AccessorKw returns a constant keyword and allocates
// nothing. A refusal allocates its error.
func AccessorKw(m *emit.Method) (string, error) {
	if m.Optional && m.Accessor != symbol.AccessorNone {
		return "", refuse("an accessor admits no ?, and %s states one", m.Name)
	}
	switch m.Accessor {
	case symbol.AccessorNone:
		return "", nil
	case symbol.AccessorGet:
		switch {
		case m.Params.Len() != 0:
			return "", refuse("a getter takes nothing, and %s takes parameters", m.Name)
		case m.Returns.Len() == 0:
			return "", refuse("a getter returns its property, and %s returns nothing", m.Name)
		case len(m.TypeParams) != 0:
			return "", refuse("an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "get ", nil
	default:
		switch {
		case m.Params.Len() != 1:
			return "", refuse("a setter takes its one value, and %s does not", m.Name)
		case m.Returns.Len() != 0:
			return "", refuse("a setter returns nothing, and %s returns", m.Name)
		case len(m.TypeParams) != 0:
			return "", refuse("an accessor admits no type parameters, and %s declares some", m.Name)
		}
		return "set ", nil
	}
}

// Hard writes the # of the hard-private member s where the name of s does
// not start with one, and nothing for a name that does. A hard-private
// name makes the member private at run time. Hard accepts a private
// visibility beside it and returns an error for any other visibility.
// Hard allocates nothing. A refusal allocates its error.
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
	case vis != symbol.VisibilityUnknown && vis != symbol.VisibilityPrivate:
		return "", refuse("a hard-private name is private, and %s states another visibility beside it", name)
	case strings.HasPrefix(name, hardMark):
		return "", nil
	}
	return hardMark, nil
}

// Mods writes the leading keywords of the module-level declaration d. It
// writes export for a public or unknown visibility and nothing for
// package scope. It adds abstract for an abstract class and async for an
// async function. Mods returns an error for these declarations:
//
//   - a protected, private or internal declaration, because a
//     module-level declaration cannot have that visibility;
//   - a final class, because TypeScript does not seal a class;
//   - a constant without a value, because const X = declares nothing;
//   - an annotation list on a declaration that is not a class, because
//     TypeScript decorates only classes and their members.
//
// Mods allocates once for the keywords of an abstract class or an async
// function, and nothing for every other declaration. A refusal allocates
// its error.
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

// MemberMods writes the leading keywords of the class member d in the
// order of the TypeScript grammar. The order is accessibility, static,
// abstract, override, async on a method and readonly on a field. A
// hard-private member takes no accessibility, because TypeScript allows
// none beside a # name. TypeScript allows async only on a method with a
// body that is not an accessor. An async abstract method and an async
// accessor express their asynchrony in the promise of their annotation.
// MemberMods returns an error for these members:
//
//   - a final method, because TypeScript has no final methods;
//   - a method with a default, because TypeScript has no default
//     methods;
//   - an abstract method with a body, because an abstract method is a
//     signature;
//   - a member with package or internal accessibility, because class
//     members do not take one.
//
// MemberMods allocates one join for each keyword after the first, and
// nothing for a member with one keyword or none. A refusal allocates its
// error.
func MemberMods(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Field:
		part, err := accessibility(t.Visibility, t.Hard, t.Name)
		if err != nil {
			return "", err
		}
		if t.Level == symbol.LevelType {
			part += staticKeyword
		}
		if t.Mutability == symbol.MutabilityImmutable {
			part += readonlyKeyword
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
		part, err := accessibility(t.Visibility, t.Hard, t.Name)
		if err != nil {
			return "", err
		}
		if t.Level == symbol.LevelType {
			part += staticKeyword
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

// IndexMods writes the keywords of the class index signature m: static
// for a type-level signature, and readonly for an immutable one. An index
// signature is a declaration without a body, so IndexMods returns an
// error for a body. It also returns an error for an accessibility,
// abstract, override, final, a default, async, a hard-private name, a ?,
// a throws clause and decorators, because TypeScript has none of them on
// an index signature. IndexMods allocates the joined keywords of a static
// readonly signature, and nothing for every other signature. A refusal
// allocates its error.
func IndexMods(m *emit.Method) (string, error) {
	switch {
	case !m.Body.IsZero():
		return "", refuse("an index signature is a declaration without a body, and %s states one",
			m.Name)
	case m.Visibility != symbol.VisibilityUnknown && m.Visibility != symbol.VisibilityPublic:
		return "", refuse("an index signature takes no accessibility, and %s states one", m.Name)
	case m.Abstract || m.Override || m.Final || m.HasDefault || m.Async || m.Hard || m.Optional:
		return "", refuse("an index signature takes static and readonly alone, and %s states another "+
			"modifier", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	case len(m.Annotations) > 0:
		return "", refuse("decorators mark no index signature, and %s states annotations", m.Name)
	}
	part := ""
	if m.Level == symbol.LevelType {
		part = staticKeyword
	}
	if m.Mutability == symbol.MutabilityImmutable {
		part += readonlyKeyword
	}
	return part, nil
}

// CtorMods writes the keywords of the class constructor m. The keywords
// consist of its accessibility alone. A constructor is an instance
// member without type parameters, so CtorMods returns an error for
// static and for type parameters. It also returns an error for abstract,
// override, final, a default, async, an accessor, a hard-private name, a
// ?, a throws clause and decorators, because TypeScript has none of them
// on a constructor. CtorMods allocates nothing. A refusal allocates its
// error.
func CtorMods(m *emit.Method) (string, error) {
	switch {
	case m.Level == symbol.LevelType:
		return "", refuse("a constructor belongs to instances, and %s states type level", m.Name)
	case len(m.TypeParams) > 0:
		return "", refuse("a constructor takes no type parameters, and %s states %d",
			m.Name, len(m.TypeParams))
	case m.Abstract || m.Override || m.Final || m.HasDefault || m.Async ||
		m.Hard || m.Optional || m.Accessor != symbol.AccessorNone:
		return "", refuse("a constructor takes an accessibility alone, and %s states another "+
			"modifier", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	case len(m.Annotations) > 0:
		return "", refuse("decorators mark no constructor, and %s states annotations", m.Name)
	}
	return accessibility(m.Visibility, false, m.Name)
}

// PropMods writes the keywords of the interface property f. It writes
// readonly for an immutable property and nothing otherwise. An interface
// member does not have an accessibility, a static level or a decorator,
// so PropMods returns an error for each of them. PropMods allocates
// nothing. A refusal allocates its error.
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
		return readonlyKeyword, nil
	}
	return "", nil
}

// SigMods checks the interface member signature m. It writes readonly for
// an immutable index signature and nothing for every other signature. A
// signature takes no other keyword and no body, so SigMods returns an
// error for another modifier or a body. An async method signature
// expresses its asynchrony in the promise of its annotation. SigMods
// returns an error for an async index or construct signature, because
// neither returns a promise. It also returns an error for an index or
// construct signature with a ?, because neither can be absent from a
// value. SigMods allocates nothing. A refusal allocates its error.
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
	case m.Optional && (m.Indexer || m.Constructs):
		return "", refuse("an index or construct signature admits no ?, and %s states one", m.Name)
	case len(m.Throws) > 0:
		return "", unthrown(m.Name)
	}
	if m.Indexer && m.Mutability == symbol.MutabilityImmutable {
		return readonlyKeyword, nil
	}
	return "", nil
}

// Binding writes the keyword of the module-level binding v. It writes
// const for an immutable binding and let otherwise. It returns an error
// for an immutable binding without an initializer, because TypeScript
// requires the initializer of a const in its declaration. Binding
// returns a constant keyword and allocates nothing. A refusal allocates
// its error.
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

// Decorators writes one decorator line at the top level for each
// annotation of a. A line has the name after the marker, and the
// arguments verbatim in parentheses where the annotation has any. It
// returns an error for an assigned annotation, because a decorator passes
// its arguments in parentheses alone. It allocates what [textfmt.Marked]
// allocates. That is the lines, sized once, and nothing for no
// annotations.
func Decorators(a symbol.Annotations) (string, error) {
	if err := unassigned(a); err != nil {
		return "", err
	}
	return textfmt.Marked(a, "@", ""), nil
}

// MemberDecorators writes the decorator lines of a member as
// [Decorators] writes them. Each line starts with the member indent of a
// class. It returns the error of Decorators, and allocates what
// Decorators allocates.
func MemberDecorators(a symbol.Annotations) (string, error) {
	if err := unassigned(a); err != nil {
		return "", err
	}
	return textfmt.Marked(a, "@", "", memberIndent), nil
}

// unassigned returns an error for the first assigned annotation of as,
// because a decorator passes its arguments in parentheses alone, and nil
// where no annotation is assigned. It allocates nothing, and an error
// allocates its message.
func unassigned(as symbol.Annotations) error {
	for _, a := range as {
		if a.Assigned {
			return refuse("a decorator passes its arguments in parentheses, and %s assigns its argument", a.Name)
		}
	}
	return nil
}

// variance returns the declaration-site keyword of the variance v, which
// TypeScript writes before the name of a type parameter. It returns in
// for contravariance, out for covariance and nothing for invariance.
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

// exported writes the module-level visibility v. It writes export for a
// public or unknown visibility, and nothing for package scope, because a
// TypeScript module is its own package. The other visibilities have no
// module-level form, so exported returns an error that names name.
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

// accessibility writes the accessibility v of a class member. It writes
// nothing for a public or unknown visibility, and the keyword for
// private and protected. It writes nothing for a hard-private member,
// because [Hard] checks its visibility. Package and internal visibility
// have no member form, so accessibility returns an error that names name.
func accessibility(v symbol.Visibility, hard bool, name string) (string, error) {
	if hard {
		return "", nil
	}
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

// relativeTo returns the relative specifier of the module path module
// from the directory of the module path home. The specifier starts with
// ./ where the module is in that directory or below it. Otherwise it
// starts with one ../ for each directory that the path goes up. It
// allocates the specifier.
func relativeTo(home, module string) string {
	dir, _, nested := strings.CutLast(home, pathSep)
	if !nested {
		dir = ""
	}
	// shared is the length of the leading directories that dir and module
	// have in common, with the separator after them.
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

// memberKey writes the key of a member named name, as [MethodKey] states.
// hard and computed are the facts of the member. The name of a
// hard-private member must be an identifier after its #.
func memberKey(name string, hard, computed bool) (string, error) {
	switch {
	case hard && !spell.IsIdentifier(strings.TrimPrefix(name, hardMark)):
		return "", refuse("a hard-private name admits no quoted form, and %s is not an identifier", name)
	case hard, computed, spell.IsIdentifier(name):
		return name, nil
	}
	return quote(name), nil
}

// unthrown returns the refusal of declared failure types for the
// declaration name, because a TypeScript signature cannot declare them.
func unthrown(name string) error {
	return refuse("a signature declares no failure types, and %s states throws", name)
}

// undecorated returns the refusal of an annotation list on the
// declaration name, which a decorator cannot mark.
func undecorated(name string) error {
	return refuse("decorators mark classes and their members, and %s states annotations "+
		"elsewhere", name)
}

// refuse returns a refusal that starts with [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
