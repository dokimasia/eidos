// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Builder accumulates a backend declaration: the identity and
// target the plan validates, the comment syntax the output
// contract stamps through, and the language pieces the render
// pass varies in. Everything on it is data except the language
// functions; Build freezes it, and a Builder is not reused
// afterwards.
type Builder struct {
	name    plugin.ID
	target  plugin.Target
	syntax  plugin.CommentSyntax
	version string
	lang    render.Language
	lower   plugin.Lower
	respell plugin.Respell
	defects []string
}

// New starts a backend declaration for one target.
func New(
	name plugin.ID, target plugin.Target, syntax plugin.CommentSyntax,
) *Builder {
	return &Builder{
		name: name, target: target, syntax: syntax,
		lang: render.Language{
			Kinds:  map[symbol.Kind]string{},
			Funcs:  template.FuncMap{},
			Groups: map[render.GroupName]string{},
		},
	}
}

// Version sets the version the run fingerprint folds in: bump it
// with every change to the rendered output, because a warm cache
// keyed without it serves the old bytes after a backend change
// and nothing reports why.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// FileTemplate sets the file skeleton; undeclared, the pass's
// default renders the import block then the declarations. The
// clause a language opens its files with is the skeleton's own to
// spell; the header is not, because the output contract prepends
// it after the formatter ran.
func (b *Builder) FileTemplate(t string) *Builder {
	b.lang.File = t
	return b
}

// KindTemplates declares how the language spells emit kinds, keyed
// by kind; repeatable, merging, one spelling per kind. Which kinds
// render standalone and which render inside their hosts is the
// language's own split, and the conformance suite's every-kind
// check is what holds a backend to the full inventory.
func (b *Builder) KindTemplates(ts map[symbol.Kind]string) *Builder {
	for _, k := range slices.Sorted(maps.Keys(ts)) {
		if _, taken := b.lang.Kinds[k]; taken {
			b.defects = append(b.defects, "spells the "+k.String()+" kind twice")
			continue
		}
		b.lang.Kinds[k] = ts[k]
	}
	return b
}

// Scaffold sets the language's statement printer: how each kind of
// the neutral scaffolding vocabulary spells, recording into the
// file's import set whatever it qualifies with. The body builtin
// calls it for slot contributions and scaffold content alike.
func (b *Builder) Scaffold(
	f func(s emit.Stmt, set *render.ImportSet) ([]byte, error),
) *Builder {
	b.lang.Scaffold = f
	return b
}

// Funcs registers the language's shared template vocabulary into
// the overrideable bucket; repeatable, merging, one helper per
// name. A plugin's declared override replaces one of these names
// for every template in the pass.
func (b *Builder) Funcs(fs template.FuncMap) *Builder {
	for _, name := range slices.Sorted(maps.Keys(fs)) {
		if _, taken := b.lang.Funcs[name]; taken {
			b.defects = append(b.defects,
				"declares the "+strconv.Quote(name)+" helper twice")
			continue
		}
		b.lang.Funcs[name] = fs[name]
	}
	return b
}

// Naming sets the target's filename spelling.
func (b *Builder) Naming(n render.Naming) *Builder {
	b.lang.Naming = n
	return b
}

// Split sets the target's unit reshaping; undeclared, every unit
// files whole.
func (b *Builder) Split(s render.Split) *Builder {
	b.lang.Split = s
	return b
}

// Cluster sets the target's declaration clustering. The group
// templates its clusters select arrive through [Builder.Groups],
// and a cluster without them is a defect at Build.
func (b *Builder) Cluster(c render.Cluster) *Builder {
	b.lang.Cluster = c
	return b
}

// Groups declares the group templates a Cluster selects, keyed by
// group name; repeatable, merging, one spelling per group.
func (b *Builder) Groups(gs map[render.GroupName]string) *Builder {
	for _, g := range slices.Sorted(maps.Keys(gs)) {
		if _, taken := b.lang.Groups[g]; taken {
			b.defects = append(b.defects,
				"spells the "+string(g)+" group twice")
			continue
		}
		b.lang.Groups[g] = gs[g]
	}
	return b
}

// Imports sets the renderer for a file's collected import set:
// grouping and sorting are language facts.
func (b *Builder) Imports(r func(set *render.ImportSet) string) *Builder {
	b.lang.Imports = r
	return b
}

// Finalise sets the language formatter, run last per file. A
// failure at render is a positioned finding carrying the file it
// could not format, and the pass continues with the remaining
// files.
func (b *Builder) Finalise(f func(src []byte) ([]byte, error)) *Builder {
	b.lang.Finalise = f
	return b
}

// Coverage declares the language's fact coverage: one verdict per
// fact, with per-kind exceptions. It arms the render's guard, and
// the built backend implements [render.Coverer], which is how the
// conformance suite holds the declaration total and the rendered
// findings against it.
func (b *Builder) Coverage(c render.Coverage) *Builder {
	b.lang.Coverage = c
	return b
}

// Lower sets the construct lowering the settle applies: one
// declaration in, the target's declaration shapes out, before any
// name respells. The built backend implements [plugin.Lowerer],
// and its Render refuses an unsettled store.
func (b *Builder) Lower(l plugin.Lower) *Builder {
	b.lower = l
	return b
}

