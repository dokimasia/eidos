// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/frontend/load"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The corpus tree and the root of this repository, relative to the
// package directory the tests run in.
const (
	corpusTree     = "testdata/corpus"
	repositoryRoot = "../../.."
)

// Go passes its own run against the shared inventory.
func TestCorpus(t *testing.T) {
	t.Parallel()

	t.Run("Corpus", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance run over the Go tree", func(t *testing.T) {
			t.Parallel()

			conformance.Run(t, golang.Corpus(os.DirFS(corpusTree)))
		})
	})
}

// BenchmarkCorpus loads this repository through the entry's frontend,
// over the machine's module cache and standard library: every workspace
// unit at full depth, then the dependency rounds at signature depth. It
// reports the rounds, the workspace units and the dependency units
// beside the time and the allocations. It pins no allocation ceiling,
// because its input is the repository and the toolchain the machine has
// installed, and both change without a change to the load.
func BenchmarkCorpus(b *testing.B) {
	stores, err := gofrontend.Stores(os.Getenv)
	if err != nil {
		b.Fatalf("the machine's module cache and standard library resolve: %v", err)
	}
	c := golang.Corpus(os.DirFS(repositoryRoot))

	b.Run("Corpus/the load of this repository", func(b *testing.B) {
		var report *load.Report
		b.ReportAllocs()
		for b.Loop() {
			sink := diag.NewSink()
			_, loaded, loadErr := load.Load(context.Background(), load.Config{
				FS:        c.Sources,
				Frontends: []plugin.Frontend{c.Frontend},
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
