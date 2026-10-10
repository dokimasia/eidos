// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
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
// for concurrent use: its fields are fixed at [New], and every
// render call has frames of its own. Within one call, files
// render in parallel on up to GOMAXPROCS workers. The language's
// Scaffold, Imports, Finalise and Cluster, every helper in its
// Funcs and in a context's Funcs, and every read of a context's
// trees run on those workers concurrently, so each must be safe
// for concurrent use. Naming and Split run where the plan's layout
// calls [Pass.FileName] and [Pass.SplitUnit], on its goroutine.
type Pass struct {
	name    plugin.ID
	kinds   map[symbol.Kind]*template.Template
	refused map[symbol.Kind]string
	groups  map[GroupName]*template.Template
	file    *template.Template
	// shared is the vocabulary bound to a set of the pass's own: the
	// names the parse and the override checks read. A render binds
	// vocab per worker instead.
	shared   template.FuncMap
	vocab    func(set *ImportSet) template.FuncMap
	spell    Naming
	split    Split
	cluster  Cluster
	scaffold func(s emit.Stmt, set *ImportSet) ([]byte, error)
	imports  func(set *ImportSet) string
	final    func(src []byte) ([]byte, error)
	coverage Coverage
	// memberIndent is the language's [Language.MemberIndent], which the
	// memberbody builtin writes before each line of a member's content.
	memberIndent string
}

// Coverage returns the language's declared fact coverage, so a
// consumer of the pass reads the same data the guard does.
func (p *Pass) Coverage() Coverage { return p.coverage }

// RefusedKinds implements [Refuser]: the kinds the language refuses,
// each with its reason, in a map of the caller's own, so a consumer
// of the pass reads the refusals the render reports.
func (p *Pass) RefusedKinds() map[symbol.Kind]string { return maps.Clone(p.refused) }

// defaultFile is the skeleton a language that sets none takes.
const defaultFile = "{{" + BuiltinImports + "}}{{" + BuiltinDecls + "}}"

// New composes a language into its pass. It returns an error
// joining every fault it finds, because a composition reads every
// fault at once:
//
//   - an empty kind-template set;
//   - a kind both spelt and refused, and a refusal without a reason;
//   - a kind template, group template or file skeleton that does
//     not parse;
//   - a builtin name the shared vocabulary claims;
//   - a Cluster without group templates;
//   - a missing naming, scaffold, import renderer or formatter.
//
// The kit converts these to panics at its own Build, because there
// they are declaration defects.
func New(name plugin.ID, l Language) (*Pass, error) {
	var faults []error
	if len(l.Kinds) == 0 {
		faults = append(faults,
			errors.New("render: the language spells no kinds"))
	}
	shared := template.FuncMap{}
	if l.Funcs != nil {
		shared = l.Funcs(&ImportSet{})
	}
	for _, name := range slices.Sorted(maps.Keys(shared)) {
		if reserved(name) {
			faults = append(faults, fmt.Errorf(
				"render: the shared vocabulary claims %q, which is a builtin", name,
			))
		}
	}
	kinds := make(map[symbol.Kind]*template.Template, len(l.Kinds))
	for _, k := range slices.Sorted(maps.Keys(l.Kinds)) {
		t, err := template.New(k.String()).Funcs(unbound()).Funcs(shared).Parse(l.Kinds[k])
		if err != nil {
			faults = append(faults, fmt.Errorf("render: the %s template: %w", k, err))
			continue
		}
		kinds[k] = t
	}
	for _, k := range slices.Sorted(maps.Keys(l.Refused)) {
		if _, spelt := l.Kinds[k]; spelt {
			faults = append(faults, fmt.Errorf("render: the language spells and refuses the %s kind", k))
		}
		if l.Refused[k] == "" {
			faults = append(faults, fmt.Errorf("render: the language refuses the %s kind without a reason", k))
		}
	}
	if l.Cluster != nil && len(l.Groups) == 0 {
		faults = append(faults, errors.New(
			"render: the language clusters declarations and declares no group templates",
		))
	}
	groups := make(map[GroupName]*template.Template, len(l.Groups))
	for _, g := range slices.Sorted(maps.Keys(l.Groups)) {
		t, err := template.New(string(g)).Funcs(unbound()).Funcs(shared).Parse(l.Groups[g])
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
			errors.New("render: the language declares no formatter"))
	}
	skeleton := l.File
	if skeleton == "" {
		skeleton = defaultFile
	}
	file, err := template.New(skeletonName).Funcs(unbound()).Funcs(shared).Parse(skeleton)
	if err != nil {
		faults = append(faults, fmt.Errorf("render: the file skeleton: %w", err))
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Pass{
		name: name, kinds: kinds, refused: maps.Clone(l.Refused),
		groups: groups, file: file, shared: shared,
		vocab: l.Funcs, spell: l.Naming, split: l.Split, cluster: l.Cluster,
		scaffold: l.Scaffold, imports: l.Imports, final: l.Finalise,
		coverage: l.Coverage, memberIndent: l.MemberIndent,
	}, nil
}

