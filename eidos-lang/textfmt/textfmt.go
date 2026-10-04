// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"bytes"
)

// Normalize returns src without the whitespace a template leaves:
//
//   - It strips trailing whitespace from every line.
//   - It collapses each run of blank lines to one.
//   - It drops leading blank lines.
//   - It ends the text with exactly one newline.
//
// An empty input returns as it is. The error is always nil. It matches
// the formatter signature a backend's Finalise step takes.
//
// # Allocation contract
//
// Normalize allocates the output at the input's length, one
// allocation. An empty input allocates nothing.
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
