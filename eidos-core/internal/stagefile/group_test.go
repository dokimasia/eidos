// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The names of the calls that the recording barrier records and fails.
const (
	callStart   = "Start"
	callFile    = "File"
	callOrder   = "Order"
	callDurable = "Durable"
	callClose   = "Close"
)

// The fixture's second target, a nested target, and their bytes.
const (
	other     = "other.txt"
	nested    = "a/b/nested.txt"
	otherBody = "the other body\n"
)

// The ceilings of a group over one write of an existing target, through
// a barrier that syncs nothing.
const (
	// newGroupAllocs is the group.
	newGroupAllocs = 1
	// firstWriteAllocs is the list of entries that the first write grows.
	firstWriteAllocs = 1
	// commitOneAllocs is the commit of one write: the errors, the state
	// that the workers share, its staging names and the method value of
	// its call, the staging name, the list of changed directories, and 11
	// that os.Root spends opening the staging file and renaming it. A
	// group of one write runs on the caller's goroutine.
	commitOneAllocs = 17
)

// errBarrier is the failure that a failing barrier returns.
var errBarrier = errors.New("the barrier fails")

// recording is a barrier that syncs nothing and records each call: the
// calls in order, the base name of each file that File received, and the
// directories that Durable received. It fails the call that fail names,
// and the File of the staging file whose base name failFile names.
type recording struct {
	fail     string
	failFile string

	mu    sync.Mutex
	calls []string
	files []string
	dirs  []string
}

// Start records the call with the number of names.
func (b *recording) Start(_ *os.Root, n int) error {
	return b.call(callStart, callStart+" "+strconv.Itoa(n))
}

// File records the base name of f, and fails for the file that failFile
// names.
func (b *recording) File(f *os.File) error {
	name := filepath.Base(f.Name())
	b.mu.Lock()
	b.files = append(b.files, name)
	b.mu.Unlock()
	if name == b.failFile {
		return errBarrier
	}
	return b.call(callFile, callFile)
}

// Order records the call.
func (b *recording) Order() error { return b.call(callOrder, callOrder) }

// Durable records the call and the directories.
func (b *recording) Durable(dirs []string) error {
	b.mu.Lock()
	b.dirs = slices.Clone(dirs)
	b.mu.Unlock()
	return b.call(callDurable, callDurable)
}

// Close records the call.
func (b *recording) Close() error { return b.call(callClose, callClose) }

// call records one call, and returns errBarrier where the barrier fails
// the method.
func (b *recording) call(method, entry string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, entry)
	if method == b.fail {
		return errBarrier
	}
	return nil
}

// quiet is a barrier that syncs nothing and records nothing, through which
// the allocation checks measure the group alone.
type quiet struct{}

// Start returns nil.
func (quiet) Start(*os.Root, int) error { return nil }

// File returns nil.
func (quiet) File(*os.File) error { return nil }

// Order returns nil.
func (quiet) Order() error { return nil }

// Durable returns nil.
func (quiet) Durable([]string) error { return nil }

// Close returns nil.
func (quiet) Close() error { return nil }

// sealing is a barrier whose File makes the directory dir read-only and
// fails, so the group cannot remove the staging file that it wrote there.
type sealing struct {
	quiet

	dir string
}

// File makes the directory read-only, and returns errBarrier.
func (s sealing) File(*os.File) error { return errors.Join(errBarrier, os.Chmod(s.dir, 0o555)) }

