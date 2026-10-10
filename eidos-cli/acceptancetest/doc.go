// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package acceptancetest builds the binary of a consumer, and checks it as
// a process against the contract of the command line.
//
// A consumer runs [RunAcceptanceSuite] over its own binary with a
// [Fixture], and the suite runs every check over the binary. The suite does
// not check the dispatch of the host itself, such as an unknown command or
// the help of the host.
//
// # Checks
//
// Each check is a function that runs the binary over copies of the tree in
// an empty directory, and reports through an [assert.TB]. A kit test runs a
// check under [assert.Rejects], and a consumer can run one check alone. The
// kit has these checks.
//
//   - [AssertStatuses] checks the statuses 0, 1 and 64 of run.
//   - [AssertNames] checks that each kernel command runs under its name.
//   - [AssertPanic] checks that a panic exits 2 with the stack trace.
//   - [AssertDiscovery] checks the search for the config file.
//   - [AssertJSON] checks the JSON output of each kernel command except
//     watch.
//   - [AssertIdempotent] checks that a second run rewrites no file.
//   - [AssertCompiles] checks that the generated output compiles.
//   - [AssertLists] checks lists of workspaces.
//   - [AssertLocked] checks a run while another process has the lock.
//
// # Processes
//
// [Build] compiles the binary with the go command into a temporary
// directory of the test, and [Exec] runs it under an environment of PATH,
// HOME and the variables that a check adds. Exec kills a process after two
// minutes.
//
// # Dependency position
//
// acceptancetest imports cli, core/ledger, core/output, the assert module
// and the Go stdlib.
package acceptancetest
