// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package eidos is the kernel of a code-generation framework. Frontends
// parse source into a symbol graph that no language owns, plugins annotate
// the graph and generate output entities, and backends render the entities
// in target languages. A run is deterministic and incremental, and spans
// languages.
//
// The kernel is a library without a binary. A consumer composes a
// workspace in its own main package and passes it to the command kernels.
// The kernel knows no language. Language support is in satellite modules
// that implement the projection tiers, and the kernel contains no name of
// a satellite, no language string and no table per language. Its runtime
// packages import the packages of the module and the standard library
// alone.
//
// # Composition model
//
// One workspace contains the read side, which is the frontends, the
// annotators and one frozen symbol graph. Each of N plans contains a write
// side with its own generators, its layout and exactly one backend, and
// the composition declares the one output through which a run writes.
// Plans are values. They run in parallel and exchange nothing except
// declared, versioned exports. Plugins compose at compile time and
// communicate only through typed metadata on the graph. Every write
// records its authority, plugin below directive below manual, and its
// provenance. A manual override at the declaration therefore outranks any
// inference, and every fact records what it was derived from.
//
// # Authoring model
//
// This root package is the authoring surface. A plugin is a value.
// [NewPlugin] starts the declaration, and the identity, the outputs, the
// priorities and the schemas are data that the plugin builds once. Only the
// handlers, the key registrations that [Builder.Keys] declares and the
// template helpers are functions. Handlers attach to triggers indexed by
// kind, with one generated constructor for each subject kind, such as
// [OnInterface], and [OnGraph] and [OnEmit]. Gates are declarative.
// [Directive] binds a schema and [Where] binds fact predicates, and a
// handler contains no filter. A predicate on a key of another plugin takes
// a handle from [meta.Named], which the workspace binds through
// [plugin.KeyBinder] when it builds.
//
// The second parameter of a handler selects its effect, [Emitter] to
// generate or [Stamper] to annotate. [Builder.Generator],
// [Builder.Annotator] and [Builder.Dual] lower a rule set to the role of
// its effects in [go.dokimi.dev/eidos/core/plugin]. That package remains
// public for what the facade does not fit.
//
// A frontend or a backend is declared the same way, in
// [go.dokimi.dev/eidos/core/frontend] and
// [go.dokimi.dev/eidos/core/backend]. Its identity and its language facts
// are data, its functions are declared once, and a Build lowers it to the
// same roles.
//
// # Presentation
//
// A plugin whose emit names templates declares where they resolve.
// [Builder.Templates] and [Builder.Funcs] declare the tree and the helpers
// of the plugin level, which serve every target. [Builder.For] layers the
// [Templates], [Funcs] and [Overrides] of one target over them. A plugin
// that declares trees for named targets alone serves those targets alone,
// and the workspace refuses a plan of any other target at Build.
// [Emitter.JoinName] joins a family word onto a generated identifier in
// the neutral convention, and the backend of the plan spells the result in
// the case of the target when the plan settles.
//
// # Translation
//
// [Emitter.Type] spells a type of another language in the target of the
// plan. The kit folds the reference into its canonical shape under the
// rules of the language that declares it, and the spoke of the backend of
// the plan spells the shape under the lowering policy of the plan. A value
// of the key of a policy at directive authority overrides the choice of
// the policy. The kit reads it on the declaration with the type, then on
// the subject of the match, and then on the package of the declaration. A
// shape without a spelling in the target reports [RefusedType] at the
// declaration with the type, and the plan fails.
//
// # Dispatch
//
// Dispatch is indexed. A rule that a directive gates visits the carriers
// of the directive, a rule that a fact gates visits the stamped subjects,
// and only a bare rule visits the whole graph. A phase call collects the
// matches of each rule before it runs them, and numbers them in canonical
// match order. That order is the rules in declaration order, the subjects
// of each rule in the order of its index, and the gating instances of a
// subject in source order. Every invocation has its own read set, minted
// on its first read, so the derivation of a fact write names what its
// match read. The kernel skip directive excludes a subject from bare rules
// and rules that a fact gates, for every plugin or for one named plugin.
// Rules that a directive gates run regardless.
//
// A phase call runs the matches of a rule on up to the worker count of its
// context, and one at a time where the count is below two or the rule has
// one match. The placements, slot appends and findings of an invocation
// are buffered with its sequence number. They are the declarations that
// an [Out] places, the values that a [SlotView] appends and the findings
// that its match reports. The buffers apply in canonical match order when
// the rules of the phase call have run. The output therefore does not
// depend on the worker count, and a handler sees the store of the plan and
// the slots as they were when its phase call began. A stamp arrives in the
// fact store as the handler makes it, because the store ranks claims by
// their rule, subject and gating instance and not by their arrival. An
// append through [Emitter.Slot] into a value that a plugin of an earlier
// turn placed adds the plugin to the contributors of the unit that
// contains the value.
//
// # Selective dispatch
//
// The selection of a phase context restricts every rule outside the emit
// phase to the listed matches that its gates still admit and to every
// match of a candidate, in canonical match order of rule, subject identity
// and gating instance. A graph rule runs where the selection lists it, and
// the rules of the emit phase run over every value of the store of the
// plan. A full call and a selected call order their invocations
// differently within a rule, and no output depends on that order. A claim
// ranks by its rule, subject and instance, a unit orders its declarations
// by origin first, and the invocations that append into one slot share the
// origin of the value of the slot. A selected call reports the findings
// that a full call reports, in the order in which it runs their
// invocations. The journal of a phase context receives one record for
// each invocation in canonical match order once the effects of the call
// apply, and then the matches of each candidate. The invocations of a rule
// on one worker share one read set, reset between them, and a call that
// journals copies the edges of each invocation into a [store.ReadLog] as
// the invocation returns.
//
// # Failure semantics
//
// A declaration defect panics at the typed build, before any run exists,
// and so does a rule set whose effects do not fit the role of the build. A
// problem of a handler with one subject goes to the sink through its
// match, and the phase continues. A returned error is fatal to the phase
// call, wrapped with the plugin and the rule. On any worker count, the
// call returns the error earliest in canonical match order. The findings
// of the invocations up to that one arrive in the sink, and none of the
// placements or slot appends of the call apply. The stamps already made
// remain in the fact store, and on more than one worker they can include
// stamps of invocations after the failed one. The calling goroutine raises
// the panic of a handler again, the earliest in canonical match order. A
// stamp that the fact store refuses reports under [RefusedStamp] at the
// position of the subject.
//
// # Determinism
//
// Byte identity is the contract. The same workspace over the same input
// produces the same bytes on every machine, warm or cold, on any worker
// count. Every ordering is defined. Units order by plugin, cardinality,
// key, package and tag, and contributions by origin, gating instance and
// canonical match order. Output contains no clock reading and nothing from
// the environment.
//
// # Dependency position
//
// The root package imports core/plugin, core/diag, core/directive,
// core/emit, core/meta, core/node, core/position, core/rules, core/store,
// core/symbol, core/internal/grow and the Go stdlib. Nothing in the module
// imports the root package back.
package eidos
