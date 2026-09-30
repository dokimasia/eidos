// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Verdict states what one language does with an inventory feature:
// the read-side counterpart of the render coverage's verdicts.
type Verdict uint8

const (
	// Loads means the language spells the feature and the corpus
	// tree contains the spelling, and the expectations must pass.
	// It is the verdict of a corpus that registers no rules, whose
	// projections nothing can evaluate.
	Loads Verdict = iota + 1

	// Refuses means the language states that it cannot spell the
	// feature. The corpus tree must contain nothing under it,
	// because a spelling present under a refusal is a
	// contradiction.
	Refuses

	// Projects means the feature loads and projects whole: every
	// reference reachable from its declarations folds to a form
	// other than Opaque or Inline, every callable projects, every
	// type's member set is complete, and the corpus names no
	// remainder.
	Projects

	// ProjectsPartly means at least one reference folds to Opaque
	// or Inline or one member set has a gap, and every remainder
	// key the corpus names for the feature is present on the
	// declaration the remainder names.
	ProjectsPartly

	// Opaque means the reference the feature declares itself folds
	// to Opaque or Inline, an alias of an inline body for instance,
	// and every remainder key the corpus names is present.
	Opaque
)

// String returns the verdict's spelling.
func (v Verdict) String() string {
	switch v {
	case Loads:
		return "loads"
	case Refuses:
		return "refuses"
	case Projects:
		return "projects"
	case ProjectsPartly:
		return "projects partly"
	case Opaque:
		return "opaque"
	default:
		return strconv.Itoa(int(v))
	}
}

// projected reports whether a verdict is one of the three
// projection levels, which a corpus states only with rules.
func (v Verdict) projected() bool {
	return v == Projects || v == ProjectsPartly || v == Opaque
}

// Remainder names one key a feature's residue stamps on one of its
// declarations: what a language keeps in metadata where the
// projection cannot represent it.
type Remainder struct {
	// Decl locates the declaration with the remainder, the way a
	// feature's Declares does.
	Decl Decl
	// Key is the metadata key the remainder stamps under.
	Key meta.KeyName
}

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

	// Dropped lists the identities a signature-only load of the
	// Signatures roots leaves out, which the frontend suite's depth
	// check requires in the full graph and absent from the shallow
	// one. A language whose signature depth keeps every declaration
	// lists none.
	Dropped []symbol.Identity

	// Schemas are the directive schemas the tree's carriers write.
	Schemas []directive.Schema

	// Keys registers the classification keys the tree's stamps
	// write.
	Keys func(*meta.Registry) error

	// Rules is the language's projection rules, nil for a corpus
	// that returns none. With rules, every feature's verdict is a
	// projection level or a refusal, the rules run through the
	// rules suite over the tree, and each level is evaluated over
	// the feature's declarations.
	Rules rules.SourceRules

	// Remainder names, per feature, the keys the language stamps
	// where the projection cannot represent the feature whole. A
	// level below Projects requires every named key present, and
	// Projects admits none.
	Remainder map[string][]Remainder

	// PackageOf derives the package path a feature's declarations
	// load under, sub naming a feature's sibling package and empty
	// naming its own. Nil derives the corpus convention, f/<id> and
	// f/<id>/<sub>. A language whose canonical paths differ, such as
	// a dotted namespace or a module prefix, states its own, and the
	// join is its own too, because a slash is not every language's
	// separator.
	PackageOf func(featureID, sub string) string
}

// featureRoot is where the corpus convention places features, as a
// directory and as the default package prefix.
const featureRoot = "f/"

// pkg derives one feature package path, sub naming a sibling
// package and empty naming the feature's own.
func (c Corpus) pkg(featureID, sub string) string {
	if c.PackageOf != nil {
		return c.PackageOf(featureID, sub)
	}
	if sub == "" {
		return featureRoot + featureID
	}
	return featureRoot + featureID + "/" + sub
}

