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
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// UnspeltKind reports a declaration whose kind the target language
// holds no template for: the declaration is skipped and the file
// renders without it.
var UnspeltKind = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 21, Meaning: "a declaration's kind has no template in the target language",
})

// UnformattedFile reports a rendered file the language formatter
// refused: the file is withheld, because the sink never receives
// an unformatted file, and the pass continues with its siblings.
var UnformattedFile = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 22, Meaning: "the language formatter refused a rendered file",
})

// RefusedTemplate reports a template that exists and still failed
// at execute time, naming the emitting plugin: the declaration is
// skipped and the file renders without it.
var RefusedTemplate = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 23, Meaning: "a template refused a declaration at execute time",
})

// BodyConflict reports a body holding more than one content form:
// the standard and named slots still render, and no contested
// content is guessed at.
var BodyConflict = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 24, Meaning: "a body holds more than one content form",
})

// UnresolvedRef reports a template reference nothing returns: no
// tree declared for the emitting plugin, no template of that name
// in it, or a template that does not parse. The body falls back to
// its slots, so the extension points survive the broken claim.
var UnresolvedRef = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 25, Meaning: "a template reference resolves to nothing in its emitting plugin's tree",
})

// UndeclaredOverride reports a plugin helper shadowing a shared
// vocabulary name without declaring the override: the shared
// helper stands, because a silent replacement is the drift
// byte-identity cannot tolerate.
var UndeclaredOverride = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 27, Meaning: "a plugin shadows a shared template helper without declaring the override",
})

// DroppedSlots reports a body-claiming template that placed no
// marker for pending slot content: the template owns the layout,
// so nothing is appended for it, and the Error names the emitting
// plugin and counts what went unplaced. The contributor cannot be
// named, because a slot statement carries no attribution.
var DroppedSlots = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 26, Meaning: "a body-claiming template places no marker for pending contributions",
})

// UnknownGroup reports a cluster naming a group the language
// declares no template for: the cluster's declarations are
// skipped and the file renders without them.
var UnknownGroup = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 28, Meaning: "a cluster names a group the target language declares no template for",
})

// The builtin names every template resolves against: what a kind
// template, a file skeleton or a body-claiming template calls, and
// what the template lint checks for. No vocabulary may claim them.
const (
	// BuiltinBody places a callable's content; a kind template
	// calls it with the declaration under render.
	BuiltinBody = "body"
	// BuiltinUse records one import path into the file under
	// render.
	BuiltinUse = "use"
	// BuiltinImports renders the file's collected import block; it
	// is the skeleton's.
	BuiltinImports = "imports"
	// BuiltinDecls returns the file's rendered declarations; it is
	// the skeleton's.
	BuiltinDecls = "decls"
	// BuiltinSlots places everything pending in the fixed order:
	// the body-claiming template's catch-all marker.
	BuiltinSlots = "slots"
	// BuiltinSlot places one named slot where the template says.
	BuiltinSlot = "slot"
	// BuiltinNested renders one nested declaration through its
	// kind template, every line behind the given indentation: how
	// a host places its inner declarations at member depth. A
	// nested kind without a template reports and spells nothing,
	// which is the unspelt-kind rule one level down.
	BuiltinNested = "nested"
)

// skeletonName labels the parsed file skeleton.
const skeletonName = "file"

// Naming spells a unit's filename for one target: the join and
// extension of the family's word, the tag's treatment, and the
// cardinality key's stem. It is total: every unit the plan admits
// returns a name, and units returning one name assemble one file,
// which is how two plugins share it.
type Naming func(u plugin.Unit) string

// Split reshapes one unit into the units the target files
// separately: a language that names a file after the type it
// holds returns one unit per file-level type, and its Naming
// reads the lone type's name, so the demanded filename spells
// while the routing key keeps carrying the source derivation. A
// nil Split keeps every unit whole. The pass applies it before
// naming, preserves order, and calls it once per unit, so a pure
// function keeps the render deterministic.
type Split func(u plugin.Unit) []plugin.Unit

// GroupName names a declaration cluster a group template spells.
type GroupName string

// Clustered is one cluster: the group template that spells it and
// the declarations it holds, in unit order.
type Clustered struct {
	Group GroupName
	Decls []symbol.Symbol
}

