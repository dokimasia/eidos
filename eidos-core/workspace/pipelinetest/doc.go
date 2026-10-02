// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package pipelinetest checks one plan end to end over real source: a
// fixture's tree is copied into a directory, the fixture's composition
// runs over it, and the checks read what the run left on disk.
//
// [RunPipelineSuite] runs every check in a parallel subtest over
// directories of its own. Each check also runs alone against the
// [go.dokimi.dev/assert.TB] role, so a test can run it against a
// composition it must reject:
//
//   - [AssertClean]: the run reports no Error, positions every finding
//     at a file, and returns no error.
//   - [AssertGenerated]: the files under the brand's frame are the
//     fixture's wanted files, byte for byte.
//   - [AssertRecorded]: the state directory's record lists exactly the
//     wanted paths, each under the fixture's plan and with the digest of
//     its file.
//   - [AssertIdempotent]: a second run in the same directory changes no
//     byte and moves no mtime.
//   - [AssertRelocated]: a run in a second directory generates the same
//     bytes and records the same files.
//
// Every check runs one plan. A composition of any other number of
// plans fails each check, and so does a fixture without a tree or a
// composition. [AssertClean] alone judges the run's error and findings.
// The other checks read what the runs left on disk.
//
// The suite imports no satellite. A satellite runs it over a fixture
// in its own language, composed with its own frontend and backend.
//
// # Dependency position
//
// core/workspace/pipelinetest imports core/workspace,
// core/workspace/internal/rundir, core/diag, the assert module and the
// Go stdlib. It drives the whole run, so it is above core/workspace,
// and no package of the kernel imports it.
package pipelinetest
