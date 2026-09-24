// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"go/types"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
)

// The frontend's resolution and the rules' resolution both read
// these names, so each rule is pinned against its authority: the
// universe scope for the predeclared types, and goimports' assumed
// name for an import path.
func TestNames(t *testing.T) {
	t.Parallel()

	t.Run("Predeclared", func(t *testing.T) {
		t.Parallel()

		t.Run("reports every type name of the universe scope", func(t *testing.T) {
			t.Parallel()

			var count int
			for _, name := range types.Universe.Names() {
				if _, is := types.Universe.Lookup(name).(*types.TypeName); is {
					count++
					assert.True(t, golang.Predeclared(name), name+" is predeclared")
				}
			}
			assert.True(t, count >= 22, "the universe scope declares the 22 type names of Go 1.27")
		})

		t.Run("refuses the rest", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"nil", "true", "len", "iota", "unsafe.Pointer", "Row", ""} {
				assert.False(t, golang.Predeclared(name), name+" is no predeclared type")
			}
		})
	})

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the booleans, numbers and strings alone", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"bool", "int", "uint8", "byte", "rune", "uintptr", "float32", "complex128", "string"} {
				assert.True(t, golang.Basic(name), name+" is basic")
			}
			for _, name := range []string{"any", "error", "comparable", "unsafe.Pointer", "Row", ""} {
				assert.False(t, golang.Basic(name), name+" is not basic")
			}
		})
	})

	t.Run("Ordered", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the integers, floats and strings alone", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"int", "int64", "uint", "byte", "rune", "uintptr", "float64", "string"} {
				assert.True(t, golang.Ordered(name), name+" is ordered")
			}
			for _, name := range []string{"bool", "complex64", "any", "error", "Row"} {
				assert.False(t, golang.Ordered(name), name+" is not ordered")
			}
		})
	})

	t.Run("AssumedName", func(t *testing.T) {
		t.Parallel()

		t.Run("binds the name goimports assumes", func(t *testing.T) {
			t.Parallel()

			tests := []struct{ path, want string }{
				{"context", "context"},
				{"net/http", "http"},
				{"gopkg.in/yaml.v3", "yaml"},
				{"github.com/golang-jwt/jwt/v5", "jwt"},
				{"github.com/mattn/go-sqlite3", "sqlite3"},
				{"example.com/go-kit/log", "log"},
				{"v2", "v2"},
				{"example.com/vendor", "vendor"},
				{"example.com/ünïcode", "ünïcode"},
			}
			for _, tt := range tests {
				assert.Equal(t, golang.AssumedName(tt.path), tt.want, tt.path)
			}
		})
	})

	t.Run("ImportName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the alias, else the assumed name", func(t *testing.T) {
			t.Parallel()

			name, binds := golang.ImportName(&node.Import{Path: "gopkg.in/yaml.v3"})
			assert.True(t, binds && name == "yaml", "an unaliased import binds its assumed name")
			name, binds = golang.ImportName(&node.Import{Path: "context", Alias: "ctx"})
			assert.True(t, binds && name == "ctx", "an aliased import binds its alias")
		})

		t.Run("binds nothing for a blank, a dot or no import", func(t *testing.T) {
			t.Parallel()

			for _, imp := range []*node.Import{
				{Path: "embed", Alias: golang.BlankAlias},
				{Path: "example.com/dsl", Wildcard: true},
				{Path: "example.com/dsl", Alias: golang.DotAlias},
				nil,
			} {
				_, binds := golang.ImportName(imp)
				assert.False(t, binds, "the import binds no qualifier")
			}
		})
	})
}
