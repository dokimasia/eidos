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
	// FuncTypeParams writes a type parameter list.
	FuncTypeParams = "typeparams"
	// FuncBinder writes an impl block's binder.
	FuncBinder = "binder"
	// FuncParams writes a parameter list.
	FuncParams = "params"
	// FuncResults writes a return annotation.
	FuncResults = "results"
)

// Funcs is the shared template vocabulary the kind templates call.
func Funcs() template.FuncMap {
	return template.FuncMap{
		FuncDocs:       Docs,
		FuncSpell:      Spell,
		FuncTypeParams: TypeParams,
		FuncBinder:     Binder,
		FuncParams:     Params,
		FuncResults:    Results,
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
// compiler's refusal names the file. A reference carrying
// arguments holds its bare name in Spelling, and the argument
// list spells here in angle brackets.
func Spell(t *emit.TypeRef) string {
	if t == nil || t.Spelling == "" {
		return "()"
	}
	if len(t.Args) == 0 {
		return t.Spelling
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a))
	}
	return t.Spelling + "<" + strings.Join(args, ", ") + ">"
}

// TypeParams writes a type parameter list in angle brackets, or
// nothing for a declaration stating none: bounds joined by plus
// signs behind a colon, the default behind an equals sign, and a
// value parameter in the const form with its value's type.
// Variance refuses: Rust infers it from use, a declaration states
// none, and dropping it would misstate the declaration.
func TypeParams(ps []*emit.TypeParam) (string, error) {
	if len(ps) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Variance != symbol.VarianceInvariant {
			return "", fmt.Errorf(
				"rust: a type parameter states no variance, and %s states one: "+
					"variance is inferred from use", p.Name)
		}
		parts = append(parts, typeParam(p))
	}
	return "<" + strings.Join(parts, ", ") + ">", nil
}

// typeParam writes one parameter: the const form with its value
// type and default spelling, or the name behind its bounds with
// its default type.
func typeParam(p *emit.TypeParam) string {
	if p.Const {
		part := "const " + p.Name + ": " + Spell(p.Type)
		if p.DefaultValue != "" {
			part += " = " + p.DefaultValue
		}
		return part
	}
	part := p.Name
	if len(p.Bounds) > 0 {
		bounds := make([]string, 0, len(p.Bounds))
		for _, b := range p.Bounds {
			bounds = append(bounds, Spell(b))
		}
		part += ": " + strings.Join(bounds, " + ")
	}
	if p.Default != nil {
		part += " = " + Spell(p.Default)
	}
	return part
}

// Binder writes the impl binder restating a receiver's type
// arguments, or nothing for a receiver taking none. Each argument
// restates as the name the receiver references, bare: a bound
// stays on the methods the way Rust's own practice bounds
// functions rather than type definitions, and a receiver
// instantiated at a const argument has no restatable binder,
// which stays a declared limit.
func Binder(t *emit.TypeRef) string {
	if t == nil || len(t.Args) == 0 {
		return ""
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a))
	}
	return "<" + strings.Join(args, ", ") + ">"
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
