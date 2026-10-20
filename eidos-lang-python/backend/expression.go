// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"

	"go.dokimi.dev/eidos/lang/treesitter"
	pygrammar "go.dokimi.dev/eidos/lang/treesitter/python"
)

// The sources in which the lowering parses a text of a declaration: the
// value of an assignment, the default of a parameter, and an argument of a
// call, which a decorator takes. Each places the text where Python reads
// the construct, so the text parses where it is one construct of its kind.
const (
	valueOpen    = "_ = "
	valueClose   = "\n"
	defaultOpen  = "def _(_="
	defaultClose = "): ...\n"
	argumentOpen = "_("
	argumentEnd  = ")\n"
)

// textKind is the construct that a text of a declaration is.
type textKind uint8

const (
	// valueText is the value of an assignment: an expression or a tuple of
	// expressions.
	valueText textKind = iota
	// defaultText is the default of a parameter: one expression.
	defaultText
	// argumentText is an argument of a call: one expression, a keyword
	// argument or an unpacking.
	argumentText
)

// texts is the check of the texts of declarations that [Lower] runs. The
// package resolves its node kinds once, when it is initialised.
var texts = newExpressions()

// expressions checks that the texts of declarations parse as Python. It
// resolves the node kinds of the grammar once.
//
// # Concurrency
//
// An expressions is safe for concurrent use, because each check parses a
// tree of its own.
type expressions struct {
	grammar *treesitter.Grammar
	// The kinds and fields of the nodes that a check reads.
	assignment, functionDefinition, call, listSplat, tuple  treesitter.Kind
	fieldParameters, fieldArguments, fieldRight, fieldValue treesitter.Field
}

// bareTuple is the number of the children of a tuple of one element without
// a comma, as (*a) is: its parentheses and its element.
const bareTuple = 3

// newExpressions resolves the kinds and the fields that the checks read in
// the grammar of the fork of tree-sitter-python. It panics on a kind or a
// field that the grammar does not declare, through [treesitter.Grammar.Kind]
// and [treesitter.Grammar.Field]. The grammar is embedded in the module, so
// the initialisation of the package reaches that panic in every test or
// none.
func newExpressions() expressions {
	g := pygrammar.Grammar
	return expressions{
		grammar:            g,
		assignment:         g.Kind("assignment"),
		functionDefinition: g.Kind("function_definition"),
		call:               g.Kind("call"),
		listSplat:          g.Kind("list_splat"),
		tuple:              g.Kind("tuple"),
		fieldParameters:    g.Field("parameters"),
		fieldArguments:     g.Field("arguments"),
		fieldRight:         g.Field("right"),
		fieldValue:         g.Field("value"),
	}
}

// parses reports whether text is one construct of the kind k: the value of
// an assignment, the default of a parameter, or an argument of a call. It
// parses the text in the source of its construct, and requires a tree
// without an error of one statement whose construct is the text alone. A
// value is no assignment of its own, because an annotated assignment takes
// one target, and neither a value nor a default is a starred expression,
// which CPython unpacks only into a tuple, a list, a set or a call. The
// grammar declares the expression statement as a supertype, so the
// assignment and the call of a check source are children of the module.
// The lowering of the kit runs without a context, and a parse ends quickly,
// so the check parses under the background context, and a text that the
// grammar does not parse is no construct.
//
// # Allocation contract
//
// parses allocates the source and the tree of the parse.
func (x expressions) parses(text string, k textKind) bool {
	var src string
	switch k {
	case valueText:
		src = valueOpen + text + valueClose
	case defaultText:
		src = defaultOpen + text + defaultClose
	case argumentText:
		src = argumentOpen + text + argumentEnd
	}
	tree, err := x.grammar.Parse(context.Background(), "", []byte(src))
	if err == nil {
		defer tree.Close()
	}
	return err == nil && x.construct(tree, k)
}

// construct reports whether the tree of a check source is one statement
// without an error whose construct is the text of the kind k alone, by the
// rules of [expressions.parses].
func (x expressions) construct(tree *treesitter.Tree, k textKind) bool {
	for range tree.Errors() {
		return false
	}
	stmt, one := only(tree.Root())
	if !one {
		return false
	}
	switch k {
	case valueText:
		right := stmt.Child(x.fieldRight)
		return stmt.Kind() == x.assignment && !right.IsZero() && right.Kind() != x.assignment && !x.starred(right)
	case defaultText:
		param, single := only(stmt.Child(x.fieldParameters))
		return stmt.Kind() == x.functionDefinition && single && !x.starred(param.Child(x.fieldValue))
	default:
		_, argument := only(stmt.Child(x.fieldArguments))
		return stmt.Kind() == x.call && argument
	}
}

// starred reports whether n is a starred expression, bare or alone in
// parentheses. The grammar parses (*a) as a tuple without a comma, which
// CPython refuses, and (*a,) as a tuple with one.
func (x expressions) starred(n treesitter.Node) bool {
	if n.Kind() == x.tuple {
		children := 0
		for range n.AllChildren() {
			children++
		}
		if children != bareTuple {
			return false
		}
		n, _ = only(n)
	}
	return n.Kind() == x.listSplat
}

// only returns the one named child of n that is no extra, such as a
// comment, and reports false where n has none or more than one.
func only(n treesitter.Node) (treesitter.Node, bool) {
	var found treesitter.Node
	count := 0
	for c := range n.Content() {
		found = c
		count++
	}
	return found, count == 1
}
