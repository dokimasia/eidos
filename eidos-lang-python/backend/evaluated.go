// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Evaluated yields the names that a Python module evaluates when it runs
// the declaration d, so the render writes d after the declarations of the
// file that declare them. A module evaluates the decorators of a function
// when it runs the def statement. It evaluates the bases and the
// decorators of a class when it runs the class statement, and the body of
// the class runs the decorators of its methods and the class statements of
// its nested classes. The bases of a class are the types that it extends,
// implements and embeds, the bases of the bases marker of an enum, and the
// underlying type of an enum without the marker. Each name is the first
// name of a dotted spelling, as models for models.Base. A decorator of a
// method whose first name is the name of the method, as name.setter, names
// the property of the class. A marker is no decorator, and a type, a
// default and a value name nothing that the render reads, so every other
// declaration yields nothing. Evaluated allocates nothing.
func Evaluated(d symbol.Symbol, yield func(name string)) {
	var methods []*emit.Method
	var types []symbol.Symbol
	switch t := d.(type) {
	case *emit.Function:
		if t != nil {
			decoratorNames(t.Annotations, "", yield)
		}
	case *emit.Struct:
		if t == nil {
			return
		}
		baseNames(t.Extends, yield)
		baseNames(t.Implements, yield)
		for _, e := range t.Embeds.Items() {
			if e != nil && e.Ref != nil {
				name, _, _ := strings.Cut(e.Ref.Spelling, dot)
				yield(name)
			}
		}
		decoratorNames(t.Annotations, "", yield)
		methods, types = t.Methods.Items(), t.Types.Items()
	case *emit.Interface:
		if t == nil {
			return
		}
		baseNames(t.Extends, yield)
		decoratorNames(t.Annotations, "", yield)
		methods, types = t.Methods.Items(), t.Types.Items()
	case *emit.Enum:
		if t == nil {
			return
		}
		if stated, _ := markerArgs(t.Annotations, python.BasesMarker); len(stated) > 0 {
			for _, b := range stated {
				name, _, _ := strings.Cut(b, dot)
				yield(name)
			}
		} else if t.Underlying != nil {
			name, _, _ := strings.Cut(t.Underlying.Spelling, dot)
			yield(name)
		}
		decoratorNames(t.Annotations, "", yield)
		methods = t.Methods.Items()
	}
	for _, m := range methods {
		if m != nil {
			decoratorNames(m.Annotations, m.Name, yield)
		}
	}
	for _, nested := range types {
		Evaluated(nested, yield)
	}
}

// baseNames yields the first name of the spelling of each base of refs.
func baseNames(refs []*emit.TypeRef, yield func(name string)) {
	for _, r := range refs {
		if r != nil {
			name, _, _ := strings.Cut(r.Spelling, dot)
			yield(name)
		}
	}
}

// decoratorNames yields the first name of each decorator of annotations,
// which are no markers, apart from a first name that is the name of the
// method member, which names a member of its class.
func decoratorNames(annotations symbol.Annotations, member string, yield func(name string)) {
	for _, a := range annotations {
		if strings.HasPrefix(a.Name, python.MarkerPrefix) {
			continue
		}
		if name, _, _ := strings.Cut(a.Name, dot); name != member {
			yield(name)
		}
	}
}
