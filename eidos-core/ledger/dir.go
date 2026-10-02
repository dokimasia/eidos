// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.dokimi.dev/eidos/core/internal/stagefile"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// The permissions a commit writes with, the ones the output sinks
// write generated files and their directories with.
const (
	filePerm = 0o644
	dirPerm  = 0o755
)

// Dir is the ledger of one workspace root's state directory, .<brand>/.
// Each call opens the root, resolves every path inside it and closes
// it, so a Dir keeps no file open between calls.
type Dir struct {
	root  string
	brand output.Brand
	// name is the base name of the root, which a manifest without a
	// workspace name is recorded under.
	name string
}

var _ Ledger = (*Dir)(nil)

// OpenDir returns the ledger of the state directory .<brand>/ under
// root, which it creates on the first commit. The manifest is
// .<brand>/manifest.json, and a manifest without a workspace name is
// recorded under the base name of root. A commit writes the manifest
// to a staging file beside it, syncs it, renames it over the manifest
// and syncs the directory, so a reader sees the old record or the new
// one.
//
// It refuses a brand outside [output.Brand.Valid] and a root that is
// not a directory.
func OpenDir(root string, brand output.Brand) (*Dir, error) {
	if !brand.Valid() {
		return nil, fmt.Errorf(
			"ledger: %q is not a brand: a lowercase letter, then lowercase letters, digits and hyphens",
			string(brand),
		)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("ledger: resolving the workspace root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("ledger: opening the workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ledger: the workspace root %s is not a directory", root)
	}
	return &Dir{root: abs, brand: brand, name: filepath.Base(abs)}, nil
}

// BeginRun reads the recorded manifest. A missing state directory and
// a missing manifest are a workspace no run has committed, and return
// the empty manifest. A manifest that does not read returns the empty
// manifest and an error naming its path.
func (d *Dir) BeginRun(context.Context) (manifest.Manifest, error) {
	at := ManifestPath(d.brand)
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return empty(), fmt.Errorf("ledger: opening the workspace root: %w", err)
	}
	defer r.Close()
	data, err := r.ReadFile(at)
	if errors.Is(err, fs.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return empty(), fmt.Errorf("ledger: reading %s: %w", at, err)
	}
	m, err := manifest.Decode(data)
	if err != nil {
		return empty(), fmt.Errorf("ledger: reading %s: %w", at, err)
	}
	return m, nil
}

// CommitRun records the run's manifest under the root's base name
// where it names no workspace. Bytes equal to the recorded manifest's
// write nothing. Anything else creates the state directory where it is
// missing, replaces the manifest through a synced staging file, and
// syncs the directory so the rename survives a crash.
func (d *Dir) CommitRun(_ context.Context, m manifest.Manifest) error {
	if m.Workspace == "" {
		m.Workspace = d.name
	}
	data, err := manifest.Encode(m)
	if err != nil {
		return fmt.Errorf("ledger: %w", err)
	}
	at := ManifestPath(d.brand)
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return fmt.Errorf("ledger: opening the workspace root: %w", err)
	}
	defer r.Close()
	if held, err := r.ReadFile(at); err == nil && bytes.Equal(held, data) {
		return nil
	}
	if err := r.MkdirAll(StateDir(d.brand), dirPerm); err != nil {
		return fmt.Errorf("ledger: making the state directory: %w", err)
	}
	if err := stagefile.Replace(r, at, data, filePerm); err != nil {
		return fmt.Errorf("ledger: committing %s: %w", at, err)
	}
	return syncDir(r, StateDir(d.brand))
}

// syncDir syncs a directory inside the root, so a rename into it is on
// disk before the call returns.
func syncDir(r *os.Root, dir string) error {
	f, err := r.Open(dir)
	if err != nil {
		return fmt.Errorf("ledger: opening %s to sync it: %w", dir, err)
	}
	err = f.Sync()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("ledger: syncing %s: %w", dir, err)
	}
	return nil
}
