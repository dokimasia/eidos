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
	"go.dokimi.dev/eidos/core/symbol"
)

// The paths and names the resolution cases build their own tree
// from: two packages declaring one spelling, and the file binding an
// alias to both.
const (
	leftPath   = "a/left"
	rightPath  = "a/right"
	holdPath   = "svc/hold"
	thingName  = "Thing"
	holderName = "Holder"
)

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
				"there are no bindings to resolve through, so the spelling stays a spelling")
			assert.Equal(t, held.(*node.Alias).Target.Spelling, coretest.StructName,
				"and the reference keeps what the frontend wrote")
		})
	})

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("reports an ambiguous reference and keeps the first candidate", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"a/left/l.zz":  {Data: []byte("package a/left\ntype Thing string\n")},
				"a/right/r.zz": {Data: []byte("package a/right\ntype Thing string\n")},
				"svc/hold/h.zz": {
					Data: []byte("package svc/hold\nimport dual a/left a/right\ntype Holder dual.Thing\n"),
				},
			}
			g, _, sink := loadTree(t, tree)
			coretest.AssertReports(t, sink, load.AmbiguousReference)

			holder, _ := g.Lookup(symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: holdPath, Name: holderName,
				Kind: symbol.KindStruct,
			})
			assert.Equal(t, holder.(*node.Struct).Fields[0].Type.Target, symbol.Identity{
				Lang: frontendtest.ScriptedLang, Package: leftPath, Name: thingName,
				Kind: symbol.KindStruct,
			}, "the first candidate in probe order stands")

			found, _ := findingOf(sink, load.AmbiguousReference)
			assert.Contains(t, found.Msg, leftPath, "naming the survivor")
			assert.Contains(t, found.Msg, rightPath, "and the rival")
		})
	})
}
