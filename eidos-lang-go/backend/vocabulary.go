// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
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
		FuncDocs:     Docs,
		FuncSpell:    Spell,
		FuncParams:   Params,
		FuncResults:  Results,
		FuncReceiver: Receiver,
		FuncPackage:  Package,
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
// rendering back to Go needs; a missing one spells [Anonymous].
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return Anonymous
	}
	return t.Spelling
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
