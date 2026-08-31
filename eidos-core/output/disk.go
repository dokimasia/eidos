// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package output

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
)

// The permissions a commit writes with: generated files are
// ordinary source files, and their directories ordinary
// directories.
const (
	filePerm = 0o644
	dirPerm  = 0o755
)

// Disk stages for a directory tree and commits into it. The root
// is opened once and every staged path resolves inside it, so a
// symlink pointing out of the tree does not escape it either: the
// jail is the operating system's.
type Disk struct {
	staging
	root *os.Root
}

// NewDisk opens a sink over an existing directory. It refuses a
// root it cannot open, because a sink over nothing writes nowhere.
//
// Commit and Discard close the root; the sink serves one staging.
func NewDisk(root string) (*Disk, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("output: opening the sink root: %w", err)
	}
	return &Disk{root: r}, nil
}

// Write stages one file. Nothing reaches the tree until Commit.
func (d *Disk) Write(path string, body []byte) error { return d.stage(path, body) }

// Commit writes every staged file, in path order: identical bytes
// leave the file and its mtime untouched, and anything else is
// written to a staging file and renamed over the target, so a
// reader sees the old file or the new one and never half of
// either. A file that fails is one error and the rest still
// commit.
func (d *Disk) Commit() ([]Written, error) {
	if err := d.finish(); err != nil {
		return nil, err
	}
	defer d.root.Close()

	paths := d.paths()
	records := make([]Written, 0, len(paths))
	var faults []error
	for _, p := range paths {
		record, err := d.commit(p, d.files[p])
		if err != nil {
			faults = append(faults, err)
			continue
		}
		records = append(records, record)
	}
	return records, errors.Join(faults...)
}

// Discard drops the staging and closes the root.
func (d *Disk) Discard() error {
	if err := d.finish(); err != nil {
		return err
	}
	clear(d.files)
	return d.root.Close()
}

// commit writes one staged file.
func (d *Disk) commit(at string, body []byte) (Written, error) {
	action := ActionCreated
	switch existing, err := d.root.ReadFile(at); {
	case err == nil && bytes.Equal(existing, body):
		return Written{Path: at, Action: ActionUnchanged, Hash: digest(body)}, nil
	case err == nil:
		action = ActionUpdated
	case !errors.Is(err, fs.ErrNotExist):
		return Written{}, fmt.Errorf("output: reading %q before writing it: %w", at, err)
	}

	if dir := path.Dir(at); dir != "." {
		if err := d.root.MkdirAll(dir, dirPerm); err != nil {
			return Written{}, fmt.Errorf("output: making the directory for %q: %w", at, err)
		}
	}
	// A staging file a killed run left is stale by definition, and
	// writing truncates it.
	stage := at + stageSuffix
	if err := d.root.WriteFile(stage, body, filePerm); err != nil {
		return Written{}, fmt.Errorf("output: staging %q: %w", at, err)
	}
	if err := d.root.Rename(stage, at); err != nil {
		_ = d.root.Remove(stage)
		return Written{}, fmt.Errorf("output: committing %q: %w", at, err)
	}
	return Written{Path: at, Action: action, Hash: digest(body)}, nil
}
