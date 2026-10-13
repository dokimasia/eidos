// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
)

// The intervals of the watch cases: the shortest interval that watch
// accepts, and an interval below it.
const (
	shortest = intervalFlag + "=100ms"
	tooShort = intervalFlag + "=50ms"
)

// The passes during whose load the cases write their edit of the tree and
// end the context of watch. The load calls the locator after the walk of
// the tree, so the pass after editPass is the first pass that sees the
// edit.
const (
	editPass = 2
	lastPass = 4
)

// editedSummary is the summary line of text output for two rendered passes.
// The first pass created one file, the second pass updated it, and each
// pass committed one plan.
const editedSummary = "files: 1 create, 1 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; " +
	"plans: 2 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n"

// secondSource is the source of the store with a second struct.
const secondSource = source + "type Second string\n"

// cancelling is a writer that cancels a context at its first write, and
// keeps what it receives.
type cancelling struct {
	bytes.Buffer
	cancel context.CancelFunc
}

// Write cancels the context, and appends p to the buffer.
func (w *cancelling) Write(p []byte) (int, error) {
	w.cancel()
	return w.Buffer.Write(p)
}

// Watch runs each workspace in passes until its context ends, and renders
// a pass only when the pass differs from the previous one.
func TestWatch(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("returns StatusOK when the context ends between two passes", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := watched(t, stored(t, nil))
			expect.Equal(t, status, cli.StatusOK, "the end of the context stops watch")
			expect.Equal(t, stdout, "create svc/mirror.txt\n"+createdSummary, "the first pass renders its change")
		})

		t.Run("renders nothing for a pass that changes nothing", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := passed(t, stored(t, nil), nil, nil)
			expect.Equal(t, status, cli.StatusOK, "the end of the context stops watch")
			expect.Equal(t, stdout, "create svc/mirror.txt\n"+createdSummary, "the later passes render nothing")
			expect.Empty(t, stderr, "no pass reports anything")
		})

		t.Run("renders a pass after an edit of the tree", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := passed(t, stored(t, nil), files.Tree{storePath: files.Text(secondSource)}, nil)
			assert.Equal(t, stdout, "create svc/mirror.txt\nupdate svc/mirror.txt\n"+editedSummary,
				"the pass after the edit renders the update of the mirror")
		})

		t.Run("writes the events of its passes between the start event and the summary event", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := passed(t, stored(t, nil), nil, nil, jsonFlag)
			assert.Equal(t, decoded[header](t, stdout, ""),
				[]header{{Event: eventStart}, {Event: eventFile}, {Event: eventOutcome}, {Event: eventSummary}},
				"the output has the start event, the events of the first pass and the summary event")
		})

		t.Run("renders the StateLocked finding of the passes once", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			locked(t, filepath.Join(root, memberSvc))
			_, _, stderr := passed(t, root, nil, nil)
			assert.Equal(t, strings.Count(stderr, workspace.StateLocked.String()), 1,
				"the later passes find the same lock and render nothing")
		})

		t.Run("renders the error of the passes once", func(t *testing.T) {
			t.Parallel()

			_, _, stderr := passed(t, stored(t, files.Tree{storePath: files.Text(brokenSource)}), nil, nil)
			assert.Equal(t, stderr, brokenError, "the later passes return the same error and render nothing")
		})

		t.Run("renders the error of a store locator once", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := passed(t, stored(t, nil), nil, errLocate)
			expect.Equal(t, status, cli.StatusOK, "a failed pass does not stop watch")
			expect.Equal(t, stderr, locateError, "the later passes return the same error and render nothing")
		})

		t.Run("runs only the members of a list that contain a pattern", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			_, stdout, _ := watched(t, root, memberTool)
			expect.HasPrefix(t, stdout, "create tools/mirror.txt\n", "the pass runs tools")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the pass does not run svc")
		})

		t.Run("returns StatusUsage for an interval below 100ms", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdWatch, stored(t, nil), tooShort)
			expect.Equal(t, status, cli.StatusUsage, "the interval is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: watch: the interval 50ms is shorter than 100ms\n",
				"watch writes the error")
		})

		t.Run("returns StatusUsage for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdWatch, root)
			expect.Equal(t, status, cli.StatusUsage, "the config error is a usage error")
			expect.Contains(t, stderr, "field limt not found", "watch writes the fault")
		})

		t.Run("returns StatusUsage for a pattern outside the root", func(t *testing.T) {
			t.Parallel()

			status, _, _ := invoke(t, compose, cmdWatch, stored(t, nil), filepath.Join("..", "elsewhere"))
			assert.Equal(t, status, cli.StatusUsage, "the pattern is a usage error")
		})

		t.Run("returns StatusUsage for a pattern that the run refuses", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdWatch, stored(t, nil), refusedPattern)
			expect.Equal(t, status, cli.StatusUsage, "the pattern is a usage error")
			expect.Contains(t, stderr, `the pattern "svc/a...b" names no directory`,
				"watch writes the error of the run")
		})
	})
}

// watched runs watch over the fixture composition in dir with args, under a
// context that the first write to standard output cancels. It returns the
// status of watch and what it wrote to standard output and to standard
// error.
func watched(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stdout := &cancelling{cancel: cancel}
	var stderr bytes.Buffer
	env := &gocli.Env{
		Open:   gocli.OS().Open,
		Stdout: stdout,
		Stderr: &stderr,
		Args:   slices.Concat([]string{chdirFlag, dir, cmdWatch}, args),
	}
	status := cli.Run(ctx, cli.New(appHelp, compose), compose, env)
	return status, stdout.String(), stderr.String()
}

// passed runs watch over the locating composition in dir at the shortest
// interval, with args after the interval. The load of each pass calls the
// locator after the walk of the tree, and the locator returns fail. During
// editPass, passed writes edit into dir. During lastPass, it ends the
// context of watch. It returns the status of watch and what it wrote to
// standard output and to standard error. A workspace of dir without a
// tree to load calls no locator, so a list in dir needs one member that
// loads.
func passed(t *testing.T, dir string, edit files.Tree, fail error, args ...string) (int, string, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	passes := 0
	locating := locatingCompose(func(func(string) string) (map[string]fs.FS, error) {
		passes++
		if passes == editPass {
			files.Write(t, dir, edit)
		}
		if passes == lastPass {
			cancel()
		}
		return nil, fail
	})
	var stdout, stderr bytes.Buffer
	env := &gocli.Env{
		Open:   gocli.OS().Open,
		Stdout: &stdout,
		Stderr: &stderr,
		Args:   slices.Concat([]string{chdirFlag, dir, cmdWatch, shortest}, args),
	}
	status := cli.Run(ctx, cli.New(appHelp, locating), locating, env)
	return status, stdout.String(), stderr.String()
}
