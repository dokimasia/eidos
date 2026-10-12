// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspacetest checks a composition of more than one plan
// against the workspace frame, over a source tree on disk.
//
// A [Fixture] has a tree, the stores that its load reads, a composition
// without its plans, the plans, every file that the plans generate, and
// an edit of the tree.
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
//     their exports, and each export lists the files that the record of
//     its plan lists.
//   - [AssertCycleRefused]: two plans that depend on each other are a
//     Build error that contains the names of both.
//   - [AssertSwept]: a composition without the last plan removes the
//     files of that plan and keeps a file that was edited after its
//     stamp.
//   - [AssertAudited]: a completeness contract that no annotator meets
//     reports at its severity.
//   - [AssertChecked]: a workspace check that reads a failed plan does
//     not run, and one that reads a clean plan reads its records.
//   - [AssertExportCutoff]: an edit that leaves the first plan's export
//     unchanged runs no invocation that read only that export.
//   - [AssertWarmChecked]: a check that a warm run calls after the edit
//     reads the files and the export that a cold run hands it.
//
// [RunWarmColdSuite] checks the warm path against the cold path over the
// fixture's [Fixture.Edit]:
//
//   - [AssertWarmUnchanged]: a warm run over an unchanged tree runs
//     nothing and writes nothing.
//   - [AssertTouched]: a file whose modification time moved hashes once
//     and parses nothing.
//   - [AssertDamaged]: a damaged sealed state runs the run again cold, with
//     one ColdState at Info.
//   - [AssertRestored]: a parse memo restores the restorable units of a
//     reverted edit without a parse, and the load parses the others again.
//   - [AssertWarmEdited]: a warm run after the edit leaves the files, the
//     record entries, the findings and the exports that a cold run over
//     the edited tree leaves.
//
// A warm run hashes each file unless its record shows that the file is
// unchanged. The checks of an unchanged tree first move the modification
// time of every file into the past and run the fixture twice, so each
// record shows its file unchanged.
//
// Every check takes the assert module's TB role, so the kernel's tests
// run each one against compositions it must reject. A check stops where
// [Fixture.Compose] returns an error.
//
// # Shared checks
//
// The pipeline kit and the acceptance kit share these parts:
//
//   - [AssertRerunUnchanged] moves the modification time of every file
//     outside the sealed state into the past, runs a second time, and
//     fails on each file that the second run rewrote or added.
//   - [Want] reads the goldens of a fixture from a directory, and
//     [AssertGolden] matches a run against them, so -update rewrites them.
//   - [Generated] reads the files that a run recorded as the input of a
//     toolchain check.
//
// # Dependency position
//
// core/workspace/workspacetest imports core/workspace,
// core/workspace/internal/rundir, core/frontend/load, core/plugin,
// core/output, core/ledger, core/manifest, core/meta, core/diag,
// core/position, core/symbol, core/toolchain, the assert module with its
// files and golden packages, and the Go stdlib. It drives the whole run,
// so it is above core/workspace, and no package of the kernel imports it.
package workspacetest
