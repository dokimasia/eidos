// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"cmp"

	"go.dokimi.dev/eidos/core/position"
)

// Origin is the name of the reporter of a finding or of the writer of a
// claim. It is the declared name of a plugin or one of the kernel phases
// that follow. The service provider interface declares the same type as
// plugin.ID for plugins. This package declares it so that the layers
// below that interface can use it.
type Origin string

// These constants are the kernel phases that report findings. A phase is
// an origin as a plugin is, so a consumer that filters by origin needs no
// second rule for the kernel.
const (
	// PhaseBuild composes the workspace and populates its registries.
	PhaseBuild Origin = "build"
	// PhaseLoad is the phase in which the frontends parse units into the
	// graph.
	PhaseLoad Origin = "load"
	// PhaseLink resolves type spellings into canonical identities.
	PhaseLink Origin = "link"
	// PhaseFreeze seals the graph and builds its indexes.
	PhaseFreeze Origin = "freeze"
	// PhaseAnnotate is the phase in which the annotators stamp facts.
	PhaseAnnotate Origin = "annotate"
	// PhaseGenerate is the phase in which the generators of a plan
	// produce emit trees.
	PhaseGenerate Origin = "generate"
	// PhaseLayout routes the settled declarations of a plan to the files
	// that contain them.
	PhaseLayout Origin = "layout"
	// PhaseRender is the phase in which a backend writes an emit tree as
	// source.
	PhaseRender Origin = "render"
	// PhaseClose merges the manifests, checks for collisions and
	// sweeps.
	PhaseClose Origin = "close"
)

// Diag is one finding.
//
// Every field is API. Consumers write scripts against the fields, tests
// assert on the code, and the machine output contains the fields under a
// versioned schema.
type Diag struct {
	// Code identifies the finding across releases.
	Code Code
	// Severity is the effect of the finding on the run. The zero value is
	// [SeverityError].
	Severity Severity
	// Pos is the position of the declaration or the file that caused the
	// finding. It is never zero, because a finding without a position is
	// a defect of its reporter.
	Pos position.Pos
	// Msg is one sentence in the present tense. It contains the name of
	// the subject and the refusal or the finding.
	Msg string
	// Origin is the plugin, or the kernel phase, that reported it.
	Origin Origin
	// Related contains secondary positions, such as the position of the
	// other declaration of a collision, or the position of the export that
	// causes a refusal.
	Related []position.Pos
}

// Positioned reports whether the finding has a position that a reader
// can open: a file. A finding about a declaration also has the line and
// the column of the declaration. A finding about a whole file has the
// file alone. Every test kit checks the position of a finding through
// it.
func (d Diag) Positioned() bool { return d.Pos.File != "" }

// Compare orders two findings by position, then code, then message,
// then origin. It returns a negative number, zero or a positive number
// as d sorts before, with or after o. A producer that reports from
// parallel workers merges its findings in this order, so the report does
// not depend on the order in which the workers finish.
func (d Diag) Compare(o Diag) int {
	return cmp.Or(
		d.Pos.Compare(o.Pos),
		cmp.Compare(d.Code.Prefix, o.Code.Prefix),
		cmp.Compare(d.Code.Number, o.Code.Number),
		cmp.Compare(d.Msg, o.Msg),
		cmp.Compare(d.Origin, o.Origin),
	)
}
