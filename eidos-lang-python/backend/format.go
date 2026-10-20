// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/python"
)

// lineWidth is the width of a line of the layout of ruff's formatter, in
// characters.
const lineWidth = 88

// maxIndent is the number of levels of indentation that the tokenizer of
// CPython accepts. The layout keeps the last statement of each level.
const maxIndent = 100

// The marks of a bracket group, which the vocabulary writes on one line
// and [Layout.Format] lays out. A group opens after its bracket and closes
// before the closing bracket. Its items are the parameters of a signature,
// the bases of a class, the arguments of a decorator and the elements of a
// literal. No mark is a character of Python source.
const (
	// hugOpen opens a group whose items share one indented line where the
	// whole line is too long, as the parameters of a signature do.
	hugOpen = "\x01"
	// listOpen opens a group of the elements of a literal, which wraps to
	// one element on each line, as __all__ does.
	listOpen = "\x02"
	// itemSep separates two items of a group.
	itemSep = "\x03"
	// groupClose closes a group.
	groupClose = "\x04"
	// commentMark separates an item from its trailing comment. A group with
	// a comment wraps to one item on each line, because a comment ends its
	// line.
	commentMark = "\x05"
)

// The parts of a wrapped group: the separator of the items on one line,
// the comma after each item on a line of its own, and the space and the
// mark before the comment of an item.
const (
	itemsSep     = ", "
	itemEnd      = ","
	commentSpace = "  "
	commentLead  = "# "
)

// The marks of the lines that the layout reads: the start of a comment, the
// opener of a block, the openers of a definition, the closing brackets that
// continue a statement at its own indentation, and the words that open an
// import statement.
const (
	commentOpen = '#'
	blockOpener = ":"
	decoratorAt = "@"
	classWord   = "class "
	defWord     = "def "
	asyncWord   = "async def "
	closers     = ")]}"
	importStart = python.ImportWord + space
	fromStart   = python.FromWord + space
)

// The quotes of a string. A triple quote opens a string that can span
// lines, a single quote character opens a string that ends on its line, and
// the backslash escapes the next character.
const (
	tripleDouble = `"""`
	tripleSingle = `'''`
	doubleChar   = '"'
	singleChar   = '\''
	escape       = '\\'
)

// itemKind is what a line of a statement starts.
type itemKind uint8

const (
	// simpleItem is a statement that is not a definition.
	simpleItem itemKind = iota
	// definitionItem is a class or a function, or the decorator of one.
	definitionItem
	// continuedItem is the closing bracket of a statement, which continues
	// the item before it.
	continuedItem
)

// item is one statement of a block, as the layout separates it from the
// statement before it in the same block.
type item struct {
	kind itemKind
	// class reports that the definition is a class, and ellipsis that its
	// body is the ellipsis on its head line.
	class    bool
	ellipsis bool
	// decorated reports that a decorator starts the definition, and headless
	// that the layout has not reached its head line yet.
	decorated bool
	headless  bool
	// imports reports that the statement is an import statement.
	imports bool
	// docstring reports that the statement is the docstring of a module or
	// of a class.
	docstring bool
}

// Layout is the layout of a Python file that ruff's formatter writes: the
// layout of a module, or of a stub file, a file with the extension .pyi,
// where Stub is true. The zero Layout is the layout of a module.
type Layout struct {
	Stub bool
}

// Format lays out a rendered Python file as ruff's formatter lays out the
// lines that the templates write:
//
//   - a bracket group that the vocabulary marked stays on one line where
//     the line has at most 88 characters. A signature, the bases of a class
//     and the arguments of a decorator then put their items on one indented
//     line where it fits, and every group otherwise puts one item on each
//     line with a trailing comma. A group of the elements of a literal and a
//     group with a comment put one item on each line at once;
//   - in a module, two blank lines separate a definition at the top level
//     from the statement before it and from the statement after it, and one
//     blank line separates two other statements that the templates
//     separate;
//   - in a stub file, one blank line separates a definition at the top level
//     from its neighbours, and a definition inside a class keeps the blank
//     line that the templates write before it. No blank line separates two
//     functions where the first has the body ... on its head line, two such
//     classes where the second has no decorator, or a function and the
//     decorated class after it. A blank line always follows a class whose
//     body is more than ..., a decorated class with the body ..., and such a
//     class before a function or before a class that is decorated or has a
//     longer body. One blank line follows the imports, and the docstring of
//     the module and of a class;
//   - inside a block, a run of blank lines becomes one blank line, and the
//     first line of a block follows its opener without a blank line;
//   - trailing whitespace goes, the file starts with its first line that is
//     not blank, and the file ends with one newline.
//
// Format leaves every line inside a string that spans lines as it is. The
// error is always nil. It matches the formatter signature that the
// Finalise step of the backend kit takes.
//
// # Allocation contract
//
// Format allocates the output once, at a bound of its length that it
// computes from the input. An empty input allocates nothing.
func (y Layout) Format(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return src, nil
	}
	l := layout{out: make([]byte, 0, outputBound(src)), src: src, stub: y.Stub}
	for start := 0; start < len(src); {
		end := bytes.IndexByte(src[start:], '\n')
		if end < 0 {
			end = len(src) - start
		}
		l.next = start + end + 1
		l.line(src[start : start+end])
		start = l.next
	}
	return l.out, nil
}

