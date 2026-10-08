// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
)

// currentName is the ledger name of the pointer to the live generation,
// relative to the state directory: where [ColdState] reports.
const currentName = "state/CURRENT"

// sealedState is the sealed state one run reads and writes: the live
// generation the run compares the tree against, nil for a cold run, and
// the generation's record of the load. commit is the next generation the
// run records into, nil for a run that writes none: a dry run, a run
// over a caller's graph or without a ledger, a run whose previous record
// does not read, and a run that cannot read its executable. recorder
// collects the record of the run's phases, and phases is that record
// prepared before any plan commits, both nil where commit is. header is
// the next generation's header, and memo the parse memo, nil where the
// run keeps none.
type sealedState struct {
	ledger   ledger.Ledger
	gen      *state.Generation
	prior    *state.LoadState
	commit   *state.Commit
	recorder *state.Recorder
	phases   *state.PhaseRecord
	header   state.Header
	memo     *state.Memo
}

// warm reports whether the run is warm. A run is warm when it opened a
// usable generation and its load compared the tree with the generation's
// record.
func (s *sealedState) warm(loaded *load.Report) bool {
	return s.gen != nil && loaded != nil && loaded.Changes != nil
}

// loadPrior returns the record a warm load reads, and nil for a cold
// run.
func (s *sealedState) loadPrior() load.Prior {
	if s.prior == nil {
		return nil
	}
	return s.prior
}

// record records the run's load in the next generation, before any plan
// commits, so damage it meets in the generation's records discards the
// run with nothing written. The header takes the load's anchor.
//
// Error modes: the error of a region that does not encode, and [damage]
// for a record of the generation that does not read whole.
func (s *sealedState) record(ctx context.Context, loaded *load.Report) error {
	if s.commit == nil {
		return nil
	}
	s.header.Anchor = loaded.Anchor
	if err := state.RecordLoad(ctx, s.commit, s.prior, loaded); err != nil {
		return damaged(fmt.Errorf("workspace: record the load: %w", err))
	}
	return nil
}

// recordPhases prepares the record of the run's phases, after Close and
// before any plan commits, so damage it meets in the generation's records
// discards the run with nothing written. modules counts the packages that
// name each module on a warm run, and a cold run passes nil, which counts
// them over the graph. A run that records no phases prepares nothing.
//
// Error modes: [damage] for a record of the generation that does not
// read whole, and the error of a claim whose value is outside the fact
// vocabulary.
func (s *sealedState) recordPhases(
	ctx context.Context, g *store.Graph, facts *meta.Facts, k meta.KernelKeys, modules map[plugin.Module]int,
) error {
	if s.recorder == nil {
		return nil
	}
	if modules == nil {
		modules = moduleCounts(g, facts, k)
	}
	p, err := state.RecordPhases(ctx, s.gen, s.recorder, state.PhaseRun{Facts: facts, Modules: modules})
	if err != nil {
		return damaged(fmt.Errorf("workspace: record the phases: %w", err))
	}
	s.phases = p
	return nil
}

// write completes the record of the run's phases with each plan's
// outcome, then makes the next generation live, strictly after every
// plan's commit, with the merged manifest's documents that differ from
// the ledger's. It records each file that a committing plan staged. It
// writes under a context without the run's cancellation, because the
// record has to match the destination once a commit wrote to it, and
// counts what it wrote in stats. A plan that does not commit keeps its
// previous records and files.
//
// Error modes: a ledger's failure to write, wrapped. A failure before
// CURRENT leaves the parent live.
func (s *sealedState) write(ctx context.Context, m manifest.Manifest, runs []*planRun, stats *Stats) error {
	var uncommitted []string
	for _, p := range runs {
		if !p.commits() {
			uncommitted = append(uncommitted, p.plan.name)
			continue
		}
		p.recordFiles()
	}
	s.phases.Commit(s.commit, uncommitted)
	result, err := s.commit.Write(context.WithoutCancel(ctx), s.ledger, s.header, m)
	stats.Generation = result.Generation != "" && (s.gen == nil || result.Generation != s.gen.Name)
	stats.Written, stats.Size = result.Written, result.Size
	if err != nil {
		return fmt.Errorf("workspace: record the run: %w", err)
	}
	return nil
}

// damage is the sealed state found damaged in a run: a record, a block
// or a region of the generation that does not read whole. [Workspace.Run]
// discards the run that met it and runs again cold.
type damage struct {
	err error
}

// Error returns the message of the error that found the damage.
func (d *damage) Error() string { return d.err.Error() }

// Unwrap returns the error that found the damage, which wraps
// [state.ErrDamaged].
func (d *damage) Unwrap() error { return d.err }

// damaged returns err as [damage] where it wraps [state.ErrDamaged], and
// err otherwise.
func damaged(err error) error {
	if errors.Is(err, state.ErrDamaged) {
		return &damage{err: err}
	}
	return err
}

// executableStat is the stat an executable's digest was taken under.
type executableStat struct {
	path    string
	size    int64
	modTime int64
}

// executableDigests caches the digest of the running executable for
// each stat the process met it under.
//
// # Concurrency
//
// An executableDigests is safe for concurrent use: one mutex guards the
// cache and the hash.
type executableDigests struct {
	mu   sync.Mutex
	seen map[executableStat][sha256.Size]byte
}

// executables is the process's cache of executable digests.
var executables = &executableDigests{seen: map[executableStat][sha256.Size]byte{}}

