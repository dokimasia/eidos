// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import (
	"errors"
	"slices"
)

// ErrMalformed reports a class file that breaks the format of chapter 4:
// a wrong magic number, an index or a length out of range, a constant of
// the wrong tag, a descriptor or a signature that does not parse, nesting
// past 64 levels, or bytes after the attributes. The error that wraps it
// names the fault.
var ErrMalformed = errors.New("classfile: malformed class file")

// magic opens every class file (§4.1).
const magic = 0xCAFEBABE

// stringClass is the binary name of String, the one class a
// ConstantValue attribute's field can have.
const stringClass = "java/lang/String"

// The attributes the reader decodes (§4.7).
const (
	attrSignature                     = "Signature"
	attrInnerClasses                  = "InnerClasses"
	attrRecord                        = "Record"
	attrPermittedSubclasses           = "PermittedSubclasses"
	attrExceptions                    = "Exceptions"
	attrMethodParameters              = "MethodParameters"
	attrConstantValue                 = "ConstantValue"
	attrVisibleAnnotations            = "RuntimeVisibleAnnotations"
	attrInvisibleAnnotations          = "RuntimeInvisibleAnnotations"
	attrVisibleParameterAnnotations   = "RuntimeVisibleParameterAnnotations"
	attrInvisibleParameterAnnotations = "RuntimeInvisibleParameterAnnotations"
)

// Class is one decoded class file (§4.1).
type Class struct {
	// Major and Minor are the class file's version.
	Major, Minor uint16

	// Access is the class's access flags. A nested class's own flags
	// are in the InnerClasses entry that names it.
	Access Access

	// Record reports a Record attribute, whose components are
	// Components.
	Record bool

	// Name is the class's binary name in internal form, as in
	// java/util/Map$Entry, and module-info for a module declaration.
	Name string

	// Super is the direct superclass's binary name, and empty for
	// java/lang/Object and a module declaration.
	Super string

	// Interfaces are the direct superinterfaces' binary names, in
	// declaration order.
	Interfaces []string

	// Signature is the class signature, and nil for a class whose
	// declaration uses no type variable and no parameterized type.
	Signature *ClassSignature

	// Fields and Methods are the class's own members, synthetic ones
	// and bridges included, in class-file order.
	Fields  []Field
	Methods []Method

	// Inner is the InnerClasses attribute: every nested class the class
	// declares or names, each with its declaring class and its own flags.
	// A nested class has an entry for itself, and a local or an anonymous
	// class's entry names no declaring class.
	Inner []InnerClass

	// Components are the components of a Record attribute, and nil for
	// a class that is not a record.
	Components []Component

	// Permitted are the binary names a PermittedSubclasses attribute
	// lists, and nil for a class that is not sealed.
	Permitted []string

	// Annotations are the class's run-time visible annotations, then its
	// invisible ones.
	Annotations []Annotation
}

// Field is one field (§4.5).
type Field struct {
	Access Access
	Name   string

	// Type is the field's descriptor type, and Signature its type with
	// type arguments, nil where the field's type uses none.
	Type      Type
	Signature *Type

	// Value is the ConstantValue attribute's constant as a Java literal,
	// and empty without one.
	Value string

	Annotations []Annotation
}

// Method is one method or constructor (§4.6): a constructor is named
// <init>, and a class initializer <clinit>.
type Method struct {
	Access Access
	Name   string

	// Params and Result are the method descriptor's types, Result nil
	// for void. A constructor of an inner class states its enclosing
	// instance first.
	Params []Type
	Result *Type

	// Signature is the method signature, and nil where the method's
	// types use no type variable and no parameterized type. Its
	// parameters can omit a descriptor's implicit leading ones (§4.7.9.1).
	Signature *MethodSignature

	// Exceptions are the binary names the Exceptions attribute lists.
	Exceptions []string

	// Parameters is the MethodParameters attribute, one entry per
	// descriptor parameter, and nil without one.
	Parameters []Parameter

	// ParamAnnotations are the parameter annotations, run-time visible
	// then invisible per parameter, and nil without either attribute.
	// Entry i annotates the descriptor's parameter
	// len(Params)-len(ParamAnnotations)+i, because a compiler may leave an
	// implicit leading parameter out (§4.7.18).
	ParamAnnotations [][]Annotation

	Annotations []Annotation
}

// Parameter is one entry of a MethodParameters attribute (§4.7.24): the
// parameter's name, empty where it has none, and its flags: final,
// synthetic or mandated.
type Parameter struct {
	Name   string
	Access Access
}

