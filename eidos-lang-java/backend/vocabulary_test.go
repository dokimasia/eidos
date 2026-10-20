// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling cases name the packages a reference imports from and
// the classes it names.
const (
	// storePkg and legacyPkg both declare rowName.
	storePkg  = "svc/store"
	legacyPkg = "svc/legacy"
	// legacyDotted is legacyPkg the way Java writes it.
	legacyDotted = "svc.legacy"
	rowName      = "Row"
	baseName     = "Base"
	innerName    = "Inner"
)

// The declarations and spellings the allocation cases name.
const (
	intType            = "int"
	stringType         = "String"
	comparableType     = "Comparable"
	keyParam           = "K"
	valueParam         = "V"
	idName             = "id"
	nameParam          = "name"
	getName            = "get"
	ioException        = "IOException"
	redConstant        = "RED"
	rowDoc             = "Row is one record."
	overrideAnnotation = "Override"
)

// The allocations of the vocabulary.
const (
	// typeParamsAllocs is a bounded and an unbounded parameter: the
	// bounded one's spelling, the joined list, and its brackets.
	typeParamsAllocs = 1 + 1 + 1
	// paramsAllocs is two parameters: each one's spelling, and the joined
	// list.
	paramsAllocs = 2 + 1
	// clauseAllocs is one clause behind its keyword.
	clauseAllocs = 1
	// funcsAllocs is the vocabulary of a file: the map of twenty-one
	// helpers, four allocations, and the speller's seven bound helpers.
	funcsAllocs = 4 + 7
	// wrapDocsAllocs is the wrap of a single-spaced text: the list of its
	// lines, each of which is a part of the text.
	wrapDocsAllocs = 1
	// typeArgsAllocs is a list of two type parameters: the written list.
	typeArgsAllocs = 1
	// textAllocs is a docblock, an annotation or a package statement,
	// sized once.
	textAllocs = 1
	// joinedKeywordAllocs is a keyword joined behind the access keyword.
	joinedKeywordAllocs = 1
	// twoJoinedKeywordAllocs is two keywords joined behind the access
	// keyword, one join each.
	twoJoinedKeywordAllocs = 2
)

// filledLine is a word as wide as the documentation, so the wrap starts a
// line with the word after it.
var filledLine = strings.Repeat("a", textfmt.DocsWidth)

