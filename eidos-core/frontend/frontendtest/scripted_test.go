// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
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
	foldFile      = "svc/b.zz"
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

// The comment the comment cases lower: documentation, a tool
// directive, and a carrier under the suite's brand, above one type.
// negatedComment and shapedComment state one carrier each, under the
// negated mark and under the bare mark in the tool-directive shape.
const (
	commentDoc     = "A records one row."
	commentSource  = "package svc\n// " + commentDoc + "\n//tool:keep forever\n// " + commentCarrier + "\ntype A int\n"
	commentCarrier = "+" + string(frontendtest.Brand) + ":table name=t"
	negatedComment = "package svc\n// -" + string(frontendtest.Brand) + ":table\ntype A int\n"
	shapedComment  = "package svc\n//" + string(frontendtest.Brand) + ":table\ntype A int\n"
)

// The scripted language substitutes for five real ones, so its own
// lowering is pinned: what it declares, binds, stamps and refuses.
func TestScripted(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		svcFile: {Data: []byte(
			"package svc\nimport api dep\ntype A api.B int\nmethod Get int\nconst low\nstamp fake.testFile yes\n+gen:table name=t\n",
		)},
	}

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true under NewScripted", func(t *testing.T) {
			t.Parallel()

			assert.True(t, frontendtest.NewScripted().Overloads(), "the scripted language overloads its methods")
		})

		t.Run("reports false for a frontend declared without overloading", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			f.Overloading = false
			assert.False(t, f.Overloads(), "the declared field decides")
		})
	})

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		t.Run("groups the files by directory", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			parts, err := f.Partition(t.Context(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{tree})
			assert.NoError(t, err, "the partition groups")
			assert.Length(t, parts, 1, "one directory is one unit")
			assert.Equal(t, parts[0][0].Path, svcFile, "the unit contains the directory's file")
		})

		t.Run("declares the manifest a shared input of every member", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			manifested := fstest.MapFS{
				modFile: {Data: []byte("mod v1\n")},
				svcFile: tree[svcFile],
			}
			parts, err := f.Partition(t.Context(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{manifested})
			assert.NoError(t, err, "the partition groups")
			assert.Equal(t, parts[0][0].Shared, []string{modFile}, "the manifest is the shared input")
		})

		t.Run("declares no shared input without a manifest", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			parts, err := f.Partition(t.Context(),
				[]plugin.SourceRef{{Path: svcFile}}, reader{tree})
			assert.NoError(t, err, "the partition groups")
			assert.Empty(t, parts[0][0].Shared, "the member has no shared input")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers every statement into the unit's builder", func(t *testing.T) {
			t.Parallel()

			gb := parsed(t, string(tree[svcFile].Data))
			assert.Length(t, gb.Packages(), 1, "one package is declared")
			file := gb.Packages()[0].Files[0]
			assert.Length(t, file.Decls, 2, "a type and a constant are declared")
			assert.Equal(t, file.Pos.Line, packageStatementLine, "the file is at its package line")
			declared := file.Decls[0].(*node.Struct)
			assert.Equal(t, declared.Pos.Line, typeStatementLine, "the type is at its statement's line")
			assert.Equal(t, declared.Fields[1].Name, secondFieldName,
				"the fields are named f0 upward, one per reference")
			assert.Length(t, gb.Scopes(), 1, "the bindings are recorded")
			assert.Length(t, gb.Attachments(), 1, "the directive statement is recorded")
			assert.Length(t, gb.StampRecords(), 1, "the stamp is recorded")
		})

		t.Run("lowers a type parameter onto the last type", func(t *testing.T) {
			t.Parallel()

			gb := parsed(t, "package svc\ntype Box T\ntypeparam T\n")
			declared := gb.Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Length(t, declared.TypeParams, 1, "the type declares one parameter")
			assert.Equal(t, declared.TypeParams[0].Name, typeParamName, "the parameter has the stated name")
			assert.Equal(t, declared.Fields[0].Type.Spelling, typeParamName,
				"a field spells the parameter like any reference")
		})

		t.Run("lowers a comment's documentation onto the next type", func(t *testing.T) {
			t.Parallel()

			declared := parsed(t, commentSource).Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Equal(t, declared.Doc, []string{commentDoc}, "only the prose line is documentation")
		})

		t.Run("lowers a comment's tool directive as an annotation", func(t *testing.T) {
			t.Parallel()

			declared := parsed(t, commentSource).Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Equal(t, declared.Annotations, symbol.Annotations{
				{Name: "tool:keep", Args: []string{"forever"}},
			}, "the marker-adjacent tool directive is an annotation")
		})

		t.Run("attaches a comment's carrier under the unit's brand", func(t *testing.T) {
			t.Parallel()

			gb := parsed(t, commentSource)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.Equal(t, gb.Attachments()[0].Raw.Name, directive.Name("table"),
				"the name is the payload's, the mark stripped")
			assert.False(t, gb.Attachments()[0].Raw.Negated, "the instance is set")
		})

		t.Run("attaches a comment's negated carrier as a negated instance", func(t *testing.T) {
			t.Parallel()

			gb := parsed(t, negatedComment)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.True(t, gb.Attachments()[0].Raw.Negated, "the instance is negated")
		})

		t.Run("attaches a comment's carrier in the tool-directive shape as a directive-shaped instance",
			func(t *testing.T) {
				t.Parallel()

				gb := parsed(t, shapedComment)
				assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
				assert.True(t, gb.Attachments()[0].Raw.DirectiveShaped, "validation reads the shape off the instance")
			})

		t.Run("folds an on statement's methods onto a type an earlier member declared", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, sink := unitOver(f, fstest.MapFS{
				svcFile:  {Data: []byte("package svc\ntype A\n")},
				foldFile: {Data: []byte("package svc\non A\nmethod Get int\n")},
			}, svcFile, foldFile)
			assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
			coretest.AssertCodes(t, sink)
			declared := u.Graph().Packages()[0].Files[0].Decls[0].(*node.Struct)
			assert.Length(t, declared.Methods, 1, "the method folds onto the type")
			assert.Equal(t, declared.Methods[0].Pos.File, foldFile, "the method is at its own file's position")
		})

		t.Run("reports an on statement that names no declared type", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, sink := unitOver(f, fstest.MapFS{
				foldFile: {Data: []byte("package svc\non A\nmethod Get int\n")},
			}, foldFile)
			assert.NoError(t, f.Parse(t.Context(), u), "the unknown type is not fatal")
			coretest.AssertReports(t, sink, frontendtest.ScriptedBadFile)
		})

		t.Run("returns the read's own error for an absent member", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, _ := unitOver(f, fstest.MapFS{}, absentFile)
			err := f.Parse(t.Context(), u)
			assert.HasError(t, err, "the parse fails")
			assert.Contains(t, err.Error(), absentFile, "the error names the path")
		})

		broken := fstest.MapFS{
			svcFile: {Data: []byte("package svc\ntype A\n+=x\nconst low\n")},
		}

		t.Run("reports a directive outside the grammar", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, sink := unitOver(f, broken, svcFile)
			assert.NoError(t, f.Parse(t.Context(), u), "the bad directive is not fatal")
			coretest.AssertReports(t, sink, frontendtest.ScriptedBadFile)
			coretest.AssertPositioned(t, sink)
			assert.Empty(t, u.Graph().Attachments(), "nothing attaches")
		})

		t.Run("lowers the statements after a directive outside the grammar", func(t *testing.T) {
			t.Parallel()

			f := frontendtest.NewScripted()
			u, _ := unitOver(f, broken, svcFile)
			assert.NoError(t, f.Parse(t.Context(), u), "the bad directive is not fatal")
			assert.Length(t, u.Graph().Packages()[0].Files[0].Decls, 2, "the type and the constant are declared")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		f := frontendtest.NewScripted()
		scope := plugin.ImportScope{
			File:     symbol.Identity{Lang: frontendtest.ScriptedLang, Package: svcPath},
			Bindings: map[string][]string{aliasName: {depPackage}},
		}

		t.Run("probes a bound alias in its package", func(t *testing.T) {
			t.Parallel()

			got := f.Resolve(scope, boundName)
			assert.Length(t, got, 1, "the probe has one tier")
			assert.Length(t, got[0], 1, "the tier has one candidate")
			assert.Equal(t, got[0][0].Package, depPackage, "the candidate is in the bound package")
		})

		t.Run("probes a bare capital in the file's own package", func(t *testing.T) {
			t.Parallel()

			own := f.Resolve(scope, ownSpelling)
			assert.Length(t, own, 1, "the probe has one tier")
			assert.Equal(t, own[0][0].Package, svcPath, "the candidate is in the file's package")
		})

		t.Run("probes nothing for an alias the file never bound", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, f.Resolve(scope, unboundName), "the probe is empty")
		})

		t.Run("probes nothing for a builtin", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, f.Resolve(scope, builtinName), "the probe is empty")
		})

		tests := []struct {
			name     string
			spelling string
		}{
			{name: "probes a bare spelling that opens with the first capital", spelling: firstCapital},
			{name: "probes a bare spelling that opens with the last capital", spelling: lastCapital},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Length(t, f.Resolve(scope, tt.spelling), 1, "the spelling is the file package's")
			})
		}
	})

	t.Run("ScriptedKeys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the classification key it stamps", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, frontendtest.ScriptedKeys(r), "the key registers")
			_, held := r.Resolve(frontendtest.ScriptedTestKey)
			assert.True(t, held, "the key has the name the stamps write")
		})

		t.Run("returns an error naming a namespace claimed twice", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, frontendtest.ScriptedKeys(r), "the first registration succeeds")
			err := frontendtest.ScriptedKeys(r)
			assert.HasError(t, err, "the second registration fails")
			assert.Contains(t, err.Error(), "fake", "the error names the namespace")
		})
	})

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		f := frontendtest.NewScriptedDependent()
		lib := plugin.StorePath(frontendtest.ScriptedStore, extMember)
		dependencies := func(tb testing.TB, store fs.FS, needs ...string) [][]plugin.SourceRef {
			tb.Helper()

			units, err := f.Dependencies(tb.Context(), roundOf(needs...), storeReader{fsys: withStore(store)})
			assert.NoError(tb, err, "the round lists the store")
			return units
		}

		t.Run("returns the scripted files of a need's directory as one unit", func(t *testing.T) {
			t.Parallel()

			store := fstest.MapFS{extMember: {Data: []byte(extSource)}}
			assert.Equal(t, dependencies(t, store, extPath), [][]plugin.SourceRef{{{Path: lib}}},
				"the member is the qualified path of the store's file")
		})

		t.Run("leaves out a file without the scripted suffix", func(t *testing.T) {
			t.Parallel()

			store := fstest.MapFS{
				extMember: {Data: []byte(extSource)},
				extNotes:  {Data: []byte("not source\n")},
			}
			assert.Equal(t, dependencies(t, store, extPath), [][]plugin.SourceRef{{{Path: lib}}},
				"the notes are not source")
		})

		t.Run("leaves out a directory with the scripted suffix", func(t *testing.T) {
			t.Parallel()

			store := fstest.MapFS{
				extMember: {Data: []byte(extSource)},
				extNested: {Data: []byte(extSource)},
			}
			assert.Equal(t, dependencies(t, store, extPath), [][]plugin.SourceRef{{{Path: lib}}},
				"a directory is never a member")
		})

		t.Run("returns no unit for a need whose directory has no scripted file", func(t *testing.T) {
			t.Parallel()

			store := fstest.MapFS{extNotes: {Data: []byte("not source\n")}}
			assert.Empty(t, dependencies(t, store, extPath), "a unit without members has nothing to parse")
		})

		t.Run("returns no unit for a need the store has no directory for", func(t *testing.T) {
			t.Parallel()

			store := fstest.MapFS{extMember: {Data: []byte(extSource)}}
			assert.Empty(t, dependencies(t, store, absentPackage), "the store does not declare the package")
		})

		t.Run("returns no unit for a load without the store", func(t *testing.T) {
			t.Parallel()

			units, err := f.Dependencies(t.Context(), roundOf(extPath), storeReader{fsys: fstest.MapFS{}})
			assert.NoError(t, err, "a store the composition leaves out turns the source off")
			assert.Empty(t, units, "nothing is read")
		})

		unplaced := []struct {
			name  string
			store fs.FS
			need  string
			want  string
		}{
			{
				name:  "reports a need whose directory has no scripted file placed nowhere",
				store: withStore(fstest.MapFS{extNotes: {Data: []byte("not source\n")}}),
				need:  extPath,
				want:  plugin.StorePath(frontendtest.ScriptedStore, extPath) + " has no scripted file",
			},
			{
				name:  "reports a need the store has no directory for placed nowhere",
				store: withStore(fstest.MapFS{extMember: {Data: []byte(extSource)}}),
				need:  absentPackage,
				want:  "no directory is at " + plugin.StorePath(frontendtest.ScriptedStore, absentPackage),
			},
			{
				name:  "reports a need placed nowhere for a load without the store",
				store: fstest.MapFS{},
				need:  extPath,
				want:  "the load provides no " + frontendtest.ScriptedStore + " store",
			},
		}
		for _, tt := range unplaced {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				round := roundOf(tt.need)
				_, err := f.Dependencies(t.Context(), round, storeReader{fsys: tt.store})
				assert.NoError(t, err, "a need placed nowhere fails nothing")
				assert.Equal(t, round.Unplaced(), []plugin.Unplaced{{Path: tt.need, Reason: tt.want}},
					"the round records the need and why")
			})
		}

		t.Run("reports nothing for a need it places", func(t *testing.T) {
			t.Parallel()

			round := roundOf(extPath)
			store := withStore(fstest.MapFS{extMember: {Data: []byte(extSource)}})
			_, err := f.Dependencies(t.Context(), round, storeReader{fsys: store})
			assert.NoError(t, err, "the round lists the store")
			assert.Empty(t, round.Unplaced(), "the store declares the package")
		})

		t.Run("returns a listing's own error", func(t *testing.T) {
			t.Parallel()

			store := failingFS{tree: fstest.MapFS{extMember: {Data: []byte(extSource)}}, fail: extPath}
			_, err := f.Dependencies(t.Context(), roundOf(extPath), storeReader{fsys: withStore(store)})
			assert.ErrorIs(t, err, fs.ErrPermission, "the error is the store's own")
		})
	})

	t.Run("Exports", func(t *testing.T) {
		t.Parallel()

		f := frontendtest.NewScriptedExporter()

		t.Run("returns the name in each package the publishing alias binds as one tier", func(t *testing.T) {
			t.Parallel()

			scope := plugin.ImportScope{Bindings: map[string][]string{
				frontendtest.ScriptedPublish: {depPackage, extPath},
			}}
			assert.Equal(t, f.Exports(scope, ownSpelling), plugin.Candidates{{
				{Lang: frontendtest.ScriptedLang, Package: depPackage, Name: ownSpelling},
				{Lang: frontendtest.ScriptedLang, Package: extPath, Name: ownSpelling},
			}}, "a published package's name is a candidate")
		})

		t.Run("returns nothing for a file that publishes no package", func(t *testing.T) {
			t.Parallel()

			scope := plugin.ImportScope{Bindings: map[string][]string{aliasName: {depPackage}}}
			assert.Empty(t, f.Exports(scope, ownSpelling), "an import publishes nothing")
		})
	})
}