// Cluster assigns one unit's declarations to named groups: a
// language that renders methods only inside a grouped block
// gathers them by the type they attach to. A declaration the
// function leaves unassigned renders through its kind template; a
// cluster renders through the group template its name selects, in
// place of its members' kind templates, at the position of its
// first member. A declaration two clusters claim goes to the
// first, and a member the unit does not hold is ignored. Clusters
// stay inside one unit, so plugin attribution and canonical order
// survive. A nil Cluster leaves every declaration a singleton.
type Cluster func(decls []symbol.Symbol) []Clustered

// Language is what a target genuinely varies in; the pass owns
// everything else.
type Language struct {
	// Kinds holds the template source per emit kind: how the
	// language spells each declaration.
	Kinds map[symbol.Kind]string
	// File is the file skeleton, executed once per file over the
	// file's name and owning package; empty takes the default,
	// imports then declarations. The clause a language opens its
	// files with is the skeleton's own to spell, because not every
	// language has one. The header is not the skeleton's: the
	// output contract prepends it after the formatter ran.
	File string
	// Funcs is the language's shared template vocabulary,
	// registered once into the overrideable bucket: every kind
	// template, file skeleton and reference template calls it, and
	// a declared override replaces one name for all of them.
	Funcs template.FuncMap
	// Naming spells each unit's filename.
	Naming Naming
	// Split reshapes each unit before naming; nil files every unit
	// whole.
	Split Split
	// Cluster assigns a unit's declarations to named groups; nil
	// leaves every declaration a singleton.
	Cluster Cluster
	// Groups holds the template source per group name a Cluster
	// selects.
	Groups map[GroupName]string
	// Scaffold spells one statement of the neutral vocabulary the
	// language's way, recording into the file's import set whatever
	// it qualified with. It is the printer the body builtin calls
	// for slot contributions and scaffold content alike; a
	// statement the language cannot spell returns an error, and the
	// declaration is skipped under the execute-time code.
	Scaffold func(s emit.Stmt, set *ImportSet) ([]byte, error)
	// Imports renders one file's collected set as the block the
	// language's own formatter would leave: grouping and sorting
	// are language facts.
	Imports func(set *ImportSet) string
	// Finalise is the language formatter, run last per file.
	Finalise func(src []byte) ([]byte, error)
	// Coverage is the language's declared fact coverage. Declared,
	// it arms the guard: a stated fact the declaration refuses
	// reports and the declaration renders without it, and one the
	// declaration misses reports a defect. Left empty, the guard
	// stays off, which is what a language predating the coverage
	// contract renders under.
	Coverage Coverage
}

// Pass is one composed language's render procedure. A Pass is safe
// for concurrent use: everything it holds is fixed at [New], and
// every render call owns its own frames. Within one call, files
// render in parallel across workers bounded by GOMAXPROCS, which
// is why a context's trees must tolerate concurrent reads, as
// every fs.FS does.
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

// reserved reports whether a name belongs to the pass's builtins,
// which no vocabulary may claim.
func reserved(name string) bool {
	switch name {
	case BuiltinBody, BuiltinUse, BuiltinImports, BuiltinDecls,
		BuiltinSlots, BuiltinSlot, BuiltinNested:
		return true
	}
	return false
}

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

