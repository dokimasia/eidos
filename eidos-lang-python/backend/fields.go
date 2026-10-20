// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// fieldHost is the kind of class that declares a field, which decides how
// the field writes its facts.
type fieldHost uint8

const (
	// classHost is a class or a dataclass.
	classHost fieldHost = iota
	// tupleHost is a NamedTuple, whose fields are immutable without Final.
	tupleHost
	// protocolHost is a Protocol.
	protocolHost
	// dictHost is a TypedDict, whose keys are required unless NotRequired
	// states otherwise.
	dictHost
	// partialHost is a TypedDict with the keyword total=False, whose keys
	// are optional unless Required states otherwise.
	partialHost
	// enumHost is an enum, whose fields are attributes that are no members.
	enumHost
)

// partialKeyword is the keyword of a class statement of a TypedDict that
// makes each key optional unless Required states otherwise.
const partialKeyword = "total=False"

// The room that the lines of the fields reserve for the annotation of each
// field, and the qualifiers that an annotation can wrap its type in: the
// Annotated, the ClassVar, the Final and the InitVar of a field of a
// class. A longer line grows the text once more.
const (
	fieldRoom  = 48
	qualifiers = 4
)

// fields writes the lines of the fields fs of a class of the kind host, one
// level inside the class, each with its trailing comment and its
// docstring after it, and nothing for no fields.
func (w *Writer) fields(fs []*emit.Field, host fieldHost) (string, error) {
	size := 0
	for _, f := range fs {
		size += fieldRoom + len(f.Name) + len(f.Value) + len(f.Comment) + docstringSize(f.Doc, python.Indent)
	}
	var b strings.Builder
	b.Grow(size)
	for _, f := range fs {
		if err := w.field(&b, f, host); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// field writes one field into b:
//
//   - a field with a type is name: type, with its value after an equals
//     sign. A stub file writes ... for the value;
//   - a field without a type is an attribute of the class, name = value, and
//     a field without a type and without a value has the type of a dynamic
//     value. A field of a NamedTuple without a type has the type of a
//     dynamic value, because an attribute without an annotation is no field
//     of a NamedTuple;
//   - a field that a value of a class may lack has None among its types,
//     and None as its value where it states none;
//   - an immutable field of a Protocol that belongs to instances is a
//     read-only property, because a Protocol attribute is one that a holder
//     can assign;
//   - a field of an enum is an attribute that is no member. A field of the
//     members, at the level of an instance, is name: type without a value,
//     because an enum sets the attributes of its members in __init__. A
//     field of the enum, at the level of the type, without a value is
//     name: type, and with a value it is the assignment
//     name = nonmember(cast("type", value)), because a value in the body of
//     an enum declares a member, and a type checker refuses an annotation of
//     the assignment. A stub file writes every field of an enum without a
//     value, because a type checker reads a value of a stub as a member.
//
// Error modes: a name that is no identifier, a field of a NamedTuple whose
// name starts with an underscore, which NamedTuple refuses, a key of a
// TypedDict with a value, a field of the members of an enum with a value,
// and the refusals of the types.
func (w *Writer) field(b *strings.Builder, f *emit.Field, host fieldHost) error {
	dict := host == dictHost || host == partialHost
	members := host == enumHost && f.Level != symbol.LevelType
	switch {
	case !spell.IsIdentifier(f.Name):
		return refuse("a field of a class has an identifier as its name, and %q is not one", f.Name)
	case host == tupleHost && strings.HasPrefix(f.Name, discardName):
		return refuse("NamedTuple refuses a field whose name starts with an underscore, and %s has one", f.Name)
	case dict && f.Value != "":
		return refuse("a key of a TypedDict takes no value, and %s states one", f.Name)
	case members && f.Value != "":
		return refuse("an enum sets the attributes of its members in __init__, and %s states a value in the class",
			f.Name)
	case host == protocolHost && f.Mutability == symbol.MutabilityImmutable && f.Level != symbol.LevelType:
		return w.property(b, f)
	}
	b.WriteString(python.Indent)
	b.WriteString(f.Name)
	value := f.Value
	if f.Optional && !dict && !members && value == "" {
		value = noneName
	}
	if host == enumHost && w.stub {
		value = ""
	}
	nonmember := host == enumHost && value != ""
	switch {
	case nonmember:
		if err := w.nonmember(b, f, value); err != nil {
			return err
		}
	case f.Type != nil:
		b.WriteString(annotationSep)
		if err := w.writeFieldType(b, f, host); err != nil {
			return err
		}
	case value == "" || host == tupleHost:
		b.WriteString(annotationSep)
		b.WriteString(w.dynamic())
	}
	if value != "" && !nonmember {
		if w.stub {
			value = emptyBody
		}
		b.WriteString(assign)
		b.WriteString(value)
	}
	writeComment(b, f.Comment)
	b.WriteString(newline)
	writeDocstring(b, f.Doc, python.Indent)
	return nil
}

// nonmember writes the value of a field of an enum into b: the assignment
// of nonmember(cast("type", value)) for a field with a type, with None among
// the types of a field that a value may lack, and nonmember(value) for a
// field without a type. cast states the type of the attribute to a type
// checker, and the string keeps Python from evaluating the type.
//
// Error modes: the refusals of the type.
func (w *Writer) nonmember(b *strings.Builder, f *emit.Field, value string) error {
	b.WriteString(assign)
	b.WriteString(w.enumItem(nonmemberName))
	b.WriteString(callOpen)
	if f.Type == nil {
		b.WriteString(value)
		b.WriteString(callClose)
		return nil
	}
	text, err := w.spellLazy(f.Type)
	if err != nil {
		return err
	}
	if f.Optional && f.Type.Form != symbol.FormOptional {
		text += unionSep + noneName
	}
	b.WriteString(w.typing(castName))
	b.WriteString(callOpen)
	b.WriteString(python.Quote(text))
	b.WriteString(argSep)
	b.WriteString(value)
	b.WriteString(callClose + callClose)
	return nil
}

// property writes an immutable field of a Protocol as a read-only property:
// the decorator property, which a member of the Protocol of that name hides,
// and a method that returns the type of the field.
func (w *Writer) property(b *strings.Builder, f *emit.Field) error {
	b.WriteString(python.Indent + decoratorAt)
	b.WriteString(w.builtin(propertyDecorator, propertyDecorator))
	b.WriteString(newline + python.Indent + defKeyword)
	b.WriteString(f.Name)
	b.WriteString(callOpen + selfName + callClose + resultArrow)
	if f.Type == nil {
		b.WriteString(w.dynamic())
	} else if err := w.writeAnnotation(b, f.Type, f.Annotations, f.Optional); err != nil {
		return err
	}
	if len(f.Doc) == 0 {
		b.WriteString(inlineBody)
		writeComment(b, f.Comment)
		b.WriteString(newline)
		return nil
	}
	b.WriteString(bodyOpener)
	writeComment(b, f.Comment)
	b.WriteString(newline)
	writeDocstring(b, f.Doc, python.Indent+python.Indent)
	b.WriteString(python.Indent + python.Indent + emptyBody + newline)
	return nil
}

// fieldType writes the annotation of a field of a class of the kind host as
// [Writer.writeFieldType] writes it, as a text of its own.
func (w *Writer) fieldType(f *emit.Field, host fieldHost) (string, error) {
	var b strings.Builder
	if err := w.writeFieldType(&b, f, host); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeFieldType writes the annotation of a field of a class of the kind
// host into b:
//
//   - in a TypedDict, NotRequired of the type of a key that a value may
//     lack, and ReadOnly of the type of an immutable key. Under the keyword
//     total=False, where every key is optional unless it states otherwise, a
//     key that a value may lack is the type alone, and Required wraps the
//     type of a key that a value has;
//   - in a class, the type with None among its types where a value may
//     lack the field, ClassVar of the type of a field of the type, Final of
//     the type of an immutable field, and ClassVar of Final from Python
//     3.13 for an immutable field of the type, which Final alone states
//     before. A NamedTuple writes no Final, because its fields are
//     immutable;
//   - InitVar of the type of a field with the InitVar marker, and Annotated
//     of the whole annotation with the metadata of the annotated marker.
func (w *Writer) writeFieldType(b *strings.Builder, f *emit.Field, host fieldHost) error {
	text, err := w.Spell(f.Type)
	if err != nil {
		return err
	}
	var buf [qualifiers]string
	wraps := buf[:0]
	metadata, annotated := markerArgs(f.Annotations, python.AnnotatedMarker)
	if annotated {
		wraps = append(wraps, w.typing(annotatedName))
	}
	immutable := f.Mutability == symbol.MutabilityImmutable
	none := false
	if host == dictHost || host == partialHost {
		if immutable {
			wraps = append(wraps, w.typing(readOnlyName))
		}
		switch {
		case host == dictHost && f.Optional:
			wraps = append(wraps, w.typing(notRequiredName))
		case host == partialHost && !f.Optional:
			wraps = append(wraps, w.typing(requiredName))
		}
	} else {
		if f.Level == symbol.LevelType && (!immutable || minorOf(w.set) >= minor313) {
			wraps = append(wraps, w.typing(classVarName))
		}
		if immutable && host != tupleHost {
			wraps = append(wraps, w.typing(finalName))
		}
		if _, initVar := markerArgs(f.Annotations, python.InitVarMarker); initVar {
			wraps = append(wraps, w.set.BindItem(dataModule, initVarName, false))
		}
		none = f.Optional && f.Type.Form != symbol.FormOptional
	}
	for _, q := range wraps {
		b.WriteString(q)
		b.WriteString(argsOpen)
	}
	b.WriteString(text)
	if none {
		b.WriteString(unionSep)
		b.WriteString(noneName)
	}
	for i := range slices.Backward(wraps) {
		if annotated && i == 0 {
			writeMetadata(b, metadata)
			continue
		}
		b.WriteString(argsClose)
	}
	return nil
}
