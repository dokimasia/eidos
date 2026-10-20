// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// syntaxAllocs is the comment syntax: its list of line openers.
const syntaxAllocs = 1

// TestLang pins the identity, the extensions and the comment forms of the
// satellite, because a change breaks compositions outside this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("spells python", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(python.Lang), "python", "the language that every identity names")
		})
	})

	t.Run("CodePrefix", func(t *testing.T) {
		t.Parallel()

		t.Run("spells PYTHON", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(python.CodePrefix), "PYTHON", "the prefix of every diagnostic code")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells python", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(python.Name), "python", "the identity that findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .py", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.Extension, ".py", "the suffix of a module of source")
		})
	})

	t.Run("StubExtension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .pyi", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.StubExtension, ".pyi", "the suffix of a stub file")
		})
	})

	t.Run("InitModule", func(t *testing.T) {
		t.Parallel()

		t.Run("spells __init__", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.InitModule, "__init__", "the base name of the module of a package directory")
		})
	})

	t.Run("PackageSep", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a slash", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.PackageSep, "/", "the separator of the segments of a package")
		})
	})

	t.Run("ModuleSep", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a dot", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.ModuleSep, ".", "the separator of the segments of a module name")
		})
	})

	t.Run("Indent", func(t *testing.T) {
		t.Parallel()

		t.Run("spells four spaces", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.Indent, "    ", "one level of the indentation of the formatter of ruff")
		})
	})

	t.Run("FrontendVersion", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a version", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, python.FrontendVersion, "", "the version that every unit key folds")
		})
	})

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(python.Target), string(python.Lang),
				"the emitter copies a reference of a Python source as written into a plan of the target")
		})
	})

	t.Run("BackendVersion", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a version", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, python.BackendVersion, "", "the version that the composition fingerprint folds")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.Syntax().Line, []string{"#"}, "the line comment of Python")
		})

		t.Run("returns no block comment", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, python.Syntax().Blocks, "Python has no block comment")
		})

		t.Run("reports no directive convention", func(t *testing.T) {
			t.Parallel()

			assert.False(t, python.Syntax().Directives, "a comment of Python states a tool directive as prose")
		})
	})
}

// TestLangAllocs checks the ceiling of Syntax in the ordinary run, which
// does not run benchmarks.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = python.Syntax() }, syntaxAllocs, "Syntax allocates its list of line openers")
	assert.Length(t, got.Line, 1, "Syntax returns the line comment")
}

// BenchmarkLang measures the comment syntax that a composition reads once
// per frontend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = python.Syntax()
		}
		assert.Length(b, got.Line, 1, "Syntax returns the line comment")
	})
}
