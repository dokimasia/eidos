// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of an arrow function, which is the function value of
// TypeScript.
const (
	// arrowOpen opens an arrow function without parameters, because
	// TypeScript assigns a function of fewer parameters to a function
	// type.
	arrowOpen = "() => "
	// emptyBody is the body of an arrow function without results.
	emptyBody = "{}"
	// objectOpen opens an object literal. TypeScript reads a brace after
	// the arrow as a block, so an arrow function returns an object
	// literal in parentheses.
	objectOpen = "{"
	// tupleOpen and tupleClose enclose the results of a function of two
	// or more results, and resultSep separates them. The arrow type of
	// such a function returns a tuple.
	tupleOpen  = "["
	tupleClose = "]"
	resultSep  = ", "
	// newKeyword and abstractKeyword open the spelling of a constructor
	// type. An arrow function is no value of a constructor type.
	newKeyword      = "new"
	abstractKeyword = "abstract"
)

// leaves is TypeScript's spelling of the literal leaves. A number
// keeps its text, and a string quotes in TypeScript's own grammar.
// The vocabulary has one absent value and TypeScript has two, so the
// derived one is null, which a JSON-shaped value takes.
var leaves = scaffold.Leaves{
	Lang:   typescript.Lang,
	Absent: typescript.TypeNull,
	Quote:  quote,
}

// target spells a value tree as TypeScript, importing through the
// file's import set what its references and callees name.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(typescript.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference through the file's [Speller], so a value
// and a declaration import one declaration under one name.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	return NewSpeller(t.set).Spell(ref)
}

// Callee spells the function of a call or the declaration of a name value
// under the name that its import binds. TypeScript refers to an imported
// name bare. The import is a value import, because the value needs the
// declaration at run time. The import specifier is the module path of the
// declaration relative to the module of the file. A declaration in no
// module, a declaration of the module of the file, and an identity
// without a name spell their own name.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Package == "" || id.Name == "" || id.Package == t.set.Home() {
		return id.Name, nil
	}
	return t.set.BindItem(relativeTo(t.set.Home(), id.Package), id.Name, false), nil
}

// Conversion spells a type assertion. TypeScript erases its types,
// so a defined type has no constructor to call, and the value is its
// inner one, asserted to the type the projection named.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return inner + " as " + typ, nil
}

// Composite spells an array literal for a list, an array, a tuple and
// the global Array and ReadonlyArray, and an object literal for every
// other type, such as a record or a map. The reference decides, and an
// entry that the literal cannot write is refused.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	if ref != nil && (ref.Form == symbol.FormList || ref.Form == symbol.FormArray || ref.Form == symbol.FormTuple ||
		ref.Form == symbol.FormNamed && ref.Target.IsZero() && ref.Package == "" &&
			(ref.Spelling == typescript.TypeArray || ref.Spelling == typescript.TypeReadonlyArray)) {

		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Name != "" || e.Key != "" {
				return "", render.RefuseValue(t.Lang(),
					"a %s value has a named entry, and an array literal takes none", typ)
			}
			parts = append(parts, e.Value)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Name != "":
			parts = append(parts, e.Name+": "+e.Value)
		case e.Key != "":
			parts = append(parts, "["+e.Key+"]: "+e.Value)
		default:
			return "", render.RefuseValue(t.Lang(),
				"a %s value has a positional element, and an object literal names every entry", typ)
		}
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

// Address refuses: TypeScript has no address operator, and a
// reference to a value is the value.
func (t target) Address(emit.Value, string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "TypeScript spells no address of a value")
}

// Construction spells a new expression, which constructs a class from its
// arguments, as new Set([1]). The class is a value at run time, so the
// construction writes its reference through [Speller.SpellValue], which
// imports the class for its value.
func (t target) Construction(ref *emit.TypeRef, args []string) (string, error) {
	class, err := NewSpeller(t.set).SpellValue(ref)
	if err != nil {
		return "", err
	}
	return "new " + class + "(" + strings.Join(args, ", ") + ")", nil
}

// Func spells an arrow function of a function type that returns the
// result, as () => 0. A function without results has an empty body, as
// () => {}. A function of two or more results returns them as a tuple,
// because the arrow type of such a function returns a tuple. Func does
// not write parameters or a type, so the file does not import anything
// for the function. An object literal that the function returns is in
// parentheses. A type that is not a function type refuses, and so does a
// constructor type, because an arrow function constructs nothing.
func (t target) Func(ref *emit.TypeRef, results []string) (string, error) {
	switch {
	case ref.Form != symbol.FormFunc:
		return "", render.RefuseValue(t.Lang(),
			"TypeScript writes an arrow function of a function type, and %s is not one", ref.Spelling)
	case strings.HasPrefix(ref.Spelling, newKeyword) || strings.HasPrefix(ref.Spelling, abstractKeyword):
		return "", render.RefuseValue(t.Lang(),
			"an arrow function constructs nothing, and %s is a constructor type", ref.Spelling)
	}
	switch len(results) {
	case 0:
		return arrowOpen + emptyBody, nil
	case 1:
		if strings.HasPrefix(results[0], objectOpen) {
			return arrowOpen + groupOpen + results[0] + groupClose, nil
		}
		return arrowOpen + results[0], nil
	default:
		return arrowOpen + tupleOpen + strings.Join(results, resultSep) + tupleClose, nil
	}
}

// Make refuses: TypeScript has no make.
func (t target) Make(_ *emit.TypeRef, typ string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "TypeScript has no make of %s", typ)
}

// Value spells one value of the value tree as TypeScript, through a
// target that imports into set what the references and the callees of
// the value name. The value builtin calls it, so a template writes a
// value as a body of the same file writes it. A value that TypeScript has
// no form for returns a [render.ValueError].
//
// Value allocates what [scaffold.Value] allocates: the text of each part
// of the value that it joins, and the relative specifier of the module of
// a callee of another module. A literal does not allocate.
func Value(v emit.Value, set *render.ImportSet) (string, error) {
	return scaffold.Value(target{set: set}, v)
}
