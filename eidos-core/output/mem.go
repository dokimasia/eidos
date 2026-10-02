// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"bytes"
	"maps"
	"slices"
)

// Mem stages in memory and commits into a map, for tests and for
// a dry run that computes everything and writes nowhere. It follows
// the same staging rule as the disk sink: nothing is readable
// until the commit. Its destination starts empty, so every file it
// commits is created and every staged removal finds nothing.
type Mem struct {
	staging
	committed map[string][]byte
}

var _ Sink = (*Mem)(nil)

// NewMem opens a sink over memory.
func NewMem() *Mem { return &Mem{committed: map[string][]byte{}} }

// Write stages one file.
func (m *Mem) Write(path string, body []byte) error { return m.stage(path, body) }

// Delete stages the removal of one file, which the empty destination
// does not contain.
func (m *Mem) Delete(path string) error { return m.remove(path) }

// Prepare reports every staged path against the empty destination:
// each write creates its file, and each removal finds nothing.
func (m *Mem) Prepare() ([]Change, error) {
	if err := m.prepare(); err != nil {
		return nil, err
	}
	paths := m.paths()
	changes := make([]Change, 0, len(paths))
	for _, p := range paths {
		c := Change{Path: p, Action: ActionUnchanged, Found: FoundNothing}
		if body, write := m.files[p]; write {
			c.Action, c.Hash = ActionCreated, digest(body)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// Commit makes the staged files readable through [Mem.Files]. A
// memory sink starts with no previous generation, so every file
// it commits is created and no removal has a file to remove.
func (m *Mem) Commit() ([]Written, error) {
	if err := m.finish(); err != nil {
		return nil, err
	}
	paths := slices.Sorted(maps.Keys(m.files))
	records := make([]Written, 0, len(paths))
	for _, p := range paths {
		body := m.files[p]
		m.committed[p] = body
		records = append(records, Written{
			Path: p, Action: ActionCreated, Hash: digest(body),
		})
	}
	return records, nil
}

// Discard drops the staging.
func (m *Mem) Discard() error {
	if err := m.finish(); err != nil {
		return err
	}
	clear(m.files)
	clear(m.removals)
	return nil
}

// Files returns the committed files, keyed by path: a copy of the
// map and of every body, so a caller editing either leaves the
// sink's files unchanged. Before Commit it returns nothing,
// because staged means invisible everywhere.
func (m *Mem) Files() map[string][]byte {
	out := make(map[string][]byte, len(m.committed))
	for p, body := range m.committed {
		out[p] = bytes.Clone(body)
	}
	return out
}
