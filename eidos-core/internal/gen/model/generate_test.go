// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// schemaGoMod is the go.mod of a written schema's module.
// schemaMarker opens every written schema with the marker type, which
// a field uses to admit any kind.
const (
	schemaGoMod  = "module example.test/schema\n\ngo 1.27.0\n"
	schemaMarker = "package schema\n\ntype " + model.MarkerName + " any\n\n"
)

// The fingerprint's pins: the file that declares it, and the
// declaration its value follows.
const (
	fingerprintFile = "node/fingerprint.gen.go"
	fingerprintDecl = "ModelFingerprint = \""
)

// The enum fixture: a schema that imports the module's symbol
// package, the symbol sources a case varies, and the file names they
// arrive under.
const (
	// enumSchema imports the symbol package through a field typed by
	// its anchor, which every symbol source declares.
	enumSchema = "package schema\n\nimport \"example.test/schema/symbol\"\n\n" +
		"type " + model.MarkerName + " any\n\n" +
		"// Thing has an anchor.\ntype Thing struct {\n" +
		"\tAnchor symbol.Anchor `eidos:\"both\"`\n}\n"
	// factSchema is enumSchema with an emit field stating a fact.
	factSchema = "package schema\n\nimport \"example.test/schema/symbol\"\n\n" +
		"type " + model.MarkerName + " any\n\n" +
		"// Thing has an anchor.\ntype Thing struct {\n" +
		"\tAnchor symbol.Anchor `eidos:\"both\"`\n" +
		"\tAsync  bool          `eidos:\"emit,fact=Async\"`\n}\n"
	// symbolHead opens every symbol source with the anchor type,
	// which declares no constant and so no enum.
	symbolHead = "package symbol\n\n// Anchor is what the schema imports the package for.\ntype Anchor uint8\n\n"
	// toneSource declares a two-constant enum.
	toneSource = symbolHead + "// Tone is a pitch.\ntype Tone uint8\n\n" +
		"const (\n\tToneLow Tone = iota\n\tToneHigh\n)\n"
	// toneMidSource inserts a constant into toneSource's list.
	toneMidSource = symbolHead + "// Tone is a pitch.\ntype Tone uint8\n\n" +
		"const (\n\tToneLow Tone = iota\n\tToneMid\n\tToneHigh\n)\n"
	// toneDocSource is toneSource with other documentation.
	toneDocSource = symbolHead + "// Tone names a pitch.\ntype Tone uint8\n\n" +
		"const (\n\tToneLow Tone = iota\n\tToneHigh\n)\n"
	// toneFiveSource is toneSource with the high constant at five.
	toneFiveSource = symbolHead + "// Tone is a pitch.\ntype Tone uint8\n\n" +
		"const (\n\tToneLow Tone = 0\n\tToneHigh Tone = 5\n)\n"
	// pitchSource is toneSource with the type renamed.
	pitchSource = symbolHead + "// Pitch is a pitch.\ntype Pitch uint8\n\n" +
		"const (\n\tToneLow Pitch = iota\n\tToneHigh\n)\n"
	// wideSource and narrowSource declare a string-typed constant,
	// which is no enum, at two values.
	wideSource   = toneSource + "\n// Width is a span.\ntype Width string\n\nconst WidthOne Width = \"wide\"\n"
	narrowSource = toneSource + "\n// Width is a span.\ntype Width string\n\nconst WidthOne Width = \"narrow\"\n"
	// foreignSource is toneSource with an integer constant of a type
	// another package declares.
	foreignSource = "package symbol\n\nimport \"time\"\n\n" +
		"// Anchor is what the schema imports the package for.\ntype Anchor uint8\n\n" +
		"// Tone is a pitch.\ntype Tone uint8\n\nconst (\n\tToneLow Tone = iota\n\tToneHigh\n)\n\n" +
		"// Tick is a period.\nconst Tick time.Duration = 5\n"
	// modeOneSource and modeTwoSource are a generated file's enum at
	// two values.
	modeOneSource = "package symbol\n\n// Mode is generated.\ntype Mode uint8\n\nconst ModeOne Mode = 1\n"
	modeTwoSource = "package symbol\n\n// Mode is generated.\ntype Mode uint8\n\nconst ModeOne Mode = 2\n"
	// symbolFile and generatedSymbolFile are the names the symbol
	// sources arrive under.
	symbolFile          = "symbol.go"
	generatedSymbolFile = "mode" + genfile.GeneratedSuffix
)

// The ceilings of a generation, each measured over 24 fresh processes
// after one generation, and allowing eight standard deviations above
// the mean.
const (
	// generateAllocs is one generation over the kernel's own schema, the
	// lowering, every template's execution and every file's formatting:
	// 2,384,435 on average with a standard deviation of 28.
	generateAllocs = 2_384_435 + 8*28
	// generateOneKindAllocs is one generation over a schema of one walked
	// kind: the same templates over a smaller model, 56,447 on average
	// with a standard deviation of 5.
	generateOneKindAllocs = 56_447 + 8*5
	// regenerateOneKindAllocs is one regeneration over the same schema:
	// the generation and the write of each file, 56,635 on average with
	// a standard deviation of 4.
	regenerateOneKindAllocs = 56_635 + 8*4
)

