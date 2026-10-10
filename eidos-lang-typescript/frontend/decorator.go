// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/treesitter"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// numberBits is the width at which a number argument of a decorator
// lifts. A TypeScript number is a double.
const numberBits = 64

// minusSign is the only unary operator that a number argument of a
// decorator can have.
const minusSign = "-"

// markerSeparator separates the names of a decorator's path in a finding.
const markerSeparator = "."

// decoratorsOf returns the decorators among the children of a node, in
// source order.
func (l *lowering) decoratorsOf(n treesitter.Node) []treesitter.Node {
	var out []treesitter.Node
	for child := range n.NamedChildren() {
		if child.Kind() == l.v.decorator {
			out = append(out, child)
		}
	}
	return out
}

// decorate lowers decorators onto a subject in source order, and returns
// them as the subject's annotations. A decorator whose path starts with
// the unit's brand also attaches its directive through
// [plugin.SourceUnit.AttachSugar]. AttachSugar reports such a decorator
// under [BadMarker] when its path is not a directive name or when an
// argument does not lift. A decorator of the brand is an annotation as
// well.
func (l *lowering) decorate(subject symbol.Symbol, decorators []treesitter.Node) symbol.Annotations {
	var out symbol.Annotations
	for _, d := range decorators {
		out = append(out, l.decorator(d))
		if marker, named := l.marker(d); named {
			l.u.AttachSugar(subject, marker, BadMarker)
		}
	}
	return out
}

// refuseMarkers reports each decorator of the brand among decorators under
// [UnaddressedCarrier], because they decorate a subject that the model
// cannot address. what describes the subject.
func (l *lowering) refuseMarkers(decorators []treesitter.Node, what string) {
	for _, d := range decorators {
		if marker, named := l.marker(d); named {
			l.u.Errorf(UnaddressedCarrier, marker.Pos,
				"the marker %s is on %s, which the model cannot address. Move it to a declaration",
				strings.Join(marker.Path, markerSeparator), what)
		}
	}
}

// decorator lowers one decorator to an annotation. The annotation's name
// is the decorator's expression without the @, or the callee of a call.
// The annotation's arguments are the call's arguments verbatim, one per
// argument.
func (l *lowering) decorator(d treesitter.Node) symbol.Annotation {
	expr := l.lastNamed(d)
	if expr.Kind() != l.v.callExpression {
		return symbol.Annotation{Name: expr.Compact()}
	}
	a := symbol.Annotation{Name: expr.Child(l.v.fieldFunction).Compact()}
	for arg := range expr.Child(l.v.fieldArguments).NamedChildren() {
		if arg.Kind() != l.v.comment {
			a.Args = append(a.Args, arg.Text())
		}
	}
	return a
}

// marker returns the marker of a decorator. The marker's path is the
// list of names in the decorator's dotted path, and its arguments are a
// call's arguments, lifted from the syntax. A string, a number with at
// most one minus sign, true, false and an array of them lift as
// positional arguments. An object literal in the last position lifts its
// keys and values as keyed arguments. The marker's Refusal describes the
// first argument that does not lift. marker reports false for a
// decorator whose expression is not a path of names, such as a
// parenthesized expression, and for a path that does not start with the
// unit's brand, which is ordinary metadata.
func (l *lowering) marker(d treesitter.Node) (plugin.Sugar, bool) {
	callee, args := l.lastNamed(d), treesitter.Node{}
	if callee.Kind() == l.v.callExpression {
		callee, args = callee.Child(l.v.fieldFunction), callee.Child(l.v.fieldArguments)
	}
	head := callee
	for head.Kind() == l.v.memberExpression {
		head = head.Child(l.v.fieldObject)
	}
	if head.Kind() != l.v.identifier || head.Text() != l.u.Brand() {
		return plugin.Sugar{}, false
	}
	var path []string
	for callee.Kind() == l.v.memberExpression {
		path = append(path, callee.Child(l.v.fieldProperty).Text())
		callee = callee.Child(l.v.fieldObject)
	}
	path = append(path, callee.Text())
	slices.Reverse(path)
	s := plugin.Sugar{Path: path, Pos: d.Pos()}
	var list []treesitter.Node
	for arg := range args.NamedChildren() {
		if arg.Kind() != l.v.comment {
			list = append(list, arg)
		}
	}
	for i, arg := range list {
		if arg.Kind() == l.v.object && i == len(list)-1 {
			s.Refusal = l.keyed(arg, &s.Args)
		} else {
			var v directive.RawValue
			v, s.Refusal = l.literal(arg)
			s.Args = append(s.Args, directive.RawArg{Value: v})
		}
		if s.Refusal != "" {
			break
		}
	}
	return s, true
}

// keyed lifts the members of an object literal into keyed arguments. A
// member's key is a name or a string. keyed returns the reason that a
// member does not lift, or the empty string when every member lifts.
func (l *lowering) keyed(object treesitter.Node, args *[]directive.RawArg) string {
	for member := range object.NamedChildren() {
		switch member.Kind() {
		case l.v.comment:
			continue
		case l.v.pair:
		default:
			return fmt.Sprintf("the member %s of the object is not a key with a value", member.Compact())
		}
		key := member.Child(l.v.fieldKey)
		name, named := key.Text(), key.Kind() == l.v.propertyIdentifier
		if key.Kind() == l.v.stringNode {
			name, named = typescript.Unquote(key.Text())
		}
		if !named {
			return fmt.Sprintf("the key %s is neither a name nor a string", key.Compact())
		}
		v, refusal := l.literal(member.Child(l.v.fieldValue))
		if refusal != "" {
			return refusal
		}
		*args = append(*args, directive.RawArg{Key: name, Value: v})
	}
	return ""
}

// literal lifts one argument into a directive value:
//
//   - a string literal, and a template literal without a substitution,
//     to its content;
//   - a number to the shortest decimal text that parses back to its
//     double, with a minus sign where the source has one;
//   - true and false as written;
//   - an array to the list of its elements.
//
// literal returns the reason that an argument does not lift, or the
// empty string when the argument lifts.
func (l *lowering) literal(n treesitter.Node) (directive.RawValue, string) {
	switch n.Kind() {
	case l.v.stringNode, l.v.templateString:
		if s, ok := typescript.Unquote(n.Text()); ok {
			return directive.RawValue{Text: s, Quoted: true}, ""
		}
	case l.v.number:
		if f, ok := typescript.ParseNumber(n.Text()); ok {
			return directive.RawValue{Text: numeric.Decimal(f, numberBits)}, ""
		}
	case l.v.unaryExpression:
		operand := n.Child(l.v.fieldArgument)
		f, ok := typescript.ParseNumber(operand.Text())
		if n.Child(l.v.fieldOperator).Text() == minusSign && operand.Kind() == l.v.number && ok {
			return directive.RawValue{Text: minusSign + numeric.Decimal(f, numberBits)}, ""
		}
	case l.v.trueNode, l.v.falseNode:
		return directive.RawValue{Text: n.Text()}, ""
	case l.v.array:
		// A list value is a list also when it has no element, so the list
		// is never nil.
		list := []directive.RawValue{}
		for e := range n.NamedChildren() {
			if e.Kind() == l.v.comment {
				continue
			}
			v, refusal := l.literal(e)
			if refusal != "" {
				return directive.RawValue{}, refusal
			}
			list = append(list, v)
		}
		return directive.RawValue{List: list}, ""
	}
	return directive.RawValue{}, fmt.Sprintf(
		"the argument %s is not a string, a number, a boolean or an array of them", n.Compact())
}
