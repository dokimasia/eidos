// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The configuration files the chain cases write.
const (
	baseConfig = "base.json"
	srcConfig  = "src/tsconfig.json"
)

// withConfigs returns a tree of src/a.ts and the given configurations.
func withConfigs(configs map[string]string) fstest.MapFS {
	tree := fstest.MapFS{aFile: {Data: []byte("import { X } from '" + importSpec + "';\n")}}
	for p, data := range configs {
		tree[p] = &fstest.MapFile{Data: []byte(data)}
	}
	return tree
}

// importSpec is the specifier the chain cases' module imports X from.
const importSpec = "@app/x"

// sharedOf partitions a tree and returns the shared inputs src/a.ts
// declares.
func sharedOf(tb assert.TB, tree fstest.MapFS) []string {
	tb.Helper()

	var claimed []plugin.SourceRef
	for p := range tree {
		claimed = append(claimed, plugin.SourceRef{Path: p})
	}
	units, err := frontend.New().Partition(context.Background(), claimed, treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	for _, unit := range units {
		if unit[0].Path == aFile {
			return unit[0].Shared
		}
	}
	tb.Fatalf("no unit has %s", aFile)
	return nil
}

// importTiers parses src/a.ts in a tree and returns the packages the
// tiers for its imported X name, without its own package's tier and the
// global package's.
func importTiers(tb assert.TB, tree fstest.MapFS) [][]string {
	tb.Helper()

	gb, _ := parsedTree(tb, tree, aFile, plugin.DepthFull)
	file := fileIn(tb, gb, aPackage)
	var scope plugin.ImportScope
	for _, rec := range gb.Scopes() {
		if rec.File == file {
			scope.Bindings = rec.Bindings
		}
	}
	tiers := resolved(scope, probeName)
	out := make([][]string, 0, len(tiers))
	for _, tier := range tiers[1 : len(tiers)-1] {
		out = append(out, packagesOf(tier))
	}
	return out
}

// packagesOf returns the packages of candidates.
func packagesOf(ids []symbol.Identity) []string {
	out := make([]string, 0, len(ids))
	for _, c := range ids {
		out = append(out, c.Package)
	}
	return out
}

// A file's tsconfig chain is its shared input and states how its bare
// specifiers resolve, so the chain's reading and the resolution it
// states are pinned the way TypeScript reads them.
func TestTsconfig(t *testing.T) {
	t.Parallel()

	t.Run("governing", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the nearest tsconfig.json above a file", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{rootConfig: "{}", srcConfig: "{}"})
			assert.Equal(t, sharedOf(t, tree), []string{srcConfig}, "the one in the file's own directory")
		})

		t.Run("declares no shared input for a file without a tsconfig.json above it", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, sharedOf(t, withConfigs(nil)), "no configuration governs it")
		})
	})

	t.Run("readChain", func(t *testing.T) {
		t.Parallel()

		chains := []struct {
			name    string
			configs map[string]string
			want    []string
		}{
			{
				name:    "declares the configuration a relative extends names",
				configs: map[string]string{rootConfig: `{"extends": "./base.json"}`, baseConfig: "{}"},
				want:    []string{rootConfig, baseConfig},
			},
			{
				name:    "appends .json to an extends path that names no file",
				configs: map[string]string{rootConfig: `{"extends": "./base"}`, baseConfig: "{}"},
				want:    []string{rootConfig, baseConfig},
			},
			{
				name: "declares the file a package extends names under node_modules",
				configs: map[string]string{
					rootConfig: `{"extends": "@tsconfig/node/tsconfig.json"}`,
					"node_modules/@tsconfig/node/tsconfig.json": "{}",
				},
				want: []string{rootConfig, "node_modules/@tsconfig/node/tsconfig.json"},
			},
			{
				name: "declares a package's tsconfig.json for an extends that names the package",
				configs: map[string]string{
					rootConfig: `{"extends": "@base"}`, "node_modules/@base/tsconfig.json": "{}",
				},
				want: []string{rootConfig, "node_modules/@base/tsconfig.json"},
			},
			{
				name: "declares each entry of an extends list in order",
				configs: map[string]string{
					rootConfig: `{"extends": ["./b.json", "./base.json"]}`, "b.json": "{}", baseConfig: "{}",
				},
				want: []string{rootConfig, "b.json", baseConfig},
			},
			{
				name: "stops at a cycle of extends",
				configs: map[string]string{
					rootConfig: `{"extends": "./base.json"}`, baseConfig: `{"extends": "./tsconfig.json"}`,
				},
				want: []string{rootConfig, baseConfig},
			},
		}
		for _, tt := range chains {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, sharedOf(t, withConfigs(tt.configs)), tt.want, "the chain, depth first")
			})
		}

		faults := []struct {
			name    string
			configs map[string]string
		}{
			{
				name:    "reports BadConfig for an extends that names no configuration",
				configs: map[string]string{rootConfig: `{"extends": "./missing.json"}`},
			},
			{
				name:    "reports BadConfig for an extends that is neither a path nor a list",
				configs: map[string]string{rootConfig: `{"extends": 1}`},
			},
			{
				name:    "reports BadConfig for a configuration whose options do not decode",
				configs: map[string]string{rootConfig: `{"compilerOptions": {"paths": 1}}`},
			},
			{
				name:    "reports BadConfig for a package extends no node_modules directory has",
				configs: map[string]string{rootConfig: `{"extends": "@missing/base"}`},
			},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, found := parsedTree(t, withConfigs(tt.configs), aFile, plugin.DepthFull)
				assert.Equal(t, codesOf(found), []diag.Code{frontend.BadConfig}, "the configuration's fault reports")
			})
		}

		t.Run("reads a configuration with comments and trailing commas", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{rootConfig: "{\n  // the base\n  \"compilerOptions\": " +
				"{ \"baseUrl\": \"lib\", },\n}\n"})
			assert.Equal(t, importTiers(t, tree), [][]string{{"lib/@app/x"}, {"lib/@app/x/index"}, {importSpec}},
				"TypeScript's configuration is JSON with comments")
		})
	})

	t.Run("substitutions", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a specifier through a paths pattern's substitutions in order", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"compilerOptions": {"paths": {"@app/*": ["app/*", "fallback/*"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree), [][]string{
				{"app/x"}, {"app/x/index"}, {"fallback/x"}, {"fallback/x/index"}, {importSpec},
			}, "each substitution's file, then its index")
		})

		t.Run("passes over the keys that do not match the specifier", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"compilerOptions": {"paths": {"other": ["o"], "@lib/*": ["lib/*"], "@app/*": ["app/*"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"app/x"}, "@app/* matches alone")
		})

		t.Run("prefers an exact paths key to a wildcard key", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"compilerOptions": {"paths": {"*": ["any/*"], "@app/x": ["exact"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"exact"}, "the exact key decides")
		})

		t.Run("prefers the matching wildcard key with the longest prefix", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"compilerOptions": {"paths": {"@*": ["short/*"], "@app/*": ["long/*"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"long/x"}, "@app/ is longer than @")
		})

		t.Run("resolves paths relative to the configuration's directory without a baseUrl", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				srcConfig: `{"compilerOptions": {"paths": {"@app/x": ["lib/x"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"src/lib/x"}, "relative to src/tsconfig.json")
		})
	})

	t.Run("own", func(t *testing.T) {
		t.Parallel()

		t.Run("inherits the paths an extended configuration states", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"extends": "./base.json"}`,
				baseConfig: `{"compilerOptions": {"paths": {"@app/x": ["inherited"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"inherited"}, "the base's paths apply")
		})

		t.Run("lets a later entry of an extends list override an earlier one", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"extends": ["./b.json", "./base.json"]}`,
				"b.json":   `{"compilerOptions": {"baseUrl": "first"}}`,
				baseConfig: `{"compilerOptions": {"baseUrl": "second"}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"second/@app/x"}, "the later base decides")
		})

		t.Run("overrides an extended configuration's baseUrl", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"extends": "./base.json", "compilerOptions": {"baseUrl": "mine"}}`,
				baseConfig: `{"compilerOptions": {"baseUrl": "theirs"}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"mine/@app/x"}, "the extending configuration decides")
		})

		t.Run("places inherited paths under a baseUrl the extending configuration sets", func(t *testing.T) {
			t.Parallel()

			tree := withConfigs(map[string]string{
				rootConfig: `{"extends": "./base.json", "compilerOptions": {"baseUrl": "lib"}}`,
				baseConfig: `{"compilerOptions": {"paths": {"@app/x": ["x"]}}}`,
			})
			assert.Equal(t, importTiers(t, tree)[0], []string{"lib/x"}, "paths follow the baseUrl in effect")
		})
	})
}
