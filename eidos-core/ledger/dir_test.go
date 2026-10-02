// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// The fixture record's plan, path and digest.
const (
	planName = "go-services"
	stubPath = "svc/store_stub.go"
)

// past is the mtime a fixture sets on a record, so a commit that writes
// it moves the time.
var past = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// recorded returns a well-formed manifest naming a workspace.
func recorded(workspace string) manifest.Manifest {
	return manifest.Manifest{
		Version:   manifest.Version,
		Workspace: workspace,
		Files: []manifest.Entry{{
			Path: stubPath, Plan: planName, Hash: "sha256:" + strings.Repeat("ab", 32),
		}},
	}
}

// opened returns a ledger over a fresh workspace root and the root.
func opened(t *testing.T) (*ledger.Dir, string) {
	t.Helper()

	root := t.TempDir()
	d, err := ledger.OpenDir(root, brand)
	assert.NoError(t, err, "the ledger opens")
	return d, root
}

// manifestFile returns the manifest's absolute path under a root.
func manifestFile(root string) string {
	return filepath.Join(root, filepath.FromSlash(ledger.ManifestPath(brand)))
}

// The state directory's ledger records the manifest atomically, under
// the root's name where it states none, and writes nothing for a
// manifest it already records.
func TestDir(t *testing.T) {
	t.Parallel()

	t.Run("OpenDir", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for an invalid brand", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.OpenDir(t.TempDir(), "Acme")
			assert.HasError(t, err, "the brand is refused")
		})

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()

			_, err := ledger.OpenDir(filepath.Join(t.TempDir(), "missing"), brand)
			assert.HasError(t, err, "the root is refused")
		})

		t.Run("returns an error for a root that is a file", func(t *testing.T) {
			t.Parallel()

			file := filepath.Join(t.TempDir(), "file")
			assert.NoError(t, os.WriteFile(file, nil, 0o600), "the file is written")
			_, err := ledger.OpenDir(file, brand)
			assert.HasError(t, err, "the root is refused")
		})
	})

	t.Run("BeginRun", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the empty manifest for a root without a state directory", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			got, err := d.BeginRun(t.Context())
			assert.NoError(t, err, "no record is no error")
			assert.True(t, got.Equal(manifest.Manifest{Version: manifest.Version}), "the empty manifest")
		})

		t.Run("returns the manifest the last commit recorded", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the record commits")
			got, err := d.BeginRun(t.Context())
			assert.NoError(t, err, "the record reads")
			assert.True(t, got.Equal(recorded("platform")), "the recorded manifest")
		})

		t.Run("returns an error naming the path for a record that does not decode", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(filepath.Dir(manifestFile(root)), 0o700), "the state directory is made")
			assert.NoError(t, os.WriteFile(manifestFile(root), []byte("not json"), 0o600), "a broken record")
			got, err := d.BeginRun(t.Context())
			assert.ErrorIs(t, err, manifest.ErrUnsupported, "the record does not read")
			assert.Contains(t, err.Error(), ledger.ManifestPath(brand), "the error names the record")
			assert.True(t, got.Equal(manifest.Manifest{Version: manifest.Version}), "beside the empty manifest")
		})

		t.Run("returns an error for a record that is a directory", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(manifestFile(root), 0o700), "the record's path is a directory")
			_, err := d.BeginRun(t.Context())
			assert.HasError(t, err, "the record does not read")
		})

		t.Run("returns an error for a root removed after the ledger opened", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.RemoveAll(root), "the root is removed")
			_, err := d.BeginRun(t.Context())
			assert.HasError(t, err, "the root does not open")
		})
	})

	t.Run("CommitRun", func(t *testing.T) {
		t.Parallel()

		t.Run("records a manifest without a workspace under the root's base name", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, d.CommitRun(t.Context(), recorded("")), "the record commits")
			got, err := d.BeginRun(t.Context())
			assert.NoError(t, err, "the record reads")
			assert.Equal(t, got.Workspace, filepath.Base(root), "the root's base name")
		})

		t.Run("records a manifest's own workspace name", func(t *testing.T) {
			t.Parallel()

			d, _ := opened(t)
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the record commits")
			got, err := d.BeginRun(t.Context())
			assert.NoError(t, err, "the record reads")
			assert.Equal(t, got.Workspace, "platform", "the stated name")
		})

		t.Run("writes nothing for a manifest equal to the record", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the record commits")
			assert.NoError(t, os.Chtimes(manifestFile(root), past, past), "the record ages")
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the equal record commits")
			info, err := os.Stat(manifestFile(root))
			assert.NoError(t, err, "the record stats")
			assert.True(t, info.ModTime().Equal(past), "the record's mtime does not move")
		})

		t.Run("replaces a record that does not decode", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(filepath.Dir(manifestFile(root)), 0o700), "the state directory is made")
			assert.NoError(t, os.WriteFile(manifestFile(root), []byte("not json"), 0o600), "a broken record")
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the record commits")
			got, err := d.BeginRun(t.Context())
			assert.NoError(t, err, "the record reads")
			assert.True(t, got.Equal(recorded("platform")), "the new record")
		})

		t.Run("leaves the manifest alone in the state directory", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, d.CommitRun(t.Context(), recorded("platform")), "the record commits")
			listed, err := os.ReadDir(filepath.Dir(manifestFile(root)))
			assert.NoError(t, err, "the state directory reads")
			assert.Length(t, listed, 1, "no staging file remains")
			assert.Equal(t, listed[0].Name(), filepath.Base(manifestFile(root)), "the manifest")
		})

		t.Run("returns an error for a manifest that breaks the format", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			broken := recorded("platform")
			broken.Files = append(broken.Files, broken.Files[0])
			assert.HasError(t, d.CommitRun(t.Context(), broken), "the record is refused")
			_, err := os.Stat(filepath.Dir(manifestFile(root)))
			assert.True(t, os.IsNotExist(err), "the state directory is not made")
		})

		t.Run("returns an error for a state directory path that is a file", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.WriteFile(filepath.Join(root, ledger.StateDir(brand)), nil, 0o600),
				"the state directory's path is a file")
			assert.HasError(t, d.CommitRun(t.Context(), recorded("platform")), "the record is refused")
		})

		t.Run("returns an error for a manifest path that is a directory", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.MkdirAll(filepath.Join(manifestFile(root), "kept"), 0o700),
				"the manifest's path is a directory that is not empty")
			err := d.CommitRun(t.Context(), recorded("platform"))
			assert.HasError(t, err, "the record is refused")
			assert.Contains(t, err.Error(), ledger.ManifestPath(brand), "the error names the record")
		})

		t.Run("returns an error for a root removed after the ledger opened", func(t *testing.T) {
			t.Parallel()

			d, root := opened(t)
			assert.NoError(t, os.RemoveAll(root), "the root is removed")
			assert.HasError(t, d.CommitRun(t.Context(), recorded("platform")), "the root does not open")
		})
	})
}
