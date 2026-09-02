// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command cmd regenerates the SDK facade module from the kernel's
// curated packages.
//
// It is the thin wrapper `go generate` runs; the generation itself
// is [facade.Regenerate], which a test drives in-process. It finds
// the kernel module from the working directory and writes into the
// facade module beside it, so it runs from anywhere inside the
// kernel.
package main

import (
	"fmt"
	"os"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// workingDir is where the wrapper starts looking for the kernel
// module: wherever `go generate` invoked it.
const workingDir = "."

func main() {
	if err := facade.Regenerate(workingDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
