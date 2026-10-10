// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"context"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// manifestPrefix opens the name of every manifest document a ledger
// stores.
const manifestPrefix = "manifest/"

// Edit returns an edited copy of the record a run commits, given the
// run's ordinal, counted from one. The record's file list is the edit's
// own to change.
type Edit func(run int, m manifest.Manifest) manifest.Manifest

// Rewriting is a disk ledger whose record on disk is an edited copy of
// what runs commit: the shape of a ledger that records something other
// than what a run wrote, which the conformance suites' record checks
// must expose. It applies its edit to the record it finds when it opens,
// so a later run reads the edited record, and again after each manifest
// document it writes or removes, so a commit leaves the edited record.
// It applies the edit many times, so an edit is idempotent.
//
// # Concurrency
//
// A Rewriting is safe for concurrent use: one lock serializes every
// write and removal with the rewrite that follows it.
type Rewriting struct {
	*ledger.Dir
	run  int
	edit Edit
	mu   sync.Mutex
}

// NewRewriting returns the rewriting ledger of the state directory under
// root for the run-th run over it, after it applies the edit to the
// record the directory contains.
//
// Error modes: those of [ledger.OpenDir], and of reading and writing the
// record.
func NewRewriting(ctx context.Context, root string, brand output.Brand, run int, edit Edit) (*Rewriting, error) {
	dir, err := ledger.OpenDir(root, brand)
	if err != nil {
		return nil, err
	}
	l := &Rewriting{Dir: dir, run: run, edit: edit}
	if err := l.rewrite(ctx, manifestPrefix); err != nil {
		return nil, err
	}
	return l, nil
}

// Write writes a blob, and after a manifest document, rewrites the
// record through the edit.
//
// Error modes: those of [ledger.Dir.Write], and of reading and writing
// the record back.
func (l *Rewriting) Write(ctx context.Context, name string, b []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.Dir.Write(ctx, name, b); err != nil {
		return err
	}
	return l.rewrite(ctx, name)
}

// Remove removes a blob, and after a manifest document, rewrites the
// record through the edit.
//
// Error modes: those of [ledger.Dir.Remove], and of reading and writing
// the record back.
func (l *Rewriting) Remove(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.Dir.Remove(ctx, name); err != nil {
		return err
	}
	return l.rewrite(ctx, name)
}

// rewrite reads the record back after a change to the blob the name
// states, where the blob is a manifest document, applies the edit to a
// copy of its file list and records the result. A record without
// documents is left alone, so an edit never invents a record no run
// wrote. The caller has taken the lock.
func (l *Rewriting) rewrite(ctx context.Context, name string) error {
	if !strings.HasPrefix(name, manifestPrefix) {
		return nil
	}
	m, digests, err := state.ReadManifest(ctx, l.Dir)
	if err != nil || len(digests) == 0 {
		return err
	}
	m.Files = slices.Clone(m.Files)
	_, err = state.WriteManifest(ctx, l.Dir, l.edit(l.run, m), digests)
	return err
}
