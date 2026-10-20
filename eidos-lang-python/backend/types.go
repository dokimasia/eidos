// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The modules whose names the writer spells: the builtins, typing, the
// abstract collections, and typing_extensions for a name of typing that the
// target version does not have yet.
const (
	builtinsModule = "builtins"
	typingModule   = "typing"
	abcModule      = "collections/abc"
	enumModule     = "enum"
	abcBaseModule  = "abc"
	dataModule     = "dataclasses"
)

// The names of typing and of the abstract collections that the writer
// writes for a structural type, a qualifier and the top type, and cast,
// which states the type of a value of an enum that is no member.
const (
	anyName           = "Any"
	neverName         = "Never"
	callableName      = "Callable"
	iteratorName      = "Iterator"
	asyncIteratorName = "AsyncIterator"
	mappingName       = "Mapping"
	classVarName      = "ClassVar"
	finalName         = "Final"
	notRequiredName   = "NotRequired"
	requiredName      = "Required"
	readOnlyName      = "ReadOnly"
	annotatedName     = "Annotated"
	initVarName       = "InitVar"
	typeAliasName     = "TypeAlias"
	newTypeName       = "NewType"
	protocolName      = "Protocol"
	typedDictName     = "TypedDict"
	genericName       = "Generic"
	overrideName      = "override"
	castName          = "cast"
)

// The builtins that the writer writes for a structural type and the top
// type. None is a keyword, so no declaration hides it.
const (
	noneName   = "None"
	objectName = "object"
	tupleName  = "tuple"
	listName   = "list"
	dictName   = "dict"
)

// The syntax of a type expression: the brackets of the arguments of a
// generic class, the separator of two arguments, the bar of a union, the
// ellipsis of a tuple of any length, the empty tuple and the dot of a
// qualified name.
const (
	argsOpen   = "["
	argsClose  = "]"
	argSep     = ", "
	unionSep   = " | "
	ellipsis   = "..."
	emptyTuple = "()"
	dot        = "."
)

// doubleQuote is the quote of a string that the writer writes without
// [python.Quote], because its text needs no escape, such as an identifier of
// __all__ and the name of a type variable.
const doubleQuote = `"`

// newTyping maps each name of typing that a version after 3.11 adds to the
// minor version that adds it. A file for an earlier version imports the
// name from typing_extensions, which has every name. The table lists the
// names that the stub of typing in typeshed declares under a check of
// sys.version_info.
var newTyping = map[string]int{
	overrideName:           minor312,
	"TypeAliasType":        minor312,
	readOnlyName:           minor313,
	"TypeIs":               minor313,
	"NoDefault":            minor313,
	"get_protocol_members": minor313,
	"is_protocol":          minor313,
	"evaluate_forward_ref": minor314,
	"TypeForm":             minor315,
	"NoExtraItems":         minor315,
	"disjoint_base":        minor315,
}

// typingPath returns the package path from which a file for the minor
// version minor imports name from the module whose package path is path:
// typing_extensions for a name of typing that the version does not have,
// and path otherwise.
func typingPath(path, name string, minor int) string {
	if added, newer := newTyping[name]; path == typingModule && newer && minor < added {
		return typingExtensionsModule
	}
	return path
}

// spelling is the text of a type and what the text names: whether it names
// a declaration of the file that has not finished rendering, and whether it
// names a declaration of the file at all.
type spelling struct {
	text    string
	pending bool
	local   bool
}

