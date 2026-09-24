// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"cmp"

	"go.dokimi.dev/eidos/core/position"
)

// Origin names whoever reported a finding or wrote a claim: a
// plugin's declared name, or one of the kernel phases below. The
// service provider interface spells it plugin.ID where plugins are
// the subject; the definition sits here so the layers beneath that
// interface can carry it.
type Origin string

// The kernel phases that report findings. A phase is an origin like
// any plugin, so a consumer filtering by origin needs no second rule
// for the kernel.
const (
	// PhaseBuild composes the workspace and populates its registries.
	PhaseBuild Origin = "build"
	// PhaseLoad is the frontends parsing units into the graph.
	PhaseLoad Origin = "load"
	// PhaseLink resolves type spellings into canonical identities.
	PhaseLink Origin = "link"
	// PhaseFreeze seals the graph and builds its indexes.
	PhaseFreeze Origin = "freeze"
	// PhaseAnnotate is the annotators stamping facts.
	PhaseAnnotate Origin = "annotate"
	// PhaseGenerate is a plan's generators producing emit trees.
	PhaseGenerate Origin = "generate"
	// PhaseRender is a backend spelling an emit tree as source.
	PhaseRender Origin = "render"
	// PhaseClose merges the manifests, checks for collisions and
	// sweeps.
	PhaseClose Origin = "close"
)

// Diag is one finding.
//
// Every field is API: consumers script against them, tests assert on
// the code, and machine output carries them under a versioned
// schema.
type Diag struct {
	// Code identifies the finding across releases.
	Code Code
	// Severity decides what the finding means for the run. The zero
	// value is [SeverityError].
	Severity Severity
	// Pos locates the declaration that caused the finding. It is
	// never zero: a finding without a position is a defect in
	// whatever reported it.
	Pos position.Pos
	// Msg is one sentence in the present tense, naming the thing and
	// the refusal or the finding.
	Msg string
	// Origin is the plugin, or the kernel phase, that reported it.
	Origin Origin
	// Related holds secondary positions, such as the colliding twin
	// or the export a refusal depends on.
	Related []position.Pos
}

// Compare orders two findings by position, then code, then message,
// then origin, returning a negative number, zero or a positive one
// as d sorts before, with or after o. It is the canonical order a
// producer reporting from parallel workers merges its findings in,
// so the report does not depend on which worker finished first.
func (d Diag) Compare(o Diag) int {
	return cmp.Or(
		d.Pos.Compare(o.Pos),
		cmp.Compare(d.Code.Prefix, o.Code.Prefix),
		cmp.Compare(d.Code.Number, o.Code.Number),
		cmp.Compare(d.Msg, o.Msg),
		cmp.Compare(d.Origin, o.Origin),
	)
}