// Respell sets the name convention the settle applies: every
// declared name spells through it, and references follow. The
// built backend implements [plugin.Respeller], and its Render
// refuses an unsettled store.
func (b *Builder) Respell(r plugin.Respell) *Builder {
	b.respell = r
	return b
}

// Build freezes the declaration and returns the lowered backend,
// which implements [plugin.Backend] and [plugin.Renderer] both.
// Its Render is the render pass over the declared language and
// nothing more, so a kit backend and a hand-rolled pass over the
// same language return the same bytes.
//
// Build panics on a declaration defect: an empty name, a zero
// target, a kind or helper declared twice, and everything the
// pass refuses to compose — an empty kind-template set, a
// template that does not parse, a builtin name claimed by the
// shared vocabulary, a missing naming, scaffold, import renderer
// or formatter. A wrong declaration is a bug in the backend's own
// constructor and panics on the first Build in any test.
func (b *Builder) Build() plugin.Backend {
	if b.name == "" {
		panic("backend: New with an empty name")
	}
	name := string(b.name)
	if b.target == "" {
		panic("backend: " + name + " declares no target")
	}
	if len(b.defects) > 0 {
		panic("backend: " + name + " " + strings.Join(b.defects, ", and "))
	}
	pass, err := render.New(b.name, b.lang)
	if err != nil {
		panic("backend: " + name + " declares a language the pass refuses:\n" +
			err.Error())
	}
	base := &builtBackend{
		name: b.name, target: b.target, syntax: b.syntax,
		version: b.version, pass: pass,
	}
	switch {
	case b.lower != nil && b.respell != nil:
		return &settlingBackend{builtBackend: base, lower: b.lower, respell: b.respell}
	case b.lower != nil:
		return &loweringBackend{builtBackend: base, lower: b.lower}
	case b.respell != nil:
		return &respellingBackend{builtBackend: base, respell: b.respell}
	default:
		return base
	}
}

// builtBackend is a lowered backend declaration: the roles the
// plan validates as data, and the composed pass Render lowers to.
type builtBackend struct {
	name    plugin.ID
	target  plugin.Target
	syntax  plugin.CommentSyntax
	version string
	pass    *render.Pass
}

// Name returns the backend's one identity.
func (b *builtBackend) Name() plugin.ID { return b.name }

// Version implements [plugin.Versioned]: the declared version, ""
// where none was.
func (b *builtBackend) Version() string { return b.version }

// Target returns the target the backend's plan resolves at
// composition.
func (b *builtBackend) Target() plugin.Target { return b.target }

// Syntax returns the language's comment forms, carried for the
// output contract: the generated-file header is written through
// them, after the formatter ran.
func (b *builtBackend) Syntax() plugin.CommentSyntax { return b.syntax }

// Coverage implements [render.Coverer] through the composed pass,
// so the suite reads the same data the render's guard does.
func (b *builtBackend) Coverage() render.Coverage { return b.pass.Coverage() }

// Render implements [plugin.Renderer] through the composed pass.
func (b *builtBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return b.pass.Render(ctx)
}

// refuseUnsettled is the guard a hook-declaring backend renders
// behind: an unsettled store means the plan never settled, and
// rendering it would write the wrong bytes without a finding.
func (b *builtBackend) refuseUnsettled(ctx *plugin.RenderContext) error {
	if ctx != nil && ctx.Emit != nil && !ctx.Emit.Settled() {
		return errors.New("backend: " + string(b.name) +
			" declares a lowering seam, and the store is unsettled: " +
			"the plan settles once before the render")
	}
	return nil
}

// loweringBackend is a built backend declaring the construct
// lowering seam.
type loweringBackend struct {
	*builtBackend
	lower plugin.Lower
}

// Lower implements [plugin.Lowerer] through the declared hook.
func (b *loweringBackend) Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	return b.lower(s)
}

// Render refuses an unsettled store, then renders through the
// composed pass.
func (b *loweringBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	if err := b.refuseUnsettled(ctx); err != nil {
		return nil, err
	}
	return b.builtBackend.Render(ctx)
}

// respellingBackend is a built backend declaring the name respell
// seam.
type respellingBackend struct {
	*builtBackend
	respell plugin.Respell
}

// Respell implements [plugin.Respeller] through the declared hook.
func (b *respellingBackend) Respell(
	host, kind symbol.Kind, v symbol.Visibility, name string,
) (string, error) {
	return b.respell(host, kind, v, name)
}

// Render refuses an unsettled store, then renders through the
// composed pass.
func (b *respellingBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	if err := b.refuseUnsettled(ctx); err != nil {
		return nil, err
	}
	return b.builtBackend.Render(ctx)
}

// settlingBackend is a built backend declaring both lowering
// seams.
type settlingBackend struct {
	*builtBackend
	lower   plugin.Lower
	respell plugin.Respell
}

// Lower implements [plugin.Lowerer] through the declared hook.
func (b *settlingBackend) Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	return b.lower(s)
}

// Respell implements [plugin.Respeller] through the declared hook.
func (b *settlingBackend) Respell(
	host, kind symbol.Kind, v symbol.Visibility, name string,
) (string, error) {
	return b.respell(host, kind, v, name)
}

// Render refuses an unsettled store, then renders through the
// composed pass.
func (b *settlingBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	if err := b.refuseUnsettled(ctx); err != nil {
		return nil, err
	}
	return b.builtBackend.Render(ctx)
}
