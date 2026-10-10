// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shapetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
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

// The builders give every declaration the identity, the name and the
// position that a load of the test language gives it.
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
			assert.Equal(
				t,
				names,
				[]string{firstParam, secondParam, firstValue},
				"the members are named by their positions",
			)
		})

		t.Run("gives each member an identity under the method", func(t *testing.T) {
			t.Parallel()

			m := shapetest.Method(storeName, putName, []*node.TypeRef{{Spelling: shapetest.String}},
				&node.TypeRef{Spelling: shapetest.Error})
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
			expect.Equal(t, m.ID, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Owner: storeName, Name: putName, Kind: symbol.KindMethod,
			}, "the method has an identity under its struct")
			expect.Equal(t, m.Host, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Name: storeName, Kind: symbol.KindStruct,
			}, "the method names its struct as its host")
		})
	})

	t.Run("Function", func(t *testing.T) {
		t.Parallel()

		t.Run("gives each member an identity under the function", func(t *testing.T) {
			t.Parallel()

			fn := shapetest.Function(findName, []*node.TypeRef{{Spelling: shapetest.String}})
			expect.Equal(t, fn.ID, symbol.Identity{
				Lang: shapetest.Lang, Package: shapetest.Path, Name: findName, Kind: symbol.KindFunction,
			}, "the function has an identity in the package")
			assert.Length(t, fn.Params, 1, "the function has one parameter")
			expect.Equal(t, fn.Params[0].ID.Owner, findName, "the parameter has an identity under the function")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("declares one struct for each host at its first method", func(t *testing.T) {
			t.Parallel()

			put, get := shapetest.Method(storeName, putName, nil), shapetest.Method(storeName, getName, nil)
			find := shapetest.Function(findName, nil)
			pkg := shapetest.Package(put, find, get, shapetest.Method(cacheName, getName, nil))
			assert.Length(t, pkg.Files, 1, "the package has one file")
			decls := pkg.Files[0].Decls
			assert.Length(t, decls, 3, "the file declares two structs and the function")
			store, is := decls[0].(*node.Struct)
			assert.True(t, is, "the file declares the struct of the first method first")
			expect.Equal(t, store.Name, storeName, "the first struct is the host of the first method")
			assert.Length(t, store.Methods, 2, "the struct has its two methods")
			expect.Equal(
				t,
				store.Methods[0],
				put,
				"the first method of the struct is its first method",
				assert.ByIdentity(),
			)
			expect.Equal(t, store.Methods[1], get, "the second method of the struct is its second method",
				assert.ByIdentity())
			expect.Equal(t, decls[1], symbol.Symbol(find), "the function follows the first struct",
				assert.ByIdentity())
		})

		t.Run("gives each callable its line in the file", func(t *testing.T) {
			t.Parallel()

			put, find := shapetest.Method(storeName, putName, nil), shapetest.Function(findName, nil)
			shapetest.Package(put, find)
			expect.Equal(t, put.Pos, position.Pos{File: shapetest.File, Line: 1}, "the method is on the first line")
			expect.Equal(t, find.Pos, position.Pos{File: shapetest.File, Line: 2}, "the function is on the second line")
		})

		t.Run("keeps every other declaration as it is", func(t *testing.T) {
			t.Parallel()

			state := &node.Variable{Name: stateName}
			pkg := shapetest.Package(state)
			assert.Length(t, pkg.Files[0].Decls, 1, "the file declares the variable")
			expect.Equal(t, pkg.Files[0].Decls[0], symbol.Symbol(state), "the file declares the variable as it is",
				assert.ByIdentity())
			expect.Equal(t, state.Pos, position.Pos{}, "the variable keeps its position")
		})

		t.Run("returns the package of the path", func(t *testing.T) {
			t.Parallel()

			pkg := shapetest.Package()
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
