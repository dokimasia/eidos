// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// racyMargin is how far before the sweep began a load sets its anchor:
// it covers FAT's two-second modification times and the tick of the
// clock that stamps them.
const racyMargin = 2 * time.Second

// Verdict is what the gate found a file to be.
type Verdict uint8

const (
	// VerdictInput is a file a frontend's selection claims.
	VerdictInput Verdict = 1
	// VerdictOutput is the brand's own output, proven by its trailer.
	VerdictOutput Verdict = 2
	// VerdictUnclaimed is a file no frontend claims.
	VerdictUnclaimed Verdict = 3
)

// FileRecord is what the gate recorded about one file: its stat, the
// digest of its bytes where the load read them, the verdict, and the
// package its unit declared it in.
//
// A workspace file has a workspace path and a store file a qualified
// one. The gate hashes the files a frontend claims and the files a unit
// or a door reads, and leaves Digest zero for every other file.
type FileRecord struct {
	Path    string
	Size    int64
	ModTime time.Time
	// Change and Inode are the file's change time and inode where
	// FileInfo.Sys reports them, and zero elsewhere.
	Change  time.Time
	Inode   uint64
	Digest  [sha256.Size]byte
	Verdict Verdict
	// Pkg is the package the file's unit declared it in, and zero for a
	// file no unit loaded.
	Pkg symbol.Identity
}

// gate finds what changed in one load's tree: it stats every file,
// compares each stat with the record of the last commit, and hashes a
// file only where the comparison cannot prove it unchanged.
//
// A record proves a file unchanged where its size, its modification
// time, and its change time and inode where both sides report them
// match, and its modification time precedes the anchor of the run that
// recorded it. A file that matches with a later modification time is
// racily clean: a write in the same tick of the clock could have
// changed it without moving its stat, so the gate hashes it.
type gate struct {
	tree   storeTree
	brand  output.Brand
	anchor time.Time
	// prior is the last commit's records by path and the anchor its run
	// took, both empty for a cold load.
	prior      map[string]FileRecord
	priorAnchr time.Time
	// files are this load's records by path, and walked the workspace's
	// file paths in the walk's lexical order.
	files  map[string]*FileRecord
	walked []string
	// statted and hashed count the files the gate statted and hashed.
	statted, hashed int
}

// newGate returns the gate of one load over its tree, anchored at now
// less the racy margin, comparing against the records of prior where
// prior is set.
func newGate(cfg Config, tree storeTree, now time.Time) (*gate, error) {
	g := &gate{
		tree:   tree,
		brand:  cfg.Brand,
		anchor: now.Add(-racyMargin),
		files:  map[string]*FileRecord{},
	}
	if cfg.Prior == nil {
		return g, nil
	}
	g.priorAnchr = cfg.Prior.Anchor()
	g.prior = map[string]FileRecord{}
	for rec, err := range cfg.Prior.Files() {
		if err != nil {
			return nil, fmt.Errorf("load: read the recorded files: %w", err)
		}
		g.prior[rec.Path] = rec
	}
	return g, nil
}

