// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"errors"
	"go/types"
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// keywordPath is an import path whose last element is a Go keyword.
const keywordPath = "example.com/shop/go"

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
			assert.InRange(t, count, 22, math.Inf(1), "the universe scope declares the 22 type names of Go 1.27")
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

	t.Run("NumericOf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports every integer and float type of the universe scope", func(t *testing.T) {
			t.Parallel()

			var count int
			for _, name := range types.Universe.Names() {
				tn, is := types.Universe.Lookup(name).(*types.TypeName)
				if !is {
					continue
				}
				basic, is := tn.Type().(*types.Basic)
				if !is || basic.Info()&(types.IsInteger|types.IsFloat) == 0 || basic.Info()&types.IsUntyped != 0 {
					continue
				}
				count++
				_, numeric := golang.NumericOf(name)
				expect.True(t, numeric, name+" is numeric")
			}
			assert.InRange(t, count, 15, math.Inf(1), "the universe scope declares 15 integer and float type names")
		})

		tests := []struct {
			name string
			give string
			want golang.Numeric
		}{
			{name: "returns the platform's width for int", give: "int", want: golang.Numeric{Class: rules.ScalarInt}},
			{
				name: "returns an unsigned integer of 8 bits for byte",
				give: "byte", want: golang.Numeric{Class: rules.ScalarUint, Width: 8},
			},
			{
				name: "returns a signed integer of 32 bits for rune",
				give: "rune", want: golang.Numeric{Class: rules.ScalarInt, Width: 32},
			},
			{
				name: "returns the platform's width for uintptr",
				give: "uintptr", want: golang.Numeric{Class: rules.ScalarUint},
			},
			{
				name: "returns a float of 64 bits for float64",
				give: "float64", want: golang.Numeric{Class: rules.ScalarFloat, Width: 64},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, numeric := golang.NumericOf(tt.give)
				assert.True(t, numeric, tt.give+" is numeric")
				assert.Equal(t, got, tt.want, "the class and the width of "+tt.give)
			})
		}

		t.Run("reports false for any other name", func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"complex64", "string", "bool", "any", "Row", ""} {
				_, numeric := golang.NumericOf(name)
				expect.False(t, numeric, name+" is not numeric")
			}
		})
	})

	t.Run("NumericName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of every canonical numeric type", func(t *testing.T) {
			t.Parallel()

			of := func(name string) (golang.Numeric, error) {
				n, numeric := golang.NumericOf(name)
				if !numeric {
					return n, errors.New(name + " is not numeric")
				}
				return n, nil
			}
			named := func(n golang.Numeric) (string, error) {
				name, declared := golang.NumericName(n)
				if !declared {
					return "", errors.New("the numeric type has no name")
				}
				return name, nil
			}
			names := []string{
				"int", "int8", "int16", "int32", "int64",
				"uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64",
			}
			for _, name := range names {
				expect.RoundTrip(t, of, named, name, "NumericName returns the name that NumericOf read")
			}
		})

		t.Run("returns uint for the platform's unsigned width", func(t *testing.T) {
			t.Parallel()

			got, declared := golang.NumericName(golang.Numeric{Class: rules.ScalarUint})
			assert.True(t, declared, "the platform's unsigned width has a name")
			assert.Equal(t, got, "uint", "uint precedes uintptr")
		})

		t.Run("reports false for a width that Go does not declare", func(t *testing.T) {
			t.Parallel()

			_, declared := golang.NumericName(golang.Numeric{Class: rules.ScalarFloat, Width: 16})
			assert.False(t, declared, "Go declares no float of 16 bits")
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

	t.Run("Qualifier", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "returns the assumed name of a path", give: "github.com/golang-jwt/jwt/v5", want: "jwt"},
			{name: "returns a keyword with a trailing underscore", give: keywordPath, want: "go_"},
			{
				name: "returns a keyword before a major version element with a trailing underscore",
				give: "example.com/type/v2", want: "type_",
			},
			{name: "returns a name that starts with a digit behind pkg", give: "example.com/2fa", want: "pkg2fa"},
			{name: "returns pkg for a path that assumes no name", give: "example.com/-", want: "pkg"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.Qualifier(tt.give), tt.want, "the qualifier of a generated import")
			})
		}
	})

	t.Run("ImportName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the assumed name of an unaliased import", func(t *testing.T) {
			t.Parallel()

			name, binds := golang.ImportName(&node.Import{Path: "gopkg.in/yaml.v3"})
			assert.True(t, binds, "an unaliased import binds a name")
			assert.Equal(t, name, "yaml", "the name the path assumes")
		})

		t.Run("returns the alias of an aliased import", func(t *testing.T) {
			t.Parallel()

			name, binds := golang.ImportName(&node.Import{Path: "context", Alias: "ctx"})
			assert.True(t, binds, "an aliased import binds a name")
			assert.Equal(t, name, "ctx", "the alias the import states")
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
func TestNamesAllocs(t *testing.T) {
	for _, c := range nameCalls() {
		msg := c.name + " allocates nothing"
		assert.MaxAllocs(t, c.call, 0, msg)
		c.check(t)
	}
	var escaped string
	assert.MaxAllocs(t, func() { escaped = golang.Qualifier(keywordPath) }, 1,
		"Qualifier allocates the escaped qualifier of a keyword")
	assert.Equal(t, escaped, "go_", "Qualifier escapes go")
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
		is      bool
		name    string
		numeric golang.Numeric
	)
	uint8Type := golang.Numeric{Class: rules.ScalarUint, Width: 8}
	return []nameCall{
		{
			name:  "NumericOf",
			call:  func() { numeric, is = golang.NumericOf("byte") },
			check: func(tb assert.TB) { assert.Equal(tb, numeric, uint8Type, "NumericOf returns uint8 for byte") },
		},
		{
			name:  "NumericName",
			call:  func() { name, is = golang.NumericName(uint8Type) },
			check: func(tb assert.TB) { assert.Equal(tb, name, "uint8", "NumericName returns uint8") },
		},
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
			name:  "Qualifier",
			call:  func() { name = golang.Qualifier("github.com/golang-jwt/jwt/v5") },
			check: func(tb assert.TB) { assert.Equal(tb, name, "jwt", "Qualifier returns jwt") },
		},
		{
			name:  "Unqualified",
			call:  func() { name = golang.Unqualified("time.Duration") },
			check: func(tb assert.TB) { assert.Equal(tb, name, "Duration", "Unqualified returns Duration") },
		},
		{
			name: "ImportName",
			call: func() { name, is = golang.ImportName(imp) },
			check: func(tb assert.TB) {
				assert.True(tb, is, "ImportName binds the import")
				assert.Equal(tb, name, "yaml", "ImportName returns yaml")
			},
		},
	}
}