// unitOver assembles one unit over a tree under the suite's brand,
// the way the driver does.
func unitOver(f *frontendtest.Scripted, tree fstest.MapFS, files ...string) (
	*plugin.SourceUnit, *diag.Sink,
) {
	refs := make([]plugin.SourceRef, len(files))
	for i, path := range files {
		refs[i] = plugin.SourceRef{Path: path}
	}
	sink := diag.NewSink()
	return plugin.NewSourceUnit(
		refs, tree, plugin.DepthFull, f.Syntax(), string(frontendtest.Brand), sink, f.Name(),
	), sink
}

// parsed lowers one source as the single member of a unit and
// returns the unit's builder, failing the test on a finding.
func parsed(tb testing.TB, source string) *plugin.GraphBuilder {
	tb.Helper()

	f := frontendtest.NewScripted()
	u, sink := unitOver(f, fstest.MapFS{svcFile: {Data: []byte(source)}}, svcFile)
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	coretest.AssertCodes(tb, sink)
	return u.Graph()
}

// roundOf returns a first round with one need per path.
func roundOf(paths ...string) *plugin.DependencyRound {
	needs := make([]plugin.Need, len(paths))
	for i, p := range paths {
		needs[i] = plugin.Need{Path: p}
	}
	return &plugin.DependencyRound{Number: 1, Needs: needs}
}

// withStore returns an empty workspace tree with one store beside it
// under the scripted dependent's store name.
func withStore(store fs.FS) storeTree {
	return storeTree{MapFS: fstest.MapFS{}, stores: map[string]fs.FS{frontendtest.ScriptedStore: store}}
}

// storeTree is a test tree with named stores beside it.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one of the tree's stores.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, held := t.stores[name]
	return s, held
}

// storeReader is a dependency round's door over a test tree, which
// resolves a qualified path where the tree has stores.
type storeReader struct {
	fsys fs.FS
}

// Read returns one file's bytes.
func (r storeReader) Read(path string) ([]byte, error) { return plugin.ReadFile(r.fsys, path) }

// ReadDir returns one directory's entries.
func (r storeReader) ReadDir(path string) ([]fs.DirEntry, error) { return plugin.ReadDir(r.fsys, path) }

// reader is the recorded partition door over a test tree.
type reader struct {
	tree fstest.MapFS
}

// Read returns one file's bytes.
func (r reader) Read(path string) ([]byte, error) {
	return r.tree.ReadFile(path)
}
