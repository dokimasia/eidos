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
// formatter would leave. An entry under the file's own package
// records nothing, because a file never imports its own package.
//
// An ImportSet is not safe for concurrent use: it belongs to the
// one file under render.
type ImportSet struct {
	entries map[Entry]struct{}
	// journal lists the entries in the order they were first
	// recorded, so the pass can withdraw what a skipped declaration
	// recorded before it failed.
	journal []Entry
	// home is the package path of the file under render.
	home string
}

// Home returns the package path of the file the set collects for,
// and empty where none is named.
func (s *ImportSet) Home() string { return s.home }

// SetHome names the package path of the file the set collects for.
// The render pass names it for every file; an entry under it
// records nothing.
func (s *ImportSet) SetHome(path string) { s.home = path }

// Add records one bare import path; recording a path twice
// records one.
func (s *ImportSet) Add(path string) {
	s.add(Entry{Path: path})
}

// AddNamed records the name an import of path binds; recording a
// pair twice records one.
func (s *ImportSet) AddNamed(path, name string) {
	s.add(Entry{Path: path, Name: name})
}

// AddType records a type-only binding: the name an import of path
// binds for the type checker alone, erased at run time where the
// language erases one.
func (s *ImportSet) AddType(path, name string) {
	s.add(Entry{Path: path, Name: name, TypeOnly: true})
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
func (s *ImportSet) Reset() {
	clear(s.entries)
	s.journal = s.journal[:0]
}

// add records one entry and journals it when it is new. An entry
// under the file's own package records nothing.
func (s *ImportSet) add(e Entry) {
	if s.home != "" && e.Path == s.home {
		return
	}
	if s.entries == nil {
		s.entries = map[Entry]struct{}{}
	}
	if _, held := s.entries[e]; held {
		return
	}
	s.entries[e] = struct{}{}
	s.journal = append(s.journal, e)
}

// mark returns the journal position a later rollback returns to.
func (s *ImportSet) mark() int { return len(s.journal) }

// rollback withdraws every entry first recorded after the mark.
func (s *ImportSet) rollback(mark int) {
	for _, e := range s.journal[mark:] {
		delete(s.entries, e)
	}
	s.journal = s.journal[:mark]
}
