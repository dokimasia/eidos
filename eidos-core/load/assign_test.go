// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Identities are the join everything long-lived keys on, so the
// canonical shapes the assignment step spells are pinned here.
func TestIdentities(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{
		"svc/api/user.zz": {Data: []byte(
			"package svc/api\ntype User string\nmethod Get string int\nmethod Get\n",
		)},
	}

	t.Run("spells the canonical forms", func(t *testing.T) {
		t.Parallel()

		g, _, _ := loadTree(t, tree)
		user := symbol.Identity{
			Lang: fakeLang, Package: "svc/api", Name: "User", Kind: symbol.KindStruct,
		}
		_, held := g.Lookup(user)
		assert.True(t, held, "a top-level declaration is lang:package.Name")

		field, held := g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/api", Owner: "User", Name: "f0", Kind: symbol.KindField,
		})
		assert.True(t, held, "a member is lang:package.Owner#Name")
		assert.Equal(t, field.(*node.Field).Host, user, "its host the standing owner")
	})

	t.Run("spells overloads apart by their discriminator", func(t *testing.T) {
		t.Parallel()

		g, _, sink := loadTree(t, tree)
		wide, held := g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/api", Owner: "User", Name: "Get",
			Kind: symbol.KindMethod, Disc: "string,int",
		})
		assert.True(t, held, "the parameter spellings discriminate")
		assert.Length(t, wide.(*node.Method).Params, 2, "the wide overload")

		_, held = g.Lookup(symbol.Identity{
			Lang: fakeLang, Package: "svc/api", Owner: "User", Name: "Get",
			Kind: symbol.KindMethod,
		})
		assert.True(t, held, "the nullary overload spells an empty discriminator")
		assert.False(t, carries(sink, load.DuplicateDeclaration),
			"two overloads are two declarations")
	})

	t.Run("panics on a symbol outside the node model", func(t *testing.T) {
		t.Parallel()

		evil := &foreign{fake: newFake()}
		assert.Panics(t, func() {
			_, _, _ = load.Load(context.Background(), load.Config{
				FS:        fstest.MapFS{"p/x.zz": {Data: []byte("ignored\n")}},
				Frontends: []plugin.Frontend{evil},
				Sink:      diag.NewSink(),
			})
		}, "a malformed graph is the frontend's defect, found at the splice")
	})
}

// foreign builds a declaration the node model does not admit at
// file level.
type foreign struct {
	*fake
}

// Parse ignores the source and plants a Param in the file's
// declarations.
func (*foreign) Parse(_ context.Context, u *plugin.SourceUnit) error {
	pkg := u.Graph().Package("p")
	pkg.Files = append(pkg.Files, &node.File{
		Path:  u.Files()[0].Path,
		Decls: node.Symbols{&node.Param{Name: "loose"}},
	})
	return nil
}
