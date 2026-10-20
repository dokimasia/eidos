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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// fieldsAllocs is the allocations of the lines of two fields of a class.
const fieldsAllocs = 1

// TestFields pins the lines of the fields of a class, of a Protocol and of a
// TypedDict: their annotations with the qualifiers of their facts, their
// values, their comments and their docstrings.
func TestFields(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Struct", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.Field
				giveVersion plugin.Choice
				giveStub    bool
				want        string
			}{
				{
					name: "returns a field with its type",
					give: &emit.Field{Name: nameField, Type: strRef}, want: "    name: str\n",
				},
				{
					name: "returns the value of a field after an equals sign",
					give: &emit.Field{Name: "limit", Type: intRef, Value: "10"}, want: "    limit: int = 10\n",
				},
				{
					name: "returns a field without a type and with a value as an attribute",
					give: &emit.Field{Name: "limit", Value: "10"}, want: "    limit = 10\n",
				},
				{
					name: "returns a field without a type and without a value with the type of a dynamic value",
					give: &emit.Field{Name: "data"}, want: "    data: object\n",
				},
				{
					name: "returns a field that a value may lack with None among its types and as its value",
					give: &emit.Field{Name: nameField, Type: strRef, Optional: true},
					want: "    name: str | None = None\n",
				},
				{
					name: "returns the value of a field that a value may lack",
					give: &emit.Field{Name: nameField, Type: strRef, Optional: true, Value: `"a"`},
					want: "    name: str | None = \"a\"\n",
				},
				{
					name: "returns no None for a field of an optional type",
					give: &emit.Field{
						Name: nameField, Optional: true,
						Type: &emit.TypeRef{Form: symbol.FormOptional, Elems: []*emit.TypeRef{strRef}},
					},
					want: "    name: str | None = None\n",
				},
				{
					name: "returns the value ... in a stub file",
					give: &emit.Field{Name: "limit", Type: intRef, Value: "10"}, giveStub: true,
					want: "    limit: int = ...\n",
				},
				{
					name: "returns ClassVar for a field of the type",
					give: &emit.Field{Name: "count", Type: intRef, Value: "0", Level: symbol.LevelType},
					want: "    count: ClassVar[int] = 0\n",
				},
				{
					name: "returns Final for an immutable field",
					give: &emit.Field{Name: "key", Type: strRef, Value: `"a"`, Mutability: symbol.MutabilityImmutable},
					want: "    key: Final[str] = \"a\"\n",
				},
				{
					name: "returns Final alone for an immutable field of the type before Python 3.13",
					give: &emit.Field{
						Name: "LIMIT", Type: intRef, Value: "8", Level: symbol.LevelType,
						Mutability: symbol.MutabilityImmutable,
					},
					giveVersion: python.Version312, want: "    LIMIT: Final[int] = 8\n",
				},
				{
					name: "returns ClassVar of Final for an immutable field of the type from Python 3.13",
					give: &emit.Field{
						Name: "LIMIT", Type: intRef, Value: "8", Level: symbol.LevelType,
						Mutability: symbol.MutabilityImmutable,
					},
					giveVersion: python.Version313, want: "    LIMIT: ClassVar[Final[int]] = 8\n",
				},
				{
					name: "returns InitVar for a field with the InitVar marker",
					give: &emit.Field{
						Name: "seed", Type: intRef,
						Annotations: symbol.Annotations{{Name: string(python.InitVarMarker)}},
					},
					want: "    seed: InitVar[int]\n",
				},
				{
					name: "returns the metadata of the annotated marker around the whole annotation",
					give: &emit.Field{
						Name: "key", Type: strRef, Mutability: symbol.MutabilityImmutable, Value: `"a"`,
						Annotations: symbol.Annotations{{
							Name: string(python.AnnotatedMarker), Args: []string{`"meta"`, "Gt(0)"},
						}},
					},
					want: "    key: Annotated[Final[str], \"meta\", Gt(0)] = \"a\"\n",
				},
				{
					name: "returns the comment and the docstring of a field",
					give: &emit.Field{
						Name: nameField, Type: strRef, Comment: "unique", Doc: []string{"Name keys the row."},
					},
					want: "    name: str  # unique\n    \"\"\"Name keys the row.\"\"\"\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), tt.giveStub)
					got, err := w.Struct(nil, &emit.Struct{Name: rowName, Fields: emit.SlotOf(tt.give)})
					assert.NoError(t, err, "Python declares the field")
					assert.Equal(t, got.Fields, tt.want, "the lines of the field")
				})
			}

			t.Run("returns an error for a field of a type that Python does not spell", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Struct(nil, &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: nameField, Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
				})})
				assert.HasError(t, err, "Python refuses the field")
				assert.Equal(t, err.Error(),
					"python: A & B is an intersection of types, which Python declares as a Protocol",
					"the refusal names the type")
			})
		})

		t.Run("Interface", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name          string
				give          *emit.Field
				giveTypedDict bool
				giveKeywords  []string
				giveVersion   plugin.Choice
				want          string
			}{
				{
					name: "returns an immutable field of a Protocol as a read-only property",
					give: &emit.Field{Name: nameField, Type: strRef, Mutability: symbol.MutabilityImmutable},
					want: "    @property\n    def name(self) -> str: ...\n",
				},
				{
					name: "returns the docstring of the property of a Protocol in its body",
					give: &emit.Field{
						Name: nameField, Mutability: symbol.MutabilityImmutable, Comment: "unique",
						Doc: []string{"Name keys the row."},
					},
					want: "    @property\n    def name(self) -> object:  # unique\n" +
						"        \"\"\"Name keys the row.\"\"\"\n        ...\n",
				},
				{
					name: "returns a mutable field of a Protocol as an attribute",
					give: &emit.Field{Name: nameField, Type: strRef}, want: "    name: str\n",
				},
				{
					name: "returns NotRequired for a key of a TypedDict that a value may lack",
					give: &emit.Field{Name: nameField, Type: strRef, Optional: true}, giveTypedDict: true,
					want: "    name: NotRequired[str]\n",
				},
				{
					name: "returns ReadOnly of NotRequired for an immutable key of a TypedDict that a value may lack",
					give: &emit.Field{
						Name: nameField, Type: strRef, Optional: true, Mutability: symbol.MutabilityImmutable,
					},
					giveTypedDict: true, giveVersion: python.Version313,
					want: "    name: ReadOnly[NotRequired[str]]\n",
				},
				{
					name:          "returns Required for a key of a TypedDict of total=False that a value has",
					give:          &emit.Field{Name: nameField, Type: strRef},
					giveTypedDict: true,
					giveKeywords:  []string{partialKeyword},
					want:          "    name: Required[str]\n",
				},
				{
					name:          "returns the type alone for a key of a TypedDict of total=False that a value may lack",
					give:          &emit.Field{Name: nameField, Type: strRef, Optional: true},
					giveTypedDict: true,
					giveKeywords:  []string{partialKeyword},
					want:          "    name: str\n",
				},
				{
					name:          "returns ReadOnly of Required for an immutable key of a TypedDict of total=False",
					give:          &emit.Field{Name: nameField, Type: strRef, Mutability: symbol.MutabilityImmutable},
					giveTypedDict: true,
					giveKeywords:  []string{partialKeyword},
					giveVersion:   python.Version313,
					want:          "    name: ReadOnly[Required[str]]\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					i := &emit.Interface{Name: rowName, Fields: emit.SlotOf(tt.give)}
					if tt.giveTypedDict {
						i.Annotations = symbol.Annotations{typedDictMarker}
					}
					if tt.giveKeywords != nil {
						i.Annotations = append(i.Annotations,
							symbol.Annotation{Name: string(python.KeywordsMarker), Args: tt.giveKeywords})
					}
					w, _ := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					got, err := w.Interface(nil, i)
					assert.NoError(t, err, "Python declares the field")
					assert.Equal(t, got.Fields, tt.want, "the lines of the field")
				})
			}

			t.Run("returns an error for the property of a Protocol of a type that Python does not spell",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Interface(nil, &emit.Interface{Name: rowName, Fields: emit.SlotOf(&emit.Field{
						Name: nameField, Mutability: symbol.MutabilityImmutable,
						Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					})})
					assert.HasError(t, err, "Python refuses the field")
					assert.Equal(t, err.Error(),
						"python: A & B is an intersection of types, which Python declares as a Protocol",
						"the refusal names the type")
				})
		})
	})
}

// The lines of the fields allocate once. The ordinary run, which runs no
// benchmark, checks that ceiling here, as the lines of a Protocol, whose
// head allocates once more.
func TestFieldsAllocs(t *testing.T) {
	checkAllocs(t, fieldsCalls(t))
}

// BenchmarkFields measures the lines of the fields of a Protocol.
func BenchmarkFields(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, fieldsCalls(b))
	})
}

// fieldsCalls returns the call of Interface for a Protocol with two fields,
// one of them a property.
func fieldsCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version311, false)
	store := &emit.Interface{Name: "Store", Fields: emit.SlotOf(
		&emit.Field{Name: idField, Type: intRef},
		&emit.Field{Name: nameField, Type: strRef, Mutability: symbol.MutabilityImmutable},
	)}
	var fields string
	return []allocCall{
		{
			name: "Interface", allocs: fieldsAllocs + 1,
			call: func() {
				got, _ := w.Interface(nil, store)
				fields = got.Fields
			},
			check: func(tb assert.TB) {
				assert.Equal(tb, fields, "    id: int\n    @property\n    def name(self) -> str: ...\n",
					"Interface writes the fields")
			},
		},
	}
}
