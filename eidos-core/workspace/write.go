// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"io/fs"
	"path"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// staged is one rendered file on its way to the sink: the path the
// plan's layout derived, and the stamped bytes.
type staged struct {
	path string
	body []byte
}

// render drives one plan's backend over its settled store and
// returns the files it produced, stamped through the plan's
// contract and addressed through its layout.
//
// It answers nothing for a composition declaring no output: the
// render is what a written file costs, and a run that writes
// nothing does not pay it. A backend that renders but cannot be
// asked to is a declaration defect the composition already
// refused, so the assertion here reports rather than panics.
func render(pl compiledPlan, into *plugin.Emit, sink *diag.Sink) ([]staged, error) {
	if pl.contract == nil {
		return nil, nil
	}
	renderer, renders := pl.backend.(plugin.Renderer)
	if !renders {
		return nil, fmt.Errorf(
			"backend %s writes output and does not render", pl.backend.Name(),
		)
	}
	files, err := renderer.Render(renderContext(pl, into, sink))
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	out := make([]staged, 0, len(files))
	for _, f := range files {
		body, err := pl.contract.Stamp(f)
		if err != nil {
			return nil, fmt.Errorf("stamp %s: %w", f.Name, err)
		}
		out = append(out, staged{path: layout(pl, f), body: body})
	}
	return out, nil
}

// renderContext assembles what the pass reads: the settled store,
// the plan's schedule, and the template surfaces its generators
// declare for the backend's target.
func renderContext(
	pl compiledPlan, into *plugin.Emit, sink *diag.Sink,
) *plugin.RenderContext {
	target := pl.backend.Target()
	ctx := &plugin.RenderContext{
		Emit:      into,
		Schedule:  make([]plugin.ID, 0, len(pl.entries)),
		Trees:     map[plugin.ID]fs.FS{},
		Funcs:     map[plugin.ID]template.FuncMap{},
		Overrides: map[plugin.ID][]string{},
		Sink:      sink,
		Plugin:    pl.backend.Name(),
	}
	for _, s := range pl.entries {
		ctx.Schedule = append(ctx.Schedule, s.name)
		provider, declares := s.run.(plugin.TemplateProvider)
		if !declares {
			continue
		}
		if tree, held := provider.Templates(target); held {
			ctx.Trees[s.name] = tree
		}
		if funcs := provider.TemplateFuncs(target); len(funcs) > 0 {
			ctx.Funcs[s.name] = funcs
		}
		if over := provider.Overrides(); len(over) > 0 {
			ctx.Overrides[s.name] = over
		}
	}
	return ctx
}

// layout derives one file's path: the plan's own derivation, or
// the convention — the owning package's path and the file's name
// joined by a slash, and the name alone for a plan file, which
// carries no package.
func layout(pl compiledPlan, f plugin.RenderedFile) string {
	if pl.layout != nil {
		return pl.layout(f.Pkg, f.Name)
	}
	return conventionalPath(f.Pkg, f.Name)
}

// conventionalPath is the default layout.
func conventionalPath(pkg symbol.Identity, name string) string {
	if pkg.Package == "" {
		return name
	}
	return path.Join(pkg.Package, name)
}

// commit writes every plan's staged files into the run's sink, in
// plan order and then in the order each plan rendered them, and
// commits once. The write is sequential where the render was
// parallel, so one run writes one tree in one order however the
// plans interleaved.
//
// A failed write discards the staging rather than committing half
// a tree.
func (w *Workspace) commit(staged [][]staged) ([]output.Written, error) {
	if w.sink == nil {
		return nil, nil
	}
	for _, files := range staged {
		for _, f := range files {
			if err := w.sink.Write(f.path, f.body); err != nil {
				return nil, fmt.Errorf("workspace: stage %s: %w", f.path,
					discarding(w.sink, err))
			}
		}
	}
	written, err := w.sink.Commit()
	if err != nil {
		return nil, fmt.Errorf("workspace: commit the output: %w", err)
	}
	return written, nil
}

// discarding drops a failed run's staging, joining whatever the
// discard itself reports so neither failure hides the other.
func discarding(sink output.Sink, err error) error {
	if derr := sink.Discard(); derr != nil {
		return fmt.Errorf("%w (the staging also failed to discard: %w)", err, derr)
	}
	return err
}
