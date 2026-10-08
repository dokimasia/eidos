// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package workspace composes plugins into one validated, immutable
// value and runs a source tree, or a graph the caller loaded, through
// the frame to committed files and a recorded manifest.
//
// [Builder] collects the composition: the brand, the frontends,
// annotators, plans, workspace checks, target names, metadata key
// registrations, language rules, ignored directive spellings, the
// output, the ledger, the parse memo, the workspace's name, the worker
// count and the config. [Builder.Build] runs its validation steps in one
// pass: the brand, the worker count, the memo's limit, the frontends,
// the roster, the registries with the kernel's own keys and schemas
// registered first and every registry sealed, the lowering into
// priority buckets, the options and their canonical encoding, the
// config's refinements of the plans, the plans and their compiled
// schedule, with every generator that declares templates serving its
// plan's target, each plan's [Sources] and dependencies, the plans'
// dependency order, the plans each check reads, and, where the
// composition declares output, the output contract each plan writes
// through. Every step runs even when an
// earlier one found faults. Build returns either the [Workspace] or one
// error joining everything found, so the composition's author reads
// every fault at once. A Build that succeeds has resolved every
// human-typed name in the composition. Nothing after it fails on a name.
//
// [Workspace.Describe] returns the composition as data. The description
// contains the frontends, the schedule of each role with the bucket and the
// version of each plugin, the sources, dependencies, backend and layout of
// each plan, the commit order and the checks. Describe runs nothing, so a
// command can print the composition without a run.
//
// # Plans that share a workspace
//
// A plan's [Sources] scope what its generators see: a language,
// directory patterns and a module, checked at Build and bound to each
// run's graph and facts. A plan that names others in [Plan.DependsOn]
// generates after each of them has rendered, reads their exports, the
// [plugin.ExportDoc] values that list what they rendered, and commits
// only where each of them commits. Build refuses a cycle, naming every
// plan in it. A [plugin.WorkspaceCheck] that [Builder.Checks] registers
// runs at Close over the records of the plans it reads.
//
// [Config.Plans] refines a declared plan by its name before Build compiles
// it. A [PlanConfig] leaves the plan out, or replaces its sources, its
// layout policy, its output directory or its import base. The fingerprint
// includes a refined plan as it includes a declared plan.
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
// previous record and the sealed state from the composition's ledger,
// then loads the input's tree with the composition's frontends under
// its brand, or seals the graph the caller handed over. Per-subject
// directive validation, the stamp replay, the kernel meta drops and
// the annotate schedule in bucket order follow. The plans then run in
// parallel: each generates, settles, routes, renders and stamps over
// its own emit store, scoped index, readers and findings, and a plan
// that depends on others starts after them. Plans exchange exports
// alone, so one plan's failure stops only the plans that depend on it.
// One annotator's or generator's phase call runs its matches on up to
// the worker count [Builder.Parallel] sets, one at a time by default,
// and the run's output does not depend on the count.
// A Workspace is safe for concurrent runs: nothing on it mutates after
// Build, and every mutable structure a run touches, its sink included,
// is created per call.
//
// A run takes the lock of the composition's ledger, where the ledger
// implements [ledger.Locker], before it reads the previous record, and
// releases it after the record, so one run at a time works in a state
// directory. [Input.Caller] describes the run in the holder's record. A
// run that finds the lock taken reports [StateLocked], marks every plan
// failed and writes nothing.
//
// # The sealed state
//
// A run over a tree, with a ledger, records the next generation of the
// sealed state strictly after the last commit: the record of its load,
// with each file's stat and digest and each unit's region, and the
// manifest documents that changed. The next run compares the tree with
// the live generation. The gate stats every file and hashes each file
// the record does not prove unchanged, and the load takes every unit
// whose inputs are unchanged from the generation, whose region the
// graph decodes on first read.
//
// A warm run restores its fact store from the generation, and executes
// again only the validations and annotator invocations that read what
// the load changed. It also runs an annotator invocation again when the
// invocation's subject changed, or when a fact of the subject has another
// winner than in the generation. The propagation stops at a fact whose
// winner did not change, so no record that read the fact runs again. The
// run reports the findings of every validation and annotator invocation
// that it keeps.
//
// A plan of a warm run executes again only the groups that the run's
// changes make dirty. It keeps the files of its other groups as the
// generation records them. It reads the export of a plan that it depends
// on only when it executes. An enumeration by kind or by directive
// spelling runs again only where the plan's sources admit a package in
// which a declaration of the kind, or a subject of the spelling, appeared
// or disappeared. A plan that did not commit in the run before runs
// whole, and so does a plan whose sources admit or refuse a package
// otherwise than in the generation, because the package's files or its
// module fact changed.
//
// Close compares the paths that the plans routed with the paths that the
// generation records. It audits again the declarations that appeared or
// changed and the subjects whose facts changed. It calls a workspace check
// again after a change in the graph or the facts, and after a change in
// the files or the export of a plan that the check reads. It keeps the
// record of every other check. The run reports the findings of every
// group, audit and check that it keeps. A warm run commits the same files,
// records the same manifest and reports the same findings as a cold run
// over the same tree.
//
// Each generation records the SHA-256 of [Workspace.Fingerprint] and of
// the template trees the plans render through, as the run reads them,
// and the digest of the executable that wrote it. A generation of another
// composition or another executable, and one that does not open, is
// reported under [ColdState], and the run runs cold. A run that meets a
// damaged record or region discards what it derived before any plan
// commits. It then reports [ColdState] and runs again cold. A commit that
// meets damage where it merges the runs of a table, after the plans
// committed, runs the run again cold too, which commits the same files and
// records a generation without a parent. [Input.Cold] ignores the sealed
// state and reports nothing for it. These runs do not write a generation:
//
//   - a dry run
//   - a run over a caller's graph
//   - a run whose previous record does not read
//   - a run that cannot read its executable
//   - a run whose stamp replay refused a stamp or met a dangling one,
//     because no record of the sealed state keeps that finding
//   - a run with patterns, a prune, and a run that skipped a plan, so the
//     next run compares against the generation and finds every change
//     that such a run left
//
// [Builder.Memo] keeps a parse memo: the region of every unit a run
// parsed, stored under the unit's key and the executable's digest. The
// memo's entries are in a ledger of its own, or under memo/ in the
// composition's ledger. A load restores a changed unit from the memo
// where the memo has the unit's key, and parses it otherwise. The
// commit removes the entries with the oldest modification times once
// the memo exceeds its limit.
//
// [Report.Stats] counts what the run executed: the files the gate
// statted and hashed, the units the load parsed, restored, kept and
// parsed again to link, the regions the run decoded, the subjects it
// validated, the invocations of each phase call, the files the plans
// rendered, the checks it called, and the generation and the bytes the
// commit wrote.
//
// # The output
//
// A composition declaring output through [Builder.Output] takes the
// frame to the destination. After every plan has rendered, each plan
// that can commit opens a sink of its own, stages its files and the
// removal of its stale outputs, and prepares. The preparation reports
// what each staged path contains, and a drifted or foreign file where
// the plan writes is an Error of that plan. [Input.OverwriteDrift] and
// [Input.Adopt] let a sink that implements [output.Overwriter] write over
// a drifted file and a foreign one, and the run then reports neither.
// Close then runs on one
// goroutine: two plans routing a file to one path are [PlanCollision],
// the outputs of plans the composition no longer declares are swept,
// every metadata completeness contract is audited, and the workspace
// checks run. The plans commit in dependency order, composition order
// between plans without one, and the sweep after them. A run that does
// not write a generation records the merged manifest only where a plan
// or the sweep committed. It records the manifest strictly after the
// last commit. A stale
// output is removed only where it is the brand's intact output, and one
// edited since its stamp or without the brand's frame remains under
// [KeptOutput]. A dry run, [Input.Dry], runs every phase and commits and
// records nothing. A composition declaring no output stops after the
// settle, and its plans' emit stores are the run's whole product.
//
// # Plan selection, patterns, the prune and the check
//
// [Input.Plans] runs the plans that it lists and every plan they depend
// on, transitively. Every other plan reports [PlanSkipped]: it does not
// run, and its files and record entries remain. A plan of the run that
// routes a file to the path of one of its files collides with it, and a
// workspace check that reads it does not run.
//
// [Input.Patterns] narrow the commit alone. The run generates every plan
// whole and commits the changes inside the patterns. It stages every other
// change into a sink of its own, which it prepares for
// [PlanReport.Withheld] and discards, and a withheld change leaves the
// record's entry of its path as it was. [Input.Prune] withholds every
// write and commits the removals of stale files. The sweep of the plans
// that the composition no longer declares commits under either input.
//
// [Input.Check] runs as a dry run does. It reports each path that the run
// would create, update or remove, or would refuse to write, and each file
// that the sweep would remove, as an Error under [OutOfDate] at the path.
// A change that the patterns withhold is not reported.
//
// # Suppression and strict mode
//
// A diag directive on a declaration removes the findings of the code that
// it names at the declaration's position, from the run's sink and from the
// sink of every plan, so a removed Error fails no plan. A kernel Error is
// never removed. The run reads its diag directives after validation. The
// findings that a warm run reports again from its records pass through
// the same table, because a record keeps every finding as it was
// reported, so an edit of a directive changes what the next run reports.
// [Report.Suppressions] lists each directive with the findings that it
// removed. The run reports each directive that removed nothing as an Info
// under [UnusedSuppression], except in a run that skips a plan and in a
// cancelled run. [Input.Strict] reports every Warning that remains as an
// Error.
//
// # Explaining a generation
//
// [Workspace.Explain] returns what the live generation of the ledger
// records about a [Target]. A target is the path of a generated file, the
// identity of a declaration, a key at a position, or a code at a position.
// [Workspace.ParseTarget] parses a target from a command-line argument.
// Explain reads the generation under the lock of the ledger and runs no
// phase, so it describes the last run that wrote a generation. The
// generation stores each read as a hash. Explain computes the hashes of the
// edges that it can construct, and identifies the reads with those hashes.
// Each [ExplainedRecord] contains the identified reads and the number of
// the other reads. Explain returns [ErrNoGeneration] for a ledger without a
// generation.
//
// # Failure semantics
//
// Build returns errors and collects them, and every registry beneath
// it refuses a duplicate naming both claimants. Run refuses these inputs
// with a plain error:
//
//   - an input that sets neither or both of a tree and a graph
//   - a plan that the composition does not declare
//   - a pattern that names no directory of a workspace tree
//   - Prune with Patterns or with Check
//
// A plan's own Error, and an error its generator, its sink or its
// commit returns, fails that plan and every plan that depends on it, and
// their previous files and record entries remain. A plan or a check
// that reads a failed plan reports [FailedDependency]. An Error in a
// phase every plan shares, Load, validation, Annotate or Close, a
// workspace check's included, commits nothing at all. Findings
// arrive in the report's sink, and any Error among them classifies the
// run under [ErrRunFailed]. A previous record that does not read is
// [UnreadableRecord], and the run removes nothing. A lock that another
// holder has is [StateLocked]. A ledger that fails to open, to lock, to
// read or to write the sealed state is a returned error. A cancelled
// context stops the run between units of work. A commit runs to its end
// once begun. A cancellation observed before a plan's commit skips that
// commit and the commit of every plan after it. The report states each
// plan's [PlanStatus]. Nothing here panics.
//
// # Dependency position
//
// core/workspace imports core/frontend/load, core/plugin, core/store,
// core/node, core/meta, core/directive, core/rules, core/layout,
// core/emit, core/output, core/ledger, core/manifest, core/diag,
// core/position, core/symbol, core/internal/pathset, core/internal/state
// and the Go stdlib.
// It never imports the root authoring package: plugins arrive built,
// so the composition works on the base contract every authoring layer
// lowers to.
package workspace