// outputBound returns a bound of the length of the layout of src: the
// input, two blank lines before each line, the newline after the last
// line, and for each group of a line the indentation, the comma and the
// newline of each item on a line of its own and of its closing bracket, and
// the spaces and the hash of each comment of an item.
func outputBound(src []byte) int {
	n := len(src) + len(newline)
	for rest := src; len(rest) > 0; {
		end := bytes.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest)
		}
		line := rest[:end]
		rest = rest[min(end+1, len(rest)):]
		n += 2 * len(newline)
		open := bytes.IndexAny(line, hugOpen+listOpen)
		if open < 0 {
			continue
		}
		indent := len(line) - len(bytes.TrimLeft(line, " \t"))
		lines := bytes.Count(line[open:], []byte(itemSep)) + 2
		comments := bytes.Count(line[open:], []byte(commentMark))
		n += lines*(indent+len(python.Indent)+len(itemEnd)+len(newline)) +
			comments*(len(commentSpace)+len(commentLead))
	}
	return n
}

// layout is the state of [Layout.Format] between two lines: the output, the
// input and the offset of the line after the current one, the run of blank
// lines that waits for the next line, the string that a line left open, and
// the statements of the open blocks.
type layout struct {
	out []byte
	src []byte
	// quote is the triple quote of the string that the last line left open,
	// and empty outside a string.
	quote string
	next  int
	// blanks counts the blank lines since the last line that is not blank.
	blanks int
	// open counts the depths of items that hold the last statement of an
	// open block.
	open  int
	items [maxIndent]item
	stub  bool
	// wrote reports that the output has a line, opened that the last line
	// opens a block, class that the block is the body of a class, and code
	// that the output has a statement at the top level.
	wrote  bool
	opened bool
	class  bool
	code   bool
}

// line lays out one line of the input without its newline.
func (l *layout) line(text []byte) {
	if l.quote != "" {
		l.inString(text)
		return
	}
	quote := openQuote(text, "")
	if quote == "" {
		text = bytes.TrimRight(text, " \t\r")
	}
	if len(text) == 0 {
		l.blanks++
		return
	}
	indent := len(text) - len(bytes.TrimLeft(text, " \t"))
	code := codeOf(text)
	stmt := code[min(indent, len(code)):]
	l.separate(stmt, indent)
	l.wrap(text)
	l.out = append(l.out, '\n')
	l.quote = quote
	l.opened = bytes.HasSuffix(stmt, []byte(blockOpener))
	l.class = l.opened && bytes.HasPrefix(stmt, []byte(classWord))
	l.code = l.code || indent == 0 && len(stmt) > 0
}

// inString writes a line inside a string as it is. A blank line inside a
// string is a line of the string, so the layout counts no blank lines there.
func (l *layout) inString(text []byte) {
	l.out = append(l.out, text...)
	l.out = append(l.out, '\n')
	l.quote = openQuote(text, l.quote)
}

// separate writes the blank lines before a line that is not blank by the
// rules of [Layout.Format], and records the statement that the line starts
// or continues. stmt is the code of the line without its indentation, which
// is indent bytes wide, and it is empty for a line of a comment alone.
func (l *layout) separate(stmt []byte, indent int) {
	run := l.blanks
	l.blanks = 0
	depth := min((indent+len(python.Indent)-1)/len(python.Indent), maxIndent-1)
	if l.opened {
		l.open = min(depth, l.open)
	} else {
		l.open = min(depth+1, l.open)
	}
	var p item
	if depth < l.open {
		p = l.items[depth]
	}
	f := itemOf(stmt)
	switch {
	case f.kind == continuedItem:
		return
	case p.headless:
		if f.kind != definitionItem || !f.headless {
			l.items[depth].class, l.items[depth].ellipsis, l.items[depth].headless = f.class, f.ellipsis, false
		}
		return
	case f.headless:
		head := itemOf(codeOf(l.head(indent)))
		f.class, f.ellipsis = head.class, head.ellipsis
	}
	f.docstring = (l.opened && l.class || indent == 0 && !l.code) &&
		(bytes.HasPrefix(stmt, []byte(docQuotes)) || bytes.HasPrefix(stmt, []byte(rawPrefix+docQuotes)))
	n := 0
	switch {
	case !l.wrote, l.opened:
	case l.stub:
		n = stubBlanks(p, f, depth == 0, run)
	default:
		n = moduleBlanks(p, f, depth == 0, run)
	}
	for range n {
		l.out = append(l.out, '\n')
	}
	if l.open < depth {
		clear(l.items[l.open:depth])
	}
	l.items[depth] = f
	l.open = depth + 1
	l.wrote = true
}

