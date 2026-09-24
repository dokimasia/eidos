// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package scaffold spells the neutral statement and value vocabulary
// for the C-family targets: Go, TypeScript, Java and Rust.
//
// [Scaffold] writes a statement through a target's [Grammar]: the
// tokens in which the targets differ, which are the indentation, the
// statement end, the declaring token, the binding of more than one
// name and the failure guard. [Expr] writes an expression, where a name
// spells itself and a call is fn(a, b) in every target. [Value] walks
// a value tree and hands each spelling to a [Target]. [Leaves] is the
// literal spelling the targets share, parameterized by the absent
// value, the string quoting and the number spelling.
//
// # Failure semantics
//
// A statement or an expression the grammar has no form for returns an
// error, which the render reports as a refused template. A value the
// target cannot spell returns a [render.ValueError], which the render
// reports under its value code.
//
// # Dependency position
//
// lang/scaffold imports sdk/emit, sdk/render, sdk/symbol and the Go
// stdlib. It imports no grammar and no satellite.
package scaffold
