// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// cfgOptions is the cfg set the evaluation cases load under: the feature
// x, the option unix, and the key target_os set to linux.
var cfgOptions = &frontend.Options{Features: []string{"x"}, Cfg: []string{"unix", "target_os=\"linux\""}}

// Attributes are an item's annotations, documentation and cfg gate, so
// what each attribute lowers to, and how a cfg predicate evaluates, is
// pinned.
func TestAttr(t *testing.T) {
	t.Parallel()

	t.Run("attributes", func(t *testing.T) {
		t.Parallel()

		t.Run("annotates an item with an attribute's name and its arguments verbatim", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[derive(Debug, Clone)]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "derive", Args: []string{"Debug", "Clone"}}}, "one argument per argument")
		})

		t.Run("annotates an item with an argument's tokens as written", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[serde(rename = \"a  b\")]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "serde", Args: []string{"rename = \"a  b\""}}}, "the argument's source")
		})

		t.Run("annotates an item with the value an attribute assigns verbatim", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[must_use = \"use it\"]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "must_use", Args: []string{"\"use it\""}}}, "the literal as written")
		})

		t.Run("annotates an item with an attribute's path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[serde::skip]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "serde::skip"}}, "the attribute's whole path")
		})

		t.Run("annotates an item with no argument for an empty argument list", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[derive()]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "derive"}}, "the list states none")
		})

		t.Run("annotates an item with no empty argument for a trailing comma", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[derive(Debug,)]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}}, "the comma ends the list")
		})

		t.Run("documents an item with a #[doc] string and annotates nothing", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "#[doc = \"A.\"]\npub struct A;\n", "A")
			assert.Equal(t, st.Doc, []string{"A."}, "the string is documentation")
			assert.Empty(t, st.Annotations, "and no annotation")
		})

		t.Run("documents an item with one line per line of a #[doc] string", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[doc = \"a\\nb\"]\npub struct A;\n", "A").Doc, []string{"a", "b"},
				"the escape is a line break")
		})

		t.Run("annotates an item with a #[doc] attribute that assigns no string", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "#[doc(hidden)]\npub struct A;\n", "A")
			assert.Empty(t, st.Doc, "hidden is no documentation")
			assert.Equal(t, st.Annotations, symbol.Annotations{{Name: "doc", Args: []string{"hidden"}}},
				"it is an annotation")
		})

		t.Run("stamps an item #[test] marks rust.test", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[test]\nfn probe() {}\n")
			fn := named[*node.Function](t, fileIn(t, gb, crateName).Decls, "probe")
			assert.True(t, stamped(gb, fn, string(rust.TestKey)), "the test function")
		})

		t.Run("stamps an item #[cfg(test)] marks rust.test and loads it", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(test)]\nfn helper() {}\n")
			fn := named[*node.Function](t, fileIn(t, gb, crateName).Decls, "helper")
			assert.True(t, stamped(gb, fn, string(rust.TestKey)), "the test-only function")
		})

		t.Run("stamps the file with the first cfg predicate that keeps an item out", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(windows)]\n#[cfg(feature = \"y\")]\npub struct A;\n")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"windows"}}, "one predicate per item")
		})

		t.Run("stamps the file with a cfg predicate as written", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(any(windows,  feature = \"y\"))]\npub struct A;\n")
			assert.Equal(t, stampsOf(gb, string(rust.CfgKey)), []any{[]string{"any(windows,  feature = \"y\")"}},
				"the predicate's source between the parentheses")
		})

		t.Run("loads an item whose cfg attribute states no predicate", func(t *testing.T) {
			t.Parallel()

			named[*node.Struct](t, declsOf(t, "#[cfg]\npub struct A;\n"), "A")
		})
	})

	t.Run("literal", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a raw string's content as written", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[doc = r\"a\\n\"]\npub struct A;\n", "A").Doc, []string{"a\\n"},
				"a raw string has no escapes")
		})

		t.Run("returns the content as written of a string whose escape Go does not read", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, "#[doc = \"a\\u{41}\"]\npub struct A;\n", "A").Doc, []string{"a\\u{41}"},
				"the escape as the source states it")
		})

		t.Run("reports false for a value that is not a string", func(t *testing.T) {
			t.Parallel()

			st := structOf(t, "#[doc = include_str!(\"a.md\")]\npub struct A;\n", "A")
			assert.Empty(t, st.Doc, "a macro's expansion is outside the syntax")
			assert.Equal(t, st.Annotations, symbol.Annotations{{Name: "doc", Args: []string{"include_str!(\"a.md\")"}}},
				"and the attribute is an annotation")
		})
	})

	t.Run("satisfied", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			pred string
			want bool
		}{
			{name: "reports true for a feature the load enables", pred: "feature = \"x\"", want: true},
			{name: "reports false for a feature the load does not enable", pred: "feature = \"y\"", want: false},
			{name: "reports false for a feature spelled without quotes", pred: "feature = x", want: false},
			{name: "reports true for an option the load sets", pred: "unix", want: true},
			{name: "reports false for an option the load does not set", pred: "windows", want: false},
			{name: "reports true for a key the load sets to the value", pred: "target_os = \"linux\"", want: true},
			{
				name: "reports false for a key the load sets to another value",
				pred: "target_os = \"macos\"",
				want: false,
			},
			{name: "reports true for test", pred: "test", want: true},
			{name: "reports true for all of satisfied predicates", pred: "all(unix, feature = \"x\")", want: true},
			{name: "reports false for all with one unsatisfied predicate", pred: "all(unix, windows)", want: false},
			{name: "reports true for all of no predicate", pred: "all()", want: true},
			{name: "reports true for any with one satisfied predicate", pred: "any(windows, unix)", want: true},
			{
				name: "reports false for any of unsatisfied predicates",
				pred: "any(windows, feature = \"y\")",
				want: false,
			},
			{name: "reports false for any of no predicate", pred: "any()", want: false},
			{name: "reports true for not of an unsatisfied predicate", pred: "not(windows)", want: true},
			{name: "reports false for not of a satisfied predicate", pred: "not(unix)", want: false},
			{name: "reports false for not of two predicates", pred: "not(windows, macos)", want: false},
			{name: "reports false for a combinator Rust does not define", pred: "some(unix)", want: false},
			{name: "reports false for a predicate of another shape", pred: "unix linux", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, loadedUnder(t, tt.pred), tt.want, "the item loads where the predicate is satisfied")
			})
		}

		t.Run("reports false for two predicates in one cfg attribute", func(t *testing.T) {
			t.Parallel()

			assert.False(t, loadedUnder(t, "unix, feature = \"x\""), "a cfg attribute takes one predicate")
		})
	})
}

// loadedUnder reports whether a struct A that a cfg predicate gates
// loads under the cfg set the evaluation cases state.
func loadedUnder(tb assert.TB, pred string) bool {
	tb.Helper()

	src := "#[cfg(" + pred + ")]\npub struct A;\n"
	gb, _ := parsedTree(tb, crateTree(map[string]string{libRoot: src}), libRoot, plugin.DepthFull, cfgOptions)
	return len(fileIn(tb, gb, crateName).Decls) == 1
}