// head returns the code of the head line of the definition that a decorator
// at the indentation indent starts: the first line after the decorator that
// is no decorator. It returns nil where the input ends before the head.
func (l *layout) head(indent int) []byte {
	rest := l.src[l.next:]
	for len(rest) > 0 {
		end := bytes.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest)
		}
		line := rest[:end]
		rest = rest[min(end+1, len(rest)):]
		if len(line) > indent && !bytes.HasPrefix(line[indent:], []byte(decoratorAt)) {
			return line[indent:]
		}
	}
	return nil
}

// moduleBlanks returns the number of blank lines before the statement f of a
// module after the statement p of its block, where the templates wrote run
// blank lines between them: two around a definition at the top level, one
// between two other statements that the templates separate, and none
// between two statements that the templates do not separate.
func moduleBlanks(p, f item, top bool, run int) int {
	switch {
	case run == 0:
		return 0
	case top && (f.kind == definitionItem || p.kind == definitionItem):
		return 2
	default:
		return 1
	}
}

// stubBlanks returns the number of blank lines before the statement f of a
// stub file after the statement p of its block, where the templates wrote
// run blank lines between them, by the rules of the stub files of ruff's
// formatter.
func stubBlanks(p, f item, top bool, run int) int {
	switch {
	case f.kind == definitionItem || p.kind == definitionItem:
		if requires(p, f) || !omits(p, f) && (top || run > 0) {
			return 1
		}
		return 0
	case p.imports && !f.imports, p.docstring, run > 0:
		return 1
	default:
		return 0
	}
}

// omits reports whether a stub file writes no blank line between the
// statements p and f where a definition is one of them: two functions where
// p has the body ..., two classes with the body ... where f has no
// decorator, and a function before a decorated class.
func omits(p, f item) bool {
	pFunction := p.kind == definitionItem && !p.class
	fFunction := f.kind == definitionItem && !f.class
	return p.class && p.ellipsis && f.class && f.ellipsis && !f.decorated ||
		pFunction && f.class && f.decorated ||
		pFunction && p.ellipsis && fFunction
}

// requires reports whether a stub file writes a blank line after the class
// p and before the statement f: after a class whose body is more than ...,
// and after a class with the body ... that is decorated, or before a
// function, or before a class that is decorated or has a longer body.
func requires(p, f item) bool {
	switch {
	case !p.class:
		return false
	case !p.ellipsis:
		return true
	default:
		return p.decorated || f.class && (!f.ellipsis || f.decorated) || f.kind == definitionItem && !f.class
	}
}

// itemOf returns the item that a statement starts: a continuation for a
// closing bracket, a definition for a decorator, a class and a function,
// with the class, the body ... and the decorator of a definition, and a
// simple statement otherwise, with whether it is an import statement. stmt
// is the code of the line without its indentation.
func itemOf(stmt []byte) item {
	switch {
	case len(stmt) == 0:
		return item{kind: simpleItem}
	case strings.IndexByte(closers, stmt[0]) >= 0:
		return item{kind: continuedItem}
	case bytes.HasPrefix(stmt, []byte(decoratorAt)):
		return item{kind: definitionItem, decorated: true, headless: true}
	case bytes.HasPrefix(stmt, []byte(classWord)):
		return item{kind: definitionItem, class: true, ellipsis: bytes.HasSuffix(stmt, []byte(inlineBody))}
	case bytes.HasPrefix(stmt, []byte(defWord)), bytes.HasPrefix(stmt, []byte(asyncWord)):
		return item{kind: definitionItem, ellipsis: bytes.HasSuffix(stmt, []byte(inlineBody))}
	default:
		if lazy, found := bytes.CutPrefix(stmt, []byte(lazyWord)); found {
			stmt = lazy
		}
		return item{
			kind:    simpleItem,
			imports: bytes.HasPrefix(stmt, []byte(importStart)) || bytes.HasPrefix(stmt, []byte(fromStart)),
		}
	}
}

