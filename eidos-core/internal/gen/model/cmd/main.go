// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Command cmd regenerates the declaration models from the symbol
// schema.
//
// It is the thin wrapper `go generate` runs; the generator itself
// is [model.Generate], which the mirror guard calls in-process. It
// finds the enclosing module from the working directory, so it runs
// from anywhere inside the kernel.
package main

import (
	"fmt"
	"os"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run generates the models and writes them, answering the first
// failure. Nothing is written unless every file rendered.
func run() error {
	root, err := gosource.ModuleRoot(".")
	if err != nil {
		return err
	}
	set, err := model.Generate(root)
	if err != nil {
		return err
	}
	return genfile.Write(root, set)
}
