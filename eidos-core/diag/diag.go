// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag

import "go.dokimi.dev/eidos/core/position"

// PluginID names whoever reported a finding: a plugin's declared
// name, or one of the kernel phases below.
type PluginID string

// The kernel phases that report findings. A phase is an origin like
// any plugin, so a consumer filtering by origin needs no second rule
// for the kernel.
const (
	// PhaseBuild composes the workspace and populates its registries.
	PhaseBuild PluginID = "build"
	// PhaseLoad is the frontends parsing units into the graph.
	PhaseLoad PluginID = "load"
	// PhaseLink resolves type spellings into canonical identities.
	PhaseLink PluginID = "link"
	// PhaseFreeze seals the graph and builds its indexes.
	PhaseFreeze PluginID = "freeze"
	// PhaseAnnotate is the annotators stamping facts.
	PhaseAnnotate PluginID = "annotate"
	// PhaseGenerate is a plan's generators producing emit trees.
	PhaseGenerate PluginID = "generate"
	// PhaseRender is a backend spelling an emit tree as source.
	PhaseRender PluginID = "render"
	// PhaseClose merges the manifests, checks for collisions and
	// sweeps.
	PhaseClose PluginID = "close"
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
	Origin PluginID
	// Related holds secondary positions, such as the colliding twin
	// or the export a refusal depends on.
	Related []position.Pos
}
