// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"go.dokimi.dev/eidos/core/output"
)

// manifestDir is the directory of the manifest's documents inside the
// state directory.
const manifestDir = "manifest"

// Ledger stores one workspace's record as named blobs: the generations
// of the sealed state and their segments, the manifest's documents, and
// the parse memo's entries. The kernel encodes every blob, and a Ledger
// stores bytes.
//
// A name is slash-separated and relative, and has no empty, "." or ".."
// element. A Ledger serves one run and is safe for concurrent use by
// that run's goroutines.
type Ledger interface {
	// Read returns a blob whole. A name nothing wrote returns an error
	// wrapping [fs.ErrNotExist].
	Read(ctx context.Context, name string) ([]byte, error)
	// ReadAt reads len(p) bytes of a blob from offset off, under the
	// contract of [io.ReaderAt]. A name nothing wrote returns an error
	// wrapping [fs.ErrNotExist].
	ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error)
	// Write replaces a blob atomically and durably. A reader sees the old
	// bytes or the new ones, and once Write returns, a crash of the
	// machine does not lose the new ones.
	Write(ctx context.Context, name string, b []byte) error
	// Put replaces a blob atomically without a sync. A crash of the
	// process loses nothing, and a crash of the machine can leave the old
	// bytes, the new ones or an empty blob. The memo writes its entries,
	// which check their own bytes, through Put.
	Put(ctx context.Context, name string, b []byte) error
	// Touch sets a blob's modification time to the current time. A name
	// nothing wrote returns an error wrapping [fs.ErrNotExist].
	Touch(ctx context.Context, name string) error
	// Remove deletes a blob. A name nothing wrote is not an error.
	Remove(ctx context.Context, name string) error
	// List returns the blobs whose names begin with dir and a slash,
	// sorted by name, and none where nothing wrote under dir.
	List(ctx context.Context, dir string) ([]Blob, error)
}

// Blob is one blob that [Ledger.List] returns.
type Blob struct {
	// Name is the blob's name, as Write or Put named it.
	Name string
	// Size is the blob's length in bytes.
	Size int64
	// ModTime is when the blob was last written or touched.
	ModTime time.Time
}

// StateDir returns the brand's state directory, workspace-relative:
// .<brand>.
func StateDir(brand output.Brand) string { return "." + string(brand) }

// ManifestPath returns the directory the brand's manifest documents are
// recorded in, workspace-relative and slash-separated:
// .<brand>/manifest.
func ManifestPath(brand output.Brand) string { return path.Join(StateDir(brand), manifestDir) }

// checkName returns an error wrapping [fs.ErrInvalid] for a name that is
// not slash-separated and relative, or that has an empty, "." or ".."
// element, and nil for a name a ledger stores under.
func checkName(name string) error {
	if name == "." || !fs.ValidPath(name) || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("ledger: %q is not a blob name: %w", name, fs.ErrInvalid)
	}
	return nil
}
