// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The scripted tree of the source cases. Its first package declares User
// under the table directive of the gen plugin, and its file has the stamp
// of a test. Its second package declares Row.
const (
	userFile      = "svc/api/user.zz"
	rowFile       = "svc/store/row.zz"
	apiPackage    = "svc/api"
	storePackage  = "svc/store"
	absentPackage = "svc/absent"
	userSource    = "package svc/api\nstamp fake.testFile yes\ntype User string\n+gen:table name=users\n"
	rowSource     = "package svc/store\ntype Row int string\n"
	stampValue    = "yes"
	tableName     = "gen:table"
	genID         = "gen"
	sealedKey     = "late.key"
	sealedDoc     = "a key that a test registers after the seal"
)

// The scripted tree of the dependency cases. Its package app imports the
// package ext/lib, which a dependency round loads from the store of the
// scripted language.
const (
	appFile    = "app/main.zz"
	appPackage = "app"
	appSource  = "package app\nimport ext " + libPackage + "\ntype App ext.Lib\n"
	libFile    = "ext/lib/lib.zz"
	libPackage = "ext/lib"
	libSource  = "package " + libPackage + "\ntype Lib string\n"
)

// The plugins of the composition case. The plugin owner provides the
// capability that the plugin gating runs after.
const (
	ownerID  plugin.ID = "owner"
	gatingID plugin.ID = "gating"
)

// stamping is a hand-rolled generator that provides the capability
// absentCap in the annotator role alone, which it does not implement.
type stamping struct{}

// Name returns the name owner.
func (stamping) Name() plugin.ID { return ownerID }

// Generate emits nothing.
func (stamping) Generate(*plugin.GeneratorContext) error { return nil }

// Priority places every role at priority zero.
func (stamping) Priority(plugin.Role) int { return 0 }

// Provides returns absentCap in the annotator role, and no capability in
// another role.
func (stamping) Provides(r plugin.Role) []plugin.Capability {
	if r == plugin.RoleAnnotator {
		return []plugin.Capability{absentCap}
	}
	return nil
}

// Requires returns no capability.
func (stamping) Requires() []plugin.Capability { return nil }

// After returns no capability.
func (stamping) After() []plugin.Capability { return nil }

// keyedFront is the scripted frontend that registers its classification
// key through its own role.
type keyedFront struct{ *frontendtest.Scripted }

// Keys registers the scripted classification key.
func (keyedFront) Keys(r *meta.Registry) error { return frontendtest.ScriptedKeys(r) }

// tabling is the plugin that declares the table directive of the scripted
// language.
type tabling struct{}

// Name returns the name of the plugin, gen.
func (tabling) Name() plugin.ID { return genID }

// Directives returns the table directive.
func (tabling) Directives() []directive.Schema { return frontendtest.ScriptedSchemas() }

