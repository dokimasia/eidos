// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"io/fs"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
)

// origin is the identity that the suite renders under. A render reports
// each finding under the plugin of its context, so the attribution check
// requires this origin on every finding.
const origin plugin.ID = "backendtest"

// Fixture is a plan that a test builds by hand, in the form that the
// renderer receives. It has the emit store, the schedule, and the
// template trees, helpers and override declarations that a composition
// hands over.
type Fixture struct {
	// Emit is the store of the plan, complete, as the render finds it
	// after every generator ran and every accumulator flushed.
	Emit *plugin.Emit
	// Schedule is the generators of the plan in turn order. A declared
	// template override resolves against it.
	Schedule []plugin.ID
	// Trees maps each emitting plugin to its template tree for the target
	// under test. A template reference resolves in it.
	Trees map[plugin.ID]fs.FS
	// Funcs maps each plugin to its template helpers, and Overrides maps
	// each plugin to the shared names that it replaces, as the composition
	// reads them from the plugins.
	Funcs     map[plugin.ID]template.FuncMap
	Overrides map[plugin.ID][]string
}

// Setup builds the backend under test and the emit fixture that it
// renders, new on each call, as the Setup of the plugin suite does. Two
// calls build two isolated fixtures, so the determinism check compares
// two independent renders. A test that needs a backend of its own embeds
// a kit backend and overrides the methods that it varies.
type Setup func(tb assert.TB) (backend.Backend, *Fixture)

// Context lowers the fixture to the context of one render call by b. The
// context contains every part of the fixture, reports into sink, and
// renders under the identity of the suite. [Files] routes the store into
// files through the filename speller of b.
func (f *Fixture) Context(b backend.Backend, sink *diag.Sink) *plugin.RenderContext {
	return &plugin.RenderContext{
		Emit: f.Emit, Files: Files(f.Emit, b), Schedule: f.Schedule, Trees: f.Trees,
		Funcs: f.Funcs, Overrides: f.Overrides,
		Sink: sink, Plugin: origin,
	}
}
