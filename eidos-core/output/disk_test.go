// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

// The allocations of the disk sink over one file, which TestDiskAllocs
// checks in the ordinary run and BenchmarkDisk in a benchmark run.
const (
	// newDiskAllocs is the sink, the root's two structures and the root
	// path's spelling for the system call.
	newDiskAllocs = 4
	// readFileAllocs is a read of a file through the root: the path's
	// split, its spelling for the system call, the open file's name and
	// its two structures, its status and its bytes.
	readFileAllocs = 7
	// readNothingAllocs is a read of a path without a file: the split, the
	// spelling and the error.
	readNothingAllocs = 3
	// prepareSameAllocs is the preparation of one staged file over a file
	// with the staged bytes: the path list, the list of changes, the
	// digest and the read.
	prepareSameAllocs = 3 + readFileAllocs
	// prepareNothingAllocs is the preparation of one staged file over a
	// path without a file.
	prepareNothingAllocs = 3 + readNothingAllocs
	// commitSameAllocs is the commit of one staged file over a file with
	// the staged bytes: the path list, the list of records, the read and
	// the digest.
	commitSameAllocs = 3 + readFileAllocs
	// commitUpdateAllocs is the commit of one staged file over the brand's
	// intact output of other bytes: what commitSameAllocs counts, the
	// record of the file's verification, two allocations for a frame
	// without derivation lines, and twelve for the staging file. The
	// twelve are its name, its open, and the rename's two splits, two
	// spellings and read of the target's status.
	commitUpdateAllocs = commitSameAllocs + 2 + 12
)

