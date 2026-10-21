// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The golden lists of the files of two libraries in the order in which tsc
// 7.0.2 loads them, as tsc --listFiles prints them: the default library of a
// project without options, and the library of the target ES2015.
const (
	defaultOrderFile = "default-library.txt"
	es6OrderFile     = "es6-library.txt"
)

// The shared inputs of the libraries that the selection cases expect: the
// root of each library inside its key.
var (
	es2022Input   = plugin.StorePath(typescript.LibraryStore, "lib.es2022.full.d.ts/lib.es2022.full.d.ts")
	es6Input      = plugin.StorePath(typescript.LibraryStore, es6Root+"/"+es6Root)
	es5Input      = plugin.StorePath(typescript.LibraryStore, "lib.d.ts/lib.d.ts")
	es2015Input   = plugin.StorePath(typescript.LibraryStore, "lib.es2015.d.ts/lib.es2015.d.ts")
	fallbackInput = plugin.StorePath(typescript.LibraryStore,
		"lib.es2018.asynciterable.d.ts/lib.es2018.asynciterable.d.ts")
	domES2024Input = plugin.StorePath(typescript.LibraryStore, domRoot+"+"+es2024Root+"/"+domRoot)
	es5LibInput    = plugin.StorePath(typescript.LibraryStore, es5Root+"/"+es5Root)
)

