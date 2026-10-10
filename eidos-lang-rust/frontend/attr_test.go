// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// cfgOptions is the cfg set the evaluation cases load under: the feature
// x, the option unix, and the key target_os set to linux.
var cfgOptions = &frontend.Options{Features: []string{"x"}, Cfg: []string{"unix", "target_os=\"linux\""}}

// The markers that the sugar cases write. stubMarker opens an attribute
// of the brand, and stubName is the directive that it attaches.
const (
	stubMarker = "#[" + brand + "::stub"
	stubName   = directive.Name("stub")
)

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
			assert.Contains(t, stampKeys(gb, fn), rust.TestKey, "the test function")
		})

		t.Run("stamps an item #[cfg(test)] marks rust.test and loads it", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#[cfg(test)]\nfn helper() {}\n")
			fn := named[*node.Function](t, fileIn(t, gb, crateName).Decls, "helper")
			assert.Contains(t, stampKeys(gb, fn), rust.TestKey, "the test-only function")
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

	t.Run("marker", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the directive of an attribute of the brand to its struct", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, stubMarker+"]\npub struct A;\n")
			assert.Empty(t, found, "the marker lifts")
			a := named[*node.Struct](t, fileIn(t, gb, crateName).Decls, "A")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(a), "the directive is on the struct",
				assert.ByIdentity())
			want := directive.Raw{Name: stubName, Pos: position.Pos{File: libRoot, Line: 1, Col: 1}}
			expect.Equal(t, attached[0].Raw, want, "the directive has the attribute's position and no arguments")
		})

		t.Run("keeps an attribute of the brand as an annotation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, structOf(t, stubMarker+"]\npub struct A;\n", "A").Annotations,
				symbol.Annotations{{Name: brand + "::stub"}}, "the struct has the attribute as an annotation")
		})

		t.Run("attaches the prefixed directive of an attribute with a plugin", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "#["+brand+"::gen::table]\npub struct A;\n")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			assert.Equal(t, attached[0].Raw.Name, directive.Name("gen:table"), "the name has the plugin's prefix")
		})

		t.Run("attaches nothing for an attribute outside the brand", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#[serde(rename = f())]\npub struct A;\n")
			expect.Empty(t, found, "an attribute outside the brand is metadata, whatever its arguments are")
			expect.Empty(t, gb.Attachments(), "an attribute outside the brand attaches no directive")
		})

		subjects := []struct {
			name    string
			give    string
			subject func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol
		}{
			{
				name: "attaches the directive of a marker of a field to the field",
				give: "pub struct A {\n    " + stubMarker + "]\n    pub f: i32,\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Struct](tb, fileIn(tb, gb, crateName).Decls, "A").Fields[0]
				},
			},
			{
				name: "attaches the directive of a marker of a variant to the variant",
				give: "pub enum E {\n    " + stubMarker + "]\n    V,\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Enum](tb, fileIn(tb, gb, crateName).Decls, "E").Variants[0]
				},
			},
			{
				name: "attaches the directive of a marker of a trait method to the method",
				give: "pub trait T {\n    " + stubMarker + "]\n    fn run(&self);\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Interface](tb, fileIn(tb, gb, crateName).Decls, "T").Methods[0]
				},
			},
			{
				name: "attaches the directive of a marker of a mod item to its package",
				give: stubMarker + "]\npub mod inner {}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return packageIn(tb, gb, crateName+"/inner")
				},
			},
			{
				name: "attaches the directive of an inner marker to its module's package",
				give: "#![" + brand + "::stub]\npub struct A;\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return packageIn(tb, gb, crateName)
				},
			},
		}
		for _, tt := range subjects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, tt.give)
				assert.Empty(t, found, "the marker lifts")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				expect.Equal(t, attached[0].Raw.Name, stubName, "the directive has the marker's name")
				expect.Equal(t, attached[0].Subject, tt.subject(t, gb), "the directive is on the marked subject",
					assert.ByIdentity())
			})
		}

		lifts := []struct {
			name string
			give string
			want []directive.RawArg
		}{
			{
				name: "lifts a key with a string as a keyed argument", give: `(tag = "test")`,
				want: []directive.RawArg{{Key: "tag", Value: directive.RawValue{Text: "test", Quoted: true}}},
			},
			{
				name: "lifts a name as a positional argument", give: "(writer)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "writer"}}},
			},
			{
				name: "lifts a raw string as its content", give: `(r"a\n")`,
				want: []directive.RawArg{{Value: directive.RawValue{Text: `a\n`, Quoted: true}}},
			},
			{
				name: "lifts a number with a minus sign as negative decimal text", give: "(n = -2)",
				want: []directive.RawArg{{Key: "n", Value: directive.RawValue{Text: "-2"}}},
			},
			{
				name: "lifts a hexadecimal integer as its decimal text", give: "(0x10)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "16"}}},
			},
			{
				name: "lifts an octal integer as its decimal text", give: "(0o17)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "15"}}},
			},
			{
				name: "lifts a binary integer as its decimal text", give: "(0b101)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "5"}}},
			},
			{
				name: "lifts an integer without its separators and its type suffix", give: "(1_000u32)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "1000"}}},
			},
			{
				name: "lifts an integer beyond 64 bits with every digit", give: "(18446744073709551616)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "18446744073709551616"}}},
			},
			{
				name: "lifts a float without its type suffix as its shortest decimal text", give: "(1.50f32)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "1.5"}}},
			},
			{
				name: "lifts true as written", give: "(ok = true)",
				want: []directive.RawArg{{Key: "ok", Value: directive.RawValue{Text: "true"}}},
			},
			{
				name: "lifts two arguments in order", give: `("x", n = 1)`,
				want: []directive.RawArg{
					{Value: directive.RawValue{Text: "x", Quoted: true}},
					{Key: "n", Value: directive.RawValue{Text: "1"}},
				},
			},
		}
		for _, tt := range lifts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, stubMarker+tt.give+"]\npub struct A;\n")
				assert.Empty(t, found, "the arguments lift")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				assert.Equal(t, attached[0].Raw.Args, tt.want, "the directive has the lifted arguments")
			})
		}

		refusals := []struct {
			name string
			give string
			want string
		}{
			{name: "reports BadMarker for a call", give: "(f())", want: "the argument f() is not a literal or a name"},
			{
				name: "reports BadMarker for an array", give: "([1, 2])",
				want: "the argument [1, 2] is not a literal or a name",
			},
			{
				name: "reports BadMarker for a character", give: "('c')",
				want: "the argument 'c' is not a literal or a name",
			},
			{
				name: "reports BadMarker for a minus sign before a string", give: `(-"x")`,
				want: `the argument -"x" is not a literal or a name`,
			},
			{
				name: "reports BadMarker for a float beyond the largest double", give: "(1e400)",
				want: "the argument 1e400 is not a literal or a name",
			},
			{
				name: "reports BadMarker for an integer of separators alone", give: "(0x_)",
				want: "the argument 0x_ is not a literal or a name",
			},
			{
				name: "reports BadMarker for an attribute that assigns a value", give: ` = "x"`,
				want: `the attribute assigns "x", and a marker takes its arguments in parentheses`,
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, stubMarker+tt.give+"]\npub struct A;\n")
				assert.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker},
					"the frontend reports the marker under BadMarker")
				expect.Contains(t, found[0].Msg, tt.want, "the finding describes the argument")
				expect.Equal(t, found[0].Pos, position.Pos{File: libRoot, Line: 1, Col: 1},
					"the finding is at the attribute")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}

		t.Run("reports BadMarker for an attribute of the brand alone", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "#["+brand+"]\npub struct A;\n")
			expect.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker}, "the brand alone is not a directive name")
			expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
		})

		unaddressed := []struct {
			name string
			give string
		}{
			{
				name: "reports UnaddressedCarrier for a marker on a use declaration",
				give: stubMarker + "]\nuse std::fmt;\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on an extern block",
				give: stubMarker + "]\nextern \"C\" {}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on an impl block",
				give: "pub struct A;\n" + stubMarker + "]\nimpl A {}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on an item of a trait impl",
				give: "pub struct A;\nimpl Clone for A {\n    " + stubMarker + "]\n    fn clone(&self) -> A { A }\n}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on a macro",
				give: stubMarker + "]\nmacro_rules! m { () => {} }\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on a parameter",
				give: "pub fn f(" + stubMarker + "] a: i32) {}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on a macro of a trait",
				give: "pub trait T {\n    " + stubMarker + "]\n    m!();\n}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker on a macro of an impl block",
				give: "pub struct A;\nimpl A {\n    " + stubMarker + "]\n    m!();\n}\n",
			},
		}
		for _, tt := range unaddressed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, tt.give)
				assert.Contains(t, codesOf(found), frontend.UnaddressedCarrier, "the marker attaches nowhere")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}

		t.Run("reports UnaddressedCarrier for a marker on an associated constant of an absent type",
			func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, "impl Missing {\n    "+stubMarker+"]\n    pub const N: i32 = 1;\n}\n")
				assert.Contains(t, codesOf(found), frontend.UnaddressedCarrier, "the constant has no home")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
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
func loadedUnder(tb testing.TB, pred string) bool {
	tb.Helper()

	src := "#[cfg(" + pred + ")]\npub struct A;\n"
	gb, _ := parsedTree(tb, crateTree(map[string]string{libRoot: src}), libRoot, plugin.DepthFull, cfgOptions)
	return len(fileIn(tb, gb, crateName).Decls) == 1
}
