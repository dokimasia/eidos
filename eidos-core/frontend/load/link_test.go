// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The paths and names the resolution cases build their own tree
// from: two packages declaring one spelling, and the file binding an
// alias to both.
const (
	leftPath    = "a/left"
	rightPath   = "a/right"
	missingPath = "a/missing"
	holdPath    = "svc/hold"
	thingName   = "Thing"
	holderName  = "Holder"
)

// The generic tree: a package-level type and a type parameter that
// share one spelling, a generic type declaring the parameter, and a
// sibling type that does not.
const (
	genPath       = "svc/gen"
	genFile       = "svc/gen/box.zz"
	boxName       = "Box"
	plainName     = "Plain"
	typeParamName = "T"
	genSource     = "package svc/gen\ntype T string\ntype Box T\ntypeparam T\nmethod Put T\ntype Plain T\n"
)

// The inline body that types each struct's first field under inlined:
// its spelling, and the name of its one field and its one method.
const (
	inlineSpelling = "{member}"
	memberName     = "member"
)

// The import trees: one file declaring both ends of a reference, and
// one package declaring them in two files.
const (
	localPath  = "svc/local"
	localFile  = "svc/local/a.zz"
	splitPath  = "svc/split"
	splitFile  = "svc/split/a.zz"
	targetFile = "svc/split/b.zz"
)

// The composition that splits the dual tree between two frontends of
// one language: the importer claims the holder, and a frontend
// without the role claims the two declaring packages.
const (
	holdSelection = "svc/hold/*.zz"
	dualSelection = "a/**/*.zz"
	plainID       = plugin.ID("plain")
)

// importMark leads every import the importing frontend names, so a
// case tells the value ImportOf returns from the path it receives.
const importMark = "import:"

// The fold tree: a holder declared in one file, and a method another
// file of the package folds onto it through an alias that only the
// second file binds.
const (
	foldPath       = "svc/fold"
	foldHolderFile = "svc/fold/a.zz"
	foldMethodFile = "svc/fold/b.zz"
)

// The fold tree's variants: the method's file declares another
// package, the shape of a Rust impl block in another module, and the
// file name a line directive puts in a position, which no tree has.
const (
	elsewherePath = "svc/elsewhere"
	linedFile     = "gen.y"
)

// The re-export trees: a barrel package whose files publish other
// packages, a second barrel whose file publishes the left package,
// and a barrel that publishes the first one back.
const (
	barrelPath = "svc/barrel"
	barrelFile = "svc/barrel/index.zz"
	moreFile   = "svc/barrel/more.zz"
	innerPath  = "svc/inner"
	innerFile  = "svc/inner/index.zz"
	loopPath   = "svc/loop"
	loopFile   = "svc/loop/index.zz"
)

