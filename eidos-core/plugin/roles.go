// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
)

// Annotator stamps facts over the frozen graph.
//
// A problem with one subject attaches to the context's sink and the
// phase continues; a returned error is fatal to the phase. An
// annotator never adds or removes declarations, and the store
// enforces it: nothing reachable from the context can write to the
// graph.
type Annotator interface {
	Plugin
	Annotate(ctx *AnnotatorContext) error
}

// Generator produces emit values into one plan.
//
// A problem with one subject attaches to the context's sink and the
// phase continues; a returned error is fatal to the phase. A
// generator is never seeing languages: it reads the graph through its
// context and emits neutral values into the plan's store.
type Generator interface {
	Plugin
	Generate(ctx *GeneratorContext) error
}

// WorkspaceCheck checks a claim across plans at Close, over the run's
// records: each plan's files, each plan's export, the frozen graph and
// the facts. It reports diagnostics and writes nothing.
//
// A problem with one item is a finding on the context's sink, and the
// check continues. A returned error is fatal to Close: the run commits
// nothing and returns the error, wrapped with the check's name. The run
// calls a check only where every plan it reads staged cleanly, and
// reports one Info for a check it does not call.
//
// A warm run calls a check again where it found a change in the graph or
// the facts, or where a plan the check reads rendered or removed a file or
// changed its export. Otherwise it does not call the check, and reports
// the findings of the check's last call.
type WorkspaceCheck interface {
	Plugin
	// Reads returns the names of the plans whose records the check
	// reads, and nil for every plan of the composition. The
	// composition refuses a name it does not declare, and a name
	// listed twice.
	Reads() []string
	// Check reports what the records break.
	Check(ctx *CheckContext) error
}

// AnnotatorContext is what one Annotate call may touch.
//
// The two read surfaces split by rule: Index is the dispatcher's
// routing path and records nothing, and Reader is the plugin's
// tracked path, recording into the phase's read set. Plugin and
// Bucket are the arbitration rank fields of every stamp made under
// this call.
type AnnotatorContext struct {
	Index  *Index
	Reader *store.Reader
	Facts  *meta.Facts
	Sink   *diag.Sink
	// Rules is the composition's registry of language rules, and
	// Kernel the kernel's registered keys: what the authoring
	// surface binds the kernel's walks over. A nil registry binds
	// every language to the absent rules.
	Rules  *rules.Registry
	Kernel meta.KernelKeys
	// Plugin is the caller's identity: the diagnostic origin and
	// the rank's plugin field.
	Plugin ID
	// Bucket is the priority bucket this call runs in: the rank's
	// bucket field.
	Bucket int
	// Workers is how many invocations the call may run at once. Zero
	// and one mean sequentially. A plugin implementing the role
	// directly may ignore it, and a plugin that runs work on
	// goroutines joins them before its call returns, whatever the
	// count.
	Workers int
	// Select restricts the call to what a warm run executes again, and
	// nil runs every match. A plugin implementing the role directly may
	// ignore it.
	Select *Selection
	// Journal receives a record for each invocation the call runs, and
	// nil keeps none. A plugin implementing the role directly may
	// journal nothing, and the run then records the call under
	// [WholeCall].
	Journal Journal
}

// GeneratorContext is what one Generate call may touch. Its Index
// and Reader are scoped to the plan's sources, and Emit is the plan's
// store, where every accumulator flushes.
type GeneratorContext struct {
	Index  *Index
	Reader *store.Reader
	Facts  *meta.Facts
	Emit   *Emit
	Sink   *diag.Sink
	// Rules and Kernel have the meaning the annotator's context gives
	// the fields of the same names.
	Rules  *rules.Registry
	Kernel meta.KernelKeys
	// Plugin is the caller's identity: the diagnostic origin and
	// the emit attribution.
	Plugin ID
	// Bucket is the priority bucket this call runs in, which
	// decides what an emit-triggered rule can see: the store contains
	// earlier buckets' units.
	Bucket int
	// Workers is how many invocations the call may run at once, with
	// the meaning [AnnotatorContext.Workers] states.
	Workers int
	// Select and Journal have the meaning the annotator's context gives
	// the fields of the same names.
	Select  *Selection
	Journal Journal
	// Exports are the exports of the plans the plan depends on, keyed
	// by plan name, each complete before the plan's first generator
	// runs. It is nil for a plan that depends on none. Every dependent
	// of a plan reads the same values, so a generator does not mutate
	// them.
	Exports map[string]ExportDoc
	// Target is the target of the plan's backend. Types is the backend's
	// spoke, and nil for a backend that does not implement
	// [TypeSpeller]. Policy is the plan's resolved lowering policy, which
	// Build fixes.
	Target Target
	Types  TypeSpeller
	Policy Policy
}

// PlanRecord is one plan's record as Close reads it: what the plan's
// commit records, and its export.
type PlanRecord struct {
	// Name is the plan's name.
	Name string
	// Files are the manifest entries of the files the plan routed,
	// sorted by path, each with the digest of the bytes the plan
	// staged.
	Files []manifest.Entry
	// Export is the plan's export.
	Export ExportDoc
}

// CheckContext is what one Check call may read. Its Index and Reader see
// the whole graph. The run records the reader's reads beside the check's
// findings, and a read through Index or Facts records nothing.
type CheckContext struct {
	Index  *Index
	Reader *store.Reader
	Facts  *meta.Facts
	Sink   *diag.Sink
	// Rules and Kernel have the meaning the annotator's context gives
	// the fields of the same names.
	Rules  *rules.Registry
	Kernel meta.KernelKeys
	// Plugin is the check's identity: the origin of its findings.
	Plugin ID
	// Plans are the records of the plans the check reads, in
	// composition order.
	Plans []PlanRecord
}
