// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The bases that the writer gives a class for a fact: ABC for an abstract
// class, and the base of an enum for its underlying type. auto gives a
// member of an enum without a value its value, and nonmember keeps the value
// of a field of an enum from becoming a member. NamedTuple is the base of a
// class whose fields are immutable without Final.
const (
	namedTupleName   = "NamedTuple"
	abcName          = "ABC"
	enumName         = "Enum"
	intEnumName      = "IntEnum"
	strEnumName      = "StrEnum"
	autoName         = "auto"
	nonmemberName    = "nonmember"
	intSpelling      = "int"
	strSpelling      = "str"
	classKeyword     = "class "
	metaclassKeyword = "metaclass="
)

// The parts of a functional form: the braces of its dictionary, the colon
// between a key and its value, and the separator of the arguments of the
// call.
const (
	dictOpen  = "{"
	dictClose = "}"
	keySep    = ": "
)

// The module and the call of the functional form of a named tuple of
// collections, and its keyword arguments.
const (
	collectionsModule = "collections"
	namedtupleCall    = "namedtuple"
	renameArg         = "rename=True"
	defaultsArg       = "defaults="
)

// The room that a class head reserves for each base, beside the spelling of
// a stated base, and the room of the statement around its name. A longer
// head grows its text once more.
const (
	baseRoom = 16
	headRoom = 32
)

// enumBases are the enum classes of the module enum, which the bases marker
// of an enum names bare and the writer imports from enum.
var enumBases = []string{"Enum", "IntEnum", "StrEnum", "Flag", "IntFlag", "ReprEnum"}

// Class is a class that its template writes around the parts that the
// builtins place: the head of the class, with its decorators, its class
// statement and its docstring, the lines of its fields or of the members of
// an enum, and the nested types and the methods that the template places.
// A class without a body has one head line, and a class of the functional
// form of a TypedDict or of an enum is one assignment in its head, without
// nested types or methods. The template reads the members from the fields
// of a Class, because text/template allocates every call of a method of
// the declaration.
type Class struct {
	Head    string
	Fields  string
	Types   []symbol.Symbol
	Methods []*emit.Method
}

// head is the class statement of a class that [Writer.writeHead] writes:
// its declaration, whose decorators it writes, its name, its type
// parameters of PEP 695, and its bases in order. The bases are the stated
// bases, which Python evaluates: the types that the class extends, the
// types that it implements and the types that it embeds. The names of the
// bases that the writer adds and the generic base of the type variables of
// the module that the class takes follow them, and the keywords of the
// statement come last. A head with an empty body writes the body ... on its
// line.
type head struct {
	decl       symbol.Symbol
	name       string
	typeParams string
	bases      []*emit.TypeRef
	implements []*emit.TypeRef
	embeds     []*emit.Embed
	names      []string
	// generic is the generic base, Generic or Protocol, and empty for a
	// class without one. params are its type variables, and a Protocol
	// without type variables has no brackets.
	generic  string
	params   []*emit.TypeParam
	keywords []string
	doc      []string
	comment  string
	empty    bool
}

// Struct starts the declaration s in the file with the header h, as every
// kind writer starts it, and writes the class of the struct s with its
// bases: the types that it extends, the types that it implements and the
// types that it embeds, Generic of its type parameters where they are type
// variables of the module, ABC where it is abstract and has no other base,
// and the keywords of its keywords marker. A class with the base NamedTuple
// writes its immutable fields without Final, and a NamedTuple in the form of
// [collectionsTuple] is the assignment of the functional form of namedtuple
// of collections. Each class writer takes the declaration of its kind, so
// text/template passes it without an interface conversion.
//
// Error modes: a base that the module declares later, a field whose name is
// no identifier, a field of a NamedTuple whose name starts with an
// underscore in the class syntax, the refusals of the functional form of a
// named tuple, of the types and of the type parameters, and the errors of
// the start of a declaration.
//
// # Allocation contract
//
// Struct allocates the head and the lines of the fields, and the type
// parameters of PEP 695 of a generic class.
func (w *Writer) Struct(h *emit.File, s *emit.Struct) (Class, error) {
	if err := w.declare(h, s); err != nil {
		return Class{}, err
	}
	return w.structClass(s)
}

