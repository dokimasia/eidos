// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package naming

import "strings"

// Pascal converts s to PascalCase.
//
// Each word's first rune is upper-cased and the rest lower-cased,
// except that words whose upper-cased form is a recognised initialism
// (see [CommonInitialisms]) are upper-cased in full and already-all-upper
// inputs are preserved. An empty or separator-only input returns "".
// An input already in the style returns itself and allocates
// nothing.
func (c *Caser) Pascal(s string) string {
	if c.matchesTitled(s, 0, false) {
		return s
	}
	words := c.Words(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, w := range words {
		c.writeTitleWord(&b, w)
	}
	return b.String()
}

// Camel converts s to camelCase.
//
// The first word is fully lower-cased; subsequent words are title-cased
// using the same rules as [Caser.Pascal] (initialism preservation
// included). The first word's lower-casing is unconditional, so
// "URLPath" → "urlPath", "HTTPServer" → "httpServer". An input
// already in the style returns itself and allocates nothing.
func (c *Caser) Camel(s string) string {
	if c.matchesTitled(s, 0, true) {
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
		c.writeTitleWord(&b, w)
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

// ScreamingKebab converts s to SCREAMING-KEBAB-CASE (upper-case words
// joined by '-').
func (c *Caser) ScreamingKebab(s string) string { return c.joined(s, '-', true) }

// Dot converts s to dot.case (lower-case words joined by '.').
func (c *Caser) Dot(s string) string { return c.joined(s, '.', false) }

// Title converts s to Title Case (each word title-cased per the rules
// of [Caser.Pascal], joined by single spaces). An input already in
// the style returns itself and allocates nothing.
func (c *Caser) Title(s string) string {
	if c.matchesTitled(s, ' ', false) {
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
			b.WriteByte(' ')
		}
		c.writeTitleWord(&b, w)
	}
	return b.String()
}

// joined splits s and writes its words into one Builder, separated by
// sep and case-mapped by up. It is the shared body of the five
// separator styles.
//
// It replaces a helper that built a []string of transformed words and
// handed it to strings.Join: one allocation for the slice, one per
// word whose case actually changed, and one more for Join's buffer,
// with every byte copied twice. Writing through a single Builder makes
// it one allocation for the whole call.
//
// Grow(len(s)) is a hint, not a bound. len(s) is not an upper bound on
// the output for Unicode input — U+0250 is two bytes and upper-cases
// to a three-byte rune — and Builder regrows on overflow, which is
// exactly why a Builder is the right target and a fixed buffer sized
// on that assumption would not be.
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
// allocates nothing, which is what lets an already-styled input
// pass through whole; a word carrying invalid UTF-8 never matches,
// because its output rebuilds.
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

// matchesTitled reports whether a title-family style's output for
// s is s itself, under [Caser.writeTitleWord]'s own rules per
// word: a recognised initialism stands in its canonical form, an
// all-upper ASCII word stands whole, and everything else title
// cases; the first word lower-cases whole where firstLower says
// so, which is camel's opening. sep is the byte between words,
// zero for none. The scan allocates nothing.
func (c *Caser) matchesTitled(s string, sep byte, firstLower bool) bool {
	o, ok, first := 0, true, true
	wordSpans(s, func(start, end int, dirty bool) bool {
		if dirty {
			ok = false
			return false
		}
		if !first && sep != 0 {
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
			if canon, held := c.lookupInitialism(w); held {
				if canon != w {
					ok = false
					return false
				}
			} else if !isAllUpperASCII(w) {
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
// lower-cased otherwise.
//
// Case mapping is per rune. Falling back to a byte loop past 0x7F is
// not implementable: a byte above that range is part of a multi-byte
// sequence, and case-mapping it individually would corrupt the
// encoding — the mapped rune may not even be the same width.
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

// ScreamingKebab converts s to SCREAMING-KEBAB-CASE using the default Caser.
func ScreamingKebab(s string) string { return Default().ScreamingKebab(s) }

// Dot converts s to dot.case using the default Caser.
func Dot(s string) string { return Default().Dot(s) }

// Title converts s to Title Case using the default Caser.
func Title(s string) string { return Default().Title(s) }
