// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Verdict is what one language does with a feature of the inventory. It
// is the read side of the verdicts of the render coverage.
type Verdict uint8

const (
	// Loads means that the language spells the feature, the corpus tree
	// contains the spelling, and the expectations pass. It is the verdict
	// of a corpus that registers no rules, because no projection of such a
	// corpus can be evaluated.
	Loads Verdict = iota + 1

	// Refuses means the language states that it cannot spell the
	// feature. The corpus tree must contain nothing under it,
	// because a spelling present under a refusal is a
	// contradiction.
	Refuses

	// Projects means that the feature loads and projects whole. Every
	// reference of its declarations folds to a form other than Opaque or
	// Inline, every callable projects, the member set of every type is
	// complete, and the corpus lists no remainder.
	Projects

	// ProjectsPartly means that at least one reference folds to Opaque or
	// Inline or one member set has a gap. Every remainder key that the
	// corpus lists for the feature is present on the declaration of the
	// remainder.
	ProjectsPartly

	// Opaque means that the reference that the feature declares folds to
	// Opaque or Inline, such as an alias of an inline body. Every
	// remainder key that the corpus lists is present.
	Opaque
)

// String returns the spelling of the verdict.
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

// projected reports whether a verdict is one of the three projection
// levels. A corpus states a projection level only with rules.
func (v Verdict) projected() bool {
	return v == Projects || v == ProjectsPartly || v == Opaque
}

// Remainder is one key that a language stamps on a declaration of a
// feature where the projection cannot represent the feature. The
// language keeps that part of the feature in metadata.
type Remainder struct {
	// Decl locates the declaration with the remainder, as the Declares of
	// a feature locates a declaration.
	Decl Decl
	// Key is the metadata key of the remainder.
	Key meta.KeyName
}

// Coverage maps every feature of the inventory to a verdict. [Run] checks
// that the map is total: a feature without a verdict fails, and a verdict
// of a name outside the inventory fails. A language therefore states a
// verdict for every capability, as the coverage of the render side does.
type Coverage map[string]Verdict

// Corpus is the conformance entry of one language.
type Corpus struct {
	// Frontend loads the tree.
	Frontend plugin.Frontend

	// Sources is the corpus tree of the language, with each feature under
	// f/<id>.
	Sources fs.FS

	// Coverage is the verdict of the language for each feature of the
	// inventory.
	Coverage Coverage

	// Signatures are the roots that the frontend suite loads
	// signature-only, and empty where the tree has none.
	Signatures []string

	// Dropped lists the identities that a signature-only load of the
	// Signatures roots leaves out. The depth check of the frontend suite
	// requires each in the full graph and absent from the shallow one. A
	// language whose signature depth keeps every declaration lists none.
	Dropped []symbol.Identity

	// Schemas are the directive schemas of the carriers of the tree.
	Schemas []directive.Schema

	// Keys registers the classification keys of the stamps of the tree.
	// A nil Keys registers none.
	Keys func(*meta.Registry) error

	// Rules is the projection rules of the language, and nil for a corpus
	// without rules. With rules, the verdict of every feature is a
	// projection level or a refusal, the rules suite runs the rules over
	// the tree, and each level is evaluated over the declarations of the
	// feature.
	Rules rules.SourceRules

	// Remainder lists, for each feature, the keys that the language
	// stamps where the projection cannot represent the feature whole. A
	// level below Projects requires every listed key, and Projects admits
	// none.
	Remainder map[string][]Remainder

	// PackageOf returns the path of the package that the declarations of a
	// feature load under. sub is the name of a sibling package of the
	// feature, and empty for the package of the feature itself. A nil
	// PackageOf follows the corpus convention, f/<id> and f/<id>/<sub>. A
	// language whose canonical paths differ, such as a dotted namespace or
	// a module prefix, states its own function, which joins the elements
	// with the separator of the language.
	PackageOf func(featureID, sub string) string
}

// featureRoot is the directory of the features under the corpus
// convention, and the default prefix of their packages.
const featureRoot = "f/"

// pkg returns the path of one package of a feature. sub is the name of a
// sibling package, and empty for the package of the feature itself.
func (c Corpus) pkg(featureID, sub string) string {
	if c.PackageOf != nil {
		return c.PackageOf(featureID, sub)
	}
	if sub == "" {
		return featureRoot + featureID
	}
	return featureRoot + featureID + "/" + sub
}