// symbolModule is one module a fingerprint case generates from: the
// schema, the symbol package's hand-written source, and a generated
// symbol file, empty for none.
type symbolModule struct {
	schema    string
	symbol    string
	generated string
}

// write writes the module into a fresh root and returns the root.
func (m symbolModule) write(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	symbols := filepath.Join(root, model.SymbolPackage)
	schema := filepath.Join(root, filepath.FromSlash(model.SchemaDir))
	assert.NoError(t, os.MkdirAll(schema, 0o750), "the schema directory is created")
	files := map[string]string{
		filepath.Join(root, "go.mod"):      schemaGoMod,
		filepath.Join(symbols, symbolFile): m.symbol,
		filepath.Join(schema, "schema.go"): m.schema,
	}
	if m.generated != "" {
		files[filepath.Join(symbols, generatedSymbolFile)] = m.generated
	}
	for path, content := range files {
		assert.NoError(t, os.WriteFile(path, []byte(content), 0o600), "the module file writes: "+path)
	}
	return root
}

// The generator's output set, its bytes across runs, its fingerprint
// and its refusals are what every mirror guard and unit key reads,
// so each is contract.
func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders one file per output", func(t *testing.T) {
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
				"node/codec.gen.go",
				"node/codec.gen_test.go",
				fingerprintFile,
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
			assert.Equal(t, slices.Sorted(maps.Keys(set)), want,
				"every output renders, and nothing else")
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

		t.Run("matches the committed tree", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			set, err := model.Generate(root)
			assert.NoError(t, err, "the schema generates")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the committed tree matches the schema; run `make generate` and commit")
		})

		fingerprints := []struct {
			name          string
			before, after symbolModule
			changed       bool
		}{
			{
				name:    "changes the fingerprint when a constant enters an enum mid-list",
				before:  symbolModule{schema: enumSchema, symbol: toneSource},
				after:   symbolModule{schema: enumSchema, symbol: toneMidSource},
				changed: true,
			},
			{
				name:    "changes the fingerprint when an enum constant's value changes",
				before:  symbolModule{schema: enumSchema, symbol: toneSource},
				after:   symbolModule{schema: enumSchema, symbol: toneFiveSource},
				changed: true,
			},
			{
				name:    "changes the fingerprint when an enum's type is renamed",
				before:  symbolModule{schema: enumSchema, symbol: toneSource},
				after:   symbolModule{schema: enumSchema, symbol: pitchSource},
				changed: true,
			},
			{
				name:    "changes the fingerprint when the schema states a fact",
				before:  symbolModule{schema: enumSchema, symbol: toneSource},
				after:   symbolModule{schema: factSchema, symbol: toneSource},
				changed: true,
			},
			{
				name:   "keeps the fingerprint across an enum's documentation edit",
				before: symbolModule{schema: enumSchema, symbol: toneSource},
				after:  symbolModule{schema: enumSchema, symbol: toneDocSource},
			},
			{
				name:   "keeps the fingerprint across a string constant's value",
				before: symbolModule{schema: enumSchema, symbol: wideSource},
				after:  symbolModule{schema: enumSchema, symbol: narrowSource},
			},
			{
				name: "keeps the fingerprint across a generated symbol file's constants",
				before: symbolModule{
					schema: enumSchema, symbol: toneSource, generated: modeOneSource,
				},
				after: symbolModule{
					schema: enumSchema, symbol: toneSource, generated: modeTwoSource,
				},
			},
		}
		for _, tt := range fingerprints {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				before, after := fingerprintFor(t, tt.before), fingerprintFor(t, tt.after)
				assert.Equal(t, before != after, tt.changed,
					"a recorded graph is served exactly while its encoding keeps its meaning")
			})
		}

		t.Run("returns an error for a module without a schema", func(t *testing.T) {
			t.Parallel()

			_, err := model.Generate(t.TempDir())
			assert.HasError(t, err, "a module without a schema generates nothing")
		})

		t.Run("returns an error for a schema declaring no kind", func(t *testing.T) {
			t.Parallel()

			_, err := model.Generate(schemaModule(t, ""))
			assert.HasError(t, err, "a schema declaring nothing but the marker generates nothing")
			assert.Contains(t, err.Error(), "symbols.gen_test.go",
				"naming the file that could not render against it")
			assert.HasPrefix(t, err.Error(), "model: ", "under the package prefix")
		})

		t.Run("returns an error for a fact stated on a shape the rendering does not model", func(t *testing.T) {
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
				"and the bytes on disk are what the generator produces")
		})

		t.Run("writes models that a second run reproduces", func(t *testing.T) {
			t.Parallel()

			root := symbolModule{schema: enumSchema, symbol: toneSource}.write(t)
			assert.NoError(t, model.Regenerate(root), "the first run writes the models")
			set, err := model.Generate(root)
			assert.NoError(t, err, "a second run over the written models generates")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the Kind and Fact constants the first run wrote leave the fingerprint as it was")
		})

		t.Run("writes nothing when one file fails to render", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, "// Thing counts.\ntype Thing struct {\n"+
				"\tCount int `eidos:\"emit,fact=Counted\"`\n}\n")
			assert.HasError(t, model.Regenerate(root),
				"a file that cannot render refuses the run")
			assert.Empty(t, generatedFiles(t, root),
				"and no file was written: a refused render leaves no half-generated tree")
		})
	})
}

