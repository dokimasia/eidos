// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"fmt"
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
	// FuncResults writes the one return type.
	FuncResults = "results"
	// FuncPackage writes the package clause.
	FuncPackage = "package"
)

// Anonymous is what Java writes where a declaration states no
// type: the root every reference type descends from.
const Anonymous = "Object"

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:    Docs,
		FuncSpell:   Spell,
		FuncParams:  Params,
		FuncResults: Results,
		FuncPackage: PackageClause,
	}
}

// Docs writes a declaration's documentation as a Javadoc block,
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

// Params writes a parameter list, the variadic marker included.
func Params(ps []*emit.Param) string {
	parts := make([]string, 0, len(ps))
	for i, p := range ps {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("arg%d", i)
		}
		spelling := Spell(p.Type)
		if p.Variadic != symbol.VariadicNone {
			spelling += "..."
		}
		parts = append(parts, spelling+" "+name)
	}
	return strings.Join(parts, ", ")
}

// Results writes the return type: void for none, the type for
// one, and an error for several, because a Java callable returns
// one value and a second arrives thrown, not returned.
func Results(rs []*emit.Return) (string, error) {
	switch len(rs) {
	case 0:
		return "void", nil
	case 1:
		return Spell(rs[0].Type), nil
	default:
		return "", fmt.Errorf(
			"java: a callable returns one value, and this one states %d: "+
				"a second result arrives thrown, not returned", len(rs))
	}
}

// PackageClause writes the package statement from the owning
// package's path, dots for slashes, followed by a blank line. An
// identity naming no package spells nothing, which is the default
// package.
func PackageClause(id symbol.Identity) string {
	if id.Package == "" {
		return ""
	}
	return "package " + strings.ReplaceAll(id.Package, "/", ".") + ";\n\n"
}