// structClass writes the class of a struct.
func (w *Writer) structClass(s *emit.Struct) (Class, error) {
	typeParams, err := w.TypeParams(s.TypeParams)
	if err != nil {
		return Class{}, err
	}
	keywords, _ := markerArgs(s.Annotations, python.KeywordsMarker)
	h := head{
		decl: s, name: s.Name, typeParams: typeParams, bases: s.Extends, implements: s.Implements,
		embeds: s.Embeds.Items(), keywords: keywords, doc: s.Doc, comment: s.Comment,
	}
	if len(s.TypeParams) > 0 && w.legacy(s.TypeParams) {
		h.generic, h.params = w.typing(genericName), s.TypeParams
	}
	metaclass := slices.ContainsFunc(keywords, func(k string) bool { return strings.HasPrefix(k, metaclassKeyword) })
	stated := len(h.bases) + len(h.implements) + len(h.embeds)
	var abc [1]string
	if s.Abstract && stated == 0 && h.generic == "" && !metaclass {
		abc[0] = w.set.BindItem(abcBaseModule, abcName, false)
		h.names = abc[:]
	}
	host := classHost
	if slices.ContainsFunc(s.Extends, namedTuple) {
		host = tupleHost
	}
	if host == tupleHost && collectionsTuple(s) {
		return w.functionalTuple(s)
	}
	w.class = s
	fields, err := w.fields(s.Fields.Items(), host)
	w.class = nil
	if err != nil {
		return Class{}, err
	}
	h.empty = s.Methods.Len() == 0 && s.Types.Len() == 0 && fields == ""
	text, err := w.writeHead(h)
	return Class{Head: text, Fields: fields, Types: s.Types.Items(), Methods: s.Methods.Items()}, err
}

// collectionsTuple reports whether the NamedTuple s takes the form of
// namedtuple of collections, the one form of a field whose name starts with
// an underscore: such a field, fields at the level of instances without a
// type, a docstring, a comment or a marker, and no other part than the base
// NamedTuple. Every other NamedTuple is a class, whose fields a type checker
// reads with their types.
func collectionsTuple(s *emit.Struct) bool {
	fields := s.Fields.Items()
	bare := len(s.Extends) == 1 && len(s.Implements) == 0 && s.Embeds.Len() == 0 && len(s.TypeParams) == 0 &&
		s.Methods.Len() == 0 && s.Types.Len() == 0 && len(s.Annotations) == 0 &&
		slices.ContainsFunc(fields, func(f *emit.Field) bool { return strings.HasPrefix(f.Name, discardName) })
	return bare && !slices.ContainsFunc(fields, func(f *emit.Field) bool {
		return f.Type != nil || len(f.Doc) > 0 || f.Comment != "" || len(f.Annotations) > 0 ||
			f.Level == symbol.LevelType
	})
}

