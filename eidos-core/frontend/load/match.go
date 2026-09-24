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
// It is exported for the conformance suite, which selects a
// fixture's files with the grammar the load claims them with.
func Match(patterns []string, p string) bool {
	return compile(patterns).claims(split(nil, p))
}

// matcher is one claim compiled for matching: each pattern split
// into its segments beside its polarity, so a claim over a whole
// tree splits each pattern once.
type matcher []matchPattern

// matchPattern is one split pattern and whether it claims or
// carves.
type matchPattern struct {
	segments []string
	claims   bool
}

// compile splits every pattern of a claim once.
func compile(patterns []string) matcher {
	out := make(matcher, len(patterns))
	for i, pattern := range patterns {
		claims := true
		if rest, negated := strings.CutPrefix(pattern, "!"); negated {
			claims, pattern = false, rest
		}
		out[i] = matchPattern{segments: strings.Split(pattern, "/"), claims: claims}
	}
	return out
}

// claims reports whether the claim covers a path split into its
// segments: the last matching pattern decides.
func (m matcher) claims(segments []string) bool {
	claimed := false
	for _, p := range m {
		if matchSegments(p.segments, segments) {
			claimed = p.claims
		}
	}
	return claimed
}

// split appends the slash-separated segments of p to dst[:0], so a
// caller matching many paths reuses one buffer.
func split(dst []string, p string) []string {
	dst = dst[:0]
	for {
		at := strings.IndexByte(p, '/')
		if at < 0 {
			return append(dst, p)
		}
		dst = append(dst, p[:at])
		p = p[at+1:]
	}
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
// not quietly claim nothing. An empty, "." or ".." segment is
// outside it too, because no workspace-relative path contains one:
// a leading "/" never anchors, it claims nothing.
func checkPattern(pattern string) error {
	trimmed := strings.TrimPrefix(pattern, "!")
	if trimmed == "" {
		return fmt.Errorf("load: the selection pattern %q claims nothing", pattern)
	}
	for segment := range strings.SplitSeq(trimmed, "/") {
		switch segment {
		case "**":
			continue
		case "", ".", "..":
			return fmt.Errorf(
				"load: the selection pattern %q has an empty, . or .. segment, "+
					"which no workspace-relative path contains", pattern,
			)
		}
		if _, err := path.Match(segment, "probe"); err != nil {
			return fmt.Errorf("load: the selection pattern %q is outside the glob grammar", pattern)
		}
	}
	return nil
}
