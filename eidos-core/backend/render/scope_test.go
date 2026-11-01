// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"runtime"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// keptScope is the names of the kept files of a render: InFileScope
// returns every entry for every namespace, and the other lookups find
// nothing.
type keptScope []plugin.NameEntry

// InPackage finds nothing.
func (keptScope) InPackage(string, string) (plugin.NameEntry, bool) { return plugin.NameEntry{}, false }

// OfOrigin finds nothing.
func (keptScope) OfOrigin(symbol.Identity, string) (plugin.NameEntry, bool) {
	return plugin.NameEntry{}, false
}

// InScope finds nothing.
func (keptScope) InScope(string, string) []plugin.NameEntry { return nil }

// InFileScope returns every entry.
func (k keptScope) InFileScope(symbol.Identity, string) []plugin.NameEntry { return k }

// TestScope checks the package scope of a language whose files of one
// package share one scope. The imports of a file keep off the names that
// the other files of its package declare.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		// declaredStore returns the context edit that gives the package of
		// examplePkg one source file, at path, that declares storeLocal.
		declaredStore := func(at string) func(*plugin.RenderContext) {
			return func(ctx *plugin.RenderContext) {
				ctx.Declarations = func(p string, yield func(string, []symbol.Symbol)) {
					if p == examplePkg {
						yield(at, []symbol.Symbol{coretest.Struct(examplePkg, storeLocal)})
					}
				}
			}
		}
		// declared gives the package of examplePkg one source file that
		// declares storeLocal.
		declared := declaredStore(coretest.UnitFile)

		t.Run("keeps the imports of a file off a name that a source file of its package declares", func(t *testing.T) {
			t.Parallel()

			files := renderScoped(t, scopedLanguage(), declared, scopedUnit(storeKey, alphaName))
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal2+"\n"+"type "+alphaName+" "+storeLocal2+"."+rowName+"\n",
				"the import binds around the declaration of the other file")
		})

		t.Run("keeps the imports of a file off a name that another routed file of its package declares",
			func(t *testing.T) {
				t.Parallel()

				files := renderScoped(t, scopedLanguage(), nil,
					scopedUnit(storeKey, alphaName), scopedUnit(userKey, storeLocal))
				assert.Equal(t, string(files[0].Body),
					"use "+storePkg+" as "+storeLocal2+"\n"+"type "+alphaName+" "+storeLocal2+"."+rowName+"\n",
					"the import binds around the struct of the other file")
			})

		t.Run("keeps the imports of every file of a package off a name that one routed file declares",
			func(t *testing.T) {
				t.Parallel()

				// More files than workers, so the workers render the files of
				// the package concurrently.
				n := runtime.GOMAXPROCS(0) + 1
				units := make([]plugin.Unit, 0, n+1)
				units = append(units, scopedUnit(storeKey, storeLocal))
				for i := range n {
					units = append(units, scopedUnit(strconv.Itoa(i)+".go", alphaName))
				}
				files := renderScoped(t, scopedLanguage(), nil, units...)
				assert.Length(t, files, n+1, "one file per unit")
				for _, f := range files {
					assert.HasPrefix(t, string(f.Body), "use "+storePkg+" as "+storeLocal2+"\n",
						f.Path+" binds the import around the struct of the store file")
				}
			})

		t.Run("keeps the imports of a file off a name that a kept file of its package declares", func(t *testing.T) {
			t.Parallel()

			kept := func(ctx *plugin.RenderContext) {
				ctx.Others = keptScope{{Package: examplePkg, Emitted: storeLocal, Settled: storeLocal}}
			}
			files := renderScoped(t, scopedLanguage(), kept, scopedUnit(storeKey, alphaName))
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal2+"\n"+"type "+alphaName+" "+storeLocal2+"."+rowName+"\n",
				"the import binds around the name of the kept file")
		})

		t.Run("keeps the imports of a claimed file off a name that a source file of its package declares",
			func(t *testing.T) {
				t.Parallel()

				u := claimedUnit(handleName, nil, emit.Body{})
				u.Key = examplePkg + "/" + u.Key
				u.Pkg = coretest.PackageID(examplePkg)
				l := scopedLanguage()
				l.TemplateImport = lastSegment
				claim := func(ctx *plugin.RenderContext) {
					declared(ctx)
					ctx.Trees = claimTree(importOf(storePkg) + "." + rowName)
				}
				files := renderScoped(t, l, claim, u)
				assert.HasSuffix(t, string(files[0].Body), "\n"+storeLocal2+"."+rowName,
					"the template import binds around the declaration of the other file")
			})

		t.Run("binds a name that a record of a claimed file of its package declares", func(t *testing.T) {
			t.Parallel()

			claimed := claimedUnit(storeLocal, nil, emit.Body{})
			claimed.Key = examplePkg + "/" + claimed.Key
			claimed.Pkg = coretest.PackageID(examplePkg)
			trees := func(ctx *plugin.RenderContext) { ctx.Trees = claimTree("run\n") }
			files := renderScoped(t, scopedLanguage(), trees, claimed, scopedUnit(userKey, alphaName))
			assert.Length(t, files, 2, "one file per unit")
			assert.HasSuffix(t, files[1].Path, userFile, "the file of the struct sorts after the claimed file")
			assert.HasPrefix(t, string(files[1].Body), "use "+storePkg+" as "+storeLocal+"\n",
				"the template of the claimed file declares names that the render does not know")
		})

		t.Run("binds a name that a source file declares for a language without a package scope", func(t *testing.T) {
			t.Parallel()

			l := scopedLanguage()
			l.PackageScope = nil
			files := renderScoped(t, l, declared, scopedUnit(storeKey, alphaName))
			assert.HasPrefix(t, string(files[0].Body), "use "+storePkg+" as "+storeLocal+"\n",
				"each file of the language is a namespace of its own")
		})

		t.Run("binds a name that the source file at the path of the routed file declares", func(t *testing.T) {
			t.Parallel()

			files := renderScoped(t, scopedLanguage(), declaredStore(examplePkg+"/"+storeFile),
				scopedUnit(storeKey, alphaName))
			assert.HasPrefix(t, string(files[0].Body), "use "+storePkg+" as "+storeLocal+"\n",
				"the run writes the file again, so its earlier declarations are gone")
		})

		t.Run("binds the name of an import in a package without a source file", func(t *testing.T) {
			t.Parallel()

			unloaded := func(ctx *plugin.RenderContext) {
				ctx.Declarations = func(string, func(string, []symbol.Symbol)) {}
			}
			files := renderScoped(t, scopedLanguage(), unloaded, scopedUnit(storeKey, alphaName))
			assert.HasPrefix(t, string(files[0].Body), "use "+storePkg+" as "+storeLocal+"\n",
				"no source file of the package declares the name")
		})

		t.Run("binds a name that another file without a package declares", func(t *testing.T) {
			t.Parallel()

			files := renderScoped(t, scopedLanguage(), nil,
				unitOf(emitter, storeKey, alphaName), unitOf(emitter, userKey, storeLocal))
			assert.HasPrefix(t, string(files[0].Body), "use "+storePkg+" as "+storeLocal+"\n",
				"a file without a package shares no namespace")
		})

		t.Run("reports the package scope of a file that binds an import", func(t *testing.T) {
			t.Parallel()

			files := renderScoped(t, scopedLanguage(), declared, scopedUnit(storeKey, alphaName))
			assert.True(t, files[0].PackageScope, "the names of the other files decide the imports of the file")
		})

		t.Run("reports the package scope of a file whose package scope has no name", func(t *testing.T) {
			t.Parallel()

			files := renderScoped(t, scopedLanguage(), nil, scopedUnit(storeKey, alphaName))
			assert.True(t, files[0].PackageScope, "a later file of the package can take the name of an import")
		})

		t.Run("reports no package scope for a file that binds no import", func(t *testing.T) {
			t.Parallel()

			l := scopedLanguage()
			l.Kinds[symbol.KindStruct] = structTpl
			files := renderScoped(t, l, declared, scopedUnit(storeKey, alphaName))
			assert.False(t, files[0].PackageScope, "the file binds no name that another file could take")
		})

		t.Run("reports no package scope for a language without one", func(t *testing.T) {
			t.Parallel()

			l := scopedLanguage()
			l.PackageScope = nil
			files := renderScoped(t, l, declared, scopedUnit(storeKey, alphaName))
			assert.False(t, files[0].PackageScope, "the files of the language share no namespace")
		})
	})
}

