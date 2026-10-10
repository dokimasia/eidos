// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/gosource"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

// commentMarker opens a line comment, which the subject mark is written
// as.
const commentMarker = "//"

// conventions lists the fields that recur across the kinds with one
// meaning: the type of each, and the side that declares it where one
// side is the convention.
var conventions = []struct {
	field string
	typ   reflect.Type
	side  string
}{
	{field: "ID", typ: reflect.TypeFor[symbol.Identity](), side: model.SideNodeToken},
	{field: "Origin", typ: reflect.TypeFor[symbol.Identity](), side: model.SideEmitToken},
	{field: "Pos", typ: reflect.TypeFor[position.Pos](), side: model.SideNodeToken},
	{field: "Doc", typ: reflect.TypeFor[[]string]()},
	{field: "Comment", typ: reflect.TypeFor[string](), side: model.SideBothToken},
	{field: "Annotations", typ: reflect.TypeFor[symbol.Annotations](), side: model.SideBothToken},
	{field: "Host", typ: reflect.TypeFor[symbol.Identity](), side: model.SideNodeToken},
}

// boundary contains the kinds of the module boundary, which is read from
// source and never generated, so every field of them is node-side, the
// recurring ones included. The container cases check that rule.
var boundary = map[string]bool{"Import": true, "Export": true, "Binding": true}

// eidosTag is one field's annotation, split into the parts lowering
// reads.
type eidosTag struct {
	tokens []string
	side   string
	slot   string
	fact   string
	walk   bool
	named  bool
}

// The markers are the one part of the schema lowering matches by
// name rather than by structure, so renaming either compiles here
// and breaks the generator. The names are pinned against the
// generator's own constants. The markers are the vocabulary the whole
// schema shares, so the cases that check every kind against one rule
// are here too.
func TestMarker(t *testing.T) {
	t.Parallel()

	t.Run("names each marker the way lowering matches it", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			typ  reflect.Type
			want string
		}{
			{
				name: "the heterogeneous-field marker",
				typ:  reflect.TypeFor[schema.Symbol](),
				want: model.MarkerName,
			},
			{
				name: "the body marker",
				typ:  reflect.TypeFor[schema.Body](),
				want: model.BodyMarkerName,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.typ.Name(), tt.want,
					"lowering reads the marker by name, so the spelling is contract")
				assert.Equal(t, tt.typ.Kind(), reflect.Interface,
					"a marker admits anything the field's contract allows")
				assert.Equal(t, tt.typ.NumMethod(), 0,
					"and declares no methods: it is read by name, never by structure")
			})
		}
	})

	t.Run("types a field admitting any kind by the Symbol marker", func(t *testing.T) {
		t.Parallel()

		// A field typed by a concrete kind accepts that kind alone. The
		// marker states that the language admits any kind there.
		tests := map[string]struct {
			kind  reflect.Type
			field string
		}{
			"File.Decls":      {reflect.TypeFor[schema.File](), "Decls"},
			"Struct.Types":    {reflect.TypeFor[schema.Struct](), "Types"},
			"Interface.Types": {reflect.TypeFor[schema.Interface](), "Types"},
		}
		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				kind := tt.kind
				field, held := kind.FieldByName(tt.field)
				assert.True(t, held, name+" is declared")
				assert.Equal(t, field.Type.Kind(), reflect.Slice,
					"a heterogeneous field is a list of declarations")
				assert.Equal(t, field.Type.Elem().Name(), model.MarkerName,
					"typed by the marker, so every kind is admitted")
				assert.True(t, annotation(t, kind, field.Name).walk,
					"and walked, because the declarations are inside this one")
			})
		}
	})

	t.Run("types a callable's emit content by the Body marker", func(t *testing.T) {
		t.Parallel()

		// Parsed bodies are out of scope entirely, so the field is
		// declared emit-side and the node model never sees it.
		for _, name := range []string{"Function", "Method"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				kind := everyKind()[name]
				field, held := kind.FieldByName(model.BodyMarkerName)
				assert.True(t, held, name+" declares a body")
				assert.Equal(t, field.Type.Name(), model.BodyMarkerName,
					"typed by the marker, which resolves to the emit package's own Body")
				assert.Equal(t, annotation(t, kind, field.Name).side, model.SideEmitToken,
					"emit-side alone: the node model states a signature and never a body")
			})
		}
	})

	t.Run("lists every struct the schema declares as a kind", func(t *testing.T) {
		t.Parallel()

		declared := []string{}
		for name, spec := range everyDeclaration(t) {
			if _, isStruct := spec.Type.(*ast.StructType); isStruct {
				declared = append(declared, name)
			}
		}
		kinds := everyKind()
		listed := make([]string, 0, len(kinds))
		for name := range kinds {
			listed = append(listed, name)
		}
		assert.Permutation(t, listed, declared,
			"every kind the schema declares takes part in the package-wide cases: "+
				"adding one is an edit here as well as in the schema")
	})

	t.Run("gives every recurring field convention a kind that declares it", func(t *testing.T) {
		t.Parallel()

		// The per-kind documentation does not repeat these, which is
		// sound only while some kind declares each of them.
		for _, convention := range conventions {
			declared := false
			for _, kind := range everyKind() {
				if _, held := kind.FieldByName(convention.field); held {
					declared = true
					break
				}
			}
			assert.True(t, declared,
				convention.field+" is a convention, and one no kind declares documents nothing")
		}
	})
}

