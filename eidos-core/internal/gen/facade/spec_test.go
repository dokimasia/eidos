// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// The paths the spec cases inject into and read back: the emit
// package's generated spec, and the mini kernel's one position file.
const (
	emitSpecPath = "eidos-sdk/emit/facade.gen_test.go"
	positionRel  = "position/position.go"
)

// pinnable is a surface with one declaration of every form the spec
// pins, and one of every form it leaves out.
const pinnable = `package emit

// Shade is a pinned type.
type Shade int

// Bright is a pinned constant.
const Bright Shade = 1

// Tint returns a pinned function's result.
func Tint() Shade { return Bright }

// Pick is a generic function.
func Pick[T any](v T) T { return v }

// Box is a generic type.
type Box[T any] struct{ V T }

// Number is a constraint interface.
type Number interface{ ~int | ~int64 }

// Same is a constraint interface through comparable.
type Same interface{ comparable }

// Lighter is a method.
func (s Shade) Lighter() Shade { return s + 1 }
`

// The generated spec pins every re-export that has a type to compare,
// and leaves out what has none.
func TestSpec(t *testing.T) {
	t.Parallel()

	t.Run("renderSpec", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			want string
		}{
			{name: "declares the facade's black-box test package", want: "package emit_test"},
			{
				name: "pins a type to the kernel's type",
				want: "reflect.TypeFor[emit.Shade](), reflect.TypeFor[core.Shade]()",
			},
			{name: "pins a constant to the kernel's value", want: "assert.Equal(t, emit.Bright, core.Bright,"},
			{
				name: "pins a function to the kernel function's signature",
				want: "reflect.TypeOf(emit.Tint), reflect.TypeOf(core.Tint)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, specOf(t, pinnable), tt.want, "the spec states the pin")
			})
		}

		omitted := []struct {
			name   string
			symbol string
		}{
			{name: "leaves out a generic function", symbol: "emit.Pick"},
			{name: "leaves out a generic type", symbol: "emit.Box"},
			{name: "leaves out a constraint interface of a type set", symbol: "emit.Number"},
			{name: "leaves out a constraint interface through comparable", symbol: "emit.Same"},
			{name: "leaves out a method", symbol: "Lighter"},
		}
		for _, tt := range omitted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NotContains(t, specOf(t, pinnable), tt.symbol, "the spec has no case for it")
			})
		}

		t.Run("returns an error for a package with nothing to pin", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, positionRel,
				"package position\n\n// Pick is the only re-export.\nfunc Pick[T any](v T) T { return v }\n")
			_, err := facade.Generate(root)
			assert.HasError(t, err, "a spec without a case pins nothing")
			assert.Contains(t, err.Error(), "nothing the spec can pin", "the error names the fault")
		})
	})
}

// specOf generates a mini kernel with one file injected into the emit
// package, and returns the emit package's generated spec.
func specOf(t *testing.T, content string) string {
	t.Helper()

	root := mini(t)
	poison(t, root, poisonRel, content)
	set, err := facade.Generate(root)
	assert.NoError(t, err, "the poisoned mini kernel generates")
	return string(set[emitSpecPath])
}
