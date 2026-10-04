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
	"go.dokimi.dev/assert/bench"

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

// allocRuns is the number of calls [assert.MaxAllocs] makes: one to
// warm the function, and the 100 it counts.
const allocRuns = 101

// The ceilings of a declaration's steps.
const (
	// newAllocs is one new declaration: the builder.
	newAllocs = 1
	// appendAllocs is the first Match or Classify on a new declaration:
	// the list it appends to.
	appendAllocs = 1
	// buildAllocs is one Build of a declaration without optional roles:
	// the lowered frontend.
	buildAllocs = 1
	// buildRolesAllocs is one Build of a declaration in every optional
	// role: the lowered frontend, and the struct that composes it with
	// the roles.
	buildRolesAllocs = 2
)

// setter is one field setter of a declaration, called through a
// function made before any measurement.
type setter struct {
	name string
	set  func(*frontend.Builder) *frontend.Builder
}

// setters returns every setter that writes one field of a declaration,
// each over the scripted frontend's functions.
func setters(inner *frontendtest.Scripted) []setter {
	partition, parse, resolve, opts := inner.Partition, inner.Parse, inner.Resolve, inner.Opts
	dependencies := frontendtest.NewScriptedDependent().Dependencies
	exports := frontendtest.NewScriptedExporter().Exports
	return []setter{
		{name: "Version", set: func(b *frontend.Builder) *frontend.Builder { return b.Version("1") }},
		{name: "Overloads", set: func(b *frontend.Builder) *frontend.Builder { return b.Overloads() }},
		{name: "Units", set: func(b *frontend.Builder) *frontend.Builder { return b.Units(partition) }},
		{name: "Parse", set: func(b *frontend.Builder) *frontend.Builder { return b.Parse(parse) }},
		{name: "Resolve", set: func(b *frontend.Builder) *frontend.Builder { return b.Resolve(resolve) }},
		{name: "Options", set: func(b *frontend.Builder) *frontend.Builder { return b.Options(opts) }},
		{name: "Dependencies", set: func(b *frontend.Builder) *frontend.Builder {
			return b.Dependencies(dependencies)
		}},
		{name: "Exports", set: func(b *frontend.Builder) *frontend.Builder { return b.Exports(exports) }},
	}
}

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

		dependent := frontendtest.NewScriptedDependent()
		exporter := frontendtest.NewScriptedExporter()
		roles := []struct {
			name         string
			options      bool
			dependencies bool
			exports      bool
		}{
			{name: "returns a frontend in no optional role for a declaration of none"},
			{name: "returns a frontend in the options role alone", options: true},
			{name: "returns a frontend in the dependent role alone", dependencies: true},
			{name: "returns a frontend in the exporter role alone", exports: true},
			{
				name:    "returns a frontend in the options role beside the dependent role",
				options: true, dependencies: true,
			},
			{name: "returns a frontend in the options role beside the exporter role", options: true, exports: true},
			{
				name:         "returns a frontend in the dependent role beside the exporter role",
				dependencies: true, exports: true,
			},
			{
				name:    "returns a frontend in every optional role",
				options: true, dependencies: true, exports: true,
			},
		}
		for _, tt := range roles {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := frontend.New(kitName, frontendtest.ScriptedLang, dependent.Syntax()).
					Version("1").Match(anyScripted).
					Units(dependent.Partition).Parse(dependent.Parse).Resolve(dependent.Resolve)
				if tt.options {
					b.Options(dependent.Opts)
				}
				if tt.dependencies {
					b.Dependencies(dependent.Dependencies)
				}
				if tt.exports {
					b.Exports(exporter.Exports)
				}
				built := b.Build()
				_, optioned := built.(plugin.OptionsProvider)
				_, depends := built.(plugin.Dependent)
				_, exports := built.(plugin.Exporter)
				assert.Equal(t, []bool{optioned, depends, exports},
					[]bool{tt.options, tt.dependencies, tt.exports}, "exactly the declared roles")
			})
		}

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

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what the declared function returns", func(t *testing.T) {
			t.Parallel()

			refused := errors.New("kitfake: the round refuses")
			inner := frontendtest.NewScripted()
			f := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Dependencies(func(context.Context, *plugin.DependencyRound, plugin.StoreReader) (
					[][]plugin.SourceRef, error,
				) {
					return nil, refused
				}).
				Build()
			dependent, is := f.(plugin.Dependent)
			assert.True(t, is, "the declaration states the role")
			_, err := dependent.Dependencies(context.Background(), &plugin.DependencyRound{Number: 1}, nil)
			assert.ErrorIs(t, err, refused, "the round runs the declared function")
		})
	})

	t.Run("Exports", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what the declared function returns", func(t *testing.T) {
			t.Parallel()

			published := plugin.Candidates{{{Lang: frontendtest.ScriptedLang, Package: depPath, Name: kitName}}}
			inner := frontendtest.NewScripted()
			f := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Exports(func(plugin.ImportScope, string) plugin.Candidates { return published }).
				Build()
			exporter, is := f.(plugin.Exporter)
			assert.True(t, is, "the declaration states the role")
			assert.Equal(t, exporter.Exports(plugin.ImportScope{}, kitName), published,
				"the resolution step follows the declared function")
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

// A declaration allocates its builder, the first entry of each list it
// appends to, and the lowered frontend, and its setters allocate
// nothing, in the ordinary run, which runs no benchmark. An append and a
// Build each take a declaration built before the count, because each
// changes or freezes the declaration it is called on. The check runs
// alone, because AllocsPerRun counts every goroutine's allocations and
// refuses to run beside parallel tests.
func TestFrontendAllocs(t *testing.T) {
	inner := frontendtest.NewScripted()
	syntax := inner.Syntax()
	var b *frontend.Builder
	assert.MaxAllocs(t, func() { b = frontend.New(kitName, frontendtest.ScriptedLang, syntax) }, newAllocs,
		"New allocates the builder")
	for _, tt := range setters(inner) {
		var got *frontend.Builder
		assert.MaxAllocs(t, func() { got = tt.set(b) }, 0, tt.name+" allocates nothing")
		assert.True(t, got == b, tt.name+" returns its builder")
	}

	fresh, at := news(inner, allocRuns), 0
	assert.MaxAllocs(t, func() {
		fresh[at].Match(anyScripted)
		at++
	}, appendAllocs, "Match allocates the claim of a new declaration")
	fresh, at = news(inner, allocRuns), 0
	classify := frontend.Classifier(markTests)
	assert.MaxAllocs(t, func() {
		fresh[at].Classify(classify)
		at++
	}, appendAllocs, "Classify allocates the classifier list of a new declaration")

	bare, at := declarations(inner, allocRuns, false), 0
	assert.MaxAllocs(t, func() {
		bare[at].Build()
		at++
	}, buildAllocs, "Build allocates the lowered frontend")
	roled, at := declarations(inner, allocRuns, true), 0
	assert.MaxAllocs(t, func() {
		roled[at].Build()
		at++
	}, buildRolesAllocs, "Build allocates the lowered frontend and the composition of its roles")
}

// BenchmarkFrontend measures each step of a declaration: the builder,
// the setters, the first entry of each list, and the lowering.
func BenchmarkFrontend(b *testing.B) {
	inner := frontendtest.NewScripted()

	b.Run("New", func(b *testing.B) {
		syntax := inner.Syntax()
		c := bench.Start(b).MaxAllocs(newAllocs)
		defer c.End()
		var got *frontend.Builder
		for c.Loop() {
			got = frontend.New(kitName, frontendtest.ScriptedLang, syntax)
		}
		assert.NotNil(b, got, "New returns a builder")
	})

	for _, tt := range setters(inner) {
		b.Run(tt.name, func(b *testing.B) {
			builder := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax())
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got *frontend.Builder
			for c.Loop() {
				got = tt.set(builder)
			}
			assert.True(b, got == builder, tt.name+" returns its builder")
		})
	}

	b.Run("Match/a new declaration's first pattern", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(appendAllocs)
		defer c.End()
		var builder *frontend.Builder
		for c.Loop() {
			c.Excluding(func() { builder = frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()) })
			builder.Match(anyScripted)
		}
		assert.NotPanics(b, func() {
			builder.Version("1").Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).Build()
		}, "the claim completes a declaration")
	})

	b.Run("Classify/a new declaration's first classifier", func(b *testing.B) {
		classify := frontend.Classifier(markTests)
		c := bench.Start(b).MaxAllocs(appendAllocs)
		defer c.End()
		var builder *frontend.Builder
		for c.Loop() {
			c.Excluding(func() { builder = frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()) })
			builder.Classify(classify)
		}
		assert.NotNil(b, builder, "the classifier is declared")
	})

	b.Run("Build/a declaration without optional roles", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(buildAllocs)
		defer c.End()
		var (
			builder *frontend.Builder
			got     plugin.Frontend
		)
		for c.Loop() {
			c.Excluding(func() { builder = declarations(inner, 1, false)[0] })
			got = builder.Build()
		}
		_, optioned := got.(plugin.OptionsProvider)
		assert.False(b, optioned, "the frontend has no optional role")
	})

	b.Run("Build/a declaration in every optional role", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(buildRolesAllocs)
		defer c.End()
		var (
			builder *frontend.Builder
			got     plugin.Frontend
		)
		for c.Loop() {
			c.Excluding(func() { builder = declarations(inner, 1, true)[0] })
			got = builder.Build()
		}
		_, exports := got.(plugin.Exporter)
		assert.True(b, exports, "the frontend has every optional role")
	})
}

// news returns n new declarations of the kit's frontend.
func news(inner *frontendtest.Scripted, n int) []*frontend.Builder {
	out := make([]*frontend.Builder, n)
	for i := range out {
		out[i] = frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax())
	}
	return out
}

// declarations returns n whole declarations of the kit's frontend, each
// in every optional role where roles is set and in none otherwise.
func declarations(inner *frontendtest.Scripted, n int, roles bool) []*frontend.Builder {
	out := news(inner, n)
	for i, b := range out {
		b.Version("1").Match(anyScripted).Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve)
		if roles {
			b.Options(inner.Opts).
				Dependencies(frontendtest.NewScriptedDependent().Dependencies).
				Exports(frontendtest.NewScriptedExporter().Exports)
		}
		out[i] = b
	}
	return out
}
