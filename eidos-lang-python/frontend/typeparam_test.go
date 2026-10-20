// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestTypeparam pins the type parameters of the syntax of PEP 695 and of
// the type variables of a module.
func TestTypeparam(t *testing.T) {
	t.Parallel()

	t.Run("typeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers each type parameter with its bound and its default", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f[T: int, U = str](a: T, b: U) -> None: ...\n"), "f")
			assert.Length(t, fn.TypeParams, 2, "the two parameters")
			expect.Equal(t, fn.TypeParams[0].Name, "T", "the first name")
			expect.Equal(t, spellings(fn.TypeParams[0].Bounds), []string{"int"}, "the bound")
			expect.Equal(t, fn.TypeParams[1].Default.Spelling, "str", "the default")
		})

		t.Run("lowers the constraints of a type parameter to one Union bound", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f[T: (int, str)](a: T) -> None: ...\n"), "f")
			bound := fn.TypeParams[0].Bounds[0]
			expect.Equal(t, bound.Form, symbol.FormUnion, "the constraints are one union")
			expect.Equal(t, bound.Spelling, "(int, str)", "spelled as the constraints of a call of TypeVar")
			expect.Equal(t, spellings(bound.Elems), []string{"int", "str"}, "each constraint")
		})

		t.Run("keeps the mark of a variadic type parameter in its name", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f[*Ts, **P](): ...\n"), "f")
			expect.Equal(t, []string{fn.TypeParams[0].Name, fn.TypeParams[1].Name}, []string{"*Ts", "**P"},
				"the marks of a tuple and of a parameter specification")
		})
	})

	t.Run("typeVarCall", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the keywords of a call of TypeVar", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "from typing import TypeVar\n\n"+
				"T = TypeVar(\"T\", bound=\"Node\", covariant=True, default=int)\n\ndef f(a: T) -> None: ...\n"), "f")
			assert.Length(t, fn.TypeParams, 1, "the type variable of the signature")
			tp := fn.TypeParams[0]
			expect.Equal(t, spellings(tp.Bounds), []string{"Node"}, "the bound of the string")
			expect.Equal(t, tp.Variance, symbol.VarianceOut, "covariant is out")
			expect.Equal(t, tp.Default.Spelling, "int", "the default")
		})

		t.Run("lowers contravariant to the variance in", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "from typing import TypeVar\n\n"+
				"T = TypeVar(\"T\", contravariant=True)\n\ndef f(a: T) -> None: ...\n"), "f")
			expect.Equal(t, fn.TypeParams[0].Variance, symbol.VarianceIn, "contravariant is in")
		})

		t.Run("lowers the constraints of a call of TypeVar to one Union bound", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "from typing import TypeVar\n\nS = TypeVar(\"S\", int, str)\n\n"+
				"def f(a: S) -> None: ...\n"), "f")
			bound := fn.TypeParams[0].Bounds[0]
			expect.Equal(t, bound.Form, symbol.FormUnion, "the constraints are one union")
			expect.Equal(t, bound.Spelling, "(int, str)", "spelled in parentheses")
		})

		t.Run("gives each declaration a type parameter of its own", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "from typing import TypeVar\n\nT = TypeVar(\"T\", bound=int)\n\n"+
				"def f(a: T) -> None: ...\ndef g(a: T) -> None: ...\n")
			f, g := named[*node.Function](t, decls, "f"), named[*node.Function](t, decls, "g")
			fb, gb := f.TypeParams[0].Bounds[0], g.TypeParams[0].Bounds[0]
			expect.NotEqual(t, fb, gb, "a node of the graph has one place", assert.ByIdentity())
		})
	})

	t.Run("usedVars", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the type variable of an unpacking", func(t *testing.T) {
			t.Parallel()

			src := "from typing import TypeVarTuple\n\nTs = TypeVarTuple(\"Ts\")\n\ndef f(*args: *Ts) -> None: ...\n"
			fn := named[*node.Function](t, declsOf(t, src), "f")
			assert.Length(t, fn.TypeParams, 1, "the variable of the unpacking")
			expect.Equal(t, fn.TypeParams[0].Name, "*Ts", "a type variable tuple keeps its mark")
		})
	})
}
