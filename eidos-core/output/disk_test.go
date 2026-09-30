// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// The brand the disk sink writes as, and another tool's brand.
const (
	diskBrand  output.Brand = "acme"
	rivalBrand output.Brand = "rival"
)

// The bodies the overwrite cases stamp, and the file they target.
const (
	firstBody  = "package svc\n"
	secondBody = "package svc\n\nfunc Load() {}\n"
	storeFile  = "store.go"
)

// past is the mtime the unchanged check pins against: a fixed
// instant, so the assertion is exact whatever granularity the
// filesystem keeps.
var past = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

// disk opens a sink over root under the fixture brand, failing the
// test where it cannot.
func disk(t *testing.T, root string) output.Sink {
	t.Helper()

	d, err := output.NewDisk(root, diskBrand)
	assert.NoError(t, err, "the root opens")
	return d
}

// committed stages one file and commits it, returning the record.
func committed(t *testing.T, root, path, body string) output.Written {
	t.Helper()

	s := disk(t, root)
	assert.NoError(t, s.Write(path, []byte(body)), "the file stages")
	got, err := s.Commit()
	assert.NoError(t, err, "the commit succeeds")
	assert.Length(t, got, 1, "one staged file is one record")
	return got[0]
}

// stampedAs returns body stamped as one brand's output.
func stampedAs(t *testing.T, brand output.Brand, body string) string {
	t.Helper()

	out, err := contract(t, brand, goSyntax()).Stamp(plugin.RenderedFile{Body: []byte(body)})
	assert.NoError(t, err, "the body stamps")
	return string(out)
}

// placed writes content at root/name outside the sink, the way a
// person or another tool leaves a file.
func placed(t *testing.T, root, name, content string) string {
	t.Helper()

	at := filepath.Join(root, name)
	assert.NoError(t, os.WriteFile(at, []byte(content), 0o644), "the fixture places the file")
	return at
}

