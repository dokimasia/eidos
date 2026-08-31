// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package naming

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// ErrInvalidInitialism is returned by [Caser.WithInitialisms] when a
// candidate initialism does not satisfy the contract: non-empty,
// composed of ASCII upper-case letters or digits, and starting with a
// letter (e.g. "URL", "UTF8", "HTTP2"). Lower-case letters and any
// other rune are rejected.
var ErrInvalidInitialism = errors.New(
	"naming: initialism must be non-empty, start with an upper-case ASCII letter, and contain only upper-case letters and digits",
)

// CommonInitialisms is the canonical list of initialisms recognised by
// [Default]: the acronyms the common style guides keep upper-case, so
// identifiers round-trip predictably ("url_path" → Pascal → "URLPath",
// not "UrlPath").
//
// Consumers that need a different set construct a custom Caser via
// [New] + [Caser.WithInitialisms].
var CommonInitialisms = []string{
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID",
	"HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS",
	"RAM", "RHS", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP",
	"TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI", "URL", "UTF8",
	"VM", "XML", "XMPP", "XSRF", "XSS",
}

// Caser holds case-conversion configuration, primarily the set of
// recognised initialisms. The zero value is unusable; construct with
// [Default] or [New].
//
// Caser values are immutable once constructed. [Caser.WithInitialisms]
// returns a new Caser, leaving the receiver unchanged. This makes it
// safe to share a Caser across goroutines without locking.
//
// # Idempotence
//
// Every style is idempotent over identifiers built from ASCII letters
// and the recognised separators: converting an already-converted name
// returns it unchanged. That covers the input a frontend derives from
// source identifiers, which is what these functions exist for.
//
// Outside that domain a first application can move a word boundary
// that the second then reads differently, so f(f(x)) may differ from
// f(x). Every style reaches a fixed point by the second application —
// f(f(f(x))) always equals f(f(x)) — so the divergence is bounded at
// one step rather than oscillating. Three worked cases:
//
//	Pascal("aA1")        -> "AA1" -> "Aa1"
//	Pascal("aÉ")         -> "AÉ"  -> "Aé"
//	ScreamingSnake("ßa") -> "ßA"   -> "ß_A"
//
// In the first, splitting "aA1" yields the words "a" and "A1", which
// Pascal renders "AA1"; re-splitting that reads a leading acronym run
// instead. The others turn on runes whose case mapping changes which
// side of a boundary they fall.
//
// This is documented rather than repaired because the repair is not
// backwards compatible: these functions name generated identifiers, so
// changing their output renames symbols in consumers' generated code.
// Callers converting non-ASCII input, or feeding already-converted
// names back through a style, should convert once from the original
// source name.
type Caser struct {
	// initialisms maps the normalised (upper-case) form of a
	// recognised initialism to its canonical spelling. Both are the
	// same text for anything passing isValidInitialism, which is the
	// point: a hit can return the stored string instead of the
	// freshly upper-cased probe, so recognising "http" as "HTTP"
	// costs no allocation.
	initialisms map[string]string
}

// Default returns a Caser pre-loaded with [CommonInitialisms]. The same
// pointer is returned on every call; callers must not assume otherwise
// but the value is safe to share.
func Default() *Caser { return defaultCaser }

// New returns a fresh Caser with no initialisms recognised. Words are
// split and converted purely by case and separator transitions; "url"
// becomes "Url" rather than "URL" under Pascal.
func New() *Caser {
	return &Caser{initialisms: map[string]string{}}
}

// WithInitialisms returns a new Caser that recognises the given
// initialisms in addition to whatever the receiver already recognised.
//
// Each initialism must be non-empty and consist solely of upper-case
// ASCII letters; otherwise [ErrInvalidInitialism] is returned wrapping
// the offending value.
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
// The returned slice is a fresh copy; callers may modify it freely.
func (c *Caser) Initialisms() []string {
	// Pre-sized rather than slices.Collect: collecting walks a
	// growth sequence from cap 1 for a set whose size is known.
	out := slices.AppendSeq(make([]string, 0, len(c.initialisms)), maps.Keys(c.initialisms))
	slices.Sort(out)
	return out
}

// maxProbeLen bounds the stack buffer [Caser.lookupInitialism] normalises
// into. Words longer than this fall back to strings.ToUpper, which is
// correct but allocates; no entry of [CommonInitialisms] comes close, and
// a longer word is overwhelmingly unlikely to be one.
const maxProbeLen = 16

// lookupInitialism reports whether w names a recognised initialism, and
// returns its canonical spelling.
//
// The probe is the allocation this package used to pay on every word.
// strings.ToUpper is not inlinable and builds its result in its own
// frame, so the buffer was allocated whether or not the lookup hit —
// and on a miss, which is the common case, it was discarded one line
// later. Upper-casing into a stack array instead lets the compiler
// elide the conversion in m[string(buf[:n])] entirely.
//
// A non-ASCII byte falls through to strings.ToUpper rather than
// reporting a miss. That distinction is load-bearing: 'ı' (U+0131)
// upper-cases to 'I', so "ıd" probes "ID" and legitimately hits a
// registered initialism. A fast path that declared a miss on
// non-ASCII would silently stop recognising it.
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
// ASCII letter. The caller guarantees s is non-empty; an empty input
// is undefined.
func isAllUpperASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// isValidInitialism reports whether s is a valid initialism: non-empty,
// composed of ASCII upper-case letters or digits, with the first byte
// being a letter (so "8K" or "8" are rejected; "UTF8" and "HTTP2" are
// accepted).
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

// defaultCaser is the singleton returned by Default. It is built without
// re-validating CommonInitialisms because that list is fixed here and
// its twin holds every entry to the same validation WithInitialisms
// applies.
var defaultCaser = withInitialismsUnchecked(CommonInitialisms)

// withInitialismsUnchecked is the validation-skipping fast path for
// constructing a Caser from a list of trusted initialisms. Used only
// for [defaultCaser] construction; external callers go through
// [Caser.WithInitialisms].
func withInitialismsUnchecked(words []string) *Caser {
	out := &Caser{initialisms: make(map[string]string, len(words))}
	for _, w := range words {
		out.initialisms[w] = w
	}
	return out
}

// writeTitleWord writes w's title-cased form straight into b: the
// first rune upper-cased and the rest lower-cased, except that an
// all-upper input is left untouched and any word naming a recognised
// initialism is written in its canonical spelling.
//
// This is the building block of [Caser.Pascal], [Caser.Camel] and
// [Caser.Title]. It replaced a string-returning form whose result
// every caller immediately copied into a Builder and dropped — one
// allocation per word, on every style that title-cases.
func (c *Caser) writeTitleWord(b *strings.Builder, w string) {
	if canon, ok := c.lookupInitialism(w); ok {
		b.WriteString(canon)
		return
	}
	if isAllUpperASCII(w) {
		b.WriteString(w)
		return
	}
	writeTitleCased(b, w)
}

// writeTitleCased writes w with its first rune upper-cased and the
// rest lower-cased. Case mapping is per rune, not per byte: past 0x7F
// a byte is a continuation byte, and case-mapping one individually
// would corrupt the encoding.
func writeTitleCased(b *strings.Builder, w string) {
	for i, r := range w {
		if i == 0 {
			b.WriteRune(upperRune(r))
			continue
		}
		b.WriteRune(lowerRune(r))
	}
}
