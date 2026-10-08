// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
)

// The intervals of the watch cases: the shortest interval that watch
// accepts, and an interval below it.
const (
	shortest = intervalFlag + "=100ms"
	tooShort = intervalFlag + "=50ms"
)

// lastPass is the pass at whose start the cases of three passes end the
// context of watch.
const lastPass = 3

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

			status, stdout, stderr := passed(t, stored(t, nil))
			expect.Equal(t, status, cli.StatusOK, "the end of the context stops watch")
			expect.Equal(t, stdout, "create svc/mirror.txt\n"+createdSummary, "the second pass renders nothing")
			expect.Empty(t, stderr, "no pass reports anything")
		})

		t.Run("renders the StateLocked finding of the passes once", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			locked(t, root)
			_, _, stderr := passed(t, root)
			assert.Equal(t, strings.Count(stderr, workspace.StateLocked.String()), 1,
				"the second pass finds the same lock and renders nothing")
		})

		t.Run("renders the error of the passes once", func(t *testing.T) {
			t.Parallel()

			_, _, stderr := passed(t, stored(t, files.Tree{storePath: files.Text(brokenSource)}))
			assert.Equal(t, stderr, brokenError, "the second pass returns the same error and renders nothing")
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
			expect.Contains(t, stderr, "field workerz not found", "watch writes the fault")
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

		t.Run("returns StatusFailed for a locator that returns an error", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, failing, cmdWatch, stored(t, nil))
			expect.Equal(t, status, cli.StatusFailed, "the locator fails watch")
			expect.Equal(t, stderr, locateError, "watch writes the error of the locator")
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
	stdio := cli.IO{Stdout: stdout, Stderr: &stderr, Dir: dir, Getenv: func(string) string { return "" }}
	status := command(t, compose, cmdWatch).Run(ctx, stdio, args)
	return status, stdout.String(), stderr.String()
}

// passed runs watch over the locating composition in dir at the shortest
// interval, and ends its context at the start of lastPass, when the
// locator reads storeVar for that time. It returns the status of watch and
// what it wrote to standard output and to standard error.
func passed(t *testing.T, dir string) (int, string, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reads := 0
	getenv := func(key string) string {
		if key == storeVar {
			reads++
		}
		if reads == lastPass {
			cancel()
		}
		return ""
	}
	locating := locatingCompose(func(getenv func(string) string) (map[string]fs.FS, error) {
		getenv(storeVar)
		return nil, nil
	})
	var stdout, stderr bytes.Buffer
	stdio := cli.IO{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: getenv}
	status := command(t, locating, cmdWatch).Run(ctx, stdio, []string{shortest})
	return status, stdout.String(), stderr.String()
}
