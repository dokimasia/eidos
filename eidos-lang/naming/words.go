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
// Boundaries are inserted at:
//
//   - Separator runes (_, -, ., space, slash, tab) — consumed, never emitted.
//   - Lower-to-upper transitions (helloWorld → ["hello", "World"]).
//   - Acronym-then-word boundaries: an upper-case run followed by an
//     upper-case rune that is itself followed by a lower-case rune —
//     "HTTPServer" → ["HTTP", "Server"], "URLPath" → ["URL", "Path"].
//   - Digit-to-upper transitions: an upper-case rune after a digit
//     starts a word — "Int64Value" → ["Int64", "Value"].
//
// A digit belongs to the word before it. "Version2" is one word;
// consumers that want "Version_2" can pre-separate the digit.
//
// An empty or separator-only input returns nil.
//
// Words splits by structural rules only; the receiver's initialism set
// is used by the case converters that build on top of Words, not by
// the splitter itself. The method is defined on Caser purely for API
// symmetry with the converters.
//
// # Aliasing
//
// The returned words are substrings of s and share its backing array,
// so holding one word retains all of s. That is the right trade at
// generator lifetimes — a word is consumed into a rendered identifier
// immediately — but a caller stashing a single word out of a very
// large input for a long time should copy it.
//
// The one exception is a word containing invalid UTF-8: it is rebuilt
// with U+FFFD substituted per invalid byte, so it owns its bytes.
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

// wordSpans calls yield for every word's byte span in s, under the
// boundary rules [Caser.Words] documents, without allocating: the
// converters' identity checks walk it to compare an input against
// the output it would produce, and Words collects it into the
// slice its callers hold.
func wordSpans(s string, yield func(start, end int, dirty bool) bool) {
	// start is the byte offset the current word began at, or -1 when
	// the scan is between words. dirty records whether the current
	// word contains an invalid byte, which decides whether it can be
	// sliced or has to be rebuilt.
	start, dirty := -1, false
	// prev is the immediately preceding rune, separators included —
	// mirroring the original rune-indexed form, which read runes[i-1]
	// without regard for whether a flush had just happened. "a_B"
	// therefore sees prev='_' at 'B' and takes no case boundary,
	// having already broken on the separator.
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

// wordAt returns the word spanning [start, end) of s.
//
// The common case is a substring header — no copy, no allocation,
// which is the whole point of scanning byte offsets rather than
// accumulating runes.
//
// A word carrying invalid UTF-8 is rebuilt instead. The previous
// implementation went through []rune(s), and that conversion folds
// every invalid byte to U+FFFD; slicing preserves the raw byte. The
// difference is observable — Words("\xbe") is ["�"], not
// ["\xbe"] — and the package's own conservation oracle depends on it,
// because the test helper it compares against also iterates runes.
// Ranging a string yields utf8.RuneError for an invalid byte and
// WriteRune encodes that as U+FFFD, so this reproduces the old
// spelling exactly.
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

// decodeRuneAt returns the rune at byte offset i and its width, with
// an ASCII fast path that skips the decoder entirely for the bytes
// that make up almost every identifier this package sees.
func decodeRuneAt(s string, i int) (rune, int) {
	if s[i] < utf8.RuneSelf {
		return rune(s[i]), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// countWordStarts returns a capacity estimate for the words slice: one
// for the first word, plus one per separator, plus one per ASCII
// lower-to-upper or digit-to-upper transition.
//
// A cheap over-estimate is fine and an under-estimate merely costs a
// growth step, so the scan stays on the ASCII fast path and does not
// decode. It deliberately does not model the acronym boundary
// ("HTTPServer" counts as one), which under-counts by one per
// acronym — rare in identifiers, and cheaper to absorb than a second
// lookahead pass.
//
// Sizing on separators alone would not work: the case-boundary styles
// this package exists to serve routinely pass PascalCase input with no
// separator in it at all, where that estimate is 1 for any number of
// words and the growth sequence runs in full.
func countWordStarts(s string) int {
	n := 1
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '_' || c == '-' || c == '.' || c == ' ' || c == '\t' || c == '/':
			n++
		case i > 0 && c >= 'A' && c <= 'Z' && (s[i-1] >= 'a' && s[i-1] <= 'z' || s[i-1] >= '0' && s[i-1] <= '9'):
			n++
		}
	}
	return n
}

// breaksBefore reports whether a word boundary falls immediately
// before cur. prev is the preceding rune; next is the byte offset just
// past cur, used for the acronym lookahead.
//
// A lower-case rune or a digit before an upper-case one breaks, and
// so does the last upper-case rune of a run that a lower-case rune
// follows, the lookahead decoding one rune out of the string.
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

// isSeparator reports whether r is one of the recognised word-separator
// runes. Separators are consumed by the splitter — they appear in
// neither the words slice nor any subsequent rendering.
func isSeparator(r rune) bool {
	switch r {
	case '_', '-', '.', ' ', '\t', '/':
		return true
	default:
		return false
	}
}

// upperRune is a thin wrapper over unicode.ToUpper, named to make the
// case-conversion intent obvious at call sites in this package.
func upperRune(r rune) rune { return unicode.ToUpper(r) }

// lowerRune is the symmetrical lower-case wrapper.
func lowerRune(r rune) rune { return unicode.ToLower(r) }
