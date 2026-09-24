// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// skeletonName labels the parsed file skeleton.
const skeletonName = "file"

// Pass is one composed language's render procedure. A Pass is safe
// for concurrent use: everything it holds is fixed at [New], and
// every render call owns its own frames. Within one call, files
// render in parallel on up to GOMAXPROCS workers. The language's
// Scaffold, Imports, Finalise and Cluster, every helper in its
// Funcs and in a context's Funcs, and every read of a context's
// trees run on those workers concurrently, so each must be safe
// for concurrent use. Naming and Split run on the calling
// goroutine.
type Pass struct {
	name     plugin.ID
	kinds    map[symbol.Kind]*template.Template
	groups   map[GroupName]*template.Template
	file     *template.Template
	shared   template.FuncMap
	spell    Naming
	split    Split
	cluster  Cluster
	scaffold func(s emit.Stmt, set *ImportSet) ([]byte, error)
	imports  func(set *ImportSet) string
	final    func(src []byte) ([]byte, error)
	coverage Coverage
}

// Coverage returns the language's declared fact coverage, so a
// consumer holding the pass reads the same data the guard does.
func (p *Pass) Coverage() Coverage { return p.coverage }

// defaultFile is the skeleton a language that sets none takes.
const defaultFile = "{{" + BuiltinImports + "}}{{" + BuiltinDecls + "}}"

// New composes a language into its pass.
//
// It refuses, collecting every fault: an empty kind-template set,
// a template that does not parse, a missing naming and a missing
// formatter. The kit converts these to panics at its own Build,
// because there they are declaration defects; here they are
// composition faults, and a composition reads every fault at once.
func New(name plugin.ID, l Language) (*Pass, error) {
	var faults []error
	if len(l.Kinds) == 0 {
		faults = append(faults,
			errors.New("render: the language spells no kinds"))
	}
	for _, name := range slices.Sorted(maps.Keys(l.Funcs)) {
		if reserved(name) {
			faults = append(faults, fmt.Errorf(
				"render: the shared vocabulary claims %q, which is a builtin", name,
			))
		}
	}
	kinds := make(map[symbol.Kind]*template.Template, len(l.Kinds))
	for _, k := range slices.Sorted(maps.Keys(l.Kinds)) {
		t, err := template.New(k.String()).Funcs(unbound()).Funcs(l.Funcs).Parse(l.Kinds[k])
		if err != nil {
			faults = append(faults, fmt.Errorf("render: the %s template: %w", k, err))
			continue
		}
		kinds[k] = t
	}
	if l.Cluster != nil && len(l.Groups) == 0 {
		faults = append(faults, errors.New(
			"render: the language clusters declarations and declares no group templates",
		))
	}
	groups := make(map[GroupName]*template.Template, len(l.Groups))
	for _, g := range slices.Sorted(maps.Keys(l.Groups)) {
		t, err := template.New(string(g)).Funcs(unbound()).Funcs(l.Funcs).Parse(l.Groups[g])
		if err != nil {
			faults = append(faults, fmt.Errorf("render: the %s group template: %w", g, err))
			continue
		}
		groups[g] = t
	}
	if l.Naming == nil {
		faults = append(faults,
			errors.New("render: the language spells no filenames"))
	}
	if l.Scaffold == nil {
		faults = append(faults,
			errors.New("render: the language spells no scaffolding"))
	}
	if l.Imports == nil {
		faults = append(faults,
			errors.New("render: the language renders no import block"))
	}
	if l.Finalise == nil {
		faults = append(faults,
			errors.New("render: the language holds no formatter"))
	}
	skeleton := l.File
	if skeleton == "" {
		skeleton = defaultFile
	}
	file, err := template.New(skeletonName).Funcs(unbound()).Funcs(l.Funcs).Parse(skeleton)
	if err != nil {
		faults = append(faults, fmt.Errorf("render: the file skeleton: %w", err))
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	shared := maps.Clone(l.Funcs)
	if shared == nil {
		shared = template.FuncMap{}
	}
	return &Pass{
		name: name, kinds: kinds, groups: groups, file: file, shared: shared,
		spell: l.Naming, split: l.Split, cluster: l.Cluster,
		scaffold: l.Scaffold, imports: l.Imports, final: l.Finalise,
		coverage: l.Coverage,
	}, nil
}

// group is one output file in the making: its name, the package
// that owns it, and its units in the store's total order.
type group struct {
	name  string
	pkg   symbol.Identity
	units []plugin.Unit
}

// fileKey addresses one output file: the spelled name under the
// owning package, because two packages spell the same filename and
// stay two files, whatever directory layout places them in.
type fileKey struct {
	pkg  symbol.Identity
	name string
}

// Render takes one plan's emit through the procedure and returns the
// files as values, in name order. Findings attach to the context's
// sink at the rendered filename; a returned error is a defect in
// the inputs, never a finding.
func (p *Pass) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	if ctx == nil || ctx.Emit == nil {
		return nil, errors.New("render: the pass needs a plan's emit store")
	}
	if ctx.Sink == nil {
		return nil, errors.New("render: the pass needs a sink for its findings")
	}

	origin := ctx.Plugin
	if origin == "" {
		origin = p.name
	}

	byFile := map[fileKey]*group{}
	var order []*group
	for u := range ctx.Emit.Units() {
		units := []plugin.Unit{u}
		if p.split != nil {
			units = p.split(u)
			if len(units) == 0 && len(u.Decls) > 0 {
				// A split that returns nothing for a populated unit
				// would vanish its declarations without a finding,
				// which is the narrowing the engine exists to
				// refuse.
				ctx.Sink.Errorf(RefusedTemplate, p.unitPos(u), origin,
					"%s splits a %s unit of %d declarations into nothing, and they are skipped",
					p.name, u.Word, len(u.Decls))
				continue
			}
		}
		for _, su := range units {
			key := fileKey{pkg: su.Pkg, name: p.spell(su)}
			g, held := byFile[key]
			if !held {
				g = &group{name: key.name, pkg: key.pkg}
				byFile[key] = g
				order = append(order, g)
			}
			g.units = append(g.units, su)
		}
	}
	slices.SortFunc(order, func(a, b *group) int {
		if c := a.pkg.Compare(b.pkg); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})

	seed := &frame{pass: p, sink: ctx.Sink, origin: origin, trees: ctx.Trees}
	merged := seed.mergeVocabulary(ctx)

	// Files are independent after grouping, so workers share the
	// render. Each worker has its own frame, buffers and parsed
	// reference templates. The worker count is bounded by the
	// parallelism, never by the file count, and the trees are read
	// concurrently. A worker collects each file's findings apart, and
	// they replay into the context's sink in file order. The output
	// order and the report order are therefore the precomputed ones,
	// whatever order the workers finish in.
	type rendered struct {
		body    []byte
		plugins []plugin.ID
		sources []string
		found   []diag.Diag
		held    bool
	}
	results := make([]rendered, len(order))
	workers := min(runtime.GOMAXPROCS(0), len(order))
	var next atomic.Int64
	var bindErr atomic.Pointer[error]
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			w := &frame{
				pass: p, sink: diag.NewSink(), origin: origin,
				trees: ctx.Trees, merged: merged,
			}
			b, err := p.bind(w)
			if err != nil {
				bindErr.CompareAndSwap(nil, &err)
				return
			}
			w.bound = b
			for {
				i := int(next.Add(1)) - 1
				if i >= len(order) {
					return
				}
				body, held := w.file(order[i], b)
				r := rendered{body: body, held: held}
				if held {
					r.plugins, r.sources = derivation(order[i].units)
				}
				if found := slices.Collect(w.sink.All()); len(found) > 0 {
					r.found = found
					w.sink = diag.NewSink()
				}
				results[i] = r
			}
		})
	}
	wg.Wait()
	if err := bindErr.Load(); err != nil {
		return nil, *err
	}
	for i := range results {
		for _, d := range results[i].found {
			ctx.Sink.Report(d)
		}
	}
	files := make([]plugin.RenderedFile, 0, len(order))
	for i, g := range order {
		if results[i].held {
			files = append(files, plugin.RenderedFile{
				Name: g.name, Pkg: g.pkg,
				Plugins: results[i].plugins, Sources: results[i].sources,
				Body: results[i].body,
			})
		}
	}
	return files, nil
}

