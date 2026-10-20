// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The parts of a signature: the keywords of a definition, the receivers of
// an instance and of a class method, the separators of the positional and
// the keyword parameters, the marks of the variadic parameters, the arrow
// of the result, and the body of a callable without content.
const (
	defKeyword      = "def "
	asyncKeyword    = "async "
	selfName        = "self"
	clsName         = "cls"
	positionalSep   = "/"
	keywordSep      = "*"
	variadicMark    = "*"
	keywordsMark    = "**"
	resultArrow     = " -> "
	annotationSep   = ": "
	defaultEquals   = "="
	bodyOpener      = ":"
	inlineBody      = ": ..."
	emptyBody       = "..."
	discardName     = "_"
	initName        = "__init__"
	newName         = "__new__"
	getItemName     = "__getitem__"
	setItemName     = "__setitem__"
	valueParam      = "value"
	trailingComment = "  # "
)

// The room that a definition reserves for each parameter beside its name,
// its default and its comment, and for its results. A longer definition
// grows its text once more.
const (
	paramRoom  = 24
	resultRoom = 24
)

// callable is the part of a function or a method that its signature writes.
type callable struct {
	name       string
	async      bool
	typeParams []*emit.TypeParam
	params     []*emit.Param
	returns    []*emit.Return
	receiver   string
	comment    string
	doc        []string
}

// Function starts the declaration f in the file with the header h, as every
// kind writer starts it, and writes the module-level function f with its
// decorators, its signature, its docstring and its body. body is the
// content that the body builtin placed. A callable without content has the
// body ..., on the line of its signature where it has no docstring. A stub
// file writes ... for every body. The signature is on one line where it
// fits in the line, and wraps as [Layout.Format] wraps a group otherwise.
//
// A module declares a function of a name once, outside the overload
// signatures of one function. The import set of the file records each
// function without the mark Overload, so the writer refuses a later
// function of its name.
//
// Error modes: a function of a name that an earlier function of the file
// without the mark Overload declares, the refusals of the type parameters,
// the parameters and the results, and the errors of the start of a
// declaration.
func (w *Writer) Function(h *emit.File, f *emit.Function, body string) (string, error) {
	if err := w.declare(h, f); err != nil {
		return "", err
	}
	if w.set.Has(implementedPath, f.Name) {
		return "", refuse("a module declares two functions named %s, and a module declares a name once "+
			"outside the overloads of one function", f.Name)
	}
	if !f.Overload {
		w.set.AddNamed(implementedPath, f.Name)
	}
	c := callable{
		name: f.Name, async: f.Async, typeParams: f.TypeParams, params: f.Params.Items(),
		returns: f.Returns.Items(), comment: f.Comment, doc: f.Doc,
	}
	return w.definition(f, c, "", body)
}

// Method writes a method of the class host with its decorators, its
// signature, its docstring and its body, one level inside the class, as
// [Writer.Function] writes a function. The receiver of an instance method is
// self or the name of the receiver that the method states, the receiver of a
// class method is cls, and a static method has none. A constructing method
// is __init__, or __new__ where the method has that name, and an index
// signature writes __getitem__, and __setitem__ beside it where its entries
// can change.
//
// The method spells the type parameters of its class by the names of their
// type variables of the module, as the class does. The members of the class
// hide the builtins of their names in the signature and the decorators.
//
// Error modes: the refusals of [Writer.Function], a constructing method of a
// Protocol, which Python does not declare, an index signature without one
// key parameter and one result, and the refusals of the type parameters of
// the class.
func (w *Writer) Method(host symbol.Symbol, m *emit.Method, body string) (string, error) {
	if _, protocol := host.(*emit.Interface); protocol && m.Constructs {
		return "", refuse("a Protocol declares no construct signature, and %s states one", m.Name)
	}
	w.scope = w.scope[:0]
	if params := typeParamsOf(host); len(params) > 0 && w.legacy(params) {
		if err := w.declareVars(params); err != nil {
			return "", err
		}
	}
	w.class = host
	defer func() { w.class = nil }()
	if m.Indexer {
		return w.indexer(m, body)
	}
	c := callable{
		name: m.Name, async: m.Async, typeParams: m.TypeParams, params: m.Params.Items(),
		returns: m.Returns.Items(), receiver: receiverOf(m), comment: m.Comment, doc: m.Doc,
	}
	if m.Constructs && m.Name != newName {
		c.name = initName
	}
	return w.definition(m, c, python.Indent, body)
}

