// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The decorators that the writer writes for a fact of the model: the level
// of a method, its accessor, and its abstract, final and overriding marks.
// The property decorators of a setter and a deleter follow the name of the
// property.
const (
	staticDecorator   = "staticmethod"
	classDecorator    = "classmethod"
	propertyDecorator = "property"
	abstractDecorator = "abstractmethod"
	finalDecorator    = "final"
	overrideDecorator = overrideName
	setterSuffix      = ".setter"
	deleterSuffix     = ".deleter"
)

// deprecatedDecorator is the decorator of a deprecated declaration, which
// warnings declares from Python 3.13 and typing_extensions before.
const deprecatedDecorator = "deprecated"

// factCount is the number of decorators that the facts of a method state at
// most: override, final, the level, the accessor and abstractmethod.
const factCount = 5

// decoratorLine is the room that the decorator lines of a declaration
// reserve for each line. A longer line grows the text once more.
const decoratorLine = 32

// The modules of the standard library that declare decorators of
// [decoratorModules] and no other name that the writer imports.
const (
	functoolsModule  = "functools"
	contextlibModule = "contextlib"
	warningsModule   = "warnings"
)

// decoratorModules maps the decorators of the standard library that a
// generator states by their bare names, and that the writer states for
// facts, to the module that declares each. The writer imports a decorator
// of the table under the name that the import binds. A name of builtins
// needs no import, and the table does not list one.
var decoratorModules = map[string]string{
	abstractDecorator:      abcBaseModule,
	finalDecorator:         typingModule,
	overrideDecorator:      typingModule,
	"overload":             typingModule,
	"runtime_checkable":    typingModule,
	"no_type_check":        typingModule,
	"dataclass_transform":  typingModule,
	"type_check_only":      typingModule,
	"dataclass":            dataModule,
	"cache":                functoolsModule,
	"cached_property":      functoolsModule,
	"lru_cache":            functoolsModule,
	"total_ordering":       functoolsModule,
	"singledispatch":       functoolsModule,
	"singledispatchmethod": functoolsModule,
	"wraps":                functoolsModule,
	"contextmanager":       contextlibModule,
	"asynccontextmanager":  contextlibModule,
	"unique":               enumModule,
	"verify":               enumModule,
	"member":               enumModule,
	"nonmember":            enumModule,
	"global_enum":          enumModule,
	deprecatedDecorator:    warningsModule,
}

// Decorators writes the decorator lines of a class or a callable d: first
// the decorators of its facts that its annotations do not state, then its
// annotations in order. The facts of a method are override, final, the
// level as staticmethod or classmethod, the accessor as property or as the
// setter or the deleter of the property, and abstractmethod, in that order,
// which is the order in which Python applies them from the inside out. An
// abstract method of a Protocol has no abstractmethod, because a method of
// a Protocol without a body is abstract, and __new__ has no level
// decorator, because Python makes it a static method of its own. The fact of
// a class is final. A marker is no decorator.
//
// A decorator of the table of the standard library imports from its module,
// a dotted decorator imports the module of its first name, and a decorator
// whose first name a declaration of the file or an import of the header
// binds imports nothing. The decorators staticmethod, classmethod and
// property of the facts are builtins, which a declaration of the file or a
// member of the class hides as it hides the builtins of [Writer.Spell]. Each
// line has the arguments of its annotation verbatim in parentheses.
func (w *Writer) Decorators(d symbol.Symbol) string {
	n := decoratorCount(d, w.class)
	if n == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n * decoratorLine)
	w.writeDecorators(&b, d, "")
	return b.String()
}

// writeDecorators writes the decorator lines of d into b at the indentation
// indent, as [Writer.Decorators] writes them. The class under writing is the
// host of a method.
func (w *Writer) writeDecorators(b *strings.Builder, d symbol.Symbol, indent string) {
	annotations := annotationsOf(d)
	var buf [factCount]string
	for _, name := range factDecorators(d, w.class, annotations, buf[:0]) {
		if name == staticDecorator || name == classDecorator || name == propertyDecorator {
			name = w.builtin(name, name)
		}
		w.decorator(b, d, symbol.Annotation{Name: name}, indent)
	}
	for _, a := range annotations {
		if !strings.HasPrefix(a.Name, python.MarkerPrefix) {
			w.decorator(b, d, a, indent)
		}
	}
}

// decoratorCount returns the number of the decorator lines of d, a member
// of the class host where host is not nil.
func decoratorCount(d, host symbol.Symbol) int {
	annotations := annotationsOf(d)
	var buf [factCount]string
	n := len(factDecorators(d, host, annotations, buf[:0]))
	for _, a := range annotations {
		if !strings.HasPrefix(a.Name, python.MarkerPrefix) {
			n++
		}
	}
	return n
}

