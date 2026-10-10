// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package tools_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/golden"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/plugin/shape/tools"
	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The catalog module, relative to the tools module's directory, and the
// files that a run over it writes, relative to the catalog module.
const (
	catalogRoot  = ".."
	registryFile = "registry.gen.go"
	wiringFile   = "catalog/registry_wiring.gen.go"
	schemaFile   = "spec.schema.json"
)

// The composition generates the catalog's registry, and the catalog
// module contains what it generates. Run with -update to write the
// generated files into the catalog module.
func TestCompose(t *testing.T) {
	t.Parallel()

	t.Run("Compose", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the registry and the wiring", func(t *testing.T) {
			t.Parallel()

			files := generated(t)
			assert.Permutation(t, slices.Collect(maps.Keys(files)), []string{registryFile, wiringFile},
				"the run writes the registry of package shape and the wiring of package catalog")
		})

		t.Run("generates the registry that the catalog module contains", func(t *testing.T) {
			t.Parallel()

			for path, body := range generated(t) {
				golden.MatchAt(t, filepath.Join(catalogRoot, filepath.FromSlash(path)), body, golden.ShouldUpdate())
			}
		})
	})

	t.Run("Schema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the schema that the catalog module publishes", func(t *testing.T) {
			t.Parallel()

			golden.MatchAt(t, filepath.Join(catalogRoot, schemaFile), specfront.Schema(), golden.ShouldUpdate())
		})
	})
}

// generated runs the composition over the catalog module into memory, and
// returns the files that the run writes, keyed by their path in the
// module. A run that reports an Error fails the test with each Error.
func generated(tb testing.TB) map[string][]byte {
	tb.Helper()

	sink := output.NewMem()
	ws, err := tools.Compose().Output(func() (output.Sink, error) { return sink, nil }).Build()
	assert.NoError(tb, err, "the composition builds")
	report, err := ws.Run(tb.Context(), workspace.Input{Tree: os.DirFS(catalogRoot)})
	assert.NotNil(tb, report, "the run returns a report")
	for d := range report.Sink.All() {
		expect.NotEqual(tb, d.Severity, diag.SeverityError,
			"the run reports no Error, and reports "+d.Code.String()+" at "+d.Pos.String()+": "+d.Msg)
	}
	assert.NoError(tb, err, "the run over the catalog's specs succeeds")
	return sink.Files()
}