// functionalTuple writes the functional form of namedtuple of collections,
// as Row = namedtuple("Row", ["id", "_1"], rename=True, defaults=[0]), with
// the docstring after the assignment. rename=True is the one form of a field
// whose name starts with an underscore, because namedtuple renames such a
// field to an underscore and its index. The values of the last fields are
// the defaults.
//
// Error modes: a name with a leading underscore other than an underscore
// and the index of its field, and a field without a value after a field
// with one.
func (w *Writer) functionalTuple(s *emit.Struct) (Class, error) {
	fields := s.Fields.Items()
	names := make([]string, 0, len(fields))
	var defaults []string
	rename := false
	for i, f := range fields {
		if strings.HasPrefix(f.Name, discardName) {
			if f.Name != discardName+strconv.Itoa(i) {
				return Class{}, refuse("namedtuple renames a field with a leading underscore to an underscore and "+
					"its index, and %s is field %d", f.Name, i)
			}
			rename = true
		}
		switch {
		case f.Value != "":
			defaults = append(defaults, f.Value)
		case len(defaults) > 0:
			return Class{}, refuse("namedtuple gives its defaults to the last fields, and %s has no value after "+
				"a field with one", f.Name)
		}
		names = append(names, python.Quote(f.Name))
	}
	args := []string{python.Quote(s.Name), argsOpen + strings.Join(names, argSep) + argsClose}
	if rename {
		args = append(args, renameArg)
	}
	if len(defaults) > 0 {
		args = append(args, defaultsArg+argsOpen+strings.Join(defaults, argSep)+argsClose)
	}
	callee := w.set.BindItem(collectionsModule, namedtupleCall, false)
	return Class{Head: assignment(s.Name, callee, args, s.Doc, s.Comment)}, nil
}

// Interface starts the declaration i in the file with the header h, as
// every kind writer starts it, and writes the class of the interface i: a
// TypedDict with its bases or TypedDict for an interface with the TypedDict
// marker, and the assignment of the functional form of a TypedDict that has
// a key that is no identifier. Under the keyword total=False, a key that a
// value has takes Required, and a key that a value may lack takes no
// qualifier. Every other interface is a Protocol, after the interfaces that
// it extends.
//
// Error modes: a TypedDict with a method or a nested type, type parameters
// or bases in the functional form, a field whose name is no identifier in a
// class, the refusals of the types and the type parameters, and the errors
// of the start of a declaration.
//
// # Allocation contract
//
// Interface allocates the head and the lines of the fields, and the type
// parameters of PEP 695 of a generic class.
func (w *Writer) Interface(h *emit.File, i *emit.Interface) (Class, error) {
	if err := w.declare(h, i); err != nil {
		return Class{}, err
	}
	if _, typed := markerArgs(i.Annotations, python.TypedDictMarker); typed {
		return w.typedDict(i)
	}
	return w.protocol(i)
}

// protocol writes the Protocol of an interface.
func (w *Writer) protocol(i *emit.Interface) (Class, error) {
	typeParams, err := w.TypeParams(i.TypeParams)
	if err != nil {
		return Class{}, err
	}
	keywords, _ := markerArgs(i.Annotations, python.KeywordsMarker)
	h := head{
		decl: i, name: i.Name, typeParams: typeParams, bases: i.Extends, generic: w.typing(protocolName),
		keywords: keywords, doc: i.Doc, comment: i.Comment,
	}
	if len(i.TypeParams) > 0 && w.legacy(i.TypeParams) {
		h.params = i.TypeParams
	}
	w.class = i
	fields, err := w.fields(i.Fields.Items(), protocolHost)
	w.class = nil
	if err != nil {
		return Class{}, err
	}
	h.empty = i.Methods.Len() == 0 && i.Types.Len() == 0 && fields == ""
	text, err := w.writeHead(h)
	return Class{Head: text, Fields: fields, Types: i.Types.Items(), Methods: i.Methods.Items()}, err
}

