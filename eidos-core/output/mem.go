// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output

import "bytes"

// Mem stages in memory and commits into a map, for tests and for
// a dry run that computes everything and writes nowhere. It follows
// the same staging rule as the disk sink: nothing is readable
// until the commit. Its destination starts empty, so every file it
// commits is created and every staged removal finds nothing.
//
// The zero Mem is an empty sink, ready to use.
//
// # Concurrency
//
// A Mem belongs to one goroutine, as every [Sink] does.
//
// # Allocation contract
//
// A Mem keeps what it stages and allocates as its maps grow. Preparing
// and committing allocate the sorted path list, the list returned and
// one digest per staged file. Each method states its count.
type Mem struct {
	staging
	// committed is what Commit made readable, nil before the commit.
	committed map[string][]byte
}

var _ Sink = (*Mem)(nil)

// NewMem opens a sink over memory. It allocates the sink alone.
func NewMem() *Mem { return &Mem{} }

// Write stages one file. It keeps body without copying it, so the
// caller leaves body unchanged until the commit.
//
// Error modes are the ones [Sink.Write] lists.
//
// # Allocation contract
//
// The sink's first write allocates the staging's map and the path set's
// maps of files and of directories: five allocations for a path without
// a directory, and six for a path with one. A later write allocates
// where one of the maps grows.
func (m *Mem) Write(path string, body []byte) error { return m.stage(path, body) }

// Delete stages the removal of one file, which the empty destination
// does not contain.
//
// Error modes are the ones [Sink.Delete] lists.
//
// # Allocation contract
//
// The sink's first removal allocates the removal set, two allocations.
// A later removal allocates where the set grows.
func (m *Mem) Delete(path string) error { return m.remove(path) }

// Prepare reports every staged path against the empty destination:
// each write creates its file, and each removal finds nothing.
//
// Error modes: a second preparation, and [ErrFinished] after Commit or
// Discard.
//
// # Allocation contract
//
// Prepare allocates the sorted path list, the list of changes, and one
// digest per staged file.
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
//
// Error modes: [ErrFinished] after Commit or Discard.
//
// # Allocation contract
//
// Commit allocates the sorted path list, the list of records, the
// committed map sized to the staged files, and one digest per file.
func (m *Mem) Commit() ([]Written, error) {
	if err := m.finish(); err != nil {
		return nil, err
	}
	records := make([]Written, 0, len(m.files))
	m.committed = make(map[string][]byte, len(m.files))
	for _, p := range m.paths() {
		body, write := m.files[p]
		if !write {
			continue
		}
		m.committed[p] = body
		records = append(records, Written{Path: p, Action: ActionCreated, Hash: digest(body)})
	}
	return records, nil
}

// Discard drops the staging. It allocates nothing.
//
// Error modes: [ErrFinished] after Commit or Discard.
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
//
// # Allocation contract
//
// Files allocates the map it returns and a copy of each body.
func (m *Mem) Files() map[string][]byte {
	out := make(map[string][]byte, len(m.committed))
	for p, body := range m.committed {
		out[p] = bytes.Clone(body)
	}
	return out
}
