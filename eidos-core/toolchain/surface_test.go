// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/toolchain"
)

// surfaceGolden is the name of the golden file of a case of AssertSurface.
const surfaceGolden = "surface.txt"

// everySurface is the surface of the declarations of everyKind, in byte
// order under the line of their package.
const everySurface = `package gen
Alias ID = string
Alias Name string
Constant Bare
Constant Limit int = 8
Row embeds Mixin
Row extends Base
Row implements Reader
Row.Kind.A = 1
Row.Kind.B
Row.Touch()
Row.note? string
Shape.Area() float
Shape.Circle(radius float)
Shape.Empty()
Store extends Closer
Store permits Row
Store.Get(string) string
Store.handle Handle
Sum Shape[T]
Variable Loose
abstract Struct Row[T Comparable]
async Function Load[T any](at: string, ...int) (Row, error)
async Row.fetch(...string, int? = 10) (Row, error)
const Enum Row.Kind
constructor Row.New()
final Struct Leaf
get Row.Value() int
indexer Row.At(int) string
readonly Row.id string
readonly Variable Count int
sealed Interface Store[K]
set Row.Value(int)
static Row.Of(string) Row
static Row.total int
`

// The surface of generated output is the model of its public
// declarations, so each kind renders its own line and nothing else
// changes the text.
func TestSurface(t *testing.T) {
	t.Parallel()

	t.Run("Surface", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a line for each public feature of every kind", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			every := literal{Scripted: frontendtest.NewScripted(), decls: everyKind}
			got, err := toolchain.Surface(t.Context(), every, g)
			assert.NoError(t, err, "the output loads")
			assert.Equal(t, got, everySurface, "the surface has one line for each public feature, in byte order")
		})

		t.Run("returns one surface for other parameter names and another order of members", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			first, err := toolchain.Surface(t.Context(),
				literal{Scripted: frontendtest.NewScripted(), decls: reordered(false)}, g)
			assert.NoError(t, err, "the first output loads")
			second, err := toolchain.Surface(t.Context(),
				literal{Scripted: frontendtest.NewScripted(), decls: reordered(true)}, g)
			assert.NoError(t, err, "the second output loads")
			assert.Equal(t, second, first, "the order of the members and the names of the parameters are not surface")
		})

		t.Run("returns an empty surface for output without a public declaration", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			got, err := toolchain.Surface(t.Context(), literal{Scripted: frontendtest.NewScripted(), decls: hidden}, g)
			assert.NoError(t, err, "the output loads")
			assert.Empty(t, got, "the surface has no line")
		})

		t.Run("returns an error for an Error of the load", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{badFile: []byte("type Lost string\n")}}
			_, err := toolchain.Surface(t.Context(), frontendtest.NewScripted(), g)
			assert.HasError(t, err, "the Error of the load fails the surface")
			assert.Contains(t, err.Error(), badFile, "the error contains the position of the Error")
		})

		t.Run("returns the error of a source that does not read", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{
				Files:   map[string][]byte{apiFile: nil},
				Sources: os.DirFS(filepath.Join(t.TempDir(), missingDir)),
			}
			_, err := toolchain.Surface(t.Context(), frontendtest.NewScripted(), g)
			assert.ErrorIs(t, err, fs.ErrNotExist, "the sources of the fixture do not read")
		})
	})

	t.Run("AssertSurface", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a golden that matches the surface", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{surfaceGolden: files.Text(everySurface)})
			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			toolchain.AssertSurface(t, literal{Scripted: frontendtest.NewScripted(), decls: everyKind}, g,
				filepath.ToSlash(filepath.Join(dir, surfaceGolden)))
		})

		t.Run("fails a golden that differs from the surface", func(t *testing.T) {
			t.Parallel()

			golden := filepath.Join(files.Workspace(t, files.Tree{surfaceGolden: files.Text("package gen\n")}),
				surfaceGolden)
			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			got := assert.Rejects(t, "a golden of another surface", func(tb assert.TB) {
				toolchain.AssertSurface(tb, literal{Scripted: frontendtest.NewScripted(), decls: everyKind}, g,
					filepath.ToSlash(golden))
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"the golden file " + golden + " matches the output, and -update writes it"},
				"the rejection names the golden file")
		})

		t.Run("stops the test where the output declares nothing public", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{apiFile: nil}}
			private := literal{Scripted: frontendtest.NewScripted(), decls: hidden}
			got := assert.Rejects(t, "output without a public declaration", func(tb assert.TB) {
				toolchain.AssertSurface(tb, private, g, surfaceGolden)
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{string(frontendtest.ScriptedLang) + ": the generated output declares something public"},
				"the rejection reports the empty surface")
		})

		t.Run("stops the test where the load reports an Error", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{badFile: []byte("type Lost string\n")}}
			got := assert.Rejects(t, "output that does not load", func(tb assert.TB) {
				toolchain.AssertSurface(tb, frontendtest.NewScripted(), g, surfaceGolden)
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{string(frontendtest.ScriptedLang) + ": the generated output loads through its frontend"},
				"the rejection reports the load")
		})

		t.Run("stops the test without a frontend", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a check without a frontend", func(tb assert.TB) {
				toolchain.AssertSurface(tb, nil, output(), surfaceGolden)
			})
			assert.Equal(t, coretest.Contracts(got), []string{"toolchain: a frontend loads the generated output"},
				"the rejection reports the missing frontend")
		})
	})
}