// unbound returns the builtin names for parse-time resolution; a
// render call rebinds them to its own frame before any execute.
func unbound() template.FuncMap {
	refuse := func() (string, error) {
		return "", errors.New("render: the builtin is unbound")
	}
	return template.FuncMap{
		BuiltinBody:    func(any) (string, error) { return refuse() },
		BuiltinUse:     func(string) (string, error) { return refuse() },
		BuiltinImports: refuse,
		BuiltinDecls:   refuse,
		BuiltinNested:  func(string, symbol.Symbol) (string, error) { return refuse() },
	}
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

	byFile := map[fileKey]*group{}
	var order []*group
	for u := range ctx.Emit.Units() {
		units := []plugin.Unit{u}
		if p.split != nil {
			units = p.split(u)
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

	origin := ctx.Plugin
	if origin == "" {
		origin = p.name
	}
	seed := &frame{pass: p, sink: ctx.Sink, origin: origin, trees: ctx.Trees}
	merged := seed.mergeVocabulary(ctx)

	// Files are independent after grouping, so workers share the
	// render: each owns its frame, its buffers and its template
	// clones, bounded by the parallelism and never the file count,
	// and the trees are read concurrently, which an fs.FS supports.
	// The output order is the precomputed one, whatever order the
	// workers finish in.
	type rendered struct {
		body    []byte
		plugins []plugin.ID
		sources []string
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
				pass: p, sink: ctx.Sink, origin: origin,
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
				results[i] = r
			}
		})
	}
	wg.Wait()
	if err := bindErr.Load(); err != nil {
		return nil, *err
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
// call, which is what keeps a Pass safe for concurrent renders.
type frame struct {
	pass *Pass
	sink *diag.Sink
	// origin is the identity findings carry: the context's plugin,
	// or the pass's own name where the context carries none.
	origin  diag.Origin
	trees   map[plugin.ID]fs.FS
	merged  template.FuncMap
	out     bytes.Buffer
	fileOut bytes.Buffer
	set     ImportSet
	at      position.Pos
	plugin  plugin.ID
	// bound holds the frame's own template set once bind ran, which
	// is what the nested builtin renders through.
	bound *bound
}

// mergeVocabulary folds the plugins' helpers over the shared
// bucket: schedule order, latest wins, and plugins the schedule
// does not hold merge first, in name order, so a fixture without a
// schedule stays deterministic. A shared name shadowed without a
// declared override is refused and reported, and a reserved name
// is refused the same way, so the shared helper and the builtin
// stand.
func (f *frame) mergeVocabulary(ctx *plugin.RenderContext) template.FuncMap {
	merged := template.FuncMap{}
	scheduled := map[plugin.ID]bool{}
	order := make([]plugin.ID, 0, len(ctx.Funcs))
	for _, id := range ctx.Schedule {
		scheduled[id] = true
	}
	for _, id := range slices.Sorted(maps.Keys(ctx.Funcs)) {
		if !scheduled[id] {
			order = append(order, id)
		}
	}
	for _, id := range ctx.Schedule {
		if _, held := ctx.Funcs[id]; held {
			order = append(order, id)
		}
	}
	at := position.Pos{File: string(f.pass.name)}
	for _, id := range order {
		declared := map[string]bool{}
		for _, name := range ctx.Overrides[id] {
			declared[name] = true
		}
		fm := ctx.Funcs[id]
		for _, name := range slices.Sorted(maps.Keys(fm)) {
			_, shared := f.pass.shared[name]
			switch {
			case reserved(name):
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s claims %q, which is a builtin, and the builtin stands",
					id, name)
			case shared && !declared[name]:
				f.sink.Errorf(UndeclaredOverride, at, f.origin,
					"%s shadows the shared helper %q without declaring the override, and the shared helper stands",
					id, name)
			default:
				merged[name] = fm[name]
			}
		}
	}
	return merged
}

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
// the builtin a kind template qualifies with.
func (f *frame) use(path string) (string, error) {
	f.set.Add(path)
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

// body is the builtin a callable's kind template places its
// content with: the standard slots, the owner's named slots and
// the one content form, composed in the fixed order.
func (f *frame) body(d any) (string, error) {
	var b *emit.Body
	switch decl := d.(type) {
	case *emit.Function:
		b = &decl.Body
	case *emit.Method:
		b = &decl.Body
	default:
		return "", fmt.Errorf("a %T carries no body", d)
	}
	return f.renderBody(d, b)
}

// renderBody composes one body. For the template form the
// emitter's own template drives the layout, placing slots through
// its markers; every other form takes the fixed composition:
// prologue, content, named slots in declaration order, epilogue.
// A two-form body reports and keeps its slots, an unresolved
// reference reports and falls back to them, and a statement the
// language cannot spell fails the declaration under the
// execute-time code.
func (f *frame) renderBody(d any, b *emit.Body) (string, error) {
	form, err := b.Form()
	if err != nil {
		f.sink.Errorf(BodyConflict, f.at, f.origin,
			"%s emitted a body holding two forms: %v", f.plugin, err)
	}
	if form == emit.FormTemplate {
		if out, resolved := f.reference(d, b); resolved {
			return out, nil
		}
		form = emit.FormDefault
	}
	var out strings.Builder
	if err := f.stmts(&out, b.Prologue.Items()); err != nil {
		return "", err
	}
	switch form {
	case emit.FormStmts:
		if err := f.stmts(&out, b.Stmts); err != nil {
			return "", err
		}
	case emit.FormVerbatim:
		out.WriteString(b.Verbatim)
	case emit.FormTemplate, emit.FormDefault:
	}
	for _, named := range b.Slots {
		if err := f.stmts(&out, named.Slot.Items()); err != nil {
			return "", err
		}
	}
	if err := f.stmts(&out, b.Epilogue.Items()); err != nil {
		return "", err
	}
	return out.String(), nil
}

// reference executes a body-claiming template from the emitting
// plugin's tree. A false answer means nothing resolved, the
// finding is on the sink, and the caller falls back to the slots;
// a true answer is the template's own output, the marker rule
// checked behind it.
func (f *frame) reference(d any, b *emit.Body) (string, bool) {
	tree, held := f.trees[f.plugin]
	if !held {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s references %q, and no tree is declared for it",
			f.plugin, b.Ref.Name)
		return "", false
	}
	src, err := fs.ReadFile(tree, b.Ref.Name)
	if err != nil {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s references %q, which its tree does not hold",
			f.plugin, b.Ref.Name)
		return "", false
	}
	pl := &placement{frame: f, body: b, named: make([]bool, len(b.Slots))}
	t, err := template.New(b.Ref.Name).
		Funcs(f.pass.shared).Funcs(f.merged).
		Funcs(template.FuncMap{
			BuiltinSlots: pl.all,
			BuiltinSlot:  pl.one,
			BuiltinUse:   f.use,
		}).Parse(string(src))
	if err != nil {
		f.sink.Errorf(UnresolvedRef, f.at, f.origin,
			"%s's template %q does not parse: %v", f.plugin, b.Ref.Name, err)
		return "", false
	}
	var out strings.Builder
	data := struct{ Decl, Data any }{Decl: d, Data: b.Ref.Data}
	if err := t.Execute(&out, data); err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"%s's template %q refused: %v", f.plugin, b.Ref.Name, err)
		return "", false
	}
	if n := pl.pending(); n > 0 {
		f.sink.Errorf(DroppedSlots, f.at, f.origin,
			"%s's template %q places no marker for %d pending statements",
			f.plugin, b.Ref.Name, n)
	}
	return out.String(), true
}

