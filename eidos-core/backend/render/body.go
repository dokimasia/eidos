// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
)

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
		return "", fmt.Errorf("render: a %T carries no body", d)
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
// checked behind it. Each frame reads and parses a template once
// per emitting plugin and name. Its builtins are bound to the frame
// and write into the body under execution, so an execution copies
// no template. A refusal withdraws the imports the execution
// recorded, because its output is dropped.
func (f *frame) reference(d any, b *emit.Body) (string, bool) {
	key := refKey{plugin: f.plugin, name: b.Ref.Name}
	parsed, cached := f.refCache[key]
	if !cached {
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
		parsed, err = template.New(b.Ref.Name).
			Funcs(f.pass.shared).Funcs(f.merged).
			Funcs(referenceBuiltins(f.placeAll, f.placeOne, f.use)).
			Parse(string(src))
		if err != nil {
			f.sink.Errorf(UnresolvedRef, f.at, f.origin,
				"%s's template %q does not parse: %v", f.plugin, b.Ref.Name, err)
			return "", false
		}
		if f.refCache == nil {
			f.refCache = map[refKey]*template.Template{}
		}
		f.refCache[key] = parsed
	}
	pl := &placement{frame: f, body: b, named: make([]bool, len(b.Slots))}
	f.place = pl
	mark := f.set.mark()
	var out strings.Builder
	data := struct{ Decl, Data any }{Decl: d, Data: b.Ref.Data}
	err := parsed.Execute(&out, data)
	f.place = nil
	if err != nil {
		f.set.rollback(mark)
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

// placeAll is the slots builtin a parsed reference template calls:
// it places into the body the frame is executing for.
func (f *frame) placeAll() (string, error) {
	if f.place == nil {
		return "", errors.New("render: the slots marker ran outside a body")
	}
	return f.place.all()
}

// placeOne is the slot builtin a parsed reference template calls.
func (f *frame) placeOne(name string) (string, error) {
	if f.place == nil {
		return "", errors.New("render: the slot marker ran outside a body")
	}
	return f.place.one(name)
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
	return "", fmt.Errorf("render: the body declares no slot %q", name)
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
