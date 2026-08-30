// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package genfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/genfile"
)

const wellFormed = "package p\n\ntype T struct{ A int }\n"

func TestGenfile(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes layout", func(t *testing.T) {
			t.Parallel()

			got, err := genfile.Format("x.gen.go", []byte("package p\ntype T struct{A int}\n"))
			if err != nil {
				t.Fatalf("Format: unexpected error: %v", err)
			}
			if !strings.Contains(string(got), "type T struct{ A int }") {
				t.Fatalf("Format did not canonicalize: %q", got)
			}
		})

		t.Run("names the file it could not format", func(t *testing.T) {
			t.Parallel()

			_, err := genfile.Format("broken.gen.go", []byte("package p\nfunc ("))
			if err == nil {
				t.Fatal("Format: error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), "broken.gen.go") {
				t.Fatalf("error = %q, want it to name the file", err)
			}
			if !strings.HasPrefix(err.Error(), "genfile: ") {
				t.Fatalf("error = %q, want the package prefix", err)
			}
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("creates every file under the root", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{
				"node/kinds.gen.go": []byte(wellFormed),
				"emit/kinds.gen.go": []byte(wellFormed),
			}
			if err := genfile.Write(root, set); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			for path := range set {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
					t.Fatalf("%s: %v", path, err)
				}
			}
		})

		t.Run("refuses a path escaping the root", func(t *testing.T) {
			t.Parallel()

			err := genfile.Write(t.TempDir(), genfile.Set{"../escape.gen.go": []byte(wellFormed)})
			if err == nil {
				t.Fatal("Write: error = nil, want non-nil")
			}
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("passes when the tree matches", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			if err := genfile.Write(root, set); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if err := genfile.Verify(root, set, []string{"node"}); err != nil {
				t.Fatalf("Verify: unexpected error: %v", err)
			}
		})

		t.Run("reports a file whose bytes differ", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			if err := genfile.Write(root, set); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			edited := filepath.Join(root, "node", "kinds.gen.go")
			if err := os.WriteFile(edited, []byte("package p\n"), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			err := genfile.Verify(root, set, []string{"node"})
			if err == nil {
				t.Fatal("Verify: error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), "kinds.gen.go") {
				t.Fatalf("error = %q, want it to name the file", err)
			}
		})

		t.Run("reports a missing file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			if err := genfile.Verify(root, set, []string{"node"}); err == nil {
				t.Fatal("Verify: error = nil, want non-nil")
			}
		})

		t.Run("reports a stray generated file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			if err := genfile.Write(root, set); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			stray := filepath.Join(root, "node", "orphan.gen.go")
			if err := os.WriteFile(stray, []byte(wellFormed), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			err := genfile.Verify(root, set, []string{"node"})
			if err == nil {
				t.Fatal("Verify: error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), "orphan.gen.go") {
				t.Fatalf("error = %q, want it to name the stray", err)
			}
		})

		t.Run("ignores a hand-written file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			if err := genfile.Write(root, set); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			hand := filepath.Join(root, "node", "resolver.go")
			if err := os.WriteFile(hand, []byte(wellFormed), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if err := genfile.Verify(root, set, []string{"node"}); err != nil {
				t.Fatalf("Verify: unexpected error: %v", err)
			}
		})
	})
}
