// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tokens of the compact spelling that [Language] writes: the tokens of
// the frontend's spelling without whitespace, with one space between two
// identifier tokens.
const (
	listMark     = "[]"
	unionSep     = "|"
	intersectSep = "&"
	tupleOpen    = "["
	tupleClose   = "]"
	itemSep      = ","
	groupOpen    = "("
	groupClose   = ")"
	arrow        = "=>"
	paramPrefix  = "p"
	typeMark     = ":"
	indexOpen    = "{[key:"
	readonlyOpen = "{readonly[key:"
	indexClose   = "]:"
	bodyOpen     = "{"
	bodyClose    = "}"
	memberSep    = ";"
	optionalMark = "?"
	readonlyWord = "readonly "
	readonlyMark = "readonly"
	argsOpen     = "<"
	argsClose    = ">"
)

// errOneResult is the refusal of a function type without exactly one
// result, and of a method of an inline body with more than one. The syntax
// of a TypeScript function type requires exactly one return type.
var errOneResult = errors.New("testing: a function type of TypeScript has exactly one return type")

// Language is the spelling adapter of TypeScript for a fixture of
// nodetest. It writes the compact spelling that the TypeScript frontend
// records: the tokens of the source without whitespace, with one space
// between two identifier tokens. It puts a union, an intersection, an
// optional and a function type in parentheses where such a type is a child
// of a list, an optional, a union or an intersection. A reference to a type
// of another package has the name of the type alone, as an import binds
// it. TypeScript overloads.
//
// # Concurrency
//
// A Language is safe for concurrent use, because it has no state.
type Language struct{}

// Language is the spelling adapter of TypeScript.
var _ nodetest.Language = Language{}

// Lang returns the identity of TypeScript.
func (Language) Lang() symbol.Lang { return typescript.Lang }

// Overloads reports true, because TypeScript declares overloads of one
// function.
func (Language) Overloads() bool { return true }

// Spell writes ref in the compact spelling of TypeScript:
//
//   - T[] for a list, [A,B] for a tuple, A|B for a union and A&B for an
//     intersection.
//   - T|undefined for an optional without absent members, and T followed
//     by its absent members for any other optional, such as T|null.
//   - readonly T[] and readonly[A,B] for an immutable list and tuple. An
//     immutable list or tuple inside a list is in parentheses.
//   - (p0:A,p1:B)=>R for a function type, whose parameters are named by
//     their position.
//   - {[key:K]:V} for a map and {readonly[key:K]:V} for an immutable map,
//     and {a:T;b?:U;readonly c:V;m(p0:A):R} for an inline body.
//   - Map<K,V> for a child with type arguments.
//
// Error modes: a function type without exactly one result and a method of
// an inline body with more than one result return errOneResult, because
// the syntax requires one return type. An array, a stream, a borrow, a
// wildcard and every form that only the fold returns return an error,
// because TypeScript has no spelling for them.
func (Language) Spell(ref *node.TypeRef) (string, error) {
	children := make([]string, 0, len(ref.Elems))
	for _, elem := range ref.Elems {
		children = append(children, spelled(elem))
	}
	switch ref.Form {
	case symbol.FormList:
		elem := grouped(ref.Elems, children, itemSep)
		if len(ref.Elems) == 1 && ref.Elems[0] != nil && ref.Elems[0].Mutability == symbol.MutabilityImmutable &&
			(ref.Elems[0].Form == symbol.FormList || ref.Elems[0].Form == symbol.FormTuple) {

			elem = groupOpen + elem + groupClose
		}
		return readonly(ref, elem+listMark), nil
	case symbol.FormOptional:
		absent := string(typescript.TypeUndefined)
		if len(ref.Absent) > 0 {
			spellings := make([]string, 0, len(ref.Absent))
			for _, a := range ref.Absent {
				spellings = append(spellings, spelled(a))
			}
			absent = strings.Join(spellings, unionSep)
		}
		return grouped(ref.Elems, children, itemSep) + unionSep + absent, nil
	case symbol.FormTuple:
		return readonly(ref, tupleOpen+strings.Join(children, itemSep)+tupleClose), nil
	case symbol.FormUnion:
		return grouped(ref.Elems, children, unionSep), nil
	case symbol.FormIntersection:
		return grouped(ref.Elems, children, intersectSep), nil
	case symbol.FormFunc:
		split := min(max(ref.Split, 0), len(children))
		if len(children)-split != 1 {
			return "", errOneResult
		}
		return signature(children[:split]) + arrow + children[split], nil
	case symbol.FormMap:
		if len(children) != 2 {
			return "", fmt.Errorf("testing: a map of TypeScript has a key and a value, and this one has %d types",
				len(children))
		}
		open := indexOpen
		if ref.Mutability == symbol.MutabilityImmutable {
			open = readonlyOpen
		}
		return open + children[0] + indexClose + children[1] + bodyClose, nil
	case symbol.FormInline:
		return inline(ref)
	default:
		return "", fmt.Errorf("testing: TypeScript has no spelling of the form %d", ref.Form)
	}
}

