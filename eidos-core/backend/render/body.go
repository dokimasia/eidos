// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
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
		return "", fmt.Errorf("render: a %T has no body", d)
	}
	return f.renderBody(d, b, "")
}

// memberBody is the builtin a host's template places a member's content
// with. It returns the content that body returns, with every line that
// is not blank behind the language's member indent, so the member's
// statements are one level deeper than the scaffold writes them. The
// content keeps its line breaks. It takes a method, so text/template
// passes its argument without an interface conversion.
func (f *frame) memberBody(m *emit.Method) (string, error) {
	return f.renderBody(m, &m.Body, f.pass.memberIndent)
}

// renderBody composes one body, with every line that is not blank
// behind indent. For the template form the emitter's own template
// drives the layout, placing slots through its markers; every other
// form takes the fixed composition: prologue, content, named slots in
// declaration order, epilogue. A two-form body reports and keeps its
// slots, an unresolved reference reports and falls back to them, and a
// statement the language cannot spell fails the declaration under the
// execute-time code. The fixed composition writes the indent as it
// writes the lines, so an indent does not allocate on its own.
func (f *frame) renderBody(d any, b *emit.Body, indent string) (string, error) {
	form, err := b.Form()
	if err != nil {
		f.sink.Errorf(BodyConflict, f.at, f.origin,
			"%s emitted a body with two forms: %v", f.plugin, err)
	}
	if form == emit.FormTemplate {
		if out, resolved := f.reference(d, b); resolved {
			return indented(out, indent), nil
		}
		form = emit.FormDefault
	}
	out := lines{indent: indent}
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

// reference executes a body-claiming template from its owner's
// tree: the plugin the reference names, else the plugin whose unit
// contains the body. A false result means nothing resolved, the
// finding is on the sink, and the caller falls back to the slots.
// A true result comes with the template's own output, the marker
// rule checked behind it. Each frame reads and parses a template
// once per owner and name. Its builtins are bound to the frame and
// write into the body under execution, so an execution copies no
// template. A refusal withdraws the imports the execution recorded,
// because its output is dropped.
func (f *frame) reference(d any, b *emit.Body) (string, bool) {
	owner := b.Ref.Owner
	if owner == "" {
		owner = f.plugin
	}
	key := refKey{plugin: owner, name: b.Ref.Name}
	parsed, cached := f.refCache[key]
	if !cached {
		tree, held := f.trees[owner]
		if !held {
			f.sink.Errorf(UnresolvedRef, f.at, f.origin,
				"%s references %q, and no tree is declared for it",
				owner, b.Ref.Name)
			return "", false
		}
		src, err := fs.ReadFile(tree, b.Ref.Name)
		if err != nil {
			f.sink.Errorf(UnresolvedRef, f.at, f.origin,
				"%s references %q, which its tree does not contain",
				owner, b.Ref.Name)
			return "", false
		}
		parsed, err = template.New(b.Ref.Name).
			Funcs(f.vocab).Funcs(f.merged).
			Funcs(referenceBuiltins(f.placeAll, f.placeOne, f.use)).
			Parse(string(src))
		if err != nil {
			f.sink.Errorf(UnresolvedRef, f.at, f.origin,
				"%s's template %q does not parse: %v", owner, b.Ref.Name, err)
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
			"%s's template %q refused: %v", owner, b.Ref.Name, err)
		return "", false
	}
	if n := pl.pending(); n > 0 {
		f.sink.Errorf(DroppedSlots, f.at, f.origin,
			"%s's template %q places no marker for %d pending statements",
			owner, b.Ref.Name, n)
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
	var out lines
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

// one places a single slot where the template calls it: the
// prologue or the epilogue under its standard name, else the owner
// slot of that name. It is the slot builtin, and a name the owner
// never declared is the template's own error.
func (pl *placement) one(name string) (string, error) {
	var std *emit.Slot[emit.Stmt]
	switch name {
	case emit.SlotPrologue:
		pl.std[0], std = true, &pl.body.Prologue
	case emit.SlotEpilogue:
		pl.std[1], std = true, &pl.body.Epilogue
	}
	if std != nil {
		var out lines
		if err := pl.frame.stmts(&out, std.Items()); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	for i, named := range pl.body.Slots {
		if named.Name != name {
			continue
		}
		pl.named[i] = true
		var out lines
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
func (f *frame) stmts(out *lines, items []emit.Stmt) error {
	for _, s := range items {
		txt, err := f.pass.scaffold(s, &f.set)
		if err != nil {
			return err
		}
		out.Write(txt)
	}
	return nil
}

// lines is a text under construction whose lines that are not blank
// start with indent. The zero value writes text as it is.
//
// # Allocation contract
//
// A write allocates what the growth of the text allocates, and the
// indent adds none of its own.
type lines struct {
	b      strings.Builder
	indent string
	// mid reports that the text ends inside a line, so the next write
	// continues that line.
	mid bool
}

// Write appends p, and writes the indent before each line of p that is
// not blank and starts a line of the text.
func (l *lines) Write(p []byte) {
	if l.indent == "" {
		l.b.Write(p)
		return
	}
	for line := range bytes.Lines(p) {
		if !l.mid && (len(line) > 1 || line[0] != lineBreak) {
			l.b.Grow(len(l.indent) + len(line))
			l.b.WriteString(l.indent)
		}
		l.b.Write(line)
		l.mid = line[len(line)-1] != lineBreak
	}
}

// WriteString appends s as Write appends its bytes.
func (l *lines) WriteString(s string) {
	if l.indent == "" {
		l.b.WriteString(s)
		return
	}
	for line := range strings.Lines(s) {
		if !l.mid && line != string(lineBreak) {
			l.b.Grow(len(l.indent) + len(line))
			l.b.WriteString(l.indent)
		}
		l.b.WriteString(line)
		l.mid = !strings.HasSuffix(line, string(lineBreak))
	}
}

// String returns the text.
func (l *lines) String() string {
	return l.b.String()
}
