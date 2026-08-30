// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
)

// Annotator stamps facts over the frozen graph.
//
// A problem with one subject attaches to the context's sink and the
// phase continues; a returned error is fatal to the phase. An
// annotator never adds or removes declarations, which the store
// enforces rather than this docblock: nothing reachable from the
// context can write to the graph.
type Annotator interface {
	Plugin
	Annotate(ctx *AnnotatorContext) error
}

// Generator produces emit values into one plan.
//
// A problem with one subject attaches to the context's sink and the
// phase continues; a returned error is fatal to the phase. A
// generator is blind to languages: it reads the graph through its
// context and emits neutral values into the plan's store.
type Generator interface {
	Plugin
	Generate(ctx *GeneratorContext) error
}

// AnnotatorContext carries what one Annotate call may touch.
//
// The two read surfaces split by law: Index is the dispatcher's
// routing path and records nothing, and Reader is the plugin's
// tracked path, recording into the phase's read set. Plugin and
// Bucket are the arbitration rank fields every stamp made under
// this call carries.
type AnnotatorContext struct {
	Index  *Index
	Reader *store.Reader
	Facts  *meta.Facts
	Sink   *diag.Sink
	// Plugin is the caller's identity: the diagnostic origin and
	// the rank's plugin field.
	Plugin diag.PluginID
	// Bucket is the priority bucket this call runs in: the rank's
	// bucket field.
	Bucket int
}

// GeneratorContext carries what one Generate call may touch. Its
// Index and Reader are scoped to the plan's sources, and Emit is
// the plan's store, where every accumulator flushes.
type GeneratorContext struct {
	Index  *Index
	Reader *store.Reader
	Facts  *meta.Facts
	Emit   *Emit
	Sink   *diag.Sink
	// Plugin is the caller's identity: the diagnostic origin and
	// the emit attribution.
	Plugin diag.PluginID
	// Bucket is the priority bucket this call runs in, which
	// decides what an emit-triggered rule can see: the store holds
	// earlier buckets' units.
	Bucket int
}