// unitPos positions a finding about a unit that has no file yet:
// the unit's routing key, or the pass's own name for a plan unit,
// which derives from no source.
func (p *Pass) unitPos(u plugin.Unit) position.Pos {
	if u.Key == "" {
		return position.Pos{File: string(p.name)}
	}
	return position.Pos{File: u.Key}
}

// derivation reads a file's emitters and the keys it derives from
// off the units that assembled it, distinct and sorted. A unit
// carrying no key contributes no source, which is what a plan file
// is: it derives from the plan rather than from any declaration.
func derivation(units []plugin.Unit) (plugins []plugin.ID, sources []string) {
	plugins = make([]plugin.ID, 0, len(units))
	for _, u := range units {
		plugins = append(plugins, u.Plugin)
		if u.Key != "" {
			sources = append(sources, u.Key)
		}
	}
	slices.Sort(plugins)
	slices.Sort(sources)
	return slices.Compact(plugins), slices.Compact(sources)
}

// fileView is what the skeleton executes over: the file's spelled
// name and its owning package identity.
type fileView struct {
	Name string
	Pkg  symbol.Identity
}

// frame is one render call's mutable state: the buffer the files
// share, the sink, and the position and emitter of whatever is
// under render, which the builtins read. One frame serves one
// worker, which is what keeps a Pass safe for concurrent renders.
type frame struct {
	pass *Pass
	sink *diag.Sink
	// origin is the identity findings carry: the context's plugin,
	// or the pass's own name where the context carries none.
	origin  diag.Origin
	trees   map[plugin.ID]fs.FS
	merged  template.FuncMap
	out     bytes.Buffer
	scratch bytes.Buffer // one declaration's render, adopted on success
	// refCache contains the referenced templates this frame parsed,
	// keyed by the emitting plugin and the name, because each
	// plugin's name resolves in its own tree. Their builtins are
	// bound to this frame and write into the body under execution.
	refCache map[refKey]*template.Template
	// place is the placement of the body a reference template is
	// executing for, nil between executions.
	place   *placement
	fileOut bytes.Buffer
	set     ImportSet
	at      position.Pos
	plugin  plugin.ID
	// bound holds the frame's own template set once bind ran, which
	// is what the nested builtin renders through.
	bound *bound
}

