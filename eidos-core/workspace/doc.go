// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspace composes plugins into one validated, immutable
// value and runs a loaded graph through the frame.
//
// [Builder] collects the composition: annotators, plans, target
// names, metadata key registrations and config. [Builder.Build]
// runs six validation steps in one pass: the roster,
// the registries, the lowering into priority buckets, the options,
// the plans, and the compiled schedule. Every step runs even when
// an earlier one found faults, and the answer is either the
// [Workspace] or one error joining everything found, so the
// composition's author reads every fault at once. A Build that
// succeeds has resolved every human-typed name in the composition,
// so nothing after it fails on a name.
//
// # The run
//
// [Workspace.Run] takes one loaded graph through the frame: seal,
// per-subject directive validation, the kernel meta drops, the
// annotate schedule in bucket order, and the plans in parallel.
// Each plan owns its emit store, its scoped index and its readers,
// and plans exchange nothing, so one plan's failure does not stop
// its siblings. A Workspace is safe for concurrent runs: nothing
// on it mutates after Build, and every mutable structure a run
// touches is created per call.
//
// # Failure semantics
//
// Build returns errors and collects them; every registry beneath
// it refuses a duplicate naming both claimants. Run refuses a
// missing or pre-frozen graph with a plain error, wraps a
// handler's returned error with its role and stops the frame, and
// never stops for a finding: findings arrive in the report's sink,
// and any Error among them classifies the run under
// [ErrRunFailed]. Nothing here panics.
//
// # Dependency position
//
// core/workspace imports core/plugin, core/store, core/meta,
// core/directive, core/diag, core/symbol and the Go stdlib. It
// never imports the root authoring package: plugins arrive built,
// so the composition works on the base contract every authoring layer
// lowers to.
package workspace
