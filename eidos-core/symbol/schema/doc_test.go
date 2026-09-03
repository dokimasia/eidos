// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/gosource"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The schema is read as source by the model generator, not imported
// at run time, so nothing here calls a function. What the twins hold
// is the shape the generator lowers: the tag vocabulary, the field
// conventions the kinds share, the marker names, and which kinds are
// dispatch subjects. A reshape that breaks one of those fails here,
// naming the field, rather than inside lowering.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()
		coretest.AssertDependencyPosition(t)
	})

	t.Run("declares one struct per kind and nothing else", func(t *testing.T) {
		t.Parallel()

		declared := []string{}
		for name, spec := range everyDeclaration(t) {
			if _, isStruct := spec.Type.(*ast.StructType); isStruct {
				declared = append(declared, name)
			}
		}
		listed := []string{}
		for name := range everyKind() {
			listed = append(listed, name)
		}
		slices.Sort(declared)
		slices.Sort(listed)
		assert.Equal(t, listed, declared,
			"every kind the schema declares takes part in the package-wide cases: "+
				"adding one is an edit here as well as in the schema")
	})

	t.Run("every field states which model carries it", func(t *testing.T) {
		t.Parallel()

		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					tag, tagged := field.Tag.Lookup(model.TagKey)
					assert.True(t, tagged,
						"every field carries an "+model.TagKey+" tag; "+
							field.Name+" carries none, so the generator would drop it")
					side, _, _ := strings.Cut(tag, model.TokenSeparator)
					assert.True(t, slices.Contains(everySide(), side),
						"one of the three sides opens every tag; "+
							field.Name+" opens with "+side)
				}
			})
		}
	})

	t.Run("every token is one the vocabulary declares", func(t *testing.T) {
		t.Parallel()

		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					for _, token := range annotation(t, kind, field.Name).tokens[1:] {
						known := token == model.WalkToken || token == model.NameToken ||
							strings.HasPrefix(token, model.SlotPrefix) ||
							strings.HasPrefix(token, model.FactPrefix)
						assert.True(t, known,
							"the token set is closed; "+field.Name+" carries "+token)
					}
				}
			})
		}
	})

	t.Run("only a containment edge is walked", func(t *testing.T) {
		t.Parallel()

		// The walk is a tree over a graph that is not one. A field
		// naming another declaration by identity is a cross-reference
		// reached through a tracked read, and walking it would close a
		// cycle.
		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					if !annotation(t, kind, field.Name).walk {
						continue
					}
					assert.True(t, contains(field.Type),
						field.Name+" is walked, so it holds a declaration this one "+
							"contains rather than one it names")
				}
			})
		}
	})

	t.Run("only an emit-visible field states a fact", func(t *testing.T) {
		t.Parallel()

		// Coverage is a render question, so a fact the node model
		// alone carries could never be checked.
		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					tag := annotation(t, kind, field.Name)
					if tag.fact == "" {
						continue
					}
					assert.NotEqual(t, tag.side, model.SideNodeToken,
						field.Name+" states fact "+tag.fact+
							", so the emit model carries it")
				}
			})
		}
	})

	t.Run("only a plain string is a declared name", func(t *testing.T) {
		t.Parallel()

		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					if !annotation(t, kind, field.Name).named {
						continue
					}
					assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
						field.Name+" is a declared name, and a name is one "+
							"spelling rather than a list")
				}
			})
		}
	})

	t.Run("a slot is emit-visible slice storage", func(t *testing.T) {
		t.Parallel()

		for name, kind := range everyKind() {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					tag := annotation(t, kind, field.Name)
					if tag.slot == "" {
						continue
					}
					assert.Equal(t, field.Type.Kind(), reflect.Slice,
						field.Name+" declares slot "+tag.slot+
							", which replaces a slice with slot storage")
					assert.NotEqual(t, tag.side, model.SideNodeToken,
						field.Name+" declares slot "+tag.slot+
							", and slots are the emit model's")
				}
			})
		}
	})

	t.Run("the recurring fields mean one thing everywhere", func(t *testing.T) {
		t.Parallel()

		// The per-kind documentation does not repeat these, which only
		// holds while every kind spells them the same way.
		conventions := []struct {
			field string
			typ   reflect.Type
			side  string
		}{
			{field: "ID", typ: reflect.TypeFor[symbol.Identity](), side: model.SideNodeToken},
			{field: "Origin", typ: reflect.TypeFor[symbol.Identity](), side: model.SideEmitToken},
			{field: "Pos", typ: reflect.TypeFor[position.Pos](), side: model.SideNodeToken},
			{field: "Doc", typ: reflect.TypeFor[[]string]()},
			{field: "Comment", typ: reflect.TypeFor[string](), side: model.SideBothToken},
			{
				field: "Annotations", typ: reflect.TypeFor[symbol.Annotations](),
				side: model.SideBothToken,
			},
			{field: "Host", typ: reflect.TypeFor[symbol.Identity](), side: model.SideNodeToken},
		}
		// The module boundary is read and never generated, so its
		// kinds carry every field node-side, the recurring ones
		// included; the container cases hold that rule.
		boundary := map[string]bool{"Import": true, "Export": true, "Binding": true}
		for _, convention := range conventions {
			t.Run(convention.field, func(t *testing.T) {
				t.Parallel()

				carried := 0
				for name, kind := range everyKind() {
					field, held := kind.FieldByName(convention.field)
					if !held {
						continue
					}
					carried++
					assert.Equal(t, field.Type.String(), convention.typ.String(),
						name+"."+convention.field+" carries the shared type")
					if convention.side == "" || boundary[name] {
						continue
					}
					assert.Equal(t, annotation(t, kind, convention.field).side, convention.side,
						name+"."+convention.field+" sits on the shared side")
				}
				assert.True(t, carried > 0,
					"a convention nothing carries is documentation of nothing")
			})
		}
	})
}

// everyKind returns each declaration kind the schema declares, by
// name.
//
// The package is read by name and by structure rather than imported,
// so a case stating one rule over the whole vocabulary enumerates it
// here. The first case above holds this list against the package's
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

// everySide returns the three side tokens, one of which opens every
// tag.
func everySide() []string {
	return []string{model.SideNodeToken, model.SideEmitToken, model.SideBothToken}
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
	slices.Sort(got)
	slices.Sort(want)
	assert.Equal(tb, got, want,
		file+" marks exactly the kinds a rule can match: a mark gained or "+
			"lost changes the dispatch vocabulary", assert.EquateEmpty())
}

// commentMarker opens a line comment, which the mark is written as.
const commentMarker = "//"

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
	assert.True(tb, tagged, kind.Name()+"."+field+" carries an "+model.TagKey+" tag")

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
// whether it carries a doc directive, and both are contract: the
// kinds group by family, one file each, and the subject mark decides
// which of them a rule can match.
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
// mapped to whether it carries the dispatch subject mark.
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

// marked reports whether a declaration carries the dispatch subject
// mark.
//
// The raw comment lines are read rather than [ast.CommentGroup.Text],
// which strips directives — and the mark is one.
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
