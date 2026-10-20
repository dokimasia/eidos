// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typeKeyword opens the type statement of PEP 695.
const typeKeyword = "type "

// declarationRoom is the room that a declaration of a module reserves for
// its keywords, its qualifiers and its type beside its name, its value, its
// comment and its docstring. A longer declaration grows its text once more.
const declarationRoom = 48

// Alias starts the declaration a in the file with the header h, as every
// kind writer starts it, and writes the type alias a, with its docstring
// after it:
//
//   - a defined alias is a NewType of its target, as UserId = NewType(
//     "UserId", int);
//   - from Python 3.12, an alias is a type statement with its type
//     parameters, as type Pair[K, V] = tuple[K, V];
//   - for Python 3.11, and where the type parameters of the alias are type
//     variables of the module, an alias is an assignment annotated with
//     TypeAlias, whose target is a string where it names a declaration of
//     the file that has not finished rendering.
//
// The metadata of the annotated marker of the alias wrap the target in
// Annotated.
//
// Error modes: an alias without a target, a defined alias with type
// parameters, which NewType cannot declare, the refusals of the types, and
// the errors of the start of a declaration.
//
// # Allocation contract
//
// Alias allocates the declaration, and the type parameters of PEP 695 of a
// generic alias.
func (w *Writer) Alias(h *emit.File, a *emit.Alias) (string, error) {
	if err := w.declare(h, a); err != nil {
		return "", err
	}
	switch {
	case a.Target == nil:
		return "", refuse("an alias of a module states its target, and %s states none", a.Name)
	case a.Defined && len(a.TypeParams) > 0:
		return "", refuse("NewType declares no type parameters, and %s states them", a.Name)
	case a.Defined:
		base, err := w.SpellValue(a.Target)
		if err != nil {
			return "", err
		}
		args := []string{strconv.Quote(a.Name), base}
		return assignment(a.Name, w.typing(newTypeName), args, a.Doc, a.Comment), nil
	}
	typeParams, err := w.TypeParams(a.TypeParams)
	if err != nil {
		return "", err
	}
	statement := len(a.TypeParams) == 0 && minorOf(w.set) >= minor312 ||
		len(a.TypeParams) > 0 && !w.legacy(a.TypeParams)
	var target string
	if statement {
		target, err = w.spellLazy(a.Target)
	} else {
		target, err = w.Spell(a.Target)
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(declarationRoom + len(a.Name) + len(typeParams) + len(target) + len(a.Comment) + docstringSize(a.Doc, ""))
	if statement {
		b.WriteString(typeKeyword)
		b.WriteString(a.Name)
		b.WriteString(typeParams)
	} else {
		b.WriteString(a.Name)
		b.WriteString(annotationSep)
		b.WriteString(w.typing(typeAliasName))
	}
	b.WriteString(assign)
	w.writeAnnotated(&b, a.Annotations, "", target)
	writeComment(&b, a.Comment)
	b.WriteString(newline)
	writeDocstring(&b, a.Doc, "")
	return b.String(), nil
}

// Constant starts the declaration c in the file with the header h, as every
// kind writer starts it, and writes the constant c as an assignment
// annotated with Final, as LIMIT: Final = 10 or LIMIT: Final[int] = 10
// where it has a type, with its docstring after it. A stub file keeps the
// value, because the value of a constant is part of its type. The metadata
// of the annotated marker of the constant wrap the annotation in Annotated.
//
// Error modes: a constant without a value, the refusals of the type, and
// the errors of the start of a declaration.
//
// # Allocation contract
//
// Constant allocates the declaration.
func (w *Writer) Constant(h *emit.File, c *emit.Constant) (string, error) {
	if err := w.declare(h, c); err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", refuse("a constant takes a value, and %s states none", c.Name)
	}
	text := ""
	if c.Type != nil {
		spelled, err := w.Spell(c.Type)
		if err != nil {
			return "", err
		}
		text = spelled
	}
	var b strings.Builder
	b.Grow(declarationRoom + len(c.Name) + len(text) + len(c.Value) + len(c.Comment) + docstringSize(c.Doc, ""))
	b.WriteString(c.Name)
	b.WriteString(annotationSep)
	w.writeAnnotated(&b, c.Annotations, w.typing(finalName), text)
	b.WriteString(assign)
	b.WriteString(c.Value)
	writeComment(&b, c.Comment)
	b.WriteString(newline)
	writeDocstring(&b, c.Doc, "")
	return b.String(), nil
}

// Variable starts the declaration v in the file with the header h, as every
// kind writer starts it, and writes the variable v of a module, with its
// docstring after it: name: type = value, Final of the type for an
// immutable variable, name = value without a type, and name with the type
// of a dynamic value without a type and without a value. A stub file writes
// ... for the value, and the type of a dynamic value for a variable without
// a type. The metadata of the annotated marker of the variable wrap the
// annotation in Annotated.
//
// Error modes: the refusals of the type, and the errors of the start of a
// declaration.
//
// # Allocation contract
//
// Variable allocates the declaration.
func (w *Writer) Variable(h *emit.File, v *emit.Variable) (string, error) {
	if err := w.declare(h, v); err != nil {
		return "", err
	}
	value := v.Value
	if value != "" && w.stub {
		value = emptyBody
	}
	annotated := v.Type != nil || value == "" || w.stub
	text := ""
	if annotated {
		spelled, err := w.Spell(v.Type)
		if err != nil {
			return "", err
		}
		text = spelled
	}
	var b strings.Builder
	b.Grow(declarationRoom + len(v.Name) + len(text) + len(value) + len(v.Comment) + docstringSize(v.Doc, ""))
	b.WriteString(v.Name)
	if annotated {
		qualifier := ""
		if v.Mutability == symbol.MutabilityImmutable {
			qualifier = w.typing(finalName)
		}
		b.WriteString(annotationSep)
		w.writeAnnotated(&b, v.Annotations, qualifier, text)
	}
	if value != "" {
		b.WriteString(assign)
		b.WriteString(value)
	}
	writeComment(&b, v.Comment)
	b.WriteString(newline)
	writeDocstring(&b, v.Doc, "")
	return b.String(), nil
}

// writeAnnotated writes an annotation into b: the qualifier with the text
// in brackets, the qualifier alone for no text, and the text alone for no
// qualifier, all inside Annotated with the metadata of the annotated marker
// of annotations.
func (w *Writer) writeAnnotated(b *strings.Builder, annotations symbol.Annotations, qualifier, text string) {
	metadata, marked := markerArgs(annotations, python.AnnotatedMarker)
	if marked {
		b.WriteString(w.typing(annotatedName))
		b.WriteString(argsOpen)
	}
	switch {
	case qualifier == "":
		b.WriteString(text)
	case text == "":
		b.WriteString(qualifier)
	default:
		b.WriteString(qualifier)
		b.WriteString(argsOpen)
		b.WriteString(text)
		b.WriteString(argsClose)
	}
	if marked {
		writeMetadata(b, metadata)
	}
}

// writeMetadata writes the metadata of Annotated into b after the annotated
// type, each after a comma, and the closing bracket.
func writeMetadata(b *strings.Builder, metadata []string) {
	for _, m := range metadata {
		b.WriteString(argSep)
		b.WriteString(m)
	}
	b.WriteString(argsClose)
}
