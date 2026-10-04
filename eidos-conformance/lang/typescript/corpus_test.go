// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/conformance/lang/typescript"
)

// corpusTree is TypeScript's corpus tree, relative to the package
// directory the tests run in.
const corpusTree = "testdata/corpus"

// TypeScript passes its own run against the shared inventory.
func TestCorpus(t *testing.T) {
	t.Parallel()

	t.Run("Corpus", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance run over the TypeScript tree", func(t *testing.T) {
			t.Parallel()

			conformance.Run(t, typescript.Corpus(os.DirFS(corpusTree)))
		})
	})
}
