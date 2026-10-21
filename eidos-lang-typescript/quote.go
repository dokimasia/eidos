// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The quotes of a string literal, and the zeros that pad the hexadecimal
// digits of an escape to four.
const (
	doubleMark = '"'
	singleMark = '\''
	hexPadding = "0000"
)

// The characters that a literal writes as an escape of four hexadecimal
// digits besides the two line separators: every character before the
// space, and DEL.
const (
	firstPrintable = ' '
	deleteChar     = 0x7f
)

// The braces of an object type, and the brace that closes a substitution
// of a template literal, which [Requote] counts to find the end of a
// substitution.
const (
	blockOpen  = '{'
	blockClose = '}'
)

// requote is the state of [Requote] over one spelling. b contains the new
// spelling from the first literal whose quote changes, and written is the
// offset of the first byte of the spelling that b does not contain yet.
type requote struct {
	src     string
	style   plugin.Choice
	b       strings.Builder
	written int
}

// typ scans the type that starts at the offset i, and returns the offset
// after it. A type ends at the end of the spelling. Where substitution is
// set, the type is the content of a substitution, and it ends after the
// brace that closes the substitution. Each brace of an object type opens a
// block that a later brace closes.
func (r *requote) typ(i int, substitution bool) int {
	depth := 0
	for i < len(r.src) {
		switch r.src[i] {
		case doubleMark, singleMark:
			i = r.literal(i)
		case templateQuote:
			i = r.template(i + 1)
		case blockOpen:
			depth++
			i++
		case blockClose:
			if depth == 0 && substitution {
				return i + 1
			}
			depth--
			i++
		default:
			i++
		}
	}
	return i
}

// template scans the template literal whose text starts at the offset i,
// after its opening quote, and returns the offset after its closing quote,
// or the end of the spelling. The text keeps its bytes, and the content of
// each substitution is a type.
func (r *requote) template(i int) int {
	for i < len(r.src) {
		switch {
		case r.src[i] == escapeMark:
			i += 2
		case r.src[i] == templateQuote:
			return i + 1
		case strings.HasPrefix(r.src[i:], substitution):
			i = r.typ(i+len(substitution), true)
		default:
			i++
		}
	}
	return len(r.src)
}

// literal scans the string literal that starts at the offset i, and
// returns the offset after its closing quote. Where the quote that
// [AppendQuote] chooses for its content differs, literal writes the
// spelling up to the literal and the literal in the new quote into b. A
// literal without its closing quote ends the spelling and keeps its text.
func (r *requote) literal(i int) int {
	old := r.src[i]
	end := i + 1
	for end < len(r.src) && r.src[end] != old {
		if r.src[end] == escapeMark {
			end++
		}
		end++
	}
	if end >= len(r.src) {
		return len(r.src)
	}
	content := r.src[i+1 : end]
	q := quoteOf(content, r.style)
	if q == old {
		return end + 1
	}
	if r.b.Len() == 0 {
		// An escape doubles a character at most, so the new spelling is
		// at most twice as long.
		r.b.Grow(2 * len(r.src))
	}
	r.b.WriteString(r.src[r.written:i])
	r.b.WriteByte(q)
	for j := 0; j < len(content); j++ {
		switch c := content[j]; {
		case c == escapeMark && content[j+1] == old:
			r.b.WriteByte(old)
			j++
		case c == escapeMark:
			r.b.WriteString(content[j : j+2])
			j++
		case c == q:
			r.b.WriteByte(escapeMark)
			r.b.WriteByte(c)
		default:
			r.b.WriteByte(c)
		}
	}
	r.b.WriteByte(q)
	r.written = end + 1
	return end + 1
}

// AppendQuote appends text to dst as a string literal of TypeScript and
// returns the extended slice. The literal has the quote that style
// selects: the single quote for [SingleQuote], and the double quote for
// every other choice. A text that contains more of that quote than of the
// other quote has the other quote, as Biome and Prettier choose, because
// its literal then needs fewer escapes.
//
// The literal escapes the backslash and its own quote, and writes the
// named escapes \b, \f, \n, \r, \t and \v. It writes every other character
// before the space, DEL and the two line separators as \u with four
// hexadecimal digits, and every other character as itself. A byte that is
// not part of a UTF-8 character becomes U+FFFD. [Unquote] reads text back
// from the literal of a text of UTF-8.
//
// AppendQuote allocates only where dst has no room for the literal.
func AppendQuote(dst []byte, text string, style plugin.Choice) []byte {
	q := quoteOf(text, style)
	dst = append(dst, q)
	for _, r := range text {
		switch r {
		case escapeMark:
			dst = append(dst, escapeMark, escapeMark)
		case rune(q):
			dst = append(dst, escapeMark, q)
		case '\b':
			dst = append(dst, escapeMark, 'b')
		case '\f':
			dst = append(dst, escapeMark, 'f')
		case '\n':
			dst = append(dst, escapeMark, 'n')
		case '\r':
			dst = append(dst, escapeMark, 'r')
		case '\t':
			dst = append(dst, escapeMark, 't')
		case '\v':
			dst = append(dst, escapeMark, 'v')
		default:
			if r < firstPrintable || r == deleteChar || r == lineSeparator || r == paragraphSeparator {
				var scratch [fixedDigits]byte
				digits := strconv.AppendInt(scratch[:0], int64(r), 16)
				dst = append(dst, escapeMark, unicodeMark)
				dst = append(dst, hexPadding[len(digits):]...)
				dst = append(dst, digits...)
				continue
			}
			dst = utf8.AppendRune(dst, r)
		}
	}
	return append(dst, q)
}

// Requote returns the spelling of a type with each of its string literals
// in the quote that [AppendQuote] chooses for the content of the literal
// under style. A quote that the new quote does not escape loses its
// backslash, and a quote that the new quote escapes gains one. Every other
// escape keeps its text, and so do the text of a template literal and the
// text outside a literal. A literal inside a substitution of a template
// literal changes its quote as well. A literal without its closing quote
// keeps its text.
//
// Requote returns spelling and allocates nothing where every literal has
// its quote already. It allocates the new spelling once otherwise.
func Requote(spelling string, style plugin.Choice) string {
	if !strings.ContainsAny(spelling, string(doubleMark)+string(singleMark)) {
		return spelling
	}
	r := requote{src: spelling, style: style}
	r.typ(0, false)
	if r.b.Len() == 0 {
		return spelling
	}
	r.b.WriteString(spelling[r.written:])
	return r.b.String()
}

// quoteOf returns the quote of the literal of a content under style: the
// quote that style selects, and the other quote where the content contains
// more of the selected quote than of the other quote.
func quoteOf(content string, style plugin.Choice) byte {
	doubles := strings.Count(content, string(doubleMark))
	singles := strings.Count(content, string(singleMark))
	if style == SingleQuote {
		if singles > doubles {
			return doubleMark
		}
		return singleMark
	}
	if doubles > singles {
		return singleMark
	}
	return doubleMark
}
