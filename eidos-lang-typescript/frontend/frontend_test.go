// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

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

// The frontend runs the conformance suite every frontend runs, over real
// TypeScript source.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance suite", func(t *testing.T) {
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

		t.Run("returns a version that folds the frontend's and the grammar's", func(t *testing.T) {
			t.Parallel()

			v, versioned := frontend.New().(plugin.Versioned)
			assert.True(t, versioned, "the frontend states a version")
			assert.True(t, strings.HasPrefix(v.Version(), typescript.FrontendVersion),
				"a graph change bumps what the unit keys fold")
			assert.True(t, strings.HasSuffix(v.Version(), tsgrammar.TypeScript.Version()),
				"and so does a grammar upgrade")
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
