// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The attributes the lowering reads beside recording them: the
// documentation attribute, the module path attribute, the test marker,
// and conditional compilation.
const (
	attrDoc  = "doc"
	attrPath = "path"
	attrTest = "test"
	attrCfg  = "cfg"
)

// The parts of a cfg predicate: the combinators, the feature key, the
// separators of a predicate list and of a key's value, and the test
// option, which every load sets.
const (
	cfgAll       = "all"
	cfgAny       = "any"
	cfgNot       = "not"
	cfgFeature   = "feature"
	cfgSeparator = ","
	cfgAssign    = "="
	cfgTest      = "test"
)

// attributes is what an item's outer attributes, or a module's inner
// ones, state: the annotations they lower to, the documentation #[doc]
// attributes add, a module's path attribute, whether #[test] or
// #[cfg(test)] marks the item, and the first cfg predicate the load's
// set does not satisfy, empty where it satisfies every one.
type attributes struct {
	annotations symbol.Annotations
	docs        []string
	path        string
	test        bool
	excluded    string
}

// attributes reads an item's outer attributes, or a module's inner
// ones. Each is an annotation of its name and its arguments verbatim,
// except a #[doc] attribute that assigns a string literal, whose text
// is documentation, one line per line of the text. A #[cfg] attribute
// is evaluated as well.
func (l *lowering) attributes(items []treesitter.Node) attributes {
	var a attributes
	for _, item := range items {
		name, args, value := l.attributeParts(l.firstOf(item, l.v.attribute))
		switch name {
		case attrDoc:
			if text, ok := l.literal(value); ok {
				a.docs = append(a.docs, strings.Split(text, "\n")...)
				continue
			}
		case attrPath:
			a.path, _ = l.literal(value)
		case attrTest:
			a.test = true
		case attrCfg:
			preds := l.predicates(args)
			if len(preds) == 1 && len(preds[0]) == 1 && preds[0][0].Text() == cfgTest {
				a.test = true
			}
			if a.excluded == "" && (len(preds) != 1 || !l.satisfied(preds[0])) {
				a.excluded = l.contents(args)
			}
		}
		annotation := symbol.Annotation{Name: name}
		switch {
		case !value.IsZero():
			annotation.Args = []string{value.Text()}
		case !args.IsZero():
			for _, arg := range l.predicates(args) {
				annotation.Args = append(annotation.Args, arg[0].TextThrough(arg[len(arg)-1]))
			}
		}
		a.annotations = append(a.annotations, annotation)
	}
	return a
}

// literal returns the value a string literal states: the literal
// unquoted where its escapes are also Go's, and its content as written
// otherwise, as a raw string's is. It reports false for any other
// expression, such as a macro invocation.
func (l *lowering) literal(n treesitter.Node) (string, bool) {
	if text, err := strconv.Unquote(n.Text()); err == nil {
		return text, true
	}
	if k := n.Kind(); k != l.v.stringLiteral && k != l.v.rawStringLiteral {
		return "", false
	}
	var b strings.Builder
	for part := range n.NamedChildren() {
		b.WriteString(part.Text())
	}
	return b.String(), true
}

// attributeParts returns an attribute's name as written, its argument
// token tree, and the value it assigns, each zero where it states none.
func (l *lowering) attributeParts(attr treesitter.Node) (name string, args, value treesitter.Node) {
	for child := range attr.NamedChildren() {
		if child.Kind() == l.v.identifier || child.Kind() == l.v.scopedIdentifier {
			name = child.Compact()
			break
		}
	}
	return name, attr.Child(l.v.fieldArguments), attr.Child(l.v.fieldValue)
}

// predicates splits a token tree's contents at its top-level commas:
// one node list per argument, without the delimiters, the comments and
// the empty argument a trailing comma leaves.
func (*lowering) predicates(tt treesitter.Node) [][]treesitter.Node {
	var tokens []treesitter.Node
	for child := range tt.AllChildren() {
		if !child.IsExtra() {
			tokens = append(tokens, child)
		}
	}
	if len(tokens) < 2 {
		return nil
	}
	var out [][]treesitter.Node
	var current []treesitter.Node
	for _, token := range tokens[1 : len(tokens)-1] {
		if token.Named() || token.Text() != cfgSeparator {
			current = append(current, token)
			continue
		}
		if len(current) > 0 {
			out = append(out, current)
		}
		current = nil
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// contents returns a token tree's contents as written, between its
// delimiters.
func (l *lowering) contents(tt treesitter.Node) string {
	preds := l.predicates(tt)
	if len(preds) == 0 {
		return ""
	}
	last := preds[len(preds)-1]
	return preds[0][0].TextThrough(last[len(last)-1])
}

// satisfied evaluates one cfg predicate against the load's options: all,
// any and not over their predicates, feature = "x" where the load
// enables the feature x, any other key = "value" and any other option
// where the load sets it as written, and test always, so a test item
// loads and is marked. A predicate of any other shape is false.
func (l *lowering) satisfied(pred []treesitter.Node) bool {
	name := pred[0].Text()
	switch {
	case len(pred) == 2 && pred[1].Kind() == l.v.tokenTree:
		args := l.predicates(pred[1])
		switch name {
		case cfgAll:
			return !slices.ContainsFunc(args, func(arg []treesitter.Node) bool { return !l.satisfied(arg) })
		case cfgAny:
			return slices.ContainsFunc(args, l.satisfied)
		case cfgNot:
			return len(args) == 1 && !l.satisfied(args[0])
		}
		return false
	case len(pred) == 3 && pred[1].Text() == cfgAssign:
		value := pred[2].Text()
		if name == cfgFeature {
			feature, err := strconv.Unquote(value)
			return err == nil && slices.Contains(l.w.opts.Features, feature)
		}
		return slices.Contains(l.w.opts.Cfg, name+cfgAssign+value)
	case len(pred) == 1:
		return name == cfgTest || slices.Contains(l.w.opts.Cfg, name)
	}
	return false
}
