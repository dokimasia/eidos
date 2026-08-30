// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource_test

import (
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

func TestModule(t *testing.T) {
	t.Parallel()

	t.Run("ModuleRoot", func(t *testing.T) {
		t.Parallel()

		t.Run("finds the nearest enclosing module", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot("testdata/mod/lib")
			if err != nil {
				t.Fatalf("ModuleRoot: unexpected error: %v", err)
			}
			want, err := filepath.Abs("testdata/mod")
			if err != nil {
				t.Fatalf("Abs: %v", err)
			}
			if got != want {
				t.Fatalf("ModuleRoot = %q, want %q", got, want)
			}
		})

		t.Run("answers an absolute path", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot("testdata/mod")
			if err != nil {
				t.Fatalf("ModuleRoot: unexpected error: %v", err)
			}
			if !filepath.IsAbs(got) {
				t.Fatalf("ModuleRoot = %q, want an absolute path", got)
			}
		})

		t.Run("reports a directory outside any module", func(t *testing.T) {
			t.Parallel()

			if _, err := gosource.ModuleRoot("/"); err == nil {
				t.Fatal("ModuleRoot(/): error = nil, want non-nil")
			}
		})
	})

	t.Run("ModulePath", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the module directive", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModulePath("testdata/mod")
			if err != nil {
				t.Fatalf("ModulePath: unexpected error: %v", err)
			}
			if want := "example.test/fixture"; got != want {
				t.Fatalf("ModulePath = %q, want %q", got, want)
			}
		})

		t.Run("reports a directory holding no go.mod", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ModulePath("testdata/empty")
			if err == nil {
				t.Fatal("ModulePath: error = nil, want non-nil")
			}
			if !strings.HasPrefix(err.Error(), "gosource: ") {
				t.Fatalf("error = %q, want the package prefix", err)
			}
		})
	})
}
