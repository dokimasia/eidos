// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package emit is the declaration model a generator produces.
//
// It carries the same kinds as the node model and differs in what
// wraps them: an emit declaration has no source position, links
// back to the node symbol it derives from through its Origin, and
// holds its member lists in a [Slot] that other plugins append
// into.
//
// The callable kinds carry a [Body]: two standard slots on every
// body by construction, owner-declared named slots between them,
// and exactly one content form of four, nothing, scaffolding from
// the [Stmt] and [Expr] vocabulary, a [TemplateRef], or verbatim
// text. The vocabulary is enough to delegate and too little to
// write logic in; a generated file that wants logic calls a
// hand-written one.
//
// The kinds, [Walk] and [All] generate from the
// symbol schema. Editing a generated file fails the build, because
// the mirror guard reruns the generator and compares.
//
// # Dependency position
//
// core/emit imports core/symbol, core/position and the Go stdlib.
// It never imports core/node: an emit declaration names its origin
// by identity, so the write side cannot reach into the read side.
package emit
