// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"path"
	"strings"
	"text/template"

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

// Funcs is the shared template vocabulary the kind templates call.
// A consumer composing a variant backend registers it beside its
// own additions.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      Spell,
		FuncTypeParams: TypeParams,
		FuncParams:     Params,
		FuncResults:    Results,
		FuncReceiver:   Receiver,
		FuncPackage:    Package,
		FuncGuard:      Guard,
		FuncSigGuard:   SigGuard,
		FuncDirectives: Directives,
		FuncVarType:    VarType,
	}
}

// Docs writes a declaration's documentation as line comments,
// each prefixed with the given indentation, so a member's doc
// is at its member's depth. Called with no prefix it writes at
// the top level.
func Docs(lines []string, prefix ...string) string {
	return textfmt.LineDocs(lines, "// ", prefix...)
}

// Spell writes a type reference. A reference the graph never
// resolved spells its source spelling, which is what a Go graph
// rendering back to Go needs, and a missing one spells [Anonymous].
// A reference with arguments has its bare name in Spelling, and the
// argument list spells here in Go's brackets.
func Spell(t *emit.TypeRef) string {
	return spellref.Spell(t, "[", "]", Anonymous)
}

// TypeParams writes a type parameter list in brackets, or nothing
// for a declaration stating none. A parameter without a bound
// spells [Anonymous], one bound spells itself, and several fold
// into an inline constraint interface, which is Go's intersection.
// The formatter settles that interface's layout. Variance, defaults
// and value parameters are refused, because Go's parameters state
// none of the three.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		switch {
		case p.Variance != symbol.VarianceInvariant:
			return "", fmt.Errorf(
				"go: a type parameter states no variance, and %s states one", p.Name,
			)
		case p.Const:
			return "", fmt.Errorf(
				"go: a type parameter takes a type, and %s takes a value", p.Name,
			)
		case p.Default != nil:
			return "", fmt.Errorf(
				"go: a type parameter takes no default, and %s states one", p.Name,
			)
		}
		parts = append(parts, p.Name+" "+bound(p.Bounds))
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// bound writes one parameter's constraint: [Anonymous] for none,
// the bound itself for one, and an inline constraint interface
// for several.
func bound(bs []*emit.TypeRef) string {
	switch len(bs) {
	case 0:
		return Anonymous
	case 1:
		return Spell(bs[0])
	default:
		parts := make([]string, 0, len(bs))
		for _, b := range bs {
			parts = append(parts, Spell(b))
		}
		return "interface{ " + strings.Join(parts, "; ") + " }"
	}
}

// Params writes a parameter list, variadic marker included.
func Params(ps []*emit.Param) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, param(p))
	}
	return strings.Join(parts, ", ")
}

// param writes one parameter: its name where it has one, its
// type, which a variadic parameter opens with three dots, and its
// trailing comment as a block comment, the one comment form gofmt
// keeps inside a one-line list.
func param(p *emit.Param) string {
	spelling := Spell(p.Type)
	if p.Variadic != symbol.VariadicNone {
		spelling = "..." + spelling
	}
	if p.Name != "" {
		spelling = p.Name + " " + spelling
	}
	return spelling + textfmt.Inline(p.Comment)
}

