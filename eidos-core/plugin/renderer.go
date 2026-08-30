// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"io/fs"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// RenderedFile is one rendered output as a value: the derived
// filename and the finished bytes. Nothing here touches disk;
// paths, staging and commit belong to the sink that consumes it.
type RenderedFile struct {
	// Name is the target-spelled filename the unit's routing key
	// derives: word, tag and cardinality key joined the way the
	// language joins them.
	Name string
	// Pkg is the owning package's identity, zero for a plan file:
	// the namespace half of the file's address, because two
	// packages spell the same filename and stay two files,
	// whatever directory layout places them in.
	Pkg symbol.Identity
	// Body is the finished text: formatted, in the language's own
	// spelling. The output contract stamps and stages it.
	Body []byte
}

// Renderer renders one plan's emit into files as values.
//
// A problem with one file attaches to the context's sink and the
// pass continues with the remaining files; a returned error is
// fatal to the pass. Two calls over one store answer the same
// bytes, which the conformance suite holds every renderer to.
type Renderer interface {
	Render(ctx *RenderContext) ([]RenderedFile, error)
}

// RenderContext carries what one render call may touch.
type RenderContext struct {
	// Emit is the plan's store, complete: every generator ran and
	// every accumulator flushed before rendering starts.
	Emit *Emit
	// Schedule is the plan's generators in bucket order: what
	// declared template overrides resolve against, carried as data
	// so the pass decides nothing.
	Schedule []ID
	// Trees holds each plugin's declared template tree for this
	// target, keyed by plugin: what a template reference resolves
	// in. The composition reads them off the [TemplateProvider]
	// surface; a fixture hands them over directly.
	Trees map[ID]fs.FS
	// Sink takes the pass's findings: an unresolved reference, a
	// dropped slot marker, a format failure.
	Sink *diag.Sink
	// Plugin is the backend's own identity, the origin its
	// findings carry.
	Plugin ID
}

// TemplateProvider declares a plugin's template trees: the bodies
// its references name, per target, and the helpers those templates
// call. Overrides names the shared vocabulary entries the plugin
// deliberately replaces, which is the replace verb: a shared name
// shadowed without a declaration is a lint finding, and where two
// plugins declare an override of one name, the latest schedule
// position wins, because a plugin that changes how a construct
// renders necessarily runs after what it changes.
type TemplateProvider interface {
	Templates(t Target) (fs.FS, bool)
	TemplateFuncs(t Target) template.FuncMap
	Overrides() []string
}
