// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"
	"text/template"

	golang "go.dokimi.dev/eidos/lang/go"
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
	// FuncParams is the template name of [Speller.Params].
	FuncParams = render.HelperParams
	// FuncResults is the template name of [Speller.Results].
	FuncResults = render.HelperResults
	// FuncReceiver is the template name of [Speller.Receiver].
	FuncReceiver = "receiver"
	// FuncPackage is the template name of [Package].
	FuncPackage = "package"
	// FuncGuard is the template name of [Guard].
	FuncGuard = "guard"
	// FuncSigGuard is the template name of [SigGuard].
	FuncSigGuard = "sigguard"
	// FuncEmbedGuard is the template name of [EmbedGuard].
	FuncEmbedGuard = "embedguard"
	// FuncTag is the template name of [Tag].
	FuncTag = "tag"
	// FuncEmbeds is the template name of the Items method of an embed
	// slot, which a template calls as {{range embeds .Embeds}}.
	FuncEmbeds = "embeds"
	// FuncDirectives is the template name of [Directives].
	FuncDirectives = "directives"
	// FuncMemberDirectives is the template name of [MemberDirectives].
	FuncMemberDirectives = "memberdirectives"
	// FuncVarType is the template name of [Speller.VarType].
	FuncVarType = "vartype"
)

// Anonymous is the type that the backend writes where a declaration has
// no type. Go cannot express a missing type, and the empty interface
// accepts every value that the source left open.
const Anonymous = "any"

// These are the parts of the Go syntax that the vocabulary and the
// lowering write around the spelling of a type.
const (
	// qualifierSep separates a package qualifier from the qualified name.
	qualifierSep = "."
	// argsOpen and argsClose enclose the type arguments of a reference.
	argsOpen  = "["
	argsClose = "]"
	// variadicMark starts the type of a variadic parameter, and
	// pointerMark starts the type of the receiver of a variant's marker
	// method.
	variadicMark = "..."
	pointerMark  = "*"
	// directiveMark starts a directive comment line, and docMark starts a
	// line of documentation.
	directiveMark = "//"
	docMark       = "// "
	// memberIndent is the indent of a member inside the braces of its
	// type, such as a field or an embed in a struct and a method in an
	// interface.
	memberIndent = "\t"
)

// The parts of a struct tag that [Tag] writes. tagOpen and tagClose
// enclose the tag in a raw string literal with a space before it,
// tagKeySep separates the key of an entry from its quoted value, and
// tagEntrySep separates two entries. tagScratch is the size of the stack
// buffer of a tag.
const (
	tagOpen     = " `"
	tagClose    = '`'
	tagKeySep   = ':'
	tagEntrySep = ' '
	tagScratch  = 128
)

// Speller writes the type references of one file. It binds the package
// of every reference in the import set of the file. The import uses the
// name that [golang.Qualifier] returns for the path, or a numbered name
// where another import or a declaration of the file already uses that
// name. The written reference qualifies its name with the bound name.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text that it writes and nothing else, so a
// builtin, a bare name and a reference in its written form allocate
// nothing. The first import of a package allocates in the set. A method
// that joins more than two parts also allocates its list of parts,
// because Go places a list of at most two strings on the stack.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller of the file with the import set set.
// set must not be nil. NewSpeller allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes the type reference t and binds the import of every
// package of t. It writes [Anonymous] for a nil reference and for a
// reference with an empty spelling.
//
// Spell writes a named reference bare where it refers to a builtin, a
// type parameter or a declaration of the package of the file. Otherwise
// it qualifies the name with the bound name of the package. The package
// is the one that [spellref.PackageOf] returns for Go: the package of the
// target where the target is a Go declaration, and the package that the
// reference records otherwise. A translated reference records the package
// of the file that declares its referent. An argument list is in the
// brackets of Go.
//
// A structural reference keeps its written spelling where every child
// keeps its written spelling, so a function type keeps its parameter
// names and a channel keeps its direction. Where a child changes, Spell
// writes the reference again through [golang.Restate], which writes a
// function type without its parameter names.
//
// Spell returns an error for a reference that it cannot write correctly:
//
//   - a qualified name whose package the model does not record;
//   - a spelling that is not a Go type name;
//   - an inline body that refers to another package, because the model
//     does not record its imports;
//   - a structural reference that its structure does not determine,
//     such as an array with an expression as its length.
//
// Spell allocates nothing for a builtin or a reference in its written
// form. A qualified name allocates the name after its qualifier, and a
// restated or instantiated reference allocates its text. A refusal
// allocates its error.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	if inline := inlineImport(t); inline != "" {
		return "", refuse("%s names a package inside an inline body, and the model does not record "+
			"the imports of the body", inline)
	}
	return spellref.SpellRestated(t, argsOpen, argsClose, Anonymous, s.qualify, restate)
}

