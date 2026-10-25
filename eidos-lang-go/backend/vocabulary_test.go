// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"reflect"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling cases name packages, the names their imports bind, and
// the declarations the references name.
const (
	// storePkg is a package whose import binds storeName, and
	// legacyPkg a second package whose last element is storeName too.
	storePkg  = "example.com/store"
	legacyPkg = "example.com/legacy/store"
	storeName = "store"
	// storeName2 is the name legacyPkg binds beside storePkg.
	storeName2 = storeName + "2"
	// storeClause is the name that the package clause of storePkg
	// declares in [declaredNames].
	storeClause = "storage"
	// aliasName is the alias a source file imported storePkg under.
	aliasName = "st"
	// yamlPkg is a path whose last element is no Go identifier, and
	// yamlName the name it assumes.
	yamlPkg  = "gopkg.in/yaml.v3"
	yamlName = "yaml"
	// keywordPkg is a path whose last element is a Go keyword,
	// keywordClause the name its package clause declares, and
	// keywordName the escaped name a generated file imports it under.
	keywordPkg    = "example.com/shop/go"
	keywordClause = "golang"
	keywordName   = "go_"
	// timePkg and contextPkg are standard library packages.
	timePkg    = "time"
	contextPkg = "context"
	// rowName names a declaration of storePkg.
	rowName = "Row"
	// qualifiedRow is rowName qualified by the name storePkg binds.
	qualifiedRow = storeName + "." + rowName
	// protoLang is a language other than Go that a target can be in.
	protoLang symbol.Lang = "protobuf"
	// paramName is a type parameter of rowName.
	paramName = "T"
)

// The declarations and spellings the allocation cases name.
const (
	intType           = "int"
	comparableBound   = "comparable"
	contextName       = "ctx"
	contextType       = "Context"
	idName            = "id"
	readerName        = "Reader"
	rowDoc            = "Row is one record."
	generateDirective = "go:generate"
	jsonKey           = "json"
	dbKey             = "db"
	linuxConstraint   = "linux && amd64"
)

// generateArgs are the arguments of the generate directive the cases
// write.
var generateArgs = []string{"stringer", "-type=Colour"}

// filledLine is a word as wide as the documentation, so the wrap starts a
// line with the word after it.
var filledLine = strings.Repeat("a", textfmt.DocsWidth)

