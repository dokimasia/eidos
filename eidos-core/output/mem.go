// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import "maps"

// Mem stages in memory and commits into a map, for tests and for
// a dry run that computes everything and writes nowhere. It holds
// the same staging law as the disk sink: nothing is readable
// until the commit.
type Mem struct {
	staging
	committed map[string][]byte
}

// NewMem opens a sink over memory.
func NewMem() *Mem { return &Mem{committed: map[string][]byte{}} }

// Write stages one file.
func (m *Mem) Write(path string, body []byte) error { return m.stage(path, body) }

// Commit makes the staged files readable through [Mem.Files]. A
// memory sink starts with no previous generation, so every file
// it commits is created.
func (m *Mem) Commit() ([]Written, error) {
	if err := m.finish(); err != nil {
		return nil, err
	}
	paths := m.paths()
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
	return nil
}

// Files returns the committed files, keyed by path: a copy, so a
// caller reading them cannot edit what the sink holds. Before
// Commit it returns nothing, because staged means invisible
// everywhere.
func (m *Mem) Files() map[string][]byte { return maps.Clone(m.committed) }
