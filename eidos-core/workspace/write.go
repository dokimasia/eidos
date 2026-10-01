// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// staged is one rendered file on its way to the sink: the path the
// plan's layout routed it to, and the stamped bytes.
type staged struct {
	path string
	body []byte
}

// write routes one plan's settled store to files against the run's
// source tree and renders the files. A backend that spells no
// filenames is a declaration defect the composition already refused,
// so the assertion here returns an error and does not panic, and so
// does a defect in the routing's inputs.
func (w *Workspace) write(
	pl compiledPlan, ix *plugin.Index, src tree, into *plugin.Emit, sink *diag.Sink,
) ([]staged, error) {
	speller, spells := pl.backend.(plugin.FileSpeller)
	if !spells {
		return nil, fmt.Errorf("backend %s writes output and spells no filenames", pl.backend.Name())
	}
	packager, _ := pl.backend.(plugin.Packager)
	files, err := layout.Route(layout.Input{
		Emit:       into,
		Config:     pl.routing,
		Outputs:    pl.outputs,
		Speller:    speller,
		Packager:   packager,
		Index:      ix,
		Directives: w.directives,
		Residents:  src.residents,
		Modules:    src.modules,
		Sink:       sink,
	})
	if err != nil {
		return nil, fmt.Errorf("route: %w", err)
	}
	return render(pl, into, files, sink)
}

// render drives one plan's backend over the files its layout routed
// and returns what the backend produced, stamped through the plan's
// contract and staged at each file's routed path. A backend that does
// not render is a declaration defect the composition already refused,
// so the assertion here returns an error and does not panic.
func render(pl compiledPlan, into *plugin.Emit, files []plugin.File, sink *diag.Sink) ([]staged, error) {
	renderer, renders := pl.backend.(plugin.Renderer)
	if !renders {
		return nil, fmt.Errorf(
			"backend %s writes output and does not render", pl.backend.Name(),
		)
	}
	rendered, err := renderer.Render(renderContext(pl, into, files, sink))
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	out := make([]staged, 0, len(rendered))
	for _, f := range rendered {
		body, err := pl.contract.Stamp(f)
		if err != nil {
			return nil, fmt.Errorf("stamp %s: %w", f.Path, err)
		}
		out = append(out, staged{path: f.Path, body: body})
	}
	return out, nil
}

// renderContext assembles what the pass reads: the settled store, the
// routed files, the plan's schedule, and the template surfaces its
// generators declare for the backend's target.
func renderContext(
	pl compiledPlan, into *plugin.Emit, files []plugin.File, sink *diag.Sink,
) *plugin.RenderContext {
	target := pl.backend.Target()
	ctx := &plugin.RenderContext{
		Emit:      into,
		Files:     files,
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
		if over := provider.Overrides(target); len(over) > 0 {
			ctx.Overrides[s.name] = over
		}
	}
	return ctx
}

// commit opens the run's own sink, writes every plan's staged files
// into it, in plan order and then in the order each plan rendered
// them, and commits once. A composition declaring no output opens
// nothing. The write is sequential where the render was parallel,
// so one run writes one tree in one order however the plans
// interleaved.
//
// A failed write discards the staging and commits nothing. A commit
// refused part-way returns its error together with the records of
// the files it wrote before the refusal, so the report lists every
// file that changed on disk.
func (w *Workspace) commit(staged [][]staged) ([]output.Written, error) {
	if w.open == nil {
		return nil, nil
	}
	sink, err := w.open()
	if err != nil {
		return nil, fmt.Errorf("workspace: open the output: %w", err)
	}
	if sink == nil {
		return nil, errors.New("workspace: the output's open function returned (nil, nil)")
	}
	for _, files := range staged {
		for _, f := range files {
			if werr := sink.Write(f.path, f.body); werr != nil {
				return nil, fmt.Errorf("workspace: stage %s: %w", f.path, discarding(sink, werr))
			}
		}
	}
	written, err := sink.Commit()
	if err != nil {
		return written, fmt.Errorf("workspace: commit the output: %w", err)
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
