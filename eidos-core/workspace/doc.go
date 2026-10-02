// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspace composes plugins into one validated, immutable
// value and runs a source tree, or a graph the caller loaded, through
// the frame to committed files and a recorded manifest.
//
// [Builder] collects the composition: the brand, the frontends,
// annotators, plans, target names, metadata key registrations, ignored
// directive spellings, the output, the ledger, the workspace's name,
// the worker count and the config. [Builder.Build] runs its validation
// steps in one pass: the brand, the worker count, the frontends, the
// roster, the registries with the kernel's own keys and schemas
// registered first and every registry sealed, the lowering into
// priority buckets, the options and their canonical encoding, the plans
// and their compiled schedule, with every generator that declares
// templates serving its plan's target, and, where the composition
// declares output, the output contract each plan writes through. Every
// step runs even when an earlier one found faults. Build returns either
// the [Workspace] or one error joining everything found, so the
// composition's author reads every fault at once. A Build that succeeds
// has resolved every human-typed name in the composition. Nothing after
// it fails on a name.
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
// [Workspace.Run] takes one [Input] through the frame. It reads the
// previous record from the composition's ledger, then loads the input's
// tree with the composition's frontends under its brand and
// fingerprint, or seals the graph the caller handed over. Per-subject
// directive validation, the stamp replay, the kernel meta drops and
// the annotate schedule in bucket order follow. The plans then run in
// parallel: each generates, settles, routes, renders and stamps over
// its own emit store, scoped index, readers and findings. Plans
// exchange nothing, so one plan's failure does not stop its siblings.
// One annotator's or generator's phase call runs its matches on up to
// the worker count [Builder.Parallel] sets, one at a time by default,
// and the run's output does not depend on the count.
// A Workspace is safe for concurrent runs: nothing on it mutates after
// Build, and every mutable structure a run touches, its sink included,
// is created per call.
//
// # The output
//
// A composition declaring output through [Builder.Output] takes the
// frame to the destination. After every plan has rendered, each plan
// that can commit opens a sink of its own, stages its files and the
// removal of its stale outputs, and prepares. The preparation reports
// what each staged path contains, and a drifted or foreign file where
// the plan writes is an Error of that plan. Close then runs on one
// goroutine: two plans routing a file to one path are [PlanCollision],
// the outputs of plans the composition no longer declares are swept,
// and every metadata completeness contract is audited. The plans commit
// in composition order and the sweep after them. The ledger records the
// merged manifest strictly after the last commit, and only where a
// plan or the sweep committed. A stale output is removed
// only where it is the brand's intact output, and one edited since its
// stamp or without the brand's frame remains under [KeptOutput]. A dry
// run, [Input.Dry], runs every phase and commits and records nothing. A
// composition declaring no output stops after the settle, and its
// plans' emit stores are the run's whole product.
//
// # Failure semantics
//
// Build returns errors and collects them, and every registry beneath
// it refuses a duplicate naming both claimants. Run refuses an input
// that names neither or both of a tree and a graph with a plain error.
// A plan's own Error, and an error its generator, its sink or its
// commit returns, fails that plan alone, and its previous files and
// record entries remain. An Error in a phase every plan shares, Load,
// validation, Annotate or Close, commits nothing at all. Findings
// arrive in the report's sink, and any Error among them classifies the
// run under [ErrRunFailed]. A previous record that does not read is
// [UnreadableRecord], and the run removes nothing. A cancelled context
// stops the run between units of work. A commit runs to its end once
// begun. A cancellation observed before a plan's commit skips that
// commit and the commit of every plan after it. The report states each
// plan's [PlanStatus]. Nothing here panics.
//
// # Dependency position
//
// core/workspace imports core/frontend/load, core/plugin, core/store,
// core/node, core/meta, core/directive, core/rules, core/layout,
// core/emit, core/output, core/ledger, core/manifest, core/diag,
// core/position, core/symbol, core/internal/pathset and the Go stdlib.
// It never imports the root authoring package: plugins arrive built,
// so the composition works on the base contract every authoring layer
// lowers to.
package workspace
