// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package render

import (
	"maps"
	"slices"
)

// ImportSet is one file's collected imports: every path the file's
// spellings qualified with, deduplicated. The set is per file by
// rule, filled as a side effect of spelling, and the language's
// Imports renderer turns it into the block its own formatter would
// leave.
//
// An ImportSet is not safe for concurrent use: it belongs to the
// one file under render.
type ImportSet struct {
	paths map[string]struct{}
}

// Add records one import path; recording a path twice records one.
func (s *ImportSet) Add(path string) {
	if s.paths == nil {
		s.paths = map[string]struct{}{}
	}
	s.paths[path] = struct{}{}
}

// Paths returns every recorded path, sorted, so two renders spell
// one block.
func (s *ImportSet) Paths() []string {
	return slices.Sorted(maps.Keys(s.paths))
}

// Len returns how many paths the set holds.
func (s *ImportSet) Len() int { return len(s.paths) }

// Reset drops every path and keeps the storage, so the pass reuses
// one set across a call's files.
func (s *ImportSet) Reset() { clear(s.paths) }
