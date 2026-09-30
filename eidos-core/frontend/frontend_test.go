// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The kit fixture's name, claim and tree, so a case names what it
// declares without repeating a literal.
const (
	kitName       = "kitfake"
	anyScripted   = "**/*.zz"
	storePath     = "svc/store"
	storeTestFile = "svc/store/row_test.zz"
	depPath       = "svc/dep"
	classified    = "classified"
)

// kitFake lowers the scripted language through the kit, its
// test-file stamp moved from a parse statement into a classifier,
// which is the seam under test.
func kitFake() plugin.Frontend {
	inner := frontendtest.NewScripted()
	return frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
		Version(inner.Ver).
		Match(inner.Sel...).
		Units(inner.Partition).
		Parse(inner.Parse).
		Classify(markTests).
		Resolve(inner.Resolve).
		Options(inner.Opts).
		Build()
}

// markTests stamps the test-file key on every parsed file whose
// path ends in the test suffix.
func markTests(u *plugin.SourceUnit) error {
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		for _, f := range pkg.Files {
			if strings.HasSuffix(f.Path, "_test.zz") {
				gb.Stamp(f, meta.RawStamp{
					Key: frontendtest.ScriptedTestKey, Value: classified, Pos: f.Pos,
				})
			}
		}
	}
	return nil
}

// kitTree is the fixture the kit-built frontend loads: two
// packages, a cross-package reference, a directive statement, a
// test file for the classifier, and a signature root.
func kitTree() fstest.MapFS {
	return fstest.MapFS{
		"mod.zz": {Data: []byte("mod v1\n")},
		"svc/api/user.zz": {Data: []byte(
			"package svc/api\ntype User string\n",
		)},
		"svc/store/row.zz": {Data: []byte(
			"package svc/store\nimport api svc/api\ntype Row api.User int\n+gen:table name=users\n",
		)},
		storeTestFile: {Data: []byte(
			"package svc/store\ntype RowTest string\n",
		)},
		"svc/dep/dep.zz": {Data: []byte(
			"package svc/dep\ntype Dep string\nconst hidden\n",
		)},
	}
}

// loadKit drives one load over the kit tree under the suite's
// brand with the frontend the case built, and returns the load's
// own error.
func loadKit(f plugin.Frontend) (*store.Graph, error) {
	g, _, err := load.Load(context.Background(), load.Config{
		FS:        kitTree(),
		Frontends: []plugin.Frontend{f},
		Sink:      diag.NewSink(),
		Brand:     frontendtest.Brand,
	})
	return g, err
}

// The kit lowers a declaration to the frontend role, so the built
// frontend meets the same conformance bar a hand-rolled one does,
// and the lowering's own seams are pinned beside it: classifier
// order, fatality and the declaration defects.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return kitFake(), &frontendtest.Fixture{
					Sources:    kitTree(),
					Signatures: []string{depPath},
					Schemas:    frontendtest.ScriptedSchemas(),
					Keys:       frontendtest.ScriptedKeys,
				}
			})
		})

		t.Run("returns no options provider without a declaration", func(t *testing.T) {
			t.Parallel()

			inner := frontendtest.NewScripted()
			bare := frontend.New("bare", frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Build()
			_, is := bare.(plugin.OptionsProvider)
			assert.False(t, is, "the frontend declares no options")
		})

		t.Run("returns an options provider for a declaration", func(t *testing.T) {
			t.Parallel()

			_, is := kitFake().(plugin.OptionsProvider)
			assert.True(t, is, "the declared configuration folds into the keys")
		})

		inner := frontendtest.NewScripted()
		whole := func() *frontend.Builder {
			return frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve)
		}

		t.Run("builds a whole declaration", func(t *testing.T) {
			t.Parallel()

			assert.NotPanics(t, func() { whole().Build() }, "the declaration builds")
		})

		defects := []struct {
			name  string
			build func()
		}{
			{
				name:  "panics on an empty name",
				build: func() { frontend.New("", frontendtest.ScriptedLang, inner.Syntax()).Build() },
			},
			{
				name: "panics on an empty language",
				build: func() {
					frontend.New(kitName, "", inner.Syntax()).
						Version("1").Match(anyScripted).
						Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
				},
			},
			{
				name: "panics on a missing version",
				build: func() {
					frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
						Match(anyScripted).
						Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
				},
			},
			{
				name: "panics on an empty claim",
				build: func() {
					frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
						Version("1").
						Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
				},
			},
			{
				name: "panics on a missing partition",
				build: func() {
					frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
						Version("1").Match(anyScripted).
						Parse(inner.Parse).Resolve(inner.Resolve).Build()
				},
			},
			{
				name: "panics on a missing parse",
				build: func() {
					frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
						Version("1").Match(anyScripted).
						Units(inner.Partition).Resolve(inner.Resolve).Build()
				},
			},
			{
				name: "panics on a missing resolve",
				build: func() {
					frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
						Version("1").Match(anyScripted).
						Units(inner.Partition).Parse(inner.Parse).Build()
				},
			},
		}
		for _, tt := range defects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Panics(t, tt.build, "the declaration defect panics at Build")
			})
		}
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		inner := frontendtest.NewScripted()
		declared := func() *frontend.Builder {
			return frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve)
		}

		t.Run("reports false for a declaration without Overloads", func(t *testing.T) {
			t.Parallel()

			assert.False(t, declared().Build().Overloads(), "a language that cannot overload is the default")
		})

		t.Run("reports true for a declaration with Overloads", func(t *testing.T) {
			t.Parallel()

			assert.True(t, declared().Overloads().Build().Overloads(), "the declaration states it")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("records the classifier's stamp in the store", func(t *testing.T) {
			t.Parallel()

			g, err := loadKit(kitFake())
			assert.NoError(t, err, "the fixture loads")
			file := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: storePath,
				Name: storeTestFile, Kind: symbol.KindFile,
			}
			stamps := g.StampsOf(file)
			assert.Length(t, stamps, 1, "the classifier's stamp is in the store")
			assert.Equal(t, stamps[0].Key, frontendtest.ScriptedTestKey, "the stamp has its key")
			assert.Equal(t, stamps[0].Value.(string), classified, "the stamp has the classifier's value")
		})

		t.Run("returns the declared parse's error", func(t *testing.T) {
			t.Parallel()

			broken := errors.New("kitfake: the parse refuses")
			inner := frontendtest.NewScripted()
			f := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").
				Match(inner.Sel...).
				Units(inner.Partition).
				Parse(func(context.Context, *plugin.SourceUnit) error { return broken }).
				Classify(markTests).
				Resolve(inner.Resolve).
				Build()
			_, err := loadKit(f)
			assert.ErrorIs(t, err, broken, "the classifiers never run over a unit the parse gave up on")
		})

		t.Run("returns a classifier's error", func(t *testing.T) {
			t.Parallel()

			broken := errors.New("kitfake: the classifier refuses")
			inner := frontendtest.NewScripted()
			f := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").
				Match(inner.Sel...).
				Units(inner.Partition).
				Parse(inner.Parse).
				Classify(func(*plugin.SourceUnit) error { return broken }).
				Resolve(inner.Resolve).
				Build()
			_, err := loadKit(f)
			assert.ErrorIs(t, err, broken, "the load stops and seals no half-made stamps")
		})
	})
}
