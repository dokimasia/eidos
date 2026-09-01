// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"io/fs"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
)

// origin is the identity the suite renders under. The context's
// plugin is the origin findings carry, so a valid renderer
// reports everything back under it, which is what the attribution
// rule checks.
const origin plugin.ID = "backendtest"

// Fixture is a hand-built plan as the renderer sees it: the emit
// store, the schedule, and the template trees, helpers and
// override declarations the composition would have handed over.
type Fixture struct {
	// Emit is the plan's store, complete, the way rendering finds
	// it after every generator ran and every accumulator flushed.
	Emit *plugin.Emit
	// Schedule is the plan's generators in bucket order: what
	// declared template overrides resolve against.
	Schedule []plugin.ID
	// Trees holds each emitting plugin's template tree for the
	// target under test: what a template reference resolves in.
	Trees map[plugin.ID]fs.FS
	// Funcs holds each plugin's template helpers, and Overrides
	// the shared names each declares it replaces, the way the
	// composition reads them off the provider surface.
	Funcs     map[plugin.ID]template.FuncMap
	Overrides map[plugin.ID][]string
}

// Setup builds the renderer under test with the emit fixture it
// renders, fresh per call, the way the plugin suite's Setup does:
// two calls build two isolated fixtures, which is what makes the
// determinism check honest.
type Setup func(tb assert.TB) (plugin.Renderer, *Fixture)

// context lowers the fixture to one render call's context, whole,
// over a fresh sink and under the suite's identity.
func (f *Fixture) context(sink *diag.Sink) *plugin.RenderContext {
	return &plugin.RenderContext{
		Emit: f.Emit, Schedule: f.Schedule, Trees: f.Trees,
		Funcs: f.Funcs, Overrides: f.Overrides,
		Sink: sink, Plugin: origin,
	}
}
