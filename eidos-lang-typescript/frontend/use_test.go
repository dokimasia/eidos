// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The global names of the library that the use cases name, and a name that
// no file of the library declares.
const (
	mapName            = "Map"
	dateName           = "Date"
	intlName           = "Intl"
	promiseName        = "Promise"
	mapConstructorName = "MapConstructor"
	shapeName          = "Shape"
	intlFile           = "Intl/a.ts"
)

// The fragment of Map in the collection library of ES2015, inside the
// default library.
var defaultCollectionMap = plugin.StorePath(typescript.LibraryStore,
	defaultRoot+"/lib.es2015.collection.d.ts#"+mapName)

// A unit uses each global name of its library that a reference of the unit
// can resolve to, so a dependency round places the declarations that the
// load needs.
func TestUse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			giveSrc string
			want    []symbol.Identity
		}{
			{
				name:    "records a use of a bare name that the library declares",
				giveSrc: "export let m: Map<string, number>;\n",
				want:    []symbol.Identity{id(defaultPackage, mapName)},
			},
			{
				name:    "records a use of the first name of a qualified name",
				giveSrc: "export let f: Intl.NumberFormatOptions;\n",
				want:    []symbol.Identity{id(defaultPackage, intlName)},
			},
			{
				name:    "records a use of the name after globalThis",
				giveSrc: "export let e: globalThis.Error;\n",
				want:    []symbol.Identity{id(defaultPackage, errorName)},
			},
			{
				name:    "records a use of the first name of the entity of an import alias",
				giveSrc: "import I = Intl;\nexport let f: I.NumberFormatOptions;\n",
				want:    []symbol.Identity{id(defaultPackage, intlName)},
			},
			{
				name:    "records a use of each name of a nested type argument",
				giveSrc: "export let m: Map<string, Set<Date>>;\n",
				want: []symbol.Identity{
					id(defaultPackage, dateName), id(defaultPackage, mapName), id(defaultPackage, setName),
				},
			},
			{
				name:    "records a use of a name inside an array type",
				giveSrc: "export let dates: Date[];\n",
				want:    []symbol.Identity{id(defaultPackage, dateName)},
			},
			{
				name:    "records a use of the name of a superclass",
				giveSrc: "export class Failure extends Error {}\n",
				want:    []symbol.Identity{id(defaultPackage, errorName)},
			},
			{
				name:    "records a use of a name in a namespace of the file",
				giveSrc: "export namespace N {\n  export let m: Map<string, string>;\n}\n",
				want:    []symbol.Identity{id(defaultPackage, mapName)},
			},
			{
				name:    "records a use of a name of a script",
				giveSrc: "declare let m: Map<string, string>;\n",
				want:    []symbol.Identity{id(defaultPackage, mapName)},
			},
			{
				name:    "does not record a use of a name that a named import binds",
				giveSrc: "import { Map } from 'immutable';\nexport let m: Map<string, number>;\n",
			},
			{
				name:    "does not record a use of a name that a namespace import binds",
				giveSrc: "import * as Intl from 'intl';\nexport let f: Intl.NumberFormatOptions;\n",
			},
			{
				name:    "does not record a use of a name that the module declares",
				giveSrc: "interface Event {}\nexport let e: Event;\n",
			},
			{
				name:    "does not record a use of a name that a script declares",
				giveSrc: "interface Event {}\ndeclare let e: Event;\n",
			},
			{
				name:    "does not record a use of a name that a variable of the module declares",
				giveSrc: "declare const Event: number;\nexport let e: Event;\n",
			},
			{
				name:    "does not record a use of a name that the namespace of the reference declares",
				giveSrc: "export namespace N {\n  export interface Map {}\n  export let m: Map;\n}\n",
			},
			{
				name:    "does not record a use of a name that the library does not declare",
				giveSrc: "export let s: Shape;\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := usesOf(t, fstest.MapFS{aFile: {Data: []byte(tt.giveSrc)}})
				assert.Equal(t, got, tt.want, "the uses of the unit", assert.EquateEmpty())
			})
		}

		t.Run("records a use in the package of the library that a directive extends", func(t *testing.T) {
			t.Parallel()

			got := usesOf(t, fstest.MapFS{
				rootConfig: {Data: []byte(es2024Config)},
				aFile:      {Data: []byte(domDirective + "export let e: Event;\n")},
			})
			assert.Equal(
				t,
				got,
				[]symbol.Identity{id(domES2024Package, eventName)},
				"the use names the extended library",
			)
		})

		t.Run("does not record a use of the first name of the package of the file", func(t *testing.T) {
			t.Parallel()

			tree := storeTree{fstest.MapFS{intlFile: {Data: []byte("export let s: Shape;\n")}}, realStores(t)}
			units := partitionedThrough(t, tree, intlFile)
			f := frontend.New()
			u := plugin.NewSourceUnit(units[0], tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			assert.Empty(t, u.Uses(), "the package Intl/a of the file is no package of the library")
		})

		t.Run("does not record a use for a file without a library", func(t *testing.T) {
			t.Parallel()

			got := usesOf(t, fstest.MapFS{
				rootConfig: {Data: []byte(`{"compilerOptions": {"noLib": true}}`)},
				aFile:      {Data: []byte("export let m: Map<string, number>;\n")},
			})
			assert.Empty(t, got, "a file without a library uses nothing")
		})

		units := []struct {
			name    string
			giveES5 string
			giveDOM string
			want    []symbol.Identity
		}{
			{
				name:    "records a use of a name that a member of a library unit references",
				giveES5: "interface Error {\n  cause: Promise<string>;\n}\n",
				giveDOM: "interface Event {\n  type: string;\n}\n",
				want:    []symbol.Identity{id(defaultPackage, promiseName)},
			},
			{
				name:    "does not record a use of a name that another member of the unit declares",
				giveES5: "interface Error {\n  event: Event;\n}\n",
				giveDOM: "interface Event {\n  error: Error;\n}\n",
			},
		}
		for _, tt := range units {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				library := fstest.MapFS{
					defaultRoot + "/" + es5Root: {Data: []byte(tt.giveES5)},
					defaultRoot + "/" + domRoot: {Data: []byte(tt.giveDOM)},
				}
				tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
				got := libraryUsesOf(t, tree, defaultES5, defaultDOM)
				assert.Equal(t, got, tt.want, "the uses of the library unit", assert.EquateEmpty())
			})
		}

		t.Run("records a use of a name that the global block of a module file references", func(t *testing.T) {
			t.Parallel()

			module := plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+iteratorModule)
			library := fstest.MapFS{defaultRoot + "/" + iteratorModule: {Data: []byte(
				"export {};\ndeclare class Hidden {}\ndeclare global {\n  interface IteratorObject {\n" +
					"    h: Hidden;\n    p: Promise<string>;\n  }\n}\n",
			)}}
			tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
			got := libraryUsesOf(t, tree, module)
			assert.Equal(t, got, []symbol.Identity{id(defaultPackage, promiseName)}, "the global block uses Promise")
		})

		t.Run("records the uses of a fragment of the embedded library", func(t *testing.T) {
			t.Parallel()

			got := libraryUsesOf(t, storeTree{fstest.MapFS{}, realStores(t)}, defaultCollectionMap)
			expect.That(t, got).
				Contains(id(defaultPackage, mapConstructorName), "Map uses its constructor").
				NotContains(id(defaultPackage, mapName), "the unit declares Map").
				NotContains(id(defaultPackage, shapeName), "the library declares no Shape")
		})
	})
}

// usesOf partitions src/a.ts of a workspace beside the embedded library,
// parses its unit at full depth and returns the uses of the unit.
func usesOf(tb testing.TB, workspace fstest.MapFS) []symbol.Identity {
	tb.Helper()

	tree := storeTree{workspace, realStores(tb)}
	units := partitionedThrough(tb, tree, aFile)
	f := frontend.New()
	u := plugin.NewSourceUnit(units[0], tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return u.Uses()
}

// libraryUsesOf parses a unit of the members of a library at signature
// depth over a tree and its stores, and returns the uses of the unit.
func libraryUsesOf(tb testing.TB, tree storeTree, members ...string) []symbol.Identity {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(members))
	for _, m := range members {
		refs = append(refs, plugin.SourceRef{Path: m})
	}
	f := frontend.New()
	u := plugin.NewSourceUnit(refs, tree, plugin.DepthSignatures, f.Syntax(), brand, diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the library parses")
	return u.Uses()
}
