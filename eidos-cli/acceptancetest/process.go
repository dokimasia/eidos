// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
)

// The go command and the arguments with which Build compiles a binary.
const (
	goCommand = "go"
	goBuild   = "build"
	outFlag   = "-o"
)

// timeout is the time that the kit gives one process of a binary and the
// compilation of the output of a run. A process of a fixture binary over a
// small tree finishes within a second, so a process that runs for minutes
// hangs.
const timeout = 2 * time.Minute

// Result is what one process of a binary returned.
type Result struct {
	// Status is the exit status of the process.
	Status int
	// Stdout and Stderr are what the process wrote to standard output and
	// to standard error.
	Stdout, Stderr []byte
}

// Build compiles the main package main with the go command into a new
// temporary directory of tb, and returns the path of the binary. The go
// command runs in the working directory of the test, so it resolves main in
// the module of the package under test. The build cache of the go command
// turns a second build of an unchanged package into a link. Build stops the
// test when the package does not compile, and writes the output of the go
// command into the failure.
func Build(tb testing.TB, main string) string {
	tb.Helper()

	out := tb.TempDir()
	// The go command writes the binary into the existing directory under
	// the name of the package, with the extension of an executable of the
	// system.
	compiled, err := exec.CommandContext(tb.Context(), goCommand, goBuild, outFlag, out, main).CombinedOutput()
	assert.NoError(tb, err, "the go command builds "+main+": "+string(compiled))
	entries, err := os.ReadDir(out)
	assert.NoError(tb, err, "the directory of the binary of "+main+" reads")
	assert.Length(tb, entries, 1, "the go command writes one binary of "+main)
	return filepath.Join(out, entries[0].Name())
}

// Exec runs the binary bin in the directory dir with args, and returns the
// status of the process and what the process wrote. The process inherits
// the environment of the test process, because a frontend locates its
// stores through the environment, such as the JDK of the Java frontend
// through JAVA_HOME. Each variable of env, in the form name=value, replaces
// the inherited variable of its name. Exec kills a process that runs longer
// than two minutes, and stops the check for it and for a process that does
// not start.
func Exec(tb assert.TB, bin, dir string, env []string, args ...string) Result {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	assert.NoError(tb, ctx.Err(), "the process of "+bin+" finishes within "+timeout.String())
	if _, exited := errors.AsType[*exec.ExitError](err); !exited {
		assert.NoError(tb, err, "the process of "+bin+" starts")
	}
	return Result{Status: cmd.ProcessState.ExitCode(), Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
}
