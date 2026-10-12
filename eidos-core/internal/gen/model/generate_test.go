// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"cmp"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// schemaGoMod is the go.mod of the module of a written schema.
// schemaMarker opens every written schema with the marker type. A field
// of the marker type admits any kind.
const (
	schemaGoMod  = "module example.test/schema\n\ngo 1.27.0\n"
	schemaMarker = "package schema\n\ntype " + model.MarkerName + " any\n\n"
)

// fingerprintFile is the file that declares the fingerprint.
// fingerprintDecl is the declaration that precedes its value.
const (
	fingerprintFile = "node/fingerprint.gen.go"
	fingerprintDecl = "ModelFingerprint = \""
)

// The enum fixture has a schema that imports the symbol package of the
// module, the symbol sources that the cases vary, and the names of
// their files.
const (
	// enumSchema imports the symbol package through a field of the type
	// Anchor. Every symbol source declares Anchor.
	enumSchema = "package schema\n\nimport \"example.test/schema/symbol\"\n\n" +
		"type " + model.MarkerName + " any\n\n" +
		"// Thing has an anchor.\ntype Thing struct {\n" +
		"\tAnchor symbol.Anchor `eidos:\"both\"`\n}\n"
	// factSchema is enumSchema with an emit field that has a fact tag.
	factSchema = "package schema\n\nimport \"example.test/schema/symbol\"\n\n" +
		"type " + model.MarkerName + " any\n\n" +
		"// Thing has an anchor.\ntype Thing struct {\n" +
		"\tAnchor symbol.Anchor `eidos:\"both\"`\n" +
		"\tAsync  bool          `eidos:\"emit,fact=Async\"`\n}\n"
	// symbolHead opens every symbol source with the type Anchor. Anchor
	// has no constants, so it is no enum.
	symbolHead = "package symbol\n\n// Anchor is what the schema imports the package for.\ntype Anchor uint8\n\n"
	// toneSource declares a two-constant enum.
	toneSource = symbolHead + "// Tone is a pitch.\ntype Tone uint8\n\n" +
		"const (\n\tToneLow Tone = iota\n\tToneHigh\n)\n"
	// toneMidSource is toneSource with a constant in the middle of the
	// list.
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
	// wideSource and narrowSource declare a string constant with two
	// different values. A string constant is no enum.
	wideSource   = toneSource + "\n// Width is a span.\ntype Width string\n\nconst WidthOne Width = \"wide\"\n"
	narrowSource = toneSource + "\n// Width is a span.\ntype Width string\n\nconst WidthOne Width = \"narrow\"\n"
	// foreignSource is toneSource with an integer constant of a type
	// that another package declares.
	foreignSource = "package symbol\n\nimport \"time\"\n\n" +
		"// Anchor is what the schema imports the package for.\ntype Anchor uint8\n\n" +
		"// Tone is a pitch.\ntype Tone uint8\n\nconst (\n\tToneLow Tone = iota\n\tToneHigh\n)\n\n" +
		"// Tick is a period.\nconst Tick time.Duration = 5\n"
	// modeOneSource and modeTwoSource are the enum of a generated file
	// with two different values.
	modeOneSource = "package symbol\n\n// Mode is generated.\ntype Mode uint8\n\nconst ModeOne Mode = 1\n"
	modeTwoSource = "package symbol\n\n// Mode is generated.\ntype Mode uint8\n\nconst ModeOne Mode = 2\n"
	// symbolFile and generatedSymbolFile are the file names of the
	// symbol sources.
	symbolFile          = "symbol.go"
	generatedSymbolFile = "mode" + genfile.GeneratedSuffix
)

// The allocation budgets of a generation. Each budget is a round number
// about 5% above one measurement with Go 1.27.2.
const (
	// generateAllocs is one generation over the kernel's own schema. It
	// runs the lowering, the execution of every template and the
	// formatting of every file, and it measured 2,383,183 allocations.
	generateAllocs = 2_500_000
	// generateOneKindAllocs is one generation over a schema of one walked
	// kind. It runs the same templates over a smaller model, and it
	// measured 57,967 allocations.
	generateOneKindAllocs = 61_000
	// regenerateOneKindAllocs is one regeneration over the same schema.
	// It adds the write of each file, and it measured 58,142 allocations.
	regenerateOneKindAllocs = 61_000
)

// symbolModule is a module that a fingerprint case generates. It has
// the schema, the hand-written source of the symbol package, and a
// generated symbol file. An empty generated file means none.
type symbolModule struct {
	schema    string
	symbol    string
	generated string
}

// write writes the module into a fresh root and returns the root.
func (m symbolModule) write(t *testing.T) string {
	t.Helper()

	tree := files.Tree{
		"go.mod": files.Text(schemaGoMod),
		path.Join(model.SymbolPackage, symbolFile): files.Text(m.symbol),
		path.Join(model.SchemaDir, "schema.go"):    files.Text(m.schema),
	}
	if m.generated != "" {
		tree[path.Join(model.SymbolPackage, generatedSymbolFile)] = files.Text(m.generated)
	}
	return files.Workspace(t, tree)
}

