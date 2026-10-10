// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package numeric types a number an author wrote at a type's width:
// the integer and float values a language's rules lift from literal
// text, written back as canonical decimal text every target reads
// alike.
//
// [Int] bounds an integral value by an integer type's class and
// width. [Float] bounds a value by a float type's precision and
// writes it through [Decimal], the shortest text that reads back to
// it.
//
// # Dependency position
//
// lang/numeric imports sdk/emit, sdk/rules and the Go stdlib,
// go/constant among it.
package numeric
