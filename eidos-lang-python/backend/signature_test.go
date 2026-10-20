// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a signature: the definition.
const (
	functionAllocs = 1
	methodAllocs   = 1
)

// TestSignature pins the definitions of functions and of methods: their
// parameters in every form that a caller passes them, their results, their
// docstrings and their bodies.
func TestSignature(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Function", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.Function
				giveBody    string
				giveVersion plugin.Choice
				giveStub    bool
				want        string
			}{
				{
					name: "returns a function without content with the body ... on its line",
					give: &emit.Function{Name: "load"}, want: "def load() -> None: ...\n",
				},
				{
					name: "returns the parameters with their annotations and their defaults in a group",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef},
						&emit.Param{Name: "limit", Type: intRef, Default: "10"},
						&emit.Param{Name: "page", Default: "1"},
					)},
					want: "def load(" + hugMark + "key: str" + sepMark + "limit: int = 10" + sepMark + "page=1" +
						closeMark + ") -> None: ...\n",
				},
				{
					name: "returns a parameter that a caller may leave out with None among its types",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef, Optional: true},
					)},
					want: "def load(" + hugMark + "key: str | None = None" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns the variadic parameters with their stars",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "args", Type: intRef, Variadic: symbol.VariadicPositional},
						&emit.Param{Name: "kwargs", Type: strRef, Variadic: symbol.VariadicKeyword},
					)},
					want: "def load(" + hugMark + "*args: int" + sepMark + "**kwargs: str" + closeMark +
						") -> None: ...\n",
				},
				{
					name: "returns a star before the first parameter that a caller passes by keyword alone",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef},
						&emit.Param{Name: "limit", Type: intRef, Passing: symbol.PassingKeyword},
						&emit.Param{Name: "page", Type: intRef, Passing: symbol.PassingKeyword},
					)},
					want: "def load(" + hugMark + "key: str" + sepMark + "*" + sepMark + "limit: int" + sepMark +
						"page: int" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns no star after a positional variadic parameter",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "args", Variadic: symbol.VariadicPositional},
						&emit.Param{Name: "limit", Passing: symbol.PassingKeyword},
					)},
					want: "def load(" + hugMark + "*args" + sepMark + "limit" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns a slash after the last parameter that a caller passes by position alone",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef, Passing: symbol.PassingPositional},
						&emit.Param{Name: "limit", Type: intRef},
					)},
					want: "def load(" + hugMark + "key: str" + sepMark + "/" + sepMark + "limit: int" + closeMark +
						") -> None: ...\n",
				},
				{
					name: "returns an underscore for each parameter without a name with a number after the first",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Type: strRef}, &emit.Param{Type: intRef},
					)},
					want: "def load(" + hugMark + "_: str" + sepMark + "_1: int" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns the comment of a parameter after the mark of a comment",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef, Comment: "the row key"},
					)},
					want: "def load(" + hugMark + "key: str" + commentMark + "the row key" + closeMark +
						") -> None: ...\n",
				},
				{
					name: "returns the metadata of the annotated marker of a parameter in Annotated",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
						Name: "key", Type: strRef, Annotations: symbol.Annotations{{
							Name: string(python.AnnotatedMarker), Args: []string{`"key"`},
						}},
					})},
					want: "def load(" + hugMark + "key: Annotated[str, \"key\"]" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns an asynchronous function with async",
					give: &emit.Function{Name: "load", Async: true}, want: "async def load() -> None: ...\n",
				},
				{
					name:        "returns the type parameters of a function after its name",
					give:        &emit.Function{Name: "load", TypeParams: []*emit.TypeParam{{Name: "T"}}},
					giveVersion: python.Version312, want: "def load[T]() -> None: ...\n",
				},
				{
					name: "returns the type of one result after the arrow",
					give: &emit.Function{Name: "load", Returns: emit.SlotOf(&emit.Return{Type: intRef})},
					want: "def load() -> int: ...\n",
				},
				{
					name: "returns no arrow for one result without a type",
					give: &emit.Function{Name: "load", Returns: emit.SlotOf(&emit.Return{})},
					want: "def load(): ...\n",
				},
				{
					name: "returns a tuple of two results",
					give: &emit.Function{
						Name: "load", Returns: emit.SlotOf(&emit.Return{Type: intRef}, &emit.Return{Type: strRef}),
					},
					want: "def load() -> tuple[int, str]: ...\n",
				},
				{
					name: "returns the docstring of a function without content before the body ...",
					give: &emit.Function{Name: "load", Doc: []string{"Load reads a row."}},
					want: "def load() -> None:\n    \"\"\"Load reads a row.\"\"\"\n    ...\n",
				},
				{
					name:     "returns the content that the body builtin placed after the docstring",
					give:     &emit.Function{Name: "load", Doc: []string{"Load reads a row."}},
					giveBody: "    return\n",
					want:     "def load() -> None:\n    \"\"\"Load reads a row.\"\"\"\n    return\n",
				},
				{
					name: "returns the trailing comment of a function after its signature",
					give: &emit.Function{Name: "load", Comment: "cached"}, giveBody: "    return\n",
					want: "def load() -> None:  # cached\n    return\n",
				},
				{
					name: "returns the body ... and the default ... in a stub file",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(
						&emit.Param{Name: "limit", Type: intRef, Default: "10"},
					)},
					giveBody: "    return\n", giveStub: true,
					want: "def load(" + hugMark + "limit: int = ..." + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns the decorators of a function before its definition",
					give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "traced"}}},
					want: "@traced\ndef load() -> None: ...\n",
				},
				{
					name: "returns the decorator overload before an overload signature",
					give: &emit.Function{Name: "load", Overload: true},
					want: "@" + overloadDecorator + "\ndef load() -> None: ...\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), tt.giveStub)
					got, err := w.Function(nil, tt.give, tt.giveBody)
					assert.NoError(t, err, "Python declares the function")
					assert.Equal(t, got, tt.want, "the definition")
				})
			}

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Function(
					&emit.File{Annotations: symbol.Annotations{jsonImport}},
					&emit.Function{Name: "load"},
					"",
				)
				assert.NoError(t, err, "Python declares the function")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Function(
					&emit.File{Annotations: symbol.Annotations{badImport}},
					&emit.Function{Name: "load"},
					"",
				)
				assert.HasError(t, err, "the function refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			t.Run("returns the implementation of a function after the overloads of its name", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				assert.Total(t, func(f *emit.Function) error {
					_, err := w.Function(nil, f, "")
					return err
				}, []*emit.Function{
					{Name: "find", Overload: true}, {Name: "find", Overload: true}, {Name: "find"},
				}, "Python declares the overloads and the implementation of one function")
			})

			t.Run("returns an error for a function of a name that an earlier function without an overload declares",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Function(nil, &emit.Function{Name: "find"}, "")
					assert.NoError(t, err, "Python declares the first function")
					_, err = w.Function(nil, &emit.Function{Name: "find", Overload: true}, "")
					assert.HasError(t, err, "Python refuses the second function")
					assert.Equal(t, err.Error(), "python: a module declares two functions named find, and a module "+
						"declares a name once outside the overloads of one function", "the refusal names the function")
				})

			refusals := []struct {
				name string
				give *emit.Function
				want string
			}{
				{
					name: "returns an error for a parameter whose name is no identifier",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{Name: "row-key"})},
					want: `python: a parameter has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns an error for a variadic parameter with a default",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
						Name: "args", Variadic: symbol.VariadicPositional, Default: "()",
					})},
					want: "python: a variadic parameter takes no default, and args states one",
				},
				{
					name: "returns the error of the type of a parameter",
					give: &emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
						Name: "key", Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					})},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of the type of a result",
					give: &emit.Function{Name: "load", Returns: emit.SlotOf(&emit.Return{
						Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					})},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of the types of two results",
					give: &emit.Function{Name: "load", Returns: emit.SlotOf(&emit.Return{Type: intRef}, &emit.Return{
						Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					})},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a type parameter",
					give: &emit.Function{Name: "load", TypeParams: []*emit.TypeParam{{Name: "N", Const: true}}},
					want: "python: a type parameter takes a type, and N takes a value",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.Function(nil, tt.give, "")
					assert.HasError(t, err, "Python refuses the function")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})

		t.Run("Method", func(t *testing.T) {
			t.Parallel()

			class := &emit.Struct{Name: "Store"}
			tests := []struct {
				name     string
				giveHost symbol.Symbol
				give     *emit.Method
				giveBody string
				want     string
			}{
				{
					name: "returns an instance method with self one level inside its class",
					give: &emit.Method{Name: "get"},
					want: "    def get(" + hugMark + "self" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns the name of the receiver that an instance method states",
					give: &emit.Method{Name: "get", Receiver: &emit.Param{Name: "store"}},
					want: "    def get(" + hugMark + "store" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns a static method without a receiver",
					give: &emit.Method{Name: "create", Level: symbol.LevelType},
					want: "    @staticmethod\n    def create() -> None: ...\n",
				},
				{
					name: "returns a class method with cls",
					give: &emit.Method{
						Name: "create", Level: symbol.LevelType, Annotations: symbol.Annotations{{Name: "classmethod"}},
					},
					want: "    @classmethod\n    def create(" + hugMark + "cls" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns a constructing method as __init__",
					give: &emit.Method{Name: "new", Constructs: true, Params: emit.SlotOf(
						&emit.Param{Name: "key", Type: strRef},
					)},
					want: "    def __init__(" + hugMark + "self" + sepMark + "key: str" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns abstractmethod for an abstract method of a class",
					give: &emit.Method{Name: "get", Abstract: true},
					want: "    @abstractmethod\n    def get(" + hugMark + "self" + closeMark + ") -> None: ...\n",
				},
				{
					name:     "returns no abstractmethod for an abstract method of a Protocol",
					giveHost: &emit.Interface{Name: "Priced"},
					give:     &emit.Method{Name: "get", Abstract: true},
					want:     "    def get(" + hugMark + "self" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns a constructing method named __new__ under its name",
					give: &emit.Method{Name: "__new__", Constructs: true},
					want: "    def __new__(" + hugMark + "self" + closeMark + ") -> None: ...\n",
				},
				{
					name: "returns an index signature as __getitem__ and __setitem__",
					give: &emit.Method{
						Name: "index", Indexer: true, Params: emit.SlotOf(&emit.Param{Name: "key", Type: strRef}),
						Returns: emit.SlotOf(&emit.Return{Type: intRef}),
					},
					want: "    def __getitem__(" + hugMark + "self" + sepMark + "key: str" + closeMark + ") -> int: ...\n\n" +
						"    def __setitem__(" + hugMark + "self" + sepMark + "key: str" + sepMark + "value: int" +
						closeMark + ") -> None: ...\n",
				},
				{
					name: "returns an immutable index signature as __getitem__ alone",
					give: &emit.Method{
						Name: "index", Indexer: true, Mutability: symbol.MutabilityImmutable,
						Params:  emit.SlotOf(&emit.Param{Name: "key", Type: strRef}),
						Returns: emit.SlotOf(&emit.Return{Type: intRef}),
					},
					want: "    def __getitem__(" + hugMark + "self" + sepMark + "key: str" + closeMark + ") -> int: ...\n",
				},
				{
					name:     "returns the docstring and the body of a method one level deeper",
					give:     &emit.Method{Name: "get", Doc: []string{"Get reads a row."}},
					giveBody: "        return\n",
					want: "    def get(" + hugMark + "self" + closeMark + ") -> None:\n        \"\"\"Get reads a row.\"\"\"\n" +
						"        return\n",
				},
				{
					name:     "returns a method of a Protocol",
					giveHost: &emit.Interface{Name: "Store"}, give: &emit.Method{Name: "get"},
					want: "    def get(" + hugMark + "self" + closeMark + ") -> None: ...\n",
				},
				{
					name:     "returns builtins.bytes in a signature where a method of the class has its name",
					giveHost: &emit.Struct{Name: "Buffer", Methods: emit.SlotOf(&emit.Method{Name: "bytes"})},
					give:     &emit.Method{Name: "data", Returns: emit.SlotOf(&emit.Return{Type: bytesRef})},
					want:     "    def data(" + hugMark + "self" + closeMark + ") -> builtins.bytes: ...\n",
				},
				{
					name:     "returns builtins.staticmethod for a static method where a field of the class has its name",
					giveHost: &emit.Struct{Name: "Store", Fields: emit.SlotOf(&emit.Field{Name: "staticmethod"})},
					give:     &emit.Method{Name: "create", Level: symbol.LevelType},
					want:     "    @builtins.staticmethod\n    def create() -> None: ...\n",
				},
				{
					name: "returns builtins.bytes in a signature where a member of the enum of the method has its name",
					giveHost: &emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "bytes"}),
					},
					give: &emit.Method{Name: "data", Returns: emit.SlotOf(&emit.Return{Type: bytesRef})},
					want: "    def data(" + hugMark + "self" + closeMark + ") -> builtins.bytes: ...\n",
				},
				{
					name: "returns builtins.bytes in a signature where a method of the enum of the method has its name",
					giveHost: &emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
						Methods: emit.SlotOf(&emit.Method{Name: "bytes"}),
					},
					give: &emit.Method{Name: "data", Returns: emit.SlotOf(&emit.Return{Type: bytesRef})},
					want: "    def data(" + hugMark + "self" + closeMark + ") -> builtins.bytes: ...\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					got, err := w.Method(cmp.Or[symbol.Symbol](tt.giveHost, class), tt.give, tt.giveBody)
					assert.NoError(t, err, "Python declares the method")
					assert.Equal(t, got, tt.want, "the definition")
				})
			}

			t.Run("returns a builtin by its name in a function after a method of a class that has its name",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					host := &emit.Struct{Name: "Buffer", Methods: emit.SlotOf(&emit.Method{Name: "bytes"})}
					_, err := w.Method(host, &emit.Method{Name: "data"}, "")
					assert.NoError(t, err, "Python declares the method")
					load := &emit.Function{Name: "load", Returns: emit.SlotOf(&emit.Return{Type: bytesRef})}
					got, err := w.Function(nil, load, "")
					assert.NoError(t, err, "Python declares the function")
					assert.Equal(t, got, "def load() -> bytes: ...\n", "the builtin of the function")
				})

			t.Run("returns a type parameter of the syntax of PEP 695 that hides the type variable of the class",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.TypeParams([]*emit.TypeParam{{Name: "T", Variance: symbol.VarianceOut}})
					assert.NoError(t, err, "a covariant parameter declares the type variable T")
					host := &emit.Struct{Name: "Pack", TypeParams: []*emit.TypeParam{{
						Name: "T", Variance: symbol.VarianceOut, Bounds: []*emit.TypeRef{{Spelling: "Codec"}},
					}}}
					got, err := w.Method(host, &emit.Method{
						Name: "box", TypeParams: []*emit.TypeParam{{Name: "T"}},
						Params: emit.SlotOf(&emit.Param{Name: "item", Type: &emit.TypeRef{Spelling: "T"}}),
					}, "")
					assert.NoError(t, err, "Python declares the method")
					want := "    def box[T](" + hugMark + "self" + sepMark + "item: T" + closeMark + ") -> None: ...\n"
					assert.Equal(t, got, want, "the parameter of the method in place of the variable T2 of the class")
				})

			refusals := []struct {
				name     string
				giveHost symbol.Symbol
				give     *emit.Method
				want     string
			}{
				{
					name:     "returns an error for a constructing method of a Protocol",
					giveHost: &emit.Interface{Name: "Store"},
					give:     &emit.Method{Name: "new", Constructs: true},
					want:     "python: a Protocol declares no construct signature, and new states one",
				},
				{
					name: "returns an error for an index signature without one key",
					give: &emit.Method{Name: "index", Indexer: true, Returns: emit.SlotOf(&emit.Return{Type: intRef})},
					want: "python: an index signature takes one key and returns one value, and index does not",
				},
				{
					name: "returns the error of the key of an index signature",
					give: &emit.Method{
						Name: "index", Indexer: true, Params: emit.SlotOf(&emit.Param{Name: "row-key"}),
						Returns: emit.SlotOf(&emit.Return{Type: intRef}),
					},
					want: `python: a parameter has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns the error of the value of a mutable index signature",
					give: &emit.Method{
						Name: "index", Indexer: true, Params: emit.SlotOf(&emit.Param{Name: "key", Type: strRef}),
						Returns: emit.SlotOf(&emit.Return{
							Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
						}),
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a parameter",
					give: &emit.Method{Name: "get", Params: emit.SlotOf(&emit.Param{Name: "row-key"})},
					want: `python: a parameter has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns the error of a type parameter of the class of the method",
					giveHost: &emit.Struct{
						Name: "Box", TypeParams: []*emit.TypeParam{{Name: "N", Const: true}},
					},
					give: &emit.Method{Name: "get"},
					want: "python: a type parameter takes a type, and N takes a value",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Method(cmp.Or[symbol.Symbol](tt.giveHost, class), tt.give, "")
					assert.HasError(t, err, "Python refuses the method")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})
	})
}

// Function and Method allocate the definition. The ordinary run, which runs
// no benchmark, checks those ceilings here.
func TestSignatureAllocs(t *testing.T) {
	checkAllocs(t, signatureCalls(t))
}

// BenchmarkSignature measures the definitions of a function and of a
// method.
func BenchmarkSignature(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, signatureCalls(b))
	})
}

// signatureCalls returns the calls of Function and of Method for a callable
// with one parameter and one result. Each call of Function writes the
// function into a file of its own after a Reset of the set, because a module
// declares a function of a name once.
func signatureCalls(tb assert.TB) []allocCall {
	w, set := newWriter(tb, python.Version311, false)
	host := &emit.Struct{Name: "Store"}
	f := &emit.Function{
		Name: "load", Params: emit.SlotOf(&emit.Param{Name: "key", Type: strRef}),
		Returns: emit.SlotOf(&emit.Return{Type: intRef}),
	}
	m := &emit.Method{
		Name: "get", Params: emit.SlotOf(&emit.Param{Name: "key", Type: strRef}),
		Returns: emit.SlotOf(&emit.Return{Type: intRef}),
	}
	var got string
	return []allocCall{
		{
			name: "Function", allocs: functionAllocs,
			call: func() {
				set.Reset()
				got, _ = w.Function(nil, f, "")
			},
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "def load("+hugMark+"key: str"+closeMark+") -> int: ...\n",
					"Function writes the definition")
			},
		},
		{
			name: "Method", allocs: methodAllocs,
			call: func() { got, _ = w.Method(host, m, "") },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "    def get("+hugMark+"self"+sepMark+"key: str"+closeMark+") -> int: ...\n",
					"Method writes the definition")
			},
		},
	}
}
