// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The one-file tree the lowering cases read, and the path that no
// tree contains.
const (
	svcPath       = "svc"
	svcFile       = "svc/a.zz"
	absentFile    = "svc/gone.zz"
	depPackage    = "dep"
	aliasName     = "api"
	boundName     = "api.B"
	unboundName   = "none.B"
	ownSpelling   = "Own"
	builtinName   = "int"
	typeParamName = "T"
)

// The spellings at each end of the capital range, which is where a
// resolver's own bound decides whether a bare name is its package's.
const (
	firstCapital = "Alpha"
	lastCapital  = "Zeta"
)

// The lowering's positional facts about the one-file tree: the line
// of each statement, and the name of the second field of a
// two-reference type.
const (
	packageStatementLine = 1
	typeStatementLine    = 3
	secondFieldName      = "f1"
)

// unitOver assembles one unit over a tree, the way the driver does.
func unitOver(f *frontendtest.Scripted, tree fstest.MapFS, files ...string) (
	*plugin.SourceUnit, *diag.Sink,
) {
	refs := make([]plugin.SourceRef, len(files))
	for i, path := range files {
		refs[i] = plugin.SourceRef{Path: path}
	}
	sink := diag.NewSink()
	return plugin.NewSourceUnit(
		refs, tree, plugin.DepthFull, f.Syntax(), sink, f.Name(),
	), sink
}