// The allocations of the vocabulary.
const (
	// qualifiedAllocs is a qualified name: the name behind its bound
	// qualifier.
	qualifiedAllocs = 1
	// typeParamsAllocs is two parameters: each parameter's spelling, the
	// joined list, and its brackets.
	typeParamsAllocs = 2 + 1 + 1
	// paramsAllocs is a context and an int: the qualified type, each
	// parameter's spelling, and the joined list.
	paramsAllocs = 1 + 2 + 1
	// partsAllocs is three ints: each parameter's spelling, the list of
	// parts, which Go places on the heap past two, and the joined list.
	partsAllocs = 3 + 1 + 1
	// resultsAllocs is a qualified row and an error: the qualified type,
	// the joined list, and its parentheses.
	resultsAllocs = 1 + 1 + 1
	// varTypeAllocs is a variable's type behind its space.
	varTypeAllocs = 1
	// typeArgsAllocs is the type arguments of two type parameters: the
	// text in one buffer of its size.
	typeArgsAllocs = 1
	// funcsAllocs is the vocabulary of a file: the map of nineteen
	// helpers in four allocations, and the seven bound helpers of the
	// speller, of which spell and spellvalue share one.
	funcsAllocs = 4 + 7
	// tagAllocs is the tag of a field, built on the stack and allocated
	// once.
	tagAllocs = 1
	// docsAllocs is a docblock, sized once.
	docsAllocs = 1
	// wrapDocsAllocs is the wrap of a single-spaced text: the list of its
	// lines, each of which is a part of the text.
	wrapDocsAllocs = 1
	// directivesAllocs is the directive lines, sized once.
	directivesAllocs = 1
	// constraintAllocs is the line of a build constraint with its blank
	// line.
	constraintAllocs = 1
)

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte, and every import a
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
			_, err := spell(imported(timePkg, timePkg, "Duration"))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the import is the file's")
		})

		t.Run("returns a spellvalue helper that spells a reference as spell does", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spell, is := backend.Funcs(&set)[backend.FuncSpellValue].(func(*emit.TypeRef) (string, error))
			assert.True(t, is, "the helper spells a reference")
			got, err := spell(imported(timePkg, timePkg, "Duration"))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, got, "time.Duration", "a Go value position spells a reference as a type position does")
		})
	})

	t.Run("TypeArgs", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the names of type parameters in brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.TypeArgs([]*emit.TypeParam{{Name: "K"}, {Name: "V"}})
			assert.NoError(t, err, "the parameters spell")
			assert.Equal(t, got, "[K, V]", "the arguments of a reference to the declaration")
		})

		t.Run("binds the import of a type reference", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			got, err := s.TypeArgs([]*emit.TypeRef{ref("int"), imported(timePkg, timePkg, "Duration")})
			assert.NoError(t, err, "the references spell")
			assert.Equal(t, got, "[int, time.Duration]", "the arguments qualified")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the package of the argument is imported")
		})

		t.Run("writes nothing for no arguments", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.TypeArgs([]*emit.TypeRef(nil))
			assert.NoError(t, err, "no arguments spell")
			assert.Equal(t, got, "", "as nothing")
		})

		t.Run("returns an error for a list of another type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeArgs([]string{"K"})
			assert.HasError(t, err, "a list of text has no type arguments")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs(nil), "", "no comment")
		})

		t.Run("writes one line comment per line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "// One.\n// Two.\n", "at the top level")
		})
	})

	t.Run("WrapDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("wraps the text at the width of the documentation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" next"), []string{filledLine, "next"},
				"the word past the width starts the next line")
		})

		t.Run("starts a line with a word that starts with @", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" @see next"), []string{filledLine, "@see next"},
				"a Go comment reads an unindented line as text")
		})
	})

	t.Run("MemberDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.MemberDocs(nil), "", "no comment")
		})

		t.Run("writes each line comment behind a tab", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.MemberDocs([]string{"One.", "Two."}), "\t// One.\n\t// Two.\n",
				"at the member's depth")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		substituted := []struct {
			name string
			give *emit.TypeRef
		}{
			{
				name: "returns an error for a channel without a spelling",
				give: &emit.TypeRef{Form: symbol.FormStream, Elems: []*emit.TypeRef{ref(rowName)}},
			},
			{
				name: "returns an error for a function type whose last list has no spelling",
				give: &emit.TypeRef{Form: symbol.FormFunc, Split: 1, Elems: []*emit.TypeRef{
					{Form: symbol.FormList, Elems: []*emit.TypeRef{ref(rowName)}},
				}},
			},
		}
		for _, tt := range substituted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Spell(tt.give)
				assert.HasError(t, err, "the substitution cleared the spelling that recorded the fact")
			})
		}

		t.Run("returns the spelling of a reference that names no package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref(rowName)), rowName, "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no import")
		})

		t.Run("returns any for a missing reference", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, nil), backend.Anonymous, "the empty interface")
		})

		t.Run("returns any for a reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{}), backend.Anonymous, "the empty interface")
		})

		t.Run("writes an argument list in brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{
				Spelling: "Map",
				Args: []*emit.TypeRef{
					ref("string"),
					{Spelling: "List", Args: []*emit.TypeRef{ref("User")}},
				},
			}), "Map[string, List[User]]", "the arguments recurse")
		})

		t.Run("binds the import of the package a reference names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, imported(timePkg, timePkg, "Duration")), "time.Duration",
				"the qualifier the import binds")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: timePkg, Name: timePkg}},
				"the import under the name it binds")
		})

		t.Run("qualifies through the name the package's path assumes", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, aliased()), qualifiedRow, "the source file's alias is replaced")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: storeName}},
				"the import binds the assumed name")
		})

		t.Run("qualifies through the name that the package declares", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetNames(declaredNames)
			assert.Equal(t, spelled(t, s, aliased()), storeClause+"."+rowName,
				"the package clause names the qualifier")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storePkg, Name: storeClause}},
				"the import binds the declared name")
		})

		t.Run("qualifies a path whose last element is no identifier through its assumed name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, imported(yamlName, yamlPkg, "Node")), yamlName+".Node",
				"the major version is dropped")
		})

		t.Run("qualifies a package whose path ends in a keyword through the escaped name", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: keywordClause + "." + rowName,
				Target: symbol.Identity{
					Lang: golang.Lang, Package: keywordPkg, Name: rowName, Kind: symbol.KindStruct,
				},
			}
			assert.Equal(t, spelled(t, s, target), keywordName+"."+rowName, "go is a keyword, so the qualifier is go_")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: keywordPkg, Name: keywordName}},
				"the import binds the escaped name")
		})

		t.Run("suffixes the qualifier of a second package that assumes the same name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storeName, storePkg, rowName))
			assert.Equal(t, spelled(t, s, imported(storeName, legacyPkg, rowName)), storeName2+"."+rowName,
				"the first package keeps the plain name")
		})

		t.Run("writes a reference to the file's own package bare", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(storePkg)
			assert.Equal(t, spelled(t, s, aliased()), rowName, "a package never qualifies its own names")
			assert.Equal(t, set.Len(), 0, "a package never imports itself")
		})

		t.Run("qualifies a Go target through its package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName,
				Target:   symbol.Identity{Lang: golang.Lang, Package: storePkg, Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, target), qualifiedRow, "a declaration of another package is qualified")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the package is imported")
		})

		t.Run("writes a reference to a type parameter of another package bare", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			param := &emit.TypeRef{
				Spelling: paramName,
				Target: symbol.Identity{
					Lang: golang.Lang, Package: storePkg, Owner: rowName, Name: paramName, Kind: symbol.KindTypeParam,
				},
			}
			assert.Equal(t, spelled(t, s, param), paramName, "no package exports a type parameter")
			assert.Equal(t, set.Len(), 0, "a type parameter has no import")
		})

		t.Run("qualifies a target of another language through its recorded package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName, Package: storePkg,
				Target: symbol.Identity{Lang: protoLang, Package: "svc", Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, target), qualifiedRow, "the referent's file is in another package")
			assert.Equal(t, set.Paths(), []string{storePkg}, "the package of the referent's file is imported")
		})

		t.Run("writes a target of another language without a recorded package bare", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName,
				Target:   symbol.Identity{Lang: protoLang, Package: "svc", Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, target), rowName, "the referent is in the file's own package")
			assert.Equal(t, set.Len(), 0, "no import of another language's package")
		})

		t.Run("writes the arguments of a qualified instantiation", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			box := imported(aliasName, storePkg, "Box")
			box.Args = []*emit.TypeRef{ref("int")}
			assert.Equal(t, spelled(t, s, box), storeName+".Box[int]", "the qualifier, then the arguments")
		})

		t.Run("returns an error for a qualified reference that records no package", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(ref(aliasName + "." + rowName))
			assert.HasError(t, err, "the import cannot be named")
			assert.Contains(t, err.Error(), "does not record a package", "the error names what is missing")
		})

		t.Run("returns an error for a spelling that is not a Go type name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(imported("google.protobuf", "google/protobuf/timestamp.proto", "Timestamp"))
			assert.HasError(t, err, "the qualified name of another language is not a Go reference")
			assert.Contains(t, err.Error(), "not a Go type name", "the error names the defect")
		})

		t.Run("keeps a composite's written spelling where every child spells as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			ctxRef := imported(contextPkg, contextPkg, "Context")
			fn := fnType("func(ctx context.Context) error", 1, ctxRef, ref("error"))
			assert.Equal(t, spelled(t, s, fn), "func(ctx context.Context) error", "the parameter name is kept")
			assert.Equal(t, set.Paths(), []string{contextPkg}, "the child's import is bound")
		})

		tests := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "restates a pointer whose element spells otherwise",
				give: composite(symbol.FormOptional, "*st.Row", aliased()),
				want: "*" + qualifiedRow,
			},
			{
				name: "restates a slice whose element spells otherwise",
				give: composite(symbol.FormList, "[]st.Row", aliased()),
				want: "[]" + qualifiedRow,
			},
			{
				name: "restates a variadic parameter whose element spells otherwise",
				give: composite(symbol.FormList, "...st.Row", aliased()),
				want: "..." + qualifiedRow,
			},
			{
				name: "restates a sized array whose element spells otherwise",
				give: sizedArray(),
				want: "[4]" + qualifiedRow,
			},
			{
				name: "restates a map whose value spells otherwise",
				give: composite(symbol.FormMap, "map[string]st.Row", ref("string"), aliased()),
				want: "map[string]" + qualifiedRow,
			},
			{
				name: "restates a receive-only channel whose element spells otherwise",
				give: composite(symbol.FormStream, "<-chan st.Row", aliased()),
				want: "<-chan " + qualifiedRow,
			},
			{
				name: "restates a send-only channel whose element spells otherwise",
				give: composite(symbol.FormStream, "chan <- st.Row", aliased()),
				want: "chan<- " + qualifiedRow,
			},
			{
				name: "restates a bidirectional channel whose element spells otherwise",
				give: composite(symbol.FormStream, "chan st.Row", aliased()),
				want: "chan " + qualifiedRow,
			},
			{
				name: "restates a function type of several results without its parameter names",
				give: fnType("func(r st.Row) (st.Row, error)", 1, aliased(), aliased(), ref("error")),
				want: "func(" + qualifiedRow + ") (" + qualifiedRow + ", error)",
			},
			{
				name: "restates a function type of one result bare",
				give: fnType("func() st.Row", 0, aliased()),
				want: "func() " + qualifiedRow,
			},
			{
				name: "restates a function type of no result without a result list",
				give: fnType("func(r st.Row)", 1, aliased()),
				want: "func(" + qualifiedRow + ")",
			},
			{
				name: "restates a composite inside a composite",
				give: composite(symbol.FormList, "[]*st.Row", composite(symbol.FormOptional, "*st.Row", aliased())),
				want: "[]*" + qualifiedRow,
			},
			{
				name: "restates a slice whose element's type argument spells otherwise",
				give: composite(symbol.FormList, "[]Box[st.Row]",
					&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{aliased()}}),
				want: "[]Box[" + qualifiedRow + "]",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				assert.Equal(t, spelled(t, s, tt.give), tt.want, "restated from the structure")
			})
		}

		t.Run("returns an error restating an array whose length is an expression", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormArray, "[N]st.Row", aliased()))
			assert.HasError(t, err, "the structure states no length")
			assert.Contains(t, err.Error(), "expression", "naming the length")
		})

		t.Run("returns an error restating a form Go does not write", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormTuple, "(st.Row, int)", aliased(), ref("int")))
			assert.HasError(t, err, "Go has no tuple type")
		})

		t.Run("returns an error restating a composite missing a child", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormMap, "map[st.Row]", aliased()))
			assert.HasError(t, err, "a map takes a key and a value")
		})

		t.Run("returns an error for an inline body without children that names another package", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormInline, "struct{ When time.Time }"))
			assert.HasError(t, err, "the body's import is not recorded")
			assert.Contains(t, err.Error(), "inline body", "naming the form")
		})

		nested := []struct {
			name string
			give *emit.TypeRef
		}{
			{
				name: "returns an error for a slice of an inline body without children that names another package",
				give: composite(symbol.FormList, "[]struct{ When time.Time }",
					composite(symbol.FormInline, "struct{ When time.Time }")),
			},
			{
				name: "returns an error for an argument that is an inline body without children",
				give: &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{
					composite(symbol.FormInline, "struct{ When time.Time }"),
				}},
			},
		}
		for _, tt := range nested {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Spell(tt.give)
				assert.HasError(t, err, "the body's import is not recorded")
				assert.Contains(t, err.Error(), "inline body", "naming the form")
			})
		}

		t.Run("binds the import of each child of an inline body", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			body := composite(symbol.FormInline, "struct{ When time.Time }", imported(timePkg, timePkg, "Time"))
			assert.Equal(t, spelled(t, s, body), "struct{ When time.Time }", "the body as written")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the import of the child")
		})

		t.Run("returns an error for an inline body whose child spells otherwise", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormInline, "struct{ R st.Row }", aliased()))
			assert.HasError(t, err, "the body keeps the qualifier of its source")
			assert.Contains(t, err.Error(), "inline body", "naming the form")
		})

		t.Run("returns an error for an inline body with a child of another package", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(svcPkg)
			target := symbol.Identity{Lang: golang.Lang, Package: storePkg, Name: rowName, Kind: symbol.KindStruct}
			row := &emit.TypeRef{Spelling: rowName, Target: target}
			_, err := s.Spell(composite(symbol.FormInline, "struct{ R Row }", row))
			assert.HasError(t, err, "the file is in another package than the declaration")
		})

		t.Run("keeps the spelling of a union whose terms spell as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			union := composite(symbol.FormUnion, "~int64 | time.Duration",
				ref("int64"), imported(timePkg, timePkg, "Duration"))
			assert.Equal(t, spelled(t, s, union), "~int64 | time.Duration", "the union as written")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the import of the qualified term")
		})

		unions := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "restates a union whose term spells otherwise with the approximation mark of each term",
				give: composite(symbol.FormUnion, "~int | st.Row", ref("int"), aliased()),
				want: "~int | " + qualifiedRow,
			},
			{
				name: "restates the term of an approximation alone",
				give: composite(symbol.FormUnion, "~st.Row", aliased()),
				want: "~" + qualifiedRow,
			},
			{
				name: "restates a term in parentheses as one term",
				give: composite(symbol.FormUnion, "(st.Row) | ~int", aliased(), ref("int")),
				want: qualifiedRow + " | ~int",
			},
		}
		for _, tt := range unions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				assert.Equal(t, spelled(t, s, tt.give), tt.want, "restated from the terms")
			})
		}

		brokenUnions := []struct {
			name string
			give *emit.TypeRef
		}{
			{
				name: "returns an error restating a union without a spelling",
				give: composite(symbol.FormUnion, "", ref("int"), aliased()),
			},
			{
				name: "returns an error restating a union whose spelling does not parse",
				give: composite(symbol.FormUnion, "~int |", ref("int"), aliased()),
			},
			{
				name: "returns an error restating a union whose spelling has another number of terms",
				give: composite(symbol.FormUnion, "~st.Row", ref("int"), aliased()),
			},
		}
		for _, tt := range brokenUnions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Spell(tt.give)
				assert.HasError(t, err, "the spelling of the union does not record the marks of its terms")
			})
		}

		imports := []struct {
			name string
			home string
			give *emit.TypeRef
			want string
		}{
			{
				name: "returns an error for a package whose path is not an import path",
				give: imported(storeName, "example.com/bad store", rowName),
				want: "malformed import path",
			},
			{
				name: "returns an error for a package of an internal tree that does not contain the file",
				home: "example.com/shop/web",
				give: imported(storeName, "example.com/shop/core/internal/store", rowName),
				want: "internal",
			},
			{
				name: "returns an error for a package of the internal tree of the standard library",
				home: svcPkg,
				give: imported("abi", "internal/abi", "Type"),
				want: "standard library",
			},
			{
				name: "returns an error for an internal package of another tree",
				home: "example.com/shop/web",
				give: imported("internal", "example.com/shop/core/internal", rowName),
				want: "internal to example.com/shop/core",
			},
			{
				name: "returns an error for a declaration of an external test package",
				home: "example.com/shop/web",
				give: imported(storeName, storePkg+golang.TestSuffix, rowName),
				want: "external test package",
			},
		}
		for _, tt := range imports {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, set := speller()
				set.SetHome(tt.home)
				_, err := s.Spell(tt.give)
				assert.HasError(t, err, "the go command refuses the import")
				assert.Contains(t, err.Error(), tt.want, "naming the rule")
			})
		}

		internals := []struct {
			name string
			home string
		}{
			{name: "binds a package of an internal tree from the parent of the tree", home: "example.com/shop/core"},
			{
				name: "binds a package of an internal tree from a package below its parent",
				home: "example.com/shop/core/api",
			},
			{
				name: "binds a package of an internal tree from the external tests of its parent",
				home: "example.com/shop/core_test",
			},
		}
		for _, tt := range internals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, set := speller()
				set.SetHome(tt.home)
				assert.Equal(t, spelled(t, s, imported(storeName, "example.com/shop/core/internal/store", rowName)),
					qualifiedRow, "the tree contains the file")
			})
		}

		t.Run("writes an inline body that names no other package as written", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, composite(symbol.FormInline, "struct{ N int }")), "struct{ N int }",
				"the body as written")
		})

		t.Run("writes an inline body that does not parse as written", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, composite(symbol.FormInline, "struct{")), "struct{",
				"the formatter reports the body")
		})

		t.Run("writes any for an argument that states no type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{nil}}), "Box[any]",
				"the empty interface in the argument's place")
		})

		t.Run("returns an error for an argument that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{ref("x.Y")}})
			assert.HasError(t, err, "the argument records no package")
		})

		t.Run("returns an error for a composite whose child does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(composite(symbol.FormOptional, "*x.Y", ref("x.Y")))
			assert.HasError(t, err, "the child records no package")
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

		t.Run("writes an unbounded parameter under any", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "T"})
			assert.NoError(t, err, "an unbounded parameter spells")
			assert.Equal(t, got, "[T any]", "under the empty interface")
		})

		t.Run("writes one bound alone", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "K", Bounds: []*emit.TypeRef{ref("comparable")}})
			assert.NoError(t, err, "a bounded parameter spells")
			assert.Equal(t, got, "[K comparable]", "the bound behind the name")
		})

		t.Run("folds several bounds into a constraint interface", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t,
				&emit.TypeParam{Name: "V", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}})
			assert.NoError(t, err, "a parameter of several bounds spells")
			assert.Equal(t, got, "[V interface{ Codec; Closer }]", "Go's intersection")
		})

		t.Run("separates parameters with commas", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "K"}, &emit.TypeParam{Name: "V"})
			assert.NoError(t, err, "the parameters spell")
			assert.Equal(t, got, "[K any, V any]", "in declaration order")
		})

		t.Run("binds the import a bound names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			got, err := s.TypeParams([]*emit.TypeParam{{
				Name: "T", Bounds: []*emit.TypeRef{imported("fmt", "fmt", "Stringer")},
			}})
			assert.NoError(t, err, "the bound spells")
			assert.Equal(t, got, "[T fmt.Stringer]", "qualified")
			assert.Equal(t, set.Paths(), []string{"fmt"}, "the bound's package is imported")
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
			{name: "returns an error for a default", give: &emit.TypeParam{Name: "T", Default: ref("string")}},
			{
				name: "returns an error for a bound that does not spell",
				give: &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{aliased(), ref("x.Y")}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := typeParams(t, tt.give)
				assert.HasError(t, err, "Go's type parameters state none of it")
			})
		}
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		params := func(t *testing.T, ps ...*emit.Param) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Params(new(emit.SlotOf(ps...)))
			assert.NoError(t, err, "the parameters spell")
			return got
		}

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t), "", "no list")
		})

		t.Run("writes each parameter's name before its type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t,
				&emit.Param{Name: "ctx", Type: imported(contextPkg, contextPkg, "Context")},
				&emit.Param{Name: "id", Type: ref("int")},
			), "ctx context.Context, id int", "comma-separated")
		})

		t.Run("writes a variadic parameter's type behind three dots", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{
				Name: "keys", Type: ref("string"), Variadic: symbol.VariadicPositional,
			}), "keys ...string", "the variadic marker")
		})

		t.Run("writes an unnamed parameter as its type alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Type: ref("int")}), "int", "no name")
		})

		t.Run("writes a trailing comment as a block comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Name: "id", Type: ref("int"), Comment: "the key"}),
				"id int /* the key */", "the comment form gofmt keeps inside a list")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params(new(emit.SlotOf(&emit.Param{Name: "id", Type: ref("x.Y")})))
			assert.HasError(t, err, "the list is refused")
		})

		t.Run("returns an error for a list in which some parameters have names", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			named, unnamed := &emit.Param{Name: "id", Type: ref("int")}, &emit.Param{Type: ref("string")}
			_, err := s.Params(new(emit.SlotOf(named, unnamed)))
			assert.HasError(t, err, "Go names every parameter or none")
			assert.Contains(t, err.Error(), "1 of its 2", "counting the names")
		})

		t.Run("writes a list in which no parameter has a name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, params(t, &emit.Param{Type: ref("int")}, &emit.Param{Type: ref("string")}), "int, string",
				"the types alone")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		results := func(t *testing.T, rs ...*emit.Return) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Results(new(emit.SlotOf(rs...)))
			assert.NoError(t, err, "the results spell")
			return got
		}

		t.Run("returns nothing for no results", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t), "", "no list")
		})

		t.Run("writes one unnamed result bare", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref("error")}), " error", "without parentheses")
		})

		t.Run("parenthesises several results", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref(rowName)}, &emit.Return{Type: ref("error")}),
				" (Row, error)", "comma-separated in parentheses")
		})

		t.Run("parenthesises one named result", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Name: "n", Type: ref("int")}), " (n int)",
				"because Go requires it")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results(new(emit.SlotOf(&emit.Return{Type: ref("x.Y")})))
			assert.HasError(t, err, "the list is refused")
		})

		t.Run("returns an error for a list in which some results have names", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			named, unnamed := &emit.Return{Name: "n", Type: ref("int")}, &emit.Return{Type: ref("error")}
			_, err := s.Results(new(emit.SlotOf(named, unnamed)))
			assert.HasError(t, err, "Go names every result or none")
			assert.Contains(t, err.Error(), "1 of its 2", "counting the names")
		})
	})

	t.Run("Receiver", func(t *testing.T) {
		t.Parallel()

		receiver := func(t *testing.T, m *emit.Method) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Receiver(m)
			assert.NoError(t, err, "the receiver spells")
			return got
		}

		t.Run("writes a declared receiver's name before its type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{Receiver: &emit.Param{Name: "s", Type: ref("*Store")}}),
				"s *Store", "as a parameter")
		})

		t.Run("writes the type alone for a method attached from outside", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{Receives: ref("*Store")}), "*Store", "no name")
		})

		t.Run("returns nothing for a method without a receiver", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, receiver(t, &emit.Method{}), "", "no spelling")
		})
	})

	t.Run("VarType", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a stated type behind a space", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.VarType(&emit.Variable{Name: "count", Type: ref("int")})
			assert.NoError(t, err, "a typed variable spells")
			assert.Equal(t, got, " int", "its type behind the space")
		})

		t.Run("returns nothing for a variable with an initializer alone", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.VarType(&emit.Variable{Name: "count", Value: "8"})
			assert.NoError(t, err, "an initialized variable spells")
			assert.Equal(t, got, "", "Go infers the type")
		})

		t.Run("returns an error for a variable stating neither", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.VarType(&emit.Variable{Name: "count"})
			assert.HasError(t, err, "var x alone declares nothing Go accepts")
		})

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.VarType(&emit.Variable{Name: "count", Type: ref("x.Y")})
			assert.HasError(t, err, "the slot is refused")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identity's own name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Package(symbol.Identity{Name: storeName, Package: yamlPkg})
			assert.NoError(t, err, "a named package spells")
			assert.Equal(t, got, storeName, "the declared name")
		})

		t.Run("returns the name the path assumes for an identity without a name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Package(symbol.Identity{Package: yamlPkg})
			assert.NoError(t, err, "a package path spells")
			assert.Equal(t, got, yamlName, "the major version is dropped")
		})

		t.Run("returns the escaped name of a path that ends in a keyword for an identity without a name",
			func(t *testing.T) {
				t.Parallel()

				got, err := backend.Package(symbol.Identity{Package: keywordPkg})
				assert.NoError(t, err, "a package path spells")
				assert.Equal(t, got, keywordName, "go is a keyword, so the package declares go_")
			})

		t.Run("returns an error for an identity naming no package", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Package(symbol.Identity{})
			assert.HasError(t, err, "every Go file declares a package")
			assert.Contains(t, err.Error(), "import base", "the error names what the plan can state")
		})
	})

	t.Run("Guard", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    symbol.Symbol
			wantErr bool
		}{
			{name: "writes nothing for a final struct", give: &emit.Struct{Name: rowName, Final: true}},
			{
				name: "writes nothing for a package-scoped variable",
				give: &emit.Variable{Name: "count", Visibility: symbol.VisibilityPackage},
			},
			{
				name: "writes nothing for a method attached to a type",
				give: &emit.Method{Name: "Close", Receives: ref(rowName)},
			},
			{
				name: "writes nothing for an annotated constant",
				give: &emit.Constant{Name: "Max", Value: "8", Annotations: symbol.Annotations{{Name: "nolint"}}},
			},
			{
				name: "writes nothing for a struct stating supertypes",
				give: &emit.Struct{
					Name:       rowName,
					Extends:    []*emit.TypeRef{ref("Base")},
					Implements: []*emit.TypeRef{ref("Keyed")},
				},
			},
			{
				name:    "returns an error for an abstract struct",
				give:    &emit.Struct{Name: rowName, Abstract: true},
				wantErr: true,
			},
			{
				name:    "returns an error for an async function",
				give:    &emit.Function{Name: "Load", Async: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a static method",
				give:    &emit.Method{Name: "Load", Level: symbol.LevelType},
				wantErr: true,
			},
			{
				name:    "returns an error for an override marker",
				give:    &emit.Method{Name: "Load", Override: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a method naming no receiver type",
				give:    &emit.Method{Name: "Close"},
				wantErr: true,
			},
			{
				name:    "returns an error for a field initializer",
				give:    &emit.Field{Name: "Key", Value: "1"},
				wantErr: true,
			},
			{
				name:    "returns an error for a field's own mutability",
				give:    &emit.Field{Name: "Key", Mutability: symbol.MutabilityImmutable},
				wantErr: true,
			},
			{
				name:    "returns an error for an immutable variable",
				give:    &emit.Variable{Name: "count", Mutability: symbol.MutabilityImmutable},
				wantErr: true,
			},
			{
				name:    "returns an error for a constant without a value",
				give:    &emit.Constant{Name: "Max"},
				wantErr: true,
			},
			{
				name:    "returns an error for a visibility no case expresses",
				give:    &emit.Alias{Name: "ID", Visibility: symbol.VisibilityProtected},
				wantErr: true,
			},
			{
				name:    "returns an error for an async method",
				give:    &emit.Method{Name: loadName, Receives: ref(rowName), Async: true},
				wantErr: true,
			},
			{
				name:    "returns an error for an abstract method",
				give:    &emit.Method{Name: loadName, Receives: ref(rowName), Abstract: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a method's default body",
				give:    &emit.Method{Name: loadName, Receives: ref(rowName), HasDefault: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a type-level field",
				give:    &emit.Field{Name: idName, Level: symbol.LevelType},
				wantErr: true,
			},
			{name: "writes nothing for a sum", give: &emit.Sum{Name: shapeName}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Guard(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "Go states it nowhere")
					return
				}
				assert.NoError(t, err, "the declaration passes")
				assert.Equal(t, got, "", "the guard writes nothing")
			})
		}
	})

	t.Run("EmbedGuard", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing for an embed with a comment", func(t *testing.T) {
			t.Parallel()

			got, err := backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Comment: "stream side"})
			assert.NoError(t, err, "the embed passes")
			assert.Equal(t, got, "", "the guard writes nothing")
		})

		t.Run("returns an error for a tagged embed", func(t *testing.T) {
			t.Parallel()

			_, err := backend.EmbedGuard(&emit.Embed{Ref: ref("io.Reader"), Tag: `json:"r"`})
			assert.HasError(t, err, "Go gives a struct field alone a tag")
		})
	})

	t.Run("Tag", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a field without tag entries", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Tag(&emit.Field{Name: rowName})
			assert.NoError(t, err, "the field has no tag")
			assert.Equal(t, got, "", "the helper writes nothing")
		})

		t.Run("writes one entry as a key and a quoted value in backquotes", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Tag(&emit.Field{
				Name: idName, Tags: emit.SlotOf(&emit.TagEntry{Key: jsonKey, Value: idName}),
			})
			assert.NoError(t, err, "the entry is valid")
			assert.Equal(t, got, " `json:\"id\"`", "the tag follows the type after a space")
		})

		t.Run("separates two entries with a space", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Tag(&emit.Field{Name: idName, Tags: emit.SlotOf(
				&emit.TagEntry{Key: jsonKey, Value: idName + ",omitempty"},
				&emit.TagEntry{Key: dbKey, Value: idName},
			)})
			assert.NoError(t, err, "the entries are valid")
			assert.Equal(t, got, " `json:\"id,omitempty\" db:\"id\"`", "the entries keep their order")
		})

		t.Run("writes a tag from which reflect.StructTag reads each value back", func(t *testing.T) {
			t.Parallel()

			quoted, tabbed, unicode := `a"b\c`, "tab\there", "ünïcode"
			got, err := backend.Tag(&emit.Field{Name: idName, Tags: emit.SlotOf(
				&emit.TagEntry{Key: jsonKey, Value: quoted},
				&emit.TagEntry{Key: dbKey, Value: tabbed},
				&emit.TagEntry{Key: yamlName, Value: unicode},
			)})
			assert.NoError(t, err, "the entries are valid")
			assert.That(t, got).
				HasPrefix(" `", "the tag opens with a space and a backquote").
				HasSuffix("`", "the tag closes with a backquote")
			tag := reflect.StructTag(got[2 : len(got)-1])
			expect.Equal(t, tag.Get(jsonKey), quoted, "the quote and the backslash read back")
			expect.Equal(t, tag.Get(dbKey), tabbed, "the tab reads back")
			expect.Equal(t, tag.Get(yamlName), unicode, "the letters outside ASCII read back")
		})

		refusals := []struct {
			name string
			give *emit.TagEntry
		}{
			{name: "returns an error for an empty key", give: &emit.TagEntry{Value: idName}},
			{name: "returns an error for a key with a space", give: &emit.TagEntry{Key: "a b", Value: idName}},
			{name: "returns an error for a key with a colon", give: &emit.TagEntry{Key: "a:b", Value: idName}},
			{name: "returns an error for a key with a quote", give: &emit.TagEntry{Key: `a"b`, Value: idName}},
			{name: "returns an error for a key with a backquote", give: &emit.TagEntry{Key: "a`b", Value: idName}},
			{
				name: "returns an error for a key with a control character",
				give: &emit.TagEntry{Key: "a\x7fb", Value: idName},
			},
			{name: "returns an error for a value with a backquote", give: &emit.TagEntry{Key: jsonKey, Value: "a`b"}},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Tag(&emit.Field{Name: idName, Tags: emit.SlotOf(tt.give)})
				assert.HasError(t, err, "a raw string literal or reflect.StructTag cannot contain the entry")
			})
		}
	})

	assigned := symbol.Annotations{{Name: "deprecated", Args: []string{`"use g"`}, Assigned: true}}

	t.Run("Directives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Directives(nil)
			assert.NoError(t, err, "no directive refuses")
			assert.Equal(t, got, "", "no lines")
		})

		t.Run("writes one directive comment per annotation", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Directives(symbol.Annotations{
				{Name: "go:embed", Args: []string{"schema.sql"}},
				{Name: "nolint", Args: []string{"errcheck"}},
			})
			assert.NoError(t, err, "Go spells both directives")
			assert.Equal(t, got, "//go:embed schema.sql\n//nolint errcheck\n",
				"the arguments space-joined behind the name")
		})

		t.Run("writes every argument space-joined behind the name", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Directives(symbol.Annotations{{Name: generateDirective, Args: generateArgs}})
			assert.NoError(t, err, "Go spells the directive")
			assert.Equal(t, got, "//go:generate stringer -type=Colour\n", "two arguments in order")
		})

		t.Run("returns an error for an assigned annotation", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Directives(assigned)
			assert.HasError(t, err, "a Go directive writes its arguments after its name alone")
		})
	})

	t.Run("MemberDirectives", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberDirectives(nil)
			assert.NoError(t, err, "no directive refuses")
			assert.Equal(t, got, "", "no lines")
		})

		t.Run("writes each directive comment behind a tab", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberDirectives(symbol.Annotations{{Name: "go:fix"}, {Name: "nolint"}})
			assert.NoError(t, err, "Go spells both directives")
			assert.Equal(t, got, "\t//go:fix\n\t//nolint\n", "at the member's depth")
		})

		t.Run("returns an error for an assigned annotation", func(t *testing.T) {
			t.Parallel()

			_, err := backend.MemberDirectives(assigned)
			assert.HasError(t, err, "a Go directive writes its arguments after its name alone")
		})
	})

	t.Run("Constraint", func(t *testing.T) {
		t.Parallel()

		constraint := symbol.Annotation{Name: golang.BuildAnnotation, Args: []string{linuxConstraint}}
		generate := symbol.Annotation{Name: generateDirective, Args: generateArgs}
		tests := []struct {
			name string
			give symbol.Annotations
			want string
		}{
			{name: "returns nothing for no annotations"},
			{name: "returns nothing for annotations without a build constraint", give: symbol.Annotations{generate}},
			{
				name: "writes the build constraint as a go:build line and a blank line",
				give: symbol.Annotations{constraint},
				want: "//go:build " + linuxConstraint + "\n\n",
			},
			{
				name: "writes the build constraint without the other annotations",
				give: symbol.Annotations{generate, constraint},
				want: "//go:build " + linuxConstraint + "\n\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Constraint(tt.give), tt.want, "the line that the go command reads")
			})
		}
	})

	t.Run("NewSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a speller that binds into the given set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spelled(t, backend.NewSpeller(&set), imported(timePkg, timePkg, "Duration"))
			assert.Equal(t, set.Paths(), []string{timePkg}, "the import is the set's")
		})
	})

	t.Run("SigGuard", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    *emit.Method
			wantErr bool
		}{
			{name: "writes nothing for an abstract signature", give: &emit.Method{Name: "Get", Abstract: true}},
			{name: "writes nothing for a final signature", give: &emit.Method{Name: "Get", Final: true}},
			{
				name:    "returns an error for a default body",
				give:    &emit.Method{Name: "Get", HasDefault: true},
				wantErr: true,
			},
			{
				name:    "returns an error for type parameters",
				give:    &emit.Method{Name: "Map", TypeParams: []*emit.TypeParam{{Name: "T"}}},
				wantErr: true,
			},
			{
				name:    "returns an error for an async signature",
				give:    &emit.Method{Name: "Get", Async: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a body",
				give:    &emit.Method{Name: "Get", Body: emit.Body{Verbatim: "return nil"}},
				wantErr: true,
			},
			{
				name:    "returns an error for an override marker",
				give:    &emit.Method{Name: "Get", Override: true},
				wantErr: true,
			},
			{
				name:    "returns an error for a static method",
				give:    &emit.Method{Name: "Get", Level: symbol.LevelType},
				wantErr: true,
			},
			{
				name:    "returns an error for annotations",
				give:    &emit.Method{Name: "Get", Annotations: symbol.Annotations{{Name: "nolint"}}},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.SigGuard(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "an interface method states it nowhere")
					return
				}
				assert.NoError(t, err, "the signature passes")
				assert.Equal(t, got, "", "the guard writes nothing")
			})
		}
	})
}