// decorator writes one decorator line of d into b at the indentation
// indent.
func (w *Writer) decorator(b *strings.Builder, d symbol.Symbol, a symbol.Annotation, indent string) {
	b.WriteString(indent)
	b.WriteString(decoratorAt)
	b.WriteString(w.decoratorName(d, a.Name))
	if len(a.Args) > 0 {
		b.WriteString(callOpen)
		b.WriteString(hugOpen)
		b.WriteString(strings.Join(a.Args, itemSep))
		b.WriteString(groupClose)
		b.WriteString(callClose)
	}
	b.WriteString(statementEnd)
}

// decoratorName writes the name of a decorator of d and records its import:
// the name that the import of a decorator of the table binds, and the name
// as it is otherwise, after the import of the module of its first name
// where the name is dotted.
func (w *Writer) decoratorName(d symbol.Symbol, name string) string {
	head, _, dotted := strings.Cut(name, dot)
	switch {
	case w.set.Reserved(head), head == declaredName(d), w.marked(head):
		return name
	case dotted:
		w.set.Add(head)
		return name
	}
	module, listed := decoratorModules[name]
	if !listed {
		return name
	}
	minor := minorOf(w.set)
	if name == deprecatedDecorator && minor < minor313 {
		module = typingExtensionsModule
	}
	return w.set.BindItem(typingPath(module, name, minor), name, false)
}

// marked reports whether an import of the import marker of the header of
// the file binds name: the first name of a module that it imports whole,
// the alias of a module, and a name that it imports from a module.
func (w *Writer) marked(name string) bool {
	for _, l := range w.imports {
		top, _, _ := strings.Cut(l.path, python.PackageSep)
		switch {
		case l.local != "":
			if l.local == name {
				return true
			}
		case l.name == "":
			if top == name {
				return true
			}
		case l.name == name:
			return true
		}
	}
	return false
}

// factDecorators appends to facts the names of the decorators that the facts
// of d state and that annotations does not, and returns the extended list.
// A method states at most [factCount] of them. host is the class of a
// method, and an abstract method of a Protocol states no abstractmethod.
func factDecorators(d, host symbol.Symbol, annotations symbol.Annotations, facts []string) []string {
	switch t := d.(type) {
	case *emit.Method:
		if t.Override {
			facts = append(facts, overrideDecorator)
		}
		if t.Final {
			facts = append(facts, finalDecorator)
		}
		switch {
		case t.Level != symbol.LevelType, t.Name == newName:
		case t.Receiver != nil || stated(annotations, classDecorator):
			facts = append(facts, classDecorator)
		default:
			facts = append(facts, staticDecorator)
		}
		switch t.Accessor {
		case symbol.AccessorGet:
			facts = append(facts, propertyDecorator)
		case symbol.AccessorSet:
			facts = append(facts, t.Name+setterSuffix)
		case symbol.AccessorDelete:
			facts = append(facts, t.Name+deleterSuffix)
		case symbol.AccessorNone:
		}
		if _, protocol := host.(*emit.Interface); t.Abstract && !protocol {
			facts = append(facts, abstractDecorator)
		}
	case *emit.Struct:
		if t.Final {
			facts = append(facts, finalDecorator)
		}
	}
	out := facts[:0]
	for _, f := range facts {
		if !stated(annotations, f) {
			out = append(out, f)
		}
	}
	return out
}

// stated reports whether annotations states the decorator name, bare or
// as the last name of a dotted name, as typing.final states final.
func stated(annotations symbol.Annotations, name string) bool {
	for _, a := range annotations {
		if a.Name == name || strings.HasSuffix(a.Name, dot+name) {
			return true
		}
	}
	return false
}

// annotationsOf returns the annotations of a declaration that Python
// decorates: a class or a callable.
func annotationsOf(d symbol.Symbol) symbol.Annotations {
	switch t := d.(type) {
	case *emit.Struct:
		return t.Annotations
	case *emit.Interface:
		return t.Annotations
	case *emit.Enum:
		return t.Annotations
	case *emit.Function:
		return t.Annotations
	case *emit.Method:
		return t.Annotations
	default:
		return nil
	}
}

// declaredName returns the name that d declares, the name of a method
// included.
func declaredName(d symbol.Symbol) string {
	if m, is := d.(*emit.Method); is {
		return m.Name
	}
	return emit.DeclaredName(d)
}
