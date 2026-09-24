// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// ErrInvalidInitialism is returned by [Caser.WithInitialisms] for a
// candidate initialism that breaks the rule: non-empty, opening with
// an upper-case ASCII letter, and continuing with upper-case ASCII
// letters or digits, such as "URL", "UTF8" and "HTTP2".
var ErrInvalidInitialism = errors.New(
	"naming: initialism must be non-empty, start with an upper-case ASCII letter, and contain only upper-case letters and digits",
)

// CommonInitialisms is the list of initialisms [Default] recognises:
// the acronyms the common style guides keep upper-case, so
// "url_path" converts through Pascal to "URLPath".
//
// A consumer that needs another set builds a Caser with [New] and
// [Caser.WithInitialisms].
var CommonInitialisms = []string{
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID",
	"HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS",
	"RAM", "RHS", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP",
	"TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI", "URL", "UTF8",
	"VM", "XML", "XMPP", "XSRF", "XSS",
}

// Caser is a case-conversion configuration: the set of recognised
// initialisms. The zero value is unusable. Construct one with
// [Default] or [New].
//
// A Caser is immutable once constructed. [Caser.WithInitialisms]
// returns a new Caser and leaves the receiver unchanged, so a Caser
// is safe to share across goroutines without locking.
//
// # Idempotence
//
// Every style is idempotent over identifiers built from ASCII letters
// and the recognised separators: converting an already-converted
// name returns it unchanged. That covers the input a frontend derives
// from source identifiers.
//
// Outside that domain a first application can move a word boundary
// that the second reads differently, so f(f(x)) may differ from
// f(x). The second application of every style is a fixed point:
// f(f(f(x))) equals f(f(x)). Three worked cases:
//
//	Pascal("aA1a")       -> "AA1a" -> "Aa1a"
//	Pascal("aÉ")         -> "AÉ"   -> "Aé"
//	ScreamingSnake("ßa") -> "ßA"   -> "ß_A"
//
// In the first, "aA1a" splits into "a" and "A1a", and the second
// pass reads "AA1a" as one word, which title-cases to "Aa1a". The
// others turn on runes whose case mapping changes which side of a
// boundary they fall on. A caller converting input with digits or non-ASCII runes,
// or feeding converted names back through a style, converts once
// from the original source name.
type Caser struct {
	// initialisms maps the upper-case form of a recognised initialism
	// to its canonical spelling. Both are the same text for anything
	// passing isValidInitialism, so a hit returns the stored string
	// and recognising "http" as "HTTP" allocates nothing.
	initialisms map[string]string
}

// Default returns a Caser pre-loaded with [CommonInitialisms]. Every
// call returns the same Caser, which is safe to share because a
// Caser is immutable.
func Default() *Caser { return defaultCaser }

// New returns a Caser that recognises no initialism, so Pascal
// spells "url" as "Url".
func New() *Caser {
	return &Caser{initialisms: map[string]string{}}
}

// WithInitialisms returns a new Caser that recognises the given
// initialisms in addition to the receiver's.
//
// Each initialism is non-empty, opens with an upper-case ASCII letter
// and continues with upper-case ASCII letters or digits. Any other
// candidate returns [ErrInvalidInitialism], wrapped with the
// offending value.
func (c *Caser) WithInitialisms(words ...string) (*Caser, error) {
	out := &Caser{initialisms: maps.Clone(c.initialisms)}
	if out.initialisms == nil {
		out.initialisms = map[string]string{}
	}
	for _, w := range words {
		if !isValidInitialism(w) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidInitialism, w)
		}
		out.initialisms[w] = w
	}
	return out, nil
}

// Initialisms returns the recognised initialisms in alphabetical order.
// The returned slice is a fresh copy, and a caller may modify it.
func (c *Caser) Initialisms() []string {
	// The slice is sized to the set, so appending never regrows it.
	out := slices.AppendSeq(make([]string, 0, len(c.initialisms)), maps.Keys(c.initialisms))
	slices.Sort(out)
	return out
}

// maxProbeLen bounds the stack buffer [Caser.lookupInitialism]
// upper-cases into. A longer word upper-cases through
// strings.ToUpper, which allocates. The longest entry of
// [CommonInitialisms] is 5 bytes.
const maxProbeLen = 16