// A guard and a bare name allocate nothing, and a spelling allocates
// the text it writes. The ordinary run, which runs no benchmark,
// checks those ceilings here.
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
	named, namedSet := speller()
	namedSet.SetNames(declaredNames)
	bare, qualified := ref(intType), imported(storeName, storePkg, rowName)
	doc := []string{rowDoc}
	params := []*emit.TypeParam{{Name: keyParam, Bounds: []*emit.TypeRef{ref(comparableBound)}}, {Name: valueParam}}
	var list any = params
	args := new(emit.SlotOf(
		&emit.Param{Name: contextName, Type: imported(contextPkg, contextPkg, contextType)},
		&emit.Param{Name: idName, Type: ref(intType)},
	))
	three := new(emit.SlotOf(
		&emit.Param{Name: idName, Type: bare}, &emit.Param{Name: countName, Type: bare},
		&emit.Param{Name: contextName, Type: bare},
	))
	results := new(emit.SlotOf(&emit.Return{Type: qualified}, &emit.Return{Type: ref(errorType)}))
	method := &emit.Method{Name: getName, Receives: ref(rowName)}
	variable := &emit.Variable{Name: countName, Type: ref(intType)}
	row := &emit.Struct{Name: rowName}
	signature := &emit.Method{Name: getName, Abstract: true}
	embed := &emit.Embed{Ref: ref(readerName)}
	untagged := &emit.Field{Name: idName}
	field := &emit.Field{Name: idName, Tags: emit.SlotOf(&emit.TagEntry{Key: jsonKey, Value: idName})}
	annotations := symbol.Annotations{{Name: generateDirective, Args: generateArgs}}
	constrained := symbol.Annotations{{Name: golang.BuildAnnotation, Args: []string{linuxConstraint}}}
	pkg := symbol.Identity{Package: storePkg, Name: storeName}
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
			assert.Equal(tb, out, want, "the helper writes the Go spelling")
		}
	}
	return []allocCall{
		{
			name: "NewSpeller", call: func() { spellerOut = backend.NewSpeller(set) },
			check: func(tb assert.TB) { assert.Equal(tb, spellerOut, s, "NewSpeller returns the set's speller") },
		},
		{name: "Spell", caseName: "a builtin name", call: func() { out, err = s.Spell(bare) }, check: spells(intType)},
		{
			name: "Spell", caseName: "a qualified name", allocs: qualifiedAllocs,
			call: func() { out, err = s.Spell(qualified) }, check: spells(qualifiedRow),
		},
		{
			name: "Spell", caseName: "a qualified name of a package that the run loaded", allocs: qualifiedAllocs,
			call: func() { out, err = named.Spell(qualified) }, check: spells(storeClause + "." + rowName),
		},
		{
			name: "TypeParams", allocs: typeParamsAllocs,
			call: func() { out, err = s.TypeParams(params) }, check: spells("[K comparable, V any]"),
		},
		{
			name: "TypeArgs", allocs: typeArgsAllocs,
			call: func() { out, err = s.TypeArgs(list) }, check: spells("[K, V]"),
		},
		{
			name: "Params", caseName: "two parameters", allocs: paramsAllocs,
			call: func() { out, err = s.Params(args) }, check: spells("ctx context.Context, id int"),
		},
		{
			name: "Params", caseName: "three parameters", allocs: partsAllocs,
			call: func() { out, err = s.Params(three) }, check: spells("id int, count int, ctx int"),
		},
		{
			name: "Results", allocs: resultsAllocs,
			call: func() { out, err = s.Results(results) }, check: spells(" (store.Row, error)"),
		},
		{name: "Receiver", call: func() { out, err = s.Receiver(method) }, check: spells(rowName)},
		{
			name: "VarType", allocs: varTypeAllocs,
			call: func() { out, err = s.VarType(variable) }, check: spells(" int"),
		},
		{
			name: "Funcs", allocs: funcsAllocs,
			call:  func() { funcs = backend.Funcs(set) },
			check: func(tb assert.TB) { assert.Length(tb, funcs, 20, "Funcs returns the twenty helpers") },
		},
		{
			name: "Docs", allocs: docsAllocs,
			call: func() { out = backend.Docs(doc) }, check: spells("// " + rowDoc + "\n"),
		},
		{
			name: "WrapDocs", allocs: wrapDocsAllocs,
			call: func() { lines = backend.WrapDocs(wrapped) },
			check: func(tb assert.TB) {
				assert.Equal(tb, lines, []string{filledLine, "next"}, "WrapDocs wraps the text")
			},
		},
		{
			name: "MemberDocs", allocs: docsAllocs,
			call: func() { out = backend.MemberDocs(doc) }, check: spells("\t// " + rowDoc + "\n"),
		},
		{name: "Package", call: func() { out, err = backend.Package(pkg) }, check: spells(storeName)},
		{name: "Guard", call: func() { out, err = backend.Guard(row) }, check: spells("")},
		{name: "SigGuard", call: func() { out, err = backend.SigGuard(signature) }, check: spells("")},
		{name: "EmbedGuard", call: func() { out, err = backend.EmbedGuard(embed) }, check: spells("")},
		{
			name: "Tag", caseName: "a field without tag entries",
			call: func() { out, err = backend.Tag(untagged) }, check: spells(""),
		},
		{
			name: "Tag", caseName: "a field with one tag entry", allocs: tagAllocs,
			call: func() { out, err = backend.Tag(field) }, check: spells(" `json:\"id\"`"),
		},
		{
			name: "Directives", allocs: directivesAllocs,
			call:  func() { out, err = backend.Directives(annotations) },
			check: spells("//go:generate stringer -type=Colour\n"),
		},
		{
			name: "MemberDirectives", allocs: directivesAllocs,
			call:  func() { out, err = backend.MemberDirectives(annotations) },
			check: spells("\t//go:generate stringer -type=Colour\n"),
		},
		{
			name: "Constraint", caseName: "annotations without a build constraint",
			call: func() { out = backend.Constraint(annotations) }, check: spells(""),
		},
		{
			name: "Constraint", caseName: "a build constraint", allocs: constraintAllocs,
			call:  func() { out = backend.Constraint(constrained) },
			check: spells("//go:build " + linuxConstraint + "\n\n"),
		},
	}
}