// past is the mtime the unchanged check pins against: a fixed
// instant, so the assertion is exact whatever granularity the
// filesystem keeps.
var past = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

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

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()

		t.Run("removes nothing from the tree before the commit", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			at := placed(t, root, storeFile, stampedAs(t, diskBrand, firstBody))
			assert.NoError(t, disk(t, root).Delete(storeFile), "the removal stages")
			_, err := os.Stat(at)
			assert.NoError(t, err, "the file remains")
		})
	})

	t.Run("Prepare", func(t *testing.T) {
		t.Parallel()

		writes := []struct {
			name     string
			existing func(t *testing.T) string
			want     output.Found
			action   output.Action
		}{
			{
				name:     "returns FoundSame for a file with the staged bytes",
				existing: func(t *testing.T) string { t.Helper(); return stampedAs(t, diskBrand, secondBody) },
				want:     output.FoundSame, action: output.ActionUnchanged,
			},
			{
				name:     "returns FoundIntact for the brand's output with other bytes",
				existing: func(t *testing.T) string { t.Helper(); return stampedAs(t, diskBrand, firstBody) },
				want:     output.FoundIntact, action: output.ActionUpdated,
			},
			{
				name:     "returns FoundDrifted for the brand's output edited since its stamp",
				existing: drifted,
				want:     output.FoundDrifted, action: output.ActionUpdated,
			},
			{
				name:     "returns FoundForeign for a hand-written file",
				existing: func(*testing.T) string { return firstBody },
				want:     output.FoundForeign, action: output.ActionUpdated,
			},
			{
				name:     "returns FoundForeign for another brand's output",
				existing: func(t *testing.T) string { t.Helper(); return stampedAs(t, rivalBrand, firstBody) },
				want:     output.FoundForeign, action: output.ActionUpdated,
			},
			{
				name: "returns FoundForeign for the brand's output with a line after its trailer",
				existing: func(t *testing.T) string {
					t.Helper()
					return stampedAs(t, diskBrand, firstBody) + "// an edit after the trailer\n"
				},
				want: output.FoundForeign, action: output.ActionUpdated,
			},
		}
		for _, tt := range writes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				staged := stampedAs(t, diskBrand, secondBody)
				got := prepared(t, tt.existing(t), &staged)
				assert.Equal(t, got.Found, tt.want, "the verdict")
				assert.Equal(t, got.Action, tt.action, "the action the commit takes")
			})
		}

		removals := []struct {
			name     string
			existing func(t *testing.T) string
			want     output.Found
			action   output.Action
		}{
			{
				name:     "returns ActionDeleted for a removal of the brand's intact output",
				existing: func(t *testing.T) string { t.Helper(); return stampedAs(t, diskBrand, firstBody) },
				want:     output.FoundIntact, action: output.ActionDeleted,
			},
			{
				name:     "returns ActionUnchanged for a removal of the brand's drifted output",
				existing: drifted,
				want:     output.FoundDrifted, action: output.ActionUnchanged,
			},
			{
				name:     "returns ActionUnchanged for a removal of a hand-written file",
				existing: func(*testing.T) string { return firstBody },
				want:     output.FoundForeign, action: output.ActionUnchanged,
			},
		}
		for _, tt := range removals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := prepared(t, tt.existing(t), nil)
				assert.Equal(t, got.Found, tt.want, "the verdict")
				assert.Equal(t, got.Action, tt.action, "the action the commit takes")
				assert.Equal(t, got.Hash, "", "a removal stages no bytes")
			})
		}

		t.Run("returns FoundForeign for a directory at the staged path", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.Mkdir(filepath.Join(root, storeFile), 0o755), "the fixture makes a directory")
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")
			got, err := s.Prepare()
			assert.NoError(t, err, "the staging prepares")
			assert.Equal(t, got[0].Found, output.FoundForeign, "a directory is no file of the brand's")
		})

		t.Run("returns an error for a path a symlink leads out of the root", func(t *testing.T) {
			t.Parallel()

			root, outside := t.TempDir(), t.TempDir()
			placed(t, outside, "escaped.go", firstBody)
			assert.NoError(t, os.Symlink(outside, filepath.Join(root, "away")),
				"the fixture links out of the root")
			s := disk(t, root)
			assert.NoError(t, s.Write("away/escaped.go", []byte(firstBody)), "the path stages")
			_, err := s.Prepare()
			assert.HasError(t, err, "the read does not escape the root")
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

		t.Run("keeps the bytes of a file a failed commit leaves", func(t *testing.T) {
			t.Parallel()

			got, err := refusedOverwrite(t, firstBody)
			assert.HasError(t, err, "the commit fails")
			assert.Equal(t, got, firstBody, "the file has its own bytes")
		})

		t.Run("returns an error for a file edited after the preparation", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			at := placed(t, root, storeFile, stampedAs(t, diskBrand, firstBody))
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
			got, err := s.Prepare()
			assert.NoError(t, err, "the staging prepares")
			assert.Equal(t, got[0].Found, output.FoundIntact, "the file is intact at the preparation")
			edited := drifted(t)
			assert.NoError(t, os.WriteFile(at, []byte(edited), 0o644), "a person edits the file")
			_, err = s.Commit()
			assert.HasError(t, err, "the commit refuses the edited file")
			after, readErr := os.ReadFile(at)
			assert.NoError(t, readErr, "the file reads")
			assert.Equal(t, string(after), edited, "the edit remains")
		})

		t.Run("removes the brand's intact output a removal names", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			at := placed(t, root, storeFile, stampedAs(t, diskBrand, firstBody))
			s := disk(t, root)
			assert.NoError(t, s.Delete(storeFile), "the removal stages")
			got, err := s.Commit()
			assert.NoError(t, err, "the commit succeeds")
			assert.Equal(t, got, []output.Written{{Path: storeFile, Action: output.ActionDeleted}},
				"one removal without a digest")
			_, statErr := os.Stat(at)
			assert.True(t, os.IsNotExist(statErr), "the file is gone")
		})

		t.Run("returns an error for the brand's output it cannot remove", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			dir := filepath.Join(root, "svc")
			assert.NoError(t, os.Mkdir(dir, 0o755), "the fixture makes the directory")
			placed(t, dir, storeFile, stampedAs(t, diskBrand, firstBody))
			assert.NoError(t, os.Chmod(dir, 0o555), "the directory refuses a removal")
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			s := disk(t, root)
			assert.NoError(t, s.Delete("svc/"+storeFile), "the removal stages")
			got, err := s.Commit()
			assert.HasError(t, err, "the removal fails")
			assert.Contains(t, err.Error(), `"svc/`+storeFile+`"`, "the error names the file")
			assert.Empty(t, got, "no record claims the removal")
		})

		kept := []struct {
			name     string
			existing func(t *testing.T) string
		}{
			{name: "leaves the brand's drifted output a removal names", existing: drifted},
			{
				name:     "leaves a hand-written file a removal names",
				existing: func(*testing.T) string { return firstBody },
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				existing := tt.existing(t)
				at := placed(t, root, storeFile, existing)
				s := disk(t, root)
				assert.NoError(t, s.Delete(storeFile), "the removal stages")
				got, err := s.Commit()
				assert.NoError(t, err, "a kept file is no fault")
				assert.Empty(t, got, "no record claims a removal")
				after, readErr := os.ReadFile(at)
				assert.NoError(t, readErr, "the file remains")
				assert.Equal(t, string(after), existing, "with its own bytes")
			})
		}

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

