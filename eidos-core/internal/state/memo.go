// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/store"
)

// The ledger names of the parse memo: its directory, the blob that
// states its size in bytes, and the blob that states when the memo was
// last listed whole.
const (
	memoDir     = "memo"
	totalName   = "memo/total"
	trimmedName = "memo/trimmed"
)

// trimInterval is how often a commit lists the whole memo to correct
// its total, whatever the total states: the go command trims its build
// cache as often.
const trimInterval = 24 * time.Hour

// The share of its cap a trim leaves the memo under: nine tenths.
const (
	trimKeep  = 9
	trimWhole = 10
)

// Memo is the parse memo over a ledger: each parsed unit's region under
// the unit's key and the executable's digest, so an entry serves the
// build that wrote it. A load reads it through Get and fills it through
// Put, and the run's commit writes what the load put through
// [Memo.Write].
//
// An entry is a region's blob, whose trailing CRC-32C covers its
// bytes, so an entry a crash tore decodes as a miss and the next commit
// removes it. Entries write through [ledger.Ledger.Put], without a
// sync, because a lost entry costs a parse and never correctness.
//
// # Concurrency
//
// A Memo is safe for concurrent use: one mutex guards what the load
// read and put.
type Memo struct {
	ctx   context.Context
	l     ledger.Ledger
	exe   [sha256.Size]byte
	limit int64
	now   time.Time

	mu      sync.Mutex
	pending map[string]*store.Region
	hits    map[string]bool
	torn    map[string]bool
	failure error
}

// NewMemo returns the parse memo of one run over a ledger, under the
// run's executable digest, with a cap in bytes, at the instant the run
// started. The memo reads its entries through ctx, the run's context.
func NewMemo(ctx context.Context, l ledger.Ledger, exe [sha256.Size]byte, limit int64, now time.Time) *Memo {
	return &Memo{
		ctx: ctx, l: l, exe: exe, limit: limit, now: now,
		pending: map[string]*store.Region{}, hits: map[string]bool{}, torn: map[string]bool{},
	}
}

// Get returns the region recorded under a unit's key, decoded, and
// reports false on a miss: no entry, an entry of another build, and an
// entry that does not decode, which the commit removes. A ledger that
// fails to read misses, and [Memo.Write] returns its error.
func (m *Memo) Get(key []byte) (*store.Region, bool) {
	name := m.name(key)
	b, err := m.l.Read(m.ctx, name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && m.failure == nil {
			m.failure = fmt.Errorf("state: read the memo's %s: %w", name, err)
		}
		return nil, false
	}
	r, err := DecodeRegion(b)
	if err != nil {
		m.torn[name] = true
		return nil, false
	}
	m.hits[name] = true
	return r, true
}

// Put records a parsed unit's region under its key, for the commit to
// write.
func (m *Memo) Put(key []byte, r *store.Region) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending[m.name(key)] = r
}

// Write writes every region the load put, touches every entry a load
// hit so recency follows use, and removes every entry that did not
// decode. It adds the bytes it wrote to the memo's total, and trims the
// memo where the total exceeds the cap or the last trim is more than a
// day old: it lists the memo, takes the true total, and removes the
// entries with the oldest modification times until the memo is under
// nine tenths of its cap. A memo whose load put nothing, hit nothing and
// met nothing torn, and that no trim is due for, writes nothing. It
// returns the bytes it wrote.
//
// Error modes: the first failure of a read Get met, and every write,
// touch, removal or listing of the ledger that fails, joined. A region
// that does not encode is joined too, and its entry is not written.
func (m *Memo) Write(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	errs := []error{m.failure}
	var written int64
	for _, name := range slices.Sorted(maps.Keys(m.pending)) {
		b, err := AppendRegion(nil, m.pending[name])
		if err == nil {
			err = m.l.Put(ctx, name, b)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		written += int64(len(b))
	}
	for _, name := range slices.Sorted(maps.Keys(m.hits)) {
		errs = append(errs, m.l.Touch(ctx, name))
	}
	for _, name := range slices.Sorted(maps.Keys(m.torn)) {
		errs = append(errs, m.l.Remove(ctx, name))
	}
	total, trimmed := m.read(ctx, totalName), m.read(ctx, trimmedName)
	changed := written > 0
	if total+written > m.limit || m.now.Sub(time.Unix(0, trimmed)) > trimInterval {
		var err error
		if total, err = m.trim(ctx); err != nil {
			errs = append(errs, err)
		}
		changed = true
	} else {
		total += written
	}
	if changed {
		errs = append(errs, m.l.Put(ctx, totalName, []byte(strconv.FormatInt(total, 10)+"\n")))
	}
	return written, errors.Join(errs...)
}

// trim lists the memo, removes the entries with the oldest modification
// times until the memo is under nine tenths of its cap where it exceeds
// the cap, records the instant of the trim, and returns the memo's true
// total.
//
// Error modes: a listing, a removal or a write of the ledger that
// fails, joined.
func (m *Memo) trim(ctx context.Context) (int64, error) {
	blobs, err := m.l.List(ctx, memoDir)
	if err != nil {
		return 0, fmt.Errorf("state: list the memo: %w", err)
	}
	entries := slices.DeleteFunc(blobs, func(b ledger.Blob) bool { return !entryName(b.Name) })
	var total int64
	for _, b := range entries {
		total += b.Size
	}
	var errs []error
	if total > m.limit {
		slices.SortFunc(entries, func(a, b ledger.Blob) int {
			return cmp.Or(a.ModTime.Compare(b.ModTime), strings.Compare(a.Name, b.Name))
		})
		keep := m.limit * trimKeep / trimWhole
		for _, b := range entries {
			if total < keep {
				break
			}
			if err := m.l.Remove(ctx, b.Name); err != nil {
				errs = append(errs, err)
				continue
			}
			total -= b.Size
		}
	}
	errs = append(errs, m.l.Put(ctx, trimmedName, []byte(strconv.FormatInt(m.now.UnixNano(), 10)+"\n")))
	return total, errors.Join(errs...)
}

// read returns the number one of the memo's own blobs states, and zero
// for a blob that is missing or does not parse, which a trim corrects.
func (m *Memo) read(ctx context.Context, name string) int64 {
	b, err := m.l.Read(ctx, name)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// name returns the ledger name of a unit's entry: the hex SHA-256 of the
// unit's key and the executable's digest, under its first byte.
func (m *Memo) name(key []byte) string {
	h := sha256.New()
	h.Write(key)
	h.Write(m.exe[:])
	digest := hex.EncodeToString(h.Sum(nil))
	return memoDir + "/" + digest[:2] + "/" + digest
}

// entryName reports whether a ledger name is an entry of the memo, and
// not one of its own blobs.
func entryName(name string) bool {
	rest, under := strings.CutPrefix(name, memoDir+"/")
	bucket, digest, nested := strings.Cut(rest, "/")
	return under && nested && len(bucket) == 2 && len(digest) == 2*sha256.Size && strings.HasPrefix(digest, bucket)
}
