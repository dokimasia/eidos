// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is what a frontend brings to the suite: the tree the
// selection claims from, and what the checks that need more can
// read. A field left empty skips the checks that need it, and the
// suite states each skip.
type Fixture struct {
	// Sources is the workspace tree the suite loads.
	Sources fs.FS

	// Signatures are the directory roots loaded signature-only,
	// for the depth check.
	Signatures []string

	// Schemas are the directive schemas the fixture's carriers
	// write, for validation at the suite's stand-in freeze.
	Schemas []directive.Schema

	// Keys registers the classification keys the fixture's stamps
	// write, for the stand-in apply.
	Keys func(*meta.Registry) error
}

// Setup builds the frontend under test and its fixture, fresh per
// check.
type Setup func(tb assert.TB) (plugin.Frontend, *Fixture)

// RunFrontendSuite holds a frontend to the read side's contract:
// deterministic parses, positioned findings, no silently dropped
// file, the workspace's own outputs refused, honest unit keys, the
// jailed read, signature depth, validated directive attachments
// and resolved references. It
// drives the SPI, so a kit-built frontend and a hand-rolled one
// meet the same checks.
func RunFrontendSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("parses deterministically", func(t *testing.T) {
		t.Parallel()
		AssertDeterministicParse(t, setup)
	})
	t.Run("positions every finding", func(t *testing.T) {
		t.Parallel()
		AssertPositionedDiagnostics(t, setup)
	})
	t.Run("classifies without dropping a file", func(t *testing.T) {
		t.Parallel()
		AssertClassified(t, setup)
	})
	t.Run("refuses its own outputs", func(t *testing.T) {
		t.Parallel()
		AssertOwnedExcluded(t, setup)
	})
	t.Run("folds honest unit keys", func(t *testing.T) {
		t.Parallel()
		AssertFingerprinted(t, setup)
	})
	t.Run("reads through the jail alone", func(t *testing.T) {
		t.Parallel()
		AssertJailedReads(t, setup)
	})
	t.Run("loads signature roots shallow", func(t *testing.T) {
		t.Parallel()
		if _, fx := setup(t); len(fx.Signatures) == 0 {
			t.Skip("the fixture states no signature roots")
		}
		AssertSignatureDepth(t, setup)
	})
	t.Run("attaches validated directives", func(t *testing.T) {
		t.Parallel()
		if _, fx := setup(t); len(fx.Schemas) == 0 {
			t.Skip("the fixture states no directive schemas")
		}
		AssertAttachedDirectives(t, setup)
	})
	t.Run("resolves references", func(t *testing.T) {
		t.Parallel()
		AssertLinked(t, setup)
	})
}

// loaded is one drive of the pipeline over a fixture.
type loaded struct {
	graph  *store.Graph
	report *load.Report
	sink   *diag.Sink
}

// drive loads the fixture through the real pipeline, failing the
// test on a fatal load error.
func drive(
	tb assert.TB, f plugin.Frontend, fx *Fixture, mutate ...func(*load.Config),
) *loaded {
	tb.Helper()

	got, err := tryDrive(f, fx, mutate...)
	assert.NoError(tb, err, "the fixture loads")
	return got
}

// tryDrive loads the fixture and returns the load's own error, for
// the checks that perturb inputs a language may refuse.
func tryDrive(
	f plugin.Frontend, fx *Fixture, mutate ...func(*load.Config),
) (*loaded, error) {
	sink := diag.NewSink()
	cfg := load.Config{
		FS:         fx.Sources,
		Frontends:  []plugin.Frontend{f},
		Sink:       sink,
		PluginSet:  []byte("frontendtest"),
		Signatures: fx.Signatures,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	g, report, err := load.Load(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return &loaded{graph: g, report: report, sink: sink}, nil
}

// selected returns the fixture files the frontend's claim matches,
// in tree order.
func selected(tb assert.TB, f plugin.Frontend, fx *Fixture) []string {
	tb.Helper()

	var out []string
	err := fs.WalkDir(fx.Sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && load.Match(f.Selection(), path) {
			out = append(out, path)
		}
		return nil
	})
	assert.NoError(tb, err, "the fixture tree walks")
	assert.NotEmpty(tb, out, "the selection claims something to check")
	return out
}

// encoded renders the graph as bytes, packages in the store's own
// order, so two loads compare whole.
func encoded(tb assert.TB, g *store.Graph) []byte {
	tb.Helper()

	var buf bytes.Buffer
	for pkg := range g.ByKind(symbol.KindPackage) {
		b, err := json.Marshal(pkg)
		assert.NoError(tb, err, "a package encodes")
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// copyTree materializes a fixture tree so one file can be
// perturbed without touching the original.
func copyTree(tb assert.TB, fsys fs.FS) fstest.MapFS {
	tb.Helper()

	out := fstest.MapFS{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return readErr
		}
		out[path] = &fstest.MapFile{Data: b}
		return nil
	})
	assert.NoError(tb, err, "the fixture tree copies")
	return out
}

// keysOf maps each unit's first member onto its key.
func keysOf(report *load.Report) map[string][]byte {
	out := make(map[string][]byte, len(report.Units))
	for _, u := range report.Units {
		out[u.Files[0]] = u.Key
	}
	return out
}
