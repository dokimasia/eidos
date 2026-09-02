// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
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
	// sink stages and commits what the plans render, nil for a
	// composition that stops after the settle.
	sink output.Sink
}

// ErrRunFailed classifies a run that reported errors; the findings
// themselves are in the report's sink.
var ErrRunFailed = errors.New("workspace: the run reported errors")

// Report is what a run leaves behind, for callers and their tests:
// the findings, the arbitrated facts, and each plan's emit store.
type Report struct {
	Sink  *diag.Sink
	Facts *meta.Facts
	// Written records what the run committed to its sink, in the
	// order the sink reports: empty for a composition declaring no
	// output, and for a run whose findings kept it from writing.
	Written []output.Written
	// Emits holds each plan's store, keyed by plan name. It is
	// empty where the frame stopped before the plans ran.
	Emits map[string]*plugin.Emit
}
