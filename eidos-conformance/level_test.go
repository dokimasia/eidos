// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/conformance"
	gocorpus "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
)

// goTree is Go's corpus tree in its package under lang, which the level
// checks grade with Go's rules.
const goTree = "lang/go/testdata/corpus"

// The levels are predicates over the projections: a feature is at the
// level its language declared, never above and never below.
func TestLevel(t *testing.T) {
	t.Parallel()

	t.Run("AssertLevel", func(t *testing.T) {
		t.Parallel()

		t.Run("pins a whole projection to the top level", func(t *testing.T) {
			t.Parallel()

			c, fx := goCorpus(), goFixture(t)
			conformance.AssertLevel(t, c, fx, featureByID(t, "struct_fields"), conformance.Projects)
			got := assert.Rejects(t, "a whole projection declared partial", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "struct_fields"), conformance.ProjectsPartly)
			})
			assert.Length(t, got, 1, "the check fails once")
			assert.Equal(t, got[0].Contract, "struct_fields projects partly, so a reference, a member set or a "+
				"callable falls short: a capability nobody declared is a capability nobody tested",
				"a capability nobody declared is a capability nobody tested")
		})

		t.Run("pins a partial projection to the second level", func(t *testing.T) {
			t.Parallel()

			c, fx := goCorpus(), goFixture(t)
			conformance.AssertLevel(t, c, fx, featureByID(t, "composite_refs"), conformance.ProjectsPartly)
			got := assert.Rejects(t, "a partial projection declared whole", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "composite_refs"), conformance.Projects)
			})
			assert.Equal(t, contracts(got),
				[]string{"composite_refs projects, so no reference it reaches folds to Opaque or Inline"},
				"the check fails naming what fell short")
			opaque, _ := got[0].Got()
			assert.NotEqual(t, opaque, any(0), "and how many references fold")
		})

		t.Run("pins a remainder to the stamps the load made", func(t *testing.T) {
			t.Parallel()

			c, fx := goCorpus(), goFixture(t)
			testFile := conformance.Decl{Name: "f/test_classification/a_test.go", Kind: symbol.KindFile}
			c.Remainder = map[string][]conformance.Remainder{
				"test_classification": {{Decl: testFile, Key: golang.TestFileKey}},
			}
			got := assert.Rejects(t, "a remainder under the top level", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "test_classification"), conformance.Projects)
			})
			assert.Equal(t, contracts(got), []string{
				"test_classification projects, and a whole projection leaves no remainder",
			}, "a whole projection leaves none")

			c.Remainder = map[string][]conformance.Remainder{
				"struct_fields": {{
					Decl: conformance.Decl{Name: "Point", Kind: symbol.KindStruct},
					Key:  golang.ComparableKey,
				}},
			}
			got = assert.Rejects(t, "a remainder nothing stamped", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "struct_fields"), conformance.ProjectsPartly)
			})
			assert.NotEmpty(t, got, "the check fails")
			expect.That(t, got[0].Contract).
				HasPrefix("the load stamps "+string(golang.ComparableKey)+" on ", "the key is named").
				HasSuffix(", which struct_fields names as a remainder", "and absent from the declaration")
		})

		t.Run("pins an alias of an inline body to the opaque level", func(t *testing.T) {
			t.Parallel()

			c, fx := opaqueCorpus(t)
			f := conformance.Feature{
				ID:       "struct_fields",
				Declares: []conformance.Decl{{Name: "Blob", Kind: symbol.KindAlias}},
			}
			conformance.AssertLevel(t, c, fx, f, conformance.Opaque)
			got := assert.Rejects(t, "an opaque alias declared to project", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, f, conformance.Projects)
			})
			assert.Equal(t, contracts(got),
				[]string{"struct_fields projects, so no reference it reaches folds to Opaque or Inline"},
				"the alias's own target folds to Inline")
			opaque, _ := got[0].Got()
			assert.Equal(t, opaque, any(1), "the alias's target is the one reference that folds")

			whole := featureByID(t, "struct_fields")
			got = assert.Rejects(t, "a whole feature declared opaque", func(tb assert.TB) {
				conformance.AssertLevel(tb, goCorpus(), goFixture(t), whole, conformance.Opaque)
			})
			assert.Equal(t, contracts(got), []string{
				"struct_fields is opaque, so an alias it declares has a target that folds to Opaque or Inline",
			}, "opaque needs an alias whose target folds to Opaque or Inline")
		})

		t.Run("measures each nested declaration once", func(t *testing.T) {
			t.Parallel()

			c, fx := nestedCorpus(t)
			f := conformance.Feature{
				ID:       "struct_fields",
				Declares: []conformance.Decl{{Name: "Outer", Kind: symbol.KindStruct}},
			}
			got := assert.Rejects(t, "a nested member gap under the top level", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, f, conformance.Projects)
			})
			assert.Equal(t, contracts(got), []string{"struct_fields projects, so no member set it declares has a gap"},
				"the check fails once, for the member gaps")
			gaps, _ := got[0].Got()
			assert.Equal(t, gaps, any(3), "Outer's one gap and Inner's two, with Outer measured once though it is "+
				"both declared and in the feature's package")
		})

		t.Run("fails a level under a corpus without rules", func(t *testing.T) {
			t.Parallel()

			c, fx := goCorpus(), goFixture(t)
			c.Rules = nil
			got := assert.Rejects(t, "a level without rules", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "struct_fields"), conformance.Projects)
			})
			assert.Equal(t, contracts(got),
				[]string{"struct_fields states projects under a corpus whose rules evaluate it"},
				"nothing can evaluate it")
		})

		t.Run("fails a verdict that is no projection level", func(t *testing.T) {
			t.Parallel()

			c, fx := goCorpus(), goFixture(t)
			got := assert.Rejects(t, "the load verdict as a level", func(tb assert.TB) {
				conformance.AssertLevel(tb, c, fx, featureByID(t, "struct_fields"), conformance.Loads)
			})
			assert.Equal(t, contracts(got), []string{"struct_fields states projects, projects partly or opaque"},
				"loads is the read-side verdict")
		})
	})
}

