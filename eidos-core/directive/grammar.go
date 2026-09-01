// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/core/position"
)

// Name is a directive's spelling: bare while one plugin claims it,
// plugin-prefixed as "plugin:name" always.
//
// A Name compares by its bytes, so the store's directive index and
// the registry's lookups key on it directly. The zero Name spells
// nothing; [Parse] never returns one, because the grammar requires
// a name before the first argument.
type Name string

// The grammar's punctuation. The spellings live here once, so the
// parser and its refusals agree on every byte.
const (
	prefixSep    = ':'
	keySep       = '='
	listOpen     = '['
	listClose    = ']'
	listSep      = ','
	quote        = '"'
	escape       = '\\'
	continuation = `\`
	joinSep      = " "
)

// Raw is one instance as a carrier hands it over: the grammar
// parsed, the values untyped.
//
// Raw exists because parsing and typing have two callers with two
// failure audiences: a parse error is the author's typo, found
// where the carrier is read, and a type error is a schema
// violation, found at validation with the registry in hand. A
// carrier therefore produces Raw without holding a registry, and
// [Validate] turns Raw into the typed [Directive] handlers receive.
//
// Raw is a plain value: copy it freely. The store sorts a
// subject's instances by Pos at its seal, so the field is
// load-bearing for determinism, not decoration.
type Raw struct {
	// Name is the spelling as written, bare or prefixed.
	Name Name
	// Args holds every argument in source order, positional and
	// keyed alike, so a diagnostic about one argument can point at
	// it.
	Args []RawArg
	// Pos is the carrier line, filled by the carrier's owner.
	Pos position.Pos
}

// RawArg is one argument as written, keyed or positional, in the
// order the author wrote it. Keeping both forms in one ordered
// slice is what lets a diagnostic point at any argument and lets
// validation detect a key written twice with both offsets in hand.
type RawArg struct {
	// Key is empty for a positional argument.
	Key string
	// Value is the spelling, quoting resolved.
	Value RawValue
	// Col is the argument's byte offset within the payload, the
	// same unit parse errors carry; the carrier's owner converts
	// to a file position. On a joined continuation the offset
	// addresses the joined payload, so a diagnostic there points
	// at the instance's first line.
	Col int
}

// RawValue is one value as written: a scalar spelling, or a list.
// Exactly one form is populated: List is nil for a scalar, and
// Text is empty for a list. A nested list parses — the grammar
// admits it — and validation refuses it against every schema.
type RawValue struct {
	// Text is the scalar spelling with escapes resolved; empty for
	// a list.
	Text string
	// Quoted says the spelling was quoted, which is what lets an
	// empty string be a value.
	Quoted bool
	// List holds the elements of a list value, nil for a scalar.
	List []RawValue
}

// Join folds continued carrier lines into one payload: a line
// ending in a backslash joins the next, marker already stripped,
// with a single space. The rule is grammar, so it lives here once
// rather than in every frontend.
func Join(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed, continued := strings.CutSuffix(line, continuation)
		if continued {
			// Whatever sat before the marker, the join is exactly
			// one space.
			trimmed = strings.TrimRight(trimmed, " \t")
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, joinSep)
}

// Parse reads one payload against the pinned grammar. The payload
// is the text after the carrier marker, one logical line with
// continuations already joined.
//
// A payload outside the grammar returns an error carrying the byte
// offset where reading stopped; the caller owns the file position
// and converts. Parse never panics, whatever the bytes.
func Parse(payload string) (Raw, error) {
	p := &parser{payload: payload}
	p.skipSpace()

	name, err := p.name()
	if err != nil {
		return Raw{}, err
	}
	raw := Raw{Name: name}

	for {
		p.skipSpace()
		if p.done() {
			return raw, nil
		}
		arg, err := p.arg()
		if err != nil {
			return Raw{}, err
		}
		raw.Args = append(raw.Args, arg)
	}
}

// parser reads one payload left to right. at is the next unread
// byte, and every refusal names it.
type parser struct {
	payload string
	at      int
}

// done reports whether the payload is fully read.
func (p *parser) done() bool { return p.at >= len(p.payload) }

// peek returns the next byte without reading it; zero at the end.
func (p *parser) peek() byte {
	if p.done() {
		return 0
	}
	return p.payload[p.at]
}

// skipSpace reads past spaces and tabs.
func (p *parser) skipSpace() {
	for !p.done() && (p.peek() == ' ' || p.peek() == '\t') {
		p.at++
	}
}

// fail returns a refusal naming the offset where reading stopped.
func (p *parser) fail(format string, args ...any) error {
	return fmt.Errorf("directive: offset %d: %s", p.at, fmt.Sprintf(format, args...))
}

// name reads the directive name: an identifier, optionally
// prefixed by another and a colon.
func (p *parser) name() (Name, error) {
	first, err := p.ident("a directive name")
	if err != nil {
		return "", err
	}
	if p.peek() != prefixSep {
		return Name(first), nil
	}
	p.at++
	second, err := p.ident("a directive name after its plugin prefix")
	if err != nil {
		return "", err
	}
	return Name(first + string(prefixSep) + second), nil
}

// ident reads one identifier: a letter, then letters, digits,
// hyphens and underscores.
func (p *parser) ident(what string) (string, error) {
	start := p.at
	if p.done() || !isLetter(p.peek()) {
		return "", p.fail("%s starts with a letter", what)
	}
	for !p.done() && isIdent(p.peek()) {
		p.at++
	}
	return p.payload[start:p.at], nil
}

// arg reads one argument: a key=value pair, or a positional value.
func (p *parser) arg() (RawArg, error) {
	col := p.at

	// A keyed argument starts with an identifier followed by '='.
	// A bare value may also start with letters, so read ahead and
	// decide at the separator: bare values cannot contain '='.
	if isLetter(p.peek()) {
		mark := p.at
		key, err := p.ident("a key")
		if err != nil {
			return RawArg{}, err
		}
		if p.peek() == keySep {
			p.at++
			value, err := p.value()
			if err != nil {
				return RawArg{}, err
			}
			return RawArg{Key: key, Value: value, Col: col}, nil
		}
		p.at = mark
	}
	if p.peek() == keySep {
		return RawArg{}, p.fail("an argument does not start with %q", string(keySep))
	}

	value, err := p.value()
	if err != nil {
		return RawArg{}, err
	}
	return RawArg{Value: value, Col: col}, nil
}

// value reads one value: a list, a quoted string, or a bare
// spelling.
func (p *parser) value() (RawValue, error) {
	switch p.peek() {
	case listOpen:
		return p.list()
	case quote:
		return p.quoted()
	default:
		return p.bare()
	}
}

// list reads a bracketed, comma-separated value list.
func (p *parser) list() (RawValue, error) {
	p.at++ // consume '['
	out := RawValue{List: []RawValue{}}

	p.skipSpace()
	if p.peek() == listClose {
		p.at++
		return out, nil
	}
	for {
		element, err := p.value()
		if err != nil {
			return RawValue{}, err
		}
		out.List = append(out.List, element)

		switch p.peek() {
		case listSep:
			p.at++
			p.skipSpace()
		case listClose:
			p.at++
			return out, nil
		default:
			return RawValue{}, p.fail("a list element ends with %q or %q",
				string(listSep), string(listClose))
		}
	}
}

// quoted reads a double-quoted string, resolving the four escapes.
func (p *parser) quoted() (RawValue, error) {
	p.at++ // consume the opening quote
	var out strings.Builder
	for {
		if p.done() {
			return RawValue{}, p.fail("a quoted value is unterminated")
		}
		c := p.peek()
		p.at++
		switch c {
		case quote:
			return RawValue{Text: out.String(), Quoted: true}, nil
		case escape:
			if p.done() {
				return RawValue{}, p.fail("a quoted value is unterminated")
			}
			e := p.peek()
			p.at++
			switch e {
			case quote, escape:
				out.WriteByte(e)
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			default:
				p.at -= 2
				return RawValue{}, p.fail(
					`an escape is one of \" \\ \n \t, not \%s`, string(e),
				)
			}
		default:
			out.WriteByte(c)
		}
	}
}

// bare reads an unquoted spelling: everything up to whitespace or
// grammar punctuation.
func (p *parser) bare() (RawValue, error) {
	start := p.at
	for !p.done() && isBare(p.peek()) {
		p.at++
	}
	if p.at == start {
		return RawValue{}, p.fail("a value is a list, a quoted string or a bare spelling")
	}
	return RawValue{Text: p.payload[start:p.at]}, nil
}

// isLetter reports an ASCII letter.
func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isIdent reports a byte an identifier may continue with.
func isIdent(c byte) bool {
	return isLetter(c) || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// isBare reports a byte a bare value may carry: anything but
// whitespace and the grammar's punctuation.
func isBare(c byte) bool {
	switch c {
	case ' ', '\t', quote, listOpen, listClose, listSep, keySep:
		return false
	}
	return true
}
