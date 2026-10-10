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
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
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
	// FuncResults writes a result list.
	FuncResults = "results"
	// FuncReceiver writes a method's receiver.
	FuncReceiver = "receiver"
	// FuncPackage writes a file's package clause name.
	FuncPackage = "package"
	// FuncGuard refuses what Go states nowhere.
	FuncGuard = "guard"
	// FuncSigGuard refuses what an interface method states nowhere.
	FuncSigGuard = "sigguard"
	// FuncEmbedGuard refuses what an interface's embed states nowhere.
	FuncEmbedGuard = "embedguard"
	// FuncDirectives writes annotations as directive comment lines.
	FuncDirectives = "directives"
	// FuncVarType writes a variable's type slot, empty where an
	// initializer lets Go infer.
	FuncVarType = "vartype"
)

// Anonymous is what Go writes where a declaration states no type.
// Go has no way to spell the absence of one, and the empty
// interface is the spelling that accepts what the source left
// open.
const Anonymous = "any"

// The Go syntax a composite reference is restated in.
const (
	// qualifierSep separates a package qualifier from the name it
	// qualifies.
	qualifierSep = "."
	// variadicMark opens a variadic parameter's type.
	variadicMark = "..."
	pointerMark  = "*"
	sliceMark    = "[]"
	// mapOpen opens a map's key, and mapClose closes it before the
	// value.
	mapOpen  = "map["
	mapClose = "]"
	// recvChan, sendChan and chanWord open a receive-only, a
	// send-only and a bidirectional channel.
	recvChan = "<-chan "
	sendChan = "chan<- "
	chanWord = "chan "
	funcOpen = "func("
	// directiveMark opens a directive comment line.
	directiveMark = "//"
)

// Speller spells type references for one file. Every package a
// spelling names is bound in the file's import set, under the name
// the import path assumes or a numbered one where another import or
// a declaration of the file takes that name, and the spelling
// qualifies through the bound name.
//
// # Concurrency
//
// A Speller is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// Each method allocates the text it writes and nothing else, so a
// builtin, a bare name and a reference spelled as written allocate
// nothing. Binding a package's import for the first time allocates in
// the set. A method that joins more than two parts also allocates its
// list of parts, because Go places a list of at most two strings on
// the stack.
type Speller struct {
	set *render.ImportSet
}

// NewSpeller returns the speller for the file whose import set is
// set. The set is not nil. It allocates nothing.
func NewSpeller(set *render.ImportSet) Speller { return Speller{set: set} }

// Spell writes a type reference and binds the import of every
// package it names. A missing reference, or one spelling nothing,
// spells [Anonymous].
//
// A named reference spells bare where it names a builtin, a type
// parameter or a declaration of the file's own package, and behind
// its package's bound name otherwise. The package is its target's
// where the target is a Go declaration, and the one the reference
// records where it has no target. A reference whose target is
// another language's declaration spells as written, because its
// package names no Go import. An argument list spells in Go's
// brackets.
//
// A structural reference keeps its written spelling where every
// child spells as written, which keeps a function type's parameter
// names and a channel's direction. Where a child spells otherwise,
// the reference is restated from its structure: a pointer, a slice,
// a variadic parameter, a sized array, a map, a channel in its
// written direction, and a function type without its parameter
// names.
//
// Spell returns an error for a reference it cannot spell correctly:
// a qualified name whose package the model does not record, a
// spelling that is no Go type name, an inline body that names
// another package, whose imports the model does not record, and a
// structural reference to restate that the structure does not
// determine, which is an array whose length is an expression.
//
// Spell allocates nothing for a builtin or a reference spelled as
// written. A qualified name allocates the name behind its qualifier,
// and a restated or instantiated reference allocates its text. A
// refusal allocates its error.
func (s Speller) Spell(t *emit.TypeRef) (string, error) {
	if t == nil || t.Spelling == "" {
		return Anonymous, nil
	}
	out, _, err := s.spell(t)
	return out, err
}