// The vocabulary is what the kind templates spell through, so each
// helper's output is pinned byte for byte, and every import a
// spelling needs is pinned beside it.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a spell helper bound to the file's import set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spell, is := backend.Funcs(&set)[backend.FuncSpell].(func(*emit.TypeRef) (string, error))
			assert.True(t, is, "the helper spells a reference")
			_, err := spell(imported(storePkg, rowName))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the import is the file's")
		})

		t.Run("returns a spellvalue helper that imports a class as the spell helper does", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spell, is := backend.Funcs(&set)[backend.FuncSpellValue].(func(*emit.TypeRef) (string, error))
			assert.True(t, is, "the helper spells a reference")
			got, err := spell(imported(storePkg, rowName))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, got, rowName, "the simple name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: rowName}},
				"Java imports a class for its type and its value alike")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs(nil), "", "no block")
		})

		t.Run("writes a Javadoc block at the top level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"One."}), "/**\n * One.\n */\n", "one gutter line per line")
		})
	})

	t.Run("WrapDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps a block tag on the line before it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" @see next"), []string{filledLine + " @see", "next"},
				"a Javadoc block tag appears at the beginning of a line")
		})

		t.Run("starts a line with a dash", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" - next"), []string{filledLine, "- next"},
				"a Javadoc comment reads a dash as text")
		})
	})

	t.Run("MemberDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.MemberDocs(nil), "", "no block")
		})

		t.Run("writes the Javadoc block behind the member indent", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.MemberDocs([]string{"One."}), "    /**\n     * One.\n     */\n",
				"the whole block indented to the member's depth")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a reference that names no package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref("List<Row>")), "List<Row>", "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no import")
		})

		t.Run("returns Object for a missing reference", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, nil), backend.Anonymous, "the root type")
		})

		t.Run("writes an argument list in angle brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{
				Spelling: "Map",
				Args: []*emit.TypeRef{
					ref("String"),
					{Spelling: "List", Args: []*emit.TypeRef{ref(rowName)}},
				},
			}), "Map<String, List<Row>>", "the arguments recurse")
		})

		t.Run("imports the class a reference names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, imported(storePkg, rowName)), rowName, "the simple name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: rowName}}, "the class's import")
		})

		t.Run("imports a target of the language from its package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName,
				Target:   symbol.Identity{Lang: java.Lang, Package: storePkg, Name: rowName},
			}
			assert.Equal(t, spelled(t, s, target), rowName, "the simple name")
			assert.Equal(t, set.Paths(), []string{storePkg}, "imported from the declaring package")
		})

		t.Run("writes the qualified name of a class whose simple name another import claimed", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			spelled(t, s, imported(storePkg, rowName))
			assert.Equal(t, spelled(t, s, imported(legacyPkg, rowName)), legacyDotted+"."+rowName,
				"the way javac reads a clash")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the first class alone is imported")
		})

		t.Run("writes the qualified name of a class whose simple name the file declares", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.Reserve(rowName)
			assert.Equal(t, spelled(t, s, imported(legacyPkg, rowName)), legacyDotted+"."+rowName,
				"the file's own class keeps the simple name")
		})

		t.Run("imports a member class through its file-level class", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, imported(storePkg, rowName+".Key")), rowName+".Key",
				"the spelling as written")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: rowName}}, "the outer class's import")
		})

		t.Run("writes a class of the file's own package bare", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(storePkg)
			assert.Equal(t, spelled(t, s, imported(storePkg, rowName)), rowName, "no import needed")
			assert.Equal(t, set.Len(), 0, "so none is recorded")
		})

		t.Run("returns an error for a composite whose child is written qualified", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storePkg, rowName))
			array := &emit.TypeRef{
				Form: symbol.FormArray, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(legacyPkg, rowName)},
			}
			_, err := s.Spell(array)
			assert.HasError(t, err, "the spelling does not follow from the structure")
			assert.Contains(t, err.Error(), string(java.Lang)+": ", "under the language's prefix")
		})
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		typeParams := func(t *testing.T, ps ...*emit.TypeParam) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.TypeParams(ps)
		}

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t)
			assert.NoError(t, err, "no parameters spell")
			assert.Equal(t, got, "", "as nothing")
		})

		t.Run("returns an error for a bound that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeParams([]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{unspellable(t, s)}}})
			assert.HasError(t, err, "the bound's refusal is the list's")
		})

		t.Run("joins the bounds with ampersands behind extends", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t,
				&emit.TypeParam{Name: "K"},
				&emit.TypeParam{Name: "V", Bounds: refs("Codec", "Closeable")})
			assert.NoError(t, err, "the parameters spell")
			assert.Equal(t, got, "<K, V extends Codec & Closeable>", "the bounds joined")
		})

		tests := []struct {
			name string
			give *emit.TypeParam
		}{
			{name: "returns an error for a variance", give: &emit.TypeParam{Name: "T", Variance: symbol.VarianceOut}},
			{
				name: "returns an error for a value parameter",
				give: &emit.TypeParam{Name: "N", Const: true, Type: ref("int")},
			},
			{name: "returns an error for a default", give: &emit.TypeParam{Name: "T", Default: ref("String")}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := typeParams(t, tt.give)
				assert.HasError(t, err, "Java's type parameters state none of it")
			})
		}
	})

	t.Run("TypeArgs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give any
			want string
		}{
			{name: "returns nothing for no list", give: nil, want: ""},
			{name: "returns nothing for an empty list", give: []*emit.TypeParam{}, want: ""},
			{
				name: "writes the names of type parameters as the arguments of a reference",
				give: []*emit.TypeParam{{Name: keyParam}, {Name: valueParam, Bounds: refs(comparableType)}},
				want: "<K, V>",
			},
			{
				name: "writes the spellings of type references as the arguments of a reference",
				give: []*emit.TypeRef{ref(stringType), {Spelling: "List", Args: refs(rowName)}},
				want: "<String, List<Row>>",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				got, err := s.TypeArgs(tt.give)
				assert.NoError(t, err, "the list spells")
				assert.Equal(t, got, tt.want, "the type arguments")
			})
		}

		t.Run("imports the class of a type reference", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			got, err := s.TypeArgs([]*emit.TypeRef{imported(storePkg, rowName)})
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, "<Row>", "the simple name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: rowName}}, "the class's import")
		})

		t.Run("returns an error for a type reference that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeArgs([]*emit.TypeRef{unspellable(t, s)})
			assert.HasError(t, err, "the reference's refusal is the list's")
		})

		t.Run("returns an error for a list of another type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeArgs([]string{rowName})
			assert.HasError(t, err, "a list of names is no list of type arguments")
		})
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		params := func(t *testing.T, ps ...*emit.Param) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Params(new(emit.SlotOf(ps...)))
			assert.NoError(t, err, "the list spells")
			return got
		}

		t.Run("writes each type before its name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Name: "key", Type: ref("String")}), "String key", "the Java order")
		})

		t.Run("writes a variadic parameter's type behind three dots", func(t *testing.T) {
			t.Parallel()

			rest := &emit.Param{Name: "rest", Type: ref("int"), Variadic: symbol.VariadicPositional}
			assert.Equal(t, params(t, rest), "int... rest", "the variadic marker")
		})

		t.Run("names an unnamed parameter by its position", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Type: ref("int")}), "int arg0", "Java requires a name")
		})

		t.Run("writes the annotations of a parameter before its type", func(t *testing.T) {
			t.Parallel()

			key := &emit.Param{Name: "key", Type: ref("String"), Annotations: symbol.Annotations{
				{Name: "NotNull"}, {Name: "Size", Args: []string{"max = 8"}},
			}}
			assert.Equal(t, params(t, key), "@NotNull @Size(max = 8) String key", "Java annotates a parameter inline")
		})

		t.Run("returns an error for an assigned annotation of a parameter", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params(new(emit.SlotOf(&emit.Param{
				Name: "key", Type: ref("String"), Annotations: assignedAnnotations(),
			})))
			assert.HasError(t, err, "Java passes the arguments of an annotation in parentheses alone")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params(new(emit.SlotOf(&emit.Param{Name: "rows", Type: unspellable(t, s)})))
			assert.HasError(t, err, "the type's refusal is the list's")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		t.Run("writes void for no results", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Results(&emit.Slot[*emit.Return]{})
			assert.NoError(t, err, "no results spell")
			assert.Equal(t, got, "void", "the empty return type")
		})

		t.Run("writes one result as itself", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Results(new(emit.SlotOf(&emit.Return{Type: ref(rowName)})))
			assert.NoError(t, err, "one result spells")
			assert.Equal(t, got, rowName, "the return type")
		})

		t.Run("returns an error for a result that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results(new(emit.SlotOf(&emit.Return{Type: unspellable(t, s)})))
			assert.HasError(t, err, "the result's refusal is the return type's")
		})

		t.Run("returns an error for a second result", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results(new(emit.SlotOf(&emit.Return{Type: ref(rowName)}, &emit.Return{Type: ref("Err")})))
			assert.HasError(t, err, "a second result arrives thrown, not returned")
		})
	})

	t.Run("PackageClause", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the package with dots for slashes", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.PackageClause(symbol.Identity{Package: "svc/api"}), "package svc.api;\n\n",
				"a blank line after the clause")
		})

		t.Run("returns nothing for the default package", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.PackageClause(symbol.Identity{}), "", "no clause")
		})
	})

	t.Run("TypeMods", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    symbol.Symbol
			want    string
			wantErr bool
		}{
			{name: "writes public for an unstated visibility", give: &emit.Struct{Name: rowName}, want: "public "},
			{
				name: "writes final alone for a package-scoped final class",
				give: &emit.Struct{Name: rowName, Visibility: symbol.VisibilityPackage, Final: true},
				want: "final ",
			},
			{
				name: "writes abstract behind the access of an abstract class",
				give: &emit.Struct{Name: rowName, Abstract: true},
				want: "public abstract ",
			},
			{
				name: "writes a private abstract static sealed member class in Java's order",
				give: &emit.Struct{
					Name: innerName, Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType,
					Abstract: true, Sealed: true,
				},
				want: "private abstract static sealed ",
			},
			{
				name: "writes a protected static final member class in Java's order",
				give: &emit.Struct{
					Name: innerName, Visibility: symbol.VisibilityProtected, Level: symbol.LevelType, Final: true,
				},
				want: "protected static final ",
			},
			{
				name: "writes sealed behind the access of a sealed interface",
				give: &emit.Interface{Name: "Shape", Sealed: true},
				want: "public sealed ",
			},
			{
				name: "writes the access alone for a member enum",
				give: &emit.Enum{Name: "Phase", Visibility: symbol.VisibilityPrivate},
				want: "private ",
			},
			{
				name:    "returns an error for an abstract final class",
				give:    &emit.Struct{Name: rowName, Abstract: true, Final: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a final sealed class",
				give:    &emit.Struct{Name: rowName, Final: true, Sealed: true},
				wantErr: true,
			},
			{
				name:    "returns an error for an internal scope",
				give:    &emit.Struct{Name: rowName, Visibility: symbol.VisibilityInternal},
				wantErr: true,
			},
			{
				name:    "returns an error for an interface's internal scope",
				give:    &emit.Interface{Name: "Shape", Visibility: symbol.VisibilityInternal},
				wantErr: true,
			},
			{name: "returns an error for an alias", give: &emit.Alias{Name: "Id"}, wantErr: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.TypeMods(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "javac rejects the keywords")
					return
				}
				assert.NoError(t, err, "the keywords spell")
				assert.Equal(t, got, tt.want, "in Java's stated order")
			})
		}
	})

	t.Run("MemberType", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    symbol.Symbol
			wantErr bool
		}{
			{name: "writes nothing for a member class of an unstated scope", give: &emit.Struct{Name: innerName}},
			{
				name: "writes nothing for a public member interface",
				give: &emit.Interface{Name: innerName, Visibility: symbol.VisibilityPublic},
			},
			{name: "writes nothing for a member enum of an unstated scope", give: &emit.Enum{Name: "Phase"}},
			{
				name: "writes nothing for a kind that is no member type",
				give: &emit.Alias{Name: "Id", Visibility: symbol.VisibilityPrivate},
			},
			{
				name:    "returns an error for a private member class",
				give:    &emit.Struct{Name: innerName, Visibility: symbol.VisibilityPrivate},
				wantErr: true,
			},
			{
				name:    "returns an error for a protected member interface",
				give:    &emit.Interface{Name: innerName, Visibility: symbol.VisibilityProtected},
				wantErr: true,
			},
			{
				name:    "returns an error for a package-scoped member enum",
				give:    &emit.Enum{Name: "Phase", Visibility: symbol.VisibilityPackage},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.MemberType(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "every member type of an interface is public")
					return
				}
				assert.NoError(t, err, "the member type passes")
				assert.Equal(t, got, "", "the helper writes nothing")
			})
		}
	})

	t.Run("ConstantMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for an interface constant", func(t *testing.T) {
			t.Parallel()

			got, err := backend.ConstantMods(&emit.Field{Name: "MAX", Type: ref("int"), Value: "8"})
			assert.NoError(t, err, "the constant spells")
			assert.Equal(t, got, "", "Java reads it as public static final")
		})

		tests := []struct {
			name string
			give *emit.Field
		}{
			{
				name: "returns an error for a field without an initializer",
				give: &emit.Field{Name: "MAX", Type: ref("int")},
			},
			{
				name: "returns an error for a private field",
				give: &emit.Field{Name: "MAX", Value: "8", Visibility: symbol.VisibilityPrivate},
			},
			{
				name: "returns an error for a mutable field",
				give: &emit.Field{Name: "MAX", Value: "8", Mutability: symbol.MutabilityMutable},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.ConstantMods(tt.give)
				assert.HasError(t, err, "an interface field is a public constant")
			})
		}
	})

	t.Run("FieldMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a field's keywords in Java's stated order", func(t *testing.T) {
			t.Parallel()

			key := &emit.Field{Name: "key", Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable}
			got, err := backend.FieldMods(key)
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "public static final ", "access, static, final")
		})

		t.Run("writes a stated access keyword", func(t *testing.T) {
			t.Parallel()

			got, err := backend.FieldMods(&emit.Field{Name: "key", Visibility: symbol.VisibilityProtected})
			assert.NoError(t, err, "the keyword spells")
			assert.Equal(t, got, "protected ", "the access alone")
		})

		t.Run("returns an error for an internal scope", func(t *testing.T) {
			t.Parallel()

			_, err := backend.FieldMods(&emit.Field{Name: idName, Visibility: symbol.VisibilityInternal})
			assert.HasError(t, err, "no access keyword spells the scope")
		})
	})

	t.Run("MethodMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes access before static", func(t *testing.T) {
			t.Parallel()

			load := &emit.Method{Name: "load", Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType}
			got, err := backend.MethodMods(load)
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "private static ", "Java's order")
		})

		t.Run("writes access before abstract", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MethodMods(&emit.Method{Name: "load", Abstract: true})
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "public abstract ", "a signature")
		})

		t.Run("writes access before final", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MethodMods(&emit.Method{Name: getName, Final: true})
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "public final ", "Java's order")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for an async method", give: &emit.Method{Name: "load", Async: true}},
			{name: "returns an error for a default body", give: &emit.Method{Name: "load", HasDefault: true}},
			{
				name: "returns an error for an abstract method with a body",
				give: &emit.Method{Name: "load", Abstract: true, Body: emit.Body{Verbatim: "return 1;"}},
			},
			{
				name: "returns an error for an internal scope",
				give: &emit.Method{Name: getName, Visibility: symbol.VisibilityInternal},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.MethodMods(tt.give)
				assert.HasError(t, err, "a class method cannot state it")
			})
		}
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a bare signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "load"})
			assert.NoError(t, err, "the signature passes")
			assert.Equal(t, got, "", "implicitly public")
		})

		t.Run("writes default for a method with a default body", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "load", HasDefault: true})
			assert.NoError(t, err, "the method spells")
			assert.Equal(t, got, "default ", "at instance level")
		})

		t.Run("writes static for a type-level body", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "make", HasDefault: true, Level: symbol.LevelType})
			assert.NoError(t, err, "the method spells")
			assert.Equal(t, got, "static ", "which excludes default")
		})

		t.Run("writes private for a private method with a body", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(
				&emit.Method{Name: getName, Visibility: symbol.VisibilityPrivate, HasDefault: true},
			)
			assert.NoError(t, err, "the method spells")
			assert.Equal(t, got, "private ", "javac rejects private beside default")
		})

		t.Run("writes private static for a private type-level method with a body", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{
				Name: getName, Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType, HasDefault: true,
			})
			assert.NoError(t, err, "the method spells")
			assert.Equal(t, got, "private static ", "access before static")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for an override marker", give: &emit.Method{Name: "load", Override: true}},
			{
				name: "returns an error for a body without a default",
				give: &emit.Method{Name: "load", Body: emit.Body{Verbatim: "return 1;"}},
			},
			{name: "returns an error for a final marker", give: &emit.Method{Name: getName, Final: true}},
			{name: "returns an error for an async method", give: &emit.Method{Name: getName, Async: true}},
			{
				name: "returns an error for a protected scope",
				give: &emit.Method{Name: getName, Visibility: symbol.VisibilityProtected},
			},
			{
				name: "returns an error for a private method without a body",
				give: &emit.Method{Name: getName, Visibility: symbol.VisibilityPrivate},
			},
			{
				name: "returns an error for a static method without a body",
				give: &emit.Method{Name: getName, Level: symbol.LevelType},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.SigMods(tt.give)
				assert.HasError(t, err, "an interface method cannot state it")
			})
		}
	})

	t.Run("Heritage", func(t *testing.T) {
		t.Parallel()

		heritage := func(t *testing.T, d symbol.Symbol) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.Heritage(d)
		}

		t.Run("writes a class's superclass behind extends", func(t *testing.T) {
			t.Parallel()

			got, err := heritage(t, &emit.Struct{Name: rowName, Extends: refs(baseName)})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Base", "one superclass")
		})

		t.Run("writes a class's contracts behind implements after its superclass", func(t *testing.T) {
			t.Parallel()

			got, err := heritage(t, &emit.Struct{
				Name: rowName, Extends: refs(baseName), Implements: refs("Keyed", "Closeable"),
			})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Base implements Keyed, Closeable", "comma-joined contracts")
		})

		t.Run("writes an interface's widened contracts behind extends", func(t *testing.T) {
			t.Parallel()

			got, err := heritage(t, &emit.Interface{Name: "Store", Extends: refs("Keyed", "Closeable")})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Keyed, Closeable", "comma-joined")
		})

		t.Run("writes a sealed type's subtypes last behind permits", func(t *testing.T) {
			t.Parallel()

			got, err := heritage(t, &emit.Interface{
				Name: "Shape", Sealed: true, Extends: refs("Figure"), Permits: refs("Circle", "Square"),
			})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Figure permits Circle, Square", "the permits clause last")
		})

		t.Run("writes a sealed class's permits clause alone", func(t *testing.T) {
			t.Parallel()

			got, err := heritage(t, &emit.Struct{Name: "Shape", Sealed: true, Permits: refs("Circle")})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " permits Circle", "no other clause")
		})

		t.Run("returns an error for a superclass that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: "Cache", Extends: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the superclass's refusal is the clause's")
		})

		t.Run("returns an error for a contract that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: "Cache", Implements: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the contract's refusal is the clause's")
		})

		t.Run("returns an error for a widened contract that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Interface{Name: "Store", Extends: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the widened contract's refusal is the clause's")
		})

		t.Run("returns an error for a permitted subtype that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			shape := &emit.Interface{Name: "Shape", Sealed: true, Permits: []*emit.TypeRef{unspellable(t, s)}}
			_, err := s.Heritage(shape)
			assert.HasError(t, err, "the subtype's refusal is the clause's")
		})

		t.Run("imports the classes a heritage names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			row := &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{imported(storePkg, baseName)}}
			_, err := s.Heritage(row)
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the superclass's import")
		})

		embedded := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "writes no clause for the embed of an interface",
				give: &emit.Interface{Name: "Store", Embeds: emit.SlotOf(&emit.Embed{Ref: ref(baseName)})},
			},
			{
				name: "writes no clause for the embed of a class",
				give: &emit.Struct{Name: rowName, Embeds: emit.SlotOf(&emit.Embed{Ref: ref(baseName)})},
			},
		}
		for _, tt := range embedded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := heritage(t, tt.give)
				assert.NoError(t, err, "the coverage refuses an embed, so the heritage spells")
				assert.Equal(t, got, "", "nothing promotes members")
			})
		}

		tests := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns an error for a second superclass",
				give: &emit.Struct{Name: rowName, Extends: refs("A", "B")},
			},
			{
				name: "returns an error for a sealed class without permits",
				give: &emit.Struct{Name: "Shape", Sealed: true},
			},
			{
				name: "returns an error for permits on a type that is not sealed",
				give: &emit.Interface{Name: "Shape", Permits: refs("Circle")},
			},
			{name: "returns an error for an enum", give: &emit.Enum{Name: "Phase"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := heritage(t, tt.give)
				assert.HasError(t, err, "Java cannot state the heritage")
			})
		}
	})

	t.Run("Throws", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no failure types", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Throws(nil)
			assert.NoError(t, err, "no clause spells")
			assert.Equal(t, got, "", "as nothing")
		})

		t.Run("returns an error for a failure type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Throws([]*emit.TypeRef{unspellable(t, s)})
			assert.HasError(t, err, "the failure type's refusal is the clause's")
		})

		t.Run("joins the failure types behind throws", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Throws(refs("IOException", "SQLException"))
			assert.NoError(t, err, "the clause spells")
			assert.Equal(t, got, " throws IOException, SQLException", "comma-joined")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Annotate(nil)
			assert.NoError(t, err, "no annotation refuses")
			assert.Equal(t, got, "", "no lines")
		})

		t.Run("writes one annotation line per annotation", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Annotate(symbol.Annotations{
				{Name: "Deprecated"},
				{Name: "SuppressWarnings", Args: []string{`"unchecked"`}},
			})
			assert.NoError(t, err, "Java spells both annotations")
			assert.Equal(t, got, "@Deprecated\n@SuppressWarnings(\"unchecked\")\n", "the arguments verbatim")
		})

		t.Run("returns an error for an assigned annotation", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Annotate(assignedAnnotations())
			assert.HasError(t, err, "Java passes the arguments of an annotation in parentheses alone")
		})
	})

	t.Run("MemberAnnotate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberAnnotate(nil)
			assert.NoError(t, err, "no annotation refuses")
			assert.Equal(t, got, "", "no lines")
		})

		t.Run("writes each annotation line behind the member indent", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberAnnotate(symbol.Annotations{
				{Name: "Deprecated"},
				{Name: "SuppressWarnings", Args: []string{`"unchecked"`}},
			})
			assert.NoError(t, err, "Java spells both annotations")
			assert.Equal(t, got, "    @Deprecated\n    @SuppressWarnings(\"unchecked\")\n", "the arguments verbatim")
		})

		t.Run("returns an error for an assigned annotation", func(t *testing.T) {
			t.Parallel()

			_, err := backend.MemberAnnotate(assignedAnnotations())
			assert.HasError(t, err, "Java passes the arguments of an annotation in parentheses alone")
		})
	})

	t.Run("NewSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a speller that imports into the given set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spelled(t, backend.NewSpeller(&set), imported(storePkg, rowName))
			assert.Equal(t, set.Paths(), []string{storePkg}, "the import is the set's")
		})
	})

	t.Run("EnumVariantName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a constant's name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.EnumVariantName(&emit.EnumVariant{Name: redConstant})
			assert.NoError(t, err, "a constant without a value spells")
			assert.Equal(t, got, redConstant, "the name alone")
		})

		t.Run("returns an error for a constant with a value", func(t *testing.T) {
			t.Parallel()

			_, err := backend.EnumVariantName(&emit.EnumVariant{Name: redConstant, Value: "1"})
			assert.HasError(t, err, "a valued constant takes the constructor form the templates do not spell")
		})
	})
}

