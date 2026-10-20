// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"iter"
	"strings"
	"unicode/utf8"
)

// DocsWidth is the width in columns at which the backends wrap
// documentation. A comment marker and one level of indent keep a line of
// 72 columns of text below 80 columns.
const DocsWidth = 72

// The parts of a text that [Wrap] reads. asciiSpace is the set of bytes
// that separate two words, and decimalDigits the digits of an ordinal.
const (
	asciiSpace    = " \t\n\r\v\f"
	decimalDigits = "0123456789"
)

// The limits of the block openers of CommonMark that [opens] reads: the
// most number signs of a heading, the most digits of an ordinal, and the
// fewest asterisks or underscores of a thematic break.
const (
	headingSigns  = 6
	ordinalDigits = 9
	breakMarks    = 3
)

// Markup selects the words that a documentation syntax reads as the start
// of a block where they open a line. A target passes the markup of its
// documentation to [Wrap], which keeps such a word on the line before it.
// The values combine with |.
type Markup uint8

const (
	// MarkupNone reads no word as the start of a block. A Go and a
	// protobuf comment read an unindented line as text.
	MarkupNone Markup = 0
	// MarkupTags reads a word that starts with @ as a block tag, as TSDoc
	// and Javadoc do.
	MarkupTags Markup = 1
	// MarkupMarkdown reads the block openers of CommonMark: a heading of
	// one to six number signs, a word that starts with > or <, a plus sign
	// or an asterisk alone, an ordinal of up to nine digits with a period or
	// a parenthesis, a run of dashes or equals signs, a run of three or
	// more asterisks or underscores, and a word that starts with three
	// backticks or three tildes.
	MarkupMarkdown Markup = 2
)

// Wrap returns the lines of text wrapped at width columns. A blank line of
// text separates two paragraphs, and an empty line separates them in the
// result. Text without a word returns nil.
//
// The rules of the wrap:
//   - A run of spaces, tabs and line breaks inside a paragraph is one
//     space.
//   - A line takes each next word that keeps it at the width or below. The
//     width counts runes.
//   - A word longer than the width is a line of its own, because the wrap
//     never breaks a word.
//   - A word that m reads as the start of a block never opens a line after
//     the first line of its paragraph. The wrap keeps it on the line before
//     it, past the width where it must.
//   - A width below one wraps nothing, so each paragraph is one line.
//
// # Allocation contract
//
// Wrap counts the lines first, so it allocates the list of lines once. A
// line whose words one space separates in text is a part of text, and a
// separator is the empty string, so neither allocates. Wrap allocates each
// other line once.
func Wrap(text string, width int, m Markup) []string {
	w := wrapper{width: width, markup: m}
	w.paragraphs(text)
	if w.count == 0 {
		return nil
	}
	w.lines = make([]string, 0, w.count)
	w.started = false
	w.paragraphs(text)
	return w.lines
}

// wrapper is the state of one pass of [Wrap]. The first pass counts the
// lines into count, and the second pass appends them to lines, which the
// first pass leaves nil.
type wrapper struct {
	lines []string
	width int
	count int
	// started reports that a paragraph wrapped before, so the next one
	// starts after an empty line.
	started bool
	markup  Markup
}

// paragraphs wraps each paragraph of text in order. A paragraph is a run
// of lines that are not blank.
func (w *wrapper) paragraphs(text string) {
	start, end, offset := -1, 0, 0
	for line := range strings.Lines(text) {
		switch {
		case strings.TrimSpace(line) == "":
			if start >= 0 {
				w.paragraph(text[start:end])
				start = -1
			}
		case start < 0:
			start, end = offset, offset+len(line)
		default:
			end = offset + len(line)
		}
		offset += len(line)
	}
	if start >= 0 {
		w.paragraph(text[start:end])
	}
}

// paragraph wraps the words of one paragraph p, after an empty line where
// a paragraph wrapped before. A line is the span of p from its first word
// to its last, and clean reports that one space separates each two of its
// words.
func (w *wrapper) paragraph(p string) {
	if w.started {
		w.line("", true)
	}
	start, end, runes, clean := -1, 0, 0, true
	for i, word := range words(p) {
		j, n := i+len(word), utf8.RuneCountInString(word)
		switch {
		case start < 0:
			start, end, runes = i, j, n
		case w.width < 1 || runes+1+n <= w.width || opens(word, w.markup):
			clean = clean && p[end:i] == " "
			end, runes = j, runes+1+n
		default:
			w.line(p[start:end], clean)
			start, end, runes, clean = i, j, n, true
		}
	}
	if start >= 0 {
		w.line(p[start:end], clean)
	}
	w.started = true
}

// line counts one line in the first pass and appends it in the second. A
// line that is not clean takes one space between each two of its words,
// in one buffer of the size of the line.
func (w *wrapper) line(s string, clean bool) {
	if w.lines == nil {
		w.count++
		return
	}
	if !clean {
		var b strings.Builder
		b.Grow(len(s))
		for _, word := range words(s) {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(word)
		}
		s = b.String()
	}
	w.lines = append(w.lines, s)
}

// words yields each word of s with its offset in s. A run of the bytes of
// asciiSpace separates two words, so a space that is not ASCII, such as a
// no-break space, is part of a word.
func words(s string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		start := -1
		for i := 0; i <= len(s); i++ {
			if i < len(s) && strings.IndexByte(asciiSpace, s[i]) < 0 {
				if start < 0 {
					start = i
				}
				continue
			}
			if start >= 0 {
				if !yield(start, s[start:i]) {
					return
				}
				start = -1
			}
		}
	}
}

// opens reports whether a line that starts with word starts a block under
// the markup m. word is not empty.
func opens(word string, m Markup) bool {
	if m&MarkupTags != 0 && word[0] == '@' {
		return true
	}
	if m&MarkupMarkdown == 0 {
		return false
	}
	switch {
	case word == "+" || word == "*":
		// A bullet list item.
		return true
	case word[0] == '>' || word[0] == '<':
		// A block quote, and an HTML block.
		return true
	case strings.HasPrefix(word, "```") || strings.HasPrefix(word, "~~~"):
		// A fenced code block.
		return true
	case len(word) <= headingSigns && strings.Trim(word, "#") == "":
		// An ATX heading.
		return true
	case strings.Trim(word, "-") == "" || strings.Trim(word, "=") == "":
		// A bullet list item, a thematic break, and the underline of a
		// setext heading.
		return true
	case len(word) >= breakMarks && (strings.Trim(word, "*") == "" || strings.Trim(word, "_") == ""):
		// A thematic break.
		return true
	}
	last, digits := word[len(word)-1], word[:len(word)-1]
	return (last == '.' || last == ')') && digits != "" && len(digits) <= ordinalDigits &&
		strings.Trim(digits, decimalDigits) == ""
}
