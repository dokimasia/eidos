// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit

import (
	"fmt"
	"strconv"
	"strings"
)

// Form names a body's content form.
type Form uint8

const (
	// FormDefault is nothing: the kind template renders the
	// standard slots and no content of its own.
	FormDefault Form = iota
	// FormStmts is scaffolding from the neutral vocabulary.
	FormStmts
	// FormTemplate claims the body for the emitting plugin's
	// template.
	FormTemplate
	// FormVerbatim is literal text.
	FormVerbatim
)

// String answers the form's spelling. The lint rung and render
// findings name forms, so a consumer matching on the spelling
// matches on API. A form nothing declares answers its number
// rather than a name.
func (f Form) String() string {
	switch f {
	case FormDefault:
		return "default"
	case FormStmts:
		return "scaffolding"
	case FormTemplate:
		return "template"
	case FormVerbatim:
		return "verbatim"
	default:
		return strconv.Itoa(int(f))
	}
}

// Body is a callable's emit-side content: the standard slots every
// body carries, the named slots its owner declared, and the
// content between them. The zero Body is the kind template's
// default, nothing but the standard slots, rendered automatically.
//
// A Body is not safe for concurrent use, for the same reason a
// [Slot] is not: plans run in parallel, and each owns its own emit
// declarations.
type Body struct {
	// Prologue and Epilogue exist on every body by construction:
	// the extension points a cross-cutting plugin appends to
	// whether the owner anticipated it or not.
	Prologue Slot[Stmt] `json:"prologue,omitzero"`
	Epilogue Slot[Stmt] `json:"epilogue,omitzero"`
	// Slots holds the owner's declared extension points, in
	// declaration order, rendered between the standard pair. The
	// elements are pointers because [Body.Declare] answers handles
	// into the list, and a following declaration must not move
	// what an earlier caller already holds.
	Slots []*NamedSlot `json:"slots,omitzero"`

	// The content, exactly one form set; all three zero is the
	// default form, and [Body.Form] is the one question the render
	// and the lint rung ask. Stmts is scaffolding, Ref claims the
	// body for a template in the emitting plugin's tree, and
	// Verbatim is literal text: the sharp knife the lint rung
	// cannot see into, which contributes no imports and composes
	// with nothing.
	Stmts    []Stmt       `json:"stmts,omitzero"`
	Ref      *TemplateRef `json:"ref,omitzero"`
	Verbatim string       `json:"verbatim,omitzero"`
}

// NamedSlot is one owner-declared extension point.
type NamedSlot struct {
	Name string     `json:"name"`
	Slot Slot[Stmt] `json:"stmts,omitzero"`
}

// Declare answers the named slot, adding it in declaration order
// on first use; declaring a name twice answers the existing slot.
// Declaring is the owner's act: a contributor looks a slot up
// through [Body.Slot] instead.
func (b *Body) Declare(name string) *Slot[Stmt] {
	for _, s := range b.Slots {
		if s.Name == name {
			return &s.Slot
		}
	}
	s := &NamedSlot{Name: name}
	b.Slots = append(b.Slots, s)
	return &s.Slot
}

// Slot answers a declared slot and false for a name the owner
// never declared, so a contribution into an invented extension
// point fails where it is made.
func (b *Body) Slot(name string) (*Slot[Stmt], bool) {
	for _, s := range b.Slots {
		if s.Name == name {
			return &s.Slot, true
		}
	}
	return nil, false
}

// Form answers which content form the body holds, and an error
// naming the forms where more than one is set: a body built with
// two contents is a defect, and the render and the lint rung both
// ask this one question.
func (b *Body) Form() (Form, error) {
	form := FormDefault
	var set []string
	if len(b.Stmts) > 0 {
		form = FormStmts
		set = append(set, FormStmts.String())
	}
	if b.Ref != nil {
		form = FormTemplate
		set = append(set, FormTemplate.String())
	}
	if b.Verbatim != "" {
		form = FormVerbatim
		set = append(set, FormVerbatim.String())
	}
	if len(set) > 1 {
		return FormDefault, fmt.Errorf(
			"emit: the body holds %s at once, and content is one form",
			strings.Join(set, " and "),
		)
	}
	return form, nil
}

// IsZero reports whether the body holds nothing at all, which is
// what lets an encoder omit an untouched one.
func (b Body) IsZero() bool {
	return b.Prologue.IsZero() && b.Epilogue.IsZero() && len(b.Slots) == 0 &&
		len(b.Stmts) == 0 && b.Ref == nil && b.Verbatim == ""
}
