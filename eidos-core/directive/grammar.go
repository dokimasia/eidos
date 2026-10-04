// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/core/position"
)

// Name is a directive's spelling: bare while one plugin claims it,
// plugin-prefixed as "plugin:name" always.
//
// A Name compares by its bytes, so the store's directive index and
// the registry's lookups key on it directly. The zero Name spells
// nothing. [Parse] never returns one, because the grammar requires
// a name before the first argument.
type Name string

// Plugin returns the plugin prefix of a prefixed spelling, and the
// empty string for a bare one: a kernel name, or a plugin's name
// written without its prefix.
func (n Name) Plugin() string {
	plugin, _, prefixed := strings.Cut(string(n), string(prefixSep))
	if !prefixed {
		return ""
	}
	return plugin
}

// The grammar's punctuation. The spellings are defined here once,
// so the parser and its refusals agree on every byte.
const (
	prefixSep = ':'
	keySep    = '='
	listOpen  = '['
	listClose = ']'
	listSep   = ','
	quote     = '"'
	escape    = '\\'
	joinSep   = " "
)

// Continuation ends a carrier line whose payload continues on the
// next: the one spelling [Join] folds, exported so a comment split
// recognizes a continued carrier without respelling the grammar.
const Continuation = `\`

// Raw is one instance as a carrier hands it over: the grammar
// parsed, the values untyped.
//
// Raw exists because parsing and typing have two callers with two
// failure audiences: a parse error is the author's typo, found
// where the carrier is read, and a type error is a schema
// violation, found at validation with the registry in hand. A
// carrier therefore produces Raw without a registry, and [Validate]
// turns Raw into the typed [Directive] handlers receive.
//
// Raw is a plain value: copy it freely. The store sorts a
// subject's instances by Pos at its seal, so the determinism of
// every later pass depends on the field.
type Raw struct {
	// Name is the spelling as written, bare or prefixed.
	Name Name
	// Args contains every argument in source order, positional and
	// keyed alike, so a diagnostic about one argument can point at
	// it.
	Args []RawArg
	// Pos is the carrier line, filled by the carrier's owner.
	Pos position.Pos
	// Negated is set by the carrier's owner for an instance written
	// in the negated form. The grammar reads a negated payload the
	// same way as any other.
	Negated bool
	// DirectiveShaped is set by the carrier's owner for an instance
	// whose carrier line has the host toolchain's tool-directive
	// shape. A formatter may move such a line to the end of its doc
	// comment, as gofmt does, so validation warns where one subject's
	// instances of a repeatable directive mix the shape with other
	// carriers.
	DirectiveShaped bool
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
	// unit parse errors report. The carrier's owner converts it to
	// a file position. On a joined continuation the offset
	// addresses the joined payload, so a diagnostic there points
	// at the instance's first line.
	Col int
}

// RawValue is one value as written: a scalar spelling, or a list.
// Exactly one form is populated: List is nil for a scalar, and
// Text is empty for a list. A nested list parses, and validation
// refuses it against every schema.
type RawValue struct {
	// Text is the scalar spelling with escapes resolved, and empty
	// for a list.
	Text string
	// Quoted reports whether the spelling was quoted, so the empty
	// string can be a value.
	Quoted bool
	// List contains the elements of a list value, and is nil for a
	// scalar.
	List []RawValue
}

// Join folds continued carrier lines into one payload: a line
// ending in a backslash joins the next, marker already stripped,
// with a single space. The rule is grammar, so it is defined here
// once and in no frontend.
//
// # Allocation contract
//
// Join returns a part of the line without allocating for one line, and
// allocates the joined payload once for more.
func Join(lines []string) string {
	if len(lines) == 1 {
		return folded(0, lines[0])
	}
	n := len(joinSep) * max(len(lines)-1, 0)
	for i, line := range lines {
		n += len(folded(i, line))
	}
	var out strings.Builder
	out.Grow(n)
	for i, line := range lines {
		if i > 0 {
			out.WriteString(joinSep)
		}
		out.WriteString(folded(i, line))
	}
	return out.String()
}

// folded returns line i of a continued payload as [Join] joins it: the
// continuation marker and the blanks before it cut, and a later line's
// leading blanks cut too, so the join is exactly one space.
func folded(i int, line string) string {
	trimmed, continued := strings.CutSuffix(line, Continuation)
	if continued {
		trimmed = strings.TrimRight(trimmed, " \t")
	}
	if i > 0 {
		trimmed = strings.TrimLeft(trimmed, " \t")
	}
	return trimmed
}

// argScratch is how many arguments of one payload, and how many
// elements of one list, the parser collects on the stack before it
// copies them out. A payload with more grows onto the heap.
const argScratch = 8

// Parse reads one payload against the pinned grammar. The payload
// is the text after the carrier marker, one logical line with
// continuations already joined.
//
// A payload outside the grammar returns an error with the byte
// offset where reading stopped. The caller converts the offset to a
// file position. Parse never panics, whatever the bytes.
//
// # Allocation contract
//
// The name, the keys and the unquoted values are parts of the payload.
// Parse allocates the instance's arguments once, each list's elements
// once, and each quoted value with an escape once.
func Parse(payload string) (Raw, error) {
	p := &parser{payload: payload}
	p.skipSpace()

	name, err := p.name()
	if err != nil {
		return Raw{}, err
	}
	raw := Raw{Name: name}

	// Whitespace separates the name from each argument and each
	// argument from the next. A token that stops at any other byte
	// is refused.
	var scratch [argScratch]RawArg
	args := scratch[:0]
	for !p.done() {
		if !p.blank() {
			return Raw{}, p.fail("an argument follows whitespace, and %q does not", string(p.peek()))
		}
		p.skipSpace()
		if p.done() {
			break
		}
		arg, err := p.arg()
		if err != nil {
			return Raw{}, err
		}
		args = append(args, arg)
	}
	if len(args) > 0 {
		raw.Args = slices.Clone(args)
	}
	return raw, nil
}

// parser reads one payload left to right. at is the next unread
// byte, and every refusal names it.
type parser struct {
	payload string
	at      int
}

// done reports whether the payload is fully read.
func (p *parser) done() bool { return p.at >= len(p.payload) }

// peek returns the next byte without reading it, and zero at the
// end.
func (p *parser) peek() byte {
	if p.done() {
		return 0
	}
	return p.payload[p.at]
}

// blank reports whether the next byte is a space or a tab.
func (p *parser) blank() bool {
	c := p.peek()
	return c == ' ' || c == '\t'
}

// skipSpace reads past spaces and tabs.
func (p *parser) skipSpace() {
	for !p.done() && p.blank() {
		p.at++
	}
}

// fail returns a refusal naming the offset where reading stopped.
func (p *parser) fail(format string, args ...any) error {
	return fmt.Errorf("directive: offset %d: %s", p.at, fmt.Sprintf(format, args...))
}

// name reads the directive name: an identifier, optionally
// prefixed by another and a colon. The name is the part of the
// payload it was read from.
func (p *parser) name() (Name, error) {
	start := p.at
	first, err := p.ident("a directive name")
	if err != nil {
		return "", err
	}
	if p.peek() != prefixSep {
		return Name(first), nil
	}
	p.at++
	if _, err := p.ident("a directive name after its plugin prefix"); err != nil {
		return "", err
	}
	return Name(p.payload[start:p.at]), nil
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

// list reads a bracketed, comma-separated value list. Whitespace
// may follow a comma and nothing else, as the pinned grammar reads.
// The elements collect on the stack and copy out once.
func (p *parser) list() (RawValue, error) {
	p.at++ // consume '['
	if p.peek() == listClose {
		p.at++
		return RawValue{List: []RawValue{}}, nil
	}
	var scratch [argScratch]RawValue
	elements := scratch[:0]
	for {
		element, err := p.value()
		if err != nil {
			return RawValue{}, err
		}
		elements = append(elements, element)

		switch p.peek() {
		case listSep:
			p.at++
			p.skipSpace()
		case listClose:
			p.at++
			return RawValue{List: slices.Clone(elements)}, nil
		default:
			return RawValue{}, p.fail("a list element ends with %q or %q",
				string(listSep), string(listClose))
		}
	}
}

// quoted reads a double-quoted string, resolving the four escapes. A
// value without an escape is the part of the payload between its
// quotes.
func (p *parser) quoted() (RawValue, error) {
	p.at++ // consume the opening quote
	start := p.at
	for !p.done() {
		switch p.peek() {
		case quote:
			text := p.payload[start:p.at]
			p.at++
			return RawValue{Text: text, Quoted: true}, nil
		case escape:
			return p.escaped(start)
		}
		p.at++
	}
	return RawValue{}, p.fail("a quoted value is unterminated")
}

// escaped reads the rest of a quoted value from its first escape, the
// value having started at start, and resolves the escapes into a string
// of its own.
func (p *parser) escaped(start int) (RawValue, error) {
	var out strings.Builder
	out.WriteString(p.payload[start:p.at])
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
				// The escaped character can take more than one byte,
				// so the refusal decodes the whole rune.
				r, _ := utf8.DecodeRuneInString(p.payload[p.at+1:])
				return RawValue{}, p.fail(
					`an escape is one of \" \\ \n \t, not \%s`, string(r),
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

// isLetter reports whether c is an ASCII letter.
func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isIdent reports whether c may continue an identifier.
func isIdent(c byte) bool {
	return isLetter(c) || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// isIdentifier reports whether s is one whole ident of the grammar:
// a letter, then letters, digits, hyphens and underscores.
func isIdentifier(s string) bool {
	if s == "" || !isLetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isIdent(s[i]) {
			return false
		}
	}
	return true
}

// isBare reports whether a bare value may contain c: anything but
// whitespace and the grammar's punctuation.
func isBare(c byte) bool {
	switch c {
	case ' ', '\t', quote, listOpen, listClose, listSep, keySep:
		return false
	}
	return true
}
