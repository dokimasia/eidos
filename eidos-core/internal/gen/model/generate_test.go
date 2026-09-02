// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// The module a written schema arrives in, and the marker every
// such schema declares so a field can hold any kind.
const (
	schemaGoMod  = "module example.test/schema\n\ngo 1.27.0\n"
	schemaMarker = "package schema\n\ntype " + model.MarkerName + " any\n\n"
)

// moduleRoot returns the kernel's module root, which every case
// here generates against.
func moduleRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return root
}

// schemaModule writes one schema into a fresh module root and
// returns the root, so a case can generate from a schema shape the
// kernel's own does not hold.
func schemaModule(t *testing.T, schema string) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(model.SchemaDir))
	assert.NoError(t, os.MkdirAll(dir, 0o750), "the schema directory is created")
	assert.NoError(t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte(schemaGoMod), 0o600),
		"the module's go.mod writes")
	assert.NoError(t,
		os.WriteFile(filepath.Join(dir, "schema.go"), []byte(schemaMarker+schema), 0o600),
		"the schema writes")
	return root
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders every owned file", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			assert.NoError(t, err, "the schema generates")
			want := []string{
				"emit/facts.gen.go",
				"emit/facts.gen_test.go",
				"emit/kinds.gen.go",
				"emit/kinds.gen_test.go",
				"emit/names.gen.go",
				"emit/names.gen_test.go",
				"emit/slots.gen.go",
				"emit/slots.gen_test.go",
				"emit/symbols.gen.go",
				"emit/symbols.gen_test.go",
				"emit/walk.gen.go",
				"emit/walk.gen_test.go",
				"match.gen.go",
				"match.gen_test.go",
				"node/fingerprint.gen.go",
				"node/fingerprint.gen_test.go",
				"node/kinds.gen.go",
				"node/kinds.gen_test.go",
				"node/symbols.gen.go",
				"node/symbols.gen_test.go",
				"node/walk.gen.go",
				"node/walk.gen_test.go",
				"symbol/fact.gen.go",
				"symbol/fact.gen_test.go",
				"symbol/kind.gen.go",
				"symbol/kind.gen_test.go",
			}
			assert.Equal(t, slices.Sorted(maps(set)), want,
				"every owned file renders, and nothing else")
		})

		t.Run("carries the schema's documentation into the models", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			assert.NoError(t, err, "the schema generates")
			kinds := string(set["node/kinds.gen.go"])
			for _, want := range []string{
				"// Struct is a type values can be made of",
				"// This is the node spelling of the kind.",
			} {
				assert.Contains(t, kinds, want,
					"the schema's documentation is carried into the models")
			}
		})

		t.Run("produces the same bytes twice", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			first, err := model.Generate(root)
			assert.NoError(t, err, "the first run generates")
			second, err := model.Generate(root)
			assert.NoError(t, err, "and the second")
			for path, want := range first {
				assert.Equal(t, string(second[path]), string(want),
					"two runs produce the same bytes")
			}
		})

		t.Run("reports a module holding no schema", func(t *testing.T) {
			t.Parallel()

			_, err := model.Generate(t.TempDir())
			assert.HasError(t, err, "a module holding no schema is reported")
		})

		t.Run("matches the committed tree", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			set, err := model.Generate(root)
			assert.NoError(t, err, "the schema generates")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the committed tree matches the schema; run `make generate` and commit")
		})

		t.Run("reports a schema declaring no kind", func(t *testing.T) {
			t.Parallel()

			_, err := model.Generate(schemaModule(t, ""))
			assert.HasError(t, err, "a schema declaring nothing but the marker is reported")
			assert.Contains(t, err.Error(), "symbols.gen_test.go",
				"naming the file that could not render against it")
			assert.HasPrefix(t, err.Error(), "model: ", "under the package prefix")
		})

		t.Run("reports a fact stated on a shape rendering does not model", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, "// Thing counts.\ntype Thing struct {\n"+
				"\tCount int `eidos:\"emit,fact=Counted\"`\n}\n")
			_, err := model.Generate(root)
			assert.HasError(t, err,
				"a fact whose statedness has no expression fails the generation")
			assert.Contains(t, err.Error(), "facts.gen.go",
				"naming the file whose traversal would otherwise have walked wrong")
			assert.HasPrefix(t, err.Error(), "genfile: ",
				"under the prefix of the step that refused it")
		})
	})

	t.Run("Regenerate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the models into the module it ran inside", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, reachSchema)
			assert.NoError(t, model.Regenerate(filepath.Join(root, model.SchemaDir)),
				"a directory anywhere inside the module regenerates")

			set, err := model.Generate(root)
			assert.NoError(t, err, "the same schema generates in memory")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"and what landed on disk is what the generator produces")
		})

		t.Run("writes nothing when one file refuses", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, "// Thing counts.\ntype Thing struct {\n"+
				"\tCount int `eidos:\"emit,fact=Counted\"`\n}\n")
			assert.HasError(t, model.Regenerate(root),
				"a file that cannot render refuses the run")
			assert.Empty(t, generatedFiles(t, root),
				"and no file landed: a refused render leaves no half-generated tree")
		})
	})
}

// generatedFiles lists every generated file under root, which is
// what a refused run has to leave empty.
func generatedFiles(t *testing.T, root string) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		case strings.HasSuffix(d.Name(), genfile.GeneratedSuffix),
			strings.HasSuffix(d.Name(), genfile.GeneratedTestSuffix):
			found = append(found, path)
		}
		return nil
	})
	assert.NoError(t, err, "the module tree walks")
	return found
}

// maps yields a set's paths, so a case can sort them.
func maps(set genfile.Set) func(func(string) bool) {
	return func(yield func(string) bool) {
		for path := range set {
			if !yield(path) {
				return
			}
		}
	}
}