// The resolution step turns spellings into identities through each
// language's own bindings, and its degradations are pinned beside
// its successes.
func TestLink(t *testing.T) {
	t.Parallel()

	t.Run("link", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a bound spelling to the declaring identity", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			row, _ := g.Lookup(rowID())
			assert.Equal(t, row.(*node.Struct).Fields[0].Type.Target, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Name: userName,
				Kind: symbol.KindStruct,
			}, "the target is the declaration the binding names")
		})

		t.Run("leaves a builtin unresolved", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			row, _ := g.Lookup(rowID())
			assert.True(t, row.(*node.Struct).Fields[1].Type.Target.IsZero(), "a builtin keeps its spelling alone")
		})

		t.Run("resolves nothing in a file with no recorded scope", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, oneFileTree(), with(&everyKind{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)

			found, indexed := g.Lookup(assigned("", coretest.AliasName, symbol.KindAlias))
			assert.True(t, indexed, "the alias is indexed")
			assert.True(t, found.(*node.Alias).Target.Target.IsZero(),
				"the file records no bindings, so the reference resolves to nothing")
			assert.Equal(t, found.(*node.Alias).Target.Spelling, coretest.StructName,
				"the reference keeps what the frontend wrote")
		})

		t.Run("targets a type parameter before a package type of its spelling", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, fstest.MapFS{genFile: {Data: []byte(genSource)}})
			coretest.AssertCodes(t, sink)
			param := genDecl(boxName, typeParamName, symbol.KindTypeParam)

			box, _ := g.Lookup(genDecl("", boxName, symbol.KindStruct))
			assert.Equal(t, box.(*node.Struct).Fields[0].Type.Target, param,
				"a field of the generic type names its parameter")
			assert.Equal(t, box.(*node.Struct).Methods[0].Params[0].Type.Target, param,
				"a member nested in the type names it too")

			plain, _ := g.Lookup(genDecl("", plainName, symbol.KindStruct))
			assert.Equal(t, plain.(*node.Struct).Fields[0].Type.Target,
				genDecl("", typeParamName, symbol.KindStruct),
				"a sibling type sees no parameter of another declaration")
		})

		t.Run("resolves a reference among an inline body's members", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, stdTree(), with(&inlined{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			row, _ := g.Lookup(rowID())
			assert.Equal(t, row.(*node.Struct).Fields[0].Type.Fields[0].Type.Target, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Name: userName, Kind: symbol.KindStruct,
			}, "the body's field resolves under the type that declares the typed field")
		})

		t.Run("targets nothing for a spelling of an inline method's type parameter", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, fstest.MapFS{genFile: {Data: []byte(genSource)}},
				with(&inlined{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			plain, _ := g.Lookup(genDecl("", plainName, symbol.KindStruct))
			assert.True(t, plain.(*node.Struct).Fields[0].Type.Methods[0].Params[0].Type.Target.IsZero(),
				"the parameter shadows the package type T and has no identity to target")
		})

		t.Run("resolves a folded method's reference through its own file's bindings", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, foldTree())
			coretest.AssertCodes(t, sink)
			holder, found := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: foldPath, Name: holderName, Kind: symbol.KindStruct,
			})
			assert.True(t, found, "the holder is indexed")
			assert.Equal(t, holder.(*node.Struct).Methods[0].Params[0].Type.Target, thingIn(leftPath),
				"the file that writes the method binds the alias, and the holder's file does not")
		})

		t.Run("resolves a folded method's reference through its own file's bindings in another package",
			func(t *testing.T) {
				t.Parallel()

				g, _, sink := loadTree(t, elsewhereTree())
				coretest.AssertCodes(t, sink)
				holder, found := g.Lookup(symbol.Identity{
					Lang: frontendtest.ScriptedLang, Package: foldPath, Name: holderName, Kind: symbol.KindStruct,
				})
				assert.True(t, found, "the holder is indexed")
				assert.Equal(t, holder.(*node.Struct).Methods[0].Params[0].Type.Target, thingIn(leftPath),
					"the method's file declares another package and binds the alias")
			})

		t.Run("resolves a declaration whose position names no recorded file under its enclosing scope",
			func(t *testing.T) {
				t.Parallel()

				tree := foldTree()
				tree[foldHolderFile] = &fstest.MapFile{Data: []byte("package " + foldPath + "\nimport dep " +
					leftPath + "\ntype " + holderName + "\nmethod Put dep." + thingName + "\n")}
				delete(tree, foldMethodFile)
				g, _, sink := loadTree(t, tree, with(&relined{frontendtest.NewScripted()}))
				coretest.AssertCodes(t, sink)
				holder, _ := g.Lookup(symbol.Identity{
					Lang: frontendtest.ScriptedLang, Package: foldPath, Name: holderName, Kind: symbol.KindStruct,
				})
				method := holder.(*node.Struct).Methods[0]
				assert.Equal(t, method.Pos.File, linedFile, "the method's position names the directive's file")
				assert.Equal(t, method.Params[0].Type.Target, thingIn(leftPath),
					"the holder's file binds the alias the method spells")
			})

		t.Run("hands Resolve the innermost enclosing type as the owner", func(t *testing.T) {
			t.Parallel()

			rec := &ownerRecorder{Scripted: frontendtest.NewScripted(), owners: map[string]symbol.Identity{}}
			loadTree(t, stdTree(), with(rec))
			assert.Equal(t, rec.owners["api.User"], rowID(),
				"a field's reference resolves under the type that declares the field")
		})
	})

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("reports nothing for a reference with one candidate in the graph", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, stdTree())
			row, _ := g.Lookup(rowID())
			assert.False(t, row.(*node.Struct).Fields[0].Type.Target.IsZero(),
				"the bound spelling resolved")
			coretest.AssertCodes(t, sink)
		})

		t.Run("reports AmbiguousReference for two candidates in one tier", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, dualTree(leftPath, rightPath))
			coretest.AssertReports(t, sink, load.AmbiguousReference)

			found, _ := findingOf(sink, load.AmbiguousReference)
			assert.Contains(t, found.Msg, leftPath, "the finding names the target")
			assert.Contains(t, found.Msg, rightPath, "the finding names the other candidate")
		})

		t.Run("targets the first candidate of an ambiguous tier", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, dualTree(leftPath, rightPath))
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(leftPath),
				"the first candidate in probe order is the target")
		})

		t.Run("lets an earlier tier shadow a later one without an ambiguity", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, dualTree(leftPath, rightPath), with(&tiered{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(leftPath),
				"the first tier with a candidate in the graph decides")
		})

		t.Run("passes over a tier with no candidate in the graph", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, dualTree(missingPath, rightPath), with(&tiered{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(rightPath),
				"a later tier decides where every earlier one names nothing the graph contains")
		})

		t.Run("sets the package from ImportOf for a declaration of another package", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, stdTree(), with(&importing{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			row, _ := g.Lookup(rowID())
			assert.Equal(t, row.(*node.Struct).Fields[0].Type.Package, importMark+apiFile,
				"the import the frontend names for the declaring file")
		})

		t.Run("sets the package from ImportOf for a declaration of another file of the package", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, splitTree(), with(&importing{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderRef(t, g, splitPath).Package, importMark+targetFile,
				"an import names a file, so one package's two files import each other")
		})

		t.Run("leaves the package empty for a declaration of the reference's own file", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, localTree(), with(&importing{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			ref := holderRef(t, g, localPath)
			assert.Equal(t, ref.Target, thingIn(localPath), "the reference resolves")
			assert.Empty(t, ref.Package, "a file needs no import of itself")
		})

		t.Run("leaves the package empty for a frontend without the importer role", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			row, _ := g.Lookup(rowID())
			assert.Empty(t, row.(*node.Struct).Fields[0].Type.Package,
				"the load records no declaring file for the language")
		})

		t.Run("leaves the package empty for a declaration a plain frontend loaded", func(t *testing.T) {
			t.Parallel()

			own := &importing{frontendtest.NewScripted()}
			own.Sel = []string{holdSelection}
			plain := frontendtest.NewScripted()
			plain.ID = plainID
			plain.Sel = []string{dualSelection}
			g, _, sink := loadTree(t, dualTree(leftPath), with(own, plain))
			coretest.AssertCodes(t, sink)
			ref := holderRef(t, g, holdPath)
			assert.Equal(t, ref.Target, thingIn(leftPath), "the reference resolves across the two frontends")
			assert.Empty(t, ref.Package, "the load records no declaring file for the other frontend's declarations")
		})
	})

	t.Run("hitsOf", func(t *testing.T) {
		t.Parallel()

		t.Run("follows a re-export to the declaration it publishes", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, barrelTree(leftPath), with(frontendtest.NewScriptedExporter()))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(leftPath),
				"the barrel declares no Thing and publishes the left package's")
		})

		t.Run("follows no re-export for a frontend outside the exporter role", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, barrelTree(leftPath))
			assert.True(t, holderRef(t, g, holdPath).Target.IsZero(), "the barrel declares no Thing")
		})

		t.Run("follows a re-export of a re-export", func(t *testing.T) {
			t.Parallel()

			tree := barrelTree(innerPath)
			tree[innerFile] = &fstest.MapFile{Data: publishing(innerPath, leftPath)}
			g, _, _ := loadTree(t, tree, with(frontendtest.NewScriptedExporter()))
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(leftPath),
				"the inner barrel publishes what the outer one publishes")
		})

		t.Run("reports AmbiguousReference for a name two published packages declare", func(t *testing.T) {
			t.Parallel()

			_, _, sink := loadTree(t, barrelTree(leftPath, rightPath), with(frontendtest.NewScriptedExporter()))
			coretest.AssertReports(t, sink, load.AmbiguousReference)
		})

		t.Run("resolves nothing through a cycle of re-exports", func(t *testing.T) {
			t.Parallel()

			tree := barrelTree(loopPath)
			tree[loopFile] = &fstest.MapFile{Data: publishing(loopPath, barrelPath)}
			g, _, _ := loadTree(t, tree, with(frontendtest.NewScriptedExporter()))
			assert.True(t, holderRef(t, g, holdPath).Target.IsZero(),
				"following stops where the path meets the barrel's name again")
		})

		t.Run("decides by the first file whose re-exports resolve", func(t *testing.T) {
			t.Parallel()

			tree := barrelTree(missingPath)
			tree[moreFile] = &fstest.MapFile{Data: publishing(barrelPath, leftPath)}
			g, _, _ := loadTree(t, tree, with(frontendtest.NewScriptedExporter()))
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(leftPath),
				"the first file publishes nothing the graph declares")
		})

		t.Run("decides by the first file in path order whatever the partition's order", func(t *testing.T) {
			t.Parallel()

			tree := barrelTree(rightPath)
			tree[moreFile] = &fstest.MapFile{Data: publishing(barrelPath, leftPath)}
			g, _, sink := loadTree(t, tree, with(&reversedExporter{frontendtest.NewScriptedExporter()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderRef(t, g, holdPath).Target, thingIn(rightPath),
				"the barrel's index file sorts before its second file")
		})

		t.Run("follows no re-export for a candidate that names a member", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, barrelTree(leftPath), with(&memberExporter{frontendtest.NewScriptedExporter()}))
			assert.True(t, holderRef(t, g, holdPath).Target.IsZero(), "no file re-exports a type's member")
		})
	})
}