// Each method of the disk sink allocates what it stages, reads and
// writes in the ordinary run, which runs no benchmark. Each call that
// consumes its sink takes a sink of its own, opened before the count. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestDiskAllocs(t *testing.T) {
	root := t.TempDir()
	same := []byte(stampedAs(t, diskBrand, firstBody))
	other := []byte(stampedAs(t, diskBrand, secondBody))
	placed(t, root, storeFile, string(same))

	built := make([]*output.Disk, 0, allocRuns)
	assert.MaxAllocs(t, func() {
		d, err := output.NewDisk(root, diskBrand)
		if err != nil {
			t.Fatalf("NewDisk: unexpected error: %v", err)
		}
		built = append(built, d)
	}, newDiskAllocs, "NewDisk allocates the sink and its root")
	discardAll(t, built)

	at, empty := 0, disks(t, root, nil)
	assert.MaxAllocs(t, func() {
		if err := empty[at].Write(storeFile, same); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		at++
	}, firstWriteAllocs, "Write allocates the staging's maps on the first write")

	at, empty = 0, disks(t, root, nil)
	assert.MaxAllocs(t, func() {
		if err := empty[at].Delete(storeFile); err != nil {
			t.Fatalf("Delete: unexpected error: %v", err)
		}
		at++
	}, firstDeleteAllocs, "Delete allocates the removal set on the first removal")

	at, sames := 0, disks(t, root, same)
	assert.MaxAllocs(t, func() {
		if _, err := sames[at].Prepare(); err != nil {
			t.Fatalf("Prepare: unexpected error: %v", err)
		}
		at++
	}, prepareSameAllocs, "Prepare allocates the path list, the changes, the digest and the read")

	at, absent := 0, disks(t, t.TempDir(), same)
	assert.MaxAllocs(t, func() {
		if _, err := absent[at].Prepare(); err != nil {
			t.Fatalf("Prepare: unexpected error: %v", err)
		}
		at++
	}, prepareNothingAllocs, "Prepare allocates the read of a path without a file")

	at, sames = 0, disks(t, root, same)
	assert.MaxAllocs(t, func() {
		if _, err := sames[at].Commit(); err != nil {
			t.Fatalf("Commit: unexpected error: %v", err)
		}
		at++
	}, commitSameAllocs, "Commit allocates the path list, the records, the read and the digest")

	at, updates := 0, alternating(t, root, other, same)
	assert.MaxAllocs(t, func() {
		if _, err := updates[at].Commit(); err != nil {
			t.Fatalf("Commit: unexpected error: %v", err)
		}
		at++
	}, commitUpdateAllocs, "Commit allocates the verification and the staging file of an update")

	at, sames = 0, disks(t, root, same)
	assert.MaxAllocs(t, func() {
		if err := sames[at].Discard(); err != nil {
			t.Fatalf("Discard: unexpected error: %v", err)
		}
		at++
	}, 0, "Discard allocates nothing")
}

// BenchmarkDisk measures each method of the disk sink over one file of
// the brand's, each call that consumes its sink on a sink opened outside
// the measurement.
func BenchmarkDisk(b *testing.B) {
	root, nothing := b.TempDir(), b.TempDir()
	same := []byte(stampedAs(b, diskBrand, firstBody))
	other := []byte(stampedAs(b, diskBrand, secondBody))
	placed(b, root, storeFile, string(same))

	b.Run("NewDisk", func(b *testing.B) {
		var d *output.Disk
		discard := func() { discarded(b, d) }
		c := bench.Start(b).MaxAllocs(newDiskAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			c.Excluding(discard)
			d, err = output.NewDisk(root, diskBrand)
		}
		assert.NoError(b, err, "the root opens")
		discard()
	})

	prepare := func(d *output.Disk) error { _, err := d.Prepare(); return err }

	b.Run("Write", func(b *testing.B) {
		b.Run("the first file of a staging", func(b *testing.B) {
			benchStaged(b, root, nil, firstWriteAllocs, func(d *output.Disk) error { return d.Write(storeFile, same) })
		})
	})

	b.Run("Delete", func(b *testing.B) {
		b.Run("the first removal of a staging", func(b *testing.B) {
			benchStaged(b, root, nil, firstDeleteAllocs, func(d *output.Disk) error { return d.Delete(storeFile) })
		})
	})

	b.Run("Prepare", func(b *testing.B) {
		b.Run("a file with the staged bytes", func(b *testing.B) {
			benchStaged(b, root, same, prepareSameAllocs, prepare)
		})

		b.Run("a path without a file", func(b *testing.B) {
			benchStaged(b, nothing, same, prepareNothingAllocs, prepare)
		})
	})

	b.Run("Discard", func(b *testing.B) {
		b.Run("a staging of one file", func(b *testing.B) {
			benchStaged(b, root, same, 0, (*output.Disk).Discard)
		})
	})

	// The root's file has same, and next persists across the runs of the
	// update's sub-benchmark, so every commit of it stages the body the
	// file lacks.
	bodies, next := [2][]byte{other, same}, 0
	b.Run("Commit", func(b *testing.B) {
		b.Run("a file with the staged bytes", func(b *testing.B) {
			benchStaged(
				b,
				root,
				same,
				commitSameAllocs,
				func(d *output.Disk) error { _, err := d.Commit(); return err },
			)
		})

		b.Run("an update of the brand's file", func(b *testing.B) {
			var d *output.Disk
			fresh := func() {
				d = opened(b, root, bodies[next%2])
				next++
			}
			c := bench.Start(b).MaxAllocs(commitUpdateAllocs)
			defer c.End()
			var (
				got []output.Written
				err error
			)
			for c.Loop() {
				c.Excluding(fresh)
				got, err = d.Commit()
			}
			assert.NoError(b, err, "the update commits")
			assert.Equal(b, got[0].Action, output.ActionUpdated, "the commit updates the file")
		})
	})
}

