// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"context"
	"sync"

	"go.dokimi.dev/eidos/core/manifest"
)

// Mem records in memory: the ledger of a test, and of a composition
// that keeps no state. One Mem serves any number of runs one after
// another, each beginning with the manifest the last one committed, so
// a test can run a workspace twice over one record. It keeps the
// manifest it is handed and returns that value, so neither the run
// nor a caller changes a manifest after recording it. A Mem is safe
// for concurrent use.
type Mem struct {
	mu       sync.Mutex
	recorded manifest.Manifest
	// writes counts the commits that changed the record.
	writes int
}

var _ Ledger = (*Mem)(nil)

// NewMem returns a ledger that records in memory, for tests and for a
// composition that keeps no state. It starts as a workspace no run has
// committed.
func NewMem() *Mem { return &Mem{recorded: empty()} }

// BeginRun returns the manifest the last commit recorded, and the empty
// manifest before the first. It never returns an error.
func (l *Mem) BeginRun(context.Context) (manifest.Manifest, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.recorded, nil
}

// CommitRun records the manifest. A manifest equal to the recorded one
// changes nothing. It never returns an error.
func (l *Mem) CommitRun(_ context.Context, m manifest.Manifest) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m.Equal(l.recorded) {
		return nil
	}
	l.recorded = m
	l.writes++
	return nil
}

// Writes returns how many commits changed the record: what a test reads
// to tell a run that changed nothing from one that rewrote the record.
func (l *Mem) Writes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writes
}
