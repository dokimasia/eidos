// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The attributes that the lowering reads beside recording them: the
// documentation attribute, the module path attribute, the test marker,
// conditional compilation, and the representation of an enum.
const (
	attrDoc  = "doc"
	attrPath = "path"
	attrTest = "test"
	attrCfg  = "cfg"
	attrRepr = "repr"
)

// The parts of a cfg predicate: the combinators, the feature key, the
// separators of a predicate list and of the value of a key, and the test
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

// The parts of a Rust numeric literal that the argument of a marker lifts:
// the minus sign before it, the separator between its digits, the zero
// that opens the prefix of another radix, the bit that an ASCII letter sets
// in lower case, the letters of the radix prefixes, and the width of the
// value.
const (
	minusSign  = "-"
	separator  = "_"
	zeroDigit  = '0'
	caseBit    = 0x20
	hexRadix   = 'x'
	octalRadix = 'o'
	binRadix   = 'b'
	numberBits = 64
)

// The type suffixes of the integer and the float literals of Rust.
var (
	integerSuffixes = []string{
		"u8", "u16", "u32", "u64", "u128", "usize", "i8", "i16", "i32", "i64", "i128", "isize",
	}
	floatSuffixes = []string{"f32", "f64"}
)

// attributes is what the outer attributes of an item, or the inner
// attributes of a module, contain. Its fields are:
//
//   - the annotations that the attributes lower to;
//   - the documentation that #[doc] attributes add;
//   - the path attribute of a module;
//   - whether #[test] or #[cfg(test)] marks the item;
//   - the first cfg predicate that the set of the load does not satisfy,
//     and the empty predicate where the set satisfies them all;
//   - the markers of the brand among the attributes.
type attributes struct {
	annotations symbol.Annotations
	docs        []string
	path        string
	test        bool
	excluded    string
	sugars      []plugin.Sugar
}

// attributes reads the outer attributes of an item, or the inner
// attributes of a module. Each attribute is an annotation of its name and
// its arguments verbatim, and an attribute that assigns its value, as
// #[must_use = "use it"] does, is an assigned annotation of the value. A
// #[doc] attribute that assigns a string literal
// is documentation instead, one line per line of the text. The lowering
// also evaluates a #[cfg] attribute. An attribute whose path starts with
// the brand of the unit is also the marker of a directive, which
// [lowering.marker] lifts.
func (l *lowering) attributes(items []treesitter.Node) attributes {
	var a attributes
	for _, item := range items {
		name, args, value := l.attributeParts(item.FirstOf(l.v.attribute))
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
		if head, _, _ := strings.Cut(name, pathSeparator); head == l.w.u.Brand() {
			a.sugars = append(a.sugars, l.marker(item, name, args, value))
		}
		annotation := symbol.Annotation{Name: name}
		switch {
		case !value.IsZero():
			annotation.Args, annotation.Assigned = []string{value.Text()}, true
		case !args.IsZero():
			for _, arg := range l.predicates(args) {
				annotation.Args = append(annotation.Args, arg[0].TextThrough(arg[len(arg)-1]))
			}
		}
		a.annotations = append(a.annotations, annotation)
	}
	return a
}

// marker returns the marker of an attribute of the brand. Its path is the
// path of the attribute split at ::. Its arguments are the arguments of
// the token tree of the attribute, which [lowering.argument] lifts from the
// syntax. An argument key = value lifts as a keyed argument, and any other
// argument as a positional one. The Refusal of the marker describes the
// first argument that does not lift, and an attribute that assigns a
// value, which is not the form of a marker.
func (l *lowering) marker(item treesitter.Node, name string, args, value treesitter.Node) plugin.Sugar {
	s := plugin.Sugar{Path: strings.Split(name, pathSeparator), Pos: item.Pos()}
	if !value.IsZero() {
		s.Refusal = fmt.Sprintf("the attribute assigns %s, and a marker takes its arguments in parentheses",
			value.Text())
		return s
	}
	for _, arg := range l.predicates(args) {
		key := ""
		if len(arg) > 2 && arg[0].Kind() == l.v.identifier && !arg[1].Named() && arg[1].Text() == cfgAssign {
			key, arg = arg[0].Text(), arg[2:]
		}
		v, refusal := l.argument(arg)
		if refusal != "" {
			s.Refusal = refusal
			return s
		}
		s.Args = append(s.Args, directive.RawArg{Key: key, Value: v})
	}
	return s
}

