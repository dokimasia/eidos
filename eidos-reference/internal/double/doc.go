// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package double builds the methods of a test double, which mockgen and
// stubgen share.
//
// A double has one method for each method of its interface, with the
// signature of the interface's method. [Of] mirrors the effective methods
// of an interface of the graph, and [Copy] copies the methods of an
// interface that a plugin emitted. Each method of a double has a receiver
// and a name for each parameter, because the body of the method passes
// every parameter on. The receiver and each parameter without a name in
// the interface get a name that no other parameter and no result of the
// method has. A parameter or a result with the name of a predeclared
// identifier of Go, such as append, gets such a name too, because the body
// of a stub calls append. The template of a target writes the receiver
// where the target writes one, and a weaver reads it to tell the method of
// a double from the signature of an interface.
//
// [Signatures] copies the signatures of methods without a receiver, for a
// record that a test of a double reads. A weaver passes over such a copy.
//
// # Targets
//
// A double plugin serves Go and TypeScript, so it declares a template tree
// for [GoTarget] and for [TypeScriptTarget]. The two constants have the
// spellings of the targets of the satellites, which such a plugin does not
// import.
//
// # Diagnostics
//
// [Of] reports [Constraint] at a Go constraint interface,
// [IncompleteMembers] at an interface whose members its rules cannot list
// in full, and [Property] at an interface with a property. All three are
// Errors, and the plugin does not write a double of the interface.
//
// # Dependency position
//
// The package imports the root, diag, emit, meta, node and plugin packages
// of the SDK facade, eidos-lang's naming, the Go satellite, and the Go
// stdlib.
package double
