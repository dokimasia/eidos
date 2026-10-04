// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The file the resolution cases parse, the package it declares, and
// the paths its imports name.
const (
	resolveFile    = "p/a.go"
	resolvePackage = "p"
	apiPath        = "example.test/fix/api"
	floodPath      = "example.test/fix/flood"
)

// resolveSource imports one package unaliased, one under an alias,
// one blank and one dot import.
const resolveSource = "package p\n\nimport (\n\t\"example.test/fix/api\"\n\tren \"example.test/fix/other\"\n" +
	"\t_ \"example.test/fix/blank\"\n\t. \"example.test/fix/flood\"\n)\n\nvar _ = api.User{}\n"

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want plugin.Candidates
		}{
			{
				name: "returns the import a qualifier binds as one tier",
				give: "api.User", want: plugin.Candidates{{at(apiPath, "User")}},
			},
			{
				name: "returns the unaliased imports for an unbound qualifier",
				give: "ghost.X", want: plugin.Candidates{{at(apiPath, "X")}},
			},
			{
				name: "returns the own package then the dot import for an exported bare name",
				give: "Thing", want: plugin.Candidates{{at(resolvePackage, "Thing"), at(floodPath, "Thing")}},
			},
			{
				name: "returns the own package alone for an unexported bare name",
				give: "thing", want: plugin.Candidates{{at(resolvePackage, "thing")}},
			},
			{name: "returns no tier for a predeclared type", give: "int"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f, scope := scopeOf(t, resolveSource)
				assert.Equal(t, f.Resolve(scope, tt.give), tt.want, "the candidates in probe order")
			})
		}

		t.Run("returns the own package for a file without a recorded scope", func(t *testing.T) {
			t.Parallel()

			f, scope := scopeOf(t, resolveSource)
			scope.Bindings = nil
			assert.Equal(t, f.Resolve(scope, "Thing"), plugin.Candidates{{at(resolvePackage, "Thing")}},
				"the own package alone")
		})

		t.Run("returns no tier for a qualifier in a file without a recorded scope", func(t *testing.T) {
			t.Parallel()

			f, scope := scopeOf(t, resolveSource)
			scope.Bindings = nil
			assert.Empty(t, f.Resolve(scope, "api.User"), "no import binds the qualifier")
		})
	})
}

// scopeOf parses one file and returns the frontend with the file's
// recorded import scope.
func scopeOf(tb assert.TB, src string) (plugin.Frontend, plugin.ImportScope) {
	tb.Helper()

	tree := fstest.MapFS{resolveFile: {Data: []byte(src)}}
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: resolveFile}}, tree, plugin.DepthFull,
		f.Syntax(), brand, diag.NewSink(), f.Name(),
	)
	assert.NoError(tb, f.Parse(context.Background(), u), "the file parses")
	scopes := u.Graph().Scopes()
	assert.Length(tb, scopes, 1, "one file, one scope")
	return f, plugin.ImportScope{
		File: symbol.Identity{
			Lang: frontend.Lang, Package: resolvePackage, Name: resolveFile, Kind: symbol.KindFile,
		},
		Bindings: scopes[0].Bindings,
	}
}

// at returns a candidate identity: a package and a name in Go, with
// no kind.
func at(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: frontend.Lang, Package: pkg, Name: name}
}
