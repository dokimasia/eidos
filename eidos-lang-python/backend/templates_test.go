// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestTemplates pins the kinds that Python declares at the level of a
// module, the kind that it refuses, and the files that the file skeleton and
// the kind templates write.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a template for each kind that Python declares at the level of a module", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.KindTemplates(), map[symbol.Kind]string{
				symbol.KindStruct:    backend.StructTemplate,
				symbol.KindInterface: backend.InterfaceTemplate,
				symbol.KindEnum:      backend.EnumTemplate,
				symbol.KindFunction:  backend.FunctionTemplate,
				symbol.KindAlias:     backend.AliasTemplate,
				symbol.KindConstant:  backend.ConstantTemplate,
				symbol.KindVariable:  backend.VariableTemplate,
			}, "the kind templates")
		})

		t.Run("returns a map of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.KindTemplates(), symbol.KindStruct)
			assert.Length(t, backend.KindTemplates(), 7, "a change of one map leaves the next map unchanged")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the reason of the method, which Python declares inside its class", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.RefusedKinds(), map[symbol.Kind]string{
				symbol.KindMethod: "Python declares a method inside the class it belongs to",
			}, "the refused kinds")
		})

		t.Run("returns a map of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.RefusedKinds(), symbol.KindMethod)
			assert.Length(t, backend.RefusedKinds(), 1, "a change of one map leaves the next map unchanged")
		})
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			giveBackend kit.Backend
			giveVersion plugin.Choice
			give        []symbol.Symbol
			want        string
		}{
			{
				name:        "writes the head, the imports, __all__, the type variables and the declarations in order",
				giveBackend: backend.New(),
				giveVersion: python.Version311,
				give: []symbol.Symbol{
					&emit.File{Doc: []string{"Shop models."}, Annotations: symbol.Annotations{{
						Name: string(python.AllMarker), Args: []string{"Box", "load"},
					}}},
					&emit.Struct{
						Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}},
						Fields: emit.SlotOf(&emit.Field{Name: "item", Type: &emit.TypeRef{Spelling: "T"}}),
					},
					&emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
						Name: "box", Type: &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "int"}}},
					})},
				},
				want: "\"\"\"Shop models.\"\"\"\n\nfrom typing import Generic, TypeVar\n\n__all__ = [\"Box\", \"load\"]\n\n" +
					"T = TypeVar(\"T\")\n\n\nclass Box(Generic[T]):\n    item: T\n\n\n" +
					"def load(box: Box[int]) -> None: ...\n",
			},
			{
				name:        "writes the nested types, the fields and the methods of a class in that order",
				giveBackend: backend.New(), giveVersion: python.Version312,
				give: []symbol.Symbol{&emit.Struct{
					Name: rowName, Doc: []string{"Row is one record."},
					Types: emit.SlotOf[symbol.Symbol](&emit.Enum{
						Name: "Key", Variants: emit.SlotOf(&emit.EnumVariant{Name: "ID"}),
					}),
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: "int"}}),
					Methods: emit.SlotOf(
						&emit.Method{
							Name:    "get",
							Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "int"}}),
						},
						&emit.Method{
							Name:   "put",
							Params: emit.SlotOf(&emit.Param{Name: "value", Type: &emit.TypeRef{Spelling: "int"}}),
						},
					),
				}},
				want: "from enum import Enum, auto\n\n\nclass Row:\n    \"\"\"Row is one record.\"\"\"\n\n" +
					"    class Key(Enum):\n        ID = auto()\n\n    id: int\n\n    def get(self) -> int: ...\n\n" +
					"    def put(self, value: int) -> None: ...\n",
			},
			{
				name:        "writes the members and the methods of an enum",
				giveBackend: backend.New(), giveVersion: python.Version312,
				give: []symbol.Symbol{&emit.Enum{
					Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
					Methods: emit.SlotOf(&emit.Method{
						Name: "label", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "str"}}),
					}),
				}},
				want: "from enum import Enum, auto\n\n\nclass Phase(Enum):\n    OPEN = auto()\n\n" +
					"    def label(self) -> str: ...\n",
			},
			{
				name:        "writes a function with its body, an alias, a constant and a variable",
				giveBackend: backend.New(), giveVersion: python.Version312,
				give: []symbol.Symbol{
					&emit.Function{
						Name: "load", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "int"}}),
						Body: emit.Body{Verbatim: "    return 1\n"},
					},
					&emit.Alias{Name: "Id", Target: &emit.TypeRef{Spelling: "int"}},
					&emit.Constant{Name: "LIMIT", Type: &emit.TypeRef{Spelling: "int"}, Value: "8"},
					&emit.Variable{Name: "count", Type: &emit.TypeRef{Spelling: "int"}, Value: "0"},
				},
				want: "from typing import Final\n\n\ndef load() -> int:\n    return 1\n\n\ntype Id = int\n\n" +
					"LIMIT: Final[int] = 8\n\ncount: int = 0\n",
			},
			{
				name:        "writes a stub file with the body ... and the blank lines of a stub file",
				giveBackend: backend.Stubs(), giveVersion: python.Version312,
				give: []symbol.Symbol{
					&emit.Function{
						Name: "load", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "int"}}),
						Body: emit.Body{Verbatim: "    return 1\n"},
					},
					&emit.Struct{
						Name: rowName,
						Fields: emit.SlotOf(
							&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: "int"}, Value: "0"},
						),
						Methods: emit.SlotOf(&emit.Method{Name: "get"}, &emit.Method{Name: "put"}),
					},
				},
				want: "def load() -> int: ...\n\nclass Row:\n    id: int = ...\n\n    def get(self) -> None: ...\n" +
					"    def put(self) -> None: ...\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, diags := rendered(t, tt.giveBackend, versionPolicy(t, tt.giveVersion), tt.give...)
				assert.Empty(t, diags, "the render reports nothing")
				assert.Equal(t, got, tt.want, "the file")
			})
		}

		t.Run("withholds a file whose header Python refuses", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(plugin.Unit{
				Plugin: emitter, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
				Pkg: symbol.Identity{Lang: python.Lang, Package: unitPkg, Name: unitName, Kind: symbol.KindPackage},
				Decls: []symbol.Symbol{
					&emit.File{Annotations: symbol.Annotations{{Name: "dataclass"}}},
					&emit.Constant{Name: "LIMIT", Value: "8"},
				},
			}), "the unit arrives")
			files, diags := backendtest.Rendered(t, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
				return backend.New(), &backendtest.Fixture{
					Emit: e, Schedule: []plugin.ID{emitter}, Policy: versionPolicy(tb, python.Version312),
				}
			})
			assert.Empty(t, files, "the render withholds the file")
			assert.Length(t, diags, 1, "the render reports the skeleton")
			assert.Contains(t, diags[0].Msg,
				"python: a Python file states the header markers alone, and its header states dataclass",
				"the report names the annotation of the header")
		})
	})
}