// Qualify returns name, which an import of TypeScript binds in the file
// without the name of its module.
func (Language) Qualify(_ *node.Package, name string) string { return name }

// grouped joins the spellings of the children with sep, and puts each child
// that is a union, an intersection, an optional or a function type in
// parentheses, so the frontend reads the child as one member.
func grouped(elems []*node.TypeRef, children []string, sep string) string {
	parts := make([]string, 0, len(children))
	for i, child := range children {
		switch elems[i].Form {
		case symbol.FormUnion, symbol.FormIntersection, symbol.FormOptional, symbol.FormFunc:
			child = groupOpen + child + groupClose
		default:
			// Any other form is one member of the parent already.
		}
		parts = append(parts, child)
	}
	return strings.Join(parts, sep)
}

// readonly returns text after readonly where ref is immutable, and text
// alone otherwise. The compact spelling has a space between readonly and
// text only where text starts with a character of an identifier.
func readonly(ref *node.TypeRef, text string) string {
	if ref.Mutability != symbol.MutabilityImmutable {
		return text
	}
	if r, _ := utf8.DecodeRuneInString(text); typescript.IsIdentifierPart(r) {
		return readonlyWord + text
	}
	return readonlyMark + text
}

// signature writes the parameter list of a function type or a method, each
// parameter named by its position.
func signature(params []string) string {
	named := make([]string, 0, len(params))
	for i, p := range params {
		named = append(named, paramPrefix+strconv.Itoa(i)+typeMark+p)
	}
	return groupOpen + strings.Join(named, itemSep) + groupClose
}

// inline writes an inline body: each field with its readonly and its
// optional mark, and each method with its parameters and its one result.
func inline(ref *node.TypeRef) (string, error) {
	members := make([]string, 0, len(ref.Fields)+len(ref.Methods))
	for _, f := range ref.Fields {
		member := f.Name
		if f.Optional {
			member += optionalMark
		}
		if f.Mutability == symbol.MutabilityImmutable {
			member = readonlyWord + member
		}
		members = append(members, member+typeMark+spelled(f.Type))
	}
	for _, m := range ref.Methods {
		params := make([]string, 0, len(m.Params))
		for _, p := range m.Params {
			params = append(params, spelled(p.Type))
		}
		member := m.Name + signature(params)
		switch len(m.Returns) {
		case 0:
			// A method signature of an object type may leave its return
			// type unstated.
		case 1:
			member += typeMark + spelled(m.Returns[0].Type)
		default:
			return "", errOneResult
		}
		members = append(members, member)
	}
	return bodyOpen + strings.Join(members, memberSep) + bodyClose, nil
}

// spelled returns the spelling of a child with its type arguments in angle
// brackets, and the empty string for a nil reference.
func spelled(ref *node.TypeRef) string {
	if ref == nil {
		return ""
	}
	if len(ref.Args) == 0 {
		return ref.Spelling
	}
	args := make([]string, 0, len(ref.Args))
	for _, arg := range ref.Args {
		args = append(args, spelled(arg))
	}
	return ref.Spelling + argsOpen + strings.Join(args, itemSep) + argsClose
}
