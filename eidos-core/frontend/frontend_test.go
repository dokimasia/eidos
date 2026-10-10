// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

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
	// fakeRoot is the environment variable that the store case reads.
	fakeRoot = "FAKEROOT"
)

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
		{name: "Stores", set: func(b *frontend.Builder) *frontend.Builder { return b.Stores(locate) }},
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

		t.Run("returns a key provider that registers nothing without a declaration", func(t *testing.T) {
			t.Parallel()

			inner := frontendtest.NewScripted()
			bare := frontend.New("bare", frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Build()
			keyed, is := bare.(plugin.KeyProvider)
			assert.True(t, is, "every built frontend has the key role")
			r := meta.NewRegistry()
			assert.NoError(t, keyed.Keys(r), "the registration succeeds")
			assert.Empty(t, slices.Collect(r.Keys()), "the registration registers nothing")
		})

		dependent := frontendtest.NewScriptedDependent()
		exporter := frontendtest.NewScriptedExporter()
		roles := []struct {
			name         string
			options      bool
			dependencies bool
			exports      bool
			stores       bool
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
				name:    "returns a frontend in the options, dependent and exporter roles",
				options: true, dependencies: true, exports: true,
			},
			{
				name:         "returns a frontend in the dependent role beside the store role",
				dependencies: true, stores: true,
			},
			{
				name:    "returns a frontend in the options, dependent and store roles",
				options: true, dependencies: true, stores: true,
			},
			{
				name:         "returns a frontend in the dependent, exporter and store roles",
				dependencies: true, exports: true, stores: true,
			},
			{
				name:    "returns a frontend in every optional role",
				options: true, dependencies: true, exports: true, stores: true,
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
				if tt.stores {
					b.Stores(locate)
				}
				built := b.Build()
				_, optioned := built.(plugin.OptionsProvider)
				_, depends := built.(plugin.Dependent)
				_, exports := built.(plugin.Exporter)
				_, locates := built.(plugin.StoreLocator)
				assert.Equal(t, []bool{optioned, depends, exports, locates},
					[]bool{tt.options, tt.dependencies, tt.exports, tt.stores}, "exactly the declared roles")
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
			{
				name:  "panics on stores without dependency rounds",
				build: func() { whole().Stores(locate).Build() },
			},
			{
				name:  "panics on a nil key registration",
				build: func() { whole().Keys(frontendtest.ScriptedKeys, nil).Build() },
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
			_, err := dependent.Dependencies(t.Context(), &plugin.DependencyRound{Number: 1}, nil)
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

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what the declared function returns", func(t *testing.T) {
			t.Parallel()

			missing := errors.New("kitfake: set FAKEROOT")
			inner := frontendtest.NewScriptedDependent()
			f := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Dependencies(inner.Dependencies).
				Stores(func(getenv func(string) string) (map[string]fs.FS, error) {
					return nil, fmt.Errorf("%w for %s", missing, getenv(fakeRoot))
				}).
				Build()
			locator, is := f.(plugin.StoreLocator)
			assert.True(t, is, "the declaration states the role")
			_, err := locator.Stores(func(key string) string { return key + "=unset" })
			assert.ErrorIs(t, err, missing, "the frontend runs the declared function")
			assert.Contains(t, err.Error(), fakeRoot+"=unset", "the function reads the environment of the caller")
		})
	})

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		inner := frontendtest.NewScripted()
		declared := func(register ...func(*meta.Registry) error) plugin.KeyProvider {
			built := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
				Version("1").Match(anyScripted).
				Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve).
				Keys(register...).
				Build()
			keyed, is := built.(plugin.KeyProvider)
			assert.True(t, is, "every built frontend has the key role")
			return keyed
		}

		t.Run("runs the declared registrations in declaration order", func(t *testing.T) {
			t.Parallel()

			var order []string
			first := func(*meta.Registry) error {
				order = append(order, "first")
				return nil
			}
			second := func(*meta.Registry) error {
				order = append(order, "second")
				return nil
			}
			assert.NoError(t, declared(first, second).Keys(meta.NewRegistry()), "the registrations succeed")
			assert.Equal(t, order, []string{"first", "second"}, "the registrations run in declaration order")
		})

		t.Run("registers the keys of the declared registration", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, declared(frontendtest.ScriptedKeys).Keys(r.For(string(frontendtest.ScriptedLang))),
				"the registration succeeds")
			_, held := r.Resolve(frontendtest.ScriptedTestKey)
			assert.True(t, held, "the scripted key is registered")
		})

		t.Run("returns the faults of every registration joined", func(t *testing.T) {
			t.Parallel()

			early, late := errors.New("kitfake: the early registration fails"),
				errors.New("kitfake: the late registration fails")
			err := declared(
				func(*meta.Registry) error { return early },
				func(*meta.Registry) error { return late },
			).Keys(meta.NewRegistry())
			expect.That(t, err).
				ErrorIs(early, "the error has the first fault").
				ErrorIs(late, "the error has the second fault")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("records the classifier's stamp in the store", func(t *testing.T) {
			t.Parallel()

			g, err := loadKit(t, kitFake())
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
			_, err := loadKit(t, f)
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
			_, err := loadKit(t, f)
			assert.ErrorIs(t, err, broken, "the load stops and seals no half-made stamps")
		})
	})
}

