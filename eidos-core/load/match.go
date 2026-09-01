// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"path"
	"strings"
)

// Match reports whether a pattern list claims a path.
//
// The grammar is the selection contract's: each pattern matches the
// whole workspace-relative slash path, "**" spans any number of
// segments, and one segment matches under [path.Match]'s rules. A
// "!" prefix negates. Patterns apply in order and the last match
// decides, so a claim can carve out what it does not own:
//
//	**/*.go
//	!**/testdata/**
//
// The workspace's Build refuses overlapping claims with the same
// grammar, which is why it is exported rather than the driver's
// own.
func Match(patterns []string, p string) bool {
	claimed := false
	segments := strings.Split(p, "/")
	for _, pattern := range patterns {
		want := true
		if rest, negated := strings.CutPrefix(pattern, "!"); negated {
			want, pattern = false, rest
		}
		if matchSegments(strings.Split(pattern, "/"), segments) {
			claimed = want
		}
	}
	return claimed
}

// matchSegments matches a split pattern against split path
// segments, "**" spanning zero or more of them.
func matchSegments(pattern, segments []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			rest := pattern[1:]
			for skip := 0; skip <= len(segments); skip++ {
				if matchSegments(rest, segments[skip:]) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 {
			return false
		}
		ok, err := path.Match(pattern[0], segments[0])
		if err != nil || !ok {
			return false
		}
		pattern, segments = pattern[1:], segments[1:]
	}
	return len(segments) == 0
}

// checkPattern refuses a pattern outside the grammar before
// anything matches against it: a claim that cannot be read must
// not quietly claim nothing.
func checkPattern(pattern string) error {
	trimmed := strings.TrimPrefix(pattern, "!")
	if trimmed == "" {
		return fmt.Errorf("load: the selection pattern %q claims nothing", pattern)
	}
	for segment := range strings.SplitSeq(trimmed, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, "probe"); err != nil {
			return fmt.Errorf("load: the selection pattern %q is outside the glob grammar", pattern)
		}
	}
	return nil
}