// everyKind returns each declaration kind the schema declares, by
// name.
//
// The package is read by name and by structure rather than imported,
// so a case stating one rule over the whole vocabulary enumerates it
// here. A case of [TestMarker] checks this list against the package's
// own source.
func everyKind() map[string]reflect.Type {
	return map[string]reflect.Type{
		"Alias":       reflect.TypeFor[schema.Alias](),
		"Binding":     reflect.TypeFor[schema.Binding](),
		"Constant":    reflect.TypeFor[schema.Constant](),
		"Embed":       reflect.TypeFor[schema.Embed](),
		"Enum":        reflect.TypeFor[schema.Enum](),
		"EnumVariant": reflect.TypeFor[schema.EnumVariant](),
		"Export":      reflect.TypeFor[schema.Export](),
		"Field":       reflect.TypeFor[schema.Field](),
		"File":        reflect.TypeFor[schema.File](),
		"Function":    reflect.TypeFor[schema.Function](),
		"Import":      reflect.TypeFor[schema.Import](),
		"Interface":   reflect.TypeFor[schema.Interface](),
		"Method":      reflect.TypeFor[schema.Method](),
		"Package":     reflect.TypeFor[schema.Package](),
		"Param":       reflect.TypeFor[schema.Param](),
		"Return":      reflect.TypeFor[schema.Return](),
		"Struct":      reflect.TypeFor[schema.Struct](),
		"Sum":         reflect.TypeFor[schema.Sum](),
		"SumVariant":  reflect.TypeFor[schema.SumVariant](),
		"TypeParam":   reflect.TypeFor[schema.TypeParam](),
		"TypeRef":     reflect.TypeFor[schema.TypeRef](),
		"Variable":    reflect.TypeFor[schema.Variable](),
	}
}

// familyOf returns the kinds one schema file declares, by name, and
// fails for a struct the file declares that [everyKind] does not list.
func familyOf(tb assert.TB, file string) map[string]reflect.Type {
	tb.Helper()

	kinds := everyKind()
	out := map[string]reflect.Type{}
	for name := range subjectsOf(tb, file) {
		assert.Contains(tb, kinds, name, file+" declares "+name+", and every kind takes part in the shared cases")
		out[name] = kinds[name]
	}
	return out
}

// everySide returns the three side tokens, one of which opens every
// tag.
func everySide() []string {
	return []string{model.SideNodeToken, model.SideEmitToken, model.SideBothToken}
}

// assertAnnotations fails unless every field of the kinds has a tag
// from the closed vocabulary, under the rules lowering relies on: a side
// opens every tag, every other token is known, only a containment edge
// is walked, only an emit-visible field states a fact, only a plain
// string is a declared name, and a slot is emit-visible slice storage.
func assertAnnotations(t *testing.T, kinds map[string]reflect.Type) {
	t.Helper()

	for name, kind := range kinds {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for field := range kind.Fields() {
				raw, tagged := field.Tag.Lookup(model.TagKey)
				assert.True(t, tagged,
					"every field has an "+model.TagKey+" tag, and "+
						field.Name+" has none, so the generator would drop it")
				side, _, _ := strings.Cut(raw, model.TokenSeparator)
				assert.Contains(t, everySide(), side,
					"one of the three sides opens every tag, and "+field.Name+" opens with "+side)

				tag := annotation(t, kind, field.Name)
				for _, token := range tag.tokens[1:] {
					known := token == model.WalkToken || token == model.NameToken ||
						strings.HasPrefix(token, model.SlotPrefix) ||
						strings.HasPrefix(token, model.FactPrefix)
					assert.True(t, known, "the token set is closed, and "+field.Name+" states "+token)
				}
				// The walk is a tree over a graph that is not one. A field
				// naming another declaration by identity is a cross-reference
				// reached through a tracked read, and walking it would close
				// a cycle.
				if tag.walk {
					assert.True(t, contains(field.Type),
						field.Name+" is walked, so it is a declaration this one "+
							"contains rather than one it names")
				}
				// Coverage is a render question, so a fact only the node
				// model states could never be checked.
				if tag.fact != "" {
					assert.NotEqual(t, tag.side, model.SideNodeToken,
						field.Name+" states fact "+tag.fact+", so the emit model declares it")
				}
				if tag.named {
					assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
						field.Name+" is a declared name, and a name is one spelling rather than a list")
				}
				if tag.slot != "" {
					assert.Equal(t, field.Type.Kind(), reflect.Slice,
						field.Name+" declares slot "+tag.slot+", which replaces a slice with slot storage")
					assert.NotEqual(t, tag.side, model.SideNodeToken,
						field.Name+" declares slot "+tag.slot+", and slots are the emit model's")
				}
			}
		})
	}
}