// A declaration allocates its builder, the first entry of each list it
// appends to, and the lowered frontend, and its setters allocate
// nothing, in the ordinary run, which runs no benchmark. An append and a
// Build each take a declaration built outside the count, because each
// changes or freezes the declaration it is called on. The check runs
// alone, because the count includes every goroutine's allocations.
func TestFrontendAllocs(t *testing.T) {
	inner := frontendtest.NewScripted()
	syntax := inner.Syntax()
	var b *frontend.Builder
	assert.MaxAllocs(t, func() { b = frontend.New(kitName, frontendtest.ScriptedLang, syntax) }, newAllocs,
		"New allocates the builder")
	for _, tt := range setters(inner) {
		var got *frontend.Builder
		assert.MaxAllocs(t, func() { got = tt.set(b) }, 0, tt.name+" allocates nothing")
		assert.Equal(t, got, b, tt.name+" returns its builder", assert.ByIdentity())
	}

	fresh := func() *frontend.Builder { return frontend.New(kitName, frontendtest.ScriptedLang, syntax) }
	var appended *frontend.Builder
	assert.MaxAllocsWithSetup(t, fresh, func(b *frontend.Builder) { appended = b.Match(anyScripted) },
		appendAllocs, "Match allocates the claim of a new declaration")
	assert.NotNil(t, appended, "Match returns its builder")
	classify := frontend.Classifier(markTests)
	assert.MaxAllocsWithSetup(t, fresh, func(b *frontend.Builder) { appended = b.Classify(classify) },
		appendAllocs, "Classify allocates the classifier list of a new declaration")
	assert.NotNil(t, appended, "Classify returns its builder")
	register := frontendtest.ScriptedKeys
	assert.MaxAllocsWithSetup(t, fresh, func(b *frontend.Builder) { appended = b.Keys(register) },
		appendAllocs, "Keys allocates the registration list of a new declaration")
	assert.NotNil(t, appended, "Keys returns its builder")

	var built plugin.Frontend
	assert.MaxAllocsWithSetup(t, func() *frontend.Builder { return declaration(inner, false) },
		func(b *frontend.Builder) { built = b.Build() }, buildAllocs, "Build allocates the lowered frontend")
	_, optioned := built.(plugin.OptionsProvider)
	assert.False(t, optioned, "the frontend has no optional role")
	assert.MaxAllocsWithSetup(t, func() *frontend.Builder { return declaration(inner, true) },
		func(b *frontend.Builder) { built = b.Build() }, buildRolesAllocs,
		"Build allocates the lowered frontend and the composition of its roles")
	_, optioned = built.(plugin.OptionsProvider)
	assert.True(t, optioned, "the frontend is in the options role")
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
			assert.Equal(b, got, builder, tt.name+" returns its builder", assert.ByIdentity())
		})
	}

	b.Run("Match", func(b *testing.B) {
		b.Run("a new declaration's first pattern", func(b *testing.B) {
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
	})

	b.Run("Classify", func(b *testing.B) {
		b.Run("a new declaration's first classifier", func(b *testing.B) {
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
	})

	b.Run("Keys", func(b *testing.B) {
		b.Run("a new declaration's first registration", func(b *testing.B) {
			register := frontendtest.ScriptedKeys
			c := bench.Start(b).MaxAllocs(appendAllocs)
			defer c.End()
			var builder *frontend.Builder
			for c.Loop() {
				c.Excluding(func() { builder = frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()) })
				builder.Keys(register)
			}
			assert.NotNil(b, builder, "the registration is declared")
		})
	})

	b.Run("Build", func(b *testing.B) {
		b.Run("a declaration without optional roles", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(buildAllocs)
			defer c.End()
			var (
				builder *frontend.Builder
				got     plugin.Frontend
			)
			for c.Loop() {
				c.Excluding(func() { builder = declaration(inner, false) })
				got = builder.Build()
			}
			_, optioned := got.(plugin.OptionsProvider)
			assert.False(b, optioned, "the frontend has no optional role")
		})

		b.Run("a declaration in every optional role", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(buildRolesAllocs)
			defer c.End()
			var (
				builder *frontend.Builder
				got     plugin.Frontend
			)
			for c.Loop() {
				c.Excluding(func() { builder = declaration(inner, true) })
				got = builder.Build()
			}
			_, exports := got.(plugin.Exporter)
			assert.True(b, exports, "the frontend has every optional role")
		})
	})
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
// brand with the frontend the case built, under the test's context,
// and returns the load's own error.
func loadKit(tb testing.TB, f plugin.Frontend) (*store.Graph, error) {
	tb.Helper()

	g, _, err := load.Load(tb.Context(), load.Config{
		FS:        kitTree(),
		Frontends: []plugin.Frontend{f},
		Sink:      diag.NewSink(),
		Brand:     frontendtest.Brand,
	})
	return g, err
}

// declaration returns a whole declaration of the kit's frontend, in
// every optional role where roles is set and in none otherwise.
func declaration(inner *frontendtest.Scripted, roles bool) *frontend.Builder {
	b := frontend.New(kitName, frontendtest.ScriptedLang, inner.Syntax()).
		Version("1").Match(anyScripted).Units(inner.Partition).Parse(inner.Parse).Resolve(inner.Resolve)
	if roles {
		b.Options(inner.Opts).
			Dependencies(frontendtest.NewScriptedDependent().Dependencies).
			Exports(frontendtest.NewScriptedExporter().Exports).
			Stores(locate)
	}
	return b
}

// locate locates one empty store under the name of the scripted store.
func locate(func(string) string) (map[string]fs.FS, error) {
	return map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{}}, nil
}
