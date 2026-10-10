// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"golang.org/x/mod/module"
)

// parseSums adds the hashes one go.sum records to sums, keyed by
// module and version. A go.sum line is a module path, a version and a
// hash, separated by spaces, and a line of another shape is left out.
// The line that hashes a module's go.mod file alone spells its version
// with a /go.mod suffix, so its key is never a tree's. A hash already
// in sums is kept, so the first go.sum that records a module decides.
func parseSums(data []byte, sums map[module.Version]string) {
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		key := module.Version{Path: fields[0], Version: fields[1]}
		if _, met := sums[key]; !met {
			sums[key] = fields[2]
		}
	}
}