// Spell writes the type reference t in an annotation, and records the
// import of each declaration of another module that it names:
//
//   - a named reference writes its name and its arguments in brackets. A
//     declaration of another module imports from its module, under the
//     name that the import binds, and a name of typing that the target
//     version does not have imports from typing_extensions. A qualified
//     reference keeps its spelling: one whose first name the header of the
//     file binds imports nothing more, and one through a module, such as
//     json.JSONDecoder, imports the module. A declaration of a module that
//     the header imports with a star writes its name and imports nothing,
//     unless a declaration of the file takes the name or the name starts
//     with an underscore, which a star import does not bind. A type
//     parameter of the declaration under writing whose type variable of the
//     module takes another name writes that name. In a body, the
//     declaration under writing is the callable of the body and its host;
//   - an optional writes the union of its type and None, a union its
//     members with bars, a tuple tuple[A, B] and tuple[()] without members,
//     a function type Callable[[A, B], R], a list list[T] and an immutable
//     one tuple[T, ...], a map dict[K, V] and an immutable one Mapping[K, V],
//     a stream Iterator[T] and an asynchronous one AsyncIterator[T], a
//     borrow its type and a wildcard its bound;
//   - a nil reference writes the choice of python.dynamic: object, or Any
//     of typing.
//
// A builtin is builtins.name where a declaration of the file hides its name,
// and the file then imports the module builtins. A member of the class
// under writing hides the name too, because Python evaluates the
// annotations of a class in the namespace of its body. That covers a named
// reference of the module builtins, and the tuple, list, dict and object
// that the forms write.
//
// The writer writes a structural reference from its structure and not from
// its written spelling, because the spelling of Python names its forms,
// such as Optional and Callable, and the file imports them. In a module for
// Python 3.11 to 3.13, an annotation that names a declaration of the file
// that has not finished rendering is a string, because Python evaluates the
// annotation when it runs the definition. A stub file and a module for
// Python 3.14 and later write no string.
//
// Error modes: an intersection, an inline type, a structural type without
// the type that it contains, an array of an unstated length, a function
// type whose results start outside its types, a form of the projection,
// the module of a reference under a name that the file binds otherwise,
// and the refusals of a type parameter that a body names before its
// signature declares it.
func (w *Writer) Spell(t *emit.TypeRef) (string, error) {
	s, err := w.spell(t)
	if err != nil {
		return "", err
	}
	if s.pending && !w.stub && minorOf(w.set) < minor314 {
		return python.Quote(s.text), nil
	}
	return s.text, nil
}

// SpellValue writes the type reference t where Python evaluates it when the
// file runs, such as a base of a class, as [Writer.Spell] writes it without
// the string form. It returns an error for a reference to a declaration of
// the file that has not finished rendering in a module, because the module
// has not defined it yet when it evaluates the reference.
func (w *Writer) SpellValue(t *emit.TypeRef) (string, error) {
	s, err := w.spell(t)
	switch {
	case err != nil:
		return "", err
	case s.pending && !w.stub:
		return "", refuse("Python evaluates %s when the module runs, and the module declares it later", s.text)
	}
	return s.text, nil
}

// spellLazy writes the type reference t where Python evaluates it when a
// reader asks for it, such as the bound of a type parameter of PEP 695 and
// the target of a type statement, as [Writer.Spell] writes it without the
// string form.
func (w *Writer) spellLazy(t *emit.TypeRef) (string, error) {
	s, err := w.spell(t)
	return s.text, err
}

// spellLater writes the type reference t where Python evaluates it before
// every declaration of the file, such as the bound of a type variable: as a
// string where it names a declaration of the file, in a module.
func (w *Writer) spellLater(t *emit.TypeRef) (string, error) {
	s, err := w.spell(t)
	if err != nil {
		return "", err
	}
	if s.local && !w.stub {
		return python.Quote(s.text), nil
	}
	return s.text, nil
}

// spell writes the type reference t by the rules of [Writer.Spell], without
// the string form.
func (w *Writer) spell(t *emit.TypeRef) (spelling, error) {
	if t == nil {
		return spelling{text: w.dynamic()}, nil
	}
	switch t.Form {
	case symbol.FormNamed:
		return w.named(t)
	case symbol.FormOptional:
		inner, err := w.child(t)
		inner.text += unionSep + noneName
		return inner, err
	case symbol.FormUnion:
		if len(t.Elems) == 0 {
			return spelling{text: w.typing(neverName)}, nil
		}
		return w.joined(t.Elems, "", unionSep, "")
	case symbol.FormTuple:
		if len(t.Elems) == 0 {
			return spelling{text: w.builtin(tupleName, tupleName+argsOpen+emptyTuple+argsClose)}, nil
		}
		return w.joined(t.Elems, w.builtin(tupleName, tupleName+argsOpen), argSep, argsClose)
	case symbol.FormList:
		elem, err := w.child(t)
		if t.Mutability == symbol.MutabilityImmutable {
			elem.text = w.builtin(tupleName, tupleName+argsOpen) + elem.text + argSep + ellipsis + argsClose
		} else {
			elem.text = w.builtin(listName, listName+argsOpen) + elem.text + argsClose
		}
		return elem, err
	case symbol.FormArray:
		return w.array(t)
	case symbol.FormMap:
		if t.Mutability == symbol.MutabilityImmutable {
			return w.joined(t.Elems, w.set.BindItem(abcModule, mappingName, false)+argsOpen, argSep, argsClose)
		}
		return w.joined(t.Elems, w.builtin(dictName, dictName+argsOpen), argSep, argsClose)
	case symbol.FormFunc:
		return w.function(t)
	case symbol.FormStream:
		head := iteratorName
		if t.Async {
			head = asyncIteratorName
		}
		head = w.set.BindItem(abcModule, head, false)
		return w.joined(t.Elems, head+argsOpen, argSep, argsClose)
	case symbol.FormBorrow:
		return w.child(t)
	case symbol.FormWildcard:
		if len(t.Elems) == 0 {
			return spelling{text: w.builtin(objectName, objectName)}, nil
		}
		return w.child(t)
	case symbol.FormIntersection:
		return spelling{}, refuse("%s is an intersection of types, which Python declares as a Protocol", t.Spelling)
	case symbol.FormInline:
		return spelling{}, refuse("%s is an inline type, which Python declares as a class", t.Spelling)
	default:
		return spelling{}, refuse("%s has a form of the projection, which no backend writes", t.Spelling)
	}
}

