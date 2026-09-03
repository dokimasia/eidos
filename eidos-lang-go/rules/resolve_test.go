// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	r := gorules.New()
	row := id(fxPath, "Row", symbol.KindStruct)

	t.Run(
		"resolves a bare callable and a package variable in the subject's package",
		func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, err := r.Resolve(f.scope(row), "Find", directive.ResolveCallableInScope, f.view)
			assert.NoError(t, err, "Find is declared beside Row")
			assert.Equal(t, got.(node.Declaration).Identity().Name, "Find", "and is what arrives")
			got, err = r.Resolve(f.scope(row), "Registry", directive.ResolvePackageVar, f.view)
			assert.NoError(t, err, "Registry is a package variable")
			assert.Equal(t, got.Kind(), symbol.KindVariable, "of the variable kind")
			got, err = r.Resolve(f.scope(row), "Limit", directive.ResolvePackageVar, f.view)
			assert.NoError(t, err, "a constant is a package-level binding too")
			assert.Equal(t, got.Kind(), symbol.KindConstant, "of the constant kind")
			_, err = r.Resolve(f.scope(row), "Ghost", directive.ResolveCallableInScope, f.view)
			assert.HasError(t, err, "a name nothing declares refuses")
			assert.Contains(t, err.Error(), "Ghost", "naming the spelling")
		},
	)

	t.Run("resolves a qualified name through the file's imports", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		got, err := r.Resolve(f.scope(row), "dep.Target", directive.ResolveTypeInScope, f.view)
		assert.NoError(t, err, "the import binds dep")
		assert.Equal(t, got.(node.Declaration).Identity(), id(depPath, "Target", symbol.KindStruct),
			"to the sibling's declaration")
		_, err = r.Resolve(f.scope(row), "ghost.Target", directive.ResolveTypeInScope, f.view)
		assert.HasError(t, err, "a qualifier no import binds refuses")
		_, err = r.Resolve(f.scope(row), "dep.Ghost", directive.ResolveCallableInScope, f.view)
		assert.HasError(t, err, "a name the sibling does not declare refuses")
	})

	t.Run("stands in for a builtin and for a type outside the workspace", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		got, err := r.Resolve(f.scope(row), "int", directive.ResolveTypeInScope, f.view)
		assert.NoError(t, err, "a builtin names itself")
		assert.Equal(t, got.(node.Declaration).Identity(),
			symbol.Identity{Lang: golang.Lang, Name: "int", Kind: symbol.KindAlias},
			"with no package, which is how a witness spells a builtin")
		got, err = r.Resolve(f.scope(row), "time.Duration", directive.ResolveTypeInScope, f.view)
		assert.NoError(t, err, "the standard library is never in the graph")
		assert.Equal(t, got.(node.Declaration).Identity().Package, "time",
			"so the type is named by the path a backend imports")
	})

	t.Run("resolves a value field and a member through the subject's type", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		got, err := r.Resolve(f.scope(row), "ID", directive.ResolveValueField, f.view)
		assert.NoError(t, err, "ID is a field of Row")
		assert.Equal(t, got.Kind(), symbol.KindField, "and arrives as one")
		field := f.field(t, "Row", "ID")
		got, err = r.Resolve(f.scope(field.ID), "Rename", directive.ResolveMemberOnHandle, f.view)
		assert.NoError(t, err, "a member subject resolves on its host's type")
		assert.Equal(t, got.Kind(), symbol.KindMethod, "a method is a member")
		derived := id(fxPath, "Derived", symbol.KindStruct)
		got, err = r.Resolve(f.scope(derived), "Kind", directive.ResolveValueField, f.view)
		assert.NoError(t, err, "a promoted field resolves through the member walk")
		assert.Equal(
			t,
			got.(node.Declaration).Identity().Owner,
			"Base",
			"on the type that declares it",
		)
		_, err = r.Resolve(f.scope(row), "Rename", directive.ResolveValueField, f.view)
		assert.HasError(t, err, "a method is no field")
		_, err = r.Resolve(
			f.scope(id(fxPath, "Find", symbol.KindFunction)),
			"ID",
			directive.ResolveValueField,
			f.view,
		)
		assert.HasError(t, err, "a function belongs to no type")
	})

	t.Run("resolves a host parameter on the subject's own signature", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		load := symbol.Identity{
			Lang: golang.Lang, Package: fxPath, Name: "Load", Kind: symbol.KindFunction,
			Disc: "context.Context,int",
		}
		got, err := r.Resolve(f.scope(load), "id", directive.ResolveHostParam, f.view)
		assert.NoError(t, err, "id is a parameter of Load")
		assert.Equal(t, got.Kind(), symbol.KindParam, "and arrives as one")
		_, err = r.Resolve(f.scope(load), "ghost", directive.ResolveHostParam, f.view)
		assert.HasError(t, err, "a parameter Load does not declare refuses")
		_, err = r.Resolve(f.scope(row), "id", directive.ResolveHostParam, f.view)
		assert.HasError(t, err, "a struct has no parameters")
	})

	t.Run("refuses what it cannot read", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		_, err := r.Resolve(f.scope(row), "  ", directive.ResolveTypeInScope, f.view)
		assert.HasError(t, err, "nothing resolves to nothing")
		_, err = r.Resolve(f.scope(row), "ID", directive.ResolveMetadataKey, f.view)
		assert.HasError(t, err, "the metadata kind is validation's, not the language's")
		ghost := id(fxPath, "Ghost", symbol.KindStruct)
		_, err = r.Resolve(f.scope(ghost), "ID", directive.ResolveValueField, f.view)
		assert.HasError(t, err, "a subject the view does not hold refuses")
		_, err = r.Resolve(
			rules.Scope{Subject: row},
			"dep.Target",
			directive.ResolveTypeInScope,
			f.view,
		)
		assert.HasError(t, err, "a qualified name without a file has no imports to read")
	})
}
