// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace

import (
	"errors"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// Workspace is the validated, immutable composition: the sealed
// registries and the compiled schedules, and nothing else, because
// everything mutable belongs to one run. Concurrent runs are safe;
// each creates its own sink, fact store, indexes and emit stores.
type Workspace struct {
	keys       *meta.Registry
	directives *directive.Registry
	annotate   []annEntry
	plans      []compiledPlan
}

// ErrRunFailed classifies a run that reported errors; the findings
// themselves are in the report's sink.
var ErrRunFailed = errors.New("workspace: the run reported errors")

// Report is what a run leaves behind, for callers and their tests:
// the findings, the arbitrated facts, and each plan's emit store.
type Report struct {
	Sink  *diag.Sink
	Facts *meta.Facts
	// Emits holds each plan's store, keyed by plan name. It is
	// empty where the frame stopped before the plans ran.
	Emits map[string]*plugin.Emit
}