// Component is one record component (§4.7.30).
type Component struct {
	Name string

	// Type is the component's descriptor type, and Signature its type
	// with type arguments, nil where the component's type uses none.
	Type      Type
	Signature *Type

	Annotations []Annotation
}

// InnerClass is one entry of an InnerClasses attribute (§4.7.6).
type InnerClass struct {
	// Inner is the nested class's binary name.
	Inner string

	// Outer is the declaring class's binary name, and empty for a local
	// or an anonymous class.
	Outer string

	// Name is the nested class's simple name, and empty for an
	// anonymous class.
	Name string

	// Access is the flags the nested class's declaration states, which
	// its own class file cannot: private, protected and static among
	// them.
	Access Access
}

// Parse decodes one class file. It returns an error wrapping
// [ErrMalformed] for a class file that breaks the format, and no Class.
// The returned Class shares no memory with data.
func Parse(data []byte) (*Class, error) {
	r := &reader{data: data}
	if m := r.u4(); m != magic {
		r.fail("the magic number is %#x", m)
	}
	c := &Class{}
	c.Minor = r.u2()
	c.Major = r.u2()
	r.readPool()
	c.Access = Access(r.u2())
	c.Name = r.class(r.u2())
	c.Super = r.optionalClass(r.u2())
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		c.Interfaces = append(c.Interfaces, r.class(r.u2()))
	}
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		c.Fields = append(c.Fields, r.field())
	}
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		c.Methods = append(c.Methods, r.method())
	}
	var visible, invisible []Annotation
	r.attributes(func(name string, b *reader) bool {
		switch name {
		case attrSignature:
			c.Signature = b.classSignature(b.u2())
		case attrInnerClasses:
			for n := b.u2(); n > 0 && b.err == nil; n-- {
				c.Inner = append(c.Inner, b.innerClass())
			}
		case attrRecord:
			c.Record = true
			for n := b.u2(); n > 0 && b.err == nil; n-- {
				c.Components = append(c.Components, b.component())
			}
		case attrPermittedSubclasses:
			c.Permitted = b.classes()
		case attrVisibleAnnotations:
			visible = b.annotations()
		case attrInvisibleAnnotations:
			invisible = b.annotations()
		default:
			return false
		}
		return true
	})
	c.Annotations = slices.Concat(visible, invisible)
	if r.err == nil && len(r.data) > 0 {
		r.fail("%d bytes follow the attributes", len(r.data))
	}
	if r.err != nil {
		return nil, r.err
	}
	return c, nil
}

// attributes reads an attributes table (§4.7). It hands decode each
// attribute's name and a reader over that attribute's bytes alone, which
// shares the pool, and decode reports whether it decoded the attribute.
// The reader skips an attribute decode leaves undecoded by its length,
// and fails for one whose decode failed or left bytes unread.
func (r *reader) attributes(decode func(name string, body *reader) bool) {
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		name := r.utf8(r.u2())
		data, _ := r.take(r.u4())
		if r.err != nil {
			return
		}
		body := &reader{data: data, pool: r.pool}
		if !decode(name, body) {
			continue
		}
		if body.err == nil && len(body.data) > 0 {
			body.fail("the %s attribute has %d bytes past its items", name, len(body.data))
		}
		r.err = body.err
	}
}

// field reads one field_info structure.
func (r *reader) field() Field {
	f := Field{Access: Access(r.u2()), Name: r.utf8(r.u2()), Type: r.fieldType(r.u2())}
	var visible, invisible []Annotation
	r.attributes(func(name string, b *reader) bool {
		switch name {
		case attrSignature:
			t := b.fieldType(b.u2())
			f.Signature = &t
		case attrConstantValue:
			f.Value = b.constant(b.u2(), f.Type)
		case attrVisibleAnnotations:
			visible = b.annotations()
		case attrInvisibleAnnotations:
			invisible = b.annotations()
		default:
			return false
		}
		return true
	})
	f.Annotations = slices.Concat(visible, invisible)
	return f
}

