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

// scopeOf parses one file and returns its recorded import scope.
func scopeOf(tb assert.TB, src string) (plugin.Frontend, plugin.ImportScope) {
	tb.Helper()

	tree := fstest.MapFS{"p/a.go": {Data: []byte(src)}}
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: "p/a.go"}}, tree, plugin.DepthFull,
		f.Syntax(), diag.NewSink(), f.Name(),
	)
	assert.NoError(tb, f.Parse(context.Background(), u), "the file parses")
	scopes := u.Graph().Scopes()
	assert.Length(tb, scopes, 1, "one file, one scope")
	return f, plugin.ImportScope{
		File:     symbol.Identity{Lang: frontend.Lang, Package: "p", Name: "p/a.go", Kind: symbol.KindFile},
		Bindings: scopes[0].Bindings,
	}
}

// Resolution is Go's own probing, so its normalization and its
// empty results are pinned beside its candidates.
func TestResolve(t *testing.T) {
	t.Parallel()

	src := "package p\n\nimport (\n\t\"example.test/fix/api\"\n\tren \"example.test/fix/other\"\n\t_ \"example.test/fix/blank\"\n)\n\n" +
		"var _ = api.User{}\nvar _ = ren.Thing{}\n"

	t.Run("probes qualified spellings through the bindings", func(t *testing.T) {
		t.Parallel()

		f, scope := scopeOf(t, src)
		got := f.Resolve(scope, "api.User")
		assert.Length(t, got, 1, "the default local name binds")
		assert.Equal(t, got[0], symbol.Identity{
			Lang: frontend.Lang, Package: "example.test/fix/api", Name: "User",
		}, "at the imported path")

		renamed := f.Resolve(scope, "ren.Thing")
		assert.Length(t, renamed, 1, "a renamed import binds its alias")
		assert.Equal(t, renamed[0].Package, "example.test/fix/other", "at its path")

		fallback := f.Resolve(scope, "ghost.X")
		assert.Length(t, fallback, 1,
			"an unbound qualifier probes the unaliased imports alone, because an alias and a blank bind no other name")
		assert.Equal(t, fallback[0].Package, "example.test/fix/api",
			"a package's clause can differ from its assumed name, and the graph decides")
		blank := f.Resolve(scope, "blank.X")
		assert.Length(t, blank, 1, "a blank import binds no qualifier")
		assert.Equal(t, blank[0].Package, "example.test/fix/api", "so the spelling falls back like any other")

		versioned, vscope := scopeOf(t,
			"package p\n\nimport \"gopkg.in/yaml.v3\"\n\nvar _ yaml.Node\n")
		got = versioned.Resolve(vscope, "yaml.Node")
		assert.Length(t, got, 1, "a versioned path binds the name it assumes")
		assert.Equal(t, got[0].Package, "gopkg.in/yaml.v3", "at its path")
	})

	t.Run("probes dot imports for bare exported spellings", func(t *testing.T) {
		t.Parallel()

		dotted := "package p\n\nimport . \"example.test/fix/flood\"\n\nvar _ = Thing{}\n"
		f, scope := scopeOf(t, dotted)
		got := f.Resolve(scope, "Thing")
		assert.Length(t, got, 2, "the own package first, then the dot import")
		assert.Equal(t, got[0].Package, "p", "home before flood")
		assert.Equal(t, got[1].Package, "example.test/fix/flood", "the flooded scope probes")
		assert.Length(t, f.Resolve(scope, "thing"), 1,
			"a dot import floods exported names alone")
	})

	t.Run("strips decoration before probing", func(t *testing.T) {
		t.Parallel()

		f, scope := scopeOf(t, src)
		assert.Equal(t, f.Resolve(scope, "*[]api.User")[0].Name, "User",
			"pointers and slices unwrap")
		assert.Equal(t, f.Resolve(scope, "[4]api.User")[0].Name, "User",
			"array lengths unwrap")
		assert.Equal(t, f.Resolve(scope, "*List[api.User]")[0], symbol.Identity{
			Lang: frontend.Lang, Package: "p", Name: "List",
		}, "a decorated instantiation resolves the way a bare one does")
		assert.Equal(t, f.Resolve(scope, "(row)")[0].Name, "row",
			"parentheses are punctuation")
		assert.Equal(t, f.Resolve(scope, "row")[0], symbol.Identity{
			Lang: frontend.Lang, Package: "p", Name: "row",
		}, "a bare spelling probes its own package, whatever its case")
	})

	t.Run("returns no candidate for what no declaration declares", func(t *testing.T) {
		t.Parallel()

		f, scope := scopeOf(t, src)
		assert.Empty(t, f.Resolve(scope, "int"), "a builtin is nobody's")
		assert.Empty(t, f.Resolve(scope, "map[string]int"), "a map is a shape")
		assert.Empty(t, f.Resolve(scope, "func(int) error"), "a func type is a shape")
		assert.Empty(t, f.Resolve(scope, "chan int"), "a channel is a shape")
		assert.Empty(t, f.Resolve(scope, "~int"), "an approximation is a term, not a name")
		assert.Empty(t, f.Resolve(scope, "A|B"), "a union never fabricates an identity")
	})
}