// restate restates a composite of the model through [golang.Restate]. A
// composite of the model without a spelling is one that rules.Substitute
// restated, and the spelling recorded facts that the model does not.
//
// Error modes: a channel without a spelling returns an error, because
// its spelling recorded its direction. A function type whose last
// parameter is a list without a spelling returns an error, because the
// spelling of the list recorded whether the parameter is variadic.
func restate(t *emit.TypeRef, elems []string) (string, error) {
	if t.Form == symbol.FormStream && t.Spelling == "" {
		return "", refuse("a channel without a spelling does not record its direction")
	}
	if last := t.Split - 1; t.Form == symbol.FormFunc && last >= 0 && last < len(t.Elems) &&
		t.Elems[last] != nil && t.Elems[last].Form == symbol.FormList && t.Elems[last].Spelling == "" {

		return "", refuse("a function type takes a list without a spelling last, " +
			"and the reference does not record whether the parameter is variadic")
	}
	return golang.Restate(t, elems)
}

// TypeParams writes the type parameter list ps in brackets, and nothing
// for an empty list. A parameter without a bound gets [Anonymous], a
// parameter with one bound gets that bound, and two or more bounds
// become an inline constraint interface, which is the intersection of
// Go. The formatter decides the layout of that interface. TypeParams
// returns an error for a variance, a default and a value parameter,
// because a Go type parameter has none of them.
//
// TypeParams allocates the spelling of each parameter, the joined list
// with its brackets, and what the spelling of each bound allocates. Two
// parameters with builtin bounds allocate four times.
func (s Speller) TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		switch {
		case p.Variance != symbol.VarianceInvariant:
			return "", refuse("a type parameter states no variance, and %s states one", p.Name)
		case p.Const:
			return "", refuse("a type parameter takes a type, and %s takes a value", p.Name)
		case p.Default != nil:
			return "", refuse("a type parameter takes no default, and %s states one", p.Name)
		}
		b, err := s.bound(p.Bounds)
		if err != nil {
			return "", err
		}
		parts = append(parts, p.Name+" "+b)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// Params writes the parameter list in the slot ps with the variadic
// marker. A template passes the slot by address, as in
// {{params .Params}}, so the call reads the slot without a reflective
// method call. Params allocates the spelling of each named parameter,
// the joined list, and what the spelling of each type allocates.
func (s Speller) Params(ps *emit.Slot[*emit.Param]) (string, error) {
	items := ps.Items()
	parts := make([]string, 0, len(items))
	for _, p := range items {
		part, err := s.param(p)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", "), nil
}