// TypeParams writes a type parameter list in brackets, or nothing
// for a declaration stating none. A parameter without a bound
// spells [Anonymous], one bound spells itself, and several fold
// into an inline constraint interface, which is Go's intersection.
// The formatter settles that interface's layout. Variance, defaults
// and value parameters return an error, because Go's parameters
// state none of the three.
//
// TypeParams allocates each parameter's spelling, the joined list and
// its brackets, and what each bound's spelling allocates: four
// allocations for two parameters of builtin bounds.
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

// Params writes a parameter list, variadic marker included. It
// allocates each named parameter's spelling and the joined list, and
// what each type's spelling allocates.
func (s Speller) Params(ps []*emit.Param) (string, error) {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		part, err := s.param(p)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", "), nil
}

// Results writes a result list: nothing, one bare type, or a
// parenthesised list, which is how Go spells each case. A single
// named result still parenthesises, because Go requires it. It
// allocates the list it writes, the joined list of several results
// beside it, and what each type's spelling allocates.
func (s Speller) Results(rs []*emit.Return) (string, error) {
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

// Receiver writes a method's receiver. A method declared outside
// the type it attaches to spells the type alone, which Go accepts,
// because a receiver no body reads needs no name. It allocates what the
// receiver's spelling allocates, and nothing for a bare type.
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

// VarType writes a variable's type slot: the spelled type behind a
// space, nothing where an initializer lets Go infer, and an error
// where the declaration states neither, because `var x` alone
// declares nothing Go accepts. It allocates the type behind its space,
// and what the type's spelling allocates.
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

// spell writes one reference and reports whether the result is the
// reference's written spelling, which a parent composite keeps where
// every child's is.
func (s Speller) spell(t *emit.TypeRef) (string, bool, error) {
	switch {
	case t == nil || t.Spelling == "":
		return Anonymous, true, nil
	case t.Form == symbol.FormNamed:
		return s.named(t)
	case t.Form == symbol.FormInline && qualifiesInside(t.Spelling):
		return "", false, refuse("%s names a package inside an inline body, whose imports "+
			"the model does not record", t.Spelling)
	default:
		return s.composite(t)
	}
}

// named writes a named reference, its argument list in brackets.
func (s Speller) named(t *emit.TypeRef) (string, bool, error) {
	name, written, err := s.qualified(t)
	if err != nil {
		return "", false, err
	}
	if len(t.Args) == 0 {
		return name, written, nil
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		arg, argWritten, err := s.spell(a)
		if err != nil {
			return "", false, err
		}
		written = written && argWritten
		args = append(args, arg)
	}
	return name + "[" + strings.Join(args, ", ") + "]", written, nil
}

// qualified writes a named reference's name through its package's
// bound name, and reports whether that is the name as written.
func (s Speller) qualified(t *emit.TypeRef) (string, bool, error) {
	pkg, err := packageOf(t)
	if err != nil || pkg == "" {
		return t.Spelling, true, err
	}
	name := golang.Unqualified(t.Spelling)
	if local := s.set.Bind(pkg, golang.AssumedName(pkg)); local != "" {
		name = local + qualifierSep + name
	}
	return name, name == t.Spelling, nil
}

// composite writes a structural reference: as written where every
// child spells as written, and restated from its structure
// otherwise. The settle rewrites the spelling for the names it
// respells beneath it, so a written spelling names each child as the
// child spells.
func (s Speller) composite(t *emit.TypeRef) (string, bool, error) {
	elems := make([]string, 0, len(t.Elems))
	written := true
	for _, c := range t.Elems {
		out, childWritten, err := s.spell(c)
		if err != nil {
			return "", false, err
		}
		written = written && childWritten
		elems = append(elems, out)
	}
	if written {
		return t.Spelling, true, nil
	}
	out, err := restate(t, elems)
	return out, false, err
}

// bound writes one parameter's constraint: [Anonymous] for none,
// the bound itself for one, and an inline constraint interface
// for several.
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

// param writes one parameter: its name where it has one, its
// type, which a variadic parameter opens with three dots, and its
// trailing comment as a block comment, the one comment form gofmt
// keeps inside a one-line list.
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

// Funcs returns the shared template vocabulary the kind templates
// call, bound to one file's import set: every reference a helper
// spells binds its package's import there. A consumer composing a
// variant backend registers it beside its own additions. Funcs
// allocates the map of twelve helpers, four allocations, and the
// speller's six bound helpers.
func Funcs(set *render.ImportSet) template.FuncMap {
	s := NewSpeller(set)
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      s.Spell,
		FuncTypeParams: s.TypeParams,
		FuncParams:     s.Params,
		FuncResults:    s.Results,
		FuncReceiver:   s.Receiver,
		FuncPackage:    Package,
		FuncGuard:      Guard,
		FuncSigGuard:   SigGuard,
		FuncEmbedGuard: EmbedGuard,
		FuncDirectives: Directives,
		FuncVarType:    s.VarType,
	}
}

