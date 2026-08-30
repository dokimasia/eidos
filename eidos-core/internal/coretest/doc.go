// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package coretest holds the assertions and fixtures the kernel's
// own tests share.
//
// It exists so that a rule every package is held to is written once.
// [AssertDependencyPosition] is that rule for package documentation,
// and the fixtures below are the graph a case is written against:
// [Package] and [Struct] build the node model a frontend would hand
// over, [Frozen] and [Reading] put it behind a sealed graph and a
// tracked reader.
//
// Nothing here belongs to a shipped surface. Plugin authors get
// their own testing package; this one is for the kernel.
//
// # Dependency position
//
// core/internal/coretest imports core/store, core/node,
// core/symbol, core/internal/gosource, the assert module and the
// Go stdlib. It is imported by test packages alone, so nothing it
// depends on can cycle back through it.
package coretest
