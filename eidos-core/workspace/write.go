// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"io/fs"
	"slices"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// stagedFile is one rendered file on its way to the sink: the path the
// plan's layout routed it to, the stamped bytes, and what the manifest
// and the findings record about it.
type stagedFile struct {
	path string
	body []byte
	// plugins are the emitters whose units assembled the file and the
	// plugins that appended into the units' slots, distinct and sorted.
	plugins []plugin.ID
	// sources are the canonical identities of the declarations the file
	// derives from, sorted, in the spelling symbol.Parse reads back.
	sources []string
	// at is the origin of the file's first declaration, and the file
	// itself where the origin has no position: where a finding about
	// the file is positioned.
	at position.Pos
	// first is the file's first declaration, which a finding about the
	// file names through [layout.Describe], and nil for a file without
	// one.
	first symbol.Symbol
	// pkg is the package the file declares, and findings are what the
	// render reported about the file. A recording run's groups set group,
	// the key unit of the file's group, and names, the file-level names
	// the file declares, which the file's artifact records.
	pkg      symbol.Identity
	findings []diag.Diag
	group    plugin.UnitRef
	names    []plugin.NameEntry
	// packages are the packages whose declared names the render read for
	// the file, in the language of the plan's target and in path order.
	packages []symbol.Identity
}

// write routes one plan's settled store to files against the run's
// source tree, renders the files and stamps each one. others are the
// names of the plan's files that a warm run keeps, and nil for a store
// that contains the whole plan. It returns the stamped files, and the
// routed files that rendered at the same places. A defect in the inputs
// of the routing returns an error.
func (w *Workspace) write(
	pl *compiledPlan, ix *plugin.Index, src tree, into *plugin.Emit, sink *diag.Sink, others plugin.Names,
) ([]stagedFile, []plugin.File, error) {
	in := layout.Input{
		Emit:       into,
		Config:     pl.routing,
		Outputs:    pl.outputs,
		Speller:    pl.backend,
		Packager:   pl.backend,
		Index:      ix,
		Residents:  src.residents,
		Directives: w.directives,
		Modules:    src.modules,
		Others:     others,
		Target:     pl.backend.Target(),
		Sink:       sink,
	}
	files, err := layout.Route(in)
	if err != nil {
		return nil, nil, fmt.Errorf("route: %w", err)
	}
	return render(pl, ix, src, into, files, sink)
}

// render drives one plan's backend over the files its layout routed
// and returns what the backend produced, stamped through the plan's
// contract, at each file's routed path and in path order. It also returns
// the routed files the backend rendered, at the same places.
func render(
	pl *compiledPlan, ix *plugin.Index, src tree, into *plugin.Emit, files []plugin.File, sink *diag.Sink,
) ([]stagedFile, []plugin.File, error) {
	rendered, err := pl.backend.Render(renderContext(pl, src, into, files, sink))
	if err != nil {
		return nil, nil, fmt.Errorf("render: %w", err)
	}
	lang := symbol.Lang(pl.backend.Target())
	out := make([]stagedFile, 0, len(rendered))
	routedFiles := make([]plugin.File, 0, len(rendered))
	routed := 0
	for _, f := range rendered {
		body, err := pl.contract.Stamp(f)
		if err != nil {
			return nil, nil, fmt.Errorf("stamp %s: %w", f.Path, err)
		}
		// The render returns the routed files it rendered, in their
		// path order, so the routed file of each is found by walking on.
		for routed < len(files) && files[routed].Path != f.Path {
			routed++
		}
		if routed == len(files) {
			return nil, nil, fmt.Errorf("render: %s returns %s, which the layout did not route",
				pl.backend.Name(), f.Path)
		}
		staged := stagedFile{
			path: f.Path, body: body, plugins: f.Plugins, at: position.Pos{File: f.Path},
			pkg: files[routed].Pkg, findings: f.Findings,
		}
		if len(f.Packages) > 0 {
			staged.packages = make([]symbol.Identity, 0, len(f.Packages))
			for _, p := range f.Packages {
				pkg := symbol.Identity{Lang: lang, Package: p, Kind: symbol.KindPackage}
				staged.packages = append(staged.packages, pkg)
			}
		}
		describeFile(&staged, &files[routed], ix)
		out = append(out, staged)
		routedFiles = append(routedFiles, files[routed])
	}
	return out, routedFiles, nil
}

// describeFile records what the manifest and the findings read off a
// routed file: the identities its units derive from, and its first
// declaration and that declaration's origin.
func describeFile(staged *stagedFile, f *plugin.File, ix *plugin.Index) {
	n := 0
	for _, u := range f.Units {
		n += len(u.Origins)
	}
	if n > 0 {
		staged.sources = make([]string, 0, n)
		for _, u := range f.Units {
			for _, origin := range u.Origins {
				staged.sources = append(staged.sources, origin.String())
			}
		}
		slices.Sort(staged.sources)
		staged.sources = slices.Compact(staged.sources)
	}
	if len(f.Units) == 0 || len(f.Units[0].Decls) == 0 {
		return
	}
	staged.first = f.Units[0].Decls[0]
	origin, _ := emit.OriginOf(staged.first)
	if origin.IsZero() {
		return
	}
	if s, held := ix.Lookup(origin); held {
		if pos := s.Position(); !pos.IsZero() {
			staged.at = pos
		}
	}
}

// renderContext assembles what the pass reads: the settled store, the
// routed files, the plan's schedule, the template surfaces its
// generators declare for the backend's target, and the declared names of
// the packages that the run loaded in the language of the target. The
// names cover every package of the run, because a file can import a
// package outside the sources of its plan.
func renderContext(
	pl *compiledPlan, src tree, into *plugin.Emit, files []plugin.File, sink *diag.Sink,
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
	if src.packages != nil {
		lang := symbol.Lang(target)
		ctx.Names = func(path string) (string, bool) {
			p, loaded := src.packages(symbol.Identity{Lang: lang, Package: path, Kind: symbol.KindPackage})
			if !loaded {
				return "", false
			}
			return p.Name, true
		}
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
