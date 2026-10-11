// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"sync"
	"sync/atomic"
)

// Workers is how many goroutines a commit writes, reads and syncs on.
// Without a sync, 32 workers wrote 30,000 staging files of 3 KiB in 0.22
// to 0.24 s on btrfs.
const Workers = 32

// dirPerm is the mode of a directory that a commit creates, the mode the
// output sinks and the ledger create their directories with.
const dirPerm = 0o755

// ErrNotApplied is the error of each name of a commit whose barrier
// failed before the renames. The commit did not rename or remove a file,
// so every target keeps its old bytes.
var ErrNotApplied = errors.New("stagefile: a barrier failed before the renames")

// Group is a set of writes and removals that one commit stages under a
// root. [Group.Commit] applies them through a [Barrier]: a reader of a
// target sees its old bytes or its new bytes, and after a crash of the
// machine each target contains its old bytes or its new bytes.
//
// # Concurrency
//
// A Group serves one commit, and one goroutine calls its methods. Commit
// writes and syncs on goroutines of its own.
//
// # Allocation contract
//
// Write and Remove append one entry of 48 bytes on a 64-bit system to the
// group's list, which grows as a slice does. Commit allocates for each
// name what [os.Root] spends on its paths, and what its five steps keep:
// the staging names, the errors, the changed directories and the
// goroutines of the pool.
type Group struct {
	root    *os.Root
	staging Staging
	barrier Barrier
	entries []entry
}

// entry is one staged write or removal of a name. The entry of a write
// has the bytes and the mode of the write, and keeps the bytes without a
// copy. The entry of a removal has remove set.
type entry struct {
	name   string
	body   []byte
	perm   fs.FileMode
	remove bool
}

// writing is the state that the workers of step 2 share: the group, the
// staging name and the error of each entry, indexed as the entries are,
// and every directory that the workers created, under a lock.
type writing struct {
	group   *Group
	stages  []string
	errs    []error
	mu      sync.Mutex
	created []string
}

// NewGroup returns an empty group over r. The group sets the names of its
// staging files by s, and makes its writes and removals durable through
// b. It allocates the group.
func NewGroup(r *os.Root, s Staging, b Barrier) *Group {
	return &Group{root: r, staging: s, barrier: b}
}

// Write stages body for name with perm. The directory of name does not
// have to exist, because Commit creates it. Write keeps body and does not
// copy it, so the caller leaves body unchanged until the commit. A group
// takes each name once.
func (g *Group) Write(name string, body []byte, perm fs.FileMode) {
	g.entries = append(g.entries, entry{name: name, body: body, perm: perm})
}

// Remove stages the removal of the file at name. A group takes each name
// once.
func (g *Group) Remove(name string) {
	g.entries = append(g.entries, entry{name: name, remove: true})
}

// Commit applies the staged writes and removals in five steps:
//
//  1. It calls [Barrier.Start] with the number of names.
//  2. [Workers] goroutines create the missing directories of each write
//     one level at a time, write its staging file, call [Barrier.File] on
//     it and close it.
//  3. It calls [Barrier.Order].
//  4. One goroutine renames each staging file over its target and removes
//     the file of each removal, in name order.
//  5. It calls [Barrier.Durable] with the directory of each name that step
//     4 changed and the parent of each directory that step 2 created.
//
// A failure in step 2 or step 4 fails its name and removes its staging
// file, and the other names continue. Where Start or Order fails, Commit
// renames nothing, removes every staging file, and returns
// [ErrNotApplied] for each name that did not fail before. Where Durable
// fails, the renames are visible, and Commit returns the error. A commit
// that calls Start also calls [Barrier.Close] once at its end, after a
// failure too. Commit does not call the barrier for a group without a
// name. It does not read a context, so it runs to its end once it begins.
//
// Error modes: the error of each name that failed, keyed by the name, and
// the error of Start, Order, Durable or Close, each wrapped. A failed
// removal of a staging file is joined into the error of its name, or into
// the error of the barrier that failed.
//
// # Allocation contract
//
// Commit allocates the staging name and the error of each name, the
// changed directories, the goroutines of the pool, and what [os.Root]
// spends on the paths of each write and each removal.
func (g *Group) Commit() (failed map[string]error, err error) {
	if len(g.entries) == 0 {
		return failed, err
	}
	slices.SortFunc(g.entries, func(a, b entry) int { return cmp.Compare(a.name, b.name) })
	errs := make([]error, len(g.entries))
	if err := g.barrier.Start(g.root, len(g.entries)); err != nil {
		return g.unapplied(errs), errors.Join(fmt.Errorf("stagefile: start the barrier: %w", err), g.close())
	}
	w := &writing{group: g, stages: make([]string, len(g.entries)), errs: errs}
	Each(len(g.entries), w.entry)
	if err := g.barrier.Order(); err != nil {
		return g.unapplied(errs), errors.Join(
			fmt.Errorf("stagefile: pass the order barrier: %w", err), g.discard(w.stages), g.close(),
		)
	}
	dirs := g.apply(w.stages, errs)
	if err := g.barrier.Durable(changed(dirs, w.created)); err != nil {
		return g.failures(errs), errors.Join(fmt.Errorf("stagefile: pass the durability barrier: %w", err), g.close())
	}
	return g.failures(errs), g.close()
}