// FileName spells one unit's filename through the language's Naming:
// the kit's [plugin.FileSpeller] half the plan's layout calls.
func (p *Pass) FileName(u plugin.Unit) string { return p.spell(u) }

// SplitUnit reshapes one unit through the language's Split, and
// returns the unit whole for a language that declares none.
func (p *Pass) SplitUnit(u plugin.Unit) []plugin.Unit {
	if p.split == nil {
		return []plugin.Unit{u}
	}
	return p.split(u)
}

// Render takes the context's routed files through the procedure and
// returns them as values, in the context's order, which the layout
// sorts by path. A file whose every declaration is skipped is
// withheld. Findings attach to the context's sink at the file's path,
// and a rendered file lists its own in its Findings. A returned error
// is a defect in the inputs, never a finding.
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
	order := ctx.Files

	seed := &frame{pass: p, sink: ctx.Sink, origin: origin, trees: ctx.Trees}
	merged := seed.mergeVocabulary(ctx)

	// Files are independent of each other, so workers share the
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
				body, held := w.file(&order[i], b)
				r := rendered{body: body, held: held}
				if held {
					r.plugins, r.sources = derivation(order[i].Units)
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
	for i := range order {
		if results[i].held {
			files = append(files, plugin.RenderedFile{
				Path: order[i].Path, Pkg: order[i].Pkg,
				Plugins: results[i].plugins, Sources: results[i].sources,
				Body: results[i].body, Findings: results[i].found,
			})
		}
	}
	return files, nil
}

// derivation reads a file's plugins and the keys it derives from off
// the units that assembled it, distinct and sorted: each unit's
// emitter and the plugins that appended into its slots. A unit
// without a key contributes no source, which is what a plan file is:
// it derives from the plan and from no declaration.
func derivation(units []plugin.Unit) (plugins []plugin.ID, sources []string) {
	plugins = make([]plugin.ID, 0, len(units))
	for _, u := range units {
		plugins = append(plugins, u.Plugin)
		plugins = append(plugins, u.Contributors...)
		if u.Key != "" {
			sources = append(sources, u.Key)
		}
	}
	slices.Sort(plugins)
	slices.Sort(sources)
	return slices.Compact(plugins), slices.Compact(sources)
}

// fileView is what the skeleton executes over: the file's spelled
// name and its package identity.
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
	// origin is the identity findings report under: the context's
	// plugin, or the pass's own name where the context names none.
	origin diag.Origin
	trees  map[plugin.ID]fs.FS
	merged template.FuncMap
	// vocab is the language's vocabulary bound to this frame's
	// import set, which bind sets and a reference template parses
	// with.
	vocab   template.FuncMap
	out     bytes.Buffer
	scratch bytes.Buffer // one declaration's render, adopted on success
	// refCache contains the referenced templates this frame parsed,
	// keyed by the plugin whose tree each resolved in and the name,
	// because one name resolves in each plugin's tree apart. Their
	// builtins are bound to this frame and write into the body under
	// execution.
	refCache map[refKey]*template.Template
	// place is the placement of the body a reference template is
	// executing for, nil between executions.
	place   *placement
	fileOut bytes.Buffer
	set     ImportSet
	at      position.Pos
	plugin  plugin.ID
	// bound is the frame's own template set once bind ran, which the
	// nested builtin renders through.
	bound *bound
}

