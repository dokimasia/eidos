// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package genfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

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
			assert.NoError(t, err, "well-formed source formats")
			assert.Contains(t, string(got), "type T struct{ A int }",
				"and comes back in canonical layout")
		})

		t.Run("names the file it could not format", func(t *testing.T) {
			t.Parallel()

			_, err := genfile.Format("broken.gen.go", []byte("package p\nfunc ("))
			assert.HasError(t, err, "source that does not parse is refused")
			assert.Contains(t, err.Error(), "broken.gen.go", "naming the file")
			assert.HasPrefix(t, err.Error(), "genfile: ", "under the package prefix")
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
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			for path := range set {
				_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
				assert.NoError(t, err, "every file arrives under the root")
			}
		})

		t.Run("refuses a path escaping the root", func(t *testing.T) {
			t.Parallel()

			err := genfile.Write(t.TempDir(), genfile.Set{"../escape.gen.go": []byte(wellFormed)})
			assert.HasError(t, err, "a path escaping the root is refused")
		})

		t.Run("reports a directory it cannot create", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			blocker := filepath.Join(root, "node")
			assert.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0o600),
				"the blocking file writes")
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.HasError(t, genfile.Write(root, set),
				"a directory it cannot create is reported")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a set path escaping the root", func(t *testing.T) {
			t.Parallel()

			set := genfile.Set{"../escape.gen.go": []byte(wellFormed)}
			assert.HasError(t, genfile.Verify(t.TempDir(), set, nil),
				"a set path escaping the root is refused")
		})

		t.Run("refuses an owned directory escaping the root", func(t *testing.T) {
			t.Parallel()

			err := genfile.Verify(t.TempDir(), genfile.Set{}, []string{"../elsewhere"})
			assert.HasError(t, err, "an owned directory escaping the root is refused")
		})

		t.Run("passes over a directory the generator has not created", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, genfile.Verify(t.TempDir(), genfile.Set{}, []string{"node"}),
				"a directory the generator has not created passes")
		})

		t.Run("passes when the tree matches", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			assert.NoError(t, genfile.Verify(root, set, []string{"node"}),
				"a tree matching its set verifies")
		})

		t.Run("reports a file whose bytes differ", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			edited := filepath.Join(root, "node", "kinds.gen.go")
			assert.NoError(t, os.WriteFile(edited, []byte("package p\n"), 0o600),
				"the edit arrives")
			err := genfile.Verify(root, set, []string{"node"})
			assert.HasError(t, err, "a file whose bytes differ fails the mirror")
			assert.Contains(t, err.Error(), "kinds.gen.go", "and is named")
		})

		t.Run("reports a missing file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.HasError(t, genfile.Verify(root, set, []string{"node"}),
				"a file the set holds and the tree lacks fails the mirror")
		})

		t.Run("reports a stray generated file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			stray := filepath.Join(root, "node", "orphan.gen.go")
			assert.NoError(t, os.WriteFile(stray, []byte(wellFormed), 0o600),
				"the stray arrives")
			err := genfile.Verify(root, set, []string{"node"})
			assert.HasError(t, err, "a generated file the set does not name is a stray")
			assert.Contains(t, err.Error(), "orphan.gen.go", "and is named")
		})

		t.Run("reports a stray generated test", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			stray := filepath.Join(root, "node", "orphan.gen_test.go")
			assert.NoError(t, os.WriteFile(stray, []byte(wellFormed), 0o600),
				"the stray arrives")
			err := genfile.Verify(root, set, []string{"node"})
			assert.HasError(t, err, "a generated test the set does not name is a stray too")
			assert.Contains(t, err.Error(), "orphan.gen_test.go", "and is named")
		})

		t.Run("ignores a hand-written file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			set := genfile.Set{"node/kinds.gen.go": []byte(wellFormed)}
			assert.NoError(t, genfile.Write(root, set), "the set writes")
			hand := filepath.Join(root, "node", "resolver.go")
			assert.NoError(t, os.WriteFile(hand, []byte(wellFormed), 0o600),
				"the hand-written file arrives")
			assert.NoError(t, genfile.Verify(root, set, []string{"node"}),
				"a hand-written file is no stray: the guard owns generated names alone")
		})
	})
}
