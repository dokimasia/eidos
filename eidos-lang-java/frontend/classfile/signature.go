// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import "strings"

// maxDepth caps the nesting of type arguments, array types and
// annotation values the reader follows, so a crafted class file cannot
// exhaust the stack.
const maxDepth = 64

// The base types' descriptor letters (§4.3.2).
const (
	baseByte    = 'B'
	baseChar    = 'C'
	baseDouble  = 'D'
	baseFloat   = 'F'
	baseInt     = 'I'
	baseLong    = 'J'
	baseShort   = 'S'
	baseBoolean = 'Z'
)

// The characters of the descriptor and signature grammars (§4.3,
// §4.7.9.1): the base types, the openers of a class type, a type
// variable and an array, the punctuation around type arguments and
// parameters, and the wildcard indicators.
const (
	baseTypes       = "BCDFIJSZ"
	classOpen       = 'L'
	varOpen         = 'T'
	arrayOpen       = '['
	typeEnd         = ';'
	argsOpen        = '<'
	argsClose       = '>'
	suffixMark      = '.'
	boundMark       = ':'
	paramsOpen      = '('
	paramsClose     = ')'
	voidResult      = 'V'
	throwsMark      = '^'
	wildcardAny     = '*'
	wildcardExtends = '+'
	wildcardSuper   = '-'
	binarySeparator = "$"
)

// The bytes an identifier ends at: each byte no identifier contains
// (§4.7.9.1), and for a class type's first name every one of them but
// the slash its package path is spelled with.
const (
	identifierStop = ".;[/<>:"
	classNameStop  = ".;[<>:"
)

// keywords maps each base type's descriptor letter to the keyword that
// names it in source.
var keywords = map[byte]string{
	baseByte: "byte", baseChar: "char", baseDouble: "double", baseFloat: "float", baseInt: "int", baseLong: "long",
	baseShort: "short", baseBoolean: "boolean",
}

// Kind is a type's form.
type Kind uint8

// The forms of a type.
const (
	// KindBase is a primitive type, named by its descriptor letter.
	KindBase Kind = 1
	// KindClass is a class or interface type.
	KindClass Kind = 2
	// KindArray is an array type.
	KindArray Kind = 3
	// KindVar is a type variable.
	KindVar Kind = 4
)

// Bound is how a type argument relates to its type (§4.7.9.1).
type Bound uint8

// The bounds of a type argument.
const (
	// BoundExact is a type argument that is its type.
	BoundExact Bound = 1
	// BoundExtends is a wildcard whose upper bound is its type, as in
	// ? extends T.
	BoundExtends Bound = 2
	// BoundSuper is a wildcard whose lower bound is its type, as in
	// ? super T.
	BoundSuper Bound = 3
	// BoundAny is the unbounded wildcard ?, which has no type.
	BoundAny Bound = 4
)

// Type is a field descriptor (§4.3.2) or a Java type signature
// (§4.7.9.1). A descriptor's class type has one name and no type
// arguments, and a descriptor has no type variable.
type Type struct {
	// Kind is the type's form, which decides the field that is set.
	Kind Kind

	// Base is a base type's descriptor letter: B, C, D, F, I, J, S or Z.
	Base byte

	// Class is a class type's names in order: the binary name in
	// internal form of the class a signature names first, then the
	// simple name of each class a suffix nests in it, each with the type
	// arguments the type states for it.
	Class []ClassName

	// Elem is an array type's component type.
	Elem *Type

	// Var is a type variable's name.
	Var string
}

// Keyword returns the keyword that names a base type in source, as int
// names I, and empty for a type of another kind. It allocates nothing.
func (t Type) Keyword() string { return keywords[t.Base] }

