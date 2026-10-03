// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"
)

// Mem stores blobs in memory: the ledger of a test, and of a composition
// that keeps no state on disk. One Mem serves any number of runs one
// after another, so a test can run a workspace twice over one record.
//
// A logical clock stamps each blob: every write, put and touch advances
// it by one tick, and a blob's modification time is its tick as
// nanoseconds after the Unix epoch. Two blobs written in sequence
// therefore never share a modification time, and the times do not
// depend on the wall clock.
//
// # Concurrency
//
// A Mem is safe for concurrent use.
//
// # Allocation contract
//
// [Mem.Write] and [Mem.Put] copy the bytes they store, and [Mem.Read]
// returns a copy, so neither the caller nor the run changes a blob after
// it is stored. [Mem.List] allocates its result.
type Mem struct {
	mu    sync.Mutex
	blobs map[string]memBlob
	// clock is the last tick a write, put or touch took.
	clock int64
	// writes counts the calls to Write and Put that stored a blob.
	writes int
}

// memBlob is one stored blob and the tick it was last written or
// touched at.
type memBlob struct {
	data []byte
	tick int64
}

var _ Ledger = (*Mem)(nil)

// NewMem returns an empty ledger in memory.
func NewMem() *Mem { return &Mem{blobs: map[string]memBlob{}} }

// Read returns a copy of a blob.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], and a name nothing
// wrote wraps [fs.ErrNotExist].
func (l *Mem) Read(ctx context.Context, name string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := l.blob(ctx, name)
	if err != nil {
		return nil, err
	}
	return slices.Clone(b.data), nil
}

// ReadAt copies len(p) bytes of a blob from offset off, under the
// contract of [io.ReaderAt].
//
// Error modes: an invalid name wraps [fs.ErrInvalid], a name nothing
// wrote wraps [fs.ErrNotExist], a negative offset is an error, and a read
// that ends at the blob's end returns [io.EOF] beside the count.
func (l *Mem) ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := l.blob(ctx, name)
	if err != nil {
		return 0, err
	}
	if off < 0 {
		return 0, fmt.Errorf("ledger: read %s at %d: negative offset", name, off)
	}
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Write stores a copy of b under the name at the next tick.
//
// Error modes: an invalid name wraps [fs.ErrInvalid].
func (l *Mem) Write(ctx context.Context, name string, b []byte) error { return l.store(ctx, name, b) }

// Put stores a copy of b under the name at the next tick, as Write does:
// memory has no crash to survive.
//
// Error modes: an invalid name wraps [fs.ErrInvalid].
func (l *Mem) Put(ctx context.Context, name string, b []byte) error { return l.store(ctx, name, b) }

// Touch moves a blob to the next tick.
//
// Error modes: an invalid name wraps [fs.ErrInvalid], and a name nothing
// wrote wraps [fs.ErrNotExist].
func (l *Mem) Touch(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, err := l.blob(ctx, name)
	if err != nil {
		return err
	}
	l.clock++
	b.tick = l.clock
	l.blobs[name] = b
	return nil
}

// Remove deletes a blob. A name nothing wrote is not an error.
//
// Error modes: an invalid name wraps [fs.ErrInvalid].
func (l *Mem) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.blobs, name)
	return nil
}

// List returns the blobs whose names begin with dir and a slash, sorted
// by name.
//
// Error modes: an invalid dir wraps [fs.ErrInvalid].
func (l *Mem) List(ctx context.Context, dir string) ([]Blob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := checkName(dir); err != nil {
		return nil, err
	}
	prefix := dir + "/"
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Blob
	for name, b := range l.blobs {
		if strings.HasPrefix(name, prefix) {
			out = append(out, Blob{Name: name, Size: int64(len(b.data)), ModTime: time.Unix(0, b.tick)})
		}
	}
	slices.SortFunc(out, func(a, b Blob) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Writes returns how many calls to Write and Put stored a blob: what a
// test reads to tell a run that wrote nothing from one that wrote a
// generation.
func (l *Mem) Writes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writes
}

// store keeps a copy of b under the name at the next tick, and counts
// the write. The caller has not taken the lock.
func (l *Mem) store(ctx context.Context, name string, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clock++
	l.blobs[name] = memBlob{data: slices.Clone(b), tick: l.clock}
	l.writes++
	return nil
}

// blob returns the blob a name stores. The caller has taken the lock.
func (l *Mem) blob(ctx context.Context, name string) (memBlob, error) {
	if err := ctx.Err(); err != nil {
		return memBlob{}, err
	}
	if err := checkName(name); err != nil {
		return memBlob{}, err
	}
	b, held := l.blobs[name]
	if !held {
		return memBlob{}, fmt.Errorf("ledger: %s: %w", name, fs.ErrNotExist)
	}
	return b, nil
}
