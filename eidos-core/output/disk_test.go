// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"cmp"
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
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

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

	t.Run("Overwrite", func(t *testing.T) {
		t.Parallel()

		allowed := []struct {
			name     string
			existing func(t *testing.T) string
			allow    []output.Found
		}{
			{
				name:     "lets the commit write over the brand's drifted output",
				existing: drifted,
				allow:    []output.Found{output.FoundDrifted},
			},
			{
				name:     "lets the commit write over a hand-written file",
				existing: func(*testing.T) string { return firstBody },
				allow:    []output.Found{output.FoundForeign},
			},
			{
				name:     "lets the commit write over another brand's output",
				existing: func(t *testing.T) string { t.Helper(); return stampedAs(t, rivalBrand, firstBody) },
				allow:    []output.Found{output.FoundForeign},
			},
			{
				name:     "lets the commit write over the brand's drifted output under both verdicts",
				existing: drifted,
				allow:    []output.Found{output.FoundDrifted, output.FoundForeign},
			},
		}
		for _, tt := range allowed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := files.Workspace(t, files.Tree{storeFile: files.Text(tt.existing(t))})
				d := opened(t, root, nil)
				assert.NoError(t, d.Overwrite(tt.allow...), "the verdicts are allowed")
				next := stampedAs(t, diskBrand, secondBody)
				assert.NoError(t, d.Write(storeFile, []byte(next)), "the file stages")
				got, err := d.Commit()
				assert.NoError(t, err, "the commit writes over the file")
				assert.Length(t, got, 1, "one record for the file")
				expect.Equal(t, got[0].Action, output.ActionUpdated, "the commit updates the file")
				files.HasContent(t, filepath.Join(root, storeFile), next, "the file has the staged bytes")
			})
		}

		kept := []struct {
			name     string
			existing func(t *testing.T) string
			allow    output.Found
		}{
			{
				name:     "keeps refusing a hand-written file under the drifted verdict",
				existing: func(*testing.T) string { return firstBody },
				allow:    output.FoundDrifted,
			},
			{
				name:     "keeps refusing the brand's drifted output under the foreign verdict",
				existing: drifted,
				allow:    output.FoundForeign,
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				existing := tt.existing(t)
				root := files.Workspace(t, files.Tree{storeFile: files.Text(existing)})
				d := opened(t, root, nil)
				assert.NoError(t, d.Overwrite(tt.allow), "the verdict is allowed")
				assert.NoError(t, d.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
				_, err := d.Commit()
				assert.HasError(t, err, "the commit refuses the file")
				files.HasContent(t, filepath.Join(root, storeFile), existing, "the file has its own bytes")
			})
		}

		t.Run("returns an error from the commit for a directory at the staged path", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile: files.Dir()})
			d := opened(t, root, nil)
			assert.NoError(t, d.Overwrite(output.FoundForeign), "the foreign verdict is allowed")
			assert.NoError(t, d.Write(storeFile, []byte(firstBody)), "the file stages")
			_, err := d.Commit()
			assert.HasError(t, err, "the commit does not write over a directory")
			files.IsDir(t, filepath.Join(root, storeFile), "the directory remains")
		})

		t.Run("leaves the brand's drifted output that a removal names", func(t *testing.T) {
			t.Parallel()

			existing := drifted(t)
			root := files.Workspace(t, files.Tree{storeFile: files.Text(existing)})
			d := opened(t, root, nil)
			assert.NoError(t, d.Overwrite(output.FoundDrifted, output.FoundForeign), "both verdicts are allowed")
			assert.NoError(t, d.Delete(storeFile), "the removal stages")
			got, err := d.Commit()
			assert.NoError(t, err, "a kept file is no fault")
			assert.Empty(t, got, "no record claims a removal")
			files.HasContent(t, filepath.Join(root, storeFile), existing, "the file remains with its own bytes")
		})

		invalid := []struct {
			name  string
			found output.Found
		}{
			{name: "returns an error for FoundNothing", found: output.FoundNothing},
			{name: "returns an error for FoundSame", found: output.FoundSame},
			{name: "returns an error for FoundIntact", found: output.FoundIntact},
			{name: "returns an error for the zero Found", found: 0},
		}
		for _, tt := range invalid {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.HasError(t, opened(t, t.TempDir(), nil).Overwrite(tt.found), "the verdict is refused")
			})
		}

		t.Run("allows nothing for a list with a verdict it refuses", func(t *testing.T) {
			t.Parallel()

			existing := drifted(t)
			root := files.Workspace(t, files.Tree{storeFile: files.Text(existing)})
			d := opened(t, root, nil)
			assert.HasError(t, d.Overwrite(output.FoundDrifted, output.FoundIntact), "the list is refused")
			assert.NoError(t, d.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
			_, err := d.Commit()
			assert.HasError(t, err, "the commit refuses the drifted file")
			files.HasContent(t, filepath.Join(root, storeFile), existing, "the file has its own bytes")
		})

		t.Run("returns an error after the preparation", func(t *testing.T) {
			t.Parallel()

			d := opened(t, t.TempDir(), nil)
			_, err := d.Prepare()
			assert.NoError(t, err, "the staging prepares")
			assert.HasError(t, d.Overwrite(output.FoundDrifted), "a prepared staging is closed")
			assert.NoError(t, d.Discard(), "the sink discards")
		})

		t.Run("returns ErrFinished after the commit", func(t *testing.T) {
			t.Parallel()

			d := opened(t, t.TempDir(), nil)
			_, err := d.Commit()
			assert.NoError(t, err, "the commit succeeds")
			assert.ErrorIs(t, d.Overwrite(output.FoundDrifted), output.ErrFinished, "a sink serves one staging")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing to the tree before the commit", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, disk(t, root).Write(storeFile, []byte(firstBody)), "the file stages")
			files.Equal(t, os.DirFS(root), files.Tree{}, "the tree is unchanged")
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()

		t.Run("removes nothing from the tree before the commit", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile: files.Text(stampedAs(t, diskBrand, firstBody))})
			assert.NoError(t, disk(t, root).Delete(storeFile), "the removal stages")
			files.IsFile(t, filepath.Join(root, storeFile), "the file remains")
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

			root := files.Workspace(t, files.Tree{storeFile: files.Dir()})
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")
			got, err := s.Prepare()
			assert.NoError(t, err, "the staging prepares")
			assert.Equal(t, got[0].Found, output.FoundForeign, "a directory is no file of the brand's")
		})

		t.Run("returns an error for a path a symlink leads out of the root", func(t *testing.T) {
			t.Parallel()

			outside := files.Workspace(t, files.Tree{"escaped.go": files.Text(firstBody)})
			root := files.Workspace(t, files.Tree{"away": files.Link(outside)})
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
			files.HasContent(t, filepath.Join(root, "svc", "api", "store.go"), "package api\n",
				"the file is on disk with the staged bytes")
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
			files.HasContent(t, filepath.Join(root, storeFile), next, "the file has the new bytes")
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

			root := files.Workspace(t, files.Tree{storeFile: files.Text(stampedAs(t, diskBrand, firstBody))})
			at := filepath.Join(root, storeFile)
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
			got, err := s.Prepare()
			assert.NoError(t, err, "the staging prepares")
			assert.Equal(t, got[0].Found, output.FoundIntact, "the file is intact at the preparation")
			edited := drifted(t)
			assert.NoError(t, os.WriteFile(at, []byte(edited), 0o644), "a person edits the file")
			_, err = s.Commit()
			assert.HasError(t, err, "the commit refuses the edited file")
			files.HasContent(t, at, edited, "the edit remains")
		})

		t.Run("removes the brand's intact output a removal names", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile: files.Text(stampedAs(t, diskBrand, firstBody))})
			at := filepath.Join(root, storeFile)
			s := disk(t, root)
			assert.NoError(t, s.Delete(storeFile), "the removal stages")
			got, err := s.Commit()
			assert.NoError(t, err, "the commit succeeds")
			assert.Equal(t, got, []output.Written{{Path: storeFile, Action: output.ActionDeleted}},
				"one removal without a digest")
			files.Absent(t, at, "the file is gone")
		})

		t.Run("returns an error for the brand's output it cannot remove", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{"svc/" + storeFile: files.Text(stampedAs(t, diskBrand, firstBody))})
			dir := filepath.Join(root, "svc")
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

				existing := tt.existing(t)
				root := files.Workspace(t, files.Tree{storeFile: files.Text(existing)})
				at := filepath.Join(root, storeFile)
				s := disk(t, root)
				assert.NoError(t, s.Delete(storeFile), "the removal stages")
				got, err := s.Commit()
				assert.NoError(t, err, "a kept file is no fault")
				assert.Empty(t, got, "no record claims a removal")
				files.HasContent(t, at, existing, "the file remains with its own bytes")
			})
		}

		t.Run("commits the other files after a refused overwrite", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile: files.Text(firstBody)})
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
			files.Equal(t, os.DirFS(root), files.Tree{storeFile: files.Text(firstBody)},
				"the rename leaves the target alone")
		})

		t.Run("replaces a staging file a killed run left", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeFile + ".stage": files.Text("half a file")})
			committed(t, root, storeFile, firstBody)
			files.Absent(t, filepath.Join(root, storeFile+".stage"), "the stale staging file is gone")
		})

		t.Run("returns an error for a path that escapes through a symlink", func(t *testing.T) {
			t.Parallel()

			outside := t.TempDir()
			root := files.Workspace(t, files.Tree{"away": files.Link(outside)})
			s := disk(t, root)
			assert.NoError(t, s.Write("away/escaped.go", []byte("package x\n")),
				"the workspace-relative path stages")
			assert.NoError(t, s.Write("kept.go", []byte(firstBody)), "the sibling stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the link does not escape the operating system's jail")
			files.Absent(t, filepath.Join(outside, "escaped.go"), "nothing is written outside the root")
			assert.Length(t, got, 1, "the commit continues past the refusal")
			assert.Equal(t, got[0].Path, "kept.go", "the sibling is committed")
		})

		t.Run("returns an error naming a file whose directory it cannot make", func(t *testing.T) {
			t.Parallel()

			// A dangling link stands where the directory belongs.
			root := files.Workspace(t, files.Tree{"svc": files.Link("ghost")})
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

			// A directory occupies the staging path.
			root := files.Workspace(t, files.Tree{storeFile + ".stage": files.Dir()})
			s := disk(t, root)
			assert.NoError(t, s.Write(storeFile, []byte(firstBody)), "the file stages")

			got, err := s.Commit()
			assert.HasError(t, err, "the bytes are written to the tree through the staging file alone")
			assert.Contains(t, err.Error(), "staging", "the error names the step that failed")
			assert.Empty(t, got, "no record claims a file that was never written")
			files.Absent(t, filepath.Join(root, storeFile), "the target was never written")
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
			files.Absent(t, stage, "the failed staging file is gone")
			files.Absent(t, filepath.Join(root, storeFile), "the target was never written")
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
			files.Equal(t, os.DirFS(root), files.Tree{}, "the tree is unchanged")
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
// consumes its sink takes a sink of its own, opened outside the count,
// and the test discards every sink no call finished. Each count keeps
// the first error of its calls, which cmp.Or returns without
// allocating. The check runs alone, because the count includes every
// goroutine's allocations.
func TestDiskAllocs(t *testing.T) {
	same := []byte(stampedAs(t, diskBrand, firstBody))
	other := []byte(stampedAs(t, diskBrand, secondBody))
	root := files.Workspace(t, files.Tree{storeFile: files.Bytes(same)})

	var open []*output.Disk
	t.Cleanup(func() { discardAll(t, open) })
	built := make([]*output.Disk, 0, allocRuns)
	var err error
	assert.MaxAllocs(t, func() {
		d, nerr := output.NewDisk(root, diskBrand)
		err = cmp.Or(err, nerr)
		built = append(built, d)
	}, newDiskAllocs, "NewDisk allocates the sink and its root")
	assert.NoError(t, err, "every root opens")
	discardAll(t, built)

	sink := func(dir string, body []byte) func() *output.Disk {
		return func() *output.Disk {
			d := opened(t, dir, body)
			open = append(open, d)
			return d
		}
	}
	prepare := func(d *output.Disk) {
		_, perr := d.Prepare()
		err = cmp.Or(err, perr)
	}
	commit := func(d *output.Disk) {
		_, cerr := d.Commit()
		err = cmp.Or(err, cerr)
	}

	assert.MaxAllocsWithSetup(t, sink(root, nil), func(d *output.Disk) { err = cmp.Or(err, d.Write(storeFile, same)) },
		firstWriteAllocs, "Write allocates the staging's maps on the first write")
	assert.NoError(t, err, "every first write stages")

	assert.MaxAllocsWithSetup(t, sink(root, nil), func(d *output.Disk) { err = cmp.Or(err, d.Delete(storeFile)) },
		firstDeleteAllocs, "Delete allocates the removal set on the first removal")
	assert.NoError(t, err, "every first removal stages")

	assert.MaxAllocsWithSetup(t, sink(root, same), prepare,
		prepareSameAllocs, "Prepare allocates the path list, the changes, the digest and the read")
	assert.NoError(t, err, "every staging over the file prepares")

	assert.MaxAllocsWithSetup(t, sink(t.TempDir(), same), prepare,
		prepareNothingAllocs, "Prepare allocates the read of a path without a file")
	assert.NoError(t, err, "every staging over no file prepares")

	assert.MaxAllocsWithSetup(t, sink(root, same), commit,
		commitSameAllocs, "Commit allocates the path list, the records, the read and the digest")
	assert.NoError(t, err, "every commit of the file's own bytes succeeds")

	// Each sink stages the body the file lacks, so every commit in order
	// updates the file the commit before it wrote.
	bodies, next := [2][]byte{other, same}, 0
	updating := func() *output.Disk {
		d := sink(root, bodies[next%2])()
		next++
		return d
	}
	assert.MaxAllocsWithSetup(t, updating, commit,
		commitUpdateAllocs, "Commit allocates the verification and the staging file of an update")
	assert.NoError(t, err, "every update commits")

	assert.MaxAllocsWithSetup(t, sink(root, same), func(d *output.Disk) { err = cmp.Or(err, d.Discard()) },
		0, "Discard allocates nothing")
	assert.NoError(t, err, "every discard succeeds")
}

// BenchmarkDisk measures each method of the disk sink over one file of
// the brand's, each call that consumes its sink on a sink opened outside
// the measurement.
func BenchmarkDisk(b *testing.B) {
	same := []byte(stampedAs(b, diskBrand, firstBody))
	other := []byte(stampedAs(b, diskBrand, secondBody))
	root, nothing := files.Workspace(b, files.Tree{storeFile: files.Bytes(same)}), b.TempDir()

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

	tree := files.Tree{}
	if existing != "" {
		tree[storeFile] = files.Text(existing)
	}
	s := disk(t, files.Workspace(t, tree))
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

	root := files.Workspace(t, files.Tree{storeFile: files.Text(existing)})
	s := disk(t, root)
	assert.NoError(t, s.Write(storeFile, []byte(stampedAs(t, diskBrand, secondBody))), "the file stages")
	_, err := s.Commit()
	return files.Read(t, filepath.Join(root, storeFile)), err
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
	err := d.Discard()
	if errors.Is(err, output.ErrFinished) {
		return
	}
	assert.NoError(tb, err, "the sink discards")
}

// discardAll closes every sink of a check that no call finished.
func discardAll(tb assert.TB, sinks []*output.Disk) {
	tb.Helper()

	for _, d := range sinks {
		discarded(tb, d)
	}
}
