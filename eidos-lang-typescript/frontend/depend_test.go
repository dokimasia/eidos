// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The module specifiers that a round passes over, a file that the release
// does not have, the key of a library whose second root the release does
// not have, and the key of a library whose first root refers to its second.
const (
	relativeNeed  = "./models"
	packageNeed   = "react"
	unknownMember = "lib.missing.d.ts"
	brokenKey     = es2024Root + "+" + unknownMember
	twoRootsKey   = "lib.es2015.d.ts+" + es5Root
)

// roundAllocs is the count of a round of one use whose library the process
// listed before. The round joins and qualifies the path of the root of the
// library, and the read of the root through the reader of the case opens and
// copies the file. The unit of the use allocates the group of its name, the
// list of its members and the joined and qualified path of each of its two
// members. The round allocates the list of its units, and the call boxes the
// reader of the case.
const roundAllocs = 13

// The names of the round cases: the three global names of the module file
// of the release, and a name that no file declares. The module file and
// lib.es2015.iterable.d.ts both declare Iterator and IteratorObject.
const (
	iteratorName            = "Iterator"
	iteratorObjectName      = "IteratorObject"
	iteratorConstructorName = "IteratorConstructor"
	missingName             = "Missing"
	iteratorModule          = "lib.es2025.iterator.d.ts"
	iterableFile            = "lib.es2015.iterable.d.ts"
	errorFile               = "lib.es2022.error.d.ts"
)

// The members of the units of the round cases: the fragments of Error in
// ES5 and in the error library of ES2022, the fragments of Iterator and of
// IteratorObject in the iterable library of ES2015, and the module file,
// each in the default library and in the library of ES2024.
var (
	defaultES5Error       = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+es5Root+"#"+errorName)
	defaultES2022Error    = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+errorFile+"#"+errorName)
	defaultIterator       = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+iterableFile+"#"+iteratorName)
	defaultIteratorObject = plugin.StorePath(
		typescript.LibraryStore,
		defaultRoot+"/"+iterableFile+"#"+iteratorObjectName,
	)
	defaultIteratorModule   = plugin.StorePath(typescript.LibraryStore, defaultRoot+"/"+iteratorModule)
	es2024ES5Error          = plugin.StorePath(typescript.LibraryStore, es2024Root+"/"+es5Root+"#"+errorName)
	es2024ES2022Error       = plugin.StorePath(typescript.LibraryStore, es2024Root+"/"+errorFile+"#"+errorName)
	es2024Iterator          = plugin.StorePath(typescript.LibraryStore, es2024Root+"/"+iterableFile+"#"+iteratorName)
	brokenPackage           = plugin.StorePath(typescript.LibraryStore, brokenKey)
	missingPackage          = plugin.StorePath(typescript.LibraryStore, unknownMember)
	twoRootsPackage         = plugin.StorePath(typescript.LibraryStore, twoRootsKey)
	twoRootsES5Error        = plugin.StorePath(typescript.LibraryStore, twoRootsKey+"/"+es5Root+"#"+errorName)
	defaultIteratorGroup    = []string{defaultIterator, defaultIteratorObject, defaultIteratorModule}
	defaultErrorUnit        = []string{defaultES5Error, defaultES2022Error}
	defaultLibraryNamespace = defaultPackage + "/Intl"
)

