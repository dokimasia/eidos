// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
)

// The variable of the environment cases, in the case of its name, in
// another case, and with two values.
const (
	casedVar    = "Acme_Store"
	recasedVar  = "ACME_STORE"
	firstValue  = "first"
	secondValue = "second"
	absentVar   = "CLITEST_ABSENT"
)

// The IO of a run has the streams of the run, the working directory of its
// command and the environment of the run.
func TestIO(t *testing.T) {
	t.Parallel()

	t.Run("IOOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the streams, the environment and the directory of Chdir", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{})
			var stdout, stderr bytes.Buffer
			got, err := cli.IOOf(newRun(t, cli.Global{Chdir: dir}, &gocli.Env{
				Environ: gocli.Environ(map[string]string{storeVar: storeValue}), Stdout: &stdout, Stderr: &stderr,
			}))
			assert.NoError(t, err, "the directory exists")
			expect.Equal(t, got.Stdout, io.Writer(&stdout), "the IO writes to the standard output of the run",
				assert.ByIdentity())
			expect.Equal(t, got.Stderr, io.Writer(&stderr), "the IO writes to the standard error of the run",
				assert.ByIdentity())
			expect.Equal(t, got.Dir, dir, "the IO has the directory of Chdir")
			expect.Equal(t, got.Getenv(storeVar), storeValue, "the IO reads the environment of the run")
			expect.Equal(t, got.Getenv(absentVar), "", "a variable that the environment lacks is empty")
			expect.False(t, got.Terminal, "a buffer is no terminal")
		})

		t.Run("returns the working directory of the process for an empty Chdir", func(t *testing.T) {
			t.Parallel()

			cwd, err := os.Getwd()
			assert.NoError(t, err, "the test reads the working directory")
			want, err := filepath.EvalSymlinks(cwd)
			assert.NoError(t, err, "the working directory resolves")
			got, err := cli.IOOf(newRun(t, cli.Global{}, &gocli.Env{Stdout: io.Discard, Stderr: io.Discard}))
			assert.NoError(t, err, "the working directory exists")
			assert.Equal(t, got.Dir, want, "the IO has the working directory of the process")
		})

		t.Run("resolves a relative Chdir against the working directory of the process", func(t *testing.T) {
			t.Parallel()

			cwd, err := os.Getwd()
			assert.NoError(t, err, "the test reads the working directory")
			dir := workspaceDir(t, files.Tree{})
			rel, err := filepath.Rel(cwd, dir)
			if err != nil {
				t.Skip("the temporary directory is on another volume than the working directory")
			}
			got, err := cli.IOOf(newRun(t, cli.Global{Chdir: rel}, &gocli.Env{Stdout: io.Discard, Stderr: io.Discard}))
			assert.NoError(t, err, "the directory exists")
			assert.Equal(t, got.Dir, dir, "the IO has the directory of Chdir")
		})

		t.Run("returns the value of the first pair of a variable", func(t *testing.T) {
			t.Parallel()

			environ := func() []string { return []string{casedVar + "=" + firstValue, casedVar + "=" + secondValue} }
			run := newRun(t, cli.Global{}, &gocli.Env{Environ: environ, Stdout: io.Discard, Stderr: io.Discard})
			got, err := cli.IOOf(run)
			assert.NoError(t, err, "the working directory exists")
			assert.Equal(t, got.Getenv(casedVar), firstValue, "the first pair sets the variable")
		})

		t.Run("compares the names of variables by their case outside Windows", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == windowsOS {
				t.Skip("Windows compares the names of variables without regard to case")
			}
			got := ioWith(t, casedVar)
			assert.Equal(t, got.Getenv(recasedVar), "", "a name in another case is another variable")
		})

		t.Run("compares the names of variables without regard to case on Windows", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS != windowsOS {
				t.Skip("only Windows compares the names of variables without regard to case")
			}
			got := ioWith(t, casedVar)
			assert.Equal(t, got.Getenv(recasedVar), firstValue, "a name in another case is the same variable")
		})

		t.Run("reports a terminal for a standard error that is a character device", func(t *testing.T) {
			t.Parallel()

			device, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			assert.NoError(t, err, "the null device opens")
			t.Cleanup(func() { expect.NoError(t, device.Close(), "the null device closes") })
			got, err := cli.IOOf(newRun(t, cli.Global{}, &gocli.Env{Stdout: io.Discard, Stderr: device}))
			assert.NoError(t, err, "the working directory exists")
			assert.True(t, got.Terminal, "the null device is a character device")
		})

		t.Run("reports no terminal for a standard error that is a regular file", func(t *testing.T) {
			t.Parallel()

			file, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
			assert.NoError(t, err, "the file of standard error is created")
			t.Cleanup(func() { expect.NoError(t, file.Close(), "the file of standard error closes") })
			got, err := cli.IOOf(newRun(t, cli.Global{}, &gocli.Env{Stdout: io.Discard, Stderr: file}))
			assert.NoError(t, err, "the working directory exists")
			assert.False(t, got.Terminal, "a regular file is no terminal")
		})

		failures := []struct {
			name string
			give func(t *testing.T) string
		}{
			{
				name: "returns an error that wraps a UsageError for a Chdir that does not exist",
				give: func(t *testing.T) string {
					t.Helper()

					return filepath.Join(t.TempDir(), missingDir)
				},
			},
			{
				name: "returns an error that wraps a UsageError for a Chdir that is a file",
				give: func(t *testing.T) string {
					t.Helper()

					root := workspaceDir(t, files.Tree{storePath: files.Text(source)})
					return filepath.Join(root, filepath.FromSlash(storePath))
				},
			},
		}
		for _, tt := range failures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout bytes.Buffer
				run := newRun(t, cli.Global{Chdir: tt.give(t)}, &gocli.Env{Stdout: &stdout, Stderr: io.Discard})
				got, err := cli.IOOf(run)
				usage := assert.ErrorAs[*cli.UsageError](t, err, "the directory is a usage error")
				expect.HasPrefix(t, usage.Error(), "cli: find the working directory: ", "the error names the directory")
				expect.Equal(t, got.Dir, "", "the IO has no directory")
				expect.Equal(t, got.Stdout, io.Writer(&stdout), "the IO has the streams", assert.ByIdentity())
			})
		}
	})
}