// refusedOverwrite commits a stamped body over an existing file and
// returns the file's bytes afterwards and the commit's error.
func refusedOverwrite(t *testing.T, existing string) (string, error) {
	t.Helper()

	root := t.TempDir()
	at := placed(t, root, storeFile, existing)
	s := disk(t, root)
	assert.NoError(t, s.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
	_, err := s.Commit()
	got, readErr := os.ReadFile(at)
	assert.NoError(t, readErr, "the file reads")
	return string(got), err
}

// The disk sink is where determinism meets the filesystem: bytes
// appear whole or not at all, unchanged files are not touched, a
// file the brand did not write is never overwritten, and nothing is
// written to a path outside the root.
func TestDisk(t *testing.T) {
	t.Parallel()

	t.Run("NewDisk", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()

			_, err := output.NewDisk(filepath.Join(t.TempDir(), "absent"), diskBrand)
			assert.HasError(t, err, "a sink over nothing writes nowhere")
		})

		tests := []struct {
			name  string
			brand output.Brand
		}{
			{name: "returns an error for the empty brand", brand: ""},
			{name: "returns an error for a brand with an uppercase letter", brand: "Acme"},
			{name: "returns an error for a brand opening with a digit", brand: "2acme"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := output.NewDisk(t.TempDir(), tt.brand)
				assert.HasError(t, err, "the sink cannot prove ownership without a brand")
			})
		}
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing to the tree before the commit", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, disk(t, root).Write(storeFile, []byte(firstBody)), "the file stages")
			entries, err := os.ReadDir(root)
			assert.NoError(t, err, "the root reads")
			assert.Length(t, entries, 0, "the tree is unchanged")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		t.Run("creates a file with its parent directories", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := committed(t, root, "svc/api/store.go", "package api\n")
			assert.Equal(t, w.Action, output.ActionCreated, "the path did not exist")

			got, err := os.ReadFile(filepath.Join(root, "svc", "api", "store.go"))
			assert.NoError(t, err, "the file is on disk")
			assert.Equal(t, string(got), "package api\n", "the file has the staged bytes")
		})

		t.Run("leaves the mtime of an unchanged file untouched", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, storeFile, firstBody)
			at := filepath.Join(root, storeFile)
			assert.NoError(t, os.Chtimes(at, past, past), "the fixture ages the file")

			w := committed(t, root, storeFile, firstBody)
			assert.Equal(t, w.Action, output.ActionUnchanged, "the bytes are identical")
			info, err := os.Stat(at)
			assert.NoError(t, err, "the file exists")
			assert.True(t, info.ModTime().Equal(past), "the mtime is untouched, because build systems key on it")
		})

		t.Run("updates the brand's own output whose bytes changed", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, storeFile, stampedAs(t, diskBrand, firstBody))
			next := stampedAs(t, diskBrand, secondBody)
			w := committed(t, root, storeFile, next)
			assert.Equal(t, w.Action, output.ActionUpdated, "the bytes changed")

			got, err := os.ReadFile(filepath.Join(root, storeFile))
			assert.NoError(t, err, "the file reads")
			assert.Equal(t, string(got), next, "the file has the new bytes")
		})

		overwrites := []struct {
			name     string
			existing func(t *testing.T) string
		}{
			{
				name:     "returns an error naming a hand-written file it would overwrite",
				existing: func(*testing.T) string { return firstBody },
			},
			{
				name: "returns an error naming another brand's output it would overwrite",
				existing: func(t *testing.T) string {
					t.Helper()
					return stampedAs(t, rivalBrand, firstBody)
				},
			},
			{
				name: "returns an error naming the brand's output edited since stamping",
				existing: func(t *testing.T) string {
					t.Helper()
					return stampedAs(t, diskBrand, firstBody) + "// an edit after stamping\n"
				},
			},
		}
		for _, tt := range overwrites {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := refusedOverwrite(t, tt.existing(t))
				assert.HasError(t, err, "the commit fails")
				assert.Contains(t, err.Error(), `"`+storeFile+`"`, "the error names the file")
			})
		}

		t.Run("leaves a file it refuses to overwrite as it is", func(t *testing.T) {
			t.Parallel()

			got, err := refusedOverwrite(t, firstBody)
			assert.HasError(t, err, "the commit fails")
			assert.Equal(t, got, firstBody, "the file has its own bytes")
		})

		t.Run("commits the other files after a refused overwrite", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			placed(t, root, storeFile, firstBody)
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
			assert.NoError(t, s.Write("kept.go", []byte(firstBody)), "the sibling stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the commit reports the refusal")
			assert.Length(t, got, 1, "one record is returned")
			assert.Equal(t, got[0].Path, "kept.go", "the sibling is committed")
		})

		t.Run("leaves no staging file behind", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			committed(t, root, storeFile, firstBody)
			entries, err := os.ReadDir(root)
			assert.NoError(t, err, "the root reads")
			assert.Length(t, entries, 1, "the rename leaves one file")
			assert.Equal(t, entries[0].Name(), storeFile, "the file is the target")
		})

		t.Run("replaces a staging file a killed run left", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			stale := placed(t, root, storeFile+".stage", "half a file")
			committed(t, root, storeFile, firstBody)
			_, err := os.Stat(stale)
			assert.True(t, os.IsNotExist(err), "the stale staging file is gone")
		})

		t.Run("returns an error for a path that escapes through a symlink", func(t *testing.T) {
			t.Parallel()

			root, outside := t.TempDir(), t.TempDir()
			assert.NoError(t, os.Symlink(outside, filepath.Join(root, "away")),
				"the fixture links out of the root")

			s := disk(t, root)
			assert.NoError(t, s.Write("away/escaped.go", []byte("package x\n")),
				"the workspace-relative path stages")
			assert.NoError(t, s.Write("kept.go", []byte(firstBody)), "the sibling stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the link does not escape the operating system's jail")
			_, statErr := os.Stat(filepath.Join(outside, "escaped.go"))
			assert.True(t, os.IsNotExist(statErr), "nothing is written outside the root")
			assert.Length(t, got, 1, "the commit continues past the refusal")
			assert.Equal(t, got[0].Path, "kept.go", "the sibling is committed")
		})

		t.Run("returns an error naming a file whose directory it cannot make", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.Symlink("ghost", filepath.Join(root, "svc")),
				"the fixture leaves a dangling link where the directory belongs")

			s := disk(t, root)
			assert.NoError(t, s.Write("svc/store.go", []byte(firstBody)), "the file stages")
			assert.NoError(t, s.Write("kept.go", []byte(firstBody)), "the sibling stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the directory cannot be made over the link")
			assert.Contains(t, err.Error(), `"svc/store.go"`, "the error names the file")
			assert.Length(t, got, 1, "the commit continues past the refusal")
			assert.Equal(t, got[0].Path, "kept.go", "the sibling is committed")
		})

		t.Run("returns an error for a staging file it cannot write", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.Mkdir(filepath.Join(root, storeFile+".stage"), 0o755),
				"the fixture occupies the staging path with a directory")

			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the bytes are written to the tree through the staging file alone")
			assert.Contains(t, err.Error(), "staging", "the error names the step that failed")
			assert.Empty(t, got, "no record claims a file that was never written")
			_, statErr := os.Stat(filepath.Join(root, storeFile))
			assert.True(t, os.IsNotExist(statErr), "the target was never written")
		})

		t.Run("removes a staging file whose bytes it cannot sync", func(t *testing.T) {
			t.Parallel()

			// A FIFO at the staging path opens for writing while a
			// reader has it open. It accepts the written bytes, and
			// the sync that follows fails on it.
			root := t.TempDir()
			stage := filepath.Join(root, storeFile+".stage")
			out, err := exec.CommandContext(t.Context(), "mkfifo", stage).CombinedOutput()
			assert.NoError(t, err, "the fixture makes a FIFO at the staging path: "+string(out))
			reader, err := os.OpenFile(stage, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			assert.NoError(t, err, "a reader opens the FIFO")
			t.Cleanup(func() { _ = reader.Close() })

			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")
			got, err := s.Commit()
			assert.HasError(t, err, "unsynced bytes are never renamed over the target")
			assert.Contains(t, err.Error(), "staging", "the error names the step that failed")
			assert.Empty(t, got, "no record claims the file")
			_, statErr := os.Stat(stage)
			assert.True(t, os.IsNotExist(statErr), "the failed staging file is gone")
			_, statErr = os.Stat(filepath.Join(root, storeFile))
			assert.True(t, os.IsNotExist(statErr), "the target was never written")
		})
	})

	t.Run("Discard", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing to the tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")
			assert.NoError(t, s.Discard(), "the discard succeeds")

			entries, err := os.ReadDir(root)
			assert.NoError(t, err, "the root reads")
			assert.Length(t, entries, 0, "the tree is unchanged")
		})

		t.Run("returns ErrFinished for a second discard", func(t *testing.T) {
			t.Parallel()

			s := disk(t, t.TempDir())
			assert.NoError(t, s.Discard(), "the first discard succeeds")
			assert.ErrorIs(t, s.Discard(), output.ErrFinished, "a sink serves one staging")
		})

		t.Run("returns ErrFinished after the commit", func(t *testing.T) {
			t.Parallel()

			s := disk(t, t.TempDir())
			_, err := s.Commit()
			assert.NoError(t, err, "the commit succeeds")
			assert.ErrorIs(t, s.Discard(), output.ErrFinished, "the commit closed the root")
		})
	})
}
