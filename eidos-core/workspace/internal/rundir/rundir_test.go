// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rundir_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// The brands the cases frame files under, and the files they place.
const (
	ownBrand   output.Brand = "fixture"
	otherBrand output.Brand = "other"
	nestedFile              = "a/b.txt"
	topFile                 = "a.txt"
	deepFile                = "c/d/e.txt"
)

// placed writes content at a slash path under root.
func placed(t *testing.T, root, path, content string) {
	t.Helper()

	at := filepath.Join(root, filepath.FromSlash(path))
	assert.NoError(t, os.MkdirAll(filepath.Dir(at), 0o755), "the file's directory is made")
	assert.NoError(t, os.WriteFile(at, []byte(content), 0o644), "the file is placed")
}

// stamped returns a one-line body framed under a brand.
func stamped(t *testing.T, brand output.Brand, path string) string {
	t.Helper()

	c, err := output.NewContract(brand, plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(t, err, "the contract builds")
	b, err := c.Stamp(plugin.RenderedFile{Path: path, Body: []byte("type Row struct{}\n"), Plugins: []plugin.ID{"gen"}})
	assert.NoError(t, err, "the body stamps")
	return string(b)
}

// A conformance suite reads a run's directory through these helpers,
// so what each returns, and where each stops a check, is contract.
func TestRundir(t *testing.T) {
	t.Parallel()

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every file in the walk's lexical order", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			for _, path := range []string{deepFile, topFile, nestedFile} {
				placed(t, root, path, "x\n")
			}
			assert.Equal(t, rundir.Files(t, root), []string{nestedFile, topFile, deepFile}, "the files under root")
		})

		t.Run("stops the check where the directory does not walk", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			rundir.Files(rec, filepath.Join(t.TempDir(), "absent"))
			assert.True(t, rec.Failed(), "the check stops")
		})
	})

	t.Run("Path", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a slash path under root in the system's spelling", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rundir.Path("root", nestedFile), filepath.Join("root", "a", "b.txt"), "the joined path")
		})
	})

	t.Run("Framed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the text of each file under the brand's frame", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			own := stamped(t, ownBrand, topFile)
			placed(t, root, topFile, own)
			assert.Equal(t, rundir.Framed(t, root, ownBrand), map[string]string{topFile: own}, "the framed files")
		})

		t.Run("leaves out a file another brand framed", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed(t, root, topFile, stamped(t, otherBrand, topFile))
			assert.Length(t, rundir.Framed(t, root, ownBrand), 0, "the brand's framed files")
		})

		t.Run("leaves out a file without a frame", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed(t, root, topFile, "hand-written\n")
			assert.Length(t, rundir.Framed(t, root, ownBrand), 0, "the brand's framed files")
		})
	})

	t.Run("Texts", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each wanted file as text under its key", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rundir.Texts(map[string][]byte{topFile: []byte("x\n")}), map[string]string{topFile: "x\n"},
				"the wanted files as text")
		})
	})

	t.Run("Digest", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a file's digest as a record's entry spells it", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed(t, root, topFile, "x\n")
			sum := sha256.Sum256([]byte("x\n"))
			assert.Equal(t, rundir.Digest(t, root, topFile), "sha256:"+hex.EncodeToString(sum[:]), "the digest")
		})

		t.Run("stops the check where the file does not read", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			rundir.Digest(rec, t.TempDir(), topFile)
			assert.True(t, rec.Failed(), "the check stops")
		})
	})

	t.Run("Record", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the record the ledger committed", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			l, err := ledger.OpenDir(root, ownBrand)
			assert.NoError(t, err, "the ledger opens")
			want := manifest.Manifest{Version: manifest.Version, Workspace: "platform"}
			assert.NoError(t, l.CommitRun(t.Context(), want), "the record commits")
			assert.True(t, rundir.Record(t, root, ownBrand).Equal(want), "the record reads back")
		})

		t.Run("stops the check where the directory contains no record", func(t *testing.T) {
			t.Parallel()

			rec := assert.NewRecorder()
			rundir.Record(rec, t.TempDir(), ownBrand)
			assert.True(t, rec.Failed(), "the check stops")
		})

		t.Run("stops the check where the record does not decode", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed(t, root, ledger.ManifestPath(ownBrand), "not a record\n")
			rec := assert.NewRecorder()
			rundir.Record(rec, root, ownBrand)
			assert.True(t, rec.Failed(), "the check stops")
		})
	})
}
