// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"strconv"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// Report is what a run leaves behind: its findings, its facts, each
// plan's outcome and the manifest it recorded.
type Report struct {
	// Sink has every finding of the run: the shared phases' first, then
	// each plan's in composition order, then Close's. It has no finding
	// that a diag directive removed, and under [Input.Strict] it returns
	// every Warning at Error.
	Sink *diag.Sink
	// Facts are the arbitrated facts the plans read.
	Facts *meta.Facts
	// Load is the load's record, nil for a graph the caller handed over.
	Load *load.Report
	// Plans has one entry per plan, in composition order.
	Plans []PlanReport
	// Swept lists the files the run removed for plans the composition
	// no longer declares, sorted by path.
	Swept []output.Written
	// Manifest is the record the run committed, or the record it would
	// commit under Dry. A run that commits nothing records the previous
	// record's entries.
	Manifest manifest.Manifest
	// Emits is each plan's store, keyed by plan name. It is empty where
	// the frame stopped before the plans ran.
	Emits map[string]*plugin.Emit
	// Stats counts what the run executed.
	Stats Stats
	// Suppressed counts, for each code, the findings that a diag directive
	// removed, and is nil where no directive removed a finding.
	Suppressed map[diag.Code]int
	// Suppressions lists each diag directive of the graph, with the number
	// of findings that it removed, in position order. It is empty where the
	// frame stopped before the plans ran.
	Suppressions []Suppression
}

// PlanReport records one plan's status and the changes its commit
// made, or would make under Dry.
type PlanReport struct {
	// Name is the plan's name.
	Name string
	// Status is how the plan's run ended.
	Status PlanStatus
	// Changes lists what the plan's commit did to each path, or what it
	// would do under Dry, sorted by path. A path the commit refused is
	// absent, and a plan that did not commit lists nothing.
	Changes []output.Change
	// Withheld lists the changes that the run left for a later run, sorted
	// by path, as a preparation of their own found them: the changes
	// outside [Input.Patterns], and every write of [Input.Prune]. As in
	// Changes, a path whose file is current is unchanged. A plan that did
	// not commit lists nothing.
	Withheld []output.Change
	// Refused lists the writes that the run refused, sorted by path. The
	// path of such a write has a file that was edited since its stamp, or a
	// file without the frame of the brand, and the run did not write over
	// files of that kind. A plan that refuses a write fails.
	Refused []output.Change
}

// PlanStatus is how one plan's run ended. The zero value names no
// status.
type PlanStatus uint8

const (
	// PlanCommitted reports a plan whose staging committed.
	PlanCommitted PlanStatus = 1
	// PlanFailed reports a plan that reported an Error or returned an
	// error, or that a shared phase's Error, a Close Error or a plan it
	// depends on kept from committing.
	PlanFailed PlanStatus = 2
	// PlanCancelled reports a plan whose commit the run's cancellation
	// skipped.
	PlanCancelled PlanStatus = 3
	// PlanPrepared reports a plan of a dry run, which prepared and
	// committed nothing.
	PlanPrepared PlanStatus = 4
	// PlanSkipped reports a plan that [Input.Plans] left out: it did not
	// run, and its files and records remain.
	PlanSkipped PlanStatus = 5
)

// String returns the status's spelling for a report, and the status's
// number in the form PlanStatus(n) for an undeclared status. It
// allocates nothing for a declared status.
func (s PlanStatus) String() string {
	switch s {
	case PlanCommitted:
		return "committed"
	case PlanFailed:
		return "failed"
	case PlanCancelled:
		return "cancelled"
	case PlanPrepared:
		return "prepared"
	case PlanSkipped:
		return "skipped"
	default:
		return "PlanStatus(" + strconv.Itoa(int(s)) + ")"
	}
}
