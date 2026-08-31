// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
	"path"
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
	// FuncResults writes a result list.
	FuncResults = "results"
	// FuncReceiver writes a method's receiver.
	FuncReceiver = "receiver"
	// FuncPackage writes a file's package clause name.
	FuncPackage = "package"
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
	}
}

// Docs writes a declaration's documentation as line comments,
// each prefixed with the given indentation, so a member's doc
// sits at its member's depth. Called with no prefix it writes at
// the top level.
func Docs(lines []string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString("// ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Spell writes a type reference. A reference the graph never
// resolved carries its source spelling, which is what a Go graph
// rendering back to Go needs; a missing one spells [Anonymous]. A
// reference carrying arguments holds its bare name in Spelling,
// and the argument list spells here in Go's brackets.
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
	return t.Spelling + "[" + strings.Join(args, ", ") + "]"
}

// TypeParams writes a type parameter list in brackets, or nothing
// for a declaration stating none. A parameter without a bound
// spells [Anonymous], one bound spells itself, and several fold
// into an inline constraint interface, which is Go's intersection;
// the formatter settles that interface's layout. Variance,
// defaults and value parameters refuse: Go's parameters state none
// of the three, and dropping one would misstate the declaration.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		switch {
		case p.Variance != symbol.VarianceInvariant:
			return "", fmt.Errorf(
				"go: a type parameter states no variance, and %s states one", p.Name)
		case p.Const:
			return "", fmt.Errorf(
				"go: a type parameter takes a type, and %s takes a value", p.Name)
		case p.Default != nil:
			return "", fmt.Errorf(
				"go: a type parameter takes no default, and %s states one", p.Name)
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

// param writes one parameter: its name where it has one, and its
// type, which a variadic parameter opens with three dots.
func param(p *emit.Param) string {
	spelling := Spell(p.Type)
	if p.Variadic != symbol.VariadicNone {
		spelling = "..." + spelling
	}
	if p.Name == "" {
		return spelling
	}
	return p.Name + " " + spelling
}

// Results writes a result list: nothing, one bare type, or a
// parenthesised list, which is how Go spells each case. A single
// named result still parenthesises, because Go requires it.
func Results(rs []*emit.Return) string {
	if len(rs) == 0 {
		return ""
	}
	if len(rs) == 1 && rs[0].Name == "" {
		return " " + Spell(rs[0].Type)
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		if r.Name == "" {
			parts = append(parts, Spell(r.Type))
			continue
		}
		parts = append(parts, r.Name+" "+Spell(r.Type))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// Receiver writes a method's receiver. A method declared outside
// the type it attaches to carries the type alone, which Go
// accepts: a receiver no body reads needs no name.
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
// formatter refuses the file rather than the backend inventing a
// name for it.
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
