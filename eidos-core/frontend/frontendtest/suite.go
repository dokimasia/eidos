// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Brand is the brand every suite load runs under. A fixture writes
// its carriers under Brand's marks, fixture:, +fixture: and
// -fixture:, and the ownership check stamps the workspace's own
// copies under it.
const Brand output.Brand = "fixture"

// Fixture is the input of a frontend to the suite. It has the tree that
// the selection claims from, and the inputs of the checks that need
// more. A field left empty skips the checks that need it, and the suite
// reports each skip as a skipped subtest.
type Fixture struct {
	// Sources is the workspace tree the suite loads.
	Sources fs.FS

	// Empty is a tree whose claimed files declare nothing, such as a file
	// with a package clause alone, for the empty-load check. The load of
	// it reports no Error, and each claimed file enters the graph without
	// a declaration. The load calls a frontend only for a file that its
	// selection claims, so the load of an empty tree without a claimed
	// file never calls the frontend.
	Empty fs.FS

	// Signatures are the directory roots loaded signature-only,
	// for the depth check.
	Signatures []string

	// Dropped lists the identities a signature-only load of the
	// Signatures roots leaves out, for the depth check: each is in
	// the full graph and absent from the shallow one. A language
	// whose signature depth keeps every declaration lists none.
	Dropped []symbol.Identity

	// Schemas are the directive schemas the fixture's carriers
	// write, for validation at the freeze the suite runs in the
	// workspace's place.
	Schemas []directive.Schema

	// Keys registers the classification keys of the fixture's stamps
	// that the frontend does not register through its own role, for the
	// stamp application the suite runs in the workspace's place.
	Keys func(*meta.Registry) error

	// Stores are the named trees outside the workspace from which the
	// load reads dependency units, for the dependency check.
	Stores map[string]fs.FS

	// Reexported lists the identities of the declarations that the
	// references of the fixture resolve to only through a re-export, for
	// the re-export check. A reference in the graph targets each.
	Reexported []symbol.Identity
}

// Setup builds the frontend under test and its fixture, fresh per
// check.
type Setup func(tb assert.TB) (plugin.Frontend, *Fixture)

// RunFrontendSuite checks a frontend against the read side's contract:
// deterministic parses, positioned findings, no silently dropped
// file, the workspace's own outputs refused, honest unit keys, the
// jailed read, signature depth, validated directive attachments,
// resolved references, a tree whose files declare nothing, and for a
// frontend in the dependent or the exporter role, its dependency units
// and its re-exports. It drives the SPI, so a kit-built frontend and a
// hand-rolled one meet the same checks.
//
// One survey load at the start finds the facts that the fixture does not
// contain: the classification keys, a unit that loads full beside its
// signature roots, and a second package. The suite reports each skip that
// follows from them as a skipped subtest of its own.
func RunFrontendSuite(t *testing.T, setup Setup) {
	t.Helper()

	f, fx := setup(t)
	survey := drive(t, f, fx)

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
	if !stampsAny(survey.graph) {
		t.Run("applies classification stamps", func(t *testing.T) {
			t.Skip("the fixture's load stamps nothing")
		})
	}
	t.Run("refuses its own outputs", func(t *testing.T) {
		t.Parallel()
		AssertOwnedExcluded(t, setup)
	})
	t.Run("folds honest unit keys", func(t *testing.T) {
		t.Parallel()
		AssertFingerprinted(t, setup)
	})
	if fullUnit(survey.report) == "" {
		t.Run("keys one unit at two depths", func(t *testing.T) {
			t.Skip("every unit of the fixture loads signature-only")
		})
	}
	t.Run("reads through the jail alone", func(t *testing.T) {
		t.Parallel()
		AssertJailedReads(t, setup)
	})
	t.Run("loads signature roots shallow", func(t *testing.T) {
		t.Parallel()
		if _, fx := setup(t); len(fx.Signatures) == 0 {
			t.Skip("the fixture has no signature roots")
		}
		AssertSignatureDepth(t, setup)
	})
	t.Run("attaches validated directives", func(t *testing.T) {
		t.Parallel()
		if _, fx := setup(t); len(fx.Schemas) == 0 {
			t.Skip("the fixture has no directive schemas")
		}
		AssertAttachedDirectives(t, setup)
	})
	t.Run("resolves references", func(t *testing.T) {
		t.Parallel()
		AssertLinked(t, setup)
	})
	if packagesOf(survey.graph) < 2 {
		t.Run("resolves references across packages", func(t *testing.T) {
			t.Skip("the fixture declares one package")
		})
	}
	t.Run("loads a tree that declares nothing", func(t *testing.T) {
		t.Parallel()
		if _, fx := setup(t); fx.Empty == nil {
			t.Skip("the fixture has no tree whose files declare nothing")
		}
		AssertEmptyLoad(t, setup)
	})
	t.Run("loads dependencies through the jail", func(t *testing.T) {
		t.Parallel()
		f, fx := setup(t)
		if _, dependent := f.(plugin.Dependent); !dependent {
			t.Skip("the frontend is not in the dependent role")
		}
		if len(fx.Stores) == 0 {
			t.Skip("the fixture has no stores")
		}
		AssertDependencies(t, setup)
	})
	t.Run("follows re-exports", func(t *testing.T) {
		t.Parallel()
		f, fx := setup(t)
		if _, exports := f.(plugin.Exporter); !exports {
			t.Skip("the frontend is not in the exporter role")
		}
		if len(fx.Reexported) == 0 {
			t.Skip("the fixture lists no declaration a re-export publishes")
		}
		AssertReexports(t, setup)
	})
}

// stampsAny reports whether a load recorded a classification stamp.
func stampsAny(g *store.Graph) bool {
	for range g.Stamps() {
		return true
	}
	return false
}

// packagesOf counts the packages a graph declares.
func packagesOf(g *store.Graph) int {
	n := 0
	for range g.ByKind(symbol.KindPackage) {
		n++
	}
	return n
}

// loaded is one drive of the pipeline over a fixture.
type loaded struct {
	graph  *store.Graph
	report *load.Report
	sink   *diag.Sink
}

// drive loads the fixture through the real pipeline, under the context
// of tb, failing the test on a fatal load error.
func drive(
	tb assert.TB, f plugin.Frontend, fx *Fixture, mutate ...func(*load.Config),
) *loaded {
	tb.Helper()

	got, err := tryDrive(assert.Context(tb), f, fx, mutate...)
	assert.NoError(tb, err, "the fixture loads")
	return got
}

// tryDrive loads the fixture under [Brand] and ctx, and returns the
// load's own error, for the checks that perturb inputs a language may
// refuse.
func tryDrive(
	ctx context.Context, f plugin.Frontend, fx *Fixture, mutate ...func(*load.Config),
) (*loaded, error) {
	sink := diag.NewSink()
	cfg := load.Config{
		FS:         fx.Sources,
		Frontends:  []plugin.Frontend{f},
		Sink:       sink,
		Signatures: fx.Signatures,
		Brand:      Brand,
		Stores:     fx.Stores,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	g, report, err := load.Load(ctx, cfg)
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
		out[u.Files[0].Path] = u.Key
	}
	return out
}
