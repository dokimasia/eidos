// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the projection rules of Rust.
//
// [New] returns the value that a composition registers. It satisfies the
// source-rules contract and the capabilities for enums, error types, results,
// ownership, equality, generics, constructors and texts.
//
// # Decisions
//
//   - A trait inherits the methods of its supertraits, and a struct the
//     methods of the traits that its trait impls name. A declared member hides
//     the inherited members of its name, and two traits that declare one name
//     at one depth cancel each other, as Rust refuses the call.
//   - Every parameter is an input, because a caller cancels a future when it
//     drops it. A return of a result type has the error model of a result
//     type, and the kernel splits it into its value and its failure through
//     [Rules.Result]. A return of an iterator or a stream is a stream.
//   - The primitives and the types of the standard library that a crate names
//     have the shapes of [Rules.Builtin], because the load reads no crate of
//     the standard library.
//   - The spelling of a directive resolves in the scope order of Rust: the
//     items and the use bindings of the module, its glob imports, and the
//     paths from the crate root, the module and its parent.
//   - A value is an expression of Rust: a literal, a call of an associated
//     function such as String::from, a struct literal, a variant and the macro
//     vec!. A text that an author states for a String takes String::from
//     through [Rules.Text].
//
// # Dependency position
//
// lang/rust/rules imports the sdk facade, the satellite root, lang/naming,
// lang/numeric and the Go standard library.
package rules
