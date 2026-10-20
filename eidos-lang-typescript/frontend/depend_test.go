// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The key of a library that a source directive extends with the DOM, its
// package, and the module specifiers that a round passes over.
const (
	extendedKey   = defaultRoot + "+" + domRoot
	relativeNeed  = "./models"
	packageNeed   = "react"
	unknownMember = "lib.missing.d.ts"
)

// The members of the fake library of the default key in the order in which
// typescript-go loads them: ES5, the DOM, then the root.
var defaultMembers = []string{defaultES5, defaultDOM, defaultInput}

// TestDepend pins the dependency round: one unit for each library of the
// shared inputs and of the needs, its members in the order of typescript-go,
// and the errors of the store.
func TestDepend(t *testing.T) {
	t.Parallel()

	t.Run("dependencies", func(t *testing.T) {
		t.Parallel()

		extended := fakeLibrary()
		extended[extendedKey+"/"+defaultRoot] = &fstest.MapFile{Data: []byte(fakeRoot)}
		extended[extendedKey+"/"+es5Root] = &fstest.MapFile{Data: []byte(fakeES5)}
		extended[extendedKey+"/"+domRoot] = &fstest.MapFile{Data: []byte(fakeDOM)}
		extendedPackage := plugin.StorePath(typescript.LibraryStore, extendedKey)
		unterminated := fakeLibrary()
		unterminated[defaultRoot+"/"+defaultRoot] = &fstest.MapFile{Data: []byte(strings.TrimSuffix(fakeRoot, "\n"))}
		tests := []struct {
			name       string
			giveTree   fs.FS
			giveShared []string
			giveNumber int
			giveNeeds  []string
			want       [][]string
		}{
			{
				name:     "returns the library of a key of the shared inputs in the first round",
				giveTree: withLibrary(fstest.MapFS{}), giveShared: []string{defaultInput}, giveNumber: 1,
				want: [][]string{defaultMembers},
			},
			{
				name:     "returns the library of a key once for two shared inputs of it",
				giveTree: withLibrary(fstest.MapFS{}), giveShared: []string{defaultInput, defaultES5}, giveNumber: 1,
				want: [][]string{defaultMembers},
			},
			{
				name:       "returns the library of a root whose last directive ends the file",
				giveTree:   storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: unterminated}},
				giveShared: []string{defaultInput}, giveNumber: 1,
				want: [][]string{defaultMembers},
			},
			{
				name:     "does not return a library of the shared inputs after the first round",
				giveTree: withLibrary(fstest.MapFS{}), giveShared: []string{defaultInput}, giveNumber: 2,
			},
			{
				name:       "returns the library of a need of the package of a library",
				giveTree:   storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: extended}},
				giveNumber: 2, giveNeeds: []string{extendedPackage},
				want: [][]string{{
					plugin.StorePath(typescript.LibraryStore, extendedKey+"/"+es5Root),
					plugin.StorePath(typescript.LibraryStore, extendedKey+"/"+domRoot),
					plugin.StorePath(typescript.LibraryStore, extendedKey+"/"+defaultRoot),
				}},
			},
			{
				name:     "does not return a unit for a need of a module specifier",
				giveTree: withLibrary(fstest.MapFS{}), giveNumber: 1, giveNeeds: []string{relativeNeed, packageNeed},
			},
			{
				name:     "does not return a unit for a need of a file of a library",
				giveTree: withLibrary(fstest.MapFS{}), giveNumber: 1, giveNeeds: []string{defaultInput},
			},
			{
				name:     "does not return a library for a load without the library store",
				giveTree: storeTree{fstest.MapFS{}, nil}, giveShared: []string{defaultInput}, giveNumber: 1,
			},
			{
				name:     "does not return a unit for a shared input of the workspace",
				giveTree: withLibrary(fstest.MapFS{}), giveShared: []string{rootConfig}, giveNumber: 1,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				units, round, err := runRound(t, tt.giveTree, tt.giveShared, tt.giveNumber, tt.giveNeeds...)
				assert.NoError(t, err, "the round places its libraries")
				assert.Equal(t, membersOf(units), tt.want, "the members of each library in the order of typescript-go",
					assert.EquateEmpty())
				assert.Empty(t, round.Unplaced(), "the round reports no need")
			})
		}

		roots := []struct {
			name string
			give string
		}{
			{
				name: "returns a library without a file that a directive of a member names and the release does not have",
				give: "/// <reference lib=\"esnext.unknown\" />\n",
			},
			{
				name: "returns a library without the files of the directives in a block comment that does not close",
				give: "/* The licence.\n/// <reference lib=\"es5\" />\n",
			},
		}
		for _, tt := range roots {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				library := fstest.MapFS{defaultRoot + "/" + defaultRoot: {Data: []byte(tt.give)}}
				tree := storeTree{fstest.MapFS{}, map[string]fs.FS{typescript.LibraryStore: library}}
				units, _, err := runRound(t, tree, []string{defaultInput}, 1)
				assert.NoError(t, err, "the round places the library")
				assert.Equal(t, membersOf(units), [][]string{{defaultInput}}, "the root is the one member")
			})
		}

		broken := []struct {
			name     string
			giveTree fs.FS
		}{
			{
				name: "returns the error of the store for a root of a library that does not read",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					typescript.LibraryStore: failingFS{fakeLibrary(), defaultRoot + "/" + defaultRoot},
				}},
			},
			{
				name: "returns the error of the store for a file of a library that does not read",
				giveTree: storeTree{fstest.MapFS{}, map[string]fs.FS{
					typescript.LibraryStore: failingFS{fakeLibrary(), defaultRoot + "/" + domRoot},
				}},
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, _, err := runRound(t, tt.giveTree, []string{defaultInput}, 1)
				assert.ErrorIs(t, err, errBroken, "the round returns the failure of the read")
			})
		}

		t.Run("returns an error for a library whose root the store does not have", func(t *testing.T) {
			t.Parallel()

			missing := plugin.StorePath(typescript.LibraryStore, unknownMember+"/"+unknownMember)
			_, _, err := runRound(t, withLibrary(fstest.MapFS{}), []string{missing}, 1)
			assert.ErrorIs(t, err, fs.ErrNotExist, "a store without the file of a key fails the round")
		})
	})
}

// runRound runs a round of a number through a TypeScript frontend, over a
// workspace and its stores, with the shared inputs and the needs, under the
// context of the test. It returns the units of the round and the round,
// whose reports the load reads.
func runRound(
	tb assert.TB, tree fs.FS, shared []string, number int, needs ...string,
) ([][]plugin.SourceRef, *plugin.DependencyRound, error) {
	tb.Helper()

	round := &plugin.DependencyRound{Number: number, Shared: shared}
	for _, need := range needs {
		round.Needs = append(round.Needs, plugin.Need{Path: need})
	}
	dependent, depends := frontend.New().(plugin.Dependent)
	assert.True(tb, depends, "the TypeScript frontend is a dependent")
	units, err := dependent.Dependencies(assert.Context(tb), round, roundReader{tree})
	return units, round, err
}

// membersOf returns the member paths of each unit.
func membersOf(units [][]plugin.SourceRef) [][]string {
	out := make([][]string, 0, len(units))
	for _, unit := range units {
		members := make([]string, 0, len(unit))
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}
