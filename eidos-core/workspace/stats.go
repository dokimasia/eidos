// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// Stats counts what one run executed: what a probe asserts and what a
// report summarizes. A cold run and a warm run over one tree leave the
// same files and findings, and differ here.
type Stats struct {
	// Cold reports that the run ignored the sealed state.
	Cold bool
	// Statted and Hashed count the files the gate statted and hashed.
	Statted, Hashed int
	// Parsed, Restored and Kept count the units the run parsed, restored
	// from the memo, and took from the generation. Reparsed counts the
	// units it parsed again to link.
	Parsed, Restored, Kept, Reparsed int
	// Decoded counts the regions the run decoded.
	Decoded int
	// Validated counts the subjects whose directives the run validated. A
	// subject the graph does not contain is reported and not validated.
	Validated int
	// Invoked counts the invocations each phase call ran.
	Invoked []Invoked
	// Rendered counts the files the plans rendered and stamped.
	Rendered int
	// Checked counts the workspace checks the run called. A check that
	// reads a failed plan is not called.
	Checked int
	// Generation reports that the commit wrote a generation, Written the
	// bytes the commit wrote to the ledger, and Size the bytes of the
	// generation it wrote with the segments that generation references,
	// zero where it wrote none.
	Generation    bool
	Written, Size int64
}

// count takes the counts of a run's load: whether the run read the
// sealed state, the files the gate statted and hashed, the units by
// where the load took each region from, and the units it parsed though
// the generation kept them. A run over a caller's graph loaded nothing
// and is cold.
func (s *Stats) count(loaded *load.Report, sealed *sealedState) {
	s.Cold = sealed.gen == nil
	if loaded == nil {
		return
	}
	s.Statted, s.Hashed, s.Reparsed = loaded.Statted, loaded.Hashed, loaded.Reparsed
	for _, u := range loaded.Units {
		switch u.From {
		case load.FromParse:
			s.Parsed++
		case load.FromMemo:
			s.Restored++
		case load.FromGeneration:
			s.Kept++
		}
	}
}

// Invoked counts the invocations of one phase call.
type Invoked struct {
	// Plan is the call's plan, empty for an annotator.
	Plan   string
	Plugin plugin.ID
	Phase  plugin.Phase
	Count  int
}
