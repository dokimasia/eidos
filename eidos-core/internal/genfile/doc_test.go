// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package genfile_test

import (
	"go/token"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// The package documentation is part of the contract: it states the
// dependency position every other package relies on.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()

		files, err := gosource.ParseDir(token.NewFileSet(), ".", gosource.HandWritten)
		if err != nil {
			t.Fatalf("ParseDir: %v", err)
		}
		for _, file := range files {
			if file.Doc == nil {
				continue
			}
			if strings.Contains(file.Doc.Text(), "# Dependency position") {
				return
			}
		}
		t.Fatal("no file carries the package comment with its dependency position")
	})
}
