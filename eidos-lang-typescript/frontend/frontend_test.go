// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	tsgrammar "go.dokimi.dev/eidos/lang/treesitter/typescript"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
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
// and the kit's five, its list of key registrations included.
const newAllocs = 4 + 1 + 2 + 5

// allocCall is one call that an allocation test and a benchmark share:
// the method it calls, which names its benchmark, the case it measures
// where the method has more than one call, its allocation ceiling, the
// call, and the check of the result the call leaves.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb testing.TB)
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

		t.Run("returns a frontend that registers the typescript keys as its key provider", func(t *testing.T) {
			t.Parallel()

			provider, provides := frontend.New().(plugin.KeyProvider)
			assert.True(t, provides, "the frontend is a key provider")
			r := meta.NewRegistry()
			assert.NoError(t, provider.Keys(r.For(string(frontend.Lang))), "the registration succeeds")
			_, held := r.Resolve(typescript.TestFileKey)
			assert.True(t, held, "the test file key is registered")
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

			assert.HasPrefix(
				t,
				versionOf(t),
				typescript.FrontendVersion,
				"a graph change bumps what the unit keys fold",
			)
		})

		t.Run("returns a version that folds the grammar's version", func(t *testing.T) {
			t.Parallel()

			assert.HasSuffix(t, versionOf(t), tsgrammar.TypeScript.Version(),
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
			check: func(tb testing.TB) {
				tb.Helper()

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
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its
// ceiling: one sub-benchmark for each method, in the order the methods
// first appear, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling,
// and checks the result the last call leaves. The contract warms up with
// one call, so what the first call initialises stays out of the count.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
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
		Reexported: []symbol.Identity{
			{Lang: frontend.Lang, Package: userPackage, Name: personName, Kind: symbol.KindInterface},
		},
	}
}

// versionOf returns the version of the frontend.
func versionOf(tb testing.TB) string {
	tb.Helper()

	v, versioned := frontend.New().(plugin.Versioned)
	assert.True(tb, versioned, "the frontend states a version")
	return v.Version()
}
