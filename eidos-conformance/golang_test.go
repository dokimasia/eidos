// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"context"
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/load"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The Go corpus's signature root and its one unexported
// declaration, which a signature-only load drops.
const (
	constantsRoot = "f/constants"
	limitName     = "limit"
)

// repositoryRoot is the root of this repository, relative to the
// module directory the benchmark runs in.
const repositoryRoot = ".."

// Go's corpus entry: the first real language against the shared
// inventory. The one refusal is Go's own semantics: overloads do
// not exist. A typed constant group loads as the enum the schema
// names it.
func TestGolang(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Corpus{
		Frontend: gofrontend.New(nil),
		Sources:  os.DirFS("testdata/go"),
		Coverage: conformance.Coverage{
			"struct_fields":       conformance.Projects,
			"struct_methods":      conformance.Projects,
			"method_overloads":    conformance.Refuses,
			"constants":           conformance.Projects,
			"cross_package_ref":   conformance.Projects,
			"composite_refs":      conformance.ProjectsPartly,
			"builtin_ref":         conformance.Projects,
			"directive_carrier":   conformance.Projects,
			"test_classification": conformance.Projects,
			"interfaces":          conformance.Projects,
			"enum_values":         conformance.Projects,
		},
		Signatures: []string{constantsRoot},
		Dropped: []symbol.Identity{
			{Lang: gofrontend.Lang, Package: constantsRoot, Name: limitName, Kind: symbol.KindConstant},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    gofrontend.Keys,
		Rules:   gorules.New(),
	})
}

// BenchmarkGolang loads this repository with the Go frontend over the
// machine's module cache and standard library: every workspace unit at
// full depth, then the dependency rounds at signature depth. It
// reports the rounds, the workspace units and the dependency units
// beside the time and the allocations. It pins no allocation ceiling,
// because its input is the repository and the toolchain the machine
// has installed, and both change without a change to the load.
func BenchmarkGolang(b *testing.B) {
	stores, err := gofrontend.Stores(os.Getenv)
	if err != nil {
		b.Fatalf("the machine's module cache and standard library resolve: %v", err)
	}
	tree := os.DirFS(repositoryRoot)
	fronts := []plugin.Frontend{gofrontend.New(nil)}

	b.Run("Load", func(b *testing.B) {
		var report *load.Report
		b.ReportAllocs()
		for b.Loop() {
			sink := diag.NewSink()
			_, loaded, loadErr := load.Load(context.Background(), load.Config{
				FS:        tree,
				Frontends: fronts,
				Sink:      sink,
				Brand:     frontendtest.Brand,
				Stores:    stores,
			})
			if loadErr != nil || sink.Failed() {
				b.Fatalf("the repository loads without an error: %v", loadErr)
			}
			report = loaded
		}
		var rounds, workspace, dependency int
		for _, u := range report.Units {
			rounds = max(rounds, u.Round)
			if u.Round == 0 {
				workspace++
			} else {
				dependency++
			}
		}
		if dependency == 0 {
			b.Fatal("the dependency rounds return units")
		}
		b.ReportMetric(float64(rounds), "rounds")
		b.ReportMetric(float64(workspace), "workspace-units")
		b.ReportMetric(float64(dependency), "dependency-units")
	})
}