// A group applies a commit's writes and removals between two barriers: a
// target is old or new for a reader and after a crash, and a failure
// fails its own name alone.
func TestGroup(t *testing.T) {
	t.Parallel()

	t.Run("NewGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a group whose commit of nothing calls no barrier", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			b := &recording{}
			failed, err := stagefile.NewGroup(r, stagefile.Reserved, b).Commit()
			assert.NoError(t, err, "the empty commit succeeds")
			assert.Empty(t, failed, "and fails no name")
			assert.Empty(t, b.calls, "the barrier is not called")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing before the commit", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			files.Unchanged(t, os.DirFS(dir), func() { g.Write(target, []byte(body), mode) }, "the root is untouched")
		})

		t.Run("keeps the body without a copy", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			staged := []byte("the first bytes\n")
			g.Write(target, staged, mode)
			copy(staged, "the later bytes\n")
			committed(t, g)
			files.HasContent(t, filepath.Join(dir, target), "the later bytes\n", "the commit writes the bytes it finds")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("removes nothing before the commit", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target: files.Text(body)})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			files.Unchanged(t, os.DirFS(dir), func() { g.Remove(target) }, "the root is untouched")
		})
	})

	t.Run("Commit", func(t *testing.T) {
		t.Parallel()

		stagings := []struct {
			name string
			give stagefile.Staging
		}{
			{name: "writes each target with its body and mode through reserved names", give: stagefile.Reserved},
			{name: "writes each target with its body and mode through unique names", give: stagefile.Unique},
		}
		for _, tt := range stagings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				dir, r := rooted(t, files.Tree{target: files.Text("old and longer\n").WithMode(mode)})
				g := stagefile.NewGroup(r, tt.give, &recording{})
				g.Write(target, []byte(body), mode)
				g.Write(nested, []byte(otherBody), mode)
				committed(t, g)
				files.Equal(t, os.DirFS(dir), files.Tree{
					target: files.Text(body).WithMode(mode),
					nested: files.Text(otherBody).WithMode(mode),
				}, "the targets alone contain their bodies, and no staging file remains")
			})
		}

		t.Run("removes the file of a staged removal", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target: files.Text(body), other: files.Text(otherBody)})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Remove(target)
			committed(t, g)
			files.Equal(t, os.DirFS(dir), files.Tree{other: files.Text(otherBody)}, "the removed file is gone")
		})

		t.Run("truncates a reserved staging file that a killed process left", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{
				target + stagefile.Suffix: files.Text("a stale staging file much longer than the body\n"),
			})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write(target, []byte(body), mode)
			committed(t, g)
			files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body)},
				"the target contains the body alone, and the stale staging file is gone")
		})

		t.Run("calls the barrier's methods in their order", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, files.Tree{other: files.Text(otherBody)})
			b := &recording{}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			g.Write(nested, []byte(otherBody), mode)
			g.Remove(other)
			committed(t, g)
			assert.Equal(t, b.calls, []string{callStart + " 3", callFile, callFile, callOrder, callDurable, callClose},
				"Start with the number of names, File for each write, then Order, Durable and Close")
		})

		t.Run("passes each staging file that it wrote to File", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			b := &recording{}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			g.Write(nested, []byte(otherBody), mode)
			committed(t, g)
			assert.Permutation(t, b.files, []string{target + stagefile.Suffix, "nested.txt" + stagefile.Suffix},
				"File receives the staging file of each write")
		})

		t.Run("names a unique staging file with 16 hex digits after the target", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			b := &recording{}
			g := stagefile.NewGroup(r, stagefile.Unique, b)
			g.Write(target, []byte(body), mode)
			committed(t, g)
			assert.Length(t, b.files, 1, "File receives the one staging file")
			pattern := "^" + regexp.QuoteMeta(target) + `\.[0-9a-f]{16}` + regexp.QuoteMeta(stagefile.Suffix) + "$"
			assert.Matches(t, b.files[0], pattern,
				"the staging name is the target, a dot, 16 hex digits and the suffix")
		})

		t.Run("passes the changed directories and the parents of the created ones to Durable", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, files.Tree{"kept/" + other: files.Text(otherBody)})
			b := &recording{}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(nested, []byte(body), mode)
			g.Remove("kept/" + other)
			committed(t, g)
			assert.Equal(t, b.dirs, []string{".", "a", "a/b", "kept"},
				"the root and a are parents of created directories, and a/b and kept changed")
		})

		t.Run("writes and removes nothing when Start fails", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{other: files.Text(otherBody)})
			b := &recording{fail: callStart}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			g.Remove(other)
			var (
				failed map[string]error
				err    error
			)
			files.Unchanged(t, os.DirFS(dir), func() { failed, err = g.Commit() }, "the root is untouched")
			assert.ErrorIs(t, err, errBarrier, "the commit returns the barrier's error")
			expect.ErrorIs(t, failed[target], stagefile.ErrNotApplied, "the write is not applied")
			expect.ErrorIs(t, failed[other], stagefile.ErrNotApplied, "the removal is not applied")
			assert.Equal(t, b.calls, []string{callStart + " 2", callClose}, "and the barrier is closed")
		})

		t.Run("renames and removes nothing when Order fails", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target: files.Text("old\n"), other: files.Text(otherBody)})
			b := &recording{fail: callOrder}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			g.Remove(other)
			var (
				failed map[string]error
				err    error
			)
			files.Unchanged(t, os.DirFS(dir), func() { failed, err = g.Commit() },
				"every target keeps its old bytes, and no staging file remains")
			assert.ErrorIs(t, err, errBarrier, "the commit returns the barrier's error")
			expect.ErrorIs(t, failed[target], stagefile.ErrNotApplied, "the write is not applied")
			expect.ErrorIs(t, failed[other], stagefile.ErrNotApplied, "the removal is not applied")
			assert.Equal(t, b.calls[len(b.calls)-1], callClose, "and the barrier is closed")
		})

		t.Run("keeps the error of a write that failed before a failed Order", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			b := &recording{fail: callOrder, failFile: target + stagefile.Suffix}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			failed, err := g.Commit()
			assert.ErrorIs(t, err, errBarrier, "the commit returns the barrier's error")
			assert.That(t, failed[target]).
				ErrorIs(errBarrier, "the write keeps the error of its File").
				ErrorIsNot(stagefile.ErrNotApplied, "and not the mark of a write the barrier withheld")
		})

		t.Run("returns the error of File for its name and commits the other names", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			b := &recording{failFile: target + stagefile.Suffix}
			g := stagefile.NewGroup(r, stagefile.Reserved, b)
			g.Write(target, []byte(body), mode)
			g.Write(other, []byte(otherBody), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.ErrorIs(t, failed[target], errBarrier, "the write whose File failed fails")
			files.Equal(t, os.DirFS(dir), files.Tree{other: files.Text(otherBody)},
				"the other target is written, and the failed staging file is removed")
		})

		t.Run("returns the error of Durable after the renames", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{fail: callDurable})
			g.Write(target, []byte(body), mode)
			failed, err := g.Commit()
			assert.ErrorIs(t, err, errBarrier, "the commit returns the barrier's error")
			assert.Empty(t, failed, "and fails no name")
			files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body)}, "the rename is visible")
		})

		t.Run("returns the error of Close after the commit", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{fail: callClose})
			g.Write(target, []byte(body), mode)
			_, err := g.Commit()
			assert.ErrorIs(t, err, errBarrier, "the commit returns the barrier's error")
			files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body)}, "the target is written")
		})

		t.Run("returns the error of a rename for its name and commits the other names", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target + "/kept": files.Text("")})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write(target, []byte(body), mode)
			g.Write(other, []byte(otherBody), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed[target], "the rename over a directory fails")
			files.Equal(t, os.DirFS(dir), files.Tree{target + "/kept": files.Text(""), other: files.Text(otherBody)},
				"the other target is written, and the failed staging file is removed")
		})

		t.Run("returns the error of a removal of a missing file for its name", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Remove(target)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.ErrorIs(t, failed[target], fs.ErrNotExist, "the removal finds nothing")
		})

		t.Run("returns an error for a target whose directory is a file", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{"a": files.Text("")})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("a/"+target, []byte(body), mode)
			var (
				failed map[string]error
				err    error
			)
			files.Unchanged(t, os.DirFS(dir), func() { failed, err = g.Commit() }, "the root is untouched")
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed["a/"+target], "the directory is a file")
		})

		t.Run("returns an error for a target outside the root", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("../"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed["../"+target], "the root jails the write")
		})

		t.Run("returns an error for a staging file that does not open", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target + stagefile.Suffix + "/kept": files.Text("")})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write(target, []byte(body), mode)
			var (
				failed map[string]error
				err    error
			)
			files.Unchanged(t, os.DirFS(dir), func() { failed, err = g.Commit() }, "the root is untouched")
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed[target], "a directory takes the staging file's place")
		})

		t.Run("returns an error for a directory of a target that it cannot stat", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{"closed/kept": files.Text("")})
			restricted(t, filepath.Join(dir, "closed"), 0o600)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("closed/inner/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.ErrorIs(t, failed["closed/inner/"+target], fs.ErrPermission, "the directory refuses its search")
		})

		t.Run("returns an error for a directory of a target that it cannot create", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{"frozen/kept": files.Text("")})
			restricted(t, filepath.Join(dir, "frozen"), 0o500)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("frozen/inner/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.ErrorIs(t, failed["frozen/inner/"+target], fs.ErrPermission, "the directory refuses a new entry")
		})

		t.Run("returns an error for a target whose missing ancestor it cannot create", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			restricted(t, dir, 0o500)
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("x/y/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.ErrorIs(t, failed["x/y/"+target], fs.ErrPermission, "the root refuses the first directory")
		})

		t.Run("returns an error for a target below a dangling link", func(t *testing.T) {
			t.Parallel()

			// A dangling link stands where the directory belongs, so the
			// directory exists for a creation and is missing for a stat.
			_, r := rooted(t, files.Tree{"svc": files.Link("ghost")})
			g := stagefile.NewGroup(r, stagefile.Reserved, &recording{})
			g.Write("svc/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed["svc/"+target], "the staging file does not open below the link")
		})

		t.Run("joins the error of the removal of a failed staging file", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{"sealed/kept": files.Text("")})
			sealed := filepath.Join(dir, "sealed")
			restricted(t, sealed, 0o755)
			g := stagefile.NewGroup(r, stagefile.Reserved, sealing{dir: sealed})
			g.Write("sealed/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.That(t, failed["sealed/"+target]).
				ErrorIs(errBarrier, "the error of File returns").
				ErrorIs(fs.ErrPermission, "beside the error of the removal")
		})

		t.Run("leaves one group's whole body when groups write one target at once", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			bodies := make([]string, 16)
			for i := range bodies {
				bodies[i] = strings.Repeat(strconv.Itoa(i%10), 4096+i)
			}
			outcomes := history.Concurrently(len(bodies), time.Minute, func(writer int) (any, error) {
				g := stagefile.NewGroup(r, stagefile.Unique, stagefile.System())
				g.Write(target, []byte(bodies[writer]), mode)
				failed, err := g.Commit()
				return writer, errors.Join(err, failed[target])
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "each group finishes")
				expect.NoError(t, o.Error, "each group commits the target")
			}
			got := files.Read(t, filepath.Join(dir, target))
			assert.Contains(t, bodies, got, "the target is one group's whole body")
			files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(got)}, "no staging file remains")
		})
	})

	t.Run("Each", func(t *testing.T) {
		t.Parallel()

		t.Run("calls do once for each index below n", func(t *testing.T) {
			t.Parallel()

			calls := make([]atomic.Int32, 3*stagefile.Workers)
			stagefile.Each(len(calls), func(i int) { calls[i].Add(1) })
			for i := range calls {
				expect.Equal(t, calls[i].Load(), int32(1), "each index is called once")
			}
		})

		t.Run("calls do once with the index zero for an n of one", func(t *testing.T) {
			t.Parallel()

			var got []int
			stagefile.Each(1, func(i int) { got = append(got, i) })
			assert.Equal(t, got, []int{0}, "the one index is called once")
		})

		t.Run("calls do for no index of an n of zero", func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			stagefile.Each(0, func(int) { calls.Add(1) })
			assert.Equal(t, calls.Load(), int32(0), "nothing is called")
		})
	})
}