// typedDict writes the TypedDict of an interface: a class, or the
// assignment of its functional form where a key is no identifier.
func (w *Writer) typedDict(i *emit.Interface) (Class, error) {
	switch {
	case i.Methods.Len() > 0:
		return Class{}, refuse("a TypedDict declares keys alone, and %s states methods", i.Name)
	case i.Types.Len() > 0:
		return Class{}, refuse("a TypedDict declares keys alone, and %s states nested types", i.Name)
	}
	keywords, _ := markerArgs(i.Annotations, python.KeywordsMarker)
	host := dictHost
	if slices.Contains(keywords, partialKeyword) {
		host = partialHost
	}
	if slices.ContainsFunc(i.Fields.Items(), func(f *emit.Field) bool { return !spell.IsIdentifier(f.Name) }) {
		return w.functionalDict(i, keywords, host)
	}
	typeParams, err := w.TypeParams(i.TypeParams)
	if err != nil {
		return Class{}, err
	}
	h := head{
		decl: i, name: i.Name, typeParams: typeParams, bases: i.Extends, keywords: keywords, doc: i.Doc,
		comment: i.Comment,
	}
	var dict [1]string
	if len(i.Extends) == 0 {
		dict[0] = w.typing(typedDictName)
		h.names = dict[:]
	}
	if len(i.TypeParams) > 0 && w.legacy(i.TypeParams) {
		h.generic, h.params = w.typing(genericName), i.TypeParams
	}
	w.class = i
	fields, err := w.fields(i.Fields.Items(), host)
	w.class = nil
	if err != nil {
		return Class{}, err
	}
	h.empty = fields == ""
	text, err := w.writeHead(h)
	return Class{Head: text, Fields: fields}, err
}

// functionalDict writes the functional form of a TypedDict, as
// Movie = TypedDict("Movie", {"name": str, "year-of": int}, total=False),
// with the docstring after the assignment. host is the kind of TypedDict
// that the keywords state.
func (w *Writer) functionalDict(i *emit.Interface, keywords []string, host fieldHost) (Class, error) {
	if len(i.TypeParams) > 0 || len(i.Extends) > 0 {
		return Class{}, refuse("the functional form of a TypedDict has no bases and no type parameters, "+
			"and %s states them", i.Name)
	}
	keys := make([]string, 0, i.Fields.Len())
	for _, f := range i.Fields.Items() {
		annotation, err := w.fieldType(f, host)
		if err != nil {
			return Class{}, err
		}
		keys = append(keys, python.Quote(f.Name)+keySep+annotation)
	}
	args := append([]string{python.Quote(i.Name), dictOpen + strings.Join(keys, argSep) + dictClose}, keywords...)
	return Class{Head: assignment(i.Name, w.typing(typedDictName), args, i.Doc, i.Comment)}, nil
}

// Enum starts the declaration e in the file with the header h, as every kind
// writer starts it, and writes the class of the enum e with its members: a
// class with the bases of its bases marker, or with Enum, IntEnum for an
// underlying int, StrEnum for an underlying str, or the underlying type and
// Enum. The fields of the enum follow its members as attributes that are no
// members, which is how Python declares the state of each member. An enum
// that has a member that is no identifier is the assignment of its
// functional form.
//
// Error modes: a method or a field in the functional form, a field whose
// name is no identifier, the refusals of the underlying type and of the
// types of the fields, and the errors of the start of a declaration.
//
// # Allocation contract
//
// Enum allocates the head and the lines of the members and the fields.
func (w *Writer) Enum(h *emit.File, e *emit.Enum) (Class, error) {
	if err := w.declare(h, e); err != nil {
		return Class{}, err
	}
	return w.enumClass(e)
}