// refKey addresses one parsed reference template: the plugin whose
// tree it resolved in, and its name there.
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
// language's vocabulary and the builtins to its frame: the
// vocabulary records into the frame's import set, the plugins'
// merged helpers override it, and the builtins override both.
// Parse happened once at New, and no two calls share an executing
// tree.
func (p *Pass) bind(f *frame) (*bound, error) {
	f.vocab = p.shared
	if p.vocab != nil {
		f.vocab = p.vocab(&f.set)
	}
	builtins := template.FuncMap{
		BuiltinBody:       f.body,
		BuiltinMemberBody: f.memberBody,
		BuiltinUse:        f.use,
		BuiltinImports:    f.importsBlock,
		BuiltinDecls:      f.decls,
		BuiltinNested:     f.nested,
	}
	kinds := make(map[symbol.Kind]*template.Template, len(p.kinds))
	for k, t := range p.kinds {
		c, err := t.Clone()
		if err != nil {
			return nil, fmt.Errorf("render: cloning the %s template: %w", k, err)
		}
		kinds[k] = c.Funcs(f.vocab).Funcs(f.merged).Funcs(builtins)
	}
	groups := make(map[GroupName]*template.Template, len(p.groups))
	for g, t := range p.groups {
		c, err := t.Clone()
		if err != nil {
			return nil, fmt.Errorf("render: cloning the %s group template: %w", g, err)
		}
		groups[g] = c.Funcs(f.vocab).Funcs(f.merged).Funcs(builtins)
	}
	file, err := p.file.Clone()
	if err != nil {
		return nil, fmt.Errorf("render: cloning the file skeleton: %w", err)
	}
	return &bound{
		kinds: kinds, groups: groups,
		file: file.Funcs(f.vocab).Funcs(f.merged).Funcs(builtins),
	}, nil
}

// use records one import path into the file under render; it is
// the builtin a kind template qualifies with. A second argument
// records the name the import binds, such as an alias or a
// side-effect blank, for the languages whose import form binds one.
// More than one returns an error, because one call records one
// binding.
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

// file renders one routed file into the frame's shared buffer: every
// unit's declarations in the order the flush fixed, then the
// formatter. The file's package is the import home, so a reference
// into it spells bare and every other package it names is imported.
// A false result means the file is withheld, and the findings that
// explain it are on the sink: the skeleton or the formatter refused
// the file, or every declaration of it was skipped, which leaves no
// content to stamp. The buffer is reset per file and its bytes are
// copied out, so a formatter that returns its input, as a
// pass-through one does, never aliases storage a following file
// overwrites.
func (f *frame) file(g *plugin.File, b *bound) ([]byte, bool) {
	f.out.Reset()
	f.fileOut.Reset()
	f.set.Reset()
	f.set.SetHome(g.Pkg.Package)
	// The file's own declarations take their names before anything
	// renders, so no import binds a name the file declares.
	declared := 0
	for _, u := range g.Units {
		declared += len(u.Decls)
		for _, d := range u.Decls {
			f.set.Reserve(emit.DeclaredName(d))
		}
	}
	f.at = position.Pos{File: g.Path}
	spelt := false
	for _, u := range g.Units {
		f.plugin = u.Plugin
		spelt = f.declRun(u, b) || spelt
	}
	if declared > 0 && !spelt {
		return nil, false
	}
	name := path.Base(g.Path)
	if err := b.file.Execute(&f.fileOut, fileView{Name: name, Pkg: g.Pkg}); err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"the file skeleton refused %s: %v", g.Path, err)
		return nil, false
	}
	body, err := f.pass.final(f.fileOut.Bytes())
	if err != nil {
		f.sink.Errorf(UnformattedFile, f.at, f.origin,
			"%s cannot be formatted and is withheld: %v", g.Path, err)
		return nil, false
	}
	return bytes.Clone(body), true
}