// IOOf returns an error when the working directory of the process does not
// exist. The case changes the working directory of the process, so it runs
// alone.
func TestIOProcess(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Windows does not remove the working directory of a process")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	assert.NoError(t, os.Remove(dir), "the test removes the working directory")
	_, err := cli.IOOf(newRun(t, cli.Global{}, &gocli.Env{Stdout: io.Discard, Stderr: io.Discard}))
	usage := assert.ErrorAs[*cli.UsageError](t, err, "a removed working directory is a usage error")
	assert.HasPrefix(t, usage.Error(), "cli: find the working directory: ", "the error describes the directory")
}

// newRun returns a run of the global options global and of a command
// without options, with the streams and the environment of env, as a
// handler receives it.
func newRun(t *testing.T, global cli.Global, env *gocli.Env) *gocli.Run[cli.Global, struct{}] {
	t.Helper()

	run, err := gocli.NewRun(&global, &struct{}{}, env)
	assert.NoError(t, err, "the run has its options and both streams")
	return run
}

// ioWith returns the IO of a run whose environment has the variable name
// with the value firstValue.
func ioWith(t *testing.T, name string) cli.IO {
	t.Helper()

	environ := gocli.Environ(map[string]string{name: firstValue})
	got, err := cli.IOOf(newRun(t, cli.Global{}, &gocli.Env{Environ: environ, Stdout: io.Discard, Stderr: io.Discard}))
	assert.NoError(t, err, "the working directory exists")
	return got
}
