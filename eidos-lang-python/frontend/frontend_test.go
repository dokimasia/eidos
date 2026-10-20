// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	pygrammar "go.dokimi.dev/eidos/lang/treesitter/python"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The whole-contract fixture's signature root, its package and the one
// declaration of the package that the module does not publish, which a
// signature-only load drops, and the declaration that a reference names
// only through the module of its package.
const (
	depRoot      = "dep"
	depPackage   = "dep/dep"
	hiddenName   = "_hidden"
	userPackage  = "models/user"
	personName   = "Person"
	unparsedCode = "PYTHON-0001"
)

// newAllocs is the cost of New for nil options: the empty options, the
// state of the frontend with its vocabulary and its parse and dependency
// hooks, the two functions of the comment walk that capture the vocabulary,
// the version, the classifier of test files, the syntax, and five of the
// kit with its match, its classification and its list of key registrations.
const newAllocs = 1 + 4 + 2 + 1 + 1 + 1 + 5

// TestFrontend runs the conformance suite that every frontend runs, over
// real Python source.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("returns a frontend that registers the python keys as its key provider", func(t *testing.T) {
			t.Parallel()

			provider, provides := frontend.New(nil).(plugin.KeyProvider)
			assert.True(t, provides, "the frontend is a key provider")
			r := meta.NewRegistry()
			assert.NoError(t, provider.Keys(r.For(string(frontend.Lang))), "the registration succeeds")
			_, held := r.Resolve(python.TestFileKey)
			assert.True(t, held, "the test file key is registered")
		})

		t.Run("returns a frontend in the exporter role", func(t *testing.T) {
			t.Parallel()

			_, exports := frontend.New(nil).(plugin.Exporter)
			assert.True(t, exports, "a Python module publishes the names that it imports")
		})

		t.Run("returns a frontend in the dependent role", func(t *testing.T) {
			t.Parallel()

			_, depends := frontend.New(nil).(plugin.Dependent)
			assert.True(t, depends, "the standard library loads from typeshed")
		})

		t.Run("returns a frontend whose language overloads", func(t *testing.T) {
			t.Parallel()

			assert.True(t, frontend.New(nil).Overloads(), "Python declares overloads under @overload")
		})

		t.Run("returns a version that folds the version of the frontend", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, versionOf(t), python.FrontendVersion, "a graph change bumps what the unit keys fold")
		})

		t.Run("returns a version that folds the version of the grammar", func(t *testing.T) {
			t.Parallel()

			assert.HasSuffix(t, versionOf(t), pygrammar.Grammar.Version(),
				"a grammar upgrade bumps what the unit keys fold")
		})

		t.Run("returns a frontend that reports a syntax error as PYTHON-0001", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "class :\n")
			assert.Contains(t, codesOf(sink), frontend.UnparsedFile, "the syntax error reports")
			assert.Equal(
				t,
				frontend.UnparsedFile.String(),
				unparsedCode,
				"the prefix of the satellite and the number 1",
			)
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "returns a frontend that claims no file of a virtual environment", give: "!**/.venv/**"},
			{name: "returns a frontend that claims no installed distribution", give: "!**/site-packages/**"},
			{name: "returns a frontend that claims no cache of the interpreter", give: "!**/__pycache__/**"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, frontend.New(nil).Selection(), tt.give, "the claim leaves the directory out")
			})
		}
	})

	t.Run("Options", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error from the parse for a version that is not a version of Python 3", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(&frontend.Options{Version: "2.7"})
			tree := fstest.MapFS{appFile: {Data: []byte("x = 1\n")}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, tree, plugin.DepthFull, f.Syntax(), brand,
				diag.NewSink(), f.Name())
			err := f.Parse(t.Context(), u)
			assert.HasError(t, err, "the load fails on the option")
			assert.Contains(t, err.Error(), "2.7", "the error names the option")
		})
	})
}

// TestFrontendAllocs checks the ceiling of the construction in the ordinary
// run, which runs no benchmark.
func TestFrontendAllocs(t *testing.T) {
	checkAllocs(t, frontendCalls())
}

// BenchmarkFrontend measures the construction that a composition makes
// once per load.
func BenchmarkFrontend(b *testing.B) {
	benchCalls(b, frontendCalls())
}

// frontendCalls returns a call of New.
func frontendCalls() []allocCall {
	var f plugin.Frontend
	return []allocCall{
		{
			name: "New", allocs: newAllocs,
			call: func() { f = frontend.New(nil) },
			check: func(tb assert.TB) {
				tb.Helper()

				assert.Equal(tb, f.Name(), python.Name, "New returns the Python frontend")
			},
		},
	}
}

// pyTree is the whole-contract fixture: a project file whose module root is
// the root of the workspace, two modules and a reference between them, a
// carrier, a test file, a signature root, a file that does not parse, and a
// package whose __init__.py imports what another module declares.
func pyTree() fstest.MapFS {
	return fstest.MapFS{
		projectFile:   {Data: []byte(rootProject)},
		"api/user.py": {Data: []byte("class User:\n    name: str\n")},
		"store/row.py": {Data: []byte("from api.user import User\n\n\n# +fixture:gen:table name=rows\n" +
			"class Row:\n    \"\"\"Row is one record.\"\"\"\n\n    owner: User\n    count: int\n\n" +
			"    def get(self, n: int) -> int:\n        return self.count + n\n")},
		"store/test_row.py":  {Data: []byte("def probe() -> None: ...\n")},
		"dep/dep.py":         {Data: []byte("class Dep: ...\n\n\n_hidden = 1\n")},
		"bad/oops.py":        {Data: []byte("class :\n")},
		"models/user.py":     {Data: []byte("class Person:\n    id: str\n")},
		"models/__init__.py": {Data: []byte("from .user import Person\n")},
		"uses/api.py": {Data: []byte("from models import Person\n\n\nclass Session:\n" +
			"    user: Person\n")},
	}
}

// setup is the entry of the suite over the whole-contract fixture.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(nil), &frontendtest.Fixture{
		Sources:    pyTree(),
		Empty:      fstest.MapFS{"empty/empty.py": {Data: []byte("\n")}},
		Signatures: []string{depRoot},
		Dropped: []symbol.Identity{
			{Lang: frontend.Lang, Package: depPackage, Name: hiddenName, Kind: symbol.KindVariable},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Stores:  map[string]fs.FS{frontend.TypeshedStore: typeshed.Files},
		Reexported: []symbol.Identity{
			{Lang: frontend.Lang, Package: userPackage, Name: personName, Kind: symbol.KindStruct},
		},
	}
}

// versionOf returns the version of the frontend.
func versionOf(tb testing.TB) string {
	tb.Helper()

	v, versioned := frontend.New(nil).(plugin.Versioned)
	assert.True(tb, versioned, "the frontend states a version")
	return v.Version()
}