// A generation and a regeneration of a schema of one kind allocate
// within their ceilings in the ordinary run, which runs no benchmark.
// The kernel's own generation takes too long to repeat 101 times, so
// only [BenchmarkGenerate] checks its ceiling. The check runs alone,
// because AllocsPerRun counts every goroutine's allocations and refuses
// to run beside parallel tests.
func TestGenerateAllocs(t *testing.T) {
	root := schemaModule(t, reachSchema)
	assert.MaxAllocs(t, func() {
		if _, err := model.Generate(root); err != nil {
			t.Fatalf("Generate: unexpected error: %v", err)
		}
	}, generateOneKindAllocs, "Generate allocates the lowering, the rendering and the formatting")
	dir := filepath.Join(root, model.SchemaDir)
	assert.MaxAllocs(t, func() {
		if err := model.Regenerate(dir); err != nil {
			t.Fatalf("Regenerate: unexpected error: %v", err)
		}
	}, regenerateOneKindAllocs, "Regenerate allocates the generation and the writes")
}

// BenchmarkGenerate measures one whole generation over the kernel's
// own schema and over a schema of one kind, and a regeneration of the
// one kind: the lowering, every template's execution and every file's
// formatting, which the go:generate wrapper and each mirror guard run.
// Each case runs once before the measurement.
func BenchmarkGenerate(b *testing.B) {
	generations := []struct {
		name   string
		root   string
		allocs uint64
	}{
		{name: "the kernel's own schema", root: moduleRoot(b), allocs: generateAllocs},
		{name: "a schema of one kind", root: schemaModule(b, reachSchema), allocs: generateOneKindAllocs},
	}
	b.Run("Generate", func(b *testing.B) {
		for _, tt := range generations {
			b.Run(tt.name, func(b *testing.B) {
				_, err := model.Generate(tt.root)
				assert.NoError(b, err, "the schema generates before the measurement")
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var set genfile.Set
				for c.Loop() {
					set, err = model.Generate(tt.root)
				}
				assert.NoError(b, err, "the schema generates")
				assert.NotEmpty(b, set, "every output")
			})
		}
	})

	b.Run("Regenerate", func(b *testing.B) {
		b.Run("a schema of one kind", func(b *testing.B) {
			dir := filepath.Join(schemaModule(b, reachSchema), model.SchemaDir)
			err := model.Regenerate(dir)
			assert.NoError(b, err, "the schema regenerates before the measurement")
			c := bench.Start(b).MaxAllocs(regenerateOneKindAllocs)
			defer c.End()
			for c.Loop() {
				err = model.Regenerate(dir)
			}
			assert.NoError(b, err, "the schema regenerates")
		})
	})
}

// moduleRoot returns the kernel's module root, which every case
// here generates against.
func moduleRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return root
}

// schemaModule writes one schema into a fresh module root and
// returns the root, so a case can generate from a schema form the
// kernel's own schema does not contain.
func schemaModule(tb testing.TB, schema string) string {
	tb.Helper()

	root := tb.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(model.SchemaDir))
	assert.NoError(tb, os.MkdirAll(dir, 0o750), "the schema directory is created")
	assert.NoError(tb,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte(schemaGoMod), 0o600),
		"the module's go.mod writes")
	assert.NoError(tb,
		os.WriteFile(filepath.Join(dir, "schema.go"), []byte(schemaMarker+schema), 0o600),
		"the schema writes")
	return root
}

// fingerprintIn returns the value of the fingerprint a generated set
// declares.
func fingerprintIn(tb assert.TB, set genfile.Set) string {
	tb.Helper()

	_, rest, found := strings.Cut(string(set[fingerprintFile]), fingerprintDecl)
	assert.True(tb, found, "the fingerprint file declares the constant")
	value, _, _ := strings.Cut(rest, "\"")
	return value
}

// fingerprintFor generates a module and returns its fingerprint.
func fingerprintFor(t *testing.T, m symbolModule) string {
	t.Helper()

	set, err := model.Generate(m.write(t))
	assert.NoError(t, err, "the module generates")
	return fingerprintIn(t, set)
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
