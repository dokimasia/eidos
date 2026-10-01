// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// goTree is the whole-contract fixture: a module root, two
// packages, a cross-package reference, a carrier, a test file, a
// signature root and a file that does not parse.
func goTree() fstest.MapFS {
	return fstest.MapFS{
		"go.mod": {Data: []byte("module example.test/fix\n")},
		"api/user.go": {Data: []byte(
			"package api\n\n// User is one account.\ntype User struct{ name string }\n",
		)},
		"store/row.go": {Data: []byte(
			"package store\n\nimport \"example.test/fix/api\"\n\n" +
				"// Row is one record.\n//+fixture:gen:table name=rows\n" +
				"type Row struct {\n\towner api.User\n\tcount int\n}\n\n" +
				"func (r *Row) Get(n int) int { return r.count + n }\n",
		)},
		"store/row_test.go": {Data: []byte(
			"package store\n\nfunc probe() {}\n",
		)},
		"dep/dep.go": {Data: []byte(
			"package dep\n\ntype Dep struct{}\n\nconst hidden = 1\n",
		)},
		"bad/oops.go": {Data: []byte("package bad\n\nfunc ({ nope\n")},
	}
}

// The signature root, its package and its one unexported
// declaration, which a signature-only load drops.
const (
	depRoot    = "dep"
	depPackage = "example.test/fix/dep"
	hiddenName = "hidden"
)

// unparsedCode pins the code a syntax error reports under: the
// satellite's prefix, golang.CodePrefix, and the first number.
const unparsedCode = "GOLANG-0001"

// setup is the suite's entry over the whole-contract fixture.
func setup(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
	return frontend.New(nil), &frontendtest.Fixture{
		Sources:    goTree(),
		Signatures: []string{depRoot},
		Dropped: []symbol.Identity{
			{Lang: frontend.Lang, Package: depPackage, Name: hiddenName, Kind: symbol.KindConstant},
		},
		Schemas: frontendtest.ScriptedSchemas(),
		Keys:    frontend.Keys,
	}
}

// The frontend runs the conformance suite every frontend runs, over
// real Go source.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance suite", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, setup)
		})

		t.Run("passes the conformance suite with its stores", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontend.New(nil), &frontendtest.Fixture{
					Sources: depWorkspace(),
					Stores:  depStores(),
					Keys:    frontend.Keys,
				}
			})
		})

		t.Run("returns a frontend in the dependent role", func(t *testing.T) {
			t.Parallel()

			_, dependent := frontend.New(nil).(plugin.Dependent)
			assert.True(t, dependent, "Go sources import packages outside the workspace")
		})

		t.Run("parses past a file with a syntax error", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := unitOf(t, f, goTree(), "bad/oops.go")
			assert.NoError(t, f.Parse(context.Background(), u),
				"a syntax error is the source's problem, not the load's")
		})

		t.Run("returns a frontend that reports a syntax error as GOLANG-0001", func(t *testing.T) {
			t.Parallel()

			_, found := parsedFindings(t, nil, plugin.DepthFull, "package bad\n\nfunc ({ nope\n")
			assert.NotEmpty(t, found, "the syntax error reports")
			assert.Equal(t, found[0].Code.String(), unparsedCode, "under the satellite's prefix and the first number")
		})

		t.Run("returns a frontend whose language cannot overload", func(t *testing.T) {
			t.Parallel()

			assert.False(t, frontend.New(nil).Overloads(), "Go declares each function and method once by name")
		})

		t.Run("returns the frontend's own version", func(t *testing.T) {
			t.Parallel()

			v, versioned := frontend.New(nil).(plugin.Versioned)
			assert.True(t, versioned, "the frontend states a version")
			assert.Equal(t, v.Version(), golang.FrontendVersion,
				"the version is the frontend's, so a graph change bumps what the unit keys fold")
		})

		t.Run("returns a frontend whose folded method resolves through its own file's imports", func(t *testing.T) {
			t.Parallel()

			fx := rulestest.Loaded(t, frontend.New(nil), fstest.MapFS{
				"go.mod":       {Data: []byte("module example.test/fix\n")},
				"api/user.go":  {Data: []byte("package api\n\ntype User struct{}\n")},
				"store/row.go": {Data: []byte("package store\n\ntype Row struct{}\n")},
				"store/owner.go": {Data: []byte("package store\n\nimport \"example.test/fix/api\"\n\n" +
					"func (r *Row) Owner(u api.User) {}\n")},
			}, frontend.Keys)
			row, found := fx.Graph.Lookup(symbol.Identity{
				Lang: frontend.Lang, Package: "example.test/fix/store", Name: "Row", Kind: symbol.KindStruct,
			})
			assert.True(t, found, "the struct is indexed")
			methods := row.(*node.Struct).Methods
			assert.Length(t, methods, 1, "the method folds onto the struct its receiver names")
			assert.Equal(t, methods[0].Params[0].Type.Target, symbol.Identity{
				Lang: frontend.Lang, Package: "example.test/fix/api", Name: "User", Kind: symbol.KindStruct,
			}, "owner.go imports api, and row.go, where the struct is declared, does not")
		})

		t.Run("claims no vendor tree", func(t *testing.T) {
			t.Parallel()

			fx := rulestest.Loaded(t, frontend.New(nil), fstest.MapFS{
				"go.mod":                        {Data: []byte("module example.test/fix\n")},
				"api/user.go":                   {Data: []byte("package api\n\ntype User struct{}\n")},
				"vendor/example.com/lib/lib.go": {Data: []byte("package lib\n\ntype L struct{}\n")},
			}, frontend.Keys)
			pkg := func(path string) bool {
				id := symbol.Identity{Lang: frontend.Lang, Package: path, Kind: symbol.KindPackage}
				_, held := fx.Graph.PackageOf(id)
				return held
			}
			assert.True(t, pkg("example.test/fix/api"), "the module's package loads")
			assert.False(t, pkg("example.test/fix/vendor/example.com/lib"),
				"a vendored copy of a dependency does not load")
		})
	})
}

// unitOf partitions a tree and returns the unit with one file,
// reading carriers under the suite's brand.
func unitOf(
	tb assert.TB, f plugin.Frontend, tree fstest.MapFS, member string,
) *plugin.SourceUnit {
	tb.Helper()

	var claimed []plugin.SourceRef
	for path := range tree {
		if path != "go.mod" {
			claimed = append(claimed, plugin.SourceRef{Path: path})
		}
	}
	parts, err := f.Partition(context.Background(), claimed, treeReader{tree})
	assert.NoError(tb, err, "the fixture partitions")
	for _, part := range parts {
		for _, ref := range part {
			if ref.Path == member {
				return plugin.NewSourceUnit(part, tree, plugin.DepthFull,
					f.Syntax(), brand, diag.NewSink(), f.Name())
			}
		}
	}
	tb.Fatalf("no unit contains %s", member)
	return nil
}

// treeReader is the partition's recorded door over a test tree.
type treeReader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r treeReader) Read(path string) ([]byte, error) { return r.tree.ReadFile(path) }