// Docs writes a declaration's documentation as line comments,
// each prefixed with the given indentation, so a member's doc
// is at its member's depth. Called with no prefix it writes at
// the top level. It allocates what [textfmt.LineDocs] allocates: the
// lines, sized once, and nothing for no lines.
func Docs(lines []string, prefix ...string) string {
	return textfmt.LineDocs(lines, "// ", prefix...)
}

// Package writes the package clause's name, taken from the
// identity's own name and falling back to the name its path
// assumes, [golang.AssumedName]. Every Go file declares a package,
// and the backend invents no package name, so an identity naming no
// package returns an error, and the render withholds the file. The
// layout routes a file without a package where no Go module contains
// the file's directory and the plan states no import base for it.
// Package allocates nothing, and a refusal allocates its error.
func Package(id symbol.Identity) (string, error) {
	switch {
	case id.Name != "":
		return id.Name, nil
	case id.Package != "":
		return golang.AssumedName(id.Package), nil
	default:
		return "", refuse("every Go file declares a package, and none derives for this file: " +
			"no Go module contains its directory, and the plan states no import base for it")
	}
}

// Guard writes nothing and refuses what Go states nowhere, so a
// stated fact is never dropped in silence: asynchrony, abstractness,
// an override or default marker, a type-level member, a method with
// no receiver type, a field's own mutability or initializer, an
// immutable variable, a constant without a value, and every
// visibility beyond the exported and package scopes the name's case
// spells. A final struct or method passes, because nothing
// subclasses in Go. Guard allocates nothing for a declaration it
// passes, and a refusal allocates its error.
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

// SigGuard writes nothing and refuses what an interface method
// states nowhere. An abstract method passes, because an interface
// method is a signature without a body, and so does a final one,
// because nothing overrides in Go. A body is refused, because the
// signature cannot place it, and so are type parameters, because Go
// gives an interface method none. Everything else an interface
// method could state is refused the way [Guard] refuses it. SigGuard
// allocates nothing for a signature it passes, and a refusal allocates
// its error.
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
		return "", unannotated(m.Name)
	}
	return "", cased(m.Visibility, m.Name)
}

// EmbedGuard writes nothing and refuses what an interface's embed
// states nowhere: a tag, which Go gives a struct field alone. It
// allocates nothing for an embed it passes, and a refusal allocates its
// error.
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