// of returns the digest of the executable a record names, hashing the
// file the first time the process meets its stat.
//
// Error modes: the error of a file that does not open or does not read.
func (d *executableDigests) of(exe state.Executable) ([sha256.Size]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := executableStat{path: exe.Path, size: exe.Size, modTime: exe.ModTime.UnixNano()}
	if sum, held := d.seen[key]; held {
		return sum, nil
	}
	f, err := os.Open(exe.Path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	d.seen[key] = sum
	return sum, nil
}

// executable returns the record of the running executable: its path,
// size and modification time, and the SHA-256 of its bytes. It hashes
// the executable only where the path, the size or the modification time
// differs from recorded's, and once per process for one stat.
//
// Error modes: the error of an executable whose path does not resolve,
// or whose file does not stat, open or read.
func executable(recorded state.Executable) (state.Executable, error) {
	path, err := os.Executable()
	if err != nil {
		return state.Executable{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return state.Executable{}, err
	}
	exe := state.Executable{Path: path, Size: info.Size(), ModTime: info.ModTime()}
	if recorded.Path == exe.Path && recorded.Size == exe.Size && recorded.ModTime.Equal(exe.ModTime) {
		exe.Digest = recorded.Digest
		return exe, nil
	}
	exe.Digest, err = executables.of(exe)
	return exe, err
}

// openSealed opens the live generation of the run's ledger, checks its
// header against the run, and starts the next generation where the run
// writes one. A run over a caller's graph and a composition without a
// ledger read and write nothing. A run with Cold set, and a ledger
// without a live generation, run cold and report nothing. A generation
// that does not open, and one of another composition or executable, is
// reported under [ColdState] and the run runs cold. So is an executable
// the run cannot read, and the run then writes no generation.
//
// Error modes: a ledger that fails to read, wrapped.
func (w *Workspace) openSealed(ctx context.Context, rec *record, in Input, sink *diag.Sink) (*sealedState, error) {
	if rec.ledger == nil || in.Tree == nil {
		return &sealedState{}, nil
	}
	var gen *state.Generation
	if !in.Cold {
		var err error
		gen, err = state.Open(ctx, rec.ledger)
		if errors.Is(err, state.ErrDamaged) || errors.Is(err, state.ErrFormat) {
			w.coldState(sink, "the sealed state does not open: %v", err)
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("workspace: open the sealed state: %w", err)
		}
	}
	var recorded state.Executable
	if gen != nil {
		recorded = gen.Header.Executable
	}
	exe, err := executable(recorded)
	if err != nil {
		w.coldState(sink, "the run cannot read its executable, so it cannot tell which code wrote the state: %v", err)
		return &sealedState{}, nil
	}
	s := &sealedState{ledger: rec.ledger, header: state.Header{Composition: w.composition(), Executable: exe}}
	if gen != nil && w.usable(gen, s.header, sink) {
		s.gen, s.prior = gen, gen.Load(ctx)
	}
	if !in.Dry && rec.digests != nil {
		s.commit = state.NewCommit(s.gen, rec.digests)
		s.recorder = &state.Recorder{}
	}
	return s, nil
}

// usable reports whether a generation's header records the run's own
// composition and executable, and reports the difference under
// [ColdState] where it does not.
func (w *Workspace) usable(gen *state.Generation, run state.Header, sink *diag.Sink) bool {
	if gen.Header.Composition != run.Composition {
		w.coldState(sink, "the composition changed since the sealed state was recorded")
		return false
	}
	if gen.Header.Executable.Digest != run.Executable.Digest {
		w.coldState(sink, "the executable changed since the sealed state was recorded")
		return false
	}
	return true
}

// coldState reports that the run ignored the sealed state and why, at
// the state directory's CURRENT.
func (w *Workspace) coldState(sink *diag.Sink, format string, args ...any) {
	at := position.Pos{File: ledger.StateDir(w.brand) + "/" + currentName}
	sink.Infof(ColdState, at, diag.PhaseLoad, format, args...)
}

// recordFiles records each file that the plan staged into the plan's
// lane: its manifest entry with the digest of its commit, its package,
// its group, where a finding about it is positioned, the export rows of
// its declarations for a plan marked exported, and the names it
// declares. A plan without a lane records nothing.
func (p *planRun) recordFiles() {
	if p.lane == nil {
		return
	}
	var exported map[string][]plugin.ExportedSymbol
	if p.plan.exported {
		exported = map[string][]plugin.ExportedSymbol{}
		for _, s := range p.export.Symbols {
			exported[s.File] = append(exported[s.File], s)
		}
	}
	for i, e := range p.entries() {
		f := &p.files[i]
		p.lane.Artifact(state.Artifact{
			Entry: e, Pkg: f.pkg, Group: f.group, At: f.at, First: describe(f.first),
			Export: exported[f.path], Names: f.names,
		})
	}
}

// moduleCounts returns how many packages name each module: every package
// that has both module facts, the module and its root, counted under its
// language, its module and its root. A module rooted outside the
// workspace counts too, where the layout's list of modules leaves it out.
func moduleCounts(g *store.Graph, facts *meta.Facts, k meta.KernelKeys) map[plugin.Module]int {
	out := map[plugin.Module]int{}
	for p := range g.Packages() {
		if m, named := moduleOf(facts, p.ID, k); named {
			out[m]++
		}
	}
	return out
}
