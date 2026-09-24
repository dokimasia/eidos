// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Docs(nil), "", "no lines, no comment")
		assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "// One.\n// Two.\n",
			"one line comment per line")
		assert.Equal(t, backend.Docs([]string{"Inner."}, "\t"), "\t// Inner.\n",
			"a member's doc indents to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("[]store.Row")), "[]store.Row",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "any",
			"a declaration stating no type spells the empty interface")
		assert.Equal(t, backend.Spell(&emit.TypeRef{}), "any",
			"and so does a reference spelling nothing")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("string"),
				{Spelling: "List", Args: []*emit.TypeRef{ref("User")}},
			},
		}), "Map[string, List[User]]",
			"an argument list spells in brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{{Name: "T"}})
		assert.NoError(t, err, "an unbounded parameter spells")
		assert.Equal(t, got, "[T any]", "under the any constraint")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "K", Bounds: []*emit.TypeRef{ref("comparable")}},
			{Name: "V", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}},
		})
		assert.NoError(t, err, "bounded parameters spell")
		assert.Equal(t, got, "[K comparable, V interface{ Codec; Closer }]",
			"one bound stands alone, several fold into a constraint interface")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err, "variance refuses, because Go states none")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("int")},
		})
		assert.HasError(t, err, "a value parameter refuses")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Default: ref("string")},
		})
		assert.HasError(t, err, "a default refuses")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params(nil), "", "no parameters, no list")
		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "ctx", Type: ref("context.Context")},
			{Name: "keys", Type: ref("string"), Variadic: symbol.VariadicPositional},
		}), "ctx context.Context, keys ...string",
			"names, types and the variadic marker")
		assert.Equal(t, backend.Params([]*emit.Param{{Type: ref("int")}}), "int",
			"an unnamed parameter is its type alone")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Results(nil), "", "no results, no list")
		assert.Equal(t, backend.Results([]*emit.Return{{Type: ref("error")}}), " error",
			"one bare result stands alone")
		assert.Equal(t, backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("error")},
		}), " (Row, error)", "several parenthesise")
		assert.Equal(t, backend.Results([]*emit.Return{{Name: "n", Type: ref("int")}}),
			" (n int)", "one named result parenthesises too, because Go requires it")
	})

	t.Run("Receiver", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Receiver(&emit.Method{
			Receiver: &emit.Param{Name: "s", Type: ref("*Store")},
		}), "s *Store", "a declared receiver spells name and type")
		assert.Equal(t, backend.Receiver(&emit.Method{Receives: ref("*Store")}),
			"*Store", "a method attached from outside carries the type alone")
		assert.Equal(t, backend.Receiver(&emit.Method{}), "",
			"no receiver, no spelling")
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Package(symbol.Identity{Name: "store"}), "store",
			"the identity's own name wins")
		assert.Equal(t, backend.Package(symbol.Identity{Package: "svc/api"}), "api",
			"and the path's last element stands in")
		assert.Equal(t, backend.Package(symbol.Identity{}), "",
			"an identity naming nothing spells nothing, and the formatter refuses")
	})

	t.Run("Guard", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Guard(&emit.Struct{Name: "Row", Final: true})
		assert.NoError(t, err, "a final struct passes")
		assert.Equal(t, got, "", "and holds by writing nothing, because nothing subclasses")

		got, err = backend.Guard(&emit.Variable{
			Name: "count", Visibility: symbol.VisibilityPackage,
		})
		assert.NoError(t, err, "package visibility passes")
		assert.Equal(t, got, "", "the name's case carries it")

		_, err = backend.Guard(&emit.Struct{Name: "Row", Abstract: true})
		assert.HasError(t, err, "an abstract struct refuses")
		_, err = backend.Guard(&emit.Function{Name: "Load", Async: true})
		assert.HasError(t, err, "an async function refuses")
		_, err = backend.Guard(&emit.Method{Name: "Load", Level: symbol.LevelType})
		assert.HasError(t, err, "a static method refuses")
		_, err = backend.Guard(&emit.Method{Name: "Load", Override: true})
		assert.HasError(t, err, "an override marker refuses")
		_, err = backend.Guard(&emit.Method{Name: "Close"})
		assert.HasError(t, err, "a method naming no receiver type refuses, because func () Close() does not compile")
		_, err = backend.Guard(&emit.Method{Name: "Close", Receives: ref("Row")})
		assert.NoError(t, err, "and one attached to a type passes")
		_, err = backend.Guard(&emit.Field{Name: "Key", Value: "1"})
		assert.HasError(t, err, "a field initializer refuses")
		_, err = backend.Guard(&emit.Field{
			Name: "Key", Mutability: symbol.MutabilityImmutable,
		})
		assert.HasError(t, err, "a field's own mutability refuses")
		_, err = backend.Guard(&emit.Variable{
			Name: "count", Mutability: symbol.MutabilityImmutable,
		})
		assert.HasError(t, err, "an immutable variable refuses, because that is a constant")
		_, err = backend.Guard(&emit.Constant{
			Name:        "Max",
			Value:       "8",
			Annotations: symbol.Annotations{{Name: "nolint"}},
		})
		assert.NoError(t, err,
			"annotations pass the guard, because they render as directive lines")
		_, err = backend.Guard(&emit.Constant{Name: "Max"})
		assert.HasError(t, err,
			"a constant without a value refuses, because const Max = declares nothing")
		_, err = backend.Guard(&emit.Alias{
			Name: "ID", Visibility: symbol.VisibilityProtected,
		})
		assert.HasError(t, err, "a scope no case carries refuses")
		_, err = backend.Guard(&emit.Struct{
			Name:       "Row",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed")},
		})
		assert.NoError(t, err,
			"supertypes pass: extends spells as embedding, and implements "+
				"holds through structural satisfaction")
	})

	t.Run("EmbedGuard", func(t *testing.T) {
		t.Parallel()

		got, err := backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Comment: "stream side"})
		assert.NoError(t, err, "an interface's embed with a comment passes")
		assert.Equal(t, got, "", "and writes nothing")
		_, err = backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Tag: `json:"r"`})
		assert.HasError(t, err, "a tag refuses, because Go gives a struct field alone one")
	})

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Directives(nil), "", "no annotations, no lines")
		assert.Equal(t, backend.Directives(symbol.Annotations{
			{Name: "go:embed", Args: []string{"schema.sql"}},
			{Name: "nolint", Args: []string{"errcheck"}},
		}),
			"//go:embed schema.sql\n//nolint errcheck\n",
			"one directive comment per annotation, arguments space-joined, "+
				"the spelling verbatim")
	})

	t.Run("VarType", func(t *testing.T) {
		t.Parallel()

		got, err := backend.VarType(&emit.Variable{Name: "count", Type: ref("int")})
		assert.NoError(t, err, "a typed variable spells")
		assert.Equal(t, got, " int", "its type behind the space")

		got, err = backend.VarType(&emit.Variable{Name: "count", Value: "8"})
		assert.NoError(t, err, "an initialized variable spells")
		assert.Equal(t, got, "", "no type, so Go infers instead of any")

		_, err = backend.VarType(&emit.Variable{Name: "count"})
		assert.HasError(t, err,
			"a variable stating neither refuses, because var x alone "+
				"declares nothing Go accepts")
	})

	t.Run("SigGuard", func(t *testing.T) {
		t.Parallel()

		got, err := backend.SigGuard(&emit.Method{Name: "Get", Abstract: true})
		assert.NoError(t, err, "an abstract interface method passes")
		assert.Equal(t, got, "", "a bodiless signature is the interface's shape")

		_, err = backend.SigGuard(&emit.Method{Name: "Get", HasDefault: true})
		assert.HasError(t, err, "a default body refuses")
		got, err = backend.SigGuard(&emit.Method{Name: "Get", Final: true})
		assert.NoError(t, err, "a final marker passes, because nothing overrides in Go")
		assert.Equal(t, got, "", "and holds by writing nothing")
		_, err = backend.SigGuard(&emit.Method{
			Name: "Map", TypeParams: []*emit.TypeParam{{Name: "T"}},
		})
		assert.HasError(t, err, "type parameters refuse, because Go gives an interface method none")
		_, err = backend.SigGuard(&emit.Method{Name: "Get", Async: true})
		assert.HasError(t, err, "an async signature refuses")
		_, err = backend.SigGuard(&emit.Method{Name: "Get", Body: emit.Body{Verbatim: "return nil"}})
		assert.HasError(t, err, "a body refuses, because the signature cannot place it")
	})
}