// contracts returns the contract of each failure record, in call order.
func contracts(records []assert.Failure) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Contract
	}
	return out
}

// goCorpus returns Go's entry over its corpus tree.
func goCorpus() conformance.Corpus { return gocorpus.Corpus(os.DirFS(goTree)) }

// goFixture loads the Go corpus once for a case.
func goFixture(tb assert.TB) *rulestest.Fixture {
	tb.Helper()
	return rulestest.Loaded(tb, gofrontend.New(nil), os.DirFS(goTree), golang.Keys)
}

// opaqueCorpus returns a scripted corpus whose one feature package
// declares an alias of an inline body, and the fixture holding it,
// so the opaque level has a subject.
func opaqueCorpus(tb assert.TB) (conformance.Corpus, *rulestest.Fixture) {
	tb.Helper()

	pkg := "f/struct_fields"
	blob := &node.Alias{
		ID: symbol.Identity{
			Lang:    frontendtest.ScriptedLang,
			Package: pkg,
			Name:    "Blob",
			Kind:    symbol.KindAlias,
		},
		Name:   "Blob",
		Target: &node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline},
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(&node.Package{
		ID: symbol.Identity{
			Lang:    frontendtest.ScriptedLang,
			Package: pkg,
			Kind:    symbol.KindPackage,
		},
		Path: []string{"f", "struct_fields"},
		Files: []*node.File{
			{
				ID: symbol.Identity{
					Lang:    frontendtest.ScriptedLang,
					Package: pkg,
					Name:    "a.s",
					Kind:    symbol.KindFile,
				},
				Path:  "a.s",
				Decls: node.Symbols{blob},
			},
		},
	}), "the opaque package is admitted")
	g.Freeze()
	c := conformance.Corpus{Frontend: frontendtest.NewScripted(), Rules: rulestest.Scripted()}
	return c, &rulestest.Fixture{Graph: g, Facts: meta.NewFacts(meta.NewRegistry())}
}

// nestedCorpus returns a scripted corpus whose one feature package
// declares a struct embedding one builtin and nesting a struct that
// embeds two more, and the fixture holding it. An embedded builtin
// resolves to nothing, so each is one member-set gap, while its
// reference still projects.
func nestedCorpus(tb assert.TB) (conformance.Corpus, *rulestest.Fixture) {
	tb.Helper()

	pkg := "f/struct_fields"
	id := func(owner, name string, kind symbol.Kind) symbol.Identity {
		return symbol.Identity{
			Lang: frontendtest.ScriptedLang, Package: pkg, Owner: owner, Name: name, Kind: kind,
		}
	}
	embed := func(owner, spelling string) *node.Embed {
		return &node.Embed{
			ID:  id(owner, spelling, symbol.KindEmbed),
			Ref: &node.TypeRef{Spelling: spelling},
		}
	}
	inner := &node.Struct{
		ID:     id("Outer", "Inner", symbol.KindStruct),
		Name:   "Inner",
		Embeds: []*node.Embed{embed("Outer.Inner", "int"), embed("Outer.Inner", "string")},
	}
	outer := &node.Struct{
		ID:     id("", "Outer", symbol.KindStruct),
		Name:   "Outer",
		Embeds: []*node.Embed{embed("Outer", "bool")},
		Types:  node.Symbols{inner},
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(&node.Package{
		ID:   id("", "", symbol.KindPackage),
		Path: []string{"f", "struct_fields"},
		Files: []*node.File{{
			ID:    id("", "a.s", symbol.KindFile),
			Path:  "a.s",
			Decls: node.Symbols{outer},
		}},
	}), "the nested package is admitted")
	g.Freeze()
	c := conformance.Corpus{Frontend: frontendtest.NewScripted(), Rules: rulestest.Scripted()}
	return c, &rulestest.Fixture{Graph: g, Facts: meta.NewFacts(meta.NewRegistry())}
}