// Run checks one language's corpus against the shared inventory:
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
				Dropped:    c.Dropped,
				Schemas:    c.Schemas,
				Keys:       c.Keys,
			}
		})
	})

	if c.Rules != nil {
		t.Run("meets the rules contract", func(t *testing.T) {
			t.Parallel()
			rulestest.RunRulesSuite(t, func(tb assert.TB) (rules.SourceRules, *rulestest.Fixture) {
				return c.Rules, corpusFixture(tb, c)
			})
		})
	}

	fx := corpusFixture(t, c)
	for _, f := range Inventory() {
		verdict := c.Coverage[f.ID]
		switch {
		case verdict == Loads:
			t.Run("loads "+f.ID, func(t *testing.T) {
				t.Parallel()
				AssertFeature(t, c, fx.Graph, f)
			})
		case verdict == Refuses:
			t.Run("refuses "+f.ID, func(t *testing.T) {
				t.Parallel()
				AssertRefusedFeature(t, c, fx.Graph, f)
			})
		case verdict.projected():
			t.Run(verdict.String()+" "+f.ID, func(t *testing.T) {
				t.Parallel()
				AssertFeature(t, c, fx.Graph, f)
				AssertLevel(t, c, fx, f, verdict)
			})
		}
	}
}

// AssertCoveredInventory forces totality: every inventory feature
// has a verdict, every verdict names an inventory feature, and every
// verdict is one the corpus may state. A refusal is always one, the
// load verdict is one without rules, and a projection level is one
// with them.
func AssertCoveredInventory(tb assert.TB, c Corpus) {
	tb.Helper()

	known := map[string]bool{}
	for _, f := range Inventory() {
		known[f.ID] = true
		verdict := c.Coverage[f.ID]
		switch {
		case verdict == Refuses:
		case verdict == Loads && c.Rules == nil:
		case verdict.projected() && c.Rules != nil:
		case verdict == Loads:
			tb.Errorf("%s loads under a corpus with rules: a corpus that projects states "+
				"the level, projects, projects partly or opaque", f.ID)
		case verdict.projected():
			tb.Errorf("%s states a projection level under a corpus without rules, "+
				"which nothing can evaluate", f.ID)
		default:
			tb.Errorf("the inventory lists %s and the coverage states no verdict for it: "+
				"silence on a capability is the gap this list exists to close", f.ID)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Remainder)) {
		if !known[id] {
			tb.Errorf("the remainder names %s, which the inventory does not list", id)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Coverage)) {
		if !known[id] {
			tb.Errorf("the coverage names %s, which the inventory does not list", id)
		}
	}
}

// AssertFeature checks one covered feature against its expectations
// over the corpus graph.
func AssertFeature(tb assert.TB, c Corpus, g *store.Graph, f Feature) {
	tb.Helper()

	lang := c.Frontend.Lang()
	ctx := func(decl symbol.Symbol) *Ctx {
		return &Ctx{
			Lang: lang, Decl: decl, Graph: g,
			Pkg: func(sub string) string { return c.pkg(f.ID, sub) },
		}
	}
	for _, d := range f.Declares {
		id := identityOf(c, f, d)
		decl, held := g.Lookup(id)
		if !held {
			tb.Errorf("%s expects %s, which the load does not contain", f.ID, id)
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

// AssertRefusedFeature checks one refusal two ways: the tree
// contains nothing under the feature's directory, and the load
// contains no package the feature would occupy. The second check
// catches a spelling filed somewhere else, such as a shared file or
// a rehomed tree the directory check never walks, because a
// refusal's proof is what the graph contains, not where the bytes
// were.
func AssertRefusedFeature(tb assert.TB, c Corpus, g *store.Graph, f Feature) {
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

	occupied := map[string]bool{c.pkg(f.ID, ""): true}
	for _, d := range f.Declares {
		occupied[c.pkg(f.ID, d.Sub)] = true
	}
	for pkg := range g.Packages() {
		if occupied[pkg.ID.Package] {
			tb.Errorf("%s is refused and the load contains %s anyway: a refusal "+
				"covers nothing, wherever the spelling is", f.ID, pkg.ID.Package)
		}
	}
}

// corpusFixture loads the whole tree once, full depth, into the
// rules suite's fixture: the sealed graph and the load's stamps
// under the kernel's and the language's keys. The feature
// expectations and the levels read it, and the frontend suite drives
// its own loads.
func corpusFixture(tb assert.TB, c Corpus) *rulestest.Fixture {
	tb.Helper()

	keys := c.Keys
	if keys == nil {
		keys = func(*meta.Registry) error { return nil }
	}
	return rulestest.Loaded(tb, c.Frontend, c.Sources, keys)
}
