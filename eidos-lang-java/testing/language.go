// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"errors"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/nodetest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The tokens of the compact spelling that [Language] writes: the tokens of
// the frontend's spelling without whitespace, with one space between two
// identifier tokens.
const (
	dimensionMark = "[]"
	wildcardMark  = "?"
	extendsWord   = "extends "
	superWord     = "super "
	argsOpen      = "<"
	argsClose     = ">"
	argSep        = ","
)

// errBoundVariance is the refusal of a bounded wildcard without a variance,
// which the Java frontend never records, because a bound of Java follows
// extends or super.
var errBoundVariance = errors.New("testing: a bounded wildcard of Java has the variance of extends or super")

// Language is the spelling adapter of Java for a fixture of nodetest. It
// writes the compact spelling that the Java frontend records: T[] for an
// array type, and ?, ?extends T and ?super T for a wildcard. A reference to
// a type of another package has the canonical name of the type. Java
// overloads.
//
// # Concurrency
//
// A Language is safe for concurrent use, because it has no state.
type Language struct{}

// Language is the spelling adapter of Java.
var _ nodetest.Language = Language{}

// Lang returns the identity of Java.
func (Language) Lang() symbol.Lang { return java.Lang }

// Overloads reports true, because Java declares overloads of one method.
func (Language) Overloads() bool { return true }

// Spell writes ref in the compact spelling of Java: the spelling of its
// element and one pair of brackets for a list, and ? with the variance and
// the bound for a wildcard. A child with type arguments has them in angle
// brackets.
//
// Error modes: a bounded wildcard without a variance returns
// errBoundVariance, and every other form returns an error, because Java
// has no spelling for it.
func (Language) Spell(ref *node.TypeRef) (string, error) {
	switch ref.Form {
	case symbol.FormList:
		if len(ref.Elems) != 1 {
			return "", fmt.Errorf("testing: an array type of Java has one element type, and this one has %d",
				len(ref.Elems))
		}
		return spelled(ref.Elems[0]) + dimensionMark, nil
	case symbol.FormWildcard:
		if len(ref.Elems) == 0 {
			return wildcardMark, nil
		}
		switch ref.Variance {
		case symbol.VarianceOut:
			return wildcardMark + extendsWord + spelled(ref.Elems[0]), nil
		case symbol.VarianceIn:
			return wildcardMark + superWord + spelled(ref.Elems[0]), nil
		case symbol.VarianceInvariant:
			// A bound of Java follows extends or super, so it has a variance.
		}
		return "", errBoundVariance
	default:
		return "", fmt.Errorf("testing: Java has no spelling of the form %d", ref.Form)
	}
}

// Qualify returns the canonical name of the declaration: the parts of the
// package of pkg, then name, separated by dots. A declaration of the
// unnamed package has its name alone.
func (Language) Qualify(pkg *node.Package, name string) string {
	if len(pkg.Path) == 0 {
		return name
	}
	return strings.Join(pkg.Path, packageSep) + packageSep + name
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
	return ref.Spelling + argsOpen + strings.Join(args, argSep) + argsClose
}
