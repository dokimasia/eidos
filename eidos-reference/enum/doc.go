// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package enum generates the text of each variant of an enumeration under
// the directive enum, the parser of the text, and checks of both.
//
// enum writes the methods String, IsValid, MarshalText and UnmarshalText on
// the type of the enumeration, and the function Parse<T>, the function
// <T>Values and the error ErrUnknown<T> beside the type:
//
//	s, err := ParseStatus("listed") // errors.Is(err, ErrUnknownStatus) for any other text
//	s.String()                      // "listed"
//	StatusValues()                  // each variant, in the order of the declaration
//
// The text of a variant is its declared value where the value is text, as
// "listed" is for StatusListed. Otherwise it is the name of the variant
// without the name of the type, as Low is for LevelLow. enum reads the exact
// value of each constant from the fact that the Go frontend stamps.
//
// # What enum withholds
//
// enum withholds each method that the type declares already. Parse<T>,
// <T>Values, ErrUnknown<T>, MarshalText and UnmarshalText read the text of
// a variant, so enum writes them only where it writes String. The param
// methods=false withholds every method, and leaves the checks of the
// variants.
//
// # Files
//
// The declarations are text of Go alone, so the family [Word] claims its
// files through the template enum.tmpl, and each enumeration is a record of
// a file. The family [Word] under the tag [TestTag] claims a test file for
// each source file through the template enum.test.tmpl. The file checks that
// no two variants share a value, and checks each declaration that enum
// writes. [Surface] is the data of both templates. The tests import the
// standard library alone.
//
// # Diagnostics
//
// enum reports [DuplicateText] at the second of two variants with one
// text, and writes nothing for the enumeration.
//
// # Dependency position
//
// The package imports the package internal/tree of the module, the root of
// the Go satellite, the root, diag, directive, emit, meta and plugin
// packages of the SDK facade, and the Go stdlib. Its output is Go alone.
package enum
