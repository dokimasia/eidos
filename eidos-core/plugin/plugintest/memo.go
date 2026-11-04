// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest

import (
	"sync"

	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/store"
)

// memo is the parse memo that every fixture of a test binary shares. It
// keeps the blob of each region that a load parsed under the key of its
// unit, and each restore decodes a new region from the blob, so no fixture
// shares a node with another. The key of a unit covers the bytes of its
// files and the name, the version and the options of its frontend, so a
// fixture restores a unit only where its frontend would parse the same
// region. The memo keeps every blob for the life of the process.
//
// # Concurrency
//
// A memo is safe for concurrent use: one mutex guards its blobs.
type memo struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

var _ load.Memo = (*memo)(nil)

// parses is the memo of the fixtures of the test binary.
var parses = &memo{blobs: map[string][]byte{}}

// Get returns a region decoded from the blob under the key of a unit. It
// reports false where the memo has no blob under the key, and where the
// blob does not decode.
func (m *memo) Get(key []byte) (*store.Region, bool) {
	m.mu.Lock()
	b, held := m.blobs[string(key)]
	m.mu.Unlock()
	if !held {
		return nil, false
	}
	r, err := state.DecodeRegion(b)
	return r, err == nil
}

// Put keeps the blob of the region of a parsed unit under the key of the
// unit. It keeps nothing for a region that does not encode, so the next
// load parses the unit again.
func (m *memo) Put(key []byte, r *store.Region) {
	b, err := state.AppendRegion(nil, r)
	if err != nil {
		return
	}
	m.mu.Lock()
	m.blobs[string(key)] = b
	m.mu.Unlock()
}
