// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tokens of the compact spelling that [Language] writes: the tokens of
// the frontend's spelling without whitespace, with one space between two
// identifier tokens.
const (
	borrowMark  = "&"
	mutWord     = "mut "
	mutMark     = "mut"
	sliceOpen   = "["
	sliceClose  = "]"
	lengthSep   = ";"
	parenOpen   = "("
	parenClose  = ")"
	itemSep     = ","
	fnOpen      = "fn("
	resultArrow = "->"
	dynWord     = "dyn "
	boundSep    = "+"
	argsOpen    = "<"
	argsClose   = ">"
	pathSep     = "::"
)

// The refusals of two forms that Rust does not spell. The Rust frontend
// records the unit type () as a named type, so a tuple has at least one
// member. A function type has one return type at most.
var (
	errUnitTuple   = errors.New("testing: a tuple of Rust has at least one member")
	errManyResults = errors.New("testing: a function type of Rust has one return type at most")
)

// Language is the spelling adapter of Rust for a fixture of nodetest. It
// writes the compact spelling that the Rust frontend records: &T for a
// borrow and &mut T for a mutable one, [T;3] for an array, [T] for a
// slice, (A,B) for a tuple, fn(A,B)->R for a function pointer, and dyn A+B
// for an intersection. A reference to a type of another module has the
// path of the module and the name of the type, joined by ::. Rust declares
// no overloads.
//
// # Concurrency
//
// A Language is safe for concurrent use, because it has no state.
type Language struct{}

// Language is the spelling adapter of Rust.
var _ nodetest.Language = Language{}

// Lang returns the identity of Rust.
func (Language) Lang() symbol.Lang { return rust.Lang }

// Overloads reports false, because Rust declares one function of a name in
// a scope.
func (Language) Overloads() bool { return false }

// Spell writes ref in the compact spelling of Rust. A tuple of one member
// has the trailing comma that Rust requires, and a child with type
// arguments has them in angle brackets.
//
// Error modes: a tuple without members returns errUnitTuple, a function
// type with more than one result returns errManyResults, and every form
// that Rust has no spelling for, such as an optional or a union, returns
// an error.
func (Language) Spell(ref *node.TypeRef) (string, error) {
	children := make([]string, 0, len(ref.Elems))
	for _, elem := range ref.Elems {
		children = append(children, spelled(elem))
	}
	switch ref.Form {
	case symbol.FormBorrow:
		child := strings.Join(children, itemSep)
		if ref.Mutability != symbol.MutabilityMutable {
			return borrowMark + child, nil
		}
		// The compact spelling has a space between mut and the child only
		// where the child starts with a character of an identifier.
		if r, _ := utf8.DecodeRuneInString(child); r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return borrowMark + mutWord + child, nil
		}
		return borrowMark + mutMark + child, nil
	case symbol.FormArray:
		return sliceOpen + strings.Join(children, itemSep) + lengthSep + strconv.Itoa(ref.Length) + sliceClose, nil
	case symbol.FormList:
		return sliceOpen + strings.Join(children, itemSep) + sliceClose, nil
	case symbol.FormTuple:
		switch len(children) {
		case 0:
			return "", errUnitTuple
		case 1:
			return parenOpen + children[0] + itemSep + parenClose, nil
		default:
			return parenOpen + strings.Join(children, itemSep) + parenClose, nil
		}
	case symbol.FormFunc:
		split := min(max(ref.Split, 0), len(children))
		text := fnOpen + strings.Join(children[:split], itemSep) + parenClose
		switch results := children[split:]; len(results) {
		case 0:
			return text, nil
		case 1:
			return text + resultArrow + results[0], nil
		default:
			return "", errManyResults
		}
	case symbol.FormIntersection:
		return dynWord + strings.Join(children, boundSep), nil
	default:
		return "", fmt.Errorf("testing: Rust has no spelling of the form %d", ref.Form)
	}
}

// Qualify returns the path of the module pkg and name, joined by ::. A
// declaration of a module without a path has its name alone.
func (Language) Qualify(pkg *node.Package, name string) string {
	if len(pkg.Path) == 0 {
		return name
	}
	return strings.Join(pkg.Path, pathSep) + pathSep + name
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
