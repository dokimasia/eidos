// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	gocli "go.stealthscale.io/cli"
)

// windows is the GOOS of Windows, whose environment compares variable names
// without regard to case.
const windows = "windows"

// IO contains the standard streams, the working directory and the
// environment of the process that runs a command.
type IO struct {
	// Stdout and Stderr receive the output of the command.
	Stdout, Stderr io.Writer
	// Dir is the absolute working directory.
	Dir string
	// Getenv returns the value of an environment variable, and the empty
	// string for a variable that is not set.
	Getenv func(key string) string
	// Terminal is true when Stderr is a terminal. Text output colours the
	// severities only on a terminal. A host that replaces Stderr sets
	// Terminal for the writer that replaces it.
	Terminal bool
}

// IOOf returns the IO of run: its standard streams, the working directory
// of the command, the environment of the run, and whether its standard
// error is a terminal. The working directory is Chdir of the global options,
// resolved against the working directory of the process, or the working
// directory of the process for an empty Chdir. Symbolic links in the
// directory are resolved.
//
// Error modes: IOOf returns an IO without a Dir, and an error that wraps a
// *UsageError, when os.Getwd fails, when the directory does not resolve and
// when it is not a directory. [Begin] renders such an IO.
func IOOf[C any](run *gocli.Run[Global, C]) (IO, error) {
	stdio := IO{
		Stdout:   run.Stdout(),
		Stderr:   run.Stderr(),
		Getenv:   getenv(run.Environ()),
		Terminal: terminal(run.Stderr()),
	}
	_, dir, err := workdir(run.Global().Chdir)
	if err != nil {
		return stdio, err
	}
	stdio.Dir = dir
	return stdio, nil
}

// check panics unless s has both output streams and Getenv. A missing part
// is a defect of the host.
func (s IO) check() {
	if s.Stdout == nil || s.Stderr == nil {
		panic("cli: the IO has no output stream")
	} else if s.Getenv == nil {
		panic("cli: the IO has no Getenv")
	}
}

// workdir returns the working directory of the process, and the directory
// of a command whose Chdir is chdir: chdir resolved against the working
// directory of the process with its symbolic links resolved, or the
// resolved working directory of the process for an empty chdir.
//
// Error modes: workdir returns an error that wraps a *UsageError when
// os.Getwd fails, when the directory of the command does not resolve, and
// when it is not a directory. On macOS, os.Getwd returns the former path of
// a removed working directory, whose resolution fails.
func workdir(chdir string) (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", &UsageError{Err: fmt.Errorf("cli: find the working directory: %w", err)}
	}
	dir, err := filepath.EvalSymlinks(resolve(cwd, chdir))
	if err != nil {
		return "", "", &UsageError{Err: fmt.Errorf("cli: find the working directory: %w", err)}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", "", &UsageError{Err: fmt.Errorf("cli: find the working directory: %s is not a directory", dir)}
	}
	return cwd, dir, nil
}

// terminal reports whether w is a file of a character device, such as the
// terminal of the process.
func terminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// getenv returns the Getenv of an IO over environ, the pairs KEY=value of
// an environment. Getenv returns the value of the first pair of a variable,
// as os.Getenv does. It compares the names of variables without regard to
// case on Windows.
func getenv(environ []string) func(key string) string {
	return func(key string) string {
		for _, pair := range environ {
			name, value, ok := strings.Cut(pair, "=")
			if ok && (name == key || runtime.GOOS == windows && strings.EqualFold(name, key)) {
				return value
			}
		}
		return ""
	}
}
