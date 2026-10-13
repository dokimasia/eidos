// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package sentinel generates the checks of the sentinels and the error
// types of each package.
//
// sentinel is an annotator and a generator. The annotator stamps
// [KeyValue] on each variable whose name the rules.ErrorValueRules of its
// language accept, such as ErrNotFound in Go. The generator selects the
// variables with the fact, and the structs with the fact golang.satisfiesError
// of the Go satellite, so its handlers do not filter their subjects. It
// writes no production code: the author declares the errors, and the
// generated test file checks them.
//
// # Checks
//
// The family [Word] under the tag [TestTag] claims one test file for each
// package through the template sentinel.test.tmpl, and each sentinel and
// error type is a record of the file. The file checks these properties:
//
//   - Each sentinel has a message, and the message starts with the prefix of
//     the package.
//   - Each message differs from every other and does not start with
//     another.
//   - A sentinel does not match another under errors.Is.
//   - Each error type reports a message for its zero value, and exposes the
//     cause that it was given.
//
// The prefix is the name of the package and a colon. The option prefix
// replaces the name, and [PrefixOff] withholds the check. The cause of an
// error type is the cause of the error contract that the rules of its
// language state: in Go, the first field of the type error that a struct
// declares, where the struct has the method Unwrap that returns one error.
// A generic error type has no checks. The tests import the standard
// library alone.
//
// # Exclusion
//
// sentinel declares no directive. The kernel directive skip plugin=sentinel
// excludes a variable or a struct from its rules.
//
// # Dependency position
//
// The package imports the package internal/tree of the module, the root of
// the Go satellite, the root, emit, meta, plugin and symbol packages of the
// SDK facade, and the Go stdlib. Its output is Go alone.
package sentinel