// A keyword helper that writes one keyword or none, a guard and a name
// spelled as written allocate nothing, and every other helper allocates
// the text it writes. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestVocabularyAllocs(t *testing.T) {
	checkAllocs(t, vocabularyCalls())
}

// BenchmarkVocabulary measures each helper a kind template calls per
// declaration, and the vocabulary the render binds per file.
func BenchmarkVocabulary(b *testing.B) {
	benchCalls(b, vocabularyCalls())
}

// vocabularyCalls returns a call of every function and method of
// vocabulary.go.
func vocabularyCalls() []allocCall {
	s, set := speller()
	bare, row := ref(intType), imported(storePkg, rowName)
	params := []*emit.TypeParam{{Name: keyParam, Bounds: refs(comparableType)}, {Name: valueParam}}
	var list any = params
	args := new(emit.SlotOf(&emit.Param{Name: idName, Type: bare}, &emit.Param{Name: nameParam, Type: ref(stringType)}))
	result := new(emit.SlotOf(&emit.Return{Type: bare}))
	heir := &emit.Struct{Name: rowName, Extends: refs(baseName)}
	failures := refs(ioException)
	doc := []string{rowDoc}
	pkg := symbol.Identity{Package: storePkg}
	final := &emit.Struct{Name: rowName, Final: true}
	member := &emit.Struct{Name: innerName}
	constant := &emit.EnumVariant{Name: redConstant}
	staticFinal := &emit.Field{Name: idName, Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable}
	interfaceField := &emit.Field{Name: limitName, Value: limitValue}
	staticMethod := &emit.Method{Name: getName, Level: symbol.LevelType}
	defaultMethod := &emit.Method{Name: getName, HasDefault: true, Body: emit.Body{Verbatim: "return 1;"}}
	privateStatic := &emit.Method{
		Name: getName, Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType, HasDefault: true,
	}
	annotations := symbol.Annotations{{Name: overrideAnnotation}}
	wrapped := filledLine + " next"
	var (
		spellerOut backend.Speller
		out        string
		err        error
		funcs      template.FuncMap
		lines      []string
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "the helper spells")
			assert.Equal(tb, out, want, "the helper writes the Java spelling")
		}
	}
	return []allocCall{
		{
			name: "NewSpeller", call: func() { spellerOut = backend.NewSpeller(set) },
			check: func(tb assert.TB) { assert.Equal(tb, spellerOut, s, "NewSpeller returns the set's speller") },
		},
		{name: "Spell", caseName: "a builtin type", call: func() { out, err = s.Spell(bare) }, check: spells(intType)},
		{
			name:     "Spell",
			caseName: "an imported class",
			call:     func() { out, err = s.Spell(row) },
			check:    spells(rowName),
		},
		{
			name: "TypeParams", allocs: typeParamsAllocs,
			call: func() { out, err = s.TypeParams(params) }, check: spells("<K extends Comparable, V>"),
		},
		{
			name: "TypeArgs", allocs: typeArgsAllocs,
			call: func() { out, err = s.TypeArgs(list) }, check: spells("<K, V>"),
		},
		{
			name: "Params", allocs: paramsAllocs,
			call: func() { out, err = s.Params(args) }, check: spells("int id, String name"),
		},
		{name: "Results", call: func() { out, err = s.Results(result) }, check: spells(intType)},
		{
			name: "Heritage", allocs: clauseAllocs,
			call: func() { out, err = s.Heritage(heir) }, check: spells(" extends Base"),
		},
		{
			name: "Throws", allocs: clauseAllocs,
			call: func() { out, err = s.Throws(failures) }, check: spells(" throws IOException"),
		},
		{
			name: "Funcs", allocs: funcsAllocs,
			call:  func() { funcs = backend.Funcs(set) },
			check: func(tb assert.TB) { assert.Length(tb, funcs, 21, "Funcs returns the twenty-one helpers") },
		},
		{
			name: "Docs", allocs: textAllocs,
			call: func() { out = backend.Docs(doc) }, check: spells("/**\n * " + rowDoc + "\n */\n"),
		},
		{
			name: "WrapDocs", allocs: wrapDocsAllocs,
			call: func() { lines = backend.WrapDocs(wrapped) },
			check: func(tb assert.TB) {
				assert.Equal(tb, lines, []string{filledLine, "next"}, "WrapDocs wraps the text")
			},
		},
		{
			name: "MemberDocs", allocs: textAllocs,
			call: func() { out = backend.MemberDocs(doc) }, check: spells("    /**\n     * " + rowDoc + "\n     */\n"),
		},
		{
			name: "PackageClause", allocs: textAllocs,
			call: func() { out = backend.PackageClause(pkg) }, check: spells("package svc.store;\n\n"),
		},
		{
			name: "TypeMods", allocs: joinedKeywordAllocs,
			call: func() { out, err = backend.TypeMods(final) }, check: spells("public final "),
		},
		{name: "MemberType", call: func() { out, err = backend.MemberType(member) }, check: spells("")},
		{
			name: "EnumVariantName",
			call: func() { out, err = backend.EnumVariantName(constant) }, check: spells(redConstant),
		},
		{
			name: "FieldMods", allocs: twoJoinedKeywordAllocs,
			call: func() { out, err = backend.FieldMods(staticFinal) }, check: spells("public static final "),
		},
		{name: "ConstantMods", call: func() { out, err = backend.ConstantMods(interfaceField) }, check: spells("")},
		{
			name: "MethodMods", allocs: joinedKeywordAllocs,
			call: func() { out, err = backend.MethodMods(staticMethod) }, check: spells("public static "),
		},
		{
			name:     "SigMods",
			caseName: "a default method",
			call:     func() { out, err = backend.SigMods(defaultMethod) },
			check:    spells("default "),
		},
		{
			name: "SigMods", caseName: "a private static method", allocs: joinedKeywordAllocs,
			call: func() { out, err = backend.SigMods(privateStatic) }, check: spells("private static "),
		},
		{
			name: "Annotate", allocs: textAllocs,
			call: func() { out, err = backend.Annotate(annotations) }, check: spells("@Override\n"),
		},
		{
			name: "MemberAnnotate", allocs: textAllocs,
			call: func() { out, err = backend.MemberAnnotate(annotations) }, check: spells("    @Override\n"),
		},
	}
}

