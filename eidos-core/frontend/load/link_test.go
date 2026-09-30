// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
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