// The scripted language substitutes for five real ones, so its own
// lowering is pinned: what it declares, binds, stamps and refuses.
func TestScripted(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		svcFile: {Data: []byte(
			"package svc\nimport api dep\ntype A api.B int\nmethod Get int\nconst low\nstamp fake.testFile yes\n+gen:table name=t\n",
		)},
	}

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		t.Run("groups by directory", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			parts, err := f.Partition(context.Background(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{tree})
			assert.NoError(t, err, "the partition groups")
			assert.Length(t, parts, 1, "one directory, one unit")
			assert.Equal(t, parts[0][0].Path, svcFile, "containing the directory's file")
		})

		t.Run("declares the manifest a shared input where the tree contains one", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			manifested := fstest.MapFS{
				modFile: {Data: []byte("mod v1\n")},
				svcFile: tree[svcFile],
			}
			parts, err := f.Partition(context.Background(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{manifested})
			assert.NoError(t, err, "the partition groups")
			assert.Equal(t, parts[0][0].Shared, []string{modFile},
				"a manifest the tree contains is every member's shared input")

			bare, err := f.Partition(context.Background(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{tree})
			assert.NoError(t, err, "the partition groups")
			assert.Empty(t, bare[0][0].Shared,
				"a tree without the manifest gives its members no shared input")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers every statement into the unit's builder", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, sink := unitOver(f, tree, svcFile)
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			coretest.AssertCodes(t, sink)

			gb := u.Graph()
			assert.Length(t, gb.Packages(), 1, "one package declared")
			file := gb.Packages()[0].Files[0]
			assert.Length(t, file.Decls, 2, "a type and a constant")
			assert.Equal(t, file.Pos.Line, packageStatementLine,
				"the file is positioned at its package line, counted from one")
			declared := file.Decls[0].(*node.Struct)
			assert.Equal(t, declared.Pos.Line, typeStatementLine,
				"and every statement at the line that states it")
			assert.Equal(t, declared.Fields[1].Name, secondFieldName,
				"fields are named f0 upward, one per reference in order")
			assert.Length(t, gb.Scopes(), 1, "the bindings recorded")
			assert.Length(t, gb.Attachments(), 1, "the directive recorded")
			assert.Length(t, gb.StampRecords(), 1, "the stamp recorded")
		})

		t.Run("lowers a type parameter onto the last type", func(t *testing.T) {
			t.Parallel()

			generic := fstest.MapFS{
				svcFile: {Data: []byte("package svc\ntype Box T\ntypeparam T\n")},
			}
			f := frontendtest.NewScripted()
			u, sink := unitOver(f, generic, svcFile)
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			coretest.AssertCodes(t, sink)

			declared := u.Graph().Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Length(t, declared.TypeParams, 1, "the type declares one parameter")
			assert.Equal(t, declared.TypeParams[0].Name, typeParamName, "under the stated name")
			assert.Equal(t, declared.Fields[0].Type.Spelling, typeParamName,
				"and a field spells it the way any reference is spelled")
		})

		t.Run("reads a comment through the kernel's own split", func(t *testing.T) {
			t.Parallel()

			commented := fstest.MapFS{
				svcFile: {Data: []byte(
					"package svc\n" +
						"// A records one row.\n" +
						"//tool:keep forever\n" +
						"// +gen:table name=t\n" +
						"type A int\n",
				)},
			}
			f := frontendtest.NewScripted()
			u, sink := unitOver(f, commented, svcFile)
			assert.NoError(t, f.Parse(context.Background(), u), "the unit parses")
			coretest.AssertCodes(t, sink)

			gb := u.Graph()
			declared := gb.Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Equal(t, declared.Doc, []string{"A records one row."},
				"the comment above the declaration documents it, "+
					"and a marker-adjacent directive line is not documentation")
			assert.Equal(t, declared.Annotations, symbol.Annotations{
				{Name: "tool:keep", Args: []string{"forever"}},
			}, "a marker-adjacent tool directive lowers as an annotation")
			assert.Length(t, gb.Attachments(), 1, "and the carrier attaches")
			assert.Equal(t, string(gb.Attachments()[0].Raw.Name), "gen:table",
				"under its own name, the mark stripped by the split")
		})

		t.Run("returns the read's own error for an absent member", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, _ := unitOver(f, fstest.MapFS{}, absentFile)
			err := f.Parse(context.Background(), u)
			assert.HasError(t, err, "a member missing from the tree cannot lower")
			assert.Contains(t, err.Error(), absentFile, "naming the path")
		})

		t.Run("reports a directive outside the grammar and parses on", func(t *testing.T) {
			t.Parallel()

			broken := fstest.MapFS{
				svcFile: {Data: []byte("package svc\ntype A\n+=x\nconst low\n")},
			}
			f := frontendtest.NewScripted()
			u, sink := unitOver(f, broken, svcFile)
			assert.NoError(t, f.Parse(context.Background(), u), "a bad carrier is not fatal")
			coretest.AssertReports(t, sink, frontendtest.ScriptedBadFile)
			coretest.AssertPositioned(t, sink)

			gb := u.Graph()
			assert.Empty(t, gb.Attachments(), "nothing attaches for a payload that will not parse")
			assert.Length(t, gb.Packages()[0].Files[0].Decls, 2,
				"and the statements after it still lower")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("probes bindings and leaves builtins", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			scope := plugin.ImportScope{
				File:     symbol.Identity{Lang: frontendtest.ScriptedLang, Package: svcPath},
				Bindings: map[string][]string{aliasName: {depPackage}},
			}
			got := f.Resolve(scope, boundName)
			assert.Length(t, got, 1, "the scripted language probes in one tier")
			assert.Length(t, got[0], 1, "a bound alias probes its package")
			assert.Equal(t, got[0][0].Package, depPackage, "at the bound path")
			own := f.Resolve(scope, ownSpelling)
			assert.Length(t, own, 1, "a bare capital probes in one tier")
			assert.Equal(t, own[0][0].Package, svcPath, "the file's own package")
			assert.Empty(t, f.Resolve(scope, unboundName), "an alias the file never bound probes nothing")
			assert.Empty(t, f.Resolve(scope, builtinName), "a builtin is nobody's")
		})

		t.Run("probes a bare spelling at either end of the capital range", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			scope := plugin.ImportScope{
				File: symbol.Identity{Lang: frontendtest.ScriptedLang, Package: svcPath},
			}
			assert.Length(t, f.Resolve(scope, firstCapital), 1,
				"a spelling opening at A is its own package's")
			assert.Length(t, f.Resolve(scope, lastCapital), 1,
				"and so is one opening at Z")
		})
	})

	t.Run("ScriptedKeys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the classification key it stamps", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, frontendtest.ScriptedKeys(r), "the key registers")
			_, held := r.Resolve(frontendtest.ScriptedTestKey)
			assert.True(t, held, "under the name the stamps write")
		})

		t.Run("reports a registry that already claims the namespace", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, frontendtest.ScriptedKeys(r), "the first claim registers")
			err := frontendtest.ScriptedKeys(r)
			assert.HasError(t, err, "a namespace is claimed once")
			assert.Contains(t, err.Error(), "fake", "naming it")
		})
	})
}

// reader is the recorded partition door over a test tree.
type reader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r reader) Read(path string) ([]byte, error) {
	return r.tree.ReadFile(path)
}