// scopedLanguage returns the [binding] language that spells
// qualifiedStruct, with [declaredNames] as its package scope.
func scopedLanguage() render.Language {
	l := binding()
	l.Kinds[symbol.KindStruct] = qualifiedStruct
	l.PackageScope = declaredNames
	return l
}

// declaredNames is the package scope of the fixture language: the name of
// the identity of each source declaration.
func declaredNames(d symbol.Symbol, yield func(name string)) {
	if decl, named := d.(interface{ Identity() symbol.Identity }); named {
		yield(decl.Identity().Name)
	}
}

// scopedUnit returns a unit of the package examplePkg that emits one
// struct of name in the package's directory, with key as its routing key.
func scopedUnit(key, name string) plugin.Unit {
	u := unitOf(emitter, examplePkg+"/"+key, name)
	u.Pkg = coretest.PackageID(examplePkg)
	return u
}

// renderScoped renders the units through l, routed as the layout routes
// them, with the namespace of each file set to its package. edit changes
// the context before the render, and nil leaves it as it is. It returns
// the rendered files, and fails the test where the render reports a
// finding.
func renderScoped(
	tb assert.TB, l render.Language, edit func(*plugin.RenderContext), units ...plugin.Unit,
) []plugin.RenderedFile {
	tb.Helper()

	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	e := seeded(tb, units...)
	routed := backendtest.Files(e, p)
	for i := range routed {
		routed[i].Scope = routed[i].Pkg
	}
	sink := diag.NewSink()
	ctx := &plugin.RenderContext{Emit: e, Files: routed, Sink: sink, Plugin: passName}
	if edit != nil {
		edit(ctx)
	}
	files, err := p.Render(ctx)
	assert.NoError(tb, err, "the pass renders every file")
	coretest.AssertCodes(tb, sink)
	assert.NotEmpty(tb, files, "the units render")
	return files
}