// BinaryName returns the binary name in internal form of the class a
// class type denotes: its names joined by $, without their type
// arguments, as §4.7.9.1 maps a class type signature to its class. It
// returns empty for a type of another kind.
//
// # Allocation contract
//
// BinaryName returns a top-level class's name without allocating, and
// writes a nested class's names into one buffer sized to the result,
// one allocation.
func (t Type) BinaryName() string {
	switch len(t.Class) {
	case 0:
		return ""
	case 1:
		return t.Class[0].Name
	}
	n := len(binarySeparator) * (len(t.Class) - 1)
	for _, c := range t.Class {
		n += len(c.Name)
	}
	var b strings.Builder
	b.Grow(n)
	for i, c := range t.Class {
		if i > 0 {
			b.WriteString(binarySeparator)
		}
		b.WriteString(c.Name)
	}
	return b.String()
}

// ClassName is one name of a class type, with the type arguments the
// type states for it.
type ClassName struct {
	Name string
	Args []TypeArg
}

// TypeArg is one type argument: its bound, and its type, which is nil
// for [BoundAny].
type TypeArg struct {
	Bound Bound
	Type  *Type
}

// TypeParam is one type parameter: its name, and its bounds, the class
// bound first where the signature states one.
type TypeParam struct {
	Name   string
	Bounds []Type
}

// ClassSignature is a class signature (§4.7.9.1): the class's type
// parameters, its superclass and its superinterfaces, with their type
// arguments.
type ClassSignature struct {
	TypeParams []TypeParam
	Super      Type
	Interfaces []Type
}

// MethodSignature is a method signature or a method descriptor: the
// method's type parameters, its parameters, its result, nil for void,
// and the types its throws clause states. A descriptor states no type
// parameters and no throws clause.
type MethodSignature struct {
	TypeParams []TypeParam
	Params     []Type
	Result     *Type
	Throws     []Type
}

// sigParser parses a descriptor or a signature: the text, the index of
// its next byte, and how many reference types the parse is inside.
type sigParser struct {
	s     string
	i     int
	depth int
}

// peek returns the next byte, and 0 at the end of the text, which no
// production starts with.
func (p *sigParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

// accept advances past the next byte where it is c, and reports
// whether it was. c is never 0, which peek returns at the end.
func (p *sigParser) accept(c byte) bool {
	if p.peek() != c {
		return false
	}
	p.i++
	return true
}

// identifier returns the text up to the first byte of stop or the end,
// and reports false for an empty identifier. Every caller then accepts
// the byte that ends the identifier, so one the text ends in fails there.
func (p *sigParser) identifier(stop string) (string, bool) {
	start := p.i
	for p.i < len(p.s) && strings.IndexByte(stop, p.s[p.i]) < 0 {
		p.i++
	}
	if p.i == start {
		return "", false
	}
	return p.s[start:p.i], true
}

// javaType parses a JavaTypeSignature: a base type or a reference type.
func (p *sigParser) javaType() (Type, bool) {
	if c := p.peek(); strings.IndexByte(baseTypes, c) >= 0 {
		p.i++
		return Type{Kind: KindBase, Base: c}, true
	}
	return p.reference()
}

// reference parses a ReferenceTypeSignature: a class type, a type
// variable or an array type, inside at most maxDepth others.
func (p *sigParser) reference() (Type, bool) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return Type{}, false
	}
	switch p.peek() {
	case classOpen:
		return p.classType()
	case varOpen:
		p.i++
		name, ok := p.identifier(identifierStop)
		if !ok || !p.accept(typeEnd) {
			return Type{}, false
		}
		return Type{Kind: KindVar, Var: name}, true
	case arrayOpen:
		p.i++
		elem, ok := p.javaType()
		if !ok {
			return Type{}, false
		}
		return Type{Kind: KindArray, Elem: &elem}, true
	default:
		return Type{}, false
	}
}

