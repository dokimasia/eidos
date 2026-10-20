// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
)

// The names of builtins that the specs classify: a class, an exception
// class, a name of OSError, a function, a constant, and a class that only
// typeshed declares.
const (
	intName            = "int"
	valueErrorName     = "ValueError"
	ioErrorName        = "IOError"
	lenName            = "len"
	notImplementedName = "NotImplemented"
	functionName       = "function"
)

// TestBuiltins pins the classes of the module builtins.
func TestBuiltins(t *testing.T) {
	t.Parallel()

	t.Run("IsBuiltinClass", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a class", give: intName, want: true},
			{name: "reports true for an exception class", give: valueErrorName, want: true},
			{name: "reports true for a name of OSError", give: ioErrorName, want: true},
			{name: "reports false for a function", give: lenName},
			{name: "reports false for a constant", give: notImplementedName},
			{name: "reports false for a class that only typeshed declares", give: functionName},
			{name: "reports false for the empty name", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, python.IsBuiltinClass(tt.give), tt.want, "the class of "+tt.give)
			})
		}
	})

	t.Run("IsBuiltinException", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for an exception class", give: valueErrorName, want: true},
			{name: "reports true for a name of OSError", give: ioErrorName, want: true},
			{name: "reports false for a class that is no exception", give: intName},
			{name: "reports false for a function", give: lenName},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, python.IsBuiltinException(tt.give), tt.want, "the exception class of "+tt.give)
			})
		}
	})
}

// TestBuiltinsAllocs checks that the lookups allocate nothing in the
// ordinary run, which does not run benchmarks.
func TestBuiltinsAllocs(t *testing.T) {
	var class, exception bool
	assert.MaxAllocs(t, func() { class = python.IsBuiltinClass(intName) }, 0, "IsBuiltinClass allocates nothing")
	assert.True(t, class, "int is a class of builtins")
	assert.MaxAllocs(t, func() { exception = python.IsBuiltinException(valueErrorName) }, 0,
		"IsBuiltinException allocates nothing")
	assert.True(t, exception, "ValueError is an exception class of builtins")
}

// BenchmarkBuiltins measures the lookups that the frontend makes for each
// implicit alias and the rules make for each base of a class.
func BenchmarkBuiltins(b *testing.B) {
	b.Run("IsBuiltinClass", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = python.IsBuiltinClass(intName)
		}
		assert.True(b, got, "int is a class of builtins")
	})

	b.Run("IsBuiltinException", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = python.IsBuiltinException(valueErrorName)
		}
		assert.True(b, got, "ValueError is an exception class of builtins")
	})
}