// assignedAnnotations returns one annotation that assigns its argument, as
// a Rust attribute does.
func assignedAnnotations() symbol.Annotations {
	return symbol.Annotations{{Name: "deprecated", Args: []string{`"use g"`}, Assigned: true}}
}

// ref returns an unresolved reference spelled s.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// refs returns an unresolved reference per spelling, in order.
func refs(spellings ...string) []*emit.TypeRef {
	out := make([]*emit.TypeRef, 0, len(spellings))
	for _, s := range spellings {
		out = append(out, ref(s))
	}
	return out
}

// imported returns a reference to a class of pkg, spelled s.
func imported(pkg, s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s, Package: pkg} }

// speller returns a speller over a fresh set, and the set.
func speller() (backend.Speller, *render.ImportSet) {
	set := &render.ImportSet{}
	return backend.NewSpeller(set), set
}

// spelled spells one reference and asserts it spells.
func spelled(tb assert.TB, s backend.Speller, t *emit.TypeRef) string {
	tb.Helper()

	out, err := s.Spell(t)
	assert.NoError(tb, err, "the reference spells")
	return out
}

// unspellable binds rowName of storePkg on s and returns an array of
// legacyPkg's rowName, which s refuses: the element is written
// qualified, and a composite's spelling does not follow from its
// structure.
func unspellable(tb assert.TB, s backend.Speller) *emit.TypeRef {
	tb.Helper()

	spelled(tb, s, imported(storePkg, rowName))
	return &emit.TypeRef{
		Form: symbol.FormArray, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(legacyPkg, rowName)},
	}
}
