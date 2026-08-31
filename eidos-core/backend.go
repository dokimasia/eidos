// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// BackendBuilder accumulates a backend declaration: the identity
// and target the plan validates, the comment syntax the output
// contract stamps through, and the language pieces the render
// pass varies in. Everything on it is data except the language
// functions; Build freezes it, and a BackendBuilder is not reused
// afterwards.
type BackendBuilder struct {
	name    plugin.ID
	target  plugin.Target
	syntax  plugin.CommentSyntax
	lang    render.Language
	defects []string
}

// NewBackend starts a backend declaration for one target.
func NewBackend(
	name plugin.ID, target plugin.Target, syntax plugin.CommentSyntax,
) *BackendBuilder {
	return &BackendBuilder{
		name: name, target: target, syntax: syntax,
		lang: render.Language{
			Kinds: map[symbol.Kind]string{},
			Funcs: template.FuncMap{},
		},
	}
}

// FileTemplate sets the file skeleton; undeclared, the pass's
// default renders the import block then the declarations. The
// clause a language opens its files with is the skeleton's own to
// spell; the header is not, because the output contract prepends
// it after the formatter ran.
func (b *BackendBuilder) FileTemplate(t string) *BackendBuilder {
	b.lang.File = t
	return b
}

// KindTemplates declares how the language spells emit kinds, keyed
// by kind; repeatable, merging, one spelling per kind. Which kinds
// render standalone and which render inside their hosts is the
// language's own split, and the conformance suite's every-kind
// rung is what holds a backend to the full inventory.
func (b *BackendBuilder) KindTemplates(ts map[symbol.Kind]string) *BackendBuilder {
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
func (b *BackendBuilder) Scaffold(
	f func(s emit.Stmt, set *render.ImportSet) ([]byte, error),
) *BackendBuilder {
	b.lang.Scaffold = f
	return b
}

// Funcs registers the language's shared template vocabulary into
// the overrideable bucket; repeatable, merging, one helper per
// name. A plugin's declared override replaces one of these names
// for every template in the pass.
func (b *BackendBuilder) Funcs(fs template.FuncMap) *BackendBuilder {
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
func (b *BackendBuilder) Naming(n render.Naming) *BackendBuilder {
	b.lang.Naming = n
	return b
}

// Imports sets the renderer for a file's collected import set:
// grouping and sorting are language facts.
func (b *BackendBuilder) Imports(r func(set *render.ImportSet) string) *BackendBuilder {
	b.lang.Imports = r
	return b
}

// Finalise sets the language formatter, run last per file. A
// failure at render is a positioned finding carrying the file it
// could not format, and the pass continues with the remaining
// files.
func (b *BackendBuilder) Finalise(f func(src []byte) ([]byte, error)) *BackendBuilder {
	b.lang.Finalise = f
	return b
}

// Build freezes the declaration and answers the lowered backend,
// which implements [plugin.Backend] and [plugin.Renderer] both.
// Its Render is the render pass over the declared language and
// nothing more, so a kit backend and a hand-rolled pass over the
// same language answer the same bytes.
//
// Build panics on a declaration defect: an empty name, a zero
// target, a kind or helper declared twice, and everything the
// pass refuses to compose — an empty kind-template set, a
// template that does not parse, a builtin name claimed by the
// shared vocabulary, a missing naming, scaffold, import renderer
// or formatter. A wrong declaration is a bug in the backend's own
// constructor and fires on the first Build in any test.
func (b *BackendBuilder) Build() plugin.Backend {
	if b.name == "" {
		panic("eidos: NewBackend with an empty name")
	}
	name := string(b.name)
	if b.target == "" {
		panic("eidos: " + name + " declares no target")
	}
	if len(b.defects) > 0 {
		panic("eidos: " + name + " " + strings.Join(b.defects, ", and "))
	}
	pass, err := render.New(b.name, b.lang)
	if err != nil {
		panic("eidos: " + name + " declares a language the pass refuses:\n" +
			err.Error())
	}
	return &builtBackend{
		name: b.name, target: b.target, syntax: b.syntax, pass: pass,
	}
}

// builtBackend is a lowered backend declaration: the seats the
// plan validates as data, and the composed pass Render lowers to.
type builtBackend struct {
	name   plugin.ID
	target plugin.Target
	syntax plugin.CommentSyntax
	pass   *render.Pass
}

// Name answers the backend's one identity.
func (b *builtBackend) Name() plugin.ID { return b.name }

// Target answers the target the backend's plan resolves at
// composition.
func (b *builtBackend) Target() plugin.Target { return b.target }

// Syntax answers the language's comment forms, carried for the
// output contract: the generated-file header is written through
// them, after the formatter ran.
func (b *builtBackend) Syntax() plugin.CommentSyntax { return b.syntax }

// Render implements [plugin.Renderer] through the composed pass.
func (b *builtBackend) Render(ctx *plugin.RenderContext) ([]plugin.RenderedFile, error) {
	return b.pass.Render(ctx)
}
