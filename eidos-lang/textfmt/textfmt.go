// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package textfmt normalizes rendered source for the targets that
// run no formatter of their own.
//
// [Normalize] is the finaliser a backend without a hermetic
// pretty-printer declares: it strips trailing whitespace,
// collapses runs of blank lines to one, and ends the file with
// exactly one newline, so template whitespace artifacts never
// reach a shipped file. It never reorders, rewraps or reindents
// anything: the bytes stay the template's, minus the noise.
//
// # Dependency position
//
// lang/textfmt imports the Go stdlib alone.
package textfmt

import (
	"bytes"
)

// Normalize strips trailing whitespace from every line, collapses
// blank-line runs to one, drops leading blank lines, and ends the
// text with exactly one newline. Empty input stays empty.
func Normalize(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return src, nil
	}

	var out bytes.Buffer
	out.Grow(len(src))
	blanks, wrote := 0, false
	for line := range bytes.Lines(src) {
		trimmed := bytes.TrimRight(line, " \t\r\n")
		if len(trimmed) == 0 {
			blanks++
			continue
		}
		if wrote && blanks > 0 {
			out.WriteByte('\n')
		}
		blanks = 0
		wrote = true
		out.Write(trimmed)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}
