// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Command cmd regenerates the SDK facade module from the kernel's
// curated packages.
//
// It is the thin wrapper `go generate` runs; the generator itself
// is [facade.Generate], which the mirror guard calls in-process.
// It finds the kernel module from the working directory and writes
// into the facade module beside it, so it runs from anywhere
// inside the kernel.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run generates the facade and writes it, returning the first
// failure. Nothing is written unless every package rendered.
func run() error {
	kernelRoot, err := gosource.ModuleRoot(".")
	if err != nil {
		return err
	}
	got, err := gosource.ModulePath(kernelRoot)
	if err != nil {
		return err
	}
	if got != facade.KernelModule {
		return fmt.Errorf("run inside the kernel module: %s holds %s", kernelRoot, got)
	}
	repoRoot := filepath.Dir(kernelRoot)
	set, err := facade.Generate(repoRoot)
	if err != nil {
		return err
	}
	return genfile.Write(repoRoot, set)
}