// named writes a named reference by the rules of [Writer.Spell].
func (w *Writer) named(t *emit.TypeRef) (spelling, error) {
	if t.Spelling == "" {
		return spelling{text: w.dynamic()}, nil
	}
	head, rest, qualified := strings.Cut(t.Spelling, dot)
	var s spelling
	module := spellref.PackageOf(t, python.Lang)
	switch {
	case module == builtinsModule:
		s.text = w.builtin(head, t.Spelling)
	case module == "" || module == w.set.Home():
		s.text = t.Spelling
		variable, renamed, err := w.renamed(head)
		if err != nil {
			return spelling{}, err
		}
		if renamed {
			s.text = variable + t.Spelling[len(head):]
		}
		s.local = w.set.Reserved(head)
		s.pending = w.set.Pending(head)
	case qualified && w.marked(head):
		s.text = t.Spelling
	case qualified && moduleHead(t, head):
		if err := w.importModule(t, head); err != nil {
			return spelling{}, err
		}
		s.text = t.Spelling
	case !w.set.Reserved(head) && !strings.HasPrefix(head, discardName) &&
		w.set.Has(typingPath(module, head, minorOf(w.set)), python.StarName):
		s.text = t.Spelling
	default:
		s.text = w.set.BindItem(typingPath(module, head, minorOf(w.set)), head, false)
		if qualified {
			s.text += dot + rest
		}
	}
	if len(t.Args) == 0 {
		return s, nil
	}
	args, err := w.joined(t.Args, s.text+argsOpen, argSep, argsClose)
	args.pending = args.pending || s.pending
	args.local = args.local || s.local
	return args, err
}

// moduleHead reports whether the first name head of the qualified reference
// t names a module: the target of t is no member of a class, or t has no
// target and the first name of its package is head. Any other head is a
// class that the module of t declares, as Store of Store.Row.
func moduleHead(t *emit.TypeRef, head string) bool {
	if !t.Target.IsZero() {
		return t.Target.Owner == ""
	}
	top, _, _ := strings.Cut(t.Package, python.PackageSep)
	return top == head
}

// importModule records the import of the module through which the qualified
// reference t names its declaration, as import json for json.JSONDecoder.
// The module is the module of the import of t, or the module that the names
// of the spelling before its last name state. A head that is the first name
// of the module imports the module whole. Any other head imports the module
// under that name, as import numpy as np for np.ndarray.
//
// Error modes: a head that a declaration of the file takes, and a head of a
// module under another name that another import of the file binds.
func (w *Writer) importModule(t *emit.TypeRef, head string) error {
	module := t.Package
	if module == "" {
		prefix, _, _ := strings.CutLast(t.Spelling, dot)
		module = strings.ReplaceAll(prefix, dot, python.PackageSep)
	}
	if top, _, _ := strings.Cut(module, python.PackageSep); top == head {
		if w.set.Reserved(head) {
			return refuse("%s names the module %s, and a declaration of the file takes the name %s",
				t.Spelling, strings.ReplaceAll(module, python.PackageSep, python.ModuleSep), head)
		}
		w.set.Add(module)
		return nil
	}
	if err := w.set.Use(module+moduleMark, head, ""); err != nil {
		return refuse("%s names the module %s as %s, and the file binds %s otherwise",
			t.Spelling, strings.ReplaceAll(module, python.PackageSep, python.ModuleSep), head, head)
	}
	return nil
}

// child writes the first child of a structural reference, and refuses a
// reference without children.
func (w *Writer) child(t *emit.TypeRef) (spelling, error) {
	if len(t.Elems) == 0 {
		return spelling{}, refuse("%s is a structural type without the type that it contains", t.Spelling)
	}
	return w.spell(t.Elems[0])
}

