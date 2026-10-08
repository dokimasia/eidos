// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"io/fs"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// RenderedFile is one rendered output as a value: the routed path and
// the finished bytes. Nothing here touches disk. Staging and commit
// belong to the sink that consumes it.
type RenderedFile struct {
	// Path is the routed path the file is written at, workspace-relative
	// and slash-separated: the [File.Path] the render rendered.
	Path string
	// Pkg is the package the file declares, the [File.Pkg] the render
	// rendered, zero where the target derives none.
	Pkg symbol.Identity
	// Plugins names the emitters whose units assembled the file and
	// the plugins that appended into the units' slots, distinct and
	// sorted. The output contract writes one attribution line per
	// name.
	Plugins []ID
	// Sources names what the file derives from: the distinct
	// routing keys of its units, sorted. A source-keyed unit
	// contributes its source path, a package-keyed unit its
	// package path, and a plan file contributes nothing.
	Sources []string
	// Body is the finished text: formatted, in the language's own
	// spelling. The output contract stamps and stages it.
	Body []byte
	// Findings are the findings that the render reported to the
	// context's sink about the file, in report order. A run records
	// them with the file, and a warm run that keeps the file without
	// rendering it reports them again.
	Findings []diag.Diag
}

// Renderer renders one plan's emit into files as values.
//
// A problem with one file attaches to the context's sink and to the
// file's [RenderedFile.Findings], and the pass continues with the
// remaining files. A returned error is fatal to the pass. Two calls
// over one store return the same bytes, which the conformance suite
// checks every renderer for.
type Renderer interface {
	Render(ctx *RenderContext) ([]RenderedFile, error)
}

// RenderContext is what one render call may touch.
type RenderContext struct {
	// Emit is the plan's store, complete: every generator ran and
	// every accumulator flushed before rendering starts.
	Emit *Emit
	// Files are the plan's routed files, sorted by path: what the
	// render renders. Each file whose declarations render returns one
	// [RenderedFile], and a context with no files renders nothing.
	Files []File
	// Schedule is the plan's generators in bucket order: what
	// declared template overrides resolve against, passed as data
	// so the pass decides nothing.
	Schedule []ID
	// Trees maps each plugin to its declared template tree for this
	// target: what a template reference resolves in. The composition
	// reads them off the [TemplateProvider] surface; a fixture hands
	// them over directly.
	Trees map[ID]fs.FS
	// Funcs maps each plugin to its template helpers for this target,
	// and Overrides to the shared names it declares it replaces, both
	// read off the same surface. The merge is the pass's: schedule
	// order, the latest taking precedence, and a shared name shadowed
	// without a declaration is refused and reported.
	Funcs     map[ID]template.FuncMap
	Overrides map[ID][]string
	// Sink takes the pass's findings: an unresolved reference, a
	// dropped slot marker, a format failure.
	Sink *diag.Sink
	// Plugin is the backend's own identity, the origin of its
	// findings.
	Plugin ID
}

// SyntaxProvider declares a target's comment forms: what the
// output contract writes a generated file's frame through, and
// what a frontend reading that file back recognizes it by. A
// backend implements it, and a composition staging output asks
// through it rather than restating the language's own spelling.
type SyntaxProvider interface {
	Syntax() CommentSyntax
}

// TemplateProvider declares a plugin's presentation for each
// target: the tree its references resolve in, the helpers its
// templates call, and the shared vocabulary names it replaces.
//
// Templates returns the tree that serves a target and reports
// false where none does. TemplateTargets returns the targets the
// plugin declares a tree of its own for, sorted, so a composition
// refuses a plan whose target a plugin with such trees does not
// serve, at Build and not at render. TemplateFuncs returns the
// helpers for a target, the replacements included, and Overrides
// the replaced names for that target, which is the replace verb: a
// shared name shadowed without a declaration is a lint finding,
// and where two plugins override one name, the one at the latest
// schedule position takes effect, because a plugin that changes
// how a construct renders runs after what it changes.
type TemplateProvider interface {
	Templates(t Target) (fs.FS, bool)
	TemplateTargets() []Target
	TemplateFuncs(t Target) template.FuncMap
	Overrides(t Target) []string
}
