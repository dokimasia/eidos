// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/backend"
)

// formatAllocs is the allocation of a layout: the output.
const formatAllocs = 1

// layoutInput is a module that the allocation test and the benchmark lay
// out: a class with a docstring, a field and a wrapped method, and a
// function after it.
const layoutInput = "from shop import Row\n\nclass Store:\n    \"\"\"Store reads rows back.\"\"\"\n\n" +
	"    rows: int\n\n    def get(" + hugMark + "self" + sepMark + "key: str" + commentMark + "the row key" +
	closeMark + ") -> Row: ...\n\ndef main() -> None: ...\n"

// layoutOutput is the layout of [layoutInput].
const layoutOutput = "from shop import Row\n\n\nclass Store:\n    \"\"\"Store reads rows back.\"\"\"\n\n" +
	"    rows: int\n\n    def get(\n        self,\n        key: str,  # the row key\n    ) -> Row: ...\n\n\n" +
	"def main() -> None: ...\n"

// TestFormat pins the layout of a module and of a stub file, each the layout
// that ruff's formatter leaves as it is.
func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("Format", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name     string
				give     string
				giveStub bool
				want     string
			}{
				{name: "returns the empty input as it is"},
				{
					name: "returns a group on one line where the line fits",
					give: "def f(" + hugMark + "a: int" + sepMark + "b: str" + closeMark + ") -> None: ...\n",
					want: "def f(a: int, b: str) -> None: ...\n",
				},
				{
					name: "returns the items of a signature on one indented line where the line is too long",
					give: "def a_function_with_a_long_name(" + hugMark + "first_parameter: int" + sepMark +
						"second_parameter: str" + sepMark + "third: bytes" + closeMark + ") -> None: ...\n",
					want: "def a_function_with_a_long_name(\n    first_parameter: int, second_parameter: str, " +
						"third: bytes\n) -> None: ...\n",
				},
				{
					name: "returns one item on each line where the indented line is too long",
					give: "def f(" + hugMark + "a_first_parameter_with_a_long_name: int" + sepMark +
						"a_second_parameter_with_a_long_name: str" + sepMark + "third: bytes" + closeMark +
						") -> None: ...\n",
					want: "def f(\n    a_first_parameter_with_a_long_name: int,\n" +
						"    a_second_parameter_with_a_long_name: str,\n    third: bytes,\n) -> None: ...\n",
				},
				{
					name: "returns the elements of a literal on one line where the line fits",
					give: "__all__ = [" + listMark + "\"Row\"" + sepMark + "\"Store\"" + closeMark + "]\n",
					want: "__all__ = [\"Row\", \"Store\"]\n",
				},
				{
					name: "returns one element of a literal on each line where the line is too long",
					give: "__all__ = [" + listMark + "\"a_first_name_of_a_module\"" + sepMark +
						"\"a_second_name_of_a_module\"" + sepMark + "\"a_third_name_of_a_module\"" + closeMark + "]\n",
					want: "__all__ = [\n    \"a_first_name_of_a_module\",\n    \"a_second_name_of_a_module\",\n" +
						"    \"a_third_name_of_a_module\",\n]\n",
				},
				{
					name: "returns one item on each line of a group with a comment",
					give: "def get(" + hugMark + "self" + sepMark + "key: str" + commentMark + "the row key" +
						closeMark + ") -> str: ...\n",
					want: "def get(\n    self,\n    key: str,  # the row key\n) -> str: ...\n",
				},
				{
					name: "returns the group of an indented line at its indentation",
					give: "class Store:\n    def get(" + hugMark + "self" + sepMark + "key: str" + commentMark +
						"the key" + closeMark + ") -> str: ...\n",
					want: "class Store:\n    def get(\n        self,\n        key: str,  # the key\n    ) -> str: ...\n",
				},
				{
					name: "returns two blank lines around a definition at the top level of a module",
					give: "import os\n\nclass A: ...\n\nx = 1\n",
					want: "import os\n\n\nclass A: ...\n\n\nx = 1\n",
				},
				{
					name: "returns one blank line between two statements that the templates separate",
					give: "x = 1\n\n\n\ny = 2\n",
					want: "x = 1\n\ny = 2\n",
				},
				{
					name: "returns no blank line between two statements that the templates do not separate",
					give: "x = 1\ny = 2\n",
					want: "x = 1\ny = 2\n",
				},
				{
					name: "returns no blank line between two overloads that the templates do not separate",
					give: "@overload\ndef f(x: int) -> int: ...\n@overload\ndef f(x: str) -> str: ...\n",
					want: "@overload\ndef f(x: int) -> int: ...\n@overload\ndef f(x: str) -> str: ...\n",
				},
				{
					name: "returns no blank line before the first line of a block",
					give: "class A:\n\n    x: int\n",
					want: "class A:\n    x: int\n",
				},
				{
					name: "returns one blank line inside a block for a run of blank lines",
					give: "class A:\n    x: int\n\n\n    def f(self) -> None: ...\n",
					want: "class A:\n    x: int\n\n    def f(self) -> None: ...\n",
				},
				{
					name: "returns the closing bracket of a statement after it",
					give: "from os import (\n    path,\n)\n\nx = 1\n",
					want: "from os import (\n    path,\n)\n\nx = 1\n",
				},
				{
					name: "returns the decorator of a definition before it",
					give: "x = 1\n\n@final\nclass A: ...\n",
					want: "x = 1\n\n\n@final\nclass A: ...\n",
				},
				{
					name: "returns the lines inside a string that spans lines as they are",
					give: "x = \"\"\"a  \n\n\n  b\"\"\"\n\ny = 2\n",
					want: "x = \"\"\"a  \n\n\n  b\"\"\"\n\ny = 2\n",
				},
				{
					name: "returns a line without its trailing whitespace",
					give: "x = 1   \t\n",
					want: "x = 1\n",
				},
				{
					name: "returns the file from its first line that is not blank",
					give: "\n\nx = 1\n",
					want: "x = 1\n",
				},
				{
					name: "returns the file with one newline at its end",
					give: "x = 1\n\n\n",
					want: "x = 1\n",
				},
				{
					name: "returns a newline after a last line without one",
					give: "x = 1",
					want: "x = 1\n",
				},
				{
					name: "returns a block opener before a comment as the opener of a block",
					give: "class A:  # keyed\n\n    x: int\n",
					want: "class A:  # keyed\n    x: int\n",
				},
				{
					name: "returns a hash inside a string as no comment",
					give: "x = \"a: #\"\n\ny = 1\n",
					want: "x = \"a: #\"\n\ny = 1\n",
				},
				{
					name: "returns a triple quote in the comment of an item as no string",
					give: "def f(" + hugMark + "a: int" + commentMark + "see \"\"\"" + closeMark +
						") -> None: ...\n\n\n\nx = 1\n",
					want: "def f(\n    a: int,  # see \"\"\"\n) -> None: ...\n\n\nx = 1\n",
				},
				{
					name: "returns no blank line between two functions of a stub file whose bodies are the ellipsis",
					give: "def f() -> None: ...  # c\n\ndef g() -> None: ...\n", giveStub: true,
					want: "def f() -> None: ...  # c\ndef g() -> None: ...\n",
				},
				{
					name:     "returns no blank line between two methods of a stub file whose bodies are the ellipsis",
					give:     "class A:\n    def a(self) -> None: ...  # c\n\n    def b(self) -> None: ...\n",
					giveStub: true,
					want:     "class A:\n    def a(self) -> None: ...  # c\n    def b(self) -> None: ...\n",
				},
				{
					name: "returns no blank line between two classes of a stub file whose bodies are the ellipsis",
					give: "class A: ...  # c\n\nclass B: ...\n", giveStub: true,
					want: "class A: ...  # c\nclass B: ...\n",
				},
				{
					name: "returns no blank line between a function of a stub file and the decorated class after it",
					give: "def f() -> None: ...\n\n@final\nclass A: ...\n", giveStub: true,
					want: "def f() -> None: ...\n@final\nclass A: ...\n",
				},
				{
					name: "returns the blank lines of a stub file around its nested definitions",
					give: "x: int\n\nclass A: ...\nclass B:\n    y: int\n\n    def m(self) -> None: ...\n    @property\n" +
						"    def p(self) -> int: ...\n    class N: ...\n    def q(self) -> None: ...\ny: int\n",
					giveStub: true,
					want: "x: int\n\nclass A: ...\n\nclass B:\n    y: int\n\n    def m(self) -> None: ...\n    @property\n" +
						"    def p(self) -> int: ...\n    class N: ...\n\n    def q(self) -> None: ...\n\ny: int\n",
				},
				{
					name: "returns the blank lines of a stub file after its docstring, its imports and its classes",
					give: "\"\"\"Doc.\"\"\"\nfrom a import b\nx: int\n@final\nclass A: ...\n@final\nclass B: ...\n" +
						"class C: ...\nclass D: ...\ndef f() -> None:\n    \"\"\"Doc.\"\"\"\n    ...\ndef g() -> None: ...\n",
					giveStub: true,
					want: "\"\"\"Doc.\"\"\"\n\nfrom a import b\n\nx: int\n\n@final\nclass A: ...\n\n@final\nclass B: ...\n\n" +
						"class C: ...\nclass D: ...\n\ndef f() -> None:\n    \"\"\"Doc.\"\"\"\n    ...\n\n" +
						"def g() -> None: ...\n",
				},
				{
					name: "returns a block indented by two levels as it is",
					give: "def f() -> None:\n        return\n", want: "def f() -> None:\n        return\n",
				},
				{
					name: "returns a decorated definition at the end of the input with a line break",
					give: "@final\nclass A: ...", want: "@final\nclass A: ...\n",
				},
				{
					name: "returns a decorator at the end of the input as it is",
					give: "x = 1\n@final\n", want: "x = 1\n@final\n",
				},
				{
					name: "returns an escaped quote of a string that a line leaves open as part of the string",
					give: "x = \"\"\"a \\\"\"\" b\nc\"\"\"\ny = 1\n", want: "x = \"\"\"a \\\"\"\" b\nc\"\"\"\ny = 1\n",
				},
				{
					name: "returns a string in triple single quotes that a line leaves open as it is",
					give: "x = '''a\nb'''\ny = 1\n", want: "x = '''a\nb'''\ny = 1\n",
				},
				{
					name: "returns the mark of the comment of an item without the end of the item as it is",
					give: "x = 1  " + commentMark + "c\n", want: "x = 1  " + commentMark + "c\n",
				},
				{
					name: "returns one blank line after the docstring of a class of a stub file",
					give: "class A(" + hugMark + "B" + commentMark + "it's" + closeMark + "):  # c\n    \"\"\"Doc.\"\"\"\n" +
						"    x: int\n",
					giveStub: true,
					want:     "class A(\n    B,  # it's\n):  # c\n    \"\"\"Doc.\"\"\"\n\n    x: int\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.Layout{Stub: tt.giveStub}.Format([]byte(tt.give))
					assert.NoError(t, err, "a layout never fails")
					assert.Equal(t, string(got), tt.want, "the laid out file")
				})
			}
		})
	})
}

// Format allocates the output. The ordinary run, which runs no benchmark,
// checks that ceiling here.
func TestFormatAllocs(t *testing.T) {
	checkAllocs(t, formatCalls())
}

// BenchmarkFormat measures the layout of a module and of a stub file.
func BenchmarkFormat(b *testing.B) {
	b.Run("Layout", func(b *testing.B) {
		benchCalls(b, formatCalls())
	})
}

// formatCalls returns the calls of Format for a module and for a stub file.
func formatCalls() []allocCall {
	src := []byte(layoutInput)
	var got []byte
	return []allocCall{
		{
			name: "Format", caseName: "a module", allocs: formatAllocs,
			call: func() { got, _ = backend.Layout{}.Format(src) },
			check: func(tb assert.TB) {
				assert.Equal(tb, string(got), layoutOutput, "Format lays out the module")
			},
		},
		{
			name: "Format", caseName: "a stub file", allocs: formatAllocs,
			call: func() { got, _ = backend.Layout{Stub: true}.Format(src) },
			check: func(tb assert.TB) {
				assert.HasPrefix(
					tb,
					string(got),
					"from shop import Row\n\nclass Store:",
					"Format lays out the stub file",
				)
			},
		},
	}
}