// A group allocates within its ceilings in the ordinary run, which runs
// no benchmark. The commit writes through a barrier that syncs nothing,
// so the count is the group's own. Darwin and Windows allocate more in
// their file system calls, so the test runs on Linux alone.
func TestGroupAllocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ceilings count the allocations of Linux's file system calls")
	}
	dir, r := rooted(t, files.Tree{target: files.Text(body)})
	data := []byte(body)
	var g *stagefile.Group
	assert.MaxAllocs(t, func() { g = stagefile.NewGroup(r, stagefile.Reserved, quiet{}) }, newGroupAllocs,
		"NewGroup allocates the group")
	assert.MaxAllocsWithSetup(t, func() *stagefile.Group { return stagefile.NewGroup(r, stagefile.Reserved, quiet{}) },
		func(fresh *stagefile.Group) { fresh.Write(target, data, mode) }, firstWriteAllocs,
		"the first Write grows the list of entries")
	var err error
	assert.MaxAllocsWithSetup(t, func() *stagefile.Group {
		fresh := stagefile.NewGroup(r, stagefile.Reserved, quiet{})
		fresh.Write(target, data, mode)
		return fresh
	}, func(fresh *stagefile.Group) {
		failed, cerr := fresh.Commit()
		err = errors.Join(err, cerr, failed[target])
	}, commitOneAllocs, "Commit allocates the staging file and the rename of one write")
	assert.NoError(t, err, "every commit succeeds")
	assert.NotNil(t, g, "NewGroup returns the group")
	files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body)}, "and leaves the target alone")
}

