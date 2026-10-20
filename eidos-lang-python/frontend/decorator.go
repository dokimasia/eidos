// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The texts that a marker of the brand lifts a literal to: the two booleans
// and the minus sign of a negative number. floatBits is the width at which
// a float lifts, because a Python float is a double.
const (
	trueText  = "true"
	falseText = "false"
	minusSign = "-"
	floatBits = 64
)

// decorator is one decorator of a definition: its node, its annotation, and
// the module and the name that its callee means in the module. dotted
// reports a decorator that is a dotted name or a call of one, and a
// decorator of any other expression has no callee.
type decorator struct {
	node       treesitter.Node
	annotation symbol.Annotation
	module     string
	name       string
	dotted     bool
}

// decoratedBy reports whether one of the decorators means the name in the
// module.
func decoratedBy(decorators []decorator, module, name string) bool {
	for _, d := range decorators {
		if d.module == module && d.name == name {
			return true
		}
	}
	return false
}

// decoratorsOf lowers the decorators of a decorated definition in source
// order. The name of the annotation of a decorator is its dotted name, or
// the dotted name of the callee of a call, and its arguments are the
// arguments of the call as written, one for each argument. A decorator of
// any other expression is an annotation of the whole expression, without
// arguments.
func (l *lowering) decoratorsOf(decorated treesitter.Node) []decorator {
	var out []decorator
	for d := range decorated.NamedChildren() {
		if d.Kind() != l.v.decorator {
			continue
		}
		expr := firstNamed(d)
		callee, args := expr, treesitter.Node{}
		if expr.Kind() == l.v.call {
			callee, args = expr.Child(l.v.fieldFunction), expr.Child(l.v.fieldArguments)
		}
		dec := decorator{node: d, dotted: l.dottedName(callee)}
		if dec.dotted {
			dec.annotation.Name = callee.Compact()
			dec.module, dec.name = l.scope.canonical(dec.annotation.Name)
			for a := range args.Content() {
				dec.annotation.Args = append(dec.annotation.Args, a.Text())
			}
		} else {
			dec.annotation.Name = expr.Compact()
		}
		out = append(out, dec)
	}
	return out
}

// decorate returns the annotations of decorators on a subject in source
// order. A decorator whose dotted name starts with the brand of the unit
// also attaches its directive through [plugin.SourceUnit.AttachSugar], which
// reports a decorator that is no valid directive under [BadMarker]. A
// decorator of the brand is an annotation as well.
func (l *lowering) decorate(subject symbol.Symbol, decorators []decorator) symbol.Annotations {
	if len(decorators) == 0 {
		return nil
	}
	out := make(symbol.Annotations, 0, len(decorators))
	for _, d := range decorators {
		out = append(out, d.annotation)
		if marker, named := l.marker(d); named {
			l.u.AttachSugar(subject, marker, BadMarker)
		}
	}
	return out
}

// refuseMarkers reports each decorator of the brand among decorators under
// [UnaddressedCarrier], because the decorators are on a subject that the
// model cannot address. what describes the subject.
func (l *lowering) refuseMarkers(decorators []decorator, what string) {
	var markers []plugin.Sugar
	for _, d := range decorators {
		if marker, named := l.marker(d); named {
			markers = append(markers, marker)
		}
	}
	l.u.RefuseSugar(UnaddressedCarrier, markers, python.ModuleSep, what)
}

