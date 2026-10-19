// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf/internal/parse"
)

// checkPath is the path that the parser of a written file reports
// positions in. The render reports a refusal at the path of the file, so
// the parser needs no real path.
const checkPath = "written.proto"

// Check is the finalising step of a written file. The templates write the
// layout of buf format, so Check formats nothing: it parses the file with
// the parser of protocompile and returns the file as it is. A file of
// edition 2026 parses from a copy that states 2024, because the parser knows
// the editions up to 2024.
//
// Error modes: Check returns an error that states the line and the message
// of the first error of the parser, for a file that does not parse and for a
// construct that the version of the file does not have, such as the label
// required in proto3. The render withholds such a file.
//
// # Allocation contract
//
// Check allocates the copy of the file that the parser reads and the parse.
// A file of edition 2026 parses a second copy. A refusal allocates its
// error.
func Check(src []byte) ([]byte, error) {
	text := string(src)
	_, r, _ := parse.File(checkPath, text)
	if len(r.Diagnostics) > 0 {
		// The parse suppresses warnings, so each diagnostic is an error.
		d := &r.Diagnostics[0]
		line := 1 + strings.Count(text[:d.Primary().Start], "\n")
		return nil, fmt.Errorf(refusalPrefix+"the written file does not parse at line %d: %s", line, d.Message())
	}
	return src, nil
}