// enumClass writes the class of an enum: a class with its members and then
// its fields, or the assignment of its functional form where a member is no
// identifier.
func (w *Writer) enumClass(e *emit.Enum) (Class, error) {
	var buf [2]string
	base, err := w.enumBases(e, buf[:0])
	if err != nil {
		return Class{}, err
	}
	variants := e.Variants.Items()
	if slices.ContainsFunc(variants, func(v *emit.EnumVariant) bool { return !spell.IsIdentifier(v.Name) }) {
		return w.functionalEnum(e, base)
	}
	fs := e.Fields.Items()
	size := 0
	for _, v := range variants {
		size += len(python.Indent) + len(v.Name) + len(assign) + len(v.Value) + len(autoName) + len(callOpen) +
			len(callClose) + len(trailingComment) + len(v.Comment) + len(newline) +
			docstringSize(v.Doc, python.Indent)
	}
	for _, f := range fs {
		size += fieldRoom + len(f.Name) + len(f.Value) + len(f.Comment) + docstringSize(f.Doc, python.Indent)
	}
	var b strings.Builder
	b.Grow(size)
	for _, v := range variants {
		b.WriteString(python.Indent)
		b.WriteString(v.Name)
		b.WriteString(assign)
		if v.Value == "" {
			b.WriteString(w.enumItem(autoName))
			b.WriteString(callOpen)
			b.WriteString(callClose)
		} else {
			b.WriteString(v.Value)
		}
		writeComment(&b, v.Comment)
		b.WriteString(newline)
		writeDocstring(&b, v.Doc, python.Indent)
	}
	w.class = e
	for _, f := range fs {
		if err = w.field(&b, f, enumHost); err != nil {
			break
		}
	}
	w.class = nil
	if err != nil {
		return Class{}, err
	}
	keywords, _ := markerArgs(e.Annotations, python.KeywordsMarker)
	h := head{
		decl: e, name: e.Name, names: base, keywords: keywords, doc: e.Doc, comment: e.Comment,
		empty: len(variants) == 0 && len(fs) == 0 && e.Methods.Len() == 0,
	}
	text, err := w.writeHead(h)
	return Class{Head: text, Fields: b.String(), Methods: e.Methods.Items()}, err
}

// functionalEnum writes the functional form of an enum, as
// Color = Enum("Color", {"dark-red": 1, "BLUE": 2}), with the docstring after
// the assignment. A member without a value takes auto().
func (w *Writer) functionalEnum(e *emit.Enum, base []string) (Class, error) {
	switch {
	case e.Methods.Len() > 0:
		return Class{}, refuse("the functional form of an enum has members alone, and %s states methods", e.Name)
	case e.Fields.Len() > 0:
		return Class{}, refuse("the functional form of an enum has members alone, and %s states fields", e.Name)
	}
	members := make([]string, 0, e.Variants.Len())
	for _, v := range e.Variants.Items() {
		value := v.Value
		if value == "" {
			value = w.enumItem(autoName) + callOpen + callClose
		}
		members = append(members, python.Quote(v.Name)+keySep+value)
	}
	callee := base[len(base)-1]
	args := []string{python.Quote(e.Name), dictOpen + strings.Join(members, argSep) + dictClose}
	return Class{Head: assignment(e.Name, callee, args, e.Doc, e.Comment)}, nil
}

// enumBases appends to out the bases of an enum, and returns the extended
// list: the arguments of its bases marker, each base of enum imported from
// enum, or Enum, IntEnum for an underlying int, StrEnum for an underlying
// str, and the underlying type and Enum for any other.
func (w *Writer) enumBases(e *emit.Enum, out []string) ([]string, error) {
	if stated, _ := markerArgs(e.Annotations, python.BasesMarker); len(stated) > 0 {
		for _, b := range stated {
			if slices.Contains(enumBases, b) && !w.marked(b) && !w.set.Reserved(b) {
				b = w.set.BindItem(enumModule, b, false)
			}
			out = append(out, b)
		}
		return out, nil
	}
	if e.Underlying == nil {
		return append(out, w.set.BindItem(enumModule, enumName, false)), nil
	}
	switch e.Underlying.Spelling {
	case intSpelling:
		return append(out, w.set.BindItem(enumModule, intEnumName, false)), nil
	case strSpelling:
		return append(out, w.set.BindItem(enumModule, strEnumName, false)), nil
	}
	underlying, err := w.SpellValue(e.Underlying)
	if err != nil {
		return nil, err
	}
	return append(out, underlying, w.set.BindItem(enumModule, enumName, false)), nil
}

// namedTuple reports whether the base r is NamedTuple of typing or of
// typing_extensions, by its name alone or qualified, as typing.NamedTuple.
func namedTuple(r *emit.TypeRef) bool {
	name := r.Spelling
	if _, last, qualified := strings.CutLast(name, dot); qualified {
		name = last
	}
	module := spellref.PackageOf(r, python.Lang)
	return name == namedTupleName && (module == typingModule || module == typingExtensionsModule)
}

