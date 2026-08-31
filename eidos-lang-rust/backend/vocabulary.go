// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
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
	// FuncResults writes a return annotation.
	FuncResults = "results"
)

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:    Docs,
		FuncSpell:   Spell,
		FuncParams:  Params,
		FuncResults: Results,
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
// compiler's refusal names the file.
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return "()"
	}
	return t.Spelling
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