// Results writes the result list in the slot results. It writes nothing
// for no result, one bare type for one unnamed result, and a list in
// parentheses otherwise. One named result is in parentheses, because Go
// requires them. A template passes the slot by address, as Params
// describes. Results allocates the list that it writes, the joined list
// of two or more results, and what the spelling of each type allocates.
func (s Speller) Results(results *emit.Slot[*emit.Return]) (string, error) {
	rs := results.Items()
	if len(rs) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		typ, err := s.Spell(r.Type)
		if err != nil {
			return "", err
		}
		part := typ + textfmt.Inline(r.Comment)
		if r.Name != "" {
			part = r.Name + " " + part
		}
		parts = append(parts, part)
	}
	if len(rs) == 1 && rs[0].Name == "" {
		return " " + parts[0], nil
	}
	return " (" + strings.Join(parts, ", ") + ")", nil
}

// Receiver writes the receiver of the method m. For a method without a
// Receiver, it writes the Receives type alone. Go accepts that form,
// because a receiver that no body reads needs no name. Receiver
// allocates what the spelling of the receiver allocates, and nothing for
// a bare type.
func (s Speller) Receiver(m *emit.Method) (string, error) {
	switch {
	case m.Receiver != nil:
		return s.param(m.Receiver)
	case m.Receives != nil:
		return s.Spell(m.Receives)
	default:
		return "", nil
	}
}

// VarType writes the type of the variable v after a space. It writes
// nothing where v has an initializer and no type, because Go infers the
// type. It returns an error for a variable without a type and without
// an initializer, because Go does not accept var x alone. It allocates
// the type with its space, and what the spelling of the type allocates.
func (s Speller) VarType(v *emit.Variable) (string, error) {
	switch {
	case v.Type != nil:
		typ, err := s.Spell(v.Type)
		if err != nil {
			return "", err
		}
		return " " + typ, nil
	case v.Value != "":
		return "", nil
	default:
		return "", refuse("a variable states a type or an initializer, and %s states neither", v.Name)
	}
}

// qualify writes the name of the named reference t, qualified with the
// bound name of its package, and binds the import of the package under
// the name that [golang.Qualifier] returns for its path. For a reference
// without a target, which keeps the spelling of its source, qualify
// returns an error where the spelling is not a Go type name, and where it
// is qualified and the model does not record a package for it.
func (s Speller) qualify(t *emit.TypeRef) (string, error) {
	pkg := spellref.PackageOf(t, golang.Lang)
	if t.Target.IsZero() {
		switch {
		case strings.Count(t.Spelling, qualifierSep) > 1:
			return "", refuse("%s is not a Go type name", t.Spelling)
		case pkg == "" && strings.Contains(t.Spelling, qualifierSep):
			return "", refuse("%s is qualified, and the model does not record a package for it", t.Spelling)
		}
	}
	if pkg == "" {
		return t.Spelling, nil
	}
	name := golang.Unqualified(t.Spelling)
	if local := s.set.Bind(pkg, golang.Qualifier(pkg)); local != "" {
		name = local + qualifierSep + name
	}
	return name, nil
}

// bound writes the constraint of a type parameter with the bounds bs. It
// writes [Anonymous] for no bound, the bound for one, and an inline
// constraint interface for two or more.
func (s Speller) bound(bs []*emit.TypeRef) (string, error) {
	if len(bs) == 0 {
		return Anonymous, nil
	}
	parts := make([]string, 0, len(bs))
	for _, b := range bs {
		part, err := s.Spell(b)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return "interface{ " + strings.Join(parts, "; ") + " }", nil
}

// param writes the parameter p. It writes the name where p has one, and
// then the type, with three dots before the type of a variadic
// parameter. The trailing comment follows as a block comment, because
// gofmt keeps only that form inside a list on one line.
func (s Speller) param(p *emit.Param) (string, error) {
	spelling, err := s.Spell(p.Type)
	if err != nil {
		return "", err
	}
	if p.Variadic != symbol.VariadicNone {
		spelling = variadicMark + spelling
	}
	if p.Name != "" {
		spelling = p.Name + " " + spelling
	}
	return spelling + textfmt.Inline(p.Comment), nil
}

// Funcs returns the template vocabulary that the kind templates call. The
// helpers bind the import of the package of every reference that they
// write in set, the import set of one file. A consumer
// that composes a variant backend registers the vocabulary with its own
// additions. Every helper takes a fixed list of arguments, because
// text/template allocates the list of a variadic call and a value for
// each string argument in a template. Funcs allocates the map of sixteen
// helpers in four allocations, and the six bound helpers of the speller.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:             Docs,
		FuncMemberDocs:       MemberDocs,
		FuncSpell:            s.Spell,
		FuncTypeParams:       s.TypeParams,
		FuncParams:           s.Params,
		FuncResults:          s.Results,
		FuncReceiver:         s.Receiver,
		FuncPackage:          Package,
		FuncGuard:            Guard,
		FuncSigGuard:         SigGuard,
		FuncEmbedGuard:       EmbedGuard,
		FuncTag:              Tag,
		FuncEmbeds:           (*emit.Slot[*emit.Embed]).Items,
		FuncDirectives:       Directives,
		FuncMemberDirectives: MemberDirectives,
		FuncVarType:          s.VarType,
	}
}