// lookupInitialism reports whether w names a recognised initialism, and
// returns its canonical spelling.
//
// An ASCII word of up to [maxProbeLen] bytes upper-cases into a stack
// buffer, so the lookup allocates nothing. A word with a non-ASCII
// byte upper-cases through strings.ToUpper, because a non-ASCII rune
// can upper-case to an ASCII letter: 'ı' (U+0131) upper-cases to 'I',
// so "ıd" matches "ID".
func (c *Caser) lookupInitialism(w string) (string, bool) {
	if len(w) <= maxProbeLen {
		var buf [maxProbeLen]byte
		ascii := true
		for i := range len(w) {
			ch := w[i]
			if ch >= utf8.RuneSelf {
				ascii = false
				break
			}
			if ch >= 'a' && ch <= 'z' {
				ch -= 'a' - 'A'
			}
			buf[i] = ch
		}
		if ascii {
			canon, ok := c.initialisms[string(buf[:len(w)])]
			return canon, ok
		}
	}
	canon, ok := c.initialisms[strings.ToUpper(w)]
	return canon, ok
}

// isAllUpperASCII reports whether every byte of s is an upper-case
// ASCII letter. The caller passes a non-empty s.
func isAllUpperASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// isUpperIdentifier reports whether s is an upper-case identifier: an
// upper-case ASCII letter followed by upper-case ASCII letters,
// digits and underscores, such as "FOO" and "STATUS_ACTIVE".
func isUpperIdentifier(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			return false
		}
	}
	return true
}

// isUpperInput reports whether s is ASCII, contains an upper-case
// letter and contains no lower-case letter, so its case separates no
// words: "STATUS_ACTIVE" or "FOO-BAR".
func isUpperInput(s string) bool {
	upper := false
	for i := range len(s) {
		switch c := s[i]; {
		case c >= utf8.RuneSelf, c >= 'a' && c <= 'z':
			return false
		case c >= 'A' && c <= 'Z':
			upper = true
		}
	}
	return upper
}

// isValidInitialism reports whether s is a valid initialism: non-empty,
// opening with an upper-case ASCII letter, and continuing with
// upper-case ASCII letters or digits. "UTF8" and "HTTP2" pass, and
// "8K" and "8" do not.
func isValidInitialism(s string) bool {
	if s == "" {
		return false
	}
	if s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

// defaultCaser is the Caser [Default] returns. It is built without
// validating [CommonInitialisms], because the package's tests check
// every entry against the rule [Caser.WithInitialisms] applies.
var defaultCaser = withInitialismsUnchecked(CommonInitialisms)

// withInitialismsUnchecked builds a Caser from trusted initialisms
// without validating them. Only [defaultCaser] is built through it.
// Every other Caser is built through [Caser.WithInitialisms].
func withInitialismsUnchecked(words []string) *Caser {
	out := &Caser{initialisms: make(map[string]string, len(words))}
	for _, w := range words {
		out.initialisms[w] = w
	}
	return out
}

// writeTitleWord writes w's title-cased form into b: the first rune
// upper-cased and the rest lower-cased. A word naming a recognised
// initialism is written in its canonical spelling. When keepUpper is
// set, an all-upper ASCII word is written as it is: an acronym run.
//
// [Caser.Pascal] and [Caser.Camel] title-case every word through it.
func (c *Caser) writeTitleWord(b *strings.Builder, w string, keepUpper bool) {
	if canon, ok := c.lookupInitialism(w); ok {
		b.WriteString(canon)
		return
	}
	if keepUpper && isAllUpperASCII(w) {
		b.WriteString(w)
		return
	}
	writeTitleCased(b, w)
}

// writeTitleCased writes w with its first rune upper-cased and the
// rest lower-cased. Case mapping is per rune, because a byte past
// 0x7F is part of a multi-byte sequence.
func writeTitleCased(b *strings.Builder, w string) {
	for i, r := range w {
		if i == 0 {
			b.WriteRune(upperRune(r))
			continue
		}
		b.WriteRune(lowerRune(r))
	}
}
