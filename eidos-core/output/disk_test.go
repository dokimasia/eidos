// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// past is the mtime the unchanged check pins against: a fixed
// instant, so the assertion holds whatever granularity the
// filesystem keeps.
var past = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

// disk opens a sink over root, failing the test where it cannot.
func disk(t *testing.T, root string) output.Sink {
	t.Helper()

	d, err := output.NewDisk(root)
	assert.NoError(t, err, "the root opens")
	return d
}

// committed stages one file and commits it, returning the record.
func committed(t *testing.T, root, path, body string) output.Written {
	t.Helper()

	s := disk(t, root)
	assert.NoError(t, s.Write(path, []byte(body)), "the staging takes")
	got, err := s.Commit()
	assert.NoError(t, err, "the commit runs")
	assert.Length(t, got, 1, "one staged file, one record")
	return got[0]
}

// The disk sink is where determinism meets the filesystem: bytes
// appear whole or not at all, unchanged files are not touched, and
// nothing reaches a path outside the root.
func TestDisk(t *testing.T) {
	t.Parallel()

	t.Run("NewDisk refuses a root that is not there", func(t *testing.T) {
		t.Parallel()

		_, err := output.NewDisk(filepath.Join(t.TempDir(), "absent"))
		assert.HasError(t, err, "a sink over nothing writes nowhere")
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("creates the parent directories and the file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := committed(t, root, "svc/api/store.go", "package api\n")
			assert.Equal(t, w.Action, output.ActionCreated, "the path was not there")

			got, err := os.ReadFile(filepath.Join(root, "svc", "api", "store.go"))
			assert.NoError(t, err, "the file is on disk")
			assert.Equal(t, string(got), "package api\n", "carrying the staged bytes")
		})

		t.Run("keeps the staging invisible", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			s := disk(t, root)
			assert.NoError(t, s.Write("store.go", []byte("package svc\n")),
				"the staging takes")
			entries, err := os.ReadDir(root)
			assert.NoError(t, err, "the root reads")
			assert.Length(t, entries, 0,
				"a failed or abandoned plan leaves the tree as it found it")
		})

		t.Run("leaves an unchanged file and its mtime alone", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, "store.go", "package svc\n")
			at := filepath.Join(root, "store.go")
			assert.NoError(t, os.Chtimes(at, past, past), "the fixture ages the file")

			w := committed(t, root, "store.go", "package svc\n")
			assert.Equal(t, w.Action, output.ActionUnchanged, "the bytes are identical")
			info, err := os.Stat(at)
			assert.NoError(t, err, "the file stands")
			assert.True(t, info.ModTime().Equal(past),
				"and its mtime is untouched, because build systems key on it")
		})

		t.Run("rewrites a file whose bytes changed", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, "store.go", "package svc\n")
			w := committed(t, root, "store.go", "package svc\n\nfunc Load() {}\n")
			assert.Equal(t, w.Action, output.ActionUpdated, "the bytes moved")

			got, err := os.ReadFile(filepath.Join(root, "store.go"))
			assert.NoError(t, err, "the file reads")
			assert.Equal(t, string(got), "package svc\n\nfunc Load() {}\n",
				"carrying the new bytes whole")
		})

		t.Run("leaves no staging file behind", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, "store.go", "package svc\n")
			entries, err := os.ReadDir(root)
			assert.NoError(t, err, "the root reads")
			assert.Length(t, entries, 1,
				"the rename is what makes the write atomic, and it leaves one file")
			assert.Equal(t, entries[0].Name(), "store.go", "the file itself")
		})

		t.Run("overwrites a staging file a killed run left", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			stale := filepath.Join(root, "store.go.stage")
			assert.NoError(t, os.WriteFile(stale, []byte("half a file"), 0o644),
				"the fixture leaves a stale staging file")

			committed(t, root, "store.go", "package svc\n")
			_, err := os.Stat(stale)
			assert.True(t, os.IsNotExist(err),
				"a staging file is stale by definition, so the commit takes it")
		})

		t.Run("refuses a path escaping through a symlink", func(t *testing.T) {
			t.Parallel()

			root, outside := t.TempDir(), t.TempDir()
			assert.NoError(t, os.Symlink(outside, filepath.Join(root, "away")),
				"the fixture links out of the root")

			s := disk(t, root)
			assert.NoError(t, s.Write("away/escaped.go", []byte("package x\n")),
				"the path is workspace-relative, so staging takes it")
			assert.NoError(t, s.Write("kept.go", []byte("package svc\n")),
				"and its sibling stages too")

			got, err := s.Commit()
			assert.HasError(t, err,
				"the jail is the operating system's, so the link does not escape it")
			_, statErr := os.Stat(filepath.Join(outside, "escaped.go"))
			assert.True(t, os.IsNotExist(statErr), "nothing reached the far side")
			assert.Length(t, got, 1, "the commit kept going past the refusal")
			assert.Equal(t, got[0].Path, "kept.go", "and its sibling is on disk")
		})
	})

	t.Run("Discard leaves the tree as it was", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		s := disk(t, root)
		assert.NoError(t, s.Write("store.go", []byte("package svc\n")),
			"the staging takes")
		assert.NoError(t, s.Discard(), "the discard runs")

		entries, err := os.ReadDir(root)
		assert.NoError(t, err, "the root reads")
		assert.Length(t, entries, 0, "a discarded sink leaves no trace")
	})
}