// ref returns an unresolved reference spelled s.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// imported returns a reference to name in pkg, spelled through the
// qualifier the source file bound.
func imported(qualifier, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: qualifier + "." + name, Package: pkg}
}

// aliased is the reference to rowName a source file spelled through
// aliasName.
func aliased() *emit.TypeRef { return imported(aliasName, storePkg, rowName) }

// composite returns a structural reference over children.
func composite(form symbol.TypeForm, spelling string, children ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Form: form, Spelling: spelling, Elems: children}
}

// fnType returns a function type over its parameters then its
// results, the results beginning at split.
func fnType(spelling string, split int, children ...*emit.TypeRef) *emit.TypeRef {
	f := composite(symbol.FormFunc, spelling, children...)
	f.Split = split
	return f
}

// sizedArray returns an array of four aliased rows.
func sizedArray() *emit.TypeRef {
	array := composite(symbol.FormArray, "[4]st.Row", aliased())
	array.Length = 4
	return array
}

// speller returns a speller over a fresh set, and the set.
func speller() (backend.Speller, *render.ImportSet) {
	set := &render.ImportSet{}
	return backend.NewSpeller(set), set
}

// declaredNames returns the names that the package clauses of a run
// declare: storeClause for storePkg, storeName for legacyPkg, and false
// for any other path.
func declaredNames(path string) (string, bool) {
	switch path {
	case storePkg:
		return storeClause, true
	case legacyPkg:
		return storeName, true
	default:
		return "", false
	}
}

// spelled spells one reference and asserts it spells.
func spelled(tb assert.TB, s backend.Speller, t *emit.TypeRef) string {
	tb.Helper()

	out, err := s.Spell(t)
	assert.NoError(tb, err, "the reference spells")
	return out
}