// assertConventions fails unless every recurring field of the kinds has
// the shared type, on the shared side outside the module boundary.
func assertConventions(t *testing.T, kinds map[string]reflect.Type) {
	t.Helper()

	for _, convention := range conventions {
		t.Run(convention.field, func(t *testing.T) {
			t.Parallel()

			for name, kind := range kinds {
				field, held := kind.FieldByName(convention.field)
				if !held {
					continue
				}
				assert.Equal(t, field.Type.String(), convention.typ.String(),
					name+"."+convention.field+" has the shared type")
				if convention.side == "" || boundary[name] {
					continue
				}
				assert.Equal(t, annotation(t, kind, convention.field).side, convention.side,
					name+"."+convention.field+" is on the shared side")
			}
		})
	}
}

// assertSubjects fails unless one schema file marks exactly the named
// kinds as dispatch subjects.
//
// The mark decides which kinds get a generated Match type and trigger
// constructor, so it is the dispatch vocabulary a rule author writes
// against.
func assertSubjects(tb assert.TB, file string, want ...string) {
	tb.Helper()

	got := []string{}
	for name, subject := range subjectsOf(tb, file) {
		if subject {
			got = append(got, name)
		}
	}
	assert.Permutation(tb, got, want,
		file+" marks exactly the kinds a rule can match: a mark gained or "+
			"lost changes the dispatch vocabulary", assert.EquateEmpty())
}

// annotation returns a field's eidos tag, failing the test for a
// field the kind does not declare.
func annotation(tb assert.TB, kind reflect.Type, field string) eidosTag {
	tb.Helper()

	declared, held := kind.FieldByName(field)
	assert.True(tb, held, kind.Name()+" declares "+field)
	if !held {
		return eidosTag{}
	}
	raw, tagged := declared.Tag.Lookup(model.TagKey)
	assert.True(tb, tagged, kind.Name()+"."+field+" has an "+model.TagKey+" tag")

	out := eidosTag{tokens: strings.Split(raw, model.TokenSeparator)}
	out.side = out.tokens[0]
	for _, token := range out.tokens[1:] {
		switch {
		case token == model.WalkToken:
			out.walk = true
		case token == model.NameToken:
			out.named = true
		case strings.HasPrefix(token, model.SlotPrefix):
			out.slot = strings.TrimPrefix(token, model.SlotPrefix)
		case strings.HasPrefix(token, model.FactPrefix):
			out.fact = strings.TrimPrefix(token, model.FactPrefix)
		}
	}
	return out
}

// contains reports whether a field type is a containment edge: a
// kind this declaration owns, a list of them, or the heterogeneous
// marker in either shape.
func contains(typ reflect.Type) bool {
	if typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Interface {
		return typ.Name() == model.MarkerName
	}
	return typ.Kind() == reflect.Pointer && typ.Elem().Kind() == reflect.Struct &&
		typ.Elem().PkgPath() == reflect.TypeFor[schema.TypeRef]().PkgPath()
}

// everyDeclaration returns every type the schema declares, by name,
// read from the package's own source.
//
// Reflection cannot answer which file a type was declared in or
// whether it has a doc directive, and both are contract: the kinds
// group by family, one file each, and the subject mark decides which of
// them a rule can match.
func everyDeclaration(tb assert.TB) map[string]*ast.TypeSpec {
	tb.Helper()

	files, err := gosource.ParseDir(token.NewFileSet(), ".", gosource.HandWritten)
	assert.NoError(tb, err, "the schema package parses")

	out := map[string]*ast.TypeSpec{}
	for _, file := range files {
		for _, spec := range typeSpecs(file) {
			out[spec.Name.Name] = spec
		}
	}
	return out
}

// subjectsOf returns the type names one schema file declares, each
// mapped to whether it has the dispatch subject mark.
func subjectsOf(tb assert.TB, name string) map[string]bool {
	tb.Helper()

	fset := token.NewFileSet()
	files, err := gosource.ParseDir(fset, ".", gosource.HandWritten)
	assert.NoError(tb, err, "the schema package parses")

	out := map[string]bool{}
	for _, file := range files {
		if filepath.Base(fset.Position(file.Package).Filename) != name {
			continue
		}
		for _, spec := range typeSpecs(file) {
			out[spec.Name.Name] = marked(spec)
		}
	}
	assert.NotEmpty(tb, out, name+" declares the kinds its family covers")
	return out
}

// typeSpecs returns every type declared in one file, with the
// doc comment attached wherever the parser put it.
func typeSpecs(file *ast.File) []*ast.TypeSpec {
	var out []*ast.TypeSpec
	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typ, isType := spec.(*ast.TypeSpec)
			if !isType {
				continue
			}
			if typ.Doc == nil {
				typ.Doc = gen.Doc
			}
			out = append(out, typ)
		}
	}
	return out
}

// marked reports whether a declaration has the dispatch subject mark.
// It reads the raw comment lines, because [ast.CommentGroup.Text]
// strips directives, and the mark is one.
func marked(spec *ast.TypeSpec) bool {
	if spec.Doc == nil {
		return false
	}
	for _, line := range spec.Doc.List {
		if strings.TrimPrefix(line.Text, commentMarker) == model.SubjectMark {
			return true
		}
	}
	return false
}