// A plugin author tests a plugin over real source, so Loaded loads a tree,
// stamps it and validates its directives as a workspace does before the
// first phase call.
func TestSource(t *testing.T) {
	t.Parallel()

	t.Run("Loaded", func(t *testing.T) {
		t.Parallel()

		t.Run("adds every package of the load to the graph", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, source(nil))
			assert.Equal(t, packagesOf(f), []string{apiPackage, storePackage}, "the graph has both packages")
		})

		t.Run("adds the packages that the source names to the graph", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, source([]string{storePackage}))
			assert.Equal(t, packagesOf(f), []string{storePackage}, "the graph has the named package")
		})

		t.Run("adds each dependency package of the load beside the packages that the source names", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, dependentSource([]string{appPackage}))
			assert.Equal(t, packagesOf(f), []string{appPackage, libPackage},
				"the graph has the named package and the package of the store")
		})

		dependencies := []struct {
			name string
			give string
			want bool
		}{
			{name: "marks a dependency package of the load as a dependency", give: libPackage, want: true},
			{name: "marks no package of the tree as a dependency", give: appPackage},
		}
		for _, tt := range dependencies {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := plugintest.Loaded(t, dependentSource([]string{appPackage}))
				f.Graph.Freeze()
				pkg := symbol.Identity{Lang: frontendtest.ScriptedLang, Package: tt.give, Kind: symbol.KindPackage}
				assert.Equal(t, f.Graph.Dependency(pkg), tt.want,
					"the fixture marks the packages of a dependency round")
			})
		}

		t.Run("files the classification stamps of the load", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, source(nil))
			key, registered := meta.Lookup[string](f.Keys, frontendtest.ScriptedTestKey)
			assert.True(t, registered, "the frontend registers its key")
			got, held := meta.Get(f.Facts, fileID(t, f, userFile), key)
			assert.True(t, held, "the stamp of the file applies")
			assert.Equal(t, got, stampValue, "the fact has the value of the stamp")
		})

		t.Run("validates the directives of the plugins", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, source(nil))
			user := symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPackage, Name: "User", Kind: symbol.KindStruct,
			}
			assert.Length(t, f.Directives[user], 1, "User has one validated directive")
			assert.Equal(t, f.Directives[user][0].Name, directive.Name(tableName), "the directive is the table")
		})

		t.Run("seals the key registry", func(t *testing.T) {
			t.Parallel()

			f := plugintest.Loaded(t, source(nil))
			_, err := meta.Register[bool](f.Keys, meta.KeySpec{Name: sealedKey, Doc: sealedDoc})
			assert.HasError(t, err, "a key registers in no sealed registry")
		})

		t.Run("fails the test for a gate on a namespace that nothing claims where the source provides the capability",
			func(t *testing.T) {
				t.Parallel()

				failure := assert.Rejects(t, "a gate on a namespace that nothing claims must fail the test",
					func(tb assert.TB) {
						s := source(nil)
						s.Plugins = append(s.Plugins, eidos.NewPlugin(ownerID).
							Provides(absentCap).
							Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error { return nil })).
							Annotator())
						p := eidos.NewPlugin(gatingID).
							After(absentCap).
							Handle(eidos.Where(eidos.HasKey(meta.Named[bool](absentKey)),
								eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error { return nil }))).
							Annotator()
						plugintest.Loaded(tb, s).Annotate(tb, p)
					})
				assert.Equal(t, coretest.Contracts(failure), []string{"the gates of the plugin on named handles bind"},
					"the failure names the bind")
			})

		t.Run("binds a gate on a namespace that nothing claims where its provider lacks the role of the capability",
			func(t *testing.T) {
				t.Parallel()

				s := source(nil)
				s.Plugins = append(s.Plugins, stamping{})
				var visited []string
				p := eidos.NewPlugin(gatingID).
					After(absentCap).
					Handle(eidos.Where(eidos.HasKey(meta.Named[bool](absentKey)),
						eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Stamper) error {
							visited = append(visited, m.Struct.Name)
							return nil
						}))).
					Annotator()
				assert.NoError(t, plugintest.Loaded(t, s).Annotate(t, p).Err, "the phase call passes")
				assert.Empty(t, visited, "the gate does not select a subject")
			})

		t.Run("stops the test where the load returns an error", func(t *testing.T) {
			t.Parallel()

			s := source(nil)
			s.Tree = fstest.MapFS{}
			s.Stores = map[string]fs.FS{"go:mod": fstest.MapFS{}}
			records := assert.Rejects(t, "a store that no frontend loads", func(tb assert.TB) {
				plugintest.Loaded(tb, s)
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the source tree loads"},
				"the rejection names the load")
		})

		t.Run("fails an Error of the load", func(t *testing.T) {
			t.Parallel()

			s := source(nil)
			s.Tree = fstest.MapFS{rowFile: {Data: []byte("type Lost string\n")}}
			records := assert.Rejects(t, "a file without a package line", func(tb assert.TB) {
				plugintest.Loaded(tb, s)
			})
			assert.NotEmpty(t, records, "the check fails")
			assert.HasPrefix(t, records[0].Contract, "the load of the source tree reports no Error, and reports "+
				frontendtest.ScriptedBadFile.String(), "the rejection names the finding of the load")
		})

		t.Run("fails an Error of the validation", func(t *testing.T) {
			t.Parallel()

			s := source(nil)
			s.Tree = fstest.MapFS{userFile: {Data: []byte("package svc/api\ntype User string\n+gen:table wrong=x\n")}}
			records := assert.Rejects(t, "a directive with a key that its schema does not declare", func(tb assert.TB) {
				plugintest.Loaded(tb, s)
			})
			assert.NotEmpty(t, records, "the check fails")
			assert.HasPrefix(t, records[0].Contract, "the validation of the directives reports no Error, and reports ",
				"the rejection names the validation")
		})

		t.Run("stops the test where the load does not declare a named package", func(t *testing.T) {
			t.Parallel()

			records := assert.Rejects(t, "a source that names a package of no file", func(tb assert.TB) {
				plugintest.Loaded(tb, source([]string{apiPackage, absentPackage}))
			})
			assert.Equal(t, coretest.Contracts(records), []string{"the load declares the package " + absentPackage},
				"the rejection names the package")
		})
	})
}

// source returns the scripted source of the cases, which contains the
// packages in packages.
func source(packages []string) plugintest.Source {
	return plugintest.Source{
		Tree: fstest.MapFS{
			userFile: {Data: []byte(userSource)},
			rowFile:  {Data: []byte(rowSource)},
		},
		Brand:     frontendtest.Brand,
		Frontends: []plugin.Frontend{keyedFront{frontendtest.NewScripted()}},
		Plugins:   []plugin.Plugin{tabling{}},
		Packages:  packages,
	}
}

// dependentSource returns the scripted source of the dependency cases,
// which contains the packages in packages and the store of the scripted
// language with the package ext/lib.
func dependentSource(packages []string) plugintest.Source {
	return plugintest.Source{
		Tree: fstest.MapFS{appFile: {Data: []byte(appSource)}},
		Stores: map[string]fs.FS{
			frontendtest.ScriptedStore: fstest.MapFS{libFile: {Data: []byte(libSource)}},
		},
		Brand:     frontendtest.Brand,
		Frontends: []plugin.Frontend{frontendtest.NewScriptedDependent()},
		Packages:  packages,
	}
}

// packagesOf seals the graph of f, as the first phase call does, and
// returns the path of each package of the graph in the order of the graph.
func packagesOf(f *plugintest.Fixture) []string {
	f.Graph.Freeze()
	var out []string
	for pkg := range f.Graph.Packages() {
		out = append(out, pkg.ID.Package)
	}
	return out
}

// fileID seals the graph of f, as the first phase call does, and returns
// the identity of the file at path in the graph. It stops the test where
// the graph has no such file.
func fileID(tb assert.TB, f *plugintest.Fixture, path string) symbol.Identity {
	tb.Helper()

	f.Graph.Freeze()
	var found []*node.File
	for pkg := range f.Graph.Packages() {
		for _, file := range pkg.Files {
			if file.Path == path {
				found = append(found, file)
			}
		}
	}
	assert.Length(tb, found, 1, "the graph has the file "+path)
	return found[0].ID
}
