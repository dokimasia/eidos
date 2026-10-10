// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/conformance/lang/rust"
)

// corpusTree is Rust's corpus tree, relative to the package directory
// the tests run in.
const corpusTree = "testdata/corpus"

// Rust passes its own run against the shared inventory.
func TestCorpus(t *testing.T) {
	t.Parallel()

	t.Run("Corpus", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance run over the Rust tree", func(t *testing.T) {
			t.Parallel()

			conformance.Run(t, rust.Corpus(os.DirFS(corpusTree)))
		})
	})
}