// placement tracks which of a body's slots the template placed,
// so the marker rule has something to count.
type placement struct {
	frame *frame
	body  *emit.Body
	std   [2]bool
	named []bool
}

// all places everything not yet placed, in the fixed order:
// prologue, named slots in declaration order, epilogue. It is the
// slots builtin.
func (pl *placement) all() (string, error) {
	var out strings.Builder
	if !pl.std[0] {
		pl.std[0] = true
		if err := pl.frame.stmts(&out, pl.body.Prologue.Items()); err != nil {
			return "", err
		}
	}
	for i, named := range pl.body.Slots {
		if pl.named[i] {
			continue
		}
		pl.named[i] = true
		if err := pl.frame.stmts(&out, named.Slot.Items()); err != nil {
			return "", err
		}
	}
	if !pl.std[1] {
		pl.std[1] = true
		if err := pl.frame.stmts(&out, pl.body.Epilogue.Items()); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

// one places a single named slot where the template says. It is
// the slot builtin; a name the owner never declared is the
// template's own error.
func (pl *placement) one(name string) (string, error) {
	for i, named := range pl.body.Slots {
		if named.Name != name {
			continue
		}
		pl.named[i] = true
		var out strings.Builder
		if err := pl.frame.stmts(&out, named.Slot.Items()); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	return "", fmt.Errorf("the body declares no slot %q", name)
}

// pending counts the statements left in slots no marker placed.
func (pl *placement) pending() int {
	n := 0
	if !pl.std[0] {
		n += pl.body.Prologue.Len()
	}
	for i, named := range pl.body.Slots {
		if !pl.named[i] {
			n += named.Slot.Len()
		}
	}
	if !pl.std[1] {
		n += pl.body.Epilogue.Len()
	}
	return n
}

// declRun renders one unit's declarations: what the language's
// Cluster gathers renders through the group templates, each
// cluster at the position of its first member, and the rest
// renders through the kind templates, in the order the flush
// fixed.
func (f *frame) declRun(u plugin.Unit, b *bound) {
	memberOf := map[int]int{}
	var clusters []Clustered
	if f.pass.cluster != nil {
		clusters = f.pass.cluster(u.Decls)
		index := make(map[symbol.Symbol]int, len(u.Decls))
		for i, d := range u.Decls {
			index[d] = i
		}
		for ci, c := range clusters {
			for _, d := range c.Decls {
				i, held := index[d]
				if !held {
					continue
				}
				if _, claimed := memberOf[i]; claimed {
					continue
				}
				memberOf[i] = ci
			}
		}
	}
	rendered := make(map[int]bool, len(clusters))
	for i, d := range u.Decls {
		ci, member := memberOf[i]
		if !member {
			f.singleton(u, d, b)
			continue
		}
		if rendered[ci] {
			continue
		}
		rendered[ci] = true
		f.clustered(u, clusters[ci], b)
	}
}

// singleton renders one declaration through its kind template.
func (f *frame) singleton(u plugin.Unit, d symbol.Symbol, b *bound) {
	t, spelt := b.kinds[d.Kind()]
	if !spelt {
		f.sink.Errorf(UnspeltKind, f.at, f.origin,
			"%s holds no template for %s, and the declaration is skipped",
			f.pass.name, d.Kind())
		return
	}
	f.guard(d)
	if err := t.Execute(&f.out, d); err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"the %s template refused a declaration of %s: %v",
			d.Kind(), u.Plugin, err)
	}
}

// clustered renders one cluster through the group template its
// name selects.
func (f *frame) clustered(u plugin.Unit, c Clustered, b *bound) {
	t, held := b.groups[c.Group]
	if !held {
		f.sink.Errorf(UnknownGroup, f.at, f.origin,
			"%s clusters %d declarations under %q, which has no group template, and they are skipped",
			f.pass.name, len(c.Decls), c.Group)
		return
	}
	for _, d := range c.Decls {
		f.guard(d)
	}
	if err := t.Execute(&f.out, c); err != nil {
		f.sink.Errorf(RefusedTemplate, f.at, f.origin,
			"the %s group template refused a cluster of %s: %v",
			c.Group, u.Plugin, err)
	}
}

// guard reports the stated facts the declared coverage refuses or
// misses, before a declaration's template runs: a refused fact is
// a warning that keeps the narrowing loud, and an undeclared one
// is an error naming a defect in the backend's own declaration.
// The template still runs, so what the output holds stays the
// template's own answer: most declarations render without the
// refused fact, and a vocabulary helper refusing the combination
// outright reports beside the warning. An undeclared coverage
// leaves the guard off.
func (f *frame) guard(d symbol.Symbol) {
	if !f.pass.coverage.Declared() {
		return
	}
	emit.Facts(d, func(_ symbol.Symbol, kind symbol.Kind, fact symbol.Fact) {
		switch f.pass.coverage.Of(kind, fact) {
		case Refuses:
			f.sink.Warnf(RefusedFact, f.at, f.origin,
				"%s declares no spelling for %s stated on a %s",
				f.pass.name, fact, kind)
		case VerdictUndeclared:
			f.sink.Errorf(UndeclaredFact, f.at, f.origin,
				"%s takes no stance on %s stated on a %s: the coverage "+
					"declaration is incomplete", f.pass.name, fact, kind)
		case Renders, Holds:
		}
	})
}

// nested renders one nested declaration through its kind template,
// every line behind the given indentation, so a host places its
// inner declarations at member depth; it is the nested builtin. A
// kind without a template reports and spells nothing, the way an
// unspelt file-level declaration does; a template refusing the
// declaration propagates, so a host never renders around a
// half-spelt member. The block returns without its trailing line
// break, because the host's template supplies the separators its
// member layout uses.
func (f *frame) nested(indent string, s symbol.Symbol) (string, error) {
	t, spelt := f.bound.kinds[s.Kind()]
	if !spelt {
		f.sink.Errorf(UnspeltKind, f.at, f.origin,
			"%s holds no template for %s, and the nested declaration is skipped",
			f.pass.name, s.Kind())
		return "", nil
	}
	var out strings.Builder
	if err := t.Execute(&out, s); err != nil {
		return "", err
	}
	return indented(out.String(), indent), nil
}

// indented prefixes every non-empty line and drops the trailing
// line break, keeping blank lines bare, so an indented block
// carries no trailing spaces.
func indented(text, indent string) string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if line != "" {
			b.WriteString(indent)
			b.WriteString(line)
		}
	}
	return b.String()
}

// stmts spells a statement run through the language's printer,
// each spelling recording into the file's import set.
func (f *frame) stmts(out *strings.Builder, items []emit.Stmt) error {
	for _, s := range items {
		txt, err := f.pass.scaffold(s, &f.set)
		if err != nil {
			return err
		}
		out.Write(txt)
	}
	return nil
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
	f.at = position.Pos{File: g.name}
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