// The tsconfig of each project selects its library, as tsc 7.0.2 selects
// it, so the selection, its key and the order of its files are pinned.
func TestLibrary(t *testing.T) {
	t.Parallel()

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			giveConfigs map[string]string
			want        []string
		}{
			{
				name:        "selects the default library of ES2025 for a configuration without lib or target",
				giveConfigs: map[string]string{rootConfig: "{}"},
				want:        []string{rootConfig, defaultInput},
			},
			{
				name:        "selects the default library of a target",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"target": "ES2022"}}`},
				want:        []string{rootConfig, es2022Input},
			},
			{
				name:        "selects lib.es6.d.ts for a target of ES2015",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"target": "es2015"}}`},
				want:        []string{rootConfig, es6Input},
			},
			{
				name:        "selects lib.d.ts for a target of ES5",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"target": "ES5"}}`},
				want:        []string{rootConfig, es5Input},
			},
			{
				name:        "selects the default library of ES2025 for a target that the release does not know",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"target": "ES3"}}`},
				want:        []string{rootConfig, defaultInput},
			},
			{
				name:        "selects the files of the entries of lib under a key sorted by name",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": ["ES2024", "DOM"]}}`},
				want:        []string{rootConfig, domES2024Input},
			},
			{
				name:        "selects the file that an entry of lib names by its file name",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": ["lib.es2024.d.ts"]}}`},
				want:        []string{rootConfig, es2024Input},
			},
			{
				name:        "selects the file of a fallback name",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": ["esnext.asynciterable"]}}`},
				want:        []string{rootConfig, fallbackInput},
			},
			{
				name:        "selects a file of two entries once",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": ["es6", "es2015"]}}`},
				want:        []string{rootConfig, es2015Input},
			},
			{
				name:        "does not select a file for an empty lib",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": []}}`},
				want:        []string{rootConfig},
			},
			{
				name:        "does not select a file for an entry of lib that the release does not know",
				giveConfigs: map[string]string{rootConfig: `{"compilerOptions": {"lib": ["es2099"]}}`},
				want:        []string{rootConfig},
			},
			{
				name: "selects the files of lib in place of the default library of the target",
				giveConfigs: map[string]string{
					rootConfig: `{"compilerOptions": {"target": "ES5", "lib": ["ES2024"]}}`,
				},
				want: []string{rootConfig, es2024Input},
			},
			{
				name: "does not select a file under noLib whatever lib lists",
				giveConfigs: map[string]string{
					rootConfig: `{"compilerOptions": {"noLib": true, "lib": ["ES2024"]}}`,
				},
				want: []string{rootConfig},
			},
			{
				name: "selects the library of a configuration that the chain extends",
				giveConfigs: map[string]string{
					rootConfig: `{"extends": "./base.json"}`,
					baseConfig: `{"compilerOptions": {"lib": ["ES2024"]}}`,
				},
				want: []string{rootConfig, baseConfig, es2024Input},
			},
			{
				name: "selects the library of an extending configuration over the library of its base",
				giveConfigs: map[string]string{
					rootConfig: `{"extends": "./base.json", "compilerOptions": {"lib": ["ES5"]}}`,
					baseConfig: `{"compilerOptions": {"lib": ["ES2024"], "noLib": false}}`,
				},
				want: []string{rootConfig, baseConfig, es5LibInput},
			},
			{
				name: "does not select a file under the noLib of an extending configuration whatever its base sets",
				giveConfigs: map[string]string{
					rootConfig: `{"extends": "./base.json", "compilerOptions": {"noLib": true}}`,
					baseConfig: `{"compilerOptions": {"target": "ES2022"}}`,
				},
				want: []string{rootConfig, baseConfig},
			},
			{
				name: "selects the library of the configurations that read in a chain that does not read whole",
				giveConfigs: map[string]string{
					rootConfig: `{"extends": ["./base.json", "./missing.json"]}`,
					baseConfig: `{"compilerOptions": {"lib": ["ES2024"]}}`,
				},
				want: []string{rootConfig, baseConfig, es2024Input},
			},
			{
				name: "selects the library of a configuration whose base does not read",
				giveConfigs: map[string]string{
					rootConfig: `{"extends": "./missing.json", "compilerOptions": {"lib": ["ES2024"]}}`,
				},
				want: []string{rootConfig, es2024Input},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				units := partitionedThrough(t, storeTree{withConfigs(tt.giveConfigs), realStores(t)}, aFile)
				assert.Equal(t, units[0][0].Shared, tt.want, "the chain, then the library of the project")
			})
		}
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			config string
			want   string
		}{
			{
				name:   "reports UnknownLibrary for an entry of lib that the release does not know",
				config: `{"compilerOptions": {"lib": ["es2099"]}}`,
				want:   `lib lists "es2099"`,
			},
			{
				name:   "reports UnknownLibrary for a target that the release does not know",
				config: `{"compilerOptions": {"target": "ES3"}}`,
				want:   `target "ES3"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree := withConfigs(map[string]string{rootConfig: tt.config})
				_, found := parsedTree(t, tree, aFile, plugin.DepthFull)
				assert.Equal(t, codesOf(found), []diag.Code{frontend.UnknownLibrary},
					"the parse reports the unknown name")
				assert.Contains(t, found[0].Msg, tt.want, "the finding quotes the name")
				assert.Equal(t, found[0].Pos.File, rootConfig, "the finding is at the configuration")
			})
		}
	})

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveKey  string
			wantFile string
		}{
			{
				name:     "returns the statements of each global name of the default library in the order of tsc 7.0.2",
				giveKey:  defaultRoot,
				wantFile: defaultOrderFile,
			},
			{
				name:     "returns the statements of each global name of the library of ES2015 in the order of tsc 7.0.2",
				giveKey:  es6Root,
				wantFile: es6OrderFile,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				golden, err := os.ReadFile(filepath.Join("testdata", tt.wantFile))
				assert.NoError(t, err, "the golden order reads")
				order := strings.Fields(string(golden))
				fragments := map[string][]string{}
				for _, file := range order {
					for _, s := range library.Index[file].Statements {
						member := plugin.StorePath(typescript.LibraryStore, tt.giveKey+"/"+file+"#"+s.Name)
						if !slices.Contains(fragments[s.Name], member) {
							fragments[s.Name] = append(fragments[s.Name], member)
						}
					}
				}
				for _, file := range order {
					if library.Index[file].Module {
						for _, s := range library.Index[file].Statements {
							delete(fragments, s.Name)
						}
					}
				}
				pkg := plugin.StorePath(typescript.LibraryStore, tt.giveKey)
				round := &plugin.DependencyRound{Number: 1}
				want := make([][]string, 0, len(fragments))
				for _, name := range slices.Sorted(maps.Keys(fragments)) {
					round.Uses = append(round.Uses, id(pkg, name))
					want = append(want, fragments[name])
				}
				units, err := runRound(t, roundReader{storeTree{fstest.MapFS{}, realStores(t)}}, round)
				assert.NoError(t, err, "the round places the units of the names")
				assert.Equal(t, membersOf(units), want, "the fragments of each name in the order of tsc --listFiles")
			})
		}
	})
}
