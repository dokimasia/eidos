// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rundir reads what a run left in a directory: the files under
// a brand's frame, the digest of a file, the record in the brand's state
// directory, and which paths are the brand's sealed state. The kernel's
// conformance suites check a run through it, so each suite reads a
// directory the way the others do.
//
// Every function that reads the directory takes the check's [assert.TB]
// and stops the check where the directory does not read, because a check
// over a directory it cannot read proves nothing.
//
// # Dependency position
//
// core/workspace/internal/rundir imports core/ledger, core/manifest,
// core/output, core/internal/state, the assert module and the Go
// stdlib.
package rundir
