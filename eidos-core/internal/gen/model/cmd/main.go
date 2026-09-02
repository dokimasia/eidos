// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command cmd regenerates the declaration models from the symbol
// schema.
//
// It is the thin wrapper `go generate` runs; the generation itself
// is [model.Regenerate], which a test drives in-process. It finds
// the enclosing module from the working directory, so it runs from
// anywhere inside the kernel.
package main

import (
	"fmt"
	"os"

	"go.dokimi.dev/eidos/core/internal/gen/model"
)

// workingDir is where the wrapper starts looking for the enclosing
// module: wherever `go generate` invoked it.
const workingDir = "."

func main() {
	if err := model.Regenerate(workingDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
