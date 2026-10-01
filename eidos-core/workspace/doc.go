// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspace composes plugins into one validated, immutable
// value and runs a loaded graph through the frame.
//
// [Builder] collects the composition: the brand, annotators, plans,
// target names, metadata key registrations, ignored directive
// spellings, the output and config. [Builder.Build] runs its
// validation steps in one pass: the brand, the roster, the
// registries with the kernel's own keys and schemas registered
// first and every registry sealed, the lowering into priority
// buckets, the options and their canonical encoding, the plans and
// their compiled schedule, with every generator that declares
// templates serving its plan's target, and, where the composition
// declares output, the output contract each plan writes through.
// Every step
// runs even when an earlier one found faults, and Build returns
// either the [Workspace] or one error joining everything found, so
// the composition's author reads every fault at once. A Build that
// succeeds has resolved every human-typed name in the composition,
// so nothing after it fails on a name.
//
// # The brand
//
// [Builder.Brand] declares the composition's brand, and Build
// refuses a composition without a valid one. The brand is the name
// the source's carriers open with, the name every stamped file's
// trailer claims ownership under, and the name [Workspace.Brand]
// returns for a load to run under, so the load reads the
// composition's carriers and refuses the workspace's own outputs.
//
// # Registration
//
// Every registration binds to its registrant. A plugin claims
// metadata namespaces and registers keys through a handle bound to
// its name, and registers keys only into its own namespaces. The
// builder's own registrations bind to the composition. A plugin's
// directive schemas name the plugin that provides them.
//
// # The run
//
// [Workspace.Run] takes one loaded graph through the frame: seal,
// per-subject directive validation, the kernel meta drops, the
// stamp replay, the annotate schedule in bucket order, and the
// plans in parallel. Each plan has its own emit store, scoped index
// and readers, and plans exchange nothing, so one plan's failure
// does not stop its siblings. A Workspace is safe for concurrent
// runs: nothing on it mutates after Build, and every mutable
// structure a run touches, its sink included, is created per call.
//
// # The output
//
// A composition declaring output through [Builder.Output] takes the
// frame one step further: each plan settles, routes its declarations
// to files through its [layout.Config], renders the files through its
// backend and stamps each one through the output contract, and a run
// that reported no Error opens one sink, writes what every plan
// staged into it and commits once. The routing reads the source tree
// once per run, so every plan places its files against one view of
// it. The render is parallel per plan and the write is sequential in
// plan order, so one run writes one tree in one order however the
// plans interleaved. A run that reported an Error opens no sink, so
// the previous generation of files remains in place. A composition
// declaring no output stops after the settle, and its plans' emit
// stores are the run's whole product.
//
// # Failure semantics
//
// Build returns errors and collects them, and every registry
// beneath it refuses a duplicate naming both claimants. Run refuses
// a missing graph with a plain error, takes one the load already
// sealed as it is, wraps a handler's returned error with its
// role and stops the frame, and never stops for a finding: findings
// arrive in the report's sink, and any Error among them classifies
// the run under [ErrRunFailed]. Nothing here panics.
//
// # Dependency position
//
// core/workspace imports core/plugin, core/store, core/node,
// core/meta, core/directive, core/rules, core/layout, core/output,
// core/diag, core/symbol and the Go stdlib. It never imports the root
// authoring package: plugins arrive built, so the composition works
// on the base contract every authoring layer lowers to.
package workspace