// Every mirror guard and unit key reads the output set of the
// generator, its bytes across runs, its fingerprint and its errors.
// Each of them is part of the contract.
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
			assert.Permutation(t, slices.Collect(maps.Keys(set)), want,
				"every output renders, and nothing else")
		})

		t.Run("produces the same bytes on every run", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(
				t,
				model.Generate,
				moduleRoot(t),
				"every run produces the same files with the same bytes",
			)
		})

		t.Run("matches the committed tree", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			set, err := model.Generate(root)
			assert.NoError(t, err, "the schema generates")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the committed tree matches the schema. Run `make generate` and commit the result")
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
				if tt.changed {
					assert.NotEqual(t, after, before, "an encoding whose meaning changed gets a new fingerprint")
					return
				}
				assert.Equal(t, after, before, "an encoding that keeps its meaning keeps its fingerprint")
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
			assert.That(t, err.Error()).
				Contains("symbols.gen_test.go", "the error names the file that could not render").
				HasPrefix("model: ", "the error starts with the package prefix")
		})

		t.Run("returns an error for a fact stated on a shape the rendering does not model", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, "// Thing counts.\ntype Thing struct {\n"+
				"\tCount int `eidos:\"emit,fact=Counted\"`\n}\n")
			_, err := model.Generate(root)
			assert.HasError(t, err,
				"a fact whose statedness has no expression fails the generation")
			assert.That(t, err.Error()).
				Contains("facts.gen.go", "the error names the file of the facts traversal").
				HasPrefix("genfile: ", "the error starts with the prefix of the step that refused the fact")
		})
	})

	t.Run("Regenerate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the models into the module of its directory", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, reachSchema)
			assert.NoError(t, model.Regenerate(filepath.Join(root, model.SchemaDir)),
				"a directory anywhere inside the module regenerates")

			set, err := model.Generate(root)
			assert.NoError(t, err, "the same schema generates in memory")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the bytes on disk are the bytes that the generator produces")
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
			var err error
			files.Unchanged(t, os.DirFS(root), func() { err = model.Regenerate(root) },
				"a refused render writes no file")
			assert.HasError(t, err, "a file that cannot render fails the run")
		})
	})
}

// TestGenerateAllocs checks the budgets of a generation and a
// regeneration of a schema of one kind in a run without benchmarks. The
// kernel's own generation takes too long to repeat 101 times, so only
// [BenchmarkGenerate] checks its budget. Each count keeps the first
// error of its calls with cmp.Or, and cmp.Or does not allocate. The
// check does not run in parallel, because the count includes the
// allocations of every goroutine.
func TestGenerateAllocs(t *testing.T) {
	root := schemaModule(t, reachSchema)
	var (
		set genfile.Set
		err error
	)
	assert.MaxAllocs(t, func() {
		var gerr error
		set, gerr = model.Generate(root)
		err = cmp.Or(err, gerr)
	}, generateOneKindAllocs, "Generate allocates within its budget")
	assert.NoError(t, err, "the schema generates")
	assert.NotEmpty(t, set, "the set contains every output")
	dir := filepath.Join(root, model.SchemaDir)
	assert.MaxAllocs(t, func() { err = cmp.Or(err, model.Regenerate(dir)) }, regenerateOneKindAllocs,
		"Regenerate allocates within its budget")
	assert.NoError(t, err, "the schema regenerates")
}

// BenchmarkGenerate measures one generation over the kernel's own
// schema and over a schema of one kind, and one regeneration of the
// schema of one kind. The go:generate wrapper and each mirror guard run
// the same lowering, templates and formatting. Each case runs one
// iteration before the measurement.
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
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					set genfile.Set
					err error
				)
				for c.Loop() {
					set, err = model.Generate(tt.root)
				}
				assert.NoError(b, err, "the schema generates")
				assert.NotEmpty(b, set, "the set contains every output")
			})
		}
	})

	b.Run("Regenerate", func(b *testing.B) {
		b.Run("a schema of one kind", func(b *testing.B) {
			dir := filepath.Join(schemaModule(b, reachSchema), model.SchemaDir)
			c := bench.Start(b).Warmup(1).MaxAllocs(regenerateOneKindAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = model.Regenerate(dir)
			}
			assert.NoError(b, err, "the schema regenerates")
		})
	})
}

// moduleRoot returns the root of the kernel's module. Most cases
// generate from its schema.
func moduleRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return root
}

// schemaModule writes one schema into a fresh module root and returns
// the root. A case uses it to generate from a schema form that the
// kernel's own schema does not contain.
func schemaModule(tb testing.TB, schema string) string {
	tb.Helper()

	return files.Workspace(tb, files.Tree{
		"go.mod":                                files.Text(schemaGoMod),
		path.Join(model.SchemaDir, "schema.go"): files.Text(schemaMarker + schema),
	})
}

// fingerprintIn returns the value of the fingerprint that a generated
// set declares.
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
