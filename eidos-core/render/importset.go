// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"cmp"
	"maps"
	"slices"
)

// Entry is one collected import: the path a spelling qualified
// with, and the name it binds where the language's import form
// binds one. A bare path leaves Name empty, which is the
// side-effect or whole-namespace form; a bare and a named entry
// under one path stay two entries. TypeOnly marks a binding a
// language erases at run time — TypeScript's import type — and a
// value binding beside a type-only one under one path makes the
// whole import a value import, which is the renderer's join to
// make.
type Entry struct {
	Path     string
	Name     string
	TypeOnly bool
}

// ImportSet is one file's collected imports: every entry the
// file's spellings qualified with, deduplicated. The set is per
// file by rule, filled as a side effect of spelling, and the
// language's Imports renderer turns it into the block its own
// formatter would leave.
//
// An ImportSet is not safe for concurrent use: it belongs to the
// one file under render.
type ImportSet struct {
	entries map[Entry]struct{}
}

// Add records one bare import path; recording a path twice
// records one.
func (s *ImportSet) Add(path string) {
	s.AddNamed(path, "")
}

// AddNamed records the name an import of path binds; recording a
// pair twice records one.
func (s *ImportSet) AddNamed(path, name string) {
	if s.entries == nil {
		s.entries = map[Entry]struct{}{}
	}
	s.entries[Entry{Path: path, Name: name}] = struct{}{}
}

// AddType records a type-only binding: the name an import of path
// binds for the type checker alone, erased at run time where the
// language erases one.
func (s *ImportSet) AddType(path, name string) {
	if s.entries == nil {
		s.entries = map[Entry]struct{}{}
	}
	s.entries[Entry{Path: path, Name: name, TypeOnly: true}] = struct{}{}
}

// Paths returns every recorded path, distinct and sorted, so two
// renders spell one block. A renderer that binds no names reads
// nothing else.
func (s *ImportSet) Paths() []string {
	paths := make(map[string]struct{}, len(s.entries))
	for e := range s.entries {
		paths[e.Path] = struct{}{}
	}
	return slices.Sorted(maps.Keys(paths))
}

// Entries returns every recorded entry, sorted by path, name,
// then value before type-only, so two renders spell one block.
func (s *ImportSet) Entries() []Entry {
	entries := slices.Collect(maps.Keys(s.entries))
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Name, b.Name),
			boolCmp(a.TypeOnly, b.TypeOnly),
		)
	})
	return entries
}

// boolCmp orders false before true.
func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}

// Len returns how many distinct paths the set holds, which is
// what decides whether a block renders at all.
func (s *ImportSet) Len() int {
	paths := make(map[string]struct{}, len(s.entries))
	for e := range s.entries {
		paths[e.Path] = struct{}{}
	}
	return len(paths)
}

// Reset drops every entry and keeps the storage, so the pass
// reuses one set across a call's files.
func (s *ImportSet) Reset() { clear(s.entries) }