// dualTree returns two packages declaring Thing and a holder whose
// one field spells it through an alias bound to both paths.
func dualTree(bound ...string) fstest.MapFS {
	imports := "import dual"
	for _, path := range bound {
		imports += " " + path
	}
	return fstest.MapFS{
		"a/left/l.zz":  {Data: []byte("package a/left\ntype Thing string\n")},
		"a/right/r.zz": {Data: []byte("package a/right\ntype Thing string\n")},
		"svc/hold/h.zz": {
			Data: []byte("package svc/hold\n" + imports + "\ntype Holder dual.Thing\n"),
		},
	}
}

// foldTree returns the fold tree and the package the method's alias
// names.
func foldTree() fstest.MapFS {
	return fstest.MapFS{
		"a/left/l.zz":  {Data: []byte("package " + leftPath + "\ntype " + thingName + " string\n")},
		foldHolderFile: {Data: []byte("package " + foldPath + "\ntype " + holderName + "\n")},
		foldMethodFile: {Data: []byte("package " + foldPath + "\nimport dep " + leftPath +
			"\non " + holderName + "\nmethod Put dep." + thingName + "\n")},
	}
}

// elsewhereTree returns the fold tree with the method's file in
// another package than the holder's.
func elsewhereTree() fstest.MapFS {
	tree := foldTree()
	tree[foldMethodFile] = &fstest.MapFile{Data: []byte("package " + elsewherePath + "\nimport dep " + leftPath +
		"\non " + holderName + "\nmethod Put dep." + thingName + "\n")}
	return tree
}

