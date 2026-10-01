// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"context"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter/java"
)

// The source the grammar parses, and the name it loads under.
const (
	javaFile   = "src/A.java"
	javaSource = "package a;\n\npublic class A {\n  void run() {}\n}\n"
	javaName   = "java"
)

// The package loads one grammar, so the grammar is pinned as the one
// that parses Java.
func TestJava(t *testing.T) {
	t.Parallel()

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a class without an error", func(t *testing.T) {
			t.Parallel()

			tree, err := java.Grammar.Parse(context.Background(), javaFile, []byte(javaSource))
			assert.NoError(t, err, "the source parses")
			defer tree.Close()
			assert.Empty(t, slices.Collect(tree.Errors()), "the source fits the grammar")
		})

		t.Run("loads under the language's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Grammar.Name(), javaName, "the name a frontend reports")
		})
	})
}