// Docs writes the documentation lines of a declaration as line comments
// at the top level. It allocates what [textfmt.LineDocs] allocates. That
// is the lines, sized once, and nothing for no lines.
func Docs(lines []string) string {
	return textfmt.LineDocs(lines, docMark)
}

// MemberDocs writes the documentation lines of a member as line comments
// after one tab. A tab is the indent of a field or an embed in a struct
// and of a method in an interface. It allocates what [Docs] allocates.
func MemberDocs(lines []string) string {
	return textfmt.LineDocs(lines, docMark, memberIndent)
}

// Package writes the name in the package clause of the package id. It
// writes the name of id, or else the qualifier that [golang.Qualifier]
// returns for the path, so a path whose last element is a keyword still
// declares an identifier. Go requires a package clause in each file, and
// the backend does not invent a package name. Package therefore returns
// an error for an identity without a package, and the render withholds
// the file. The layout routes a file without a package where no Go
// module contains the directory of the file and the plan has no import
// base for it. Package allocates an escaped qualifier. A refusal
// allocates its error.
func Package(id symbol.Identity) (string, error) {
	switch {
	case id.Name != "":
		return id.Name, nil
	case id.Package != "":
		return golang.Qualifier(id.Package), nil
	default:
		return "", refuse("every Go file declares a package, and none derives for this file: " +
			"no Go module contains its directory, and the plan states no import base for it")
	}
}