// walk stats every regular file of the workspace tree, in lexical
// order, skipping the brand's state directory at the root.
func (g *gate) walk() error {
	state := "." + string(g.brand)
	err := fs.WalkDir(g.tree.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == state {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		g.statted++
		g.files[path] = g.record(path, info)
		g.walked = append(g.walked, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("load: walk the tree: %w", err)
	}
	return nil
}

// record returns a file's record from its stat: the recorded digest and
// verdict where the stat proves the file unchanged, and neither where
// it does not.
func (g *gate) record(path string, info fs.FileInfo) *FileRecord {
	change, inode := sysStat(info)
	rec := &FileRecord{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Change:  change,
		Inode:   inode,
	}
	if was, held := g.prior[path]; held && g.unchanged(was, rec) {
		rec.Digest, rec.Verdict, rec.Pkg = was.Digest, was.Verdict, was.Pkg
	}
	return rec
}

// unchanged reports whether a stat proves a file unchanged since its
// record: the size and modification time match, the change time and
// inode match where both report them, the record states a verdict, and
// the modification time precedes the recording run's anchor.
func (g *gate) unchanged(was FileRecord, now *FileRecord) bool {
	return was.Verdict != 0 &&
		was.Size == now.Size &&
		was.ModTime.Equal(now.ModTime) &&
		(was.Change.IsZero() || now.Change.IsZero() || was.Change.Equal(now.Change)) &&
		(was.Inode == 0 || now.Inode == 0 || was.Inode == now.Inode) &&
		now.ModTime.Before(g.priorAnchr)
}

// judge settles the verdict of every workspace file: a claimed file
// whose record the stat did not prove is read whole, hashed, and judged
// output where it ends in the brand's provenance trailer. An unclaimed
// file is judged without a read. It returns the claimed files the
// verdicts made inputs, per frontend, and the outputs, sorted by path.
func (g *gate) judge(claims [][]string) ([][]string, []string, error) {
	claimed := make(map[string]bool, len(g.walked))
	for _, paths := range claims {
		for _, path := range paths {
			claimed[path] = true
		}
	}
	for _, path := range g.walked {
		rec := g.files[path]
		if !claimed[path] {
			rec.Verdict = VerdictUnclaimed
			continue
		}
		if rec.Verdict == VerdictInput || rec.Verdict == VerdictOutput {
			continue
		}
		b, err := g.read(rec)
		if err != nil {
			return nil, nil, err
		}
		rec.Verdict = VerdictInput
		if prov, stamped := output.Read(b); stamped && prov.Brand == g.brand {
			rec.Verdict = VerdictOutput
		}
	}

	var excluded []string
	inputs := make([][]string, len(claims))
	for i, paths := range claims {
		for _, path := range paths {
			if g.files[path].Verdict == VerdictOutput {
				excluded = append(excluded, path)
				continue
			}
			inputs[i] = append(inputs[i], path)
		}
	}
	slices.Sort(excluded)
	return inputs, excluded, nil
}

// digest returns the digest of a file a unit or a door reads: the
// recorded one where the stat proves the file unchanged, and otherwise
// the digest of the bytes it reads now. A store file the walk did not
// visit is statted first. It reports false for a path no file is at,
// such as a shared input a frontend declares and the tree lacks, or a
// file of a store the load does not provide.
//
// Error modes: the error of a file that fails to stat or to read for a
// cause other than its absence.
func (g *gate) digest(path string) ([sha256.Size]byte, bool, error) {
	rec, held := g.files[path]
	if !held {
		info, err := plugin.Stat(g.tree, path)
		if absent(err) {
			return [sha256.Size]byte{}, false, nil
		}
		if err != nil {
			return [sha256.Size]byte{}, false, fmt.Errorf("load: stat %s: %w", path, err)
		}
		rec = g.recordStat(path, info)
	}
	if rec.Digest == ([sha256.Size]byte{}) {
		if _, err := g.read(rec); err != nil {
			return [sha256.Size]byte{}, false, err
		}
	}
	return rec.Digest, true, nil
}

// note records the digest of a file a door read, so the next load
// proves it unchanged by its stat. A store file the walk did not visit
// is statted first, and a stat that fails leaves the file unrecorded,
// so the next load reads it again. A record that states a digest keeps
// it.
func (g *gate) note(path string, digest [sha256.Size]byte) {
	rec, held := g.files[path]
	if !held {
		info, err := plugin.Stat(g.tree, path)
		if err != nil {
			return
		}
		rec = g.recordStat(path, info)
	}
	if rec.Digest == ([sha256.Size]byte{}) {
		g.hashed++
		rec.Digest = digest
	}
}

// recordStat records a file the walk did not visit from its stat, counts
// the stat, and judges a file no frontend claims unclaimed.
func (g *gate) recordStat(path string, info fs.FileInfo) *FileRecord {
	g.statted++
	rec := g.record(path, info)
	if rec.Verdict == 0 {
		rec.Verdict = VerdictUnclaimed
	}
	g.files[path] = rec
	return rec
}

// read reads a file whole, records the digest of its bytes, and returns
// them.
func (g *gate) read(rec *FileRecord) ([]byte, error) {
	b, err := plugin.ReadFile(g.tree, rec.Path)
	if err != nil {
		return nil, fmt.Errorf("load: read %s: %w", rec.Path, err)
	}
	g.hashed++
	rec.Digest = sha256.Sum256(b)
	return b, nil
}

// declare records the package each file of a region's packages is
// declared in.
func (g *gate) declare(pkg symbol.Identity, path string) {
	if rec, held := g.files[path]; held {
		rec.Pkg = pkg
	}
}

// changed returns the walked files whose record did not prove them
// unchanged, the recorded workspace files the walk did not find, and the
// records of both that the record lists, each sorted by path. A record
// proves a file unchanged where its stat matches. A cold load returns
// none of them.
func (g *gate) changed() (moved, vanished []string, was []FileRecord) {
	if g.prior == nil {
		return nil, nil, nil
	}
	for _, path := range g.walked {
		rec, held := g.prior[path]
		if held && g.unchanged(rec, g.files[path]) {
			continue
		}
		moved = append(moved, path)
		if held {
			was = append(was, rec)
		}
	}
	for path, rec := range g.prior {
		_, _, stored := plugin.CutStorePath(path)
		if _, walked := g.files[path]; !walked && !stored {
			vanished = append(vanished, path)
			was = append(was, rec)
		}
	}
	slices.Sort(moved)
	slices.Sort(vanished)
	slices.SortFunc(was, func(a, b FileRecord) int { return strings.Compare(a.Path, b.Path) })
	return moved, vanished, was
}

// records returns every record, sorted by path.
func (g *gate) records() []FileRecord {
	out := make([]FileRecord, 0, len(g.files))
	for _, rec := range g.files {
		out = append(out, *rec)
	}
	slices.SortFunc(out, func(a, b FileRecord) int { return strings.Compare(a.Path, b.Path) })
	return out
}