// joined writes the references ts between open and closer, separated by sep.
// It sizes the text for the written spellings of the references, so the
// text of references that keep their spellings allocates once.
func (w *Writer) joined(ts []*emit.TypeRef, open, sep, closer string) (spelling, error) {
	size := len(open) + len(closer) + len(sep)*len(ts)
	for _, t := range ts {
		if t != nil {
			size += len(t.Spelling)
		}
	}
	var b strings.Builder
	b.Grow(size)
	b.WriteString(open)
	var out spelling
	for i, t := range ts {
		s, err := w.spell(t)
		if err != nil {
			return spelling{}, err
		}
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s.text)
		out.pending = out.pending || s.pending
		out.local = out.local || s.local
	}
	b.WriteString(closer)
	out.text = b.String()
	return out, nil
}

// array writes an array of a stated length as a tuple of that many members.
func (w *Writer) array(t *emit.TypeRef) (spelling, error) {
	if t.Length == 0 {
		return spelling{}, refuse("%s is an array of an unstated length, which Python does not spell", t.Spelling)
	}
	elem, err := w.child(t)
	if err != nil {
		return spelling{}, err
	}
	parts := make([]string, t.Length)
	for i := range parts {
		parts[i] = elem.text
	}
	elem.text = w.builtin(tupleName, tupleName+argsOpen) + strings.Join(parts, argSep) + argsClose
	return elem, nil
}

// function writes a function type as Callable[[A, B], R]: None for no
// result, the result for one, and a tuple of the results for two or more.
func (w *Writer) function(t *emit.TypeRef) (spelling, error) {
	if t.Split < 0 || t.Split > len(t.Elems) {
		return spelling{}, refuse("%s is a function type whose results start at %d of %d types",
			t.Spelling, t.Split, len(t.Elems))
	}
	head := w.set.BindItem(abcModule, callableName, false)
	params, err := w.joined(t.Elems[:t.Split], head+argsOpen+argsOpen, argSep, argsClose+argSep)
	if err != nil {
		return spelling{}, err
	}
	results := t.Elems[t.Split:]
	var result spelling
	switch len(results) {
	case 0:
		result.text = noneName
	case 1:
		result, err = w.spell(results[0])
	default:
		result, err = w.joined(results, w.builtin(tupleName, tupleName+argsOpen), argSep, argsClose)
	}
	if err != nil {
		return spelling{}, err
	}
	return spelling{
		text:    params.text + result.text + argsClose,
		pending: params.pending || result.pending,
		local:   params.local || result.local,
	}, nil
}

// dynamic writes the type of a value of any type: the choice of
// python.dynamic, object or Any of typing.
func (w *Writer) dynamic() string {
	if c, declared := w.set.Policy().Lookup(python.DynamicPolicy); declared && c == python.Any {
		return w.typing(anyName)
	}
	return w.builtin(objectName, objectName)
}

// builtin returns text, which starts with name, a name of builtins, where
// nothing hides the name. A declaration of the file and a member of the
// class under writing hide it. builtin returns text after builtins and a
// dot for a hidden name, and records the import of the module builtins. It
// allocates the qualified text alone.
func (w *Writer) builtin(name, text string) string {
	if !w.set.Reserved(name) && !binds(w.class, name) {
		return text
	}
	w.set.Add(builtinsModule)
	return builtinsModule + dot + text
}

// binds reports whether the body of the class c binds name: a field, a
// method, a nested type or a member of an enum of that name. A field without
// a value counts too, because a type checker reads the name as the field. A
// nil c binds nothing. binds allocates nothing.
func binds(c symbol.Symbol, name string) bool {
	var (
		fields  []*emit.Field
		methods []*emit.Method
		types   []symbol.Symbol
	)
	switch t := c.(type) {
	case *emit.Struct:
		fields, methods, types = t.Fields.Items(), t.Methods.Items(), t.Types.Items()
	case *emit.Interface:
		fields, methods, types = t.Fields.Items(), t.Methods.Items(), t.Types.Items()
	case *emit.Enum:
		if slices.ContainsFunc(t.Variants.Items(), func(v *emit.EnumVariant) bool { return v.Name == name }) {
			return true
		}
		fields, methods = t.Fields.Items(), t.Methods.Items()
	}
	return slices.ContainsFunc(fields, func(f *emit.Field) bool { return f.Name == name }) ||
		slices.ContainsFunc(methods, func(m *emit.Method) bool { return m.Name == name }) ||
		slices.ContainsFunc(types, func(d symbol.Symbol) bool { return emit.DeclaredName(d) == name })
}

// typing writes a name of typing under the name that its import binds,
// from typing_extensions where the target version does not have the name.
func (w *Writer) typing(name string) string {
	return w.set.BindItem(typingPath(typingModule, name, minorOf(w.set)), name, false)
}
