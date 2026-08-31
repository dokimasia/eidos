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
func (c *Caser) Pascal(s string) string {
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
// "URLPath" → "urlPath", "HTTPServer" → "httpServer".
func (c *Caser) Camel(s string) string {
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
// of [Caser.Pascal], joined by single spaces).
func (c *Caser) Title(s string) string {
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