// benchStaged measures one call on a sink over root that stages body,
// under the bench contract at a ceiling of allocs. Each iteration opens
// a fresh sink outside the measurement and discards the one before.
func benchStaged(b *testing.B, root string, body []byte, allocs uint64, call func(*output.Disk) error) {
	b.Helper()

	var d *output.Disk
	fresh := func() {
		discarded(b, d)
		d = opened(b, root, body)
	}
	c := bench.Start(b).MaxAllocs(allocs)
	defer c.End()
	var err error
	for c.Loop() {
		c.Excluding(fresh)
		err = call(d)
	}
	assert.NoError(b, err, "the call succeeds")
	discarded(b, d)
}

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
func stampedAs(tb assert.TB, brand output.Brand, body string) string {
	tb.Helper()

	out, err := contract(tb, brand, goSyntax()).Stamp(plugin.RenderedFile{Body: []byte(body)})
	assert.NoError(tb, err, "the body stamps")
	return string(out)
}

// placed writes content at root/name outside the sink, the way a
// person or another tool leaves a file.
func placed(tb assert.TB, root, name, content string) string {
	tb.Helper()

	at := filepath.Join(root, name)
	assert.NoError(tb, os.WriteFile(at, []byte(content), 0o644), "the fixture places the file")
	return at
}

// drifted returns the brand's output of firstBody with its body edited
// after the stamp, the trailer left as it was.
func drifted(t *testing.T) string {
	t.Helper()

	return strings.Replace(stampedAs(t, diskBrand, firstBody), "package svc", "package edited", 1)
}

// prepared stages one write or one removal of storeFile over a root
// whose storeFile contains existing, where existing is not empty, and
// returns the one change Prepare reports.
func prepared(t *testing.T, existing string, staged *string) output.Change {
	t.Helper()

	root := t.TempDir()
	if existing != "" {
		placed(t, root, storeFile, existing)
	}
	s := disk(t, root)
	if staged != nil {
		assert.NoError(t, s.Write(storeFile, []byte(*staged)), "the file stages")
	} else {
		assert.NoError(t, s.Delete(storeFile), "the removal stages")
	}
	got, err := s.Prepare()
	assert.NoError(t, err, "the staging prepares")
	assert.Length(t, got, 1, "one staged path is one change")
	return got[0]
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

// opened opens a disk sink over root with body staged at storeFile, or
// with nothing staged for a nil body.
func opened(tb assert.TB, root string, body []byte) *output.Disk {
	tb.Helper()

	d, err := output.NewDisk(root, diskBrand)
	assert.NoError(tb, err, "the root opens")
	if body != nil {
		assert.NoError(tb, d.Write(storeFile, body), "the file stages")
	}
	return d
}

// discarded closes a sink a benchmark opened, where it opened one and
// no call finished it.
func discarded(tb assert.TB, d *output.Disk) {
	tb.Helper()

	if d == nil {
		return
	}
	if err := d.Discard(); err != nil && !errors.Is(err, output.ErrFinished) {
		tb.Fatalf("Discard: unexpected error: %v", err)
	}
}

// disks returns allocRuns sinks opened over root, each with body staged
// at storeFile, or with nothing staged for a nil body: one sink for each
// call of an allocation check that consumes its sink. The test discards
// every sink no call finished.
func disks(t *testing.T, root string, body []byte) []*output.Disk {
	t.Helper()

	out := make([]*output.Disk, allocRuns)
	for i := range out {
		out[i] = opened(t, root, body)
	}
	t.Cleanup(func() { discardAll(t, out) })
	return out
}

// alternating returns allocRuns sinks opened over root, staging first
// and second in turn, so each commit in order updates the file the one
// before it wrote.
func alternating(t *testing.T, root string, first, second []byte) []*output.Disk {
	t.Helper()

	out := make([]*output.Disk, allocRuns)
	for i := range out {
		body := first
		if i%2 == 1 {
			body = second
		}
		out[i] = opened(t, root, body)
	}
	t.Cleanup(func() { discardAll(t, out) })
	return out
}

// discardAll closes every sink of a check that no call finished.
func discardAll(tb assert.TB, sinks []*output.Disk) {
	tb.Helper()

	for _, d := range sinks {
		discarded(tb, d)
	}
}