// indexer writes the index signature m as __getitem__, and as __setitem__
// beside it where a holder can assign its entries.
func (w *Writer) indexer(m *emit.Method, body string) (string, error) {
	params, returns := m.Params.Items(), m.Returns.Items()
	if len(params) != 1 || len(returns) != 1 {
		return "", refuse("an index signature takes one key and returns one value, and %s does not", m.Name)
	}
	get := callable{
		name: getItemName, params: params, returns: returns, receiver: selfName, comment: m.Comment, doc: m.Doc,
	}
	text, err := w.definition(m, get, python.Indent, body)
	if err == nil && m.Mutability != symbol.MutabilityImmutable {
		set := callable{
			name: setItemName, receiver: selfName,
			params: []*emit.Param{params[0], {Name: valueParam, Type: returns[0].Type}},
		}
		var setText string
		setText, err = w.definition(nil, set, python.Indent, "")
		text += newline + setText
	}
	return text, err
}

// typeParamsOf returns the type parameters of a class, a function or a
// method d, and nil for a declaration without them.
func typeParamsOf(d symbol.Symbol) []*emit.TypeParam {
	switch t := d.(type) {
	case *emit.Struct:
		return t.TypeParams
	case *emit.Interface:
		return t.TypeParams
	case *emit.Function:
		return t.TypeParams
	case *emit.Method:
		return t.TypeParams
	default:
		return nil
	}
}

// receiverOf returns the receiver of a method: none for a static method, cls
// for a class method, and the name of the stated receiver or self for an
// instance method.
func receiverOf(m *emit.Method) string {
	switch {
	case m.Level == symbol.LevelType && m.Receiver == nil && !stated(m.Annotations, classDecorator):
		return ""
	case m.Level == symbol.LevelType:
		return clsName
	case m.Receiver != nil && m.Receiver.Name != "":
		return m.Receiver.Name
	default:
		return selfName
	}
}

// definition writes the decorators of d and the def line of c at the
// indentation indent, with its docstring and its body: the body text, or ...
// where the body is empty or the file is a stub. A nil d has no decorators.
func (w *Writer) definition(d symbol.Symbol, c callable, indent, body string) (string, error) {
	typeParams, err := w.TypeParams(c.typeParams)
	if err != nil {
		return "", err
	}
	if w.stub {
		body = ""
	}
	inner := indent + python.Indent
	size := len(indent) + len(asyncKeyword) + len(defKeyword) + len(c.name) + len(typeParams) +
		len(callOpen) + len(c.receiver) + len(callClose) + resultRoom + len(inlineBody) + len(trailingComment) +
		len(c.comment) + len(newline) + docstringSize(c.doc, inner) + len(inner) + len(emptyBody) + len(body)
	for _, p := range c.params {
		size += paramRoom + len(p.Name) + len(p.Default) + len(p.Comment)
	}
	if d != nil {
		size += decoratorCount(d, w.class) * (decoratorLine + len(indent))
	}
	var b strings.Builder
	b.Grow(size)
	if d != nil {
		w.writeDecorators(&b, d, indent)
	}
	b.WriteString(indent)
	if c.async {
		b.WriteString(asyncKeyword)
	}
	b.WriteString(defKeyword)
	b.WriteString(c.name)
	b.WriteString(typeParams)
	b.WriteString(callOpen)
	if err := w.writeParams(&b, c); err != nil {
		return "", err
	}
	b.WriteString(callClose)
	if err := w.writeResults(&b, c.returns); err != nil {
		return "", err
	}
	switch {
	case body == "" && len(c.doc) == 0:
		b.WriteString(inlineBody)
		writeComment(&b, c.comment)
		b.WriteString(newline)
	case body == "":
		b.WriteString(bodyOpener)
		writeComment(&b, c.comment)
		b.WriteString(newline)
		writeDocstring(&b, c.doc, inner)
		b.WriteString(inner)
		b.WriteString(emptyBody)
		b.WriteString(newline)
	default:
		b.WriteString(bodyOpener)
		writeComment(&b, c.comment)
		b.WriteString(newline)
		writeDocstring(&b, c.doc, inner)
		b.WriteString(body)
	}
	return b.String(), nil
}

