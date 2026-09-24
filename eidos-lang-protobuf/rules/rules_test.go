// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// identityOf returns the identity of a resolved declaration.
func identityOf(tb assert.TB, sym symbol.Symbol) symbol.Identity {
	tb.Helper()

	decl, is := sym.(node.Declaration)
	assert.True(tb, is, "the resolved symbol is a declaration")
	return decl.Identity()
}

// The rules are the projections a generator reads, so the member
// policy, the roles, the builtin fold, the type-name join and the
// directive resolution are each pinned.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("states that nothing contributes members", func(t *testing.T) {
			t.Parallel()

			r := protorules.New()
			assert.Equal(t, r.Lang(), protobuf.Lang, "the language")
			assert.Empty(t, r.Members().Contributes,
				"protobuf has no embedding and no supertypes, so a message's members are its own")
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies an rpc's parameter and its returns", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			store, is := f.decl(t, id(svcPkg, "Store", symbol.KindInterface)).(*node.Interface)
			assert.True(t, is, "Store is an interface")
			b := f.bound()

			get, held := b.CallableOf(store.Methods[0])
			assert.True(t, held, "an rpc is callable")
			assert.Equal(t, get.Params[0].Role, rules.ParamInput, "its one parameter is input")
			assert.Equal(t, get.Returns[0].Role, rules.ReturnValue, "its one return is a value")
			assert.Equal(t, get.Errors, rules.ErrorsNone, "a schema states no failure in its signature")

			watch, _ := b.CallableOf(store.Methods[1])
			assert.Equal(t, watch.Returns[0].Role, rules.ReturnStream, "a streaming response is a stream")
		})
	})

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies the wire's scalars by name", func(t *testing.T) {
			t.Parallel()

			r := protorules.New()
			cases := []struct {
				spelling string
				form     symbol.TypeForm
				class    rules.ScalarClass
				bits     int
			}{
				{"int32", symbol.FormScalar, rules.ScalarInt, 32},
				{"sint32", symbol.FormScalar, rules.ScalarInt, 32},
				{"sfixed32", symbol.FormScalar, rules.ScalarInt, 32},
				{"int64", symbol.FormScalar, rules.ScalarInt, 64},
				{"sint64", symbol.FormScalar, rules.ScalarInt, 64},
				{"sfixed64", symbol.FormScalar, rules.ScalarInt, 64},
				{"uint32", symbol.FormScalar, rules.ScalarUint, 32},
				{"fixed32", symbol.FormScalar, rules.ScalarUint, 32},
				{"uint64", symbol.FormScalar, rules.ScalarUint, 64},
				{"fixed64", symbol.FormScalar, rules.ScalarUint, 64},
				{"float", symbol.FormScalar, rules.ScalarFloat, 32},
				{"double", symbol.FormScalar, rules.ScalarFloat, 64},
				{"bool", symbol.FormBool, 0, 0},
				{"string", symbol.FormText, 0, 0},
				{"bytes", symbol.FormBytes, 0, 0},
			}
			for _, tc := range cases {
				s := r.Builtin(builtin(tc.spelling), rules.View{})
				assert.Equal(t, s.Form, tc.form, tc.spelling+" folds to its form")
				assert.Equal(t, s.Class, tc.class, tc.spelling+" with the class the wire gives it")
				assert.Equal(t, s.Bits, tc.bits, tc.spelling+" and the width")
				assert.True(t, protobuf.IsScalar(tc.spelling),
					tc.spelling+" is a scalar in the grammar the frontend resolves through too")
			}
			assert.Equal(t, r.Builtin(builtin("some.Message"), rules.View{}).Form, symbol.FormOpaque,
				"a message the workspace does not declare degrades visibly")
			assert.Equal(t, r.Builtin(nil, rules.View{}).Form, symbol.FormOpaque, "nothing is opaque")
		})

		t.Run("folds the label forms the frontend states", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			b := f.bound()
			assert.Equal(t, b.TypeOf(ref(depPkg, "Target", symbol.KindStruct)).Form, symbol.FormReference,
				"a message the workspace declares folds to a reference")
			bytes := composite("repeated bytes", symbol.FormList, builtin("bytes"))
			assert.Equal(t, b.TypeOf(bytes).Form, symbol.FormList,
				"a repeated bytes field is a list of byte strings, not one byte string")

			row, _ := f.decl(t, id(svcPkg, "Row", symbol.KindStruct)).(*node.Struct)
			byName := map[string]rules.TypeShape{}
			for _, field := range row.Fields {
				byName[field.Name] = b.TypeOf(field.Type)
			}
			assert.Equal(t, byName["tags"].Form, symbol.FormList, "a repeated field is a list")
			assert.Equal(t, byName["tags"].Elems[0].Form, symbol.FormText, "over its element")
			assert.Equal(t, byName["counts"].Form, symbol.FormMap, "a map field is a map")
			assert.Equal(t, byName["counts"].Elems[1].Class, rules.ScalarInt, "over its value")
			assert.Equal(t, byName["target"].Ref, id(depPkg, "Target", symbol.KindStruct),
				"and a reference into a sibling namespace resolves to the message the other file declares")
		})
	})

	t.Run("TypeName", func(t *testing.T) {
		t.Parallel()

		t.Run("joins a type name the way a schema spells one", func(t *testing.T) {
			t.Parallel()

			r := protorules.New()
			assert.Equal(t, r.TypeName("check", "Row"), "CheckRow", "PascalCase, which every generator follows")
			assert.Equal(t, r.TypeName("check", "row_key"), "CheckRowKey", "a snake name joins the same way")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a type outward from the subject's message", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			row := rules.Scope{Subject: id(svcPkg, "Row", symbol.KindStruct)}

			got, err := r.Resolve(row, "Key", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a message nested in the subject resolves from inside it")
			assert.Equal(t, identityOf(t, got), member(svcPkg, "Row", "Key", symbol.KindStruct),
				"as the nested message")

			got, err = r.Resolve(row, "Colour", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a sibling enum resolves from the subject's namespace")
			assert.Equal(t, got.Kind(), symbol.KindEnum, "as an enum")

			got, err = r.Resolve(row, "Row.body", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a oneof resolves by its dotted name")
			assert.Equal(t, got.Kind(), symbol.KindSum, "as the sum it projects to")

			colour := rules.Scope{Subject: id(svcPkg, "Colour", symbol.KindEnum)}
			got, err = r.Resolve(colour, "Row.Key", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a nested message resolves by its dotted name from outside its message")
			assert.Equal(t, got.Kind(), symbol.KindStruct, "as a message")

			got, err = r.Resolve(row, ".dep.Target", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "a fully-qualified name states its namespace outright")
			assert.Equal(t, identityOf(t, got), id(depPkg, "Target", symbol.KindStruct),
				"and resolves to the message there")

			_, err = r.Resolve(row, "Ghost", directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "a name nothing declares refuses")
			assert.Contains(t, err.Error(), "outward", "naming the outward search")
			_, err = r.Resolve(row, "  ", directive.ResolveTypeInScope, f.view)
			assert.HasError(t, err, "an empty spelling refuses")
			_, err = r.Resolve(row, "name", directive.ResolveMetadataKey, f.view)
			assert.HasError(t, err, "and the metadata kind is validation's, not the language's")
		})

		t.Run("resolves a service and an rpc through its service", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			row := rules.Scope{Subject: id(svcPkg, "Row", symbol.KindStruct)}

			got, err := r.Resolve(row, "Store", directive.ResolveCallableInScope, f.view)
			assert.NoError(t, err, "a service resolves as a callable scope")
			assert.Equal(t, got.Kind(), symbol.KindInterface, "as an interface")

			got, err = r.Resolve(row, "Store.Get", directive.ResolveCallableInScope, f.view)
			assert.NoError(t, err, "an rpc resolves by its service's name and its own")
			assert.Equal(t, got.Kind(), symbol.KindMethod, "as a method")
			assert.Equal(t, got.(*node.Method).Name, "Get", "the one named")

			_, err = r.Resolve(row, "Store.Ghost", directive.ResolveCallableInScope, f.view)
			assert.HasError(t, err, "an rpc the service does not declare refuses")
		})

		t.Run("resolves a field among the message's fields and its oneof members", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			row := rules.Scope{Subject: id(svcPkg, "Row", symbol.KindStruct)}

			got, err := r.Resolve(row, "name", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "a declared field resolves")
			assert.Equal(t, got.Kind(), symbol.KindField, "as a field")
			got, err = r.Resolve(row, "blob", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "a oneof member is a field of the message")
			assert.Equal(t, got.(*node.Field).Name, "blob", "and resolves as one")

			text := rules.Scope{Subject: member(svcPkg, "Row.body.text", "text", symbol.KindField)}
			got, err = r.Resolve(text, "name", directive.ResolveValueField, f.view)
			assert.NoError(t, err, "a oneof member's subject finds its message through its hosts")
			assert.Equal(t, got.(*node.Field).Name, "name", "and resolves the message's field")
			got, err = r.Resolve(text, "Key", directive.ResolveTypeInScope, f.view)
			assert.NoError(t, err, "and a type resolves from inside the message")
			assert.Equal(t, identityOf(t, got), member(svcPkg, "Row", "Key", symbol.KindStruct), "the nested one")

			_, err = r.Resolve(row, "ghost", directive.ResolveValueField, f.view)
			assert.HasError(t, err, "a field the message does not declare refuses")
		})

		t.Run("refuses a subject that belongs to no message", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()

			colour := id(svcPkg, "Colour", symbol.KindEnum)
			_, err := r.Resolve(rules.Scope{Subject: colour}, "name", directive.ResolveValueField, f.view)
			assert.HasError(t, err, "a file-level enum has no field to resolve among")

			variant := member(svcPkg, "Colour", "COLOUR_RED", symbol.KindEnumVariant)
			_, err = r.Resolve(rules.Scope{Subject: variant}, "name", directive.ResolveValueField, f.view)
			assert.HasError(t, err, "an enum variant belongs to no message")
			assert.Contains(t, err.Error(), "belongs to no message", "which the refusal states")

			ghost := id(svcPkg, "Ghost", symbol.KindStruct)
			_, err = r.Resolve(rules.Scope{Subject: ghost}, "name", directive.ResolveValueField, f.view)
			assert.HasError(t, err, "a subject the view does not contain refuses")
			assert.Contains(t, err.Error(), "does not contain", "naming the view")

			orphan := member(svcPkg, "Ghost", "name", symbol.KindField)
			_, err = r.Resolve(rules.Scope{Subject: orphan}, "name", directive.ResolveValueField, f.view)
			assert.HasError(t, err, "and so does a field whose message the view does not contain")
		})

		t.Run("resolves a parameter on an rpc's own signature", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			r := protorules.New()
			store, _ := f.decl(t, id(svcPkg, "Store", symbol.KindInterface)).(*node.Interface)
			get := store.Methods[0]
			scope := rules.Scope{Subject: get.ID}

			got, err := r.Resolve(scope, "request", directive.ResolveHostParam, f.view)
			assert.NoError(t, err, "an rpc's one parameter resolves")
			assert.Equal(t, got.Kind(), symbol.KindParam, "as a parameter")
			_, err = r.Resolve(scope, "ghost", directive.ResolveHostParam, f.view)
			assert.HasError(t, err, "one it does not declare refuses")
			_, err = r.Resolve(rules.Scope{Subject: id(svcPkg, "Row", symbol.KindStruct)},
				"request", directive.ResolveHostParam, f.view)
			assert.HasError(t, err, "and a message has no parameters")
			_, err = r.Resolve(rules.Scope{Subject: id(svcPkg, "Ghost", symbol.KindMethod)},
				"request", directive.ResolveHostParam, f.view)
			assert.HasError(t, err, "nor does an rpc the view does not contain")
		})
	})
}
