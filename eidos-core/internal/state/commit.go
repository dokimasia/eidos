// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// maxRuns is how many runs a table keeps: a commit that would leave a
// table more merges them into one.
const maxRuns = 8

// Commit is the generation a run writes over the one it opened: the
// regions it adds, the regions it no longer references, and the rows
// each table changes. A cold run's commit has no parent.
//
// A table's new rows become one run, so a commit's bytes follow the
// rows the run changed and not the size of the table. A commit merges a
// table's runs into one, dropping its tombstones, where the table would
// keep more than eight runs, or where its newer runs would contain more
// bytes than its oldest. A row a merge has written once is written
// again at most once on average, under a table of stable size.
//
// # Concurrency
//
// A Commit is not safe for concurrent use.
type Commit struct {
	parent *Generation
	// live counts each region segment's regions a unit still
	// references, starting from the parent's.
	live map[string]int
	// regions is the new region segment, and regionSegment its name.
	regions       []byte
	regionSegment string
	changes       [tableCount]map[string]entry
	// recorded are the digests of the documents the ledger contains,
	// which the commit compares the manifest's documents with.
	recorded Digests
}

// Result is what one commit wrote: the name of the generation it made
// live, the bytes it wrote to the ledger, and the bytes of the live
// generation, its segments included, zero where it wrote no generation.
type Result struct {
	Generation string
	Written    int64
	Size       int64
}

// NewCommit returns an empty commit over a parent, nil for a cold run.
// recorded are the digests of the documents the ledger contains, as
// [ReadManifest] returns them, and nil for none: the commit writes each
// document whose digest differs from them. A parent's own record of the
// documents can be older than the ledger's, because a run over a graph,
// a run that cannot read its executable, and a run that stops between
// the documents and CURRENT each write documents without a generation.
func NewCommit(parent *Generation, recorded Digests) *Commit {
	c := &Commit{parent: parent, live: map[string]int{}, recorded: recorded}
	if parent != nil {
		for s, n := range parent.segments {
			if n > 0 {
				c.live[s] = n
			}
		}
	}
	return c
}

// AddRegions makes the blobs one region segment, named by its digest,
// and returns where each blob is. A commit adds one segment: a second
// call refuses.
//
// Error modes: an error for a second call.
func (c *Commit) AddRegions(blobs [][]byte) ([]RegionRef, error) {
	if c.regionSegment != "" {
		return nil, fmt.Errorf("state: a commit adds one region segment, and it has one")
	}
	if len(blobs) == 0 {
		return nil, nil
	}
	refs := make([]RegionRef, len(blobs))
	for i, b := range blobs {
		refs[i] = RegionRef{Offset: int64(len(c.regions)), Length: int64(len(b))}
		c.regions = append(c.regions, b...)
	}
	c.regionSegment = blobName(segDir, c.regions)
	for i := range refs {
		refs[i].Segment = c.regionSegment
	}
	c.live[c.regionSegment] += len(blobs)
	return refs, nil
}

// Release records that the next generation no longer references one
// region of the parent's: a unit the run dropped or replaced.
func (c *Commit) Release(ref RegionRef) {
	if c.live[ref.Segment] > 0 {
		c.live[ref.Segment]--
	}
}

// Put records a table's row under a key, replacing the key's row in
// every older run.
func (c *Commit) Put(t Table, key, row []byte) {
	c.change(t, entry{key: bytes.Clone(key), row: bytes.Clone(row)})
}

// Delete records a tombstone for a key, deleting its row in every older
// run.
func (c *Commit) Delete(t Table, key []byte) {
	c.change(t, entry{key: bytes.Clone(key), dead: true})
}

// Write makes the commit the live generation, in the order a crash can
// interrupt without losing a run's work:
//
//  1. the region segment and the run segment, each durably
//  2. the generation, durably
//  3. each manifest document whose bytes differ from the ledger's, and the
//     removal of each document whose bucket is now empty
//  4. CURRENT, naming the generation
//  5. the removal of every generation and segment the new generation does
//     not reference and that is older than the header's anchor
//
// A commit with no region, no changed row, and a manifest whose documents
// both the ledger and the parent record unchanged writes nothing and
// returns the parent's name. A parent whose record of the documents
// differs from the ledger's gets a successor that records the ledger's.
// The header's sequence and parent are the commit's own.
//
// Error modes: a ledger's failure to write a segment, the generation or
// CURRENT returns it, and the live generation is the parent; a run that
// stops before CURRENT leaves the parent live. A manifest document that
// fails is joined into the returned error after CURRENT names the new
// generation. A removal that fails is left to the next commit, which
// retries it.
func (c *Commit) Write(ctx context.Context, l ledger.Ledger, h Header, m manifest.Manifest) (Result, error) {
	docs, err := documentsOf(m)
	if err != nil {
		return Result{}, err
	}
	unchanged := maps.Equal(c.recorded, docs.digests)
	if c.parent != nil && c.empty() && unchanged && maps.Equal(c.parent.Manifest, docs.digests) {
		return Result{Generation: c.parent.Name}, nil
	}
	tables, written, err := c.segments(ctx, l)
	if err != nil {
		return Result{}, err
	}
	h.Format = Format
	h.Sequence, h.Parent = 1, ""
	if c.parent != nil {
		h.Sequence, h.Parent = c.parent.Header.Sequence+1, c.parent.Name
	}
	g := &Generation{Header: h, Manifest: docs.digests, segments: c.segmentsOf(tables), tables: tables}
	return c.publish(ctx, l, g, docs, written)
}