// enumItem returns the name of the item name of the module enum, such as
// auto or nonmember: the name that its import binds, or that an import of
// the header binds.
func (w *Writer) enumItem(name string) string {
	if w.marked(name) {
		return name
	}
	return w.set.BindItem(enumModule, name, false)
}

// writeHead writes the head of a class: its decorators, its class statement
// with its type parameters and its bases, its trailing comment and its
// docstring, and the body ... on the line of the statement where the class
// has no body. Each stated base is in a value position, because Python
// evaluates the bases when it runs the class statement.
func (w *Writer) writeHead(h head) (string, error) {
	size := headRoom + len(h.name) + len(h.typeParams) + len(h.generic) + len(h.comment) +
		decoratorCount(h.decl, nil)*decoratorLine + docstringSize(h.doc, python.Indent) + len(h.params)*baseRoom
	for _, r := range h.bases {
		size += baseRoom + len(r.Spelling)
	}
	for _, r := range h.implements {
		size += baseRoom + len(r.Spelling)
	}
	for _, e := range h.embeds {
		size += baseRoom + len(e.Ref.Spelling)
	}
	for _, n := range h.names {
		size += len(itemSep) + len(n)
	}
	for _, k := range h.keywords {
		size += len(itemSep) + len(k)
	}
	var b strings.Builder
	b.Grow(size)
	w.writeDecorators(&b, h.decl, "")
	b.WriteString(classKeyword)
	b.WriteString(h.name)
	b.WriteString(h.typeParams)
	if len(h.bases)+len(h.implements)+len(h.embeds)+len(h.names)+len(h.keywords) > 0 || h.generic != "" {
		b.WriteString(callOpen)
		b.WriteString(hugOpen)
		first := true
		item := func() {
			if !first {
				b.WriteString(itemSep)
			}
			first = false
		}
		value := func(r *emit.TypeRef) error {
			base, err := w.SpellValue(r)
			if err != nil {
				return err
			}
			item()
			b.WriteString(base)
			return nil
		}
		for _, r := range h.bases {
			if err := value(r); err != nil {
				return "", err
			}
		}
		for _, r := range h.implements {
			if err := value(r); err != nil {
				return "", err
			}
		}
		for _, e := range h.embeds {
			if err := value(e.Ref); err != nil {
				return "", err
			}
		}
		for _, n := range h.names {
			item()
			b.WriteString(n)
		}
		if h.generic != "" {
			item()
			b.WriteString(h.generic)
			w.writeGenericArgs(&b, h.params)
		}
		for _, k := range h.keywords {
			item()
			b.WriteString(k)
		}
		b.WriteString(groupClose)
		b.WriteString(callClose)
	}
	if h.empty && len(h.doc) == 0 {
		b.WriteString(inlineBody)
		writeComment(&b, h.comment)
		b.WriteString(newline)
		return b.String(), nil
	}
	b.WriteString(bodyOpener)
	writeComment(&b, h.comment)
	b.WriteString(newline)
	writeDocstring(&b, h.doc, python.Indent)
	return b.String(), nil
}

// assignment writes the assignment of the functional form of a class: the
// name, the call of callee with the arguments args, the trailing comment,
// and the docstring after the assignment.
func assignment(name, callee string, args, doc []string, comment string) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(assign)
	b.WriteString(callee)
	b.WriteString(callOpen)
	b.WriteString(hugOpen)
	b.WriteString(strings.Join(args, itemSep))
	b.WriteString(groupClose)
	b.WriteString(callClose)
	writeComment(&b, comment)
	b.WriteString(newline)
	writeDocstring(&b, doc, "")
	return b.String()
}

// markerArgs returns the arguments of the marker m of annotations and
// reports whether annotations states the marker. A marker without arguments
// returns no arguments and true.
func markerArgs(annotations symbol.Annotations, m python.Marker) ([]string, bool) {
	for _, a := range annotations {
		if a.Name == string(m) {
			return a.Args, true
		}
	}
	return nil, false
}
