// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource_test

import (
	"go/token"
	"path/filepath"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

func TestImporter(t *testing.T) {
	t.Parallel()

	modRoot, err := filepath.Abs("testdata/mod")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	t.Run("Import", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a module-local package from disk", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			if err != nil {
				t.Fatalf("NewImporter: unexpected error: %v", err)
			}
			pkg, err := imp.Import("example.test/fixture/lib")
			if err != nil {
				t.Fatalf("Import: unexpected error: %v", err)
			}
			if pkg.Scope().Lookup("Value") == nil {
				t.Fatal("Value is missing from the imported package")
			}
			if pkg.Scope().Lookup("Generated") != nil {
				t.Fatal("Generated resolved: the importer read a .gen.go file")
			}
		})

		t.Run("answers the same package on a second call", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			if err != nil {
				t.Fatalf("NewImporter: unexpected error: %v", err)
			}
			first, err := imp.Import("example.test/fixture/lib")
			if err != nil {
				t.Fatalf("Import: unexpected error: %v", err)
			}
			second, err := imp.Import("example.test/fixture/lib")
			if err != nil {
				t.Fatalf("Import: unexpected error: %v", err)
			}
			if first != second {
				t.Fatal("Import answered two packages for one path")
			}
		})

		t.Run("delegates a standard-library path", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			if err != nil {
				t.Fatalf("NewImporter: unexpected error: %v", err)
			}
			pkg, err := imp.Import("strings")
			if err != nil {
				t.Fatalf("Import: unexpected error: %v", err)
			}
			if pkg.Scope().Lookup("ToUpper") == nil {
				t.Fatal("ToUpper is missing: strings did not resolve")
			}
		})

		t.Run("reports a module-local path that does not exist", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			if err != nil {
				t.Fatalf("NewImporter: unexpected error: %v", err)
			}
			if _, err := imp.Import("example.test/fixture/missing"); err == nil {
				t.Fatal("Import: error = nil, want non-nil")
			}
		})
	})
}