// everyKind returns a declaration of every kind that the surface renders,
// with every modifier, and the declarations that the surface leaves out:
// a struct, a function, a field and a method that are not public, a
// method of a host that is not public, a field at file level and a
// variant at file level.
func everyKind(path, _ string) []symbol.Symbol {
	at := position.Pos{File: path, Line: 1}
	public := symbol.VisibilityPublic
	private := symbol.VisibilityPrivate
	row := &node.Struct{
		Name: "Row", Pos: at, Visibility: public, Abstract: true,
		TypeParams: []*node.TypeParam{{Name: "T", Bounds: []*node.TypeRef{{Spelling: "Comparable"}}}},
		Extends:    []*node.TypeRef{{Spelling: "Base"}},
		Implements: []*node.TypeRef{{Spelling: "Reader"}},
		Embeds:     []*node.Embed{{Ref: &node.TypeRef{Spelling: "Mixin"}}},
		Fields: []*node.Field{
			{
				Name:       "id",
				Visibility: public,
				Mutability: symbol.MutabilityImmutable,
				Type:       &node.TypeRef{Spelling: "string"},
			},
			{Name: "note", Visibility: public, Optional: true, Type: &node.TypeRef{Spelling: "string"}},
			{Name: "total", Visibility: public, Level: symbol.LevelType, Type: &node.TypeRef{Spelling: "int"}},
			{Name: "secret", Visibility: private, Type: &node.TypeRef{Spelling: "string"}},
		},
		Methods: []*node.Method{
			{
				Name: "Of", Visibility: public, Level: symbol.LevelType,
				Params:  []*node.Param{{Name: "id", Type: &node.TypeRef{Spelling: "string"}}},
				Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "Row"}}},
			},
			{
				Name:       "fetch",
				Visibility: public,
				Async:      true,
				Params: []*node.Param{
					{Name: "keys", Variadic: symbol.VariadicPositional, Type: &node.TypeRef{Spelling: "string"}},
					{Name: "limit", Optional: true, Default: "10", Type: &node.TypeRef{Spelling: "int"}},
				},
				Returns: []*node.Return{
					{Type: &node.TypeRef{Spelling: "Row"}},
					{Type: &node.TypeRef{Spelling: "error"}},
				},
			},
			{
				Name: "Value", Visibility: public, Accessor: symbol.AccessorGet,
				Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "int"}}},
			},
			{
				Name: "Value", Visibility: public, Accessor: symbol.AccessorSet,
				Params: []*node.Param{{Name: "v", Type: &node.TypeRef{Spelling: "int"}}},
			},
			{Name: "New", Visibility: public, Constructs: true},
			{
				Name: "At", Visibility: public, Indexer: true,
				Params:  []*node.Param{{Name: "i", Type: &node.TypeRef{Spelling: "int"}}},
				Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
			},
			{Name: "hidden", Visibility: private},
		},
		Types: node.Symbols{
			&node.Enum{
				Name: "Kind", Visibility: public, Const: true,
				Variants: []*node.EnumVariant{{Name: "A", Value: "1"}, {Name: "B"}},
			},
			&node.Struct{Name: "Inner", Visibility: private},
		},
	}
	store := &node.Interface{
		Name: "Store", Pos: at, Visibility: public, Sealed: true,
		TypeParams: []*node.TypeParam{{Name: "K"}},
		Extends:    []*node.TypeRef{{Spelling: "Closer"}},
		Permits:    []*node.TypeRef{{Spelling: "Row"}},
		Fields:     []*node.Field{{Name: "handle", Visibility: public, Type: &node.TypeRef{Spelling: "Handle"}}},
		Methods: []*node.Method{{
			Name: "Get", Visibility: public,
			Params:  []*node.Param{{Name: "key", Type: &node.TypeRef{Spelling: "string"}}},
			Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "string"}}},
		}},
	}
	shape := &node.Sum{
		Name: "Shape", Pos: at, Visibility: public,
		TypeParams: []*node.TypeParam{{Name: "T"}},
		Variants: []*node.SumVariant{
			{Name: "Circle", Fields: []*node.Field{{Name: "radius", Type: &node.TypeRef{Spelling: "float"}}}},
			{Name: "Empty"},
		},
		Methods: []*node.Method{{
			Name: "Area", Visibility: public, Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "float"}}},
		}},
	}
	return []symbol.Symbol{
		row,
		store,
		shape,
		&node.Struct{Name: "Leaf", Pos: at, Visibility: public, Final: true},
		&node.Alias{Name: "ID", Pos: at, Visibility: public, Target: &node.TypeRef{Spelling: "string"}},
		&node.Alias{
			Name:       "Name",
			Pos:        at,
			Visibility: public,
			Defined:    true,
			Target:     &node.TypeRef{Spelling: "string"},
		},
		&node.Function{
			Name: "Load", Pos: at, Visibility: public, Async: true,
			TypeParams: []*node.TypeParam{{Name: "T", Bounds: []*node.TypeRef{{Spelling: "any"}}}},
			Params: []*node.Param{
				{Name: "path", Label: "at", Type: &node.TypeRef{Spelling: "string"}},
				{Name: "sizes", Variadic: symbol.VariadicPositional, Type: &node.TypeRef{Spelling: "int"}},
			},
			Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "Row"}}, {Type: &node.TypeRef{Spelling: "error"}}},
		},
		&node.Method{Name: "Touch", Pos: at, Visibility: public, Receives: &node.TypeRef{Spelling: "Row"}},
		&node.Struct{Name: "Hidden", Pos: at, Visibility: private},
		&node.Method{Name: "Peek", Pos: at, Visibility: public, Receives: &node.TypeRef{Spelling: "Hidden"}},
		&node.Function{Name: "helper", Pos: at, Visibility: private},
		&node.Constant{Name: "Limit", Pos: at, Visibility: public, Type: &node.TypeRef{Spelling: "int"}, Value: "8"},
		&node.Constant{Name: "Bare", Pos: at, Visibility: public},
		&node.Variable{
			Name: "Count", Pos: at, Visibility: public, Mutability: symbol.MutabilityImmutable,
			Type: &node.TypeRef{Spelling: "int"},
		},
		&node.Variable{Name: "Loose", Pos: at, Visibility: public},
		&node.Field{Name: "Stray", Pos: at, Visibility: public, Type: &node.TypeRef{Spelling: "int"}},
		&node.EnumVariant{Name: "Lone", Pos: at},
	}
}

// reordered returns the declarations of a struct with two methods. Where
// swap is set, the methods are in the other order and the parameter has
// another name.
func reordered(swap bool) func(path, content string) []symbol.Symbol {
	return func(path, _ string) []symbol.Symbol {
		apply := &node.Method{
			Name: "Apply", Visibility: symbol.VisibilityPublic,
			Params: []*node.Param{{Name: "x", Type: &node.TypeRef{Spelling: "int"}}},
		}
		reset := &node.Method{Name: "Reset", Visibility: symbol.VisibilityPublic}
		methods := []*node.Method{apply, reset}
		if swap {
			apply.Params[0].Name = "y"
			methods = []*node.Method{reset, apply}
		}
		return []symbol.Symbol{&node.Struct{
			Name: "Row", Pos: position.Pos{File: path, Line: 1}, Visibility: symbol.VisibilityPublic, Methods: methods,
		}}
	}
}

// hidden returns a struct that is not public.
func hidden(path, _ string) []symbol.Symbol {
	return []symbol.Symbol{&node.Struct{Name: "Row", Pos: position.Pos{File: path, Line: 1}}}
}