// publishing returns the source of a file of one package that
// publishes the given packages.
func publishing(pkg string, published ...string) []byte {
	return []byte("package " + pkg + "\nimport " + frontendtest.ScriptedPublish + " " +
		strings.Join(published, " ") + "\n")
}

// barrelTree returns the dual tree with the holder's alias bound to
// the barrel package, whose one file publishes the given packages.
func barrelTree(published ...string) fstest.MapFS {
	tree := dualTree(barrelPath)
	tree[barrelFile] = &fstest.MapFile{Data: publishing(barrelPath, published...)}
	return tree
}

// localTree returns one file declaring a holder and the type its one
// field spells.
func localTree() fstest.MapFS {
	return fstest.MapFS{
		localFile: {Data: []byte("package " + localPath + "\ntype " + holderName + " " + thingName +
			"\ntype " + thingName + " string\n")},
	}
}

// splitTree returns one package declaring a holder in one file and
// the type its one field spells in another.
func splitTree() fstest.MapFS {
	return fstest.MapFS{
		splitFile:  {Data: []byte("package " + splitPath + "\ntype " + holderName + " " + thingName + "\n")},
		targetFile: {Data: []byte("package " + splitPath + "\ntype " + thingName + " string\n")},
	}
}

// holderRef returns the reference the one field of a package's
// holder spells.
func holderRef(tb assert.TB, g *store.Graph, path string) *node.TypeRef {
	tb.Helper()

	holder, found := g.Lookup(symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: path, Name: holderName,
		Kind: symbol.KindStruct,
	})
	assert.True(tb, found, "the holder is indexed")
	return holder.(*node.Struct).Fields[0].Type
}