// stage writes the staging file of one write, calls the barrier's File on
// it and closes it. It returns the staging file's name. It removes a
// staging file that it fails to write, sync or close, and joins the error
// of that removal.
func (g *Group) stage(e *entry) (string, error) {
	f, name, err := g.staging.open(g.root, e.name, e.perm)
	if err != nil {
		return "", err
	}
	_, err = f.Write(e.body)
	if err == nil {
		err = g.barrier.File(f)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", errors.Join(fmt.Errorf("stagefile: %w", err), g.root.Remove(name))
	}
	return name, nil
}

// apply runs step 4. It renames each staging file over its target and
// removes the file of each removal, in name order, and returns the
// directory of each name that it changed. It records the error of each
// name that failed in errs, and removes the staging file of a rename that
// failed.
func (g *Group) apply(stages []string, errs []error) []string {
	var dirs []string
	for i := range g.entries {
		e := &g.entries[i]
		if errs[i] != nil {
			continue
		}
		var err error
		if e.remove {
			err = g.root.Remove(e.name)
		} else if err = g.root.Rename(stages[i], e.name); err != nil {
			err = errors.Join(err, g.root.Remove(stages[i]))
		}
		if err != nil {
			errs[i] = fmt.Errorf("stagefile: %w", err)
			continue
		}
		if dir := path.Dir(e.name); len(dirs) == 0 || dirs[len(dirs)-1] != dir {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// discard removes the staging file of each write after a failed order
// barrier, and joins the errors of the removals.
func (g *Group) discard(stages []string) error {
	var errs []error
	for _, stage := range stages {
		if stage != "" {
			errs = append(errs, g.root.Remove(stage))
		}
	}
	return errors.Join(errs...)
}

// close calls the barrier's Close, and wraps its error.
func (g *Group) close() error {
	if err := g.barrier.Close(); err != nil {
		return fmt.Errorf("stagefile: close the barrier: %w", err)
	}
	return nil
}

// failures returns the error of each name that failed, keyed by the name,
// and nil where no name failed.
func (g *Group) failures(errs []error) map[string]error {
	var failed map[string]error
	for i, err := range errs {
		if err == nil {
			continue
		}
		if failed == nil {
			failed = map[string]error{}
		}
		failed[g.entries[i].name] = err
	}
	return failed
}

// unapplied returns the failures of a commit whose barrier failed before
// the renames: the error of each name that failed in step 2, and
// [ErrNotApplied] for every other name.
func (g *Group) unapplied(errs []error) map[string]error {
	failed := make(map[string]error, len(g.entries))
	for i := range g.entries {
		failed[g.entries[i].name] = cmp.Or(errs[i], ErrNotApplied)
	}
	return failed
}

// entry runs step 2 for the entry at index i. Where the entry is a write,
// it creates the missing directories of its target, writes its staging
// file and records the staging file's name. It records the error of a
// write that fails.
func (w *writing) entry(i int) {
	e := &w.group.entries[i]
	if e.remove {
		return
	}
	made, err := mkdirs(w.group.root, path.Dir(e.name), nil)
	if len(made) > 0 {
		w.mu.Lock()
		w.created = append(w.created, made...)
		w.mu.Unlock()
	}
	if err == nil {
		w.stages[i], err = w.group.stage(e)
	}
	w.errs[i] = err
}

// Each calls do with each index below n on up to [Workers] goroutines, and
// returns once every call has returned. The calls run in no fixed order,
// so do writes only to the index it receives or under a lock of its own.
// Each calls do on the caller's goroutine for an n of one. It allocates
// the pool's counter and one goroutine for each worker that it starts,
// and nothing for an n of zero or one.
func Each(n int, do func(i int)) {
	if n == 1 {
		do(0)
		return
	}
	var pool struct {
		next atomic.Int64
		wg   sync.WaitGroup
	}
	for range min(Workers, n) {
		pool.wg.Go(func() {
			for i := int(pool.next.Add(1)) - 1; i < n; i = int(pool.next.Add(1)) - 1 {
				do(i)
			}
		})
	}
	pool.wg.Wait()
}

// changed returns the directories that the durability barrier makes
// durable, sorted and without a repeat: each directory that a rename or a
// removal changed, and the parent of each directory that a write created.
func changed(dirs, created []string) []string {
	for _, dir := range created {
		dirs = append(dirs, path.Dir(dir))
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// mkdirs creates dir and each missing directory above it inside the root,
// one level at a time, and appends each directory that it created to
// made. A directory that another goroutine creates first is not in made.
//
// Error modes: a path of dir that is not a directory, and the error of a
// stat or a creation that fails, each wrapped.
func mkdirs(r *os.Root, dir string, made []string) ([]string, error) {
	if dir == "." {
		return made, nil
	}
	info, err := r.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		return made, nil
	case err == nil:
		return made, fmt.Errorf("stagefile: %s is not a directory", dir)
	case !errors.Is(err, fs.ErrNotExist):
		return made, fmt.Errorf("stagefile: %w", err)
	}
	made, err = mkdirs(r, path.Dir(dir), made)
	if err != nil {
		return made, err
	}
	switch err := r.Mkdir(dir, dirPerm); {
	case err == nil:
		return append(made, dir), nil
	case errors.Is(err, fs.ErrExist):
		return made, nil
	default:
		return made, fmt.Errorf("stagefile: %w", err)
	}
}