// Guard checks the declaration d and writes nothing. It returns an error
// for every fact that Go cannot express, so the render never drops a
// fact without a report. Guard returns an error for these facts:
//
//   - async, abstract, an override marker and a default marker;
//   - a type-level member;
//   - a method without a receiver type;
//   - the mutability or the initializer of a field;
//   - an immutable variable and a constant without a value;
//   - a visibility other than the exported and the package scope, which
//     the case of a name expresses.
//
// Guard accepts a final struct or method, because Go has no subclasses.
// It allocates nothing for a declaration that it accepts. A refusal
// allocates its error.
func Guard(d symbol.Symbol) (string, error) {
	switch t := d.(type) {
	case *emit.Struct:
		if t.Abstract {
			return "", refuse("every struct can be made, and %s states abstract", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Interface:
		return "", cased(t.Visibility, t.Name)
	case *emit.Function:
		if t.Async {
			return "", refuse("concurrency is caller-side, and %s states async", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Method:
		switch {
		case t.Async:
			return "", refuse("concurrency is caller-side, and %s states async", t.Name)
		case t.Abstract:
			return "", refuse("a method states its body outright, and %s states abstract", t.Name)
		case t.Override:
			return "", refuse("a method overrides nothing, and %s states it", t.Name)
		case t.HasDefault:
			return "", refuse("default bodies belong to interfaces elsewhere, and %s states one", t.Name)
		case t.Level == symbol.LevelType:
			return "", refuse("a type-level callable is a function, and %s states a static method", t.Name)
		case t.Receiver == nil && t.Receives == nil:
			return "", refuse("a method is declared on a receiver type, and %s names none", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Field:
		switch {
		case t.Level == symbol.LevelType:
			return "", refuse("a struct declares no statics, and %s states type level", t.Name)
		case t.Mutability == symbol.MutabilityImmutable:
			return "", refuse("a field's mutability follows its binding, and %s states its own", t.Name)
		case t.Value != "":
			return "", refuse("a struct declares no field defaults, and %s states one", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Alias:
		return "", cased(t.Visibility, t.Name)
	case *emit.Constant:
		if t.Value == "" {
			return "", refuse("a constant takes a value, and %s states none", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Variable:
		if t.Mutability == symbol.MutabilityImmutable {
			return "", refuse("an immutable binding is a constant, and %s states a variable", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	default:
		return "", nil
	}
}

// SigGuard checks the interface method m and writes nothing. It accepts
// an abstract method, because an interface method is a signature
// without a body, and a final method, because nothing overrides in Go.
// It returns an error for a body, because a signature has no body, and
// for type parameters, because a Go interface method has none. It
// returns an error for every other fact of an interface method as
// [Guard] does. SigGuard allocates nothing for a signature that it
// accepts. A refusal allocates its error.
func SigGuard(m *emit.Method) (string, error) {
	switch {
	case !m.Body.IsZero():
		return "", refuse("an interface method is a signature, and %s states a body", m.Name)
	case len(m.TypeParams) > 0:
		return "", refuse("an interface method takes no type parameters, and %s states %d",
			m.Name, len(m.TypeParams))
	case m.Async:
		return "", refuse("concurrency is caller-side, and %s states async", m.Name)
	case m.Override:
		return "", refuse("a method overrides nothing, and %s states it", m.Name)
	case m.HasDefault:
		return "", refuse("an interface method has no body, and %s states a default", m.Name)
	case m.Level == symbol.LevelType:
		return "", refuse("an interface declares no statics, and %s states type level", m.Name)
	case len(m.Annotations) > 0:
		return "", refuse("an interface method takes no annotations, and %s states some", m.Name)
	}
	return "", cased(m.Visibility, m.Name)
}

// EmbedGuard checks the embed e of an interface and writes nothing. It
// returns an error for a tag, because only a struct field has a tag in
// Go. It allocates nothing for an embed that it accepts. A refusal
// allocates its error.
func EmbedGuard(e *emit.Embed) (string, error) {
	if e.Tag != "" {
		name := ""
		if e.Ref != nil {
			name = e.Ref.Spelling
		}
		return "", refuse("an interface's embed takes no tag, and %s states %q", name, e.Tag)
	}
	return "", nil
}

// Tag writes the struct tag of the field f in backquotes, with a space
// before it, and nothing for a field without tag entries. It writes each
// entry as key:"value", with the value quoted by [strconv.AppendQuote],
// and separates two entries with a space, as [reflect.StructTag] reads
// them. Tag returns an error for an empty key and for a key with a
// space, a colon, a quote, a backquote or a control character, because
// reflect.StructTag cannot read such a key. It returns an error for a
// value with a backquote, because the tag is a raw string literal.
//
// Tag allocates the tag once, and nothing for a field without entries. A
// tag longer than 128 bytes allocates once more. A refusal allocates its
// error.
func Tag(f *emit.Field) (string, error) {
	entries := f.Tags.Items()
	if len(entries) == 0 {
		return "", nil
	}
	var scratch [tagScratch]byte
	b := append(scratch[:0], tagOpen...)
	for i, e := range entries {
		unreadable := strings.ContainsFunc(e.Key, func(r rune) bool {
			return r <= ' ' || r == tagKeySep || r == '"' || r == tagClose || r == 0x7f
		})
		if e.Key == "" || unreadable {
			return "", refuse("a tag key has no space, colon, quote or control character, "+
				"and %s states %q", f.Name, e.Key)
		}
		if strings.ContainsRune(e.Value, tagClose) {
			return "", refuse("a struct tag is a raw string without a backquote, and %s states %q",
				f.Name, e.Value)
		}
		if i > 0 {
			b = append(b, tagEntrySep)
		}
		b = append(b, e.Key...)
		b = append(b, tagKeySep)
		b = strconv.AppendQuote(b, e.Value)
	}
	b = append(b, tagClose)
	return string(b), nil
}

// Directives writes one Go directive comment line at the top level for
// each annotation of as, such as //go:embed or //nolint:gosec. The
// arguments follow the name, joined by spaces. A Go directive is a
// comment with an exact spelling, so the annotation passes through
// verbatim, and the generator is responsible for a name without
// documentation. The frontend reads a name outside the tool:name form
// and the legacy forms with a space as documentation.
//
// Directives sizes the lines once and allocates them once, and allocates
// nothing for a declaration without annotations.
func Directives(as symbol.Annotations) string {
	return directiveLines(as, "")
}

// MemberDirectives writes the annotations of a member as the directive
// comment lines of [Directives]. Each line starts with one tab. It
// allocates what Directives allocates.
func MemberDirectives(as symbol.Annotations) string {
	return directiveLines(as, memberIndent)
}

// directiveLines writes one directive comment line for each annotation
// of as, each after prefix.
func directiveLines(as symbol.Annotations, prefix string) string {
	if len(as) == 0 {
		return ""
	}
	size := 0
	for _, a := range as {
		size += len(prefix) + len(directiveMark) + len(a.Name) + len("\n")
		for _, arg := range a.Args {
			size += len(" ") + len(arg)
		}
	}
	var b strings.Builder
	b.Grow(size)
	for _, a := range as {
		b.WriteString(prefix)
		b.WriteString(directiveMark)
		b.WriteString(a.Name)
		for _, arg := range a.Args {
			b.WriteString(" ")
			b.WriteString(arg)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// qualifiesInside reports whether the spelling of an inline body refers
// to a declaration of another package, which Go writes as a selector. It
// reports false for a spelling that does not parse, and the formatter
// reports that spelling.
func qualifiesInside(spelling string) bool {
	e, err := parser.ParseExpr(spelling)
	if err != nil {
		return false
	}
	qualified := false
	ast.Inspect(e, func(n ast.Node) bool {
		if _, is := n.(*ast.SelectorExpr); is {
			qualified = true
		}
		return !qualified
	})
	return qualified
}

// inlineImport returns the spelling of the first inline body inside t that
// refers to a declaration of another package, and the empty string where
// no inline body does. It walks the children and the arguments of t, and
// allocates what [qualifiesInside] allocates for each inline body.
func inlineImport(t *emit.TypeRef) string {
	switch {
	case t == nil:
		return ""
	case t.Form == symbol.FormInline && qualifiesInside(t.Spelling):
		return t.Spelling
	}
	for _, c := range t.Elems {
		if inline := inlineImport(c); inline != "" {
			return inline
		}
	}
	for _, a := range t.Args {
		if inline := inlineImport(a); inline != "" {
			return inline
		}
	}
	return ""
}

// cased returns an error for a visibility v that the case of a name
// cannot express. The first rune of a name expresses the exported and
// the package scope, and the other visibilities have no form in Go.
func cased(v symbol.Visibility, name string) error {
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic,
		symbol.VisibilityPackage:
		return nil
	default:
		return refuse("visibility spells through the name's case, and %s "+
			"states a scope no case expresses", name)
	}
}

// refusalPrefix is the start of every refusal of the backend. It is the
// identity of the language, as in the refusals of every backend.
const refusalPrefix = string(golang.Lang) + ": "

// refuse returns a refusal that starts with [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
