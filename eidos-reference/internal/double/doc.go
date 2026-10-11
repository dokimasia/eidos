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
// every parameter on. The receiver has a name that no parameter and no
// result of the method has. The template of a target writes the receiver
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
// [Of] reports [IncompleteMembers] at an interface whose members its rules
// cannot list in full, and [Property] at an interface with a property. Both
// are Errors, and the plugin writes no double of the interface.
//
// # Dependency position
//
// The package imports the root, diag, emit, node, plugin and rules packages
// of the SDK facade and the Go stdlib.
package double
