// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource_test

import (
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("ParseDir", func(t *testing.T) {
		t.Parallel()

		t.Run("skips generated and test files", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/lib")
			if err != nil {
				t.Fatalf("ParseDir: unexpected error: %v", err)
			}
			var got []string
			for _, f := range files {
				got = append(got, filepath.Base(fset.Position(f.Pos()).Filename))
			}
			if want := "lib.go"; strings.Join(got, ",") != want {
				t.Fatalf("files = %v, want only %s", got, want)
			}
		})

		t.Run("reads files in name order", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/app")
			if err != nil {
				t.Fatalf("ParseDir: unexpected error: %v", err)
			}
			var got []string
			for _, f := range files {
				got = append(got, filepath.Base(fset.Position(f.Pos()).Filename))
			}
			want := "app.go,uses_stdlib.go"
			if strings.Join(got, ",") != want {
				t.Fatalf("files = %v, want %s", got, want)
			}
		})

		t.Run("reports a directory holding no hand-written file", func(t *testing.T) {
			t.Parallel()

			if _, err := gosource.ParseDir(token.NewFileSet(), "testdata/empty"); err == nil {
				t.Fatal("ParseDir: error = nil, want non-nil")
			}
		})
	})

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("type-checks a package against its module-local imports", func(t *testing.T) {
			t.Parallel()

			modRoot, err := filepath.Abs("testdata/mod")
			if err != nil {
				t.Fatalf("Abs: %v", err)
			}
			pkg, files, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", modRoot,
			)
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			if len(files) != 2 {
				t.Fatalf("parsed %d files, want 2", len(files))
			}
			if obj := pkg.Scope().Lookup("Holder"); obj == nil {
				t.Fatal("Holder is missing from the type-checked package")
			}
			if got := pkg.Scope().Lookup("Upper"); got == nil {
				t.Fatal("Upper is missing: the stdlib import did not resolve")
			}
		})

		t.Run("does not resolve generated declarations", func(t *testing.T) {
			t.Parallel()

			modRoot, err := filepath.Abs("testdata/mod")
			if err != nil {
				t.Fatalf("Abs: %v", err)
			}
			pkg, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/lib", "example.test/fixture/lib", modRoot,
			)
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			if pkg.Scope().Lookup("Generated") != nil {
				t.Fatal("Generated resolved: the loader read a .gen.go file")
			}
		})

		t.Run("reports a package that does not type-check", func(t *testing.T) {
			t.Parallel()

			_, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", "",
			)
			if err == nil {
				t.Fatal("Load without a module root: error = nil, want non-nil")
			}
		})
	})
}
