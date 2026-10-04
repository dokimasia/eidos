// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Words splits s into its component words.
//
// A boundary falls at:
//
//   - a separator rune, which is _, -, ., space, slash or tab, and which
//     no word contains
//   - a lower-to-upper transition: "helloWorld" splits into "hello" and
//     "World"
//   - the last upper-case rune of a run that a lower-case rune follows:
//     "HTTPServer" splits into "HTTP" and "Server", and "URLPath" into
//     "URL" and "Path"
//   - an upper-case rune after a digit: "Int64Value" splits into "Int64"
//     and "Value"
//
// A digit belongs to the word before it, so "Version2" is one word. A
// consumer that wants "Version_2" separates the digit first.
//
// An empty or separator-only input returns nil. Any other allocates the
// returned slice, once for an ASCII input.
//
// Words splits by structural rules only. The receiver's initialism set
// applies in the case converters, not in the splitter, and the method is
// on Caser for symmetry with them.
//
// # Aliasing
//
// The returned words are substrings of s and share its backing array.
// The collector cannot free s while one of its words is reachable, so a
// caller that stores one word of a large input for a long time copies
// it.
//
// A word containing invalid UTF-8 is a copy with U+FFFD in place of
// each invalid byte, and it shares no bytes with s.
func (*Caser) Words(s string) []string {
	if s == "" {
		return nil
	}

	out := make([]string, 0, countWordStarts(s))
	wordSpans(s, func(start, end int, dirty bool) bool {
		out = append(out, wordAt(s, start, end, dirty))
		return true
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

// wordSpans calls yield with the byte span of each word of s, in order,
// under the boundary rules of [Caser.Words]. It sets dirty for a word
// that contains invalid UTF-8. It stops when yield returns false and
// allocates nothing. The converters write each word into their result
// as wordSpans yields it. Their identity checks compare an input with
// the output it would produce.
func wordSpans(s string, yield func(start, end int, dirty bool) bool) {
	// start is the byte offset the current word began at, or -1 when
	// the scan is between words. dirty records whether the current word
	// contains an invalid byte. A dirty word is rebuilt, and a clean one
	// is sliced.
	start, dirty := -1, false
	// prev is the rune before the current one, separators included. At
	// the 'B' of "a_B" it is '_', so the scan takes no case boundary
	// there. The separator has ended the word already.
	var prev rune
	first := true

	for i := 0; i < len(s); {
		r, sz := decodeRuneAt(s, i)
		switch {
		case isSeparator(r):
			if start >= 0 {
				if !yield(start, i, dirty) {
					return
				}
				start, dirty = -1, false
			}
		case !first && breaksBefore(prev, r, s, i+sz):
			if start >= 0 {
				if !yield(start, i, dirty) {
					return
				}
				dirty = false
			}
			start = i
		case start < 0:
			start = i
		}
		if start >= 0 && r == utf8.RuneError && sz == 1 {
			dirty = true
		}
		prev, first = r, false
		i += sz
	}
	if start >= 0 {
		yield(start, len(s), dirty)
	}
}

// wordAt returns the word spanning [start, end) of s. A word of valid
// UTF-8 is the substring, which allocates nothing. A dirty word is a
// copy with U+FFFD in place of each invalid byte, the spelling that
// ranging over a string and writing each rune produce, so Words("\xbe")
// returns ["�"].
func wordAt(s string, start, end int, dirty bool) string {
	if !dirty {
		return s[start:end]
	}
	var b strings.Builder
	b.Grow(end - start)
	for _, r := range s[start:end] {
		b.WriteRune(r)
	}
	return b.String()
}

// decodeRuneAt returns the rune at byte offset i and its width. An ASCII
// byte returns without the UTF-8 decoder.
func decodeRuneAt(s string, i int) (rune, int) {
	if s[i] < utf8.RuneSelf {
		return rune(s[i]), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// countWordStarts returns a capacity estimate for the words slice: one
// for the first word, plus one per separator, plus one per ASCII
// lower-to-upper or digit-to-upper transition, plus one per ASCII
// acronym boundary, the last upper-case letter of a run that a
// lower-case letter follows, as "HTTPServer" has at its "S".
//
// The scan stays on the ASCII fast path and does not decode. It counts
// at least the words of an ASCII input, so [Caser.Words] allocates its
// slice once for one. A non-ASCII boundary it does not count costs one
// growth step. The case boundaries count because a PascalCase input,
// which the case-boundary styles convert most, contains no separator.
func countWordStarts(s string) int {
	n := 1
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '_' || c == '-' || c == '.' || c == ' ' || c == '\t' || c == '/':
			n++
		case i > 0 && c >= 'A' && c <= 'Z' && (s[i-1] >= 'a' && s[i-1] <= 'z' || s[i-1] >= '0' && s[i-1] <= '9'):
			n++
		case i > 0 && i+1 < len(s) && c >= 'A' && c <= 'Z' && s[i-1] >= 'A' && s[i-1] <= 'Z' &&
			s[i+1] >= 'a' && s[i+1] <= 'z':
			n++
		}
	}
	return n
}

// breaksBefore reports whether a word boundary falls immediately before
// cur. prev is the preceding rune, and next is the byte offset just past
// cur, where the acronym lookahead decodes one rune.
//
// A lower-case rune or a digit before an upper-case one breaks, and so
// does the last upper-case rune of a run that a lower-case rune follows.
func breaksBefore(prev, cur rune, s string, next int) bool {
	if (unicode.IsLower(prev) || unicode.IsDigit(prev)) && unicode.IsUpper(cur) {
		return true
	}
	if unicode.IsUpper(prev) && unicode.IsUpper(cur) && next < len(s) {
		r, _ := decodeRuneAt(s, next)
		return unicode.IsLower(r)
	}
	return false
}

// isSeparator reports whether r is one of the word-separator runes. No
// word contains a separator, so none appears in a converted name.
func isSeparator(r rune) bool {
	switch r {
	case '_', '-', '.', ' ', '\t', '/':
		return true
	default:
		return false
	}
}

// upperRune returns the upper-case mapping of r, as unicode.ToUpper
// does.
func upperRune(r rune) rune { return unicode.ToUpper(r) }

// lowerRune returns the lower-case mapping of r, as unicode.ToLower
// does.
func lowerRune(r rune) rune { return unicode.ToLower(r) }
