// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package classfile decodes the parts of a Java class file that a
// declaration's signature needs, as chapter 4 of the Java Virtual Machine
// Specification for Java SE 25 lays them out.
//
// [Parse] decodes one class file into a [Class]: its version, the
// constant pool, the access flags, the class and its supertypes, the
// fields and the methods, and the attributes a signature reads. It
// decodes nothing that only executing the class needs: code, stack maps,
// line numbers and local variables are skipped by their length, as the
// specification requires of a reader of an attribute it does not
// recognise (§4.7.1). The JDK's ct.sym entries are class files too, and
// decode the same way.
//
// # Attributes
//
// The reader decodes Signature, InnerClasses, Record, PermittedSubclasses,
// Exceptions, MethodParameters and ConstantValue, and the run-time visible
// and invisible annotations and parameter annotations. A run-time visible annotation precedes an invisible one
// in every list. AnnotationDefault and Deprecated are skipped, because
// no declaration the Java frontend builds has a place for either.
//
// # Names and types
//
// A class is named by its binary name in internal form, as in
// java/util/Map$Entry, and a member by its unqualified name. Names
// decode from modified UTF-8 (§4.4.7). A [Type] is a field descriptor
// (§4.3.2) or a type signature (§4.7.9.1): a base type, a class type
// with the type arguments of each of its names, an array or a type
// variable. [Type.BinaryName] maps a class type to the class it denotes
// by the rule §4.7.9.1 states. A constant value and an annotation's
// element values are spelled as Java source spells them.
//
// # Bounds
//
// Every index and length is checked against the bytes and the constant
// pool, and the nesting of type arguments, array types and annotation
// values is capped at 64 levels. A class file that breaks the format
// returns an error wrapping [ErrMalformed] and no Class, and so does a
// class file with bytes after its attributes (§4.8).
//
// # Dependency position
//
// lang/java/frontend/classfile imports the Go stdlib alone. The Java
// frontend imports it.
package classfile