// method reads one method_info structure.
func (r *reader) method() Method {
	m := Method{Access: Access(r.u2()), Name: r.utf8(r.u2())}
	if d := r.methodSignature(r.u2()); d != nil {
		m.Params, m.Result = d.Params, d.Result
	}
	var visible, invisible []Annotation
	var visibleParams, invisibleParams [][]Annotation
	r.attributes(func(name string, b *reader) bool {
		switch name {
		case attrSignature:
			m.Signature = b.methodSignature(b.u2())
		case attrExceptions:
			m.Exceptions = b.classes()
		case attrMethodParameters:
			for n := b.u1(); n > 0 && b.err == nil; n-- {
				m.Parameters = append(m.Parameters, Parameter{Name: b.optionalUtf8(b.u2()), Access: Access(b.u2())})
			}
		case attrVisibleAnnotations:
			visible = b.annotations()
		case attrInvisibleAnnotations:
			invisible = b.annotations()
		case attrVisibleParameterAnnotations:
			visibleParams = b.parameterAnnotations()
		case attrInvisibleParameterAnnotations:
			invisibleParams = b.parameterAnnotations()
		default:
			return false
		}
		return true
	})
	m.Annotations = slices.Concat(visible, invisible)
	m.ParamAnnotations = alignedFromEnd(visibleParams, invisibleParams)
	return m
}

// component reads one record_component_info structure.
func (r *reader) component() Component {
	c := Component{Name: r.utf8(r.u2()), Type: r.fieldType(r.u2())}
	var visible, invisible []Annotation
	r.attributes(func(name string, b *reader) bool {
		switch name {
		case attrSignature:
			t := b.fieldType(b.u2())
			c.Signature = &t
		case attrVisibleAnnotations:
			visible = b.annotations()
		case attrInvisibleAnnotations:
			invisible = b.annotations()
		default:
			return false
		}
		return true
	})
	c.Annotations = slices.Concat(visible, invisible)
	return c
}

// innerClass reads one entry of an InnerClasses attribute.
func (r *reader) innerClass() InnerClass {
	return InnerClass{
		Inner: r.class(r.u2()), Outer: r.optionalClass(r.u2()), Name: r.optionalUtf8(r.u2()), Access: Access(r.u2()),
	}
}

// classes reads a count of Class constant indexes and the binary names
// they name.
func (r *reader) classes() []string {
	var out []string
	for n := r.u2(); n > 0 && r.err == nil; n-- {
		out = append(out, r.class(r.u2()))
	}
	return out
}

// fieldType returns the field descriptor or the field signature the
// Utf8 constant at an index spells, and fails the reader for one that
// does not parse.
func (r *reader) fieldType(i uint16) Type {
	s := r.utf8(i)
	t, ok := parseFieldType(s)
	if !ok {
		r.fail("the type %q does not parse", s)
	}
	return t
}

// classSignature returns the class signature the Utf8 constant at an
// index spells, and fails the reader for one that does not parse.
func (r *reader) classSignature(i uint16) *ClassSignature {
	s := r.utf8(i)
	sig, ok := parseClassSignature(s)
	if !ok {
		r.fail("the class signature %q does not parse", s)
	}
	return sig
}

// methodSignature returns the method signature or descriptor the Utf8
// constant at an index spells, and fails the reader for one that does
// not parse.
func (r *reader) methodSignature(i uint16) *MethodSignature {
	s := r.utf8(i)
	sig, ok := parseMethodSignature(s)
	if !ok {
		r.fail("the method type %q does not parse", s)
		return nil
	}
	return sig
}

// constant returns the constant at an index spelled as a literal of a
// field's type, and fails the reader for a field of a type no constant
// can initialize.
func (r *reader) constant(i uint16, t Type) string {
	switch {
	case t.BinaryName() == stringClass:
		return stringLiteral(r.str(i))
	case t.Kind != KindBase:
		r.fail("a ConstantValue initializes a field of the type %s", t.BinaryName())
		return ""
	}
	switch t.Base {
	case baseLong:
		return longLiteral(r.long(i))
	case baseFloat:
		return floatLiteral(r.float(i))
	case baseDouble:
		return doubleLiteral(r.double(i))
	case baseBoolean:
		return boolLiteral(r.integer(i))
	case baseChar:
		return charLiteral(r.integer(i))
	default:
		return intLiteral(r.integer(i))
	}
}

// alignedFromEnd merges two lists of parameter annotations whose lengths
// may differ, each aligned to the last parameters, the first list's
// annotations first in each entry.
func alignedFromEnd(first, second [][]Annotation) [][]Annotation {
	n := max(len(first), len(second))
	if n == 0 {
		return nil
	}
	out := make([][]Annotation, n)
	for i, a := range first {
		out[n-len(first)+i] = append(out[n-len(first)+i], a...)
	}
	for i, a := range second {
		out[n-len(second)+i] = append(out[n-len(second)+i], a...)
	}
	return out
}