// change records one entry, the last one for a key deciding.
func (c *Commit) change(t Table, e entry) {
	if c.changes[t] == nil {
		c.changes[t] = map[string]entry{}
	}
	c.changes[t][string(e.key)] = e
}

// segments writes the region segment and the run segment, each durably,
// and returns every table's runs after the commit and the bytes it
// wrote.
func (c *Commit) segments(ctx context.Context, l ledger.Ledger) ([tableCount][]runRef, int64, error) {
	var written int64
	if len(c.regions) > 0 {
		if err := l.Write(ctx, c.regionSegment, c.regions); err != nil {
			return [tableCount][]runRef{}, 0, fmt.Errorf("state: write the region segment: %w", err)
		}
		written += int64(len(c.regions))
	}
	tables, runs, err := c.tables(ctx)
	if err != nil || len(runs) == 0 {
		return tables, written, err
	}
	segment := blobName(segDir, runs)
	for t := range tables {
		for i := range tables[t] {
			if tables[t][i].segment == "" {
				tables[t][i].segment = segment
			}
		}
	}
	if err := l.Write(ctx, segment, runs); err != nil {
		return tables, 0, fmt.Errorf("state: write the run segment: %w", err)
	}
	return tables, written + int64(len(runs)), nil
}

// publish writes the generation durably, then the manifest's documents
// that changed, then CURRENT, and collects what the generation no longer
// references.
func (c *Commit) publish(
	ctx context.Context, l ledger.Ledger, g *Generation, docs documents, written int64,
) (Result, error) {
	blob := appendGeneration(nil, g)
	g.Name = blobName(genDir, blob)
	if err := l.Write(ctx, g.Name, blob); err != nil {
		return Result{}, fmt.Errorf("state: write the generation: %w", err)
	}
	wrote, docErr := docs.write(ctx, l, c.recorded)
	if err := l.Write(ctx, currentName, []byte(g.Name+"\n")); err != nil {
		return Result{}, fmt.Errorf("state: write CURRENT: %w", err)
	}
	return Result{
		Generation: g.Name,
		Written:    written + int64(len(blob)) + wrote,
		Size:       collect(ctx, l, g, int64(len(blob))),
	}, docErr
}

// empty reports whether the commit adds no region and changes no row.
func (c *Commit) empty() bool {
	if len(c.regions) > 0 {
		return false
	}
	for _, changes := range c.changes {
		if len(changes) > 0 {
			return false
		}
	}
	return true
}

// tables returns every table's runs after the commit, and the bytes of
// the new runs, whose references name an empty segment until the
// caller names the run segment.
func (c *Commit) tables(ctx context.Context) ([tableCount][]runRef, []byte, error) {
	var (
		out  [tableCount][]runRef
		runs []byte
	)
	if c.parent != nil {
		for t := range out {
			out[t] = slices.Clone(c.parent.tables[t])
		}
	}
	for t, changes := range c.changes {
		if len(changes) == 0 {
			continue
		}
		fresh := slices.SortedFunc(maps.Values(changes), compareEntries)
		start := len(runs)
		if merges(out[t], fresh) {
			merged, err := c.parent.readers[t].merged(ctx)
			if err != nil {
				return out, nil, fmt.Errorf("state: merge the %s table: %w", Table(t), err)
			}
			live := slices.DeleteFunc(mergeEntries(merged, fresh), func(e entry) bool { return e.dead })
			runs = appendRun(runs, live)
			out[t] = []runRef{{offset: int64(start), length: int64(len(runs) - start)}}
			continue
		}
		runs = appendRun(runs, fresh)
		out[t] = append(out[t], runRef{offset: int64(start), length: int64(len(runs) - start)})
	}
	return out, runs, nil
}

// merges reports whether a table's runs and its new run merge into one:
// the table would keep more than [maxRuns] runs, or its newer runs would
// contain more bytes than its oldest. A table without a run merges
// nothing.
func merges(runs []runRef, fresh []entry) bool {
	if len(runs) == 0 {
		return false
	}
	if len(runs)+1 > maxRuns {
		return true
	}
	var newer int64
	for _, r := range runs[1:] {
		newer += r.length
	}
	for _, e := range fresh {
		newer += int64(len(e.key) + len(e.row))
	}
	return newer > runs[0].length
}

// segmentsOf returns the segments a generation references: each region
// segment a unit still references, with its live regions, and each run
// segment a table references, with none.
func (c *Commit) segmentsOf(tables [tableCount][]runRef) map[string]int {
	out := map[string]int{}
	for s, n := range c.live {
		if n > 0 {
			out[s] = n
		}
	}
	for _, runs := range tables {
		for _, r := range runs {
			if _, held := out[r.segment]; !held {
				out[r.segment] = 0
			}
		}
	}
	return out
}

// collect removes every generation and segment a generation does not
// reference and that is older than its anchor, and returns the bytes of
// the generation and the segments it references. A listing or a removal
// that fails is left to the next commit.
func collect(ctx context.Context, l ledger.Ledger, g *Generation, size int64) int64 {
	blobs, err := l.List(ctx, "state")
	if err != nil {
		return size
	}
	for _, b := range blobs {
		_, referenced := g.segments[b.Name]
		if referenced {
			size += b.Size
			continue
		}
		stale := b.Name != g.Name && b.ModTime.Before(g.Header.Anchor) &&
			(strings.HasPrefix(b.Name, genDir+"/") || strings.HasPrefix(b.Name, segDir+"/"))
		if stale {
			_ = l.Remove(ctx, b.Name) // the next commit retries a removal that failed
		}
	}
	return size
}
