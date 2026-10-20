// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Evaluated yields the names that a TypeScript module evaluates when it
// runs the declaration d, so the render writes d after the declarations of
// the file that declare them. A module evaluates the superclass of a class
// and the decorators of a class, of its fields and of its methods when it
// runs the class, and TypeScript hoists no class. Each name is the first
// name of a qualified spelling, as ns for ns.Base. A type names nothing
// that a module evaluates, because TypeScript erases it, so the
// interfaces of a class and every other declaration yield nothing.
// Evaluated allocates nothing.
func Evaluated(d symbol.Symbol, yield func(name string)) {
	s, class := d.(*emit.Struct)
	if !class || s == nil {
		return
	}
	for _, base := range s.Extends {
		if base != nil {
			name, _, _ := strings.Cut(base.Spelling, qualifierSep)
			yield(name)
		}
	}
	decorators := func(annotations symbol.Annotations) {
		for _, a := range annotations {
			name, _, _ := strings.Cut(a.Name, qualifierSep)
			yield(name)
		}
	}
	decorators(s.Annotations)
	for _, f := range s.Fields.Items() {
		if f != nil {
			decorators(f.Annotations)
		}
	}
	for _, m := range s.Methods.Items() {
		if m != nil {
			decorators(m.Annotations)
		}
	}
}
