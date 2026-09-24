// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import "strings"

// Pascal converts s to PascalCase.
//
// Each word's first rune is upper-cased and the rest lower-cased. A
// word whose upper-cased form is a recognised initialism (see
// [CommonInitialisms]) is upper-cased in full, and any other
// all-upper ASCII word is kept whole as an acronym run, so
// Pascal("IOReader") returns "IOReader". An upper-case identifier,
// an upper-case ASCII letter followed by upper-case letters, digits
// and underscores, is returned unchanged: "FOO" and "STATUS_ACTIVE"
// keep their spelling. An empty or separator-only input returns "".
// An input already in the style returns itself and allocates
// nothing.
func (c *Caser) Pascal(s string) string {
	if isUpperIdentifier(s) || c.matchesTitled(s, false, true) {
		return s
	}
	words := c.Words(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, w := range words {
		c.writeTitleWord(&b, w, true)
	}
	return b.String()
}

// Camel converts s to camelCase.
//
// The first word is fully lower-cased, so "URLPath" becomes "urlPath"
// and "HTTPServer" becomes "httpServer". Every later word is
// title-cased by the rules of [Caser.Pascal]: a recognised
// initialism is upper-cased in full, and an all-upper ASCII word is
// kept whole in an input that also contains a lower-case letter. In
// an input without a lower-case letter no word is an acronym run, so
// "STATUS_ACTIVE" becomes "statusActive". An input already in the
// style returns itself and allocates nothing.
func (c *Caser) Camel(s string) string {
	keepUpper := !isUpperInput(s)
	if c.matchesTitled(s, true, keepUpper) {
		return s
	}
	words := c.Words(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	writeCased(&b, words[0], false)
	for _, w := range words[1:] {
		c.writeTitleWord(&b, w, keepUpper)
	}
	return b.String()
}

// Snake converts s to snake_case (lower-case words joined by '_').
func (c *Caser) Snake(s string) string { return c.joined(s, '_', false) }

// ScreamingSnake converts s to SCREAMING_SNAKE_CASE (upper-case words
// joined by '_').
func (c *Caser) ScreamingSnake(s string) string { return c.joined(s, '_', true) }

// Kebab converts s to kebab-case (lower-case words joined by '-').
func (c *Caser) Kebab(s string) string { return c.joined(s, '-', false) }

// joined splits s and writes its words into one Builder, separated by
// sep and case-mapped by up. It is the shared body of the three
// separator styles, and a conversion costs one allocation for the
// result.
//
// Grow(len(s)) is a size hint. The output of a Unicode input can be
// longer than the input: U+0250 is two bytes and upper-cases to a
// three-byte rune, and the Builder grows to fit.
func (c *Caser) joined(s string, sep byte, up bool) string {
	if matchesJoined(s, sep, up) {
		return s
	}
	words := c.Words(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, w := range words {
		if i > 0 {
			b.WriteByte(sep)
		}
		writeCased(&b, w, up)
	}
	return b.String()
}

// matchesJoined reports whether the joined style's output for s is
// s itself: every rune already in the mapped case, exactly one
// separator between words, none leading or trailing. The scan
// allocates nothing, so an already-styled input passes through
// whole. A word with invalid UTF-8 never matches, because its output
// is rebuilt.
func matchesJoined(s string, sep byte, up bool) bool {
	o, ok, first := 0, true, true
	wordSpans(s, func(start, end int, dirty bool) bool {
		if dirty {
			ok = false
			return false
		}
		if !first {
			if o >= len(s) || s[o] != sep {
				ok = false
				return false
			}
			o++
		}
		if o != start {
			ok = false
			return false
		}
		for j := start; j < end; {
			r, sz := decodeRuneAt(s, j)
			mapped := lowerRune(r)
			if up {
				mapped = upperRune(r)
			}
			if mapped != r {
				ok = false
				return false
			}
			j += sz
		}
		o, first = end, false
		return true
	})
	return ok && !first && o == len(s)
}

// matchesTitled reports whether a title-family style's output for s
// is s itself, under [Caser.writeTitleWord]'s rules per word: a
// recognised initialism appears in its canonical form, an all-upper
// ASCII word is kept whole where keepUpper is set, and every other
// word title-cases. The first word lower-cases whole where
// firstLower is set, which is camel's opening. The words follow each
// other with no separator. The scan allocates nothing.
func (c *Caser) matchesTitled(s string, firstLower, keepUpper bool) bool {
	o, ok, first := 0, true, true
	wordSpans(s, func(start, end int, dirty bool) bool {
		if dirty || o != start {
			ok = false
			return false
		}
		w := s[start:end]
		switch {
		case first && firstLower:
			for j := start; j < end; {
				r, sz := decodeRuneAt(s, j)
				if lowerRune(r) != r {
					ok = false
					return false
				}
				j += sz
			}
		default:
			if canon, known := c.lookupInitialism(w); known {
				if canon != w {
					ok = false
					return false
				}
			} else if !keepUpper || !isAllUpperASCII(w) {
				r, sz := decodeRuneAt(s, start)
				if upperRune(r) != r {
					ok = false
					return false
				}
				for j := start + sz; j < end; {
					rest, rsz := decodeRuneAt(s, j)
					if lowerRune(rest) != rest {
						ok = false
						return false
					}
					j += rsz
				}
			}
		}
		o, first = end, false
		return true
	})
	return ok && !first && o == len(s)
}

// writeCased writes w into b, upper-cased when up is set and
// lower-cased otherwise. Case mapping is per rune, because a byte
// past 0x7F is part of a multi-byte sequence and a mapped rune can
// differ in width.
func writeCased(b *strings.Builder, w string, up bool) {
	if up {
		for _, r := range w {
			b.WriteRune(upperRune(r))
		}
		return
	}
	for _, r := range w {
		b.WriteRune(lowerRune(r))
	}
}

// Words returns the component words of s using the default Caser. See
// [Caser.Words] for the splitting rules.
func Words(s string) []string { return Default().Words(s) }

// Pascal converts s to PascalCase using the default Caser.
func Pascal(s string) string { return Default().Pascal(s) }

// Camel converts s to camelCase using the default Caser.
func Camel(s string) string { return Default().Camel(s) }

// Snake converts s to snake_case using the default Caser.
func Snake(s string) string { return Default().Snake(s) }

// ScreamingSnake converts s to SCREAMING_SNAKE_CASE using the default Caser.
func ScreamingSnake(s string) string { return Default().ScreamingSnake(s) }

// Kebab converts s to kebab-case using the default Caser.
func Kebab(s string) string { return Default().Kebab(s) }
