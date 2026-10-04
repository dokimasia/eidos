// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package coretest contains the assertions and fixtures the kernel's
// own tests share.
//
// It exists so that the graph every package's cases are written
// against is built once. [Package] and [Struct] build the node model a
// frontend would hand over, and [Frozen] and [Reading] put it behind a
// sealed graph and a tracked reader. [CopyTree] copies a fixture tree
// for a case that writes into the tree it runs in. [Rewriting] is a
// disk ledger that records an edited record, for a case that checks a
// suite rejects it.
//
// Nothing here belongs to a shipped surface. Plugin authors get their
// own testing package, and this one is for the kernel.
//
// # Dependency position
//
// core/internal/coretest imports core/store, core/node,
// core/symbol, core/diag, core/position, core/ledger, core/manifest,
// core/output, core/internal/state, the assert module and the Go
// stdlib. It is imported by test packages alone, so nothing it depends
// on can cycle back through it.
package coretest