// params writes the parameter group of c, as [Writer.writeParams] writes
// it, as a text of its own.
func (w *Writer) params(c callable) (string, error) {
	var b strings.Builder
	if err := w.writeParams(&b, c); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeParams writes the parameter group of c into b: the receiver, the
// parameters, a slash after the last parameter that a caller passes by
// position alone, and a star before the first parameter that a caller
// passes by keyword alone where no variadic parameter precedes it. A
// parameter without a name is _, with a number after the first. A parameter
// has its annotation and its default, ... in a stub file. A parameter that
// a caller may leave out and that has no default takes None as its default,
// with None among its types. The group writes nothing for a callable
// without parameters.
//
// Error modes: a name that is no identifier, a variadic parameter with a
// default, and the refusals of the types.
func (w *Writer) writeParams(b *strings.Builder, c callable) error {
	if c.receiver == "" && len(c.params) == 0 {
		return nil
	}
	b.WriteString(hugOpen)
	first := true
	item := func() {
		if !first {
			b.WriteString(itemSep)
		}
		first = false
	}
	if c.receiver != "" {
		item()
		b.WriteString(c.receiver)
	}
	positional := lastPositional(c.params)
	starred := false
	unnamed := 0
	for i, p := range c.params {
		if p.Passing == symbol.PassingKeyword && !starred {
			item()
			b.WriteString(keywordSep)
		}
		item()
		if err := w.writeParam(b, p, &unnamed); err != nil {
			return err
		}
		starred = starred || p.Variadic == symbol.VariadicPositional || p.Passing == symbol.PassingKeyword
		if i == positional {
			item()
			b.WriteString(positionalSep)
		}
	}
	b.WriteString(groupClose)
	return nil
}

// lastPositional returns the index of the last parameter that a caller
// passes by position alone, and -1 where no parameter is one.
func lastPositional(params []*emit.Param) int {
	last := -1
	for i, p := range params {
		if p.Passing == symbol.PassingPositional {
			last = i
		}
	}
	return last
}

// writeParam writes one parameter of a signature into b as an item of its
// group, with its trailing comment after the mark of a comment. unnamed
// counts the parameters without a name so far.
func (w *Writer) writeParam(b *strings.Builder, p *emit.Param, unnamed *int) error {
	name := p.Name
	switch {
	case name == "":
		name = discardName
		if *unnamed > 0 {
			name += strconv.Itoa(*unnamed)
		}
		*unnamed++
	case !spell.IsIdentifier(name):
		return refuse("a parameter has an identifier as its name, and %q is not one", name)
	}
	def := p.Default
	optional := p.Optional && def == "" && p.Variadic == symbol.VariadicNone
	switch {
	case def != "" && p.Variadic != symbol.VariadicNone:
		return refuse("a variadic parameter takes no default, and %s states one", name)
	case optional:
		def = noneName
	}
	switch p.Variadic {
	case symbol.VariadicPositional:
		b.WriteString(variadicMark)
	case symbol.VariadicKeyword:
		b.WriteString(keywordsMark)
	case symbol.VariadicNone:
	}
	b.WriteString(name)
	annotated := p.Type != nil
	if annotated {
		b.WriteString(annotationSep)
		if err := w.writeAnnotation(b, p.Type, p.Annotations, optional); err != nil {
			return err
		}
	}
	if def != "" {
		if w.stub {
			def = emptyBody
		}
		if annotated {
			b.WriteString(assign)
		} else {
			b.WriteString(defaultEquals)
		}
		b.WriteString(def)
	}
	if p.Comment != "" {
		b.WriteString(commentMark)
		b.WriteString(p.Comment)
	}
	return nil
}

// writeAnnotation writes the annotation of the type t into b: the type with
// None among its types where optional is true, inside Annotated with the
// metadata of the annotated marker of annotations.
func (w *Writer) writeAnnotation(
	b *strings.Builder,
	t *emit.TypeRef,
	annotations symbol.Annotations,
	optional bool,
) error {
	text, err := w.Spell(t)
	if err != nil {
		return err
	}
	metadata, marked := markerArgs(annotations, python.AnnotatedMarker)
	if marked {
		b.WriteString(w.typing(annotatedName))
		b.WriteString(argsOpen)
	}
	b.WriteString(text)
	if optional && t.Form != symbol.FormOptional {
		b.WriteString(unionSep)
		b.WriteString(noneName)
	}
	if marked {
		writeMetadata(b, metadata)
	}
	return nil
}

// results writes the result annotation of rs as [Writer.writeResults]
// writes it, as a text of its own.
func (w *Writer) results(rs []*emit.Return) (string, error) {
	var b strings.Builder
	if err := w.writeResults(&b, rs); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeResults writes the result annotation of a callable into b: -> None
// without a result, the type of one result and nothing for one result
// without a type, and a tuple of the results for two or more.
func (w *Writer) writeResults(b *strings.Builder, rs []*emit.Return) error {
	switch len(rs) {
	case 0:
		b.WriteString(resultArrow)
		b.WriteString(noneName)
		return nil
	case 1:
		if rs[0].Type == nil {
			return nil
		}
		text, err := w.Spell(rs[0].Type)
		if err != nil {
			return err
		}
		b.WriteString(resultArrow)
		b.WriteString(text)
		return nil
	}
	types := make([]*emit.TypeRef, 0, len(rs))
	for _, r := range rs {
		types = append(types, r.Type)
	}
	text, err := w.Spell(&emit.TypeRef{Form: symbol.FormTuple, Elems: types})
	if err != nil {
		return err
	}
	b.WriteString(resultArrow)
	b.WriteString(text)
	return nil
}

// writeComment writes the trailing comment of a line into b, and nothing for
// an empty comment.
func writeComment(b *strings.Builder, comment string) {
	if comment != "" {
		b.WriteString(trailingComment)
		b.WriteString(comment)
	}
}
