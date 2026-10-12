// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest

import (
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// The words of the notation of [Neutral], one for each structural form.
const (
	neutralOptional     = "optional"
	neutralList         = "list"
	neutralArray        = "array"
	neutralMap          = "map"
	neutralFunc         = "func"
	neutralTuple        = "tuple"
	neutralUnion        = "union"
	neutralIntersection = "intersection"
	neutralStream       = "stream"
	neutralBorrow       = "borrow"
	neutralWildcard     = "wildcard"
	neutralInline       = "inline"
)

// The punctuation of the notation of [Neutral]: the parentheses of a list
// of children, the brackets of the length of an array and of type
// arguments, the separator of a list, the mark of a method of an inline
// body, the variance of a wildcard, and the qualifier of a reference to
// another package.
const (
	neutralParenOpen   = "("
	neutralParenClose  = ")"
	neutralLengthOpen  = "["
	neutralLengthClose = "]"
	neutralArgsOpen    = "<"
	neutralArgsClose   = ">"
	neutralSep         = ", "
	neutralCall        = "()"
	neutralIn          = "in "
	neutralOut         = "out "
	neutralQualifier   = "."
)

// Language is the part of a fixture that its language decides: the
// identity of the language, whether the language overloads, and how its
// source spells a reference. The testing package of a satellite provides
// one, and [Neutral] serves a language without a satellite.
//
// # Concurrency
//
// An implementation is safe for concurrent use, because the load can call
// Lang and Overloads from its workers.
type Language interface {
	// Lang returns the language of every declaration of the fixture.
	Lang() symbol.Lang

	// Overloads reports whether the language declares two callables of
	// one name in one scope, as the Overloads method of a frontend
	// reports it.
	Overloads() bool

	// Spell returns the source spelling of a structural reference. Every
	// child of ref has its spelling when Spell runs, and Spell does not
	// change ref. Spell returns an error for a form that the language
	// does not spell.
	Spell(ref *node.TypeRef) (string, error)

	// Qualify returns the spelling of a reference to the declaration name
	// in package pkg, from a file of another package. name is the name of
	// the declaration after the names of its enclosing types, each
	// followed by a dot.
	Qualify(pkg *node.Package, name string) string
}

// Neutral is the spelling adapter of a language without a satellite, such
// as the fake language of a kit's own tests. It spells a structural
// reference as the word of its form and the spellings of its children in
// parentheses, such as list(int) and map(string, int), a notation that no
// language parses. It qualifies a reference with the name of the package
// and a dot.
//
// # Concurrency
//
// A Neutral value is safe for concurrent use, because its methods read
// only their arguments and the value's own fields.
type Neutral struct {
	// Language is the language of the fixture.
	Language symbol.Lang
	// Overloading is what Overloads reports.
	Overloading bool
}

// Neutral is the Language of a fixture without a satellite.
var _ Language = Neutral{}

// Lang returns the language of the fixture.
func (n Neutral) Lang() symbol.Lang { return n.Language }

// Overloads reports whether the language of the fixture overloads.
func (n Neutral) Overloads() bool { return n.Overloading }

// Spell writes ref in the notation of Neutral. A child with type arguments
// is its spelling with the spellings of its arguments in angle brackets.
// The words of the forms are these:
//
//   - optional(T), list(T), map(K, V), stream(T) and borrow(T) for the
//     forms of one or two children.
//   - array[3](T) for an array, with its length in brackets.
//   - func(A, B)(R) for a function type, its parameters before its
//     results.
//   - tuple(A, B), union(A, B) and intersection(A, B) for the forms of
//     members.
//   - wildcard() for an unbounded wildcard, and wildcard(out T),
//     wildcard(in T) or wildcard(T) for a bound under its variance.
//   - inline(a, m()) for an inline body, the names of its fields and then
//     the names of its methods.
//
// Error modes: a reference of the named form or of a form that only the
// fold returns, such as a scalar, returns an error, because neither is a
// structural form with a spelling of its own.
func (Neutral) Spell(ref *node.TypeRef) (string, error) {
	children := make([]string, 0, len(ref.Elems))
	for _, elem := range ref.Elems {
		children = append(children, neutralSpelling(elem))
	}
	list := neutralParenOpen + strings.Join(children, neutralSep) + neutralParenClose
	switch ref.Form {
	case symbol.FormOptional:
		return neutralOptional + list, nil
	case symbol.FormList:
		return neutralList + list, nil
	case symbol.FormArray:
		return neutralArray + neutralLengthOpen + strconv.Itoa(ref.Length) + neutralLengthClose + list, nil
	case symbol.FormMap:
		return neutralMap + list, nil
	case symbol.FormFunc:
		split := min(max(ref.Split, 0), len(children))
		params := neutralParenOpen + strings.Join(children[:split], neutralSep) + neutralParenClose
		results := neutralParenOpen + strings.Join(children[split:], neutralSep) + neutralParenClose
		return neutralFunc + params + results, nil
	case symbol.FormTuple:
		return neutralTuple + list, nil
	case symbol.FormUnion:
		return neutralUnion + list, nil
	case symbol.FormIntersection:
		return neutralIntersection + list, nil
	case symbol.FormStream:
		return neutralStream + list, nil
	case symbol.FormBorrow:
		return neutralBorrow + list, nil
	case symbol.FormWildcard:
		variance := ""
		if len(children) > 0 {
			switch ref.Variance {
			case symbol.VarianceIn:
				variance = neutralIn
			case symbol.VarianceOut:
				variance = neutralOut
			case symbol.VarianceInvariant:
				// An invariant bound has no mark.
			}
		}
		return neutralWildcard + neutralParenOpen + variance + strings.Join(children, neutralSep) +
			neutralParenClose, nil
	case symbol.FormInline:
		names := make([]string, 0, len(ref.Fields)+len(ref.Methods))
		for _, f := range ref.Fields {
			names = append(names, f.Name)
		}
		for _, m := range ref.Methods {
			names = append(names, m.Name+neutralCall)
		}
		return neutralInline + neutralParenOpen + strings.Join(names, neutralSep) + neutralParenClose, nil
	default:
		return "", fmt.Errorf("nodetest: the form %d of a reference has no structural spelling", ref.Form)
	}
}

// Qualify returns the name of pkg, a dot and name.
func (Neutral) Qualify(pkg *node.Package, name string) string {
	return pkg.Name + neutralQualifier + name
}

// neutralSpelling returns the spelling of a child in the notation of
// [Neutral]: its own spelling, and the spellings of its type arguments in
// angle brackets after it.
func neutralSpelling(ref *node.TypeRef) string {
	if len(ref.Args) == 0 {
		return ref.Spelling
	}
	args := make([]string, 0, len(ref.Args))
	for _, arg := range ref.Args {
		args = append(args, neutralSpelling(arg))
	}
	return ref.Spelling + neutralArgsOpen + strings.Join(args, neutralSep) + neutralArgsClose
}