// refKey addresses one parsed reference template: the emitting
// plugin whose tree it resolved in, and its name there.
type refKey struct {
	plugin plugin.ID
	name   string
}

const (
	// admitted enters the merged vocabulary.
	admitted admission = iota
	// claimsBuiltin names a builtin, which no vocabulary may claim.
	claimsBuiltin
	// shadowsUndeclared replaces a shared helper without declaring
	// the override.
	shadowsUndeclared
)

// bound is one render call's executable templates: the kind
// templates and the file skeleton, cloned from the parse and bound
// to the call's frame.
type bound struct {
	kinds  map[symbol.Kind]*template.Template
	groups map[GroupName]*template.Template
	file   *template.Template
}

// bind clones the parsed templates for this call and binds the
// builtins to its frame: parse happened once at New, and no two
// calls share an executing tree.
func (p *Pass) bind(f *frame) (*bound, error) {
	builtins := template.FuncMap{
		BuiltinBody:    f.body,
		BuiltinUse:     f.use,
		BuiltinImports: f.importsBlock,
		BuiltinDecls:   f.decls,
		BuiltinNested:  f.nested,
	}
	kinds := make(map[symbol.Kind]*template.Template, len(p.kinds))
	for k, t := range p.kinds {
		c, err := t.Clone()
		if err != nil {
			return nil, fmt.Errorf("render: cloning the %s template: %w", k, err)
		}
		kinds[k] = c.Funcs(f.merged).Funcs(builtins)
	}
	groups := make(map[GroupName]*template.Template, len(p.groups))
	for g, t := range p.groups {
		c, err := t.Clone()
		if err != nil {
			return nil, fmt.Errorf("render: cloning the %s group template: %w", g, err)
		}
		groups[g] = c.Funcs(f.merged).Funcs(builtins)
	}
	file, err := p.file.Clone()
	if err != nil {
		return nil, fmt.Errorf("render: cloning the file skeleton: %w", err)
	}
	return &bound{
		kinds: kinds, groups: groups,
		file: file.Funcs(f.merged).Funcs(builtins),
	}, nil
}

// use records one import path into the file under render; it is
// the builtin a kind template qualifies with. A second argument
// records the name the import binds — an alias, a side-effect
// blank — for the languages whose import form binds one; more
// than one refuses, because one call records one binding.
func (f *frame) use(path string, name ...string) (string, error) {
	switch len(name) {
	case 0:
		f.set.Add(path)
	case 1:
		f.set.AddNamed(path, name[0])
	default:
		return "", fmt.Errorf("render: use records one binding, got %d", len(name))
	}
	return "", nil
}

// importsBlock renders the file's collected set the language's
// way; it is the skeleton's imports builtin, executed after the
// declarations rendered, which is what makes the set complete.
func (f *frame) importsBlock() (string, error) {
	return f.pass.imports(&f.set), nil
}

// decls returns the file's rendered declarations; it is the
// skeleton's decls builtin.
func (f *frame) decls() (string, error) {
	return f.out.String(), nil
}

// file renders one group into the frame's shared buffer: every
// unit's declarations in the order the flush fixed, then the
// formatter. A false answer means the formatter refused and the
// finding is on the sink. The buffer is reset per file and its
// bytes are copied out, so a formatter that returns its input, as
// a pass-through one does, never aliases storage a following file
// overwrites.
func (f *frame) file(g *group, b *bound) ([]byte, bool) {
	f.out.Reset()
	f.fileOut.Reset()
	f.set.Reset()
	f.set.SetHome(g.pkg.Package)
	// The package qualifies the spelled name, because two packages
	// can spell one filename and the two files remain distinct.
	f.at = position.Pos{File: path.Join(g.pkg.Package, g.name)}
	for _, u := range g.units {
		f.plugin = u.Plugin
		f.declRun(u, b)
	}
	if err := b.file.Execute(&f.fileOut, fileView{Name: g.name, Pkg: g.pkg}); err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"the file skeleton refused %s: %v", g.name, err)
		return nil, false
	}
	body, err := f.pass.final(f.fileOut.Bytes())
	if err != nil {
		f.sink.Errorf(UnformattedFile, f.at, f.origin,
			"%s cannot be formatted and is withheld: %v", g.name, err)
		return nil, false
	}
	return bytes.Clone(body), true
}
