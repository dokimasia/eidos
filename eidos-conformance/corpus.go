// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"context"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Verdict says what one language does with an inventory feature:
// the read-side kin of the render coverage's verdicts.
type Verdict uint8

const (
	// Loads means the language spells the feature and the corpus
	// tree carries the spelling; the expectations must hold.
	Loads Verdict = iota + 1

	// Refuses means the language cannot spell the feature, stated:
	// the corpus tree must carry nothing under it, because a
	// spelling present under a refusal is a contradiction.
	Refuses
)

// Coverage maps every inventory feature onto a verdict. Totality
// is checked at [Run]: a feature without a verdict fails, and a
// verdict naming no feature fails, so a language cannot go silent
// on a capability the way the render side's coverage cannot.
type Coverage map[string]Verdict

// Corpus is one language's conformance entry.
type Corpus struct {
	// Frontend loads the tree.
	Frontend plugin.Frontend

	// Sources is the language's corpus tree, features under f/<id>.
	Sources fs.FS

	// Coverage is the language's verdict per inventory feature.
	Coverage Coverage

	// Signatures are the roots the frontend suite loads
	// signature-only, empty when the tree states none.
	Signatures []string

	// Schemas are the directive schemas the tree's carriers write.
	Schemas []directive.Schema

	// Keys registers the classification keys the tree's stamps
	// write.
	Keys func(*meta.Registry) error

	// PackageOf derives the package path a feature's declarations
	// load under; nil derives the corpus convention, f/<id>. A
	// language whose canonical paths differ states its own.
	PackageOf func(featureID string) string
}

// featureRoot is where the corpus convention places features, as a
// directory and as the default package prefix.
const featureRoot = "f/"

// pkg derives one feature package path.
func (c Corpus) pkg(featureID string) string {
	if c.PackageOf != nil {
		return c.PackageOf(featureID)
	}
	return featureRoot + featureID
}

// Run holds one language's corpus to the shared inventory:
// coverage totality, the frontend conformance suite over the whole
// tree, every covered feature's expectations against one load, and
// every refused feature's absence.
func Run(t *testing.T, c Corpus) {
	t.Helper()

	t.Run("covers the whole inventory", func(t *testing.T) {
		t.Parallel()
		AssertCoveredInventory(t, c)
	})
	t.Run("meets the frontend conformance bar", func(t *testing.T) {
		t.Parallel()
		frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
			return c.Frontend, &frontendtest.Fixture{
				Sources:    c.Sources,
				Signatures: c.Signatures,
				Schemas:    c.Schemas,
				Keys:       c.Keys,
			}
		})
	})

	g := corpusGraph(t, c)
	for _, f := range Inventory() {
		switch c.Coverage[f.ID] {
		case Loads:
			t.Run("loads "+f.ID, func(t *testing.T) {
				t.Parallel()
				AssertFeature(t, c, g, f)
			})
		case Refuses:
			t.Run("refuses "+f.ID, func(t *testing.T) {
				t.Parallel()
				AssertRefusedFeature(t, c, f)
			})
		}
	}
}

// AssertCoveredInventory forces totality: every inventory feature
// carries a verdict, every verdict names an inventory feature, and
// every verdict is one of the declared two.
func AssertCoveredInventory(tb assert.TB, c Corpus) {
	tb.Helper()

	known := map[string]bool{}
	for _, f := range Inventory() {
		known[f.ID] = true
		switch c.Coverage[f.ID] {
		case Loads, Refuses:
		default:
			tb.Errorf("the inventory holds %s and the coverage says nothing: "+
				"silence on a capability is the gap this list exists to close", f.ID)
		}
	}
	ids := make([]string, 0, len(c.Coverage))
	for id := range c.Coverage {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if !known[id] {
			tb.Errorf("the coverage names %s, which the inventory does not hold", id)
		}
	}
}

// AssertFeature holds one covered feature to its expectations over
// the corpus graph.
func AssertFeature(tb assert.TB, c Corpus, g *store.Graph, f Feature) {
	tb.Helper()

	lang := c.Frontend.Lang()
	ctx := func(decl symbol.Symbol) *Ctx {
		return &Ctx{
			Lang: lang, Decl: decl, Graph: g,
			Pkg: func(sub string) string {
				if sub == "" {
					return c.pkg(f.ID)
				}
				return c.pkg(f.ID) + "/" + sub
			},
		}
	}
	for _, d := range f.Declares {
		id := symbol.Identity{
			Lang: lang, Package: ctx(nil).Pkg(d.Sub),
			Owner: d.Owner, Name: d.Name, Kind: d.Kind, Disc: d.Disc,
		}
		decl, held := g.Lookup(id)
		if !held {
			tb.Errorf("%s expects %s, which the load does not hold", f.ID, id)
			continue
		}
		if d.Check != nil {
			d.Check(tb, ctx(decl))
		}
	}
	if f.Check != nil {
		f.Check(tb, ctx(nil))
	}
}

// AssertRefusedFeature holds one refusal honest: the tree carries
// nothing under the feature's directory.
func AssertRefusedFeature(tb assert.TB, c Corpus, f Feature) {
	tb.Helper()

	dir := featureRoot + f.ID
	err := fs.WalkDir(c.Sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasPrefix(path, dir+"/") || path == dir) {
			tb.Errorf("%s is refused and %s spells it anyway: a refusal covers nothing",
				f.ID, path)
		}
		return nil
	})
	assert.NoError(tb, err, "the corpus tree walks")
}

// corpusGraph loads the whole tree once, full depth, for the
// feature expectations; the frontend suite drives its own loads.
func corpusGraph(tb assert.TB, c Corpus) *store.Graph {
	tb.Helper()

	sink := diag.NewSink()
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        c.Sources,
		Frontends: []plugin.Frontend{c.Frontend},
		Sink:      sink,
		PluginSet: []byte("conformance"),
	})
	assert.NoError(tb, err, "the corpus loads")
	for d := range sink.All() {
		if d.Severity == diag.SeverityError {
			tb.Errorf("the corpus load reported %v: a tree the language "+
				"refuses proves nothing about its expectations", d)
		}
	}
	return g
}
