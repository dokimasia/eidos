// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/conformance/lang/python"
)

// corpusTree is the corpus tree of Python, relative to the directory of the
// package that the tests run in.
const corpusTree = "testdata/corpus"

// TestCorpus checks that Python passes its own run against the shared
// inventory.
func TestCorpus(t *testing.T) {
	t.Parallel()

	t.Run("Corpus", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance run over the Python tree", func(t *testing.T) {
			t.Parallel()

			conformance.Run(t, python.Corpus(os.DirFS(corpusTree)))
		})
	})
}
