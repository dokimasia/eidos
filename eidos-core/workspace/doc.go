// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspace composes plugins into one validated, immutable
// value and runs a loaded graph through the frame.
//
// [Builder] collects the composition: annotators, plans, target
// names, metadata key registrations, ignored directive spellings
// and config. [Builder.Build] runs its validation steps in one
// pass: the roster, the registries with the kernel's own keys and
// schemas registered first and every registry sealed, the lowering
// into priority buckets, the options and their canonical encoding,
// the plans and their compiled schedule, and, where the composition
// declares output, the output contract each plan writes through.
// Every step runs even when an earlier one found faults, and the
// answer is either the [Workspace] or one error joining everything
// found, so the composition's author reads every fault at once. A
// Build that succeeds has resolved every human-typed name in the
// composition, so nothing after it fails on a name.
//
// # The run
//
// [Workspace.Run] takes one loaded graph through the frame: seal,
// per-subject directive validation, the kernel meta drops, the
// stamp replay, the annotate schedule in bucket order, and the
// plans in parallel. Each plan owns its emit store, its scoped
// index and its readers, and plans exchange nothing, so one plan's
// failure does not stop its siblings. A Workspace is safe for
// concurrent runs: nothing on it mutates after Build, and every
// mutable structure a run touches is created per call.
//
// # The output
//
// A composition declaring output through [Builder.Output] carries
// the frame one step further, and its brand is what
// [Workspace.Brand] returns for the load to refuse its own outputs
// under: each plan settles, renders through
// its backend and stamps every file through the output contract,
// and the run writes what every plan staged into the sink and
// commits once. The render is parallel per plan and the write is
// sequential in plan order, so one run writes one tree in one
// order however the plans interleaved. A run that reported an
// Error writes nothing and discards its staging, because half a
// tree is worse than none. A composition declaring no output
// stops after the settle, and its plans' emit stores are the run's
// whole product.
//
// # Failure semantics
//
// Build returns errors and collects them; every registry beneath
// it refuses a duplicate naming both claimants. Run refuses a
// missing graph with a plain error, takes one the load already
// sealed as it stands, wraps a
// handler's returned error with its role and stops the frame, and
// never stops for a finding: findings arrive in the report's sink,
// and any Error among them classifies the run under
// [ErrRunFailed]. Nothing here panics.
//
// # Dependency position
//
// core/workspace imports core/plugin, core/store, core/node,
// core/meta, core/directive, core/rules, core/output, core/diag,
// core/symbol and the Go stdlib. It
// never imports the root authoring package: plugins arrive built,
// so the composition works on the base contract every authoring layer
// lowers to.
package workspace
