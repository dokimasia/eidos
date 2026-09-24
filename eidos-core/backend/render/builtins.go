// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"errors"
	"text/template"

	"go.dokimi.dev/eidos/core/symbol"
)

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

// unbound returns the builtin names for parse-time resolution; a
// render call rebinds them to its own frame before any execute.
func unbound() template.FuncMap {
	refuse := func() (string, error) {
		return "", errors.New("render: the builtin is unbound")
	}
	return template.FuncMap{
		BuiltinBody:    func(any) (string, error) { return refuse() },
		BuiltinUse:     func(string, ...string) (string, error) { return refuse() },
		BuiltinImports: refuse,
		BuiltinDecls:   refuse,
		BuiltinNested:  func(string, symbol.Symbol) (string, error) { return refuse() },
	}
}

// referenceBuiltins is the builtin set a body-claiming template
// resolves against: the slots and slot markers and the use
// recorder. The render binds it to a frame and the lint binds
// stubs, so both parse against one set of names.
func referenceBuiltins(
	all func() (string, error),
	one func(string) (string, error),
	use func(string, ...string) (string, error),
) template.FuncMap {
	return template.FuncMap{BuiltinSlots: all, BuiltinSlot: one, BuiltinUse: use}
}
