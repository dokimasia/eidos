// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspacetest checks a composition of more than one plan
// against the workspace frame, over a source tree on disk.
//
// A [Fixture] states a tree, the stores its load reads, a composition
// without its plans, the plans, and every file the plans generate.
// [RunWorkspaceSuite] runs every check in a parallel subtest over
// temporary directories of its own. Each check composes from the
// fixture's builder and adds what it needs, so the caller supplies a
// working composition and the suite constructs every failure it
// checks:
//
//   - [AssertGenerated]: the plans generate the wanted files, each
//     recorded under the plan that routed it.
//   - [AssertCollision]: a copy of the first plan under another name
//     collides with it, and nothing commits.
//   - [AssertIsolated]: a failure seeded into the last plan keeps that
//     plan's files and entries, and every other plan commits.
//   - [AssertExported]: a probe plan that depends on every plan reads
//     their exports, and each lists what its plan's record lists.
//   - [AssertCycleRefused]: two plans that depend on each other are a
//     Build error naming both.
//   - [AssertSwept]: a composition without the last plan removes that
//     plan's files and keeps one edited since its stamp.
//   - [AssertAudited]: a completeness contract that no annotator meets
//     reports at its severity.
//   - [AssertChecked]: a workspace check that reads a failed plan does
//     not run, and one that reads a clean plan reads its records.
//
// Every check takes the assert module's TB role, so the kernel's tests
// run each one against compositions it must reject.
//
// # Dependency position
//
// core/workspace/workspacetest imports core/workspace,
// core/workspace/internal/rundir, core/plugin, core/output,
// core/ledger, core/manifest, core/meta, core/diag, core/position,
// core/symbol, the assert module and the Go stdlib. It drives the whole
// run, so it is above core/workspace, and no package of the kernel
// imports it.
package workspacetest