// thingIn returns the identity of the Thing one package declares.
func thingIn(path string) symbol.Identity {
	return symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: path, Name: thingName, Kind: symbol.KindStruct,
	}
}

// genDecl returns the identity of one declaration of the generic
// tree.
func genDecl(owner, name string, kind symbol.Kind) symbol.Identity {
	return symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: genPath, Owner: owner, Name: name, Kind: kind,
	}
}

// relined is the scripted language with every method's position
// moved into a file no tree has, the shape of a Go line directive
// that names the source a file was generated from.
type relined struct {
	*frontendtest.Scripted
}

// Parse lowers the unit, then moves every method's position.
func (f *relined) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	for _, pkg := range u.Graph().Packages() {
		node.Walk(pkg, func(s symbol.Symbol) bool {
			if m, is := s.(*node.Method); is {
				m.Pos.File = linedFile
			}
			return true
		})
	}
	return nil
}

// inlined is the scripted language with each struct's first field
// typed by an inline body, the shape of a TypeScript object type. The
// body's one field has the type the source wrote, and its one method
// declares the type parameter T and takes a parameter of type T.
type inlined struct {
	*frontendtest.Scripted
}

// Parse lowers the unit, then moves each struct's first field type
// into an inline body.
func (f *inlined) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	for _, pkg := range u.Graph().Packages() {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				st, is := decl.(*node.Struct)
				if !is || len(st.Fields) == 0 {
					continue
				}
				field := st.Fields[0]
				field.Type = &node.TypeRef{
					Spelling: inlineSpelling, Pos: field.Pos, Form: symbol.FormInline,
					Fields: []*node.Field{{Name: memberName, Pos: field.Pos, Type: field.Type}},
					Methods: []*node.Method{{
						Name: memberName, Pos: field.Pos,
						TypeParams: []*node.TypeParam{{Name: typeParamName, Pos: field.Pos}},
						Params: []*node.Param{{
							Name: memberName, Pos: field.Pos,
							Type: &node.TypeRef{Spelling: typeParamName, Pos: field.Pos},
						}},
					}},
				}
			}
		}
	}
	return nil
}

// reversedExporter is the scripted language in the exporter role
// whose partition lists each unit's members in reverse path order.
type reversedExporter struct {
	*frontendtest.ScriptedExporter
}

// Partition reverses the members of each scripted unit.
func (f *reversedExporter) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	parts, err := f.ScriptedExporter.Partition(ctx, files, r)
	for _, part := range parts {
		slices.Reverse(part)
	}
	return parts, err
}

// memberExporter is the scripted language in the exporter role whose
// every candidate names a member of the holder, the shape of a
// language that spells a nested type through its owner.
type memberExporter struct {
	*frontendtest.ScriptedExporter
}

// Resolve returns the scripted candidates, each a member of the
// holder.
func (f *memberExporter) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	tiers := f.ScriptedExporter.Resolve(scope, spelling)
	for _, tier := range tiers {
		for i := range tier {
			tier[i].Owner = holderName
		}
	}
	return tiers
}

// importing is the scripted language in the importer role. Its
// import of a file is the file's path behind importMark.
type importing struct {
	*frontendtest.Scripted
}

// ImportOf returns the file's path behind importMark.
func (*importing) ImportOf(_ plugin.ImportScope, file string) string {
	return importMark + file
}

// tiered offers every candidate the scripted language probes in a
// tier of its own, in probe order: the shape of a language whose
// scopes nest.
type tiered struct {
	*frontendtest.Scripted
}

// Resolve splits the scripted tier into one tier per candidate.
func (f *tiered) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	var out plugin.Candidates
	for _, tier := range f.Scripted.Resolve(scope, spelling) {
		for _, c := range tier {
			out = append(out, []symbol.Identity{c})
		}
	}
	return out
}

// ownerRecorder records the owner each spelling resolves under, so
// a case can check the scope the resolution step hands a language.
// The step calls Resolve from one goroutine, so the map needs no
// lock.
type ownerRecorder struct {
	*frontendtest.Scripted
	owners map[string]symbol.Identity
}

// Resolve records the owner and returns the scripted candidates.
func (f *ownerRecorder) Resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	f.owners[spelling] = scope.Owner
	return f.Scripted.Resolve(scope, spelling)
}
