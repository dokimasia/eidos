// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import "strings"

// The tags of an annotation's element values (§4.7.16.1).
const (
	valueByte       = 'B'
	valueChar       = 'C'
	valueDouble     = 'D'
	valueFloat      = 'F'
	valueInt        = 'I'
	valueLong       = 'J'
	valueShort      = 'S'
	valueBoolean    = 'Z'
	valueString     = 's'
	valueEnum       = 'e'
	valueClass      = 'c'
	valueAnnotation = '@'
	valueArray      = '['
)

// The spellings of an element value in source: an annotation's mark and
// the parentheses around its pairs, the separator of a pair's name and
// value, the separator of list items, the braces of an array, the suffix
// of a class literal and of an array type, the name of void, and the
// separator of a qualified name and of a binary name's packages.
const (
	annotationMark   = "@"
	pairsOpen        = "("
	pairsClose       = ")"
	pairSeparator    = " = "
	itemSeparator    = ", "
	arrayValueOpen   = "{"
	arrayValueClose  = "}"
	classLiteral     = ".class"
	arraySuffix      = "[]"
	voidName         = "void"
	qualifier        = "."
	packageSeparator = "/"
)

// Annotation is one annotation on a declaration (§4.7.16).
type Annotation struct {
	// Type is the annotation interface's binary name in internal form.
	Type string

	// Elements are the element-value pairs the annotation states, in
	// class-file order. An element the annotation leaves at its default
	// is absent.
	Elements []Element
}

// Element is one element-value pair of an annotation: the element's
// name, and its value as Java source spells it. A primitive or a
// string is a literal, an enum constant and a class literal name their
// type by its binary name in dotted form, a nested annotation is spelled
// with its pairs, and an array between braces.
type Element struct {
	Name  string
	Value string
}

// annotations reads a count of annotations and the annotations after it.
func (r *reader) annotations() []Annotation {
	var out []Annotation
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		out = append(out, r.annotation(0))
	}
	return out
}

// parameterAnnotations reads a count of parameters and each parameter's
// annotations after it (§4.7.18).
func (r *reader) parameterAnnotations() [][]Annotation {
	var out [][]Annotation
	for n := r.u1(); n > 0 && r.err == nil; n-- {
		out = append(out, r.annotations())
	}
	return out
}

// annotation reads one annotation inside depth element values.
func (r *reader) annotation(depth int) Annotation {
	a := Annotation{Type: r.fieldType(r.u2()).BinaryName()}
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		name := r.utf8(r.u2())
		a.Elements = append(a.Elements, Element{Name: name, Value: r.elementValue(depth + 1)})
	}
	return a
}

// elementValue reads one element value at a depth, and fails the reader
// past maxDepth and for an unknown tag.
func (r *reader) elementValue(depth int) string {
	if depth > maxDepth {
		r.fail("element values nest deeper than %d", maxDepth)
		return ""
	}
	switch tag := r.u1(); tag {
	case valueByte, valueShort, valueInt:
		return intLiteral(r.integer(r.u2()))
	case valueChar:
		return charLiteral(r.integer(r.u2()))
	case valueBoolean:
		return boolLiteral(r.integer(r.u2()))
	case valueLong:
		return longLiteral(r.long(r.u2()))
	case valueFloat:
		return floatLiteral(r.float(r.u2()))
	case valueDouble:
		return doubleLiteral(r.double(r.u2()))
	case valueString:
		return stringLiteral(r.units(r.u2()))
	case valueEnum:
		t := r.fieldType(r.u2())
		return spell(t) + qualifier + r.utf8(r.u2())
	case valueClass:
		return r.classValue(r.u2()) + classLiteral
	case valueAnnotation:
		return spellAnnotation(r.annotation(depth))
	case valueArray:
		var items []string
		for n := r.u2(); n > 0 && r.err == nil; n-- {
			items = append(items, r.elementValue(depth+1))
		}
		return arrayValueOpen + strings.Join(items, itemSeparator) + arrayValueClose
	default:
		r.fail("an element value has the unknown tag %d", tag)
		return ""
	}
}

// classValue returns the type a class literal names: the return
// descriptor the Utf8 constant at an index spells, void or a field type.
func (r *reader) classValue(i uint16) string {
	if r.utf8(i) == string(voidResult) {
		return voidName
	}
	return spell(r.fieldType(i))
}

// spell returns a descriptor's type as source names it: a base type's
// keyword, a class's binary name in dotted form, and an array as its
// component type with brackets.
func spell(t Type) string {
	switch t.Kind {
	case KindBase:
		return t.Keyword()
	case KindArray:
		return spell(*t.Elem) + arraySuffix
	default:
		return strings.ReplaceAll(t.BinaryName(), packageSeparator, qualifier)
	}
}

// spellAnnotation returns a nested annotation as source spells it: its
// type, and its pairs between parentheses where it states any.
func spellAnnotation(a Annotation) string {
	name := annotationMark + strings.ReplaceAll(a.Type, packageSeparator, qualifier)
	if len(a.Elements) == 0 {
		return name
	}
	pairs := make([]string, 0, len(a.Elements))
	for _, e := range a.Elements {
		pairs = append(pairs, e.Name+pairSeparator+e.Value)
	}
	return name + pairsOpen + strings.Join(pairs, itemSeparator) + pairsClose
}