// Run checks the corpus of one language against the shared inventory.
// It checks that the coverage is total, runs the frontend conformance
// suite over the whole tree, checks the expectations of every covered
// feature against one load, and checks that every refused feature is
// absent.
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

// AssertCoveredInventory checks that the coverage is total. Every feature
// of the inventory has a verdict, every verdict is of a feature of the
// inventory, and every verdict is one that the corpus may state. A corpus
// may always state a refusal. It may state the load verdict without rules,
// and a projection level with rules.
//
// Each feature and each name reports on its own, in the order of the
// inventory and then in the order of the names, so one run reports every
// feature without a verdict and every verdict without a feature.
func AssertCoveredInventory(tb assert.TB, c Corpus) {
	tb.Helper()

	allowed, rule := []Verdict{Refuses, Loads}, "a corpus without rules states loads or refuses"
	if c.Rules != nil {
		allowed, rule = []Verdict{Refuses, Projects, ProjectsPartly, Opaque},
			"a corpus with rules states projects, projects partly, opaque or refuses"
	}
	inventory := Inventory()
	ids := make([]string, 0, len(inventory))
	for _, f := range inventory {
		ids = append(ids, f.ID)
		expect.Contains(tb, c.Coverage, f.ID, fmt.Sprintf("the coverage states a verdict for %s: "+
			"silence on a capability is the gap this list exists to close", f.ID))
		if verdict, stated := c.Coverage[f.ID]; stated {
			expect.Contains(tb, allowed, verdict, fmt.Sprintf("%s states %s: %s", f.ID, verdict, rule))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Remainder)) {
		expect.Contains(tb, ids, id, fmt.Sprintf("the inventory lists %s, which the remainder names", id))
	}
	for _, id := range slices.Sorted(maps.Keys(c.Coverage)) {
		expect.Contains(tb, ids, id, fmt.Sprintf("the inventory lists %s, which the coverage names", id))
	}
}

// AssertFeature checks one covered feature against its expectations over
// the graph of the corpus. A declaration that the load lacks reports on
// its own, and its expectation does not run.
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
		expect.True(tb, held, fmt.Sprintf("the load contains %s, which %s declares", id, f.ID))
		if held && d.Check != nil {
			d.Check(tb, ctx(decl))
		}
	}
	if f.Check != nil {
		f.Check(tb, ctx(nil))
	}
}

// AssertRefusedFeature checks one refusal in two ways. The tree contains
// nothing under the directory of the feature, and the load contains no
// package that the feature would occupy. The second check finds a
// spelling in another place, such as a shared file or a moved tree that
// the directory check does not walk. The graph proves a refusal, and the
// place of the bytes does not. Each file and each package reports on its
// own.
func AssertRefusedFeature(tb assert.TB, c Corpus, g *store.Graph, f Feature) {
	tb.Helper()

	dir := featureRoot + f.ID
	err := fs.WalkDir(c.Sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			expect.NotEqual(tb, path, dir, fmt.Sprintf("%s is refused, so the tree spells no file %s", f.ID, dir))
			expect.False(tb, strings.HasPrefix(path, dir+"/"),
				fmt.Sprintf("%s is refused, so the tree spells no file under %s: %s", f.ID, dir, path))
		}
		return nil
	})
	assert.NoError(tb, err, "the corpus tree walks")

	occupied := map[string]bool{c.pkg(f.ID, ""): true}
	for _, d := range f.Declares {
		occupied[c.pkg(f.ID, d.Sub)] = true
	}
	for pkg := range g.Packages() {
		expect.NotContains(tb, occupied, pkg.ID.Package,
			f.ID+" is refused, so the load contains no package it occupies, wherever the spelling is")
	}
}

// corpusFixture loads the whole tree once at full depth into the fixture
// of the rules suite. The fixture has the sealed graph and the stamps of
// the load under the keys of the kernel and the language. The feature
// expectations and the levels read it, and the frontend suite runs loads
// of its own.
func corpusFixture(tb assert.TB, c Corpus) *rulestest.Fixture {
	tb.Helper()

	return rulestest.Loaded(tb, c.Frontend, c.Sources, c.Keys)
}