// classType parses a ClassTypeSignature: the first name, its package in
// it, then each suffix's simple name, each name with its type arguments.
func (p *sigParser) classType() (Type, bool) {
	p.i++
	var names []ClassName
	for stop := classNameStop; ; stop = identifierStop {
		name, ok := p.identifier(stop)
		if !ok {
			return Type{}, false
		}
		n := ClassName{Name: name}
		if p.peek() == argsOpen {
			if n.Args, ok = p.typeArgs(); !ok {
				return Type{}, false
			}
		}
		names = append(names, n)
		if p.accept(typeEnd) {
			return Type{Kind: KindClass, Class: names}, true
		}
		if !p.accept(suffixMark) {
			return Type{}, false
		}
	}
}

// typeArgs parses TypeArguments: one or more type arguments between
// angle brackets.
func (p *sigParser) typeArgs() ([]TypeArg, bool) {
	p.i++
	var args []TypeArg
	for !p.accept(argsClose) {
		arg := TypeArg{Bound: BoundExact}
		switch {
		case p.accept(wildcardAny):
			arg.Bound = BoundAny
		case p.accept(wildcardExtends):
			arg.Bound = BoundExtends
		case p.accept(wildcardSuper):
			arg.Bound = BoundSuper
		}
		if arg.Bound != BoundAny {
			t, ok := p.reference()
			if !ok {
				return nil, false
			}
			arg.Type = &t
		}
		args = append(args, arg)
	}
	return args, len(args) > 0
}

// typeParams parses the TypeParameters a signature may open with, and
// returns none where it states none. A class bound follows the first
// colon where the next byte opens a reference type, which is how a
// reader tells it from an interface bound or the next parameter's name.
func (p *sigParser) typeParams() ([]TypeParam, bool) {
	if !p.accept(argsOpen) {
		return nil, true
	}
	var params []TypeParam
	for !p.accept(argsClose) {
		name, ok := p.identifier(identifierStop)
		if !ok || !p.accept(boundMark) {
			return nil, false
		}
		param := TypeParam{Name: name}
		if c := p.peek(); c == classOpen || c == varOpen || c == arrayOpen {
			bound, ok := p.reference()
			if !ok {
				return nil, false
			}
			param.Bounds = append(param.Bounds, bound)
		}
		for p.accept(boundMark) {
			bound, ok := p.reference()
			if !ok {
				return nil, false
			}
			param.Bounds = append(param.Bounds, bound)
		}
		params = append(params, param)
	}
	return params, len(params) > 0
}

// parseFieldType parses a field descriptor or a field signature, and
// reports false where text is left after the type.
func parseFieldType(s string) (Type, bool) {
	p := &sigParser{s: s}
	t, ok := p.javaType()
	return t, ok && p.i == len(s)
}

// parseClassSignature parses a class signature.
func parseClassSignature(s string) (*ClassSignature, bool) {
	p := &sigParser{s: s}
	params, ok := p.typeParams()
	if !ok || p.peek() != classOpen {
		return nil, false
	}
	sig := &ClassSignature{TypeParams: params}
	if sig.Super, ok = p.reference(); !ok {
		return nil, false
	}
	for p.i < len(s) {
		if p.peek() != classOpen {
			return nil, false
		}
		t, ok := p.reference()
		if !ok {
			return nil, false
		}
		sig.Interfaces = append(sig.Interfaces, t)
	}
	return sig, true
}

// parseMethodSignature parses a method signature or a method
// descriptor, which the grammar of a signature contains.
func parseMethodSignature(s string) (*MethodSignature, bool) {
	p := &sigParser{s: s}
	params, ok := p.typeParams()
	if !ok || !p.accept(paramsOpen) {
		return nil, false
	}
	sig := &MethodSignature{TypeParams: params}
	for !p.accept(paramsClose) {
		t, ok := p.javaType()
		if !ok {
			return nil, false
		}
		sig.Params = append(sig.Params, t)
	}
	if !p.accept(voidResult) {
		t, ok := p.javaType()
		if !ok {
			return nil, false
		}
		sig.Result = &t
	}
	for p.accept(throwsMark) {
		t, ok := p.reference()
		if !ok {
			return nil, false
		}
		sig.Throws = append(sig.Throws, t)
	}
	return sig, p.i == len(s)
}
