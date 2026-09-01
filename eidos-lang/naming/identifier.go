// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"strings"
	"unicode"
)

// Identifier sanitises s into an identifier the C-family languages
// accept: letters and digits survive, underscores survive, every other
// rune is replaced with an underscore, and a leading digit is prefixed
// with one. An empty input returns the single underscore "_".
//
// The function does not change case; combine with [Pascal], [Camel],
// or any other style converter when a specific shape is required.
//
// Reserved words are not handled here. Which spellings a language
// forbids is that language's own knowledge, so a satellite layers the
// check on top.
func Identifier(s string) string {
	if s == "" {
		return "_"
	}
	if isValidIdentifier(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 1)
	for i, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
			b.WriteRune(r)
		case unicode.IsDigit(r):
			if i == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// isValidIdentifier reports whether s already satisfies everything
// [Identifier] would produce, so the input can be returned as-is.
//
// This is the normal case — the function exists to sanitise source
// identifiers that are usually already legal — and without the
// pre-scan every one of them paid a heap allocation and a full byte
// copy to reproduce itself.
//
// Deliberately ASCII-only. A byte at or above 0x80 falls through to
// the general loop, which keeps unicode.IsLetter's semantics for
// non-ASCII letters: 'é' is a letter and survives there, and this
// scan must not decide otherwise. A leading digit falls through too,
// since its output differs from the input by a prefixed underscore.
func isValidIdentifier(s string) bool {
	if s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			return false
		}
	}
	return true
}
