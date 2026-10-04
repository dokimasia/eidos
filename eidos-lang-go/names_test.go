// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"go/types"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
)

// nameCall is one name rule's call and the check of its result, which
// the allocation test and the benchmark share.
type nameCall struct {
	name  string
	call  func()
	check func(tb assert.TB)
}

// The frontend's resolution and the rules' resolution both read these
// names. Each rule is pinned against its authority: the universe scope
// for the predeclared types, and goimports' assumed name for an import
// path.
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

		t.Run("reports false for any other name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"nil", "true", "len", "iota", "unsafe.Pointer", "Row", ""} {
				assert.False(t, golang.Predeclared(name), name+" is no predeclared type")
			}
		})
	})

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a basic type name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"bool", "int", "uint8", "byte", "rune", "uintptr", "float32", "complex128", "string"} {
				assert.True(t, golang.Basic(name), name+" is basic")
			}
		})

		t.Run("reports false for any other name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"any", "error", "comparable", "unsafe.Pointer", "Row", ""} {
				assert.False(t, golang.Basic(name), name+" is not basic")
			}
		})
	})

	t.Run("Ordered", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for an ordered type name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"int", "int64", "uint", "byte", "rune", "uintptr", "float64", "string"} {
				assert.True(t, golang.Ordered(name), name+" is ordered")
			}
		})

		t.Run("reports false for any other name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"bool", "complex64", "any", "error", "Row"} {
				assert.False(t, golang.Ordered(name), name+" is not ordered")
			}
		})
	})

	t.Run("Unqualified", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name behind the qualifier", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Unqualified("time.Duration"), "Duration", "the qualifier is dropped")
		})

		t.Run("returns an unqualified spelling unchanged", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Unqualified("Row"), "Row", "a bare name is its own name")
		})
	})

	t.Run("AssumedName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "returns a one-element path unchanged", give: "context", want: "context"},
			{name: "returns a path's last element", give: "net/http", want: "http"},
			{name: "returns the last element before a dotted version suffix", give: "gopkg.in/yaml.v3", want: "yaml"},
			{
				name: "returns the element before a major version element",
				give: "github.com/golang-jwt/jwt/v5", want: "jwt",
			},
			{
				name: "returns the last element without its go- prefix",
				give: "github.com/mattn/go-sqlite3", want: "sqlite3",
			},
			{
				name: "returns the last element of a path with a go- prefix before it",
				give: "example.com/go-kit/log", want: "log",
			},
			{name: "returns a lone major version element unchanged", give: "v2", want: "v2"},
			{name: "returns a last element named vendor unchanged", give: "example.com/vendor", want: "vendor"},
			{
				name: "returns a last element of letters outside ASCII unchanged",
				give: "example.com/ünïcode",
				want: "ünïcode",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.AssumedName(tt.give), tt.want, "the name goimports assumes")
			})
		}
	})

	t.Run("ImportName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the assumed name of an unaliased import", func(t *testing.T) {
			t.Parallel()

			name, binds := golang.ImportName(&node.Import{Path: "gopkg.in/yaml.v3"})
			assert.True(t, binds && name == "yaml", "the name the path assumes")
		})

		t.Run("returns the alias of an aliased import", func(t *testing.T) {
			t.Parallel()

			name, binds := golang.ImportName(&node.Import{Path: "context", Alias: "ctx"})
			assert.True(t, binds && name == "ctx", "the alias the import states")
		})

		tests := []struct {
			name string
			give *node.Import
		}{
			{name: "reports false for a blank import", give: &node.Import{Path: "embed", Alias: golang.BlankAlias}},
			{name: "reports false for a wildcard import", give: &node.Import{Path: "example.com/dsl", Wildcard: true}},
			{
				name: "reports false for a dot import",
				give: &node.Import{Path: "example.com/dsl", Alias: golang.DotAlias},
			},
			{name: "reports false for a missing import", give: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, binds := golang.ImportName(tt.give)
				assert.False(t, binds, "the import binds no qualifier")
			})
		}
	})
}

// Every name rule reads the universe scope or slices its input, and
// allocates nothing. The ordinary run, which runs no benchmark, checks
// that here.
func TestNamesZeroAlloc(t *testing.T) {
	for _, c := range nameCalls() {
		msg := c.name + " allocates nothing"
		assert.MaxAllocs(t, c.call, 0, msg)
		c.check(t)
	}
}

// BenchmarkNames measures the name rules the frontend and the rules
// read for every reference they resolve.
func BenchmarkNames(b *testing.B) {
	for _, tt := range nameCalls() {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				tt.call()
			}
			tt.check(b)
		})
	}
}

// nameCalls returns a call of every name rule.
func nameCalls() []nameCall {
	imp := &node.Import{Path: "gopkg.in/yaml.v3"}
	var (
		is   bool
		name string
	)
	return []nameCall{
		{
			name:  "Predeclared",
			call:  func() { is = golang.Predeclared("int") },
			check: func(tb assert.TB) { assert.True(tb, is, "Predeclared reports int") },
		},
		{
			name:  "Basic",
			call:  func() { is = golang.Basic("int") },
			check: func(tb assert.TB) { assert.True(tb, is, "Basic reports int") },
		},
		{
			name:  "Ordered",
			call:  func() { is = golang.Ordered("int") },
			check: func(tb assert.TB) { assert.True(tb, is, "Ordered reports int") },
		},
		{
			name:  "AssumedName",
			call:  func() { name = golang.AssumedName("github.com/golang-jwt/jwt/v5") },
			check: func(tb assert.TB) { assert.Equal(tb, name, "jwt", "AssumedName returns jwt") },
		},
		{
			name:  "Unqualified",
			call:  func() { name = golang.Unqualified("time.Duration") },
			check: func(tb assert.TB) { assert.Equal(tb, name, "Duration", "Unqualified returns Duration") },
		},
		{
			name:  "ImportName",
			call:  func() { name, is = golang.ImportName(imp) },
			check: func(tb assert.TB) { assert.True(tb, is && name == "yaml", "ImportName returns yaml") },
		},
	}
}
