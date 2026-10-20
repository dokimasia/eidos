// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"strings"

	"go.dokimi.dev/eidos/sdk/position"
)

// The marks of the head of a file that typescript-go reads directives from:
// the byte order mark and the shebang that can open a file, the openings of
// a line comment, of a directive comment and of a block comment, and the end
// of a block comment.
const (
	byteOrderMark    = "\xef\xbb\xbf"
	shebang          = "#!"
	lineComment      = "//"
	directiveComment = "///"
	blockOpen        = "/*"
	blockClose       = "*/"
)

// The bytes between the comments of the head of a file, the bytes that end
// a line, and the line feed that the column of a position counts from.
const (
	spaceBytes = " \t\n\r\v\f"
	lineBreaks = "\r\n"
	lineFeed   = '\n'
)

// The parts of a reference directive: its opening, its tag, its end, the
// separator of an argument and its value, the arguments that decide what
// it refers to, and the value of a flag that is set.
const (
	directiveOpen        = "<"
	referenceTag         = "reference"
	directiveClose       = "/>"
	argumentAssign       = "="
	libArgument          = "lib"
	typesArgument        = "types"
	noDefaultLibArgument = "no-default-lib"
	trueValue            = "true"
)

// libReference is one /// <reference lib="…" /> directive at the head of a
// file: the name of the library that it refers to, and the position of its
// comment.
type libReference struct {
	name string
	pos  position.Pos
}

// libReferences returns the lib directives at the head of the source src of
// the file path, in order. The head of a file is its comments before its
// first token, after a byte order mark and a shebang line. libReferences
// reads each directive as [libDirective] reads it. It allocates the list of
// the directives and the name of each.
func libReferences(path string, src []byte) []libReference {
	var out []libReference
	at := 0
	if bytes.HasPrefix(src, []byte(byteOrderMark)) {
		at = len(byteOrderMark)
	}
	if bytes.HasPrefix(src[at:], []byte(shebang)) {
		at = lineEnd(src, at)
	}
	for {
		at = skipSpace(src, at)
		rest := src[at:]
		switch {
		case bytes.HasPrefix(rest, []byte(lineComment)):
			end := lineEnd(src, at)
			if name, refers := libDirective(src[at:end]); refers {
				out = append(out, libReference{name: name, pos: positionOf(path, src, at)})
			}
			at = end
		case bytes.HasPrefix(rest, []byte(blockOpen)):
			end := bytes.Index(rest[len(blockOpen):], []byte(blockClose))
			if end < 0 {
				return out
			}
			at += len(blockOpen) + end + len(blockClose)
		default:
			return out
		}
	}
}

// libDirective returns the library that the line comment c refers to, as
// extractPragmas and processPragmasIntoFields of typescript-go 7.0.2 read a
// comment. A directive is a /// comment whose tag is reference, followed by
// arguments of the form name="value" or name='value', up to the first
// argument that does not read. The tag and the names of the arguments read
// without regard to case. libDirective reports false for any other comment,
// and for a directive with no-default-lib="true", with a types argument, or
// without a lib argument, because none of those refers to a library. It
// allocates the name that it returns.
func libDirective(c []byte) (string, bool) {
	if !bytes.HasPrefix(c, []byte(directiveComment)) {
		return "", false
	}
	at := skipBlanks(c, len(directiveComment))
	if !bytes.HasPrefix(c[at:], []byte(directiveOpen)) ||
		!bytes.EqualFold(nameAt(c, at+len(directiveOpen)), []byte(referenceTag)) {

		return "", false
	}
	at += len(directiveOpen) + len(referenceTag)
	var (
		lib                         []byte
		hasLib, hasTypes, noDefault bool
	)
	for {
		at = skipBlanks(c, at)
		if bytes.HasPrefix(c[at:], []byte(directiveClose)) {
			break
		}
		name := nameAt(c, at)
		if len(name) == 0 {
			break
		}
		at = skipBlanks(c, at+len(name))
		if !bytes.HasPrefix(c[at:], []byte(argumentAssign)) {
			break
		}
		at = skipBlanks(c, at+len(argumentAssign))
		value, quoted := quotedAt(c, at)
		if !quoted {
			break
		}
		switch {
		case bytes.EqualFold(name, []byte(libArgument)):
			lib, hasLib = value, true
		case bytes.EqualFold(name, []byte(typesArgument)):
			hasTypes = true
		case bytes.EqualFold(name, []byte(noDefaultLibArgument)):
			noDefault = string(value) == trueValue
		}
		at += len(value) + 2
	}
	if noDefault || hasTypes || !hasLib {
		return "", false
	}
	return string(lib), true
}

// skipSpace returns the offset of the first byte at or after at in src that
// is no space, tab, line feed, carriage return, vertical tab or form feed.
func skipSpace(src []byte, at int) int {
	for at < len(src) && strings.IndexByte(spaceBytes, src[at]) >= 0 {
		at++
	}
	return at
}

// skipBlanks returns the offset of the first byte at or after at in c that
// is no space or tab, as skipBlanks of typescript-go returns it.
func skipBlanks(c []byte, at int) int {
	for at < len(c) && (c[at] == ' ' || c[at] == '\t') {
		at++
	}
	return at
}

// lineEnd returns the offset of the line feed or the carriage return that
// ends the line of src that contains the offset at. It returns the length
// of src for the last line.
func lineEnd(src []byte, at int) int {
	end := bytes.IndexAny(src[at:], lineBreaks)
	if end < 0 {
		return len(src)
	}
	return at + end
}

// nameAt returns the name at the offset at of c: the letters and the
// hyphens there, as extractName of typescript-go reads them, and nothing
// past the end of c.
func nameAt(c []byte, at int) []byte {
	start := min(at, len(c))
	end := start
	for end < len(c) && (c[end] >= 'a' && c[end] <= 'z' || c[end] >= 'A' && c[end] <= 'Z' || c[end] == '-') {
		end++
	}
	return c[start:end]
}

// quotedAt returns the value of the quoted string at the offset at of c,
// between a single or a double quote and the next quote of the same kind.
// It reports false where no quote opens at the offset at, and where no quote
// closes the value.
func quotedAt(c []byte, at int) ([]byte, bool) {
	if at >= len(c) || c[at] != '"' && c[at] != '\'' {
		return nil, false
	}
	end := bytes.IndexByte(c[at+1:], c[at])
	if end < 0 {
		return nil, false
	}
	return c[at+1 : at+1+end], true
}

// positionOf returns the position of the byte at the offset at of the source
// src of the file path: its line and its column, each from one, the column
// counted in bytes as the grammar counts it.
func positionOf(path string, src []byte, at int) position.Pos {
	line := 1 + bytes.Count(src[:at], []byte{lineFeed})
	start := bytes.LastIndexByte(src[:at], lineFeed) + 1
	return position.Pos{File: path, Line: line, Col: at - start + 1}
}
