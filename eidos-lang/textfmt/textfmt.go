// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"bytes"
)

// Normalize strips trailing whitespace from every line, collapses
// blank-line runs to one, drops leading blank lines, and ends the
// text with exactly one newline. An empty input returns empty.
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
