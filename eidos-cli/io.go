// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
	// severities only on a terminal.
	Terminal bool
}

// ProcessIO returns the IO of the running process: [os.Stdout],
// [os.Stderr], the working directory and [os.Getenv]. It sets Terminal when
// os.Stderr is a character device.
//
// Error modes: ProcessIO returns the error of [os.Getwd].
func ProcessIO() (IO, error) {
	dir, err := os.Getwd()
	if err != nil {
		return IO{}, fmt.Errorf("cli: find the working directory: %w", err)
	}
	info, err := os.Stderr.Stat()
	terminal := err == nil && info.Mode()&os.ModeCharDevice != 0
	return IO{Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir, Getenv: os.Getenv, Terminal: terminal}, nil
}

// check panics unless s has both output streams, Getenv and an absolute
// Dir. A missing part is a defect of the host.
func (s IO) check() {
	if s.Stdout == nil || s.Stderr == nil {
		panic("cli: the IO has no output stream")
	} else if s.Getenv == nil {
		panic("cli: the IO has no Getenv")
	} else if !filepath.IsAbs(s.Dir) {
		panic(fmt.Sprintf("cli: the working directory %q of the IO is not absolute", s.Dir))
	}
}