// Directives writes a declaration's annotations as Go directive
// comment lines, such as //go:embed or //nolint:gosec, one per
// annotation, its arguments space-joined behind the name, at the
// member's depth where one is given. Go's directives are comments
// whose spelling is the contract, so the annotation passes through
// verbatim, and the generator is responsible for an undocumented
// name. A name outside the tool:name shape and the legacy space
// forms reads back as documentation on the frontend side.
//
// Directives sizes the lines once and allocates them, one allocation,
// and nothing for a declaration without annotations.
func Directives(as symbol.Annotations, indent ...string) string {
	if len(as) == 0 {
		return ""
	}
	prefix := ""
	if len(indent) > 0 {
		prefix = indent[0]
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

// qualifiesInside reports whether an inline body's spelling names a
// declaration of another package, which Go writes as a selector. A
// spelling that does not parse reports false, and the formatter
// reports it.
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

// packageOf returns the import path of the package of a named reference,
// and the empty path for a reference without a package. For a reference
// whose target is a declaration of another language, it returns the
// package that the layout recorded. The spelling of a reference without
// a target must be a Go type name, and a qualified spelling must have a
// recorded package.
func packageOf(t *emit.TypeRef) (string, error) {
	switch {
	case !t.Target.IsZero() && t.Target.Lang != golang.Lang:
		return t.Package, nil
	case !t.Target.IsZero():
		return t.Target.Package, nil
	case strings.Count(t.Spelling, qualifierSep) > 1:
		return "", refuse("%s is no Go type name", t.Spelling)
	case t.Package == "" && strings.Contains(t.Spelling, qualifierSep):
		return "", refuse("%s is qualified, and the model records no package for it", t.Spelling)
	default:
		return t.Package, nil
	}
}

// restate writes a structural reference from its children's
// spellings. It returns an error for a form whose Go spelling the
// structure does not determine, and for a reference missing the
// children its form takes.
func restate(t *emit.TypeRef, elems []string) (string, error) {
	var want int
	switch t.Form {
	case symbol.FormMap:
		want = 2
	case symbol.FormFunc:
		want = t.Split
	default:
		want = 1
	}
	if len(elems) < want || t.Split > len(elems) {
		return "", refuse("%s states %d of the %d types its form takes", t.Spelling, len(elems), want)
	}
	switch t.Form {
	case symbol.FormOptional:
		return pointerMark + elems[0], nil
	case symbol.FormList:
		if strings.HasPrefix(t.Spelling, variadicMark) {
			return variadicMark + elems[0], nil
		}
		return sliceMark + elems[0], nil
	case symbol.FormArray:
		if t.Length == 0 {
			return "", refuse("%s states its length as an expression, which its structure cannot restate",
				t.Spelling)
		}
		return "[" + strconv.Itoa(t.Length) + "]" + elems[0], nil
	case symbol.FormMap:
		return mapOpen + elems[0] + mapClose + elems[1], nil
	case symbol.FormStream:
		return direction(t.Spelling) + elems[0], nil
	case symbol.FormFunc:
		return funcOpen + strings.Join(elems[:t.Split], ", ") + ")" + resultList(elems[t.Split:]), nil
	default:
		return "", refuse("%s is a %s, which its structure cannot restate", t.Spelling, t.Form)
	}
}

// direction returns the keyword a channel's written spelling opens
// with, whitespace ignored: receive-only, send-only or bidirectional.
func direction(spelling string) string {
	compact := strings.Join(strings.Fields(spelling), "")
	switch {
	case strings.HasPrefix(compact, "<-"):
		return recvChan
	case strings.HasPrefix(compact, "chan<-"):
		return sendChan
	default:
		return chanWord
	}
}

// resultList writes a function type's results: nothing, one bare
// type, or a parenthesised list.
func resultList(results []string) string {
	switch len(results) {
	case 0:
		return ""
	case 1:
		return " " + results[0]
	default:
		return " (" + strings.Join(results, ", ") + ")"
	}
}

// cased refuses the visibilities the name's case cannot express:
// exported and package scopes spell through the first rune, and
// the rest have no Go spelling at all.
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

// unannotated is the refusal for an annotation list where no
// directive line is allowed.
func unannotated(name string) error {
	return refuse("an interface method takes no annotations, and %s states some", name)
}

// refusalPrefix opens every refusal the backend returns: the
// language's identity, as every backend's refusals open.
const refusalPrefix = string(golang.Lang) + ": "

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
