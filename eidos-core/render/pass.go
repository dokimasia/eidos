// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package render

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
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

// Naming spells a unit's filename for one target: the join and
// extension of the family's word, the tag's treatment, and the
// cardinality key's stem. It is total: every unit the plan admits
// answers a name, and units answering one name assemble one file,
// which is how two plugins share it.
type Naming func(u plugin.Unit) string

// Language is what a target genuinely varies in; the pass owns
// everything else.
type Language struct {
	// Kinds holds the template source per emit kind: how the
	// language spells each declaration.
	Kinds map[symbol.Kind]string
	// Naming spells each unit's filename.
	Naming Naming
	// Scaffold spells one statement of the neutral vocabulary the
	// language's way. It is the printer the body builtin calls for
	// slot contributions and scaffold content alike; a statement
	// the language cannot spell answers an error, and the
	// declaration is skipped under the execute-time code.
	Scaffold func(s emit.Stmt) ([]byte, error)
	// Finalise is the language formatter, run last per file.
	Finalise func(src []byte) ([]byte, error)
}

// Pass is one composed language's render ritual. A Pass is safe
// for concurrent use: everything it holds is fixed at [New], and
// every render call owns its own buffers.
type Pass struct {
	name     plugin.ID
	kinds    map[symbol.Kind]*template.Template
	spell    Naming
	scaffold func(s emit.Stmt) ([]byte, error)
	final    func(src []byte) ([]byte, error)
}

// New composes a language into its pass.
//
// It refuses, collecting every fault: an empty kind-template set,
// a template that does not parse, a missing naming and a missing
// formatter. The kit converts these to panics at its own Build,
// because there they are declaration defects; here they are
// composition faults, and a composition reads the whole bill.
func New(name plugin.ID, l Language) (*Pass, error) {
	var faults []error
	if len(l.Kinds) == 0 {
		faults = append(faults,
			errors.New("render: the language spells no kinds"))
	}
	kinds := make(map[symbol.Kind]*template.Template, len(l.Kinds))
	for _, k := range slices.Sorted(maps.Keys(l.Kinds)) {
		t, err := template.New(k.String()).Funcs(unbound()).Parse(l.Kinds[k])
		if err != nil {
			faults = append(faults, fmt.Errorf("render: the %s template: %w", k, err))
			continue
		}
		kinds[k] = t
	}
	if l.Naming == nil {
		faults = append(faults,
			errors.New("render: the language spells no filenames"))
	}
	if l.Scaffold == nil {
		faults = append(faults,
			errors.New("render: the language spells no scaffolding"))
	}
	if l.Finalise == nil {
		faults = append(faults,
			errors.New("render: the language holds no formatter"))
	}
	if len(faults) > 0 {
		return nil, errors.Join(faults...)
	}
	return &Pass{
		name: name, kinds: kinds,
		spell: l.Naming, scaffold: l.Scaffold, final: l.Finalise,
	}, nil
}

// unbound answers the builtin names for parse-time resolution; a
// render call rebinds them to its own frame before any execute.
func unbound() template.FuncMap {
	return template.FuncMap{
		"body": func(any) (string, error) {
			return "", errors.New("render: the body builtin is unbound")
		},
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

// Render takes one plan's emit through the ritual and answers the
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
		key := fileKey{pkg: u.Pkg, name: p.spell(u)}
		g, held := byFile[key]
		if !held {
			g = &group{name: key.name, pkg: key.pkg}
			byFile[key] = g
			order = append(order, g)
		}
		g.units = append(g.units, u)
	}
	slices.SortFunc(order, func(a, b *group) int {
		if c := a.pkg.Compare(b.pkg); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})

	f := &frame{pass: p, sink: ctx.Sink}
	kinds, err := p.bind(f)
	if err != nil {
		return nil, err
	}
	files := make([]plugin.RenderedFile, 0, len(order))
	for _, g := range order {
		if body, rendered := f.file(g, kinds); rendered {
			files = append(files, plugin.RenderedFile{Name: g.name, Pkg: g.pkg, Body: body})
		}
	}
	return files, nil
}

// frame is one render call's mutable state: the buffer the files
// share, the sink, and the position and emitter of whatever is
// under render, which the builtins read. One frame serves one
// call, which is what keeps a Pass safe for concurrent renders.
type frame struct {
	pass   *Pass
	sink   *diag.Sink
	out    bytes.Buffer
	at     position.Pos
	plugin plugin.ID
}

// bind clones the parsed kind templates for this call and binds
// the builtins to its frame: parse happened once at New, and no
// two calls share an executing tree.
func (p *Pass) bind(f *frame) (map[symbol.Kind]*template.Template, error) {
	kinds := make(map[symbol.Kind]*template.Template, len(p.kinds))
	funcs := template.FuncMap{"body": f.body}
	for k, t := range p.kinds {
		c, err := t.Clone()
		if err != nil {
			return nil, fmt.Errorf("render: cloning the %s template: %w", k, err)
		}
		kinds[k] = c.Funcs(funcs)
	}
	return kinds, nil
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
	return f.renderBody(b)
}

// renderBody composes one body: prologue, content, named slots in
// declaration order, epilogue. A two-form body reports and keeps
// its slots; a reference resolves against the trees the context
// carries, and a statement the language cannot spell fails the
// declaration under the execute-time code.
func (f *frame) renderBody(b *emit.Body) (string, error) {
	var out strings.Builder
	if err := f.stmts(&out, b.Prologue.Items()); err != nil {
		return "", err
	}
	form, err := b.Form()
	if err != nil {
		f.sink.Errorf(BodyConflict, f.at, f.pass.name,
			"%s emitted a body holding two forms: %v", f.plugin, err)
	}
	switch form {
	case emit.FormStmts:
		if err := f.stmts(&out, b.Stmts); err != nil {
			return "", err
		}
	case emit.FormVerbatim:
		out.WriteString(b.Verbatim)
	case emit.FormTemplate:
		return "", errors.New("the pass holds no template trees")
	case emit.FormDefault:
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

// stmts spells a statement run through the language's printer.
func (f *frame) stmts(out *strings.Builder, items []emit.Stmt) error {
	for _, s := range items {
		txt, err := f.pass.scaffold(s)
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
// bytes are copied out, so a formatter that answers its input, as
// a pass-through one does, never aliases storage a following file
// overwrites.
func (f *frame) file(g *group, kinds map[symbol.Kind]*template.Template) ([]byte, bool) {
	f.out.Reset()
	f.at = position.Pos{File: g.name}
	for _, u := range g.units {
		f.plugin = u.Plugin
		for _, d := range u.Decls {
			t, spelt := kinds[d.Kind()]
			if !spelt {
				f.sink.Errorf(UnspeltKind, f.at, f.pass.name,
					"%s holds no template for %s, and the declaration is skipped",
					f.pass.name, d.Kind())
				continue
			}
			if err := t.Execute(&f.out, d); err != nil {
				f.sink.Errorf(RefusedTemplate, f.at, f.pass.name,
					"the %s template refused a declaration of %s: %v",
					d.Kind(), u.Plugin, err)
			}
		}
	}
	body, err := f.pass.final(f.out.Bytes())
	if err != nil {
		f.sink.Errorf(UnformattedFile, f.at, f.pass.name,
			"%s cannot be formatted and is withheld: %v", g.name, err)
		return nil, false
	}
	return bytes.Clone(body), true
}
