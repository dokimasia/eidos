// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/plugin/shape/tools"
)

// The fixture catalog, the files that a run over it writes, and their
// golden files.
const (
	fixtureRoot  = "testdata/catalog"
	registryFile = "registry.gen.go"
	wiringFile   = "catalog/registry_wiring.gen.go"
	goldenDir    = "testdata"
	goldenSuffix = ".golden"
)

// The paths of the specs of the fault fixtures, and the sections of a
// shape spec after its name. A fixture writes the name, the sections and
// its own sections after them.
const (
	writerPath   = "spec/shapes/writer.yaml"
	readerPath   = "spec/shapes/reader.yaml"
	declaredPath = "spec/shapes/declared.yaml"
	aliasPath    = "spec/mixins/writer.yaml"
	idPath       = "spec/mixins/id.yaml"
	iDPath       = "spec/mixins/i-d.yaml"
	shapeHead    = "form: shape\nclaim: a claim\nobservation: an observation\n" +
		"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\n"
	mixinHead = "form: mixin\nclaim: a claim\nobservation: an observation\n" +
		"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\n"
)

// The generator writes the registry of package shape and the wiring of
// package catalog, and writes nothing for specs with a fault.
func TestGenerator(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the registry into package shape and the wiring into package catalog", func(t *testing.T) {
			t.Parallel()

			files, findings := generated(t, os.DirFS(fixtureRoot))
			assert.Empty(t, findings, "the fixture catalog is valid")
			assert.Permutation(t, slices.Collect(maps.Keys(files)), []string{registryFile, wiringFile},
				"the run writes the two files")
		})

		t.Run("writes the registry and the wiring of the fixture catalog", func(t *testing.T) {
			t.Parallel()

			files, _ := generated(t, os.DirFS(fixtureRoot))
			for path, body := range files {
				golden.MatchAt(
					t,
					filepath.Join(goldenDir, filepath.Base(path)+goldenSuffix),
					body,
					golden.ShouldUpdate(),
				)
			}
		})

		t.Run("writes nothing for specs with a fault", func(t *testing.T) {
			t.Parallel()

			files, findings := generated(t, fstest.MapFS{
				writerPath: {
					Data: []byte("name: writer\n" + shapeHead + "params:\n  - {key: of, type: string, doc: a value}\n"),
				},
			})
			assert.NotEmpty(t, findings, "the spec gives the identifier WriterOf twice")
			assert.Empty(t, files, "the run writes no file")
		})
	})
}

// generated runs the composition of the tools module over a tree, and
// returns the files that the run writes by their paths, and the findings
// of the run of the Error severity.
func generated(tb testing.TB, tree fs.FS) (map[string][]byte, []diag.Diag) {
	tb.Helper()

	sink := output.NewMem()
	ws, err := tools.Compose().Output(func() (output.Sink, error) { return sink, nil }).Build()
	assert.NoError(tb, err, "the composition builds")
	report, _ := ws.Run(tb.Context(), workspace.Input{Tree: tree})
	assert.NotNil(tb, report, "the run returns a report")
	var findings []diag.Diag
	for d := range report.Sink.All() {
		if d.Severity == diag.SeverityError {
			findings = append(findings, d)
		}
	}
	return sink.Files(), findings
}

// expectFindings checks the codes and the messages of the findings of a
// run, in order.
func expectFindings(tb testing.TB, findings []diag.Diag, codes []diag.Code, msgs []string) {
	tb.Helper()

	gotCodes := make([]diag.Code, 0, len(findings))
	gotMsgs := make([]string, 0, len(findings))
	for _, d := range findings {
		gotCodes = append(gotCodes, d.Code)
		gotMsgs = append(gotMsgs, d.Msg)
	}
	expect.Equal(tb, gotCodes, codes, "the run reports the codes of the faults in order")
	expect.Equal(tb, gotMsgs, msgs, "the run reports the messages of the faults in order")
}