// TestDepend pins the dependency round: one unit for each global name that
// the units use, with the statements of the name in each file of the
// library, and the errors of the store.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		embedded := storeTree{fstest.MapFS{}, realStores(t)}
		tests := []struct {
			name      string
			giveTree  fs.FS
			giveRound plugin.DependencyRound
			want      [][]string
		}{
			{
				name:      "returns a unit of the statements of a used name in each file that declares it",
				giveTree:  embedded,
				giveRound: plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{id(defaultPackage, errorName)}},
				want:      [][]string{defaultErrorUnit},
			},
			{
				name:     "returns one unit for a global name of a module file and every other global name of the file",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{
					Number: 1, Uses: []symbol.Identity{id(defaultPackage, iteratorConstructorName)},
				},
				want: [][]string{defaultIteratorGroup},
			},
			{
				name:     "returns the unit of the global names of a module file once for two of its names",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{Number: 2, Uses: []symbol.Identity{
					id(defaultPackage, iteratorName), id(defaultPackage, iteratorObjectName),
				}},
				want: [][]string{defaultIteratorGroup},
			},
			{
				name:     "returns a unit for each used name in the order of the uses",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{Number: 3, Uses: []symbol.Identity{
					id(defaultPackage, errorName), id(defaultPackage, iteratorName),
				}},
				want: [][]string{defaultErrorUnit, defaultIteratorGroup},
			},
			{
				name:     "returns a unit for each library of a used name",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{
					id(es2024Package, errorName), id(defaultPackage, errorName),
				}},
				want: [][]string{{es2024ES5Error, es2024ES2022Error}, defaultErrorUnit},
			},
			{
				name:     "returns the fragment of a root once where the other root of the library refers to it",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{
					Number: 1, Uses: []symbol.Identity{id(twoRootsPackage, errorName)},
				},
				want: [][]string{{twoRootsES5Error}},
			},
			{
				name:     "returns a unit without the module file for a library that does not have the file",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{
					Number: 1, Uses: []symbol.Identity{id(es2024Package, iteratorName)},
				},
				want: [][]string{{es2024Iterator}},
			},
			{
				name:      "does not return a unit for a name that the library does not declare",
				giveTree:  embedded,
				giveRound: plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{id(defaultPackage, missingName)}},
			},
			{
				name:      "does not return a unit for a use of a package that is no library",
				giveTree:  embedded,
				giveRound: plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{id(aPackage, errorName)}},
			},
			{
				name:     "does not return a unit for a use of a namespace of a library",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{
					Number: 1, Uses: []symbol.Identity{id(defaultLibraryNamespace, errorName)},
				},
			},
			{
				name:     "does not return a unit for a need",
				giveTree: embedded,
				giveRound: plugin.DependencyRound{Number: 2, Needs: []plugin.Need{
					{Path: relativeNeed}, {Path: packageNeed}, {Path: defaultPackage},
				}},
			},
			{
				name:      "does not return a unit for a shared input of a library",
				giveTree:  embedded,
				giveRound: plugin.DependencyRound{Number: 1, Shared: []string{defaultInput}},
			},
			{
				name:      "does not return a unit for a load without the library store",
				giveTree:  storeTree{fstest.MapFS{}, nil},
				giveRound: plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{id(defaultPackage, errorName)}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				units, err := runRound(t, roundReader{tt.giveTree}, &tt.giveRound)
				assert.NoError(t, err, "the round places its units")
				assert.Equal(t, membersOf(units), tt.want, "the members of each unit in the order of typescript-go",
					assert.EquateEmpty())
				assert.Empty(t, tt.giveRound.Unplaced(), "the round reports no need")
			})
		}

		t.Run("reads the first root of a library once for its uses", func(t *testing.T) {
			t.Parallel()

			r := countingStoreReader{tree: embedded, reads: map[string]int{}}
			round := &plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{
				id(defaultPackage, errorName), id(defaultPackage, iteratorName),
			}}
			_, err := runRound(t, r, round)
			assert.NoError(t, err, "the round places its units")
			assert.Equal(t, r.reads, map[string]int{defaultInput: 1}, "the round reads the root alone")
		})

		broken := []struct {
			name     string
			giveTree fs.FS
			giveUse  symbol.Identity
			want     error
		}{
			{
				name: "returns the error of the store for a root of a library that does not read",
				giveTree: storeTree{
					fstest.MapFS{},
					map[string]fs.FS{
						typescript.LibraryStore: failingFS{
							realStores(t)[typescript.LibraryStore],
							defaultRoot + "/" + defaultRoot,
						},
					},
				},
				giveUse: id(defaultPackage, errorName),
				want:    errBroken,
			},
			{
				name:     "returns an error for a library whose root the store does not have",
				giveTree: embedded,
				giveUse:  id(missingPackage, errorName),
				want:     fs.ErrNotExist,
			},
			{
				name:     "returns an error for a library whose second root the release does not have",
				giveTree: embedded,
				giveUse:  id(brokenPackage, errorName),
				want:     fs.ErrNotExist,
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				round := &plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{tt.giveUse}}
				_, err := runRound(t, roundReader{tt.giveTree}, round)
				assert.ErrorIs(t, err, tt.want, "the round returns the failure of the read")
			})
		}
	})
}

// A round of one use allocates its unit and the read of the root of its
// library. The ordinary run, which runs no benchmark, checks that ceiling
// here.
func TestDependAllocs(t *testing.T) {
	checkAllocs(t, dependCalls(t))
}

// BenchmarkDepend measures a round of one use, which a load of the library
// makes for each round of its uses.
func BenchmarkDepend(b *testing.B) {
	benchCalls(b, dependCalls(b))
}

// dependCalls returns a round of the use of Error in the default library,
// whose files the first call lists for the process.
func dependCalls(tb testing.TB) []allocCall {
	tb.Helper()

	dependent, depends := frontend.New().(plugin.Dependent)
	assert.True(tb, depends, "the TypeScript frontend is a dependent")
	ctx := assert.Context(tb)
	r := roundReader{storeTree{fstest.MapFS{}, realStores(tb)}}
	round := &plugin.DependencyRound{Number: 1, Uses: []symbol.Identity{id(defaultPackage, errorName)}}
	var (
		units [][]plugin.SourceRef
		err   error
	)
	return []allocCall{
		{
			name: "Dependencies", allocs: roundAllocs,
			call: func() { units, err = dependent.Dependencies(ctx, round, r) },
			check: func(tb testing.TB) {
				tb.Helper()

				assert.NoError(tb, err, "the round places its unit")
				assert.Equal(
					tb,
					membersOf(units),
					[][]string{defaultErrorUnit},
					"Dependencies places the unit of Error",
				)
			},
		},
	}
}