// marker returns the marker of a decorator whose dotted name starts with
// the brand of the unit. The path of the marker is the names of the dotted
// name, and its arguments are the arguments of a call, lifted from the
// syntax: a positional argument as a positional argument, and a keyword
// argument as a keyed one. The Refusal of the marker describes the first
// argument that does not lift. marker reports false for a decorator that
// is outside the brand.
func (l *lowering) marker(d decorator) (plugin.Sugar, bool) {
	head, _, _ := strings.Cut(d.annotation.Name, python.ModuleSep)
	if !d.dotted || head != l.u.Brand() {
		return plugin.Sugar{}, false
	}
	s := plugin.Sugar{Path: strings.Split(d.annotation.Name, python.ModuleSep), Pos: d.node.Pos()}
	expr := firstNamed(d.node)
	if expr.Kind() != l.v.call {
		return s, true
	}
	for arg := range expr.Child(l.v.fieldArguments).Content() {
		var raw directive.RawArg
		if arg.Kind() == l.v.keywordArgument {
			raw.Key = arg.Child(l.v.fieldName).Text()
			arg = arg.Child(l.v.fieldValue)
		}
		raw.Value, s.Refusal = l.literal(arg)
		if s.Refusal != "" {
			break
		}
		s.Args = append(s.Args, raw)
	}
	return s, true
}

// literal lifts one argument of a marker into a directive value:
//
//   - a string literal, and a concatenation of them, to its value;
//   - an integer to its decimal text, and a float to the shortest decimal
//     text that parses back to its double, with a minus sign where the
//     source has one;
//   - True and False to true and false;
//   - a list or a tuple to the list of its elements;
//   - a literal in parentheses to the literal.
//
// literal returns the reason that an argument does not lift, or the empty
// string where it lifts.
func (l *lowering) literal(n treesitter.Node) (directive.RawValue, string) {
	switch n.Kind() {
	case l.v.stringNode, l.v.concatenatedString:
		if text, is := l.stringValue(n); is {
			return directive.RawValue{Text: text, Quoted: true}, ""
		}
	case l.v.integer:
		if text, is := python.ParseInt(n.Text()); is {
			return directive.RawValue{Text: text}, ""
		}
	case l.v.float:
		if f, is := python.ParseFloat(n.Text()); is {
			return directive.RawValue{Text: numeric.Decimal(f, floatBits)}, ""
		}
	case l.v.unaryOperator:
		operand := n.Child(l.v.fieldArgument)
		number := operand.Kind() == l.v.integer || operand.Kind() == l.v.float
		if number && n.Child(l.v.fieldOperator).Text() == minusSign {
			v, refusal := l.literal(operand)
			return directive.RawValue{Text: minusSign + v.Text}, refusal
		}
	case l.v.parenthesizedExpression:
		return l.literal(firstNamed(n))
	case l.v.trueNode:
		return directive.RawValue{Text: trueText}, ""
	case l.v.falseNode:
		return directive.RawValue{Text: falseText}, ""
	case l.v.list, l.v.tuple:
		// A list value is a list also when it has no element, so the list
		// is never nil.
		list := []directive.RawValue{}
		for e := range n.Content() {
			v, refusal := l.literal(e)
			if refusal != "" {
				return directive.RawValue{}, refusal
			}
			list = append(list, v)
		}
		return directive.RawValue{List: list}, ""
	}
	return directive.RawValue{}, fmt.Sprintf(
		"the argument %s is not a string, a number, a boolean, or a list or a tuple of them", n.Compact())
}

// stringValue returns the value of a string literal or of a concatenation
// of them, and reports false for an f-string, bytes, a literal that
// Unquote does not decode and any other node.
func (l *lowering) stringValue(n treesitter.Node) (string, bool) {
	switch n.Kind() {
	case l.v.stringNode:
		return python.Unquote(n.Text())
	case l.v.concatenatedString:
	default:
		return "", false
	}
	var b strings.Builder
	for part := range n.Content() {
		text, is := python.Unquote(part.Text())
		if !is {
			return "", false
		}
		b.WriteString(text)
	}
	return b.String(), true
}

// dottedName reports whether a node is a name or a dotted name: an
// identifier, or an attribute of a dotted name.
func (l *lowering) dottedName(n treesitter.Node) bool {
	for n.Kind() == l.v.attribute {
		n = n.Child(l.v.fieldObject)
	}
	return n.Kind() == l.v.identifier
}
