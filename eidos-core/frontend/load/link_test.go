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

// holderTarget returns the target the holder's one field resolved
// to.
func holderTarget(tb assert.TB, g *store.Graph) symbol.Identity {
	tb.Helper()

	holder, held := g.Lookup(symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: holdPath, Name: holderName,
		Kind: symbol.KindStruct,
	})
	assert.True(tb, held, "the holder is indexed")
	return holder.(*node.Struct).Fields[0].Type.Target
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

		t.Run("resolves through the file's bindings and leaves builtins", func(t *testing.T) {
			t.Parallel()

			g, _, _ := loadTree(t, stdTree())
			row, _ := g.Lookup(rowID())
			fields := row.(*node.Struct).Fields

			assert.Equal(t, fields[0].Type.Target, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: apiPath, Name: userName,
				Kind: symbol.KindStruct,
			}, "a bound spelling resolves to the declaring identity")
			assert.True(t, fields[1].Type.Target.IsZero(),
				"a builtin keeps its spelling alone")
		})

		t.Run("resolves nothing in a file with no recorded scope", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, oneFileTree(), with(&everyKind{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)

			held, found := g.Lookup(assigned("", coretest.AliasName, symbol.KindAlias))
			assert.True(t, found, "the alias is indexed")
			assert.True(t, held.(*node.Alias).Target.Target.IsZero(),
				"the file records no bindings, so the reference resolves to nothing")
			assert.Equal(t, held.(*node.Alias).Target.Spelling, coretest.StructName,
				"and the reference keeps what the frontend wrote")
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
				"and so does a member nested in the type")

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

		t.Run("reports an ambiguous reference and keeps the first candidate", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, dualTree(leftPath, rightPath))
			coretest.AssertReports(t, sink, load.AmbiguousReference)
			assert.Equal(t, holderTarget(t, g), thingIn(leftPath),
				"the first candidate in probe order is the target")

			found, _ := findingOf(sink, load.AmbiguousReference)
			assert.Contains(t, found.Msg, leftPath, "naming the target")
			assert.Contains(t, found.Msg, rightPath, "and the other candidate")
		})

		t.Run("lets an earlier tier shadow a later one without an ambiguity", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, dualTree(leftPath, rightPath), with(&tiered{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderTarget(t, g), thingIn(leftPath),
				"the first tier with a candidate in the graph decides")
		})

		t.Run("passes over a tier with no candidate in the graph", func(t *testing.T) {
			t.Parallel()

			g, _, sink := loadTree(t, dualTree(missingPath, rightPath), with(&tiered{frontendtest.NewScripted()}))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, holderTarget(t, g), thingIn(rightPath),
				"a later tier decides where every earlier one names nothing the graph contains")
		})
	})
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
