// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// The resolution step turns spellings into identities through each
// language's own bindings, and its degradations are pinned beside
// its successes.
func TestLink(t *testing.T) {
	t.Parallel()

	t.Run("resolves through the file's bindings and leaves builtins", func(t *testing.T) {
		t.Parallel()

		g, _, _ := loadTree(t, stdTree())
		row, _ := g.Lookup(rowID())
		fields := row.(*node.Struct).Fields

		assert.Equal(t, fields[0].Type.Target, symbol.Identity{
			Lang: fakeLang, Package: "svc/api", Name: "User", Kind: symbol.KindStruct,
		}, "a bound spelling resolves to the declaring identity")
		assert.True(t, fields[1].Type.Target.IsZero(),
			"a builtin keeps its spelling alone")
	})

	t.Run("reports an ambiguous reference and keeps the first candidate", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			"a/left/l.zz":   {Data: []byte("package a/left\ntype Thing string\n")},
			"a/right/r.zz":  {Data: []byte("package a/right\ntype Thing string\n")},
			"svc/hold/h.zz": {Data: []byte("package svc/hold\nimport dual a/left a/right\ntype Holder dual.Thing\n")},
		}
		g, _, sink := loadTree(t, tree)
		holder, _ := g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/hold", Name: "Holder", Kind: symbol.KindStruct,
		})
		assert.Equal(t, holder.(*node.Struct).Fields[0].Type.Target, symbol.Identity{
			Lang: fakeLang, Package: "a/left", Name: "Thing", Kind: symbol.KindStruct,
		}, "the first candidate in probe order stands")

		named := false
		for d := range sink.All() {
			if d.Code != load.AmbiguousReference {
				continue
			}
			named = true
			assert.Contains(t, d.Msg, "a/left", "naming the survivor")
			assert.Contains(t, d.Msg, "a/right", "and the rival")
		}
		assert.True(t, named, "several present candidates report")
	})
}
