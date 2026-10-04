// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	tsgrammar "go.dokimi.dev/eidos/lang/treesitter/typescript"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The whole-contract fixture's signature root, its package, and its one
// declaration the module does not export, which a signature-only load
// drops. The barrel's declaration is the one a reference names only
// through a re-export.
const (
	depRoot      = "dep"
	depPackage   = "dep/dep"
	hiddenName   = "hidden"
	userPackage  = "models/user"
	personName   = "Person"
	unparsedCode = "TYPESCRIPT-0001"
)

// newAllocs is the frontend New returns: the frontend's state with its
// two vocabularies and its parse hook, the version, the syntax's two,
// and the kit's four.
const newAllocs = 4 + 1 + 2 + 4

// allocCall is one call that an allocation test and a benchmark share:
// its benchmark path, its allocation ceiling, the call, and the check
// of the result the call leaves.
type allocCall struct {
	name   string
	allocs uint64
	call   func()
	check  func(tb assert.TB)
}

// The frontend runs the conformance suite every frontend runs, over real
// TypeScript source.
func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("returns a frontend in the exporter role", func(t *testing.T) {
			t.Parallel()

			_, exports := frontend.New().(plugin.Exporter)
			assert.True(t, exports, "a TypeScript module publishes names it does not declare")
		})

		t.Run("returns a frontend whose language overloads", func(t *testing.T) {
			t.Parallel()

			assert.True(t, frontend.New().Overloads(), "TypeScript declares overload signatures")
		})

		t.Run("returns a version that folds the frontend's version", func(t *testing.T) {
			t.Parallel()

			assert.True(t, strings.HasPrefix(versionOf(t), typescript.FrontendVersion),
				"a graph change bumps what the unit keys fold")
		})

		t.Run("returns a version that folds the grammar's version", func(t *testing.T) {
			t.Parallel()

			assert.True(t, strings.HasSuffix(versionOf(t), tsgrammar.TypeScript.Version()),
				"a grammar upgrade bumps what the unit keys fold")
		})

		t.Run("returns a frontend that reports a syntax error as TYPESCRIPT-0001", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class {\n")
			assert.NotEmpty(t, found, "the syntax error reports")
			assert.Equal(t, found[0].Code.String(), unparsedCode, "under the satellite's prefix and the first number")
		})

		t.Run("returns a frontend that claims no file under node_modules", func(t *testing.T) {
			t.Parallel()

			claim := frontend.New().Selection()
			assert.Contains(t, claim, "!**/node_modules/**", "an installed package's sources are not the workspace's")
		})
	})
}

// The construction allocates the frontend the kit builds. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestFrontendAllocs(t *testing.T) {
	checkAllocs(t, frontendCalls())
}

// BenchmarkFrontend measures the construction a composition makes once
// per load.
func BenchmarkFrontend(b *testing.B) {
	benchCalls(b, frontendCalls())
}

// frontendCalls returns a call of New.
func frontendCalls() []allocCall {
	var f plugin.Frontend
	return []allocCall{
		{
			name: "New", allocs: newAllocs,
			call: func() { f = frontend.New() },
			check: func(tb assert.TB) {
				assert.Equal(tb, f.Name(), typescript.Name, "New returns the TypeScript frontend")
			},
		},
	}
}

// checkAllocs checks the ceiling of every call in the ordinary run, and
// the result each call leaves.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling, one sub-benchmark each. Each call runs once before the
// contract starts, so what the first call initialises stays out of the
// count.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	for _, tt := range calls {
		b.Run(tt.name, func(b *testing.B) {
			tt.call()
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
			tt.check(b)
		})
	}
}

// tsTree is the whole-contract fixture: two modules and a reference
// between them, a carrier, a test file, a signature root, a file that
// does not parse, and a barrel module that re-exports what another
// module declares.
func tsTree() fstest.MapFS {
	return fstest.MapFS{
		"api/user.ts": {Data: []byte("export interface User {\n  name: string;\n}\n")},
		"store/row.ts": {Data: []byte("import { User } from '../api/user';\n\n" +
			"/** Row is one record. */\n// +fixture:gen:table name=rows\n" +
			"export class Row {\n  owner: User;\n  count: number;\n" +
			"  get(n: number): number { return this.count + n; }\n}\n")},
		"store/row.test.ts": {Data: []byte("export function probe(): void {}\n")},
		"dep/dep.ts":        {Data: []byte("export class Dep {}\nconst hidden = 1;\n")},
		"bad/oops.ts":       {Data: []byte("export class {\n")},
		"models/user.ts":    {Data: []byte("export interface Person {\n  id: string;\n}\n")},
		"models/index.ts":   {Data: []byte("export * from './user';\n")},
		"uses/api.ts": {Data: []byte("import { Person } from '../models';\n\n" +
			"export interface Session {\n  user: Person;\n}\n")},
	}
}

// setup is the suite's entry over the whole-contract fixture.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(), &frontendtest.Fixture{
		Sources:    tsTree(),
		Signatures: []string{depRoot},
		Dropped: []symbol.Identity{
			{Lang: frontend.Lang, Package: depPackage, Name: hiddenName, Kind: symbol.KindConstant},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    frontend.Keys,
		Reexported: []symbol.Identity{
			{Lang: frontend.Lang, Package: userPackage, Name: personName, Kind: symbol.KindInterface},
		},
	}
}

// versionOf returns the version of the frontend.
func versionOf(tb assert.TB) string {
	tb.Helper()

	v, versioned := frontend.New().(plugin.Versioned)
	assert.True(tb, versioned, "the frontend states a version")
	return v.Version()
}