// BenchmarkGroup measures a group over one write of an existing target in
// a temporary directory, through a barrier that syncs nothing.
func BenchmarkGroup(b *testing.B) {
	dir, r := rooted(b, files.Tree{target: files.Text(body)})
	data := []byte(body)

	b.Run("NewGroup", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newGroupAllocs)
		defer c.End()
		var g *stagefile.Group
		for c.Loop() {
			g = stagefile.NewGroup(r, stagefile.Reserved, quiet{})
		}
		assert.NotNil(b, g, "NewGroup returns the group")
	})

	b.Run("Write", func(b *testing.B) {
		b.Run("the first write of a group", func(b *testing.B) {
			var g *stagefile.Group
			fresh := func() { g = stagefile.NewGroup(r, stagefile.Reserved, quiet{}) }
			c := bench.Start(b).MaxAllocs(firstWriteAllocs)
			defer c.End()
			for c.Loop() {
				c.Excluding(fresh)
				g.Write(target, data, mode)
			}
			assert.NotNil(b, g, "the write is staged")
		})
	})

	b.Run("Commit", func(b *testing.B) {
		b.Run("a write of an existing target", func(b *testing.B) {
			var g *stagefile.Group
			fresh := func() {
				g = stagefile.NewGroup(r, stagefile.Reserved, quiet{})
				g.Write(target, data, mode)
			}
			c := bench.Start(b).MaxAllocs(commitOneAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				failed, cerr := g.Commit()
				err = errors.Join(err, cerr, failed[target])
			}
			assert.NoError(b, err, "every commit succeeds")
			files.Equal(b, os.DirFS(dir), files.Tree{target: files.Text(body)}, "and leaves the target alone")
		})
	})
}

// committed commits a group, failing the case where a name or a barrier
// fails.
func committed(t *testing.T, g *stagefile.Group) {
	t.Helper()

	failed, err := g.Commit()
	assert.NoError(t, err, "no barrier fails")
	assert.Empty(t, failed, "and no name fails")
}

// restricted sets the mode of the directory at dir, and restores 0o755
// when the case ends. It skips the case where a mode does not restrict
// the process: on Windows, and for the superuser.
func restricted(t *testing.T, dir string, m fs.FileMode) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not restrict a directory by its mode")
	}
	if os.Geteuid() == 0 {
		t.Skip("the mode of a directory does not restrict the superuser")
	}
	assert.NoError(t, os.Chmod(dir, m), "the mode of the directory is set")
	t.Cleanup(func() { assert.NoError(t, os.Chmod(dir, 0o755), "the mode of the directory is restored") })
}
