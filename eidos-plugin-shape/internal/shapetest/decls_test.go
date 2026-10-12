// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the declarations of the fixtures, the names that the
// builders give, and the segments of the package path.
const (
	storeName   = "Store"
	cacheName   = "Cache"
	putName     = "Put"
	getName     = "Get"
	findName    = "Find"
	stateName   = "State"
	firstParam  = "p0"
	secondParam = "p1"
	firstValue  = "r0"
	owner       = storeName + "." + putName
	pathRoot    = "acme"
	packageName = "store"
)

func TestDecls(t *testing.T) {
	t.Parallel()

	t.Run("Method", func(t *testing.T) {
		t.Parallel()

		t.Run("names the parameters and the returns by their positions", func(t *testing.T) {
			t.Parallel()

			m := shapetest.Method(storeName, putName,
				[]*node.TypeRef{{Spelling: shapetest.String}, {Spelling: shapetest.Int}},
				&node.TypeRef{Spelling: shapetest.Error})
			names := make([]string, 0, len(m.Params)+len(m.Returns))
			for _, p := range m.Params {
				names = append(names, p.Name)
			}
			for _, r := range m.Returns {
				names = append(names, r.Name)
			}
			want := []string{firstParam, secondParam, firstValue}
			assert.Equal(t, names, want, "the members are named by their positions")
		})

		t.Run("receives its host until Package places it", func(t *testing.T) {
			t.Parallel()

			m := shapetest.Method(storeName, putName, nil)
			assert.NotNil(t, m.Receives, "the method receives its host")
			expect.Equal(t, m.Receives.Spelling, storeName, "the method receives the struct of its host")
			expect.Equal(t, m.ID, symbol.Identity{}, "the method has no identity before Package")
		})
	})

	t.Run("Function", func(t *testing.T) {
		t.Parallel()

		t.Run("names the parameters by their positions", func(t *testing.T) {
			t.Parallel()

			fn := shapetest.Function(findName, []*node.TypeRef{{Spelling: shapetest.String}})
			assert.Length(t, fn.Params, 1, "the function has one parameter")
			expect.Equal(t, fn.Params[0].Name, firstParam, "the parameter is named by its position")
			expect.Equal(t, fn.ID, symbol.Identity{}, "the function has no identity before Package")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("declares one struct for each host at its first method", func(t *testing.T) {
			t.Parallel()

			put, get := shapetest.Method(storeName, putName, nil), shapetest.Method(storeName, getName, nil)
			find := shapetest.Function(findName, nil)
			pkg := shapetest.Package(t, put, find, get, shapetest.Method(cacheName, getName, nil))
			assert.Length(t, pkg.Files, 1, "the package has one file")
			decls := pkg.Files[0].Decls
			assert.Length(t, decls, 3, "the file declares two structs and the function")
			store, is := decls[0].(*node.Struct)
			assert.True(t, is, "the file declares the struct of the first method first")
			expect.Equal(t, store.Name, storeName, "the first struct is the host of the first method")
			assert.Length(t, store.Methods, 2, "the struct has its two methods")
			expect.Equal(t, store.Methods[0].Name, putName, "the first method of the struct is its first method")
			expect.Equal(t, store.Methods[1].Name, getName, "the second method of the struct is its second method")
			fn, is := decls[1].(*node.Function)
			assert.True(t, is, "a function follows the first struct")
			expect.Equal(t, fn.ID, find.ID, "the function after the first struct is the function of the fixture")
		})

		t.Run("gives each member an identity under the method", func(t *testing.T) {
			t.Parallel()

			m := shapetest.Method(storeName, putName, []*node.TypeRef{{Spelling: shapetest.String}},
				&node.TypeRef{Spelling: shapetest.Error})
			shapetest.Package(t, m)
			assert.Length(t, m.Params, 1, "the method has one parameter")
			assert.Length(t, m.Returns, 1, "the method has one return")
			expect.Equal(t, m.Params[0].ID, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Owner: owner, Name: firstParam, Kind: symbol.KindParam,
			}, "the parameter has an identity under the method")
			expect.Equal(t, m.Returns[0].ID, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Owner: owner, Name: firstValue, Kind: symbol.KindReturn,
			}, "the return has an identity under the method")
		})

		t.Run("hosts the method on the struct", func(t *testing.T) {
			t.Parallel()

			m := shapetest.Method(storeName, putName, nil)
			shapetest.Package(t, m)
			expect.Equal(t, m.ID, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Owner: storeName, Name: putName, Kind: symbol.KindMethod,
			}, "the method has an identity under its struct")
			expect.Equal(t, m.Host, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Name: storeName, Kind: symbol.KindStruct,
			}, "the method has its struct as its host")
			expect.Nil(t, m.Receives, "the placed method receives nothing")
		})

		t.Run("gives each declaration a position in the file", func(t *testing.T) {
			t.Parallel()

			put, find := shapetest.Method(storeName, putName, nil), shapetest.Function(findName, nil)
			shapetest.Package(t, put, find)
			expect.Equal(t, put.Pos.File, shapetest.File, "the method is in the file")
			expect.Equal(t, find.Pos.File, shapetest.File, "the function is in the file")
			expect.Pairwise(t, []int{put.Pos.Line, find.Pos.Line}, func(earlier, later int) bool {
				return earlier < later
			}, "the method is on a line before the function")
		})

		t.Run("gives each parameter a reference of its own", func(t *testing.T) {
			t.Parallel()

			str := &node.TypeRef{Spelling: shapetest.String}
			m := shapetest.Method(storeName, putName, []*node.TypeRef{str, str})
			shapetest.Package(t, m)
			assert.Length(t, m.Params, 2, "the method has two parameters")
			expect.NotEqual(t, m.Params[0].Type, m.Params[1].Type, "the parameters have two references",
				assert.ByIdentity())
			expect.NotEqual(t, m.Params[0].Type, str, "the first parameter has a copy of the shared reference",
				assert.ByIdentity())
			expect.Equal(t, m.Params[1].Type.Spelling, shapetest.String, "the copy has the spelling of the reference")
		})

		t.Run("keeps every other declaration in the file", func(t *testing.T) {
			t.Parallel()

			state := &node.Variable{Name: stateName}
			pkg := shapetest.Package(t, state)
			assert.Length(t, pkg.Files[0].Decls, 1, "the file declares one declaration")
			variable, is := pkg.Files[0].Decls[0].(*node.Variable)
			assert.True(t, is, "the file declares a variable")
			expect.Equal(t, variable.ID, state.ID, "the file declares the variable of the fixture")
		})

		t.Run("returns the package of the path", func(t *testing.T) {
			t.Parallel()

			pkg := shapetest.Package(t)
			expect.Equal(
				t,
				pkg.ID,
				symbol.Identity{Lang: shapetest.Lang, Package: shapetest.Path, Kind: symbol.KindPackage},
				"the package has the identity of its path",
			)
			expect.Equal(t, pkg.Path, []string{pathRoot, packageName}, "the package has the segments of its path")
			expect.Equal(t, pkg.Name, packageName, "the package has the last segment of its path as its name")
		})
	})
}
