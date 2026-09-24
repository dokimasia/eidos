// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
)

// goCorpus is the Go language's entry with its rules, the tree the
// level cases evaluate over.
func goCorpus() conformance.Corpus {
	return conformance.Corpus{
		Frontend: gofrontend.New(nil),
		Sources:  os.DirFS("testdata/go"),
		Keys:     gofrontend.Keys,
		Rules:    gorules.New(),
	}
}

// goFixture loads the Go corpus once for a case.
func goFixture(tb assert.TB) *rulestest.Fixture {
	tb.Helper()
	return rulestest.Loaded(tb, gofrontend.New(nil), os.DirFS("testdata/go"), golang.Keys)
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

// The levels are predicates over the projections: a feature sits on
// the level its language declared, never above and never below.
func TestLevel(t *testing.T) {
	t.Parallel()

	t.Run("holds a whole projection to the top level", func(t *testing.T) {
		t.Parallel()

		c, fx := goCorpus(), goFixture(t)
		conformance.AssertLevel(t, c, fx, featureByID(t, "struct_fields"), conformance.Projects)
		msg := assert.Rejects(t, "a whole projection declared partial", func(tb assert.TB) {
			conformance.AssertLevel(
				tb,
				c,
				fx,
				featureByID(t, "struct_fields"),
				conformance.ProjectsPartly,
			)
		})
		assert.Contains(
			t,
			msg,
			"projects whole",
			"a capability nobody declared is a capability nobody tested",
		)
	})

	t.Run("holds a partial projection to the second level", func(t *testing.T) {
		t.Parallel()

		c, fx := goCorpus(), goFixture(t)
		conformance.AssertLevel(
			t,
			c,
			fx,
			featureByID(t, "composite_refs"),
			conformance.ProjectsPartly,
		)
		msg := assert.Rejects(t, "a partial projection declared whole", func(tb assert.TB) {
			conformance.AssertLevel(
				tb,
				c,
				fx,
				featureByID(t, "composite_refs"),
				conformance.Projects,
			)
		})
		assert.Contains(t, msg, "Opaque", "naming what fell short")
	})

	t.Run("holds a remainder to the stamps the load made", func(t *testing.T) {
		t.Parallel()

		c, fx := goCorpus(), goFixture(t)
		testFile := conformance.Decl{Name: "f/test_classification/a_test.go", Kind: symbol.KindFile}
		c.Remainder = map[string][]conformance.Remainder{
			"test_classification": {{Decl: testFile, Key: golang.TestFileKey}},
		}
		msg := assert.Rejects(t, "a remainder under the top level", func(tb assert.TB) {
			conformance.AssertLevel(
				tb,
				c,
				fx,
				featureByID(t, "test_classification"),
				conformance.Projects,
			)
		})
		assert.Contains(t, msg, "remainder", "a whole projection leaves none")
		c.Remainder = map[string][]conformance.Remainder{
			"struct_fields": {
				{
					Decl: conformance.Decl{Name: "Point", Kind: symbol.KindStruct},
					Key:  golang.ComparableKey,
				},
			},
		}
		msg = assert.Rejects(t, "a remainder nothing stamped", func(tb assert.TB) {
			conformance.AssertLevel(
				tb,
				c,
				fx,
				featureByID(t, "struct_fields"),
				conformance.ProjectsPartly,
			)
		})
		assert.Contains(t, msg, "no such stamp", "the key is named and absent")
	})

	t.Run("holds an alias of an inline body to the opaque level", func(t *testing.T) {
		t.Parallel()

		c, fx := opaqueCorpus(t)
		f := conformance.Feature{
			ID:       "struct_fields",
			Declares: []conformance.Decl{{Name: "Blob", Kind: symbol.KindAlias}},
		}
		conformance.AssertLevel(t, c, fx, f, conformance.Opaque)
		msg := assert.Rejects(t, "an opaque alias declared to project", func(tb assert.TB) {
			conformance.AssertLevel(tb, c, fx, f, conformance.Projects)
		})
		assert.Contains(t, msg, "Inline", "the alias's own target folds to Inline")
		msg = assert.Rejects(t, "a whole feature declared opaque", func(tb assert.TB) {
			conformance.AssertLevel(
				tb,
				goCorpus(),
				goFixture(t),
				featureByID(t, "struct_fields"),
				conformance.Opaque,
			)
		})
		assert.Contains(t, msg, "no alias", "opaque needs an alias whose target folds to Opaque")
	})

	t.Run("measures nested declarations, each once", func(t *testing.T) {
		t.Parallel()

		c, fx := nestedCorpus(t)
		f := conformance.Feature{
			ID:       "struct_fields",
			Declares: []conformance.Decl{{Name: "Outer", Kind: symbol.KindStruct}},
		}
		msg := assert.Rejects(t, "a nested member gap under the top level", func(tb assert.TB) {
			conformance.AssertLevel(tb, c, fx, f, conformance.Projects)
		})
		assert.Contains(t, msg, "3 member gaps",
			"Outer's one gap and Inner's two, with Outer measured once though it is "+
				"both declared and in the feature's package")
	})

	t.Run("refuses a level without rules and a verdict that is no level", func(t *testing.T) {
		t.Parallel()

		c, fx := goCorpus(), goFixture(t)
		msg := assert.Rejects(t, "a level without rules", func(tb assert.TB) {
			without := c
			without.Rules = nil
			conformance.AssertLevel(
				tb,
				without,
				fx,
				featureByID(t, "struct_fields"),
				conformance.Projects,
			)
		})
		assert.Contains(t, msg, "without rules", "nothing can evaluate it")
		msg = assert.Rejects(t, "the load verdict as a level", func(tb assert.TB) {
			conformance.AssertLevel(tb, c, fx, featureByID(t, "struct_fields"), conformance.Loads)
		})
		assert.Contains(t, msg, "no projection level", "loads is the read-side verdict")
	})

	t.Run("spells every verdict", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, conformance.Loads.String(), "loads", "loads")
		assert.Equal(t, conformance.Refuses.String(), "refuses", "refuses")
		assert.Equal(t, conformance.Projects.String(), "projects", "projects")
		assert.Equal(t, conformance.ProjectsPartly.String(), "projects partly", "projects partly")
		assert.Equal(t, conformance.Opaque.String(), "opaque", "opaque")
		assert.Equal(t, conformance.Verdict(9).String(), "9", "an unknown value spells its number")
	})
}