// argument lifts the tokens of one argument of a marker into a directive
// value:
//
//   - a string literal to its content, which [lowering.literal] reads;
//   - an integer or a float literal, with at most one minus sign, to the
//     decimal text that [number] writes;
//   - true, false and a name as written.
//
// argument returns the reason why an argument does not lift, and the empty
// string for an argument that lifts.
func (l *lowering) argument(tokens []treesitter.Node) (directive.RawValue, string) {
	sign, operand := "", tokens
	if len(tokens) == 2 && !tokens[0].Named() && tokens[0].Text() == minusSign {
		sign, operand = minusSign, tokens[1:]
	}
	if len(operand) == 1 {
		t := operand[0]
		switch k := t.Kind(); {
		case sign == "" && (k == l.v.stringLiteral || k == l.v.rawStringLiteral):
			if s, ok := l.literal(t); ok {
				return directive.RawValue{Text: s, Quoted: true}, ""
			}
		case k == l.v.integerLiteral || k == l.v.floatLiteral:
			if s, ok := number(t.Text(), k == l.v.floatLiteral); ok {
				return directive.RawValue{Text: sign + s}, ""
			}
		case sign == "" && (k == l.v.booleanLiteral || k == l.v.identifier):
			return directive.RawValue{Text: t.Text()}, ""
		}
	}
	return directive.RawValue{}, fmt.Sprintf("the argument %s is not a literal or a name",
		tokens[0].TextThrough(tokens[len(tokens)-1]))
}

// literal returns the value of a string literal: the literal unquoted where
// its escapes are also escapes of Go, and its content as written
// otherwise, as for a raw string. It reports false for any other
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
		b.Write(part.Bytes())
	}
	return b.String(), true
}

// attributeParts returns the name of an attribute as written, its argument
// token tree, and the value that it assigns, each zero where the attribute
// has none.
func (l *lowering) attributeParts(attr treesitter.Node) (name string, args, value treesitter.Node) {
	for child := range attr.NamedChildren() {
		if child.Kind() == l.v.identifier || child.Kind() == l.v.scopedIdentifier {
			name = child.Compact()
			break
		}
	}
	return name, attr.Child(l.v.fieldArguments), attr.Child(l.v.fieldValue)
}

// predicates splits the contents of a token tree at its top-level commas:
// one node list per argument, without the delimiters, the comments and the
// empty argument that a trailing comma leaves.
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

// contents returns the contents of a token tree as written, between its
// delimiters.
func (l *lowering) contents(tt treesitter.Node) string {
	preds := l.predicates(tt)
	if len(preds) == 0 {
		return ""
	}
	last := preds[len(preds)-1]
	return preds[0][0].TextThrough(last[len(last)-1])
}

// satisfied evaluates one cfg predicate against the options of the load:
// all, any and not over their predicates, feature = "x" where the load
// enables the feature x, any other key = "value" and any other option
// where the load sets it as written, and test always, so a test item loads
// and is marked. A predicate of any other shape is false.
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

// representation returns the integer type of the repr attribute among the
// annotations of an enum, such as the u8 of #[repr(u8)], at the position
// at. It also returns the annotations without that type. A repr attribute
// with other arguments, such as the C of #[repr(C, u8)], keeps them, and
// a repr attribute without other arguments leaves the annotations. The
// integer types of Rust are the names of [integerSuffixes].
// representation returns a nil type and the annotations unchanged where
// no repr attribute states an integer type.
func representation(annotations symbol.Annotations, at position.Pos) (*node.TypeRef, symbol.Annotations) {
	for i, an := range annotations {
		if an.Name != attrRepr {
			continue
		}
		j := slices.IndexFunc(an.Args, func(arg string) bool { return slices.Contains(integerSuffixes, arg) })
		if j < 0 {
			continue
		}
		underlying := &node.TypeRef{Spelling: an.Args[j], Pos: at}
		rest := slices.Delete(slices.Clone(an.Args), j, j+1)
		out := slices.Clone(annotations)
		if len(rest) == 0 {
			return underlying, slices.Delete(out, i, i+1)
		}
		out[i] = symbol.Annotation{Name: an.Name, Args: rest}
		return underlying, out
	}
	return nil, annotations
}

// number returns the decimal text of a Rust integer or float literal
// without its separators and its type suffix. number converts an integer
// of another radix, and keeps every digit of an integer that needs more
// than 64 bits. A float is the shortest decimal text that parses back to
// its double. number reports false for a float outside the range of a
// double.
func number(text string, float bool) (string, bool) {
	digits := strings.ReplaceAll(text, separator, "")
	if float {
		for _, suffix := range floatSuffixes {
			digits = strings.TrimSuffix(digits, suffix)
		}
		f, err := strconv.ParseFloat(digits, numberBits)
		if err != nil {
			return "", false
		}
		return numeric.Decimal(f, numberBits), true
	}
	for _, suffix := range integerSuffixes {
		if trimmed, cut := strings.CutSuffix(digits, suffix); cut {
			digits = trimmed
			break
		}
	}
	base := 10
	if len(digits) > 2 && digits[0] == zeroDigit {
		switch digits[1] | caseBit {
		case hexRadix:
			base, digits = 16, digits[2:]
		case octalRadix:
			base, digits = 8, digits[2:]
		case binRadix:
			base, digits = 2, digits[2:]
		}
	}
	if u, err := strconv.ParseUint(digits, base, numberBits); err == nil {
		return strconv.FormatUint(u, 10), true
	}
	n, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return "", false
	}
	return n.String(), true
}
