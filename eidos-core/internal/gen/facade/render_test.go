// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// mini copies the testdata mini kernel into a fresh repository
// root, so a case can poison one package without touching the
// committed fixture.
func mini(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	src := filepath.Join("testdata", "mini")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	assert.NoError(t, err, "the mini kernel copies")
	return root
}

// poison writes one file into a mini kernel copy.
func poison(t *testing.T, root, rel, content string) {
	t.Helper()

	target := filepath.Join(root, "eidos-core", filepath.FromSlash(rel))
	assert.NoError(t, os.WriteFile(target, []byte(content), 0o644), "the poison writes")
}

// The renderer owns the re-export forms, so each one is pinned
// over the mini kernel: the committed fixture holds every form,
// and each way a surface can defeat re-export is injected and has
// to refuse at its position.
func TestRender(t *testing.T) {
	t.Parallel()

	t.Run("re-exports every declaration form", func(t *testing.T) {
		t.Parallel()

		set, err := facade.Generate(mini(t))
		assert.NoError(t, err, "the mini kernel generates")

		root := string(set["eidos-sdk/facade.gen.go"])
		assert.Contains(t, root, "type Rule = core.Rule",
			"a type re-exports as an alias")
		assert.Contains(t, root, "const Weight = core.Weight",
			"a constant re-declares against the kernel's")
		assert.Contains(t, root, "var Default = core.Default",
			"a variable re-declares against the kernel's")
		assert.Contains(t, root,
			"func On(k symbol.Kind, hs ...func(r *Rule) error) (Rule, error) {",
			"a function wraps under its own signature")
		assert.Contains(t, root, "return core.On(k, hs...)",
			"forwarding the variadic tail")
		assert.Contains(t, root, `core "go.dokimi.dev/eidos/core"`,
			"the kernel counterpart imports under the fixed alias")
		assert.Contains(t, root, `"go.dokimi.dev/eidos/sdk/symbol"`,
			"a qualified kernel reference respells to the facade sibling")
		assert.Contains(t, root, "[go.dokimi.dev/eidos/sdk/symbol.Kind]",
			"carried documentation respells kernel paths")

		sym := string(set["eidos-sdk/symbol/facade.gen.go"])
		assert.Contains(t, sym, "KindStruct = core.KindStruct",
			"an iota block re-declares name by name")
		assert.Contains(t, sym, "Generic[T any] = core.Generic[T]",
			"a parameterized type re-exports as a generic alias")
		assert.Contains(t, sym, "// sdk/symbol imports core/symbol.",
			"the package documentation states the computed dependency position")

		kit := string(set["eidos-sdk/plugintest/facade.gen.go"])
		assert.Contains(t, kit, `"example.test/dep"`,
			"an external import carries verbatim")
		assert.Contains(t, kit, "func Check(d dep.T) {",
			"and its qualifier stays the source's")
	})

	t.Run("refuses an unexported type in an exported signature", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		poison(t, root, "emit/poison.go",
			"package emit\n\n// Fetch returns a row.\nfunc Fetch() row { return row{} }\n\ntype row struct{}\n")
		_, err := facade.Generate(root)
		assert.HasError(t, err, "an unexported reference cannot re-export")
		assert.Contains(t, err.Error(), "unexported row", "the refusal names the type")
		assert.Contains(t, err.Error(), "poison.go", "at its position")
	})

	t.Run("refuses a kernel package outside the curated list", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		poison(t, root, "emit/poison.go",
			"package emit\n\nimport \"go.dokimi.dev/eidos/core/workspace\"\n\n"+
				"// Run runs w.\nfunc Run(w workspace.Workspace) {}\n")
		_, err := facade.Generate(root)
		assert.HasError(t, err, "a non-curated kernel reference is a surface leak")
		assert.Contains(t, err.Error(), "leaks", "the refusal says what happened")
		assert.Contains(t, err.Error(), "workspace", "and names the package")
	})

	t.Run("refuses a form syntax cannot re-export", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		poison(t, root, "emit/poison.go",
			"package emit\n\n// Shape takes an anonymous struct.\nfunc Shape(x struct{ N int }) {}\n")
		_, err := facade.Generate(root)
		assert.HasError(t, err, "an anonymous struct would re-declare, not re-export")
		assert.Contains(t, err.Error(), "unsupported", "the refusal says why")
	})

	t.Run("refuses a surface re-exporting nothing", func(t *testing.T) {
		t.Parallel()

		root := mini(t)
		poison(t, root, "plugin/plugin.go", "package plugin\n\ntype marker struct{}\n")
		_, err := facade.Generate(root)
		assert.HasError(t, err, "an empty facade package proves the list wrong")
		assert.Contains(t, err.Error(), "re-exports nothing", "the refusal says why")
	})
}
