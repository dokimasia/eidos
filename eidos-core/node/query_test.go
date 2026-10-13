// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// queriedLang is the language of the queried package.
const queriedLang symbol.Lang = "golang"

// The path of the queried package, and the names of its declarations.
const (
	queriedPkg = "svc/store"
	storeName  = "Store"
	readerName = "Reader"
	rowName    = "Row"
	getName    = "Get"
	putName    = "Put"
)

// queried is the package that the query cases read: two files, with two
// interfaces and a struct of two methods among them.
var queried = &node.Package{Files: []*node.File{
	{Path: "a.go", Decls: node.Symbols{
		&node.Interface{ID: symbol.Identity{
			Lang: queriedLang, Package: queriedPkg, Name: storeName, Kind: symbol.KindInterface,
		}},
		&node.Struct{
			ID: symbol.Identity{Lang: queriedLang, Package: queriedPkg, Name: rowName, Kind: symbol.KindStruct},
			Methods: []*node.Method{
				nil,
				{ID: symbol.Identity{
					Lang: queriedLang, Package: queriedPkg, Owner: rowName, Name: getName, Kind: symbol.KindMethod,
				}},
				{ID: symbol.Identity{
					Lang: queriedLang, Package: queriedPkg, Owner: rowName, Name: putName, Kind: symbol.KindMethod,
				}},
			},
		},
	}},
	nil,
	{Path: "b.go", Decls: node.Symbols{&node.Interface{ID: symbol.Identity{
		Lang: queriedLang, Package: queriedPkg, Name: readerName, Kind: symbol.KindInterface,
	}}}},
}}

// The queries read the declarations that a loop over the files returns,
// in the same order, so every case pins the order.
func TestQuery(t *testing.T) {
	t.Parallel()

	t.Run("DeclsOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of the type in the order of the files", func(t *testing.T) {
			t.Parallel()

			var names []string
			for i := range node.DeclsOf[*node.Interface](queried) {
				names = append(names, i.ID.Name)
			}
			assert.Equal(t, names, []string{storeName, readerName}, "the interfaces of both files, a.go first")
		})

		t.Run("stops where the caller stops", func(t *testing.T) {
			t.Parallel()

			seen := 0
			for range node.DeclsOf[*node.Interface](queried) {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the sequence honours an early return")
		})

		t.Run("yields nothing for a nil package", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(node.DeclsOf[*node.Struct](nil)), "a nil package declares nothing")
		})
	})

	t.Run("DeclNamed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declaration of the type and the name", func(t *testing.T) {
			t.Parallel()

			reader, held := node.DeclNamed[*node.Interface](queried, readerName)
			assert.True(t, held, "the package declares the interface Reader")
			assert.Equal(t, reader.ID.Name, readerName, "the declaration has the name")
		})

		t.Run("reports false for a name of another kind", func(t *testing.T) {
			t.Parallel()

			_, held := node.DeclNamed[*node.Interface](queried, rowName)
			assert.False(t, held, "Row is a struct and no interface")
		})
	})

	t.Run("MemberNamed", func(t *testing.T) {
		t.Parallel()

		row, _ := node.DeclNamed[*node.Struct](queried, rowName)

		t.Run("returns the member of the name past a nil member", func(t *testing.T) {
			t.Parallel()

			put, held := node.MemberNamed(row.Methods, putName)
			assert.True(t, held, "the struct declares the method Put")
			assert.Equal(t, put.ID.Name, putName, "the member has the name")
		})

		t.Run("reports false for a name that the list does not have", func(t *testing.T) {
			t.Parallel()

			_, held := node.MemberNamed(row.Methods, "Close")
			assert.False(t, held, "the struct declares no method Close")
		})
	})
}

// The queries allocate nothing but the iterator of DeclsOf, which the
// compiler keeps on the stack. The ordinary run, which runs no
// benchmark, checks that here.
func TestQueryAllocs(t *testing.T) {
	var n int
	assert.MaxAllocs(t, func() {
		n = 0
		for range node.DeclsOf[*node.Interface](queried) {
			n++
		}
	}, 0, "a range over DeclsOf allocates nothing")
	assert.Equal(t, n, 2, "DeclsOf yields both interfaces")
	var held bool
	assert.MaxAllocs(t, func() { _, held = node.DeclNamed[*node.Interface](queried, readerName) }, 0,
		"DeclNamed allocates nothing")
	assert.True(t, held, "DeclNamed finds the interface")
	row, _ := node.DeclNamed[*node.Struct](queried, rowName)
	assert.MaxAllocs(t, func() { _, held = node.MemberNamed(row.Methods, putName) }, 0,
		"MemberNamed allocates nothing")
	assert.True(t, held, "MemberNamed finds the method")
}

// BenchmarkQuery measures the queries that a plugin makes once per
// subject.
func BenchmarkQuery(b *testing.B) {
	b.Run("DeclsOf", func(b *testing.B) {
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = 0
			for range node.DeclsOf[*node.Interface](queried) {
				got++
			}
		}
		assert.Equal(b, got, 2, "the range yields both interfaces")
	})

	b.Run("DeclNamed", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var held bool
		for c.Loop() {
			_, held = node.DeclNamed[*node.Interface](queried, readerName)
		}
		assert.True(b, held, "DeclNamed finds the interface")
	})

	b.Run("MemberNamed", func(b *testing.B) {
		row, _ := node.DeclNamed[*node.Struct](queried, rowName)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var held bool
		for c.Loop() {
			_, held = node.MemberNamed(row.Methods, putName)
		}
		assert.True(b, held, "MemberNamed finds the method")
	})
}
