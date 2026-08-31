// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

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
	// FuncResults writes a return type.
	FuncResults = "results"
)

// Anonymous is what TypeScript writes where a declaration states
// no type: the type that admits everything while letting nothing
// through unchecked.
const Anonymous = "unknown"

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:    Docs,
		FuncSpell:   Spell,
		FuncParams:  Params,
		FuncResults: Results,
	}
}

// Docs writes a declaration's documentation as a TSDoc block,
// each line prefixed with the given indentation, so a member's
// doc sits at its member's depth.
func Docs(lines []string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	at := strings.Join(prefix, "")
	var b strings.Builder
	b.WriteString(at)
	b.WriteString("/**\n")
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString(" * ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(at)
	b.WriteString(" */\n")
	return b.String()
}

// Spell writes a type reference. The source spelling rides
// through verbatim; a missing one spells [Anonymous].
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return Anonymous
	}
	return t.Spelling
}

// Params writes a parameter list, the rest marker included.
func Params(ps []*emit.Param) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = "_"
		}
		if p.Variadic != symbol.VariadicNone {
			parts = append(parts, "..."+name+": "+Spell(p.Type)+"[]")
			continue
		}
		parts = append(parts, name+": "+Spell(p.Type))
	}
	return strings.Join(parts, ", ")
}

// Results writes a return type annotation: void for none, the
// type for one, and a tuple for several, because TypeScript
// returns one value however many the delegate hands back.
func Results(rs []*emit.Return) string {
	switch len(rs) {
	case 0:
		return ": void"
	case 1:
		return ": " + Spell(rs[0].Type)
	default:
		parts := make([]string, 0, len(rs))
		for _, r := range rs {
			parts = append(parts, Spell(r.Type))
		}
		return ": [" + strings.Join(parts, ", ") + "]"
	}
}

// Module writes the module stem a file's owning package spells:
// the path's last element. TypeScript has no package clause, so
// the stem serves headers and diagnostics rather than a
// declaration.
func Module(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	return path.Base(id.Package)
}