// wrap writes one line of the input: the line itself where it has no group,
// and the layout of its group otherwise. A line has at most one group,
// which a helper of the vocabulary marked.
func (l *layout) wrap(text []byte) {
	open := bytes.IndexAny(text, hugOpen+listOpen)
	if open < 0 {
		l.out = append(l.out, text...)
		return
	}
	hug := text[open] == hugOpen[0]
	prefix, body := text[:open], text[open+1:]
	var suffix []byte
	if end := bytes.IndexByte(body, groupClose[0]); end >= 0 {
		body, suffix = body[:end], body[end+1:]
	}
	seps := bytes.Count(body, []byte(itemSep))
	commented := bytes.IndexByte(body, commentMark[0]) >= 0
	width := utf8.RuneCount(body) + seps
	if !commented && utf8.RuneCount(prefix)+width+utf8.RuneCount(suffix) <= lineWidth {
		l.out = append(l.out, prefix...)
		l.writeItems(body)
		l.out = append(l.out, suffix...)
		return
	}
	indent := text[:len(text)-len(bytes.TrimLeft(text, " "))]
	if hug && !commented && len(indent)+len(python.Indent)+width <= lineWidth {
		l.out = append(l.out, prefix...)
		l.out = append(l.out, '\n')
		l.out = append(l.out, indent...)
		l.out = append(l.out, python.Indent...)
		l.writeItems(body)
		l.out = append(l.out, '\n')
		l.out = append(l.out, indent...)
		l.out = append(l.out, suffix...)
		return
	}
	l.out = append(l.out, prefix...)
	for range seps + 1 {
		entry := body
		if end := bytes.IndexByte(body, itemSep[0]); end >= 0 {
			entry, body = body[:end], body[end+1:]
		}
		code, comment, has := bytes.Cut(entry, []byte(commentMark))
		l.out = append(l.out, '\n')
		l.out = append(l.out, indent...)
		l.out = append(l.out, python.Indent...)
		l.out = append(l.out, code...)
		l.out = append(l.out, itemEnd...)
		if has {
			l.out = append(l.out, commentSpace...)
			l.out = append(l.out, commentLead...)
			l.out = append(l.out, comment...)
		}
	}
	l.out = append(l.out, '\n')
	l.out = append(l.out, indent...)
	l.out = append(l.out, suffix...)
}

// writeItems writes the items of a group on one line, separated by commas.
func (l *layout) writeItems(body []byte) {
	for {
		end := bytes.IndexByte(body, itemSep[0])
		if end < 0 {
			l.out = append(l.out, body...)
			return
		}
		l.out = append(l.out, body[:end]...)
		l.out = append(l.out, itemsSep...)
		body = body[end+1:]
	}
}

// codeOf returns the code of a line before its comment, without the
// whitespace after the code. A # inside a string on the line is no
// comment, and neither is the text of the comment of an item of a group.
func codeOf(text []byte) []byte {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case doubleChar, singleChar:
			i = closeSingle(text, i)
		case commentMark[0]:
			i = itemEndOf(text, i)
		case commentOpen:
			return bytes.TrimRight(text[:i], " \t\r")
		}
	}
	return bytes.TrimRight(text, " \t\r")
}

// openQuote returns the triple quote of the string that a line leaves open,
// and the empty string where it leaves none. quote is the triple quote of
// the string that the line starts inside, and empty where it starts
// outside a string. A string of one quote ends on its line, and a # outside
// a string starts a comment, which ends the line. The text of the comment
// of an item of a group is no code.
func openQuote(text []byte, quote string) string {
	for i := 0; i < len(text); i++ {
		switch {
		case quote != "" && text[i] == escape:
			i++
		case quote != "":
			if bytes.HasPrefix(text[i:], []byte(quote)) {
				i += len(quote) - 1
				quote = ""
			}
		case bytes.HasPrefix(text[i:], []byte(tripleDouble)):
			quote = tripleDouble
			i += len(quote) - 1
		case bytes.HasPrefix(text[i:], []byte(tripleSingle)):
			quote = tripleSingle
			i += len(quote) - 1
		case text[i] == doubleChar || text[i] == singleChar:
			i = closeSingle(text, i)
		case text[i] == commentMark[0]:
			i = itemEndOf(text, i)
		case text[i] == commentOpen:
			return ""
		}
	}
	return quote
}

// closeSingle returns the index of the quote that closes the string of one
// quote that starts at the index start of text, and the last index of text
// where the string does not close on the line.
func closeSingle(text []byte, start int) int {
	q := text[start]
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case escape:
			i++
		case q:
			return i
		}
	}
	return len(text) - 1
}

// itemEndOf returns the index of the last character of the comment of an
// item of a group that starts at the index start of text: the character
// before the mark that ends the item, and the last index of text where no
// mark follows.
func itemEndOf(text []byte, start int) int {
	for i := start + 1; i < len(text); i++ {
		if text[i] == itemSep[0] || text[i] == groupClose[0] {
			return i - 1
		}
	}
	return len(text) - 1
}