// Results writes a result list: nothing, one bare type, or a
// parenthesised list, which is how Go spells each case. A single
// named result still parenthesises, because Go requires it.
func Results(rs []*emit.Return) string {
	if len(rs) == 0 {
		return ""
	}
	if len(rs) == 1 && rs[0].Name == "" {
		return " " + Spell(rs[0].Type) + textfmt.Inline(rs[0].Comment)
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		if r.Name == "" {
			parts = append(parts, Spell(r.Type)+textfmt.Inline(r.Comment))
			continue
		}
		parts = append(parts, r.Name+" "+Spell(r.Type)+textfmt.Inline(r.Comment))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// Receiver writes a method's receiver. A method declared outside
// the type it attaches to spells the type alone, which Go accepts,
// because a receiver no body reads needs no name.
func Receiver(m *emit.Method) string {
	switch {
	case m.Receiver != nil:
		return param(m.Receiver)
	case m.Receives != nil:
		return Spell(m.Receives)
	default:
		return ""
	}
}

// Package writes the package clause's name, taken from the
// identity's own name and falling back to the last element of its
// path. An identity naming no package spells nothing, and the
// formatter then refuses the file. The backend invents no package
// name.
func Package(id symbol.Identity) string {
	switch {
	case id.Name != "":
		return id.Name
	case id.Package != "":
		return path.Base(id.Package)
	default:
		return ""
	}
}

// Guard writes nothing and refuses what Go states nowhere, so a
// stated fact is never dropped in silence: asynchrony, abstractness,
// an override or default marker, a type-level member, a field's own
// mutability or initializer, an immutable variable, a constant
// without a value, and every visibility beyond the exported and
// package scopes the name's case spells. A final struct or method
// passes, because nothing subclasses in Go.
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
			return "", refuse("a method carries its body outright, and %s states abstract", t.Name)
		case t.Override:
			return "", refuse("a method overrides nothing, and %s states it", t.Name)
		case t.HasDefault:
			return "", refuse("default bodies belong to interfaces elsewhere, and %s states one", t.Name)
		case t.Level == symbol.LevelType:
			return "", refuse("a type-level callable is a function, and %s states a static method", t.Name)
		}
		return "", cased(t.Visibility, t.Name)
	case *emit.Field:
		switch {
		case t.Level == symbol.LevelType:
			return "", refuse("a struct holds no statics, and %s states type level", t.Name)
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
// method is a signature without a body. A body is refused, because
// the signature cannot place it, and everything else an interface
// method could state is refused the way [Guard] refuses it.
func SigGuard(m *emit.Method) (string, error) {
	switch {
	case !m.Body.IsZero():
		return "", refuse("an interface method is a signature, and %s states a body", m.Name)
	case m.Async:
		return "", refuse("concurrency is caller-side, and %s states async", m.Name)
	case m.Override:
		return "", refuse("a method overrides nothing, and %s states it", m.Name)
	case m.Final:
		return "", refuse("an interface method admits no final, and %s states it", m.Name)
	case m.HasDefault:
		return "", refuse("an interface method carries no body, and %s states a default", m.Name)
	case m.Level == symbol.LevelType:
		return "", refuse("an interface holds no statics, and %s states type level", m.Name)
	case len(m.Annotations) > 0:
		return "", unannotated(m.Name)
	}
	return "", cased(m.Visibility, m.Name)
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
			"states a scope no case carries", name)
	}
}

// unannotated is the refusal for an annotation list where no
// directive line may sit.
func unannotated(name string) error {
	return refuse("an interface method carries no annotations, and %s states some", name)
}

// Directives writes a declaration's annotations as Go directive
// comment lines, such as //go:embed or //nolint:gosec, one per
// annotation, its arguments space-joined behind the name, at the
// member's depth where one is given. Go's directives are comments
// whose spelling is the contract, so the annotation passes through
// verbatim, and the generator is responsible for an undocumented
// name. A name outside the tool:name shape and the legacy space
// forms reads back as documentation on the frontend side.
func Directives(as symbol.Annotations, indent ...string) string {
	if len(as) == 0 {
		return ""
	}
	prefix := ""
	if len(indent) > 0 {
		prefix = indent[0]
	}
	var b strings.Builder
	for _, a := range as {
		b.WriteString(prefix + "//" + a.Name)
		if len(a.Args) > 0 {
			b.WriteString(" " + strings.Join(a.Args, " "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// VarType writes a variable's type slot: the spelled type behind a
// space, nothing where an initializer lets Go infer, and a refusal
// where the declaration states neither, because `var x` alone
// declares nothing Go accepts.
func VarType(v *emit.Variable) (string, error) {
	switch {
	case v.Type != nil:
		return " " + Spell(v.Type), nil
	case v.Value != "":
		return "", nil
	default:
		return "", refuse("a variable states a type or an initializer, and %s states neither", v.Name)
	}
}

// refuse builds a guard refusal under the package's error prefix.
func refuse(format string, args ...any) error {
	return fmt.Errorf("go: "+format, args...)
}
