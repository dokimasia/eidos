// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The imports the signature cases name: the standard library's
// context package, and a workspace package whose last element is
// context too.
const (
	contextPath    = "context"
	foreignContext = "example.test/context"
)

// param returns a parameter typed by a reference.
func param(ref *node.TypeRef) *node.Param { return &node.Param{Type: ref} }

// returnsOf returns one return per reference, in order.
func returnsOf(refs ...*node.TypeRef) []*node.Return {
	out := make([]*node.Return, 0, len(refs))
	for _, ref := range refs {
		out = append(out, &node.Return{Type: ref})
	}
	return out
}

// The rules value classifies a signature and a member walk for the
// kernel, so each classification is pinned.
func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Go", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().Lang(), golang.Lang, "the language")
		})
	})

	t.Run("Members", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a policy that walks embeds under promotion", func(t *testing.T) {
			t.Parallel()

			policy := gorules.New().Members()
			assert.Equal(t, policy.Contributes, []rules.Contribution{rules.ContributesEmbeds}, "embeds contribute")
			assert.Equal(t, policy.Shadowing, rules.ShadowPromote, "a shallower member shadows a deeper one")
			assert.Equal(t, policy.Depth, 0, "to the kernel's default depth")
			assert.True(t, policy.EmbedsAreFields, "an embedded field is a member")
		})

		t.Run("returns a policy that records an embedded field beside the members it promotes", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			derived := f.decl(t, id(fxPath, "Derived", symbol.KindStruct))
			set, is := f.bound().MembersOf(derived)
			assert.True(t, is, "a struct walks")
			var names []string
			for _, m := range set.Members {
				switch d := m.Symbol.(type) {
				case *node.Field:
					names = append(names, d.Name)
				case *node.Embed:
					names = append(names, d.ID.Name)
				}
			}
			assert.Equal(t, names, []string{"Name", "Base", "Kind"},
				"Derived's field, its embedded field Base, then the Kind that Base promotes")
		})
	})

	t.Run("ParamRole", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies a context.Context parameter as the context", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			load, is := f.decl(t, id(fxPath, "Load", symbol.KindFunction)).(*node.Function)
			assert.True(t, is, "Load is a function")
			c, _ := f.bound().CallableOf(load)
			assert.Equal(t, c.Params[0].Role, rules.ParamContext, "the context is the context")
		})

		t.Run("classifies a parameter of any other type as input", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			load, _ := f.decl(t, id(fxPath, "Load", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(load)
			assert.Equal(t, c.Params[1].Role, rules.ParamInput, "the id is input")
		})

		t.Run("classifies a Context through an aliased import as the context", func(t *testing.T) {
			t.Parallel()

			aliased := param(&node.TypeRef{Spelling: "ctx.Context", Package: contextPath})
			assert.Equal(t, gorules.New().ParamRole(aliased, rules.View{}), rules.ParamContext,
				"the package the import names decides, not the qualifier")
		})

		t.Run("classifies a Context another package named context declares as input", func(t *testing.T) {
			t.Parallel()

			foreign := param(&node.TypeRef{Spelling: "context.Context", Package: foreignContext})
			assert.Equal(t, gorules.New().ParamRole(foreign, rules.View{}), rules.ParamInput,
				"the qualifier matches, and the import path does not")
		})

		t.Run("classifies a parameter without a type as input", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, gorules.New().ParamRole(&node.Param{}, rules.View{}), rules.ParamInput, "nothing to read")
		})
	})

	t.Run("ReturnRoles", func(t *testing.T) {
		t.Parallel()

		t.Run("classifies a last error return as the error", func(t *testing.T) {
			t.Parallel()

			roles, model := gorules.New().ReturnRoles(returnsOf(builtin("int"), builtin("error")), rules.View{})
			assert.Equal(t, roles[1], rules.ReturnError, "the last return")
			assert.Equal(t, model, rules.ErrorsLastReturn, "under the last-return model")
		})

		t.Run("classifies the second of two bool returns as the ok flag", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			find, _ := f.decl(t, id(fxPath, "Find", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(find)
			assert.Equal(t, c.Returns[1].Role, rules.ReturnOkBool, "the second of two returns, a bool, is ok")
		})

		t.Run("reports no error model for a callable without an error return", func(t *testing.T) {
			t.Parallel()

			_, model := gorules.New().ReturnRoles(returnsOf(builtin("int"), builtin("bool")), rules.View{})
			assert.Equal(t, model, rules.ErrorsNone, "no error return")
		})

		t.Run("classifies an iter.Seq return as a stream", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			all, _ := f.decl(t, id(fxPath, "All", symbol.KindFunction)).(*node.Function)
			c, _ := f.bound().CallableOf(all)
			assert.Equal(t, c.Returns[0].Role, rules.ReturnStream, "the fixture's All returns iter.Seq")
		})

		t.Run("classifies an iter.Seq2 return through an aliased import as a stream", func(t *testing.T) {
			t.Parallel()

			aliased := &node.TypeRef{Spelling: "it.Seq2", Package: golang.IterPackage}
			roles, _ := gorules.New().ReturnRoles(returnsOf(aliased), rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnStream},
				"the package the import names decides, not the qualifier")
		})

		t.Run("classifies a return without a type as a value", func(t *testing.T) {
			t.Parallel()

			roles, _ := gorules.New().ReturnRoles([]*node.Return{nil, {Type: nil}}, rules.View{})
			assert.Equal(t, roles, []rules.ReturnRole{rules.ReturnValue, rules.ReturnValue}, "nothing to read")
		})
	})

	t.Run("TypeName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			word string
			base string
			want string
		}{
			{name: "keeps an exported base exported", word: "check", base: "Row", want: "CheckRow"},
			{name: "keeps an unexported base unexported", word: "check", base: "row", want: "checkRow"},
			{name: "keeps an initialism's shape", word: "mock", base: "HTTPClient", want: "MockHTTPClient"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, gorules.New().TypeName(tt.word, tt.base), tt.want, "the joined name")
			})
		}
	})
}
