// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// Side names the model a field arrives in.
//
// The zero value is [SideNode], so a field whose tag lowering
// refuses never appears on the emit model.
type Side uint8

const (
	// SideNode places the field on the node model only.
	SideNode Side = iota
	// SideEmit places the field on the emit model only.
	SideEmit
	// SideBoth places the field on both models.
	SideBoth
)

// OnNode reports whether the field arrives on the node model.
func (s Side) OnNode() bool { return s == SideNode || s == SideBoth }

// OnEmit reports whether the field arrives on the emit model.
func (s Side) OnEmit() bool { return s == SideEmit || s == SideBoth }

// KindSpec is one declaration kind, lowered from its schema struct.
//
// Kinds come back in schema declaration order: files sort by name and
// declarations keep their order within a file. That order fixes the
// generated Kind constants and every rendered switch, which is what
// makes the same schema bytes produce the same output bytes.
type KindSpec struct {
	// Name is the kind's Go type name, "Struct".
	Name string
	// Doc is the schema's documentation for the kind, one entry per
	// line with the comment markers stripped and the subject mark
	// removed.
	Doc []string
	// Subject marks a kind the dispatch surface triggers on: it
	// gets a generated Match type and constructor. Lowering refuses
	// the mark on a kind without a node-side identity, because a
	// subject that cannot be addressed cannot be dispatched.
	Subject bool
	// Fields are the annotated fields, in declaration order.
	Fields []FieldSpec
}

// FieldSpec is one annotated field of a kind.
//
// Type is the spelling the generated model uses. A schema kind name
// is bare, so a template can qualify it for the side it renders. A
// type from another package keeps its qualifier.
//
// Elem names the kind a pointer or slice field references and is
// empty for every other field. The traversal, the name respelling
// and the slot accessors switch on it. Slice reports whether the
// field is a slice, and IsSymbol reports whether the field is typed
// by the [MarkerName] marker and so admits any kind.
type FieldSpec struct {
	Name string
	// Doc is the schema's documentation for the field, and Comment
	// its trailing line comment.
	Doc      []string
	Comment  string
	Type     string
	Elem     string
	Slot     string
	Side     Side
	Walk     bool
	Slice    bool
	IsSymbol bool
	// IsName marks a declared name the respell traversal visits.
	IsName bool
	// Fact is the suffix of the generated constant for the fact the
	// field states, empty for a field that states no fact.
	Fact string
}

// EnumSpec is one enum of the symbol package: a named integer type
// and the constants of it that the package declares by hand.
type EnumSpec struct {
	// Type is the enum's type name, "Visibility".
	Type string
	// Values are the enum's constants, sorted by name.
	Values []EnumValue
}

// EnumValue is one enum constant: its name, and its value exactly as
// the type checker computes it.
type EnumValue struct {
	Name  string
	Value string
}

// Schema is one lowered schema: its declaration kinds, and the enums
// of the symbol package it imports, which the model fingerprint
// folds beside the kinds.
type Schema struct {
	// Kinds are the declaration kinds, in schema declaration order.
	Kinds []KindSpec
	// Enums are the symbol package's enums, sorted by type name, and
	// empty for a schema that imports no symbol package.
	Enums []EnumSpec
}

// Lower parses and type-checks the schema in dir and returns its
// kinds, with the enums of the symbol package it imports.
//
// modRoot is the root of the importer that resolves the schema's
// imports. A schema without imports lowers with an empty modRoot,
// and returns no enums. Type checking runs first, so an undefined
// type is reported as undefined.
//
// Every violation of the annotation contract is an error naming the
// schema position. The package documentation lists them.
func Lower(dir, modRoot string) (Schema, error) {
	fset := token.NewFileSet()
	pkg, files, err := gosource.Load(fset, dir, SchemaPackage, modRoot, gosource.HandWritten)
	if err != nil {
		return Schema{}, fmt.Errorf("model: load schema: %w", err)
	}

	declared, err := collect(fset, files)
	if err != nil {
		return Schema{}, err
	}
	structs := make(map[string]*ast.StructType, len(declared))
	for _, d := range declared {
		structs[d.name] = d.typ
	}

	kinds := make([]KindSpec, len(declared))
	for i, d := range declared {
		if kinds[i], err = lowerKind(fset, d, structs); err != nil {
			return Schema{}, err
		}
	}
	enums, err := enumsOf(fset, pkg, modRoot)
	if err != nil {
		return Schema{}, err
	}
	return Schema{Kinds: kinds, Enums: enums}, nil
}

// enumsOf returns the enums of the symbol package the schema
// imports: every constant of a named integer type the package
// declares, grouped by type. The symbol package is [SymbolPackage]
// under modRoot's module path, and a schema that imports none has
// no enums.
//
// A constant a generated file declares is left out. The generator
// writes those files from the schema's own kinds and facts, which
// the fingerprint folds directly, so one run reads the same enums
// whether or not an earlier run left the files behind.
func enumsOf(fset *token.FileSet, schema *types.Package, modRoot string) ([]EnumSpec, error) {
	if modRoot == "" {
		return nil, nil
	}
	modPath, err := gosource.ModulePath(modRoot)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	var symbols *types.Package
	for _, imported := range schema.Imports() {
		if imported.Path() == modPath+"/"+SymbolPackage {
			symbols = imported
		}
	}
	if symbols == nil {
		return nil, nil
	}
	byType := map[string][]EnumValue{}
	for _, name := range symbols.Scope().Names() {
		c, isConst := symbols.Scope().Lookup(name).(*types.Const)
		if !isConst || strings.HasSuffix(fset.Position(c.Pos()).Filename, genfile.GeneratedSuffix) {
			continue
		}
		named, isNamed := c.Type().(*types.Named)
		if !isNamed || named.Obj().Pkg() != symbols {
			continue
		}
		basic, isBasic := named.Underlying().(*types.Basic)
		if !isBasic || basic.Info()&types.IsInteger == 0 {
			continue
		}
		enum := named.Obj().Name()
		byType[enum] = append(byType[enum], EnumValue{Name: name, Value: c.Val().ExactString()})
	}
	out := make([]EnumSpec, 0, len(byType))
	for _, enum := range slices.Sorted(maps.Keys(byType)) {
		out = append(out, EnumSpec{Type: enum, Values: byType[enum]})
	}
	return out, nil
}

// declaration is one schema struct, kept in declaration order.
type declaration struct {
	name    string
	typ     *ast.StructType
	pos     token.Pos
	doc     []string
	subject bool
}

// collect gathers the schema's struct declarations in order.
//
// Imports and the two marker types pass through. A constant, a
// variable, a function, a non-struct type or an unexported struct is
// refused, because the generator turns every exported struct into a
// kind and generates nothing from the rest.
func collect(fset *token.FileSet, files []*ast.File) ([]declaration, error) {
	var out []declaration
	for _, file := range files {
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if ok && group.Tok == token.IMPORT {
				continue
			}
			if !ok || group.Tok != token.TYPE {
				return nil, at(fset, decl.Pos(),
					"a schema declares types and imports only")
			}
			for _, spec := range group.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					return nil, at(fset, spec.Pos(), "a schema declares types only")
				}
				if typeSpec.Name.Name == MarkerName ||
					typeSpec.Name.Name == BodyMarkerName {

					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || typeSpec.Assign.IsValid() {
					return nil, at(fset, typeSpec.Pos(),
						"%s is not a struct: a schema declares kinds and the %s and %s markers only",
						typeSpec.Name.Name, MarkerName, BodyMarkerName)
				}
				if !typeSpec.Name.IsExported() {
					return nil, at(fset, typeSpec.Pos(),
						"%s is not exported: every kind is public", typeSpec.Name.Name)
				}
				doc := typeSpec.Doc
				if doc == nil {
					doc = group.Doc
				}
				lines, subject := splitSubject(docLines(doc))
				out = append(out, declaration{
					name:    typeSpec.Name.Name,
					typ:     structType,
					pos:     typeSpec.Pos(),
					doc:     lines,
					subject: subject,
				})
			}
		}
	}
	return out, nil
}

// lowerKind lowers one struct into its kind spec.
//
// A field without an eidos tag is skipped, so a schema may declare
// a field the models leave out. Slot names are unique within a
// kind, because the generated accessors address a slot by its name.
func lowerKind(
	fset *token.FileSet,
	d declaration,
	structs map[string]*ast.StructType,
) (KindSpec, error) {
	kind := KindSpec{Name: d.name, Doc: d.doc, Subject: d.subject}
	slots := make(map[string]bool)

	for _, field := range d.typ.Fields.List {
		tag, ok := tagOf(field)
		if !ok {
			continue
		}
		if len(field.Names) == 0 {
			return KindSpec{}, at(fset, field.Pos(),
				"%s embeds %s under an %s tag, and the models declare no embedded field",
				d.name, types.ExprString(field.Type), TagKey)
		}
		for _, name := range field.Names {
			spec, err := lowerField(fset, name.Name, tag, field.Type, structs)
			if err != nil {
				return KindSpec{}, err
			}
			spec.Doc = docLines(field.Doc)
			if lines := docLines(field.Comment); len(lines) == 1 {
				spec.Comment = lines[0]
			}
			if spec.Slot != "" {
				if slots[spec.Slot] {
					return KindSpec{}, at(fset, field.Pos(),
						"%s declares slot %q twice", d.name, spec.Slot)
				}
				slots[spec.Slot] = true
			}
			kind.Fields = append(kind.Fields, spec)
		}
	}
	if kind.Subject && !identified(kind.Fields) {
		return KindSpec{}, at(fset, d.pos,
			"%s is marked a subject and has no node identity: "+
				"a subject that cannot be addressed cannot be dispatched", d.name)
	}
	return kind, nil
}

// identified reports whether the fields include a node-side
// identity.
func identified(fields []FieldSpec) bool {
	for _, f := range fields {
		if f.Name == "ID" && f.Side.OnNode() {
			return true
		}
	}
	return false
}

// lowerField lowers one field's tag and type into a field spec,
// refusing every annotation the contract does not allow.
func lowerField(
	fset *token.FileSet,
	name, tag string,
	expr ast.Expr,
	structs map[string]*ast.StructType,
) (FieldSpec, error) {
	spec := FieldSpec{Name: name}
	var spelt bool
	spec.Type, spec.Elem, spec.Slice, spec.IsSymbol, spelt = describe(expr, structs)
	if !spelt {
		return FieldSpec{}, at(fset, expr.Pos(),
			"%s is typed %s, which the models cannot spell: a field is a name, "+
				"a qualified name, a pointer or a slice", name, types.ExprString(expr))
	}

	tokens := strings.Split(tag, TokenSeparator)
	side, err := parseSide(fset, expr.Pos(), tokens[0])
	if err != nil {
		return FieldSpec{}, err
	}
	spec.Side = side

	for _, tok := range tokens[1:] {
		switch {
		case tok == WalkToken:
			spec.Walk = true
		case tok == NameToken:
			spec.IsName = true
		case strings.HasPrefix(tok, SlotPrefix):
			spec.Slot = strings.TrimPrefix(tok, SlotPrefix)
		case strings.HasPrefix(tok, FactPrefix):
			spec.Fact = strings.TrimPrefix(tok, FactPrefix)
		default:
			return FieldSpec{}, at(fset, expr.Pos(),
				"%s states unknown tag token %q: the vocabulary is %s, %s, %s, %s and a side",
				name, tok, WalkToken, NameToken, SlotPrefix, FactPrefix)
		}
	}
	return spec, validate(fset, expr.Pos(), spec)
}

// validate checks the rules a lowered field has to satisfy.
func validate(fset *token.FileSet, pos token.Pos, spec FieldSpec) error {
	referencesKind := spec.Elem != ""

	if spec.Walk && !referencesKind && !spec.IsSymbol {
		return at(fset, pos,
			"%s is tagged %s but %s is neither a kind nor the %s marker",
			spec.Name, WalkToken, spec.Type, MarkerName)
	}
	sliceOfKinds := spec.Slice && (referencesKind || spec.IsSymbol)
	if spec.Slot != "" && !sliceOfKinds {
		return at(fset, pos, "%s is tagged %s but %s is not a slice of kinds",
			spec.Name, SlotPrefix, spec.Type)
	}
	if spec.IsName && spec.Type != stringType {
		return at(fset, pos,
			"%s is tagged %s but %s is not a string: a name is one spelling",
			spec.Name, NameToken, spec.Type)
	}
	if spec.Fact != "" {
		if !spec.Side.OnEmit() {
			return at(fset, pos,
				"%s states the %s fact on the node side alone: coverage is "+
					"a render question, so a fact field is emit-visible",
				spec.Name, spec.Fact)
		}
		if !exportedIdent(spec.Fact) {
			return at(fset, pos,
				"%s states fact %q, which is not an exported identifier: "+
					"the value becomes the generated constant's suffix",
				spec.Name, spec.Fact)
		}
	}
	return nil
}

// exportedIdent reports whether a fact value spells as an exported
// Go identifier: a capital, then letters and digits.
func exportedIdent(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// describe renders a field's type as the generated model spells it
// and reports what lowering needs to know about it.
//
// A schema kind name is bare, so a template can qualify it per
// side. A qualified type keeps its package. elem names the
// referenced kind for a pointer or a slice of kinds. slice reports
// whether the field is a slice, and marker reports whether the
// field is typed by the heterogeneous marker. spelt is false for
// every form the models do not spell: a sized array, a map, a
// function, a channel, an instantiation or a literal type.
func describe(
	expr ast.Expr,
	structs map[string]*ast.StructType,
) (spelling, elem string, slice, marker, spelt bool) {
	switch typed := expr.(type) {
	case *ast.Ident:
		if typed.Name == MarkerName {
			return typed.Name, "", false, true, true
		}
		if _, ok := structs[typed.Name]; ok {
			return typed.Name, typed.Name, false, false, true
		}
		return typed.Name, "", false, false, true
	case *ast.StarExpr:
		inner, innerElem, _, innerMarker, innerSpelt := describe(typed.X, structs)
		return "*" + inner, innerElem, false, innerMarker, innerSpelt
	case *ast.ArrayType:
		if typed.Len != nil {
			return "", "", false, false, false
		}
		inner, innerElem, _, innerMarker, innerSpelt := describe(typed.Elt, structs)
		return "[]" + inner, innerElem, true, innerMarker, innerSpelt
	case *ast.SelectorExpr:
		pkg, ok := typed.X.(*ast.Ident)
		if !ok {
			return "", "", false, false, false
		}
		return pkg.Name + "." + typed.Sel.Name, "", false, false, true
	default:
		return "", "", false, false, false
	}
}

// tagOf reads a field's eidos tag and reports false for a field
// without one.
func tagOf(field *ast.Field) (string, bool) {
	if field.Tag == nil {
		return "", false
	}
	unquoted, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false
	}
	return reflect.StructTag(unquoted).Lookup(TagKey)
}

// parseSide reads the mandatory leading side token.
func parseSide(fset *token.FileSet, pos token.Pos, tok string) (Side, error) {
	switch tok {
	case SideNodeToken:
		return SideNode, nil
	case SideEmitToken:
		return SideEmit, nil
	case SideBothToken:
		return SideBoth, nil
	default:
		return SideNode, at(fset, pos, "unknown side %q: a tag opens with %s, %s or %s",
			tok, SideNodeToken, SideEmitToken, SideBothToken)
	}
}

// splitSubject strips the subject mark from a kind's documentation
// and reports whether the mark was present.
func splitSubject(lines []string) ([]string, bool) {
	subject := false
	out := lines[:0]
	for _, line := range lines {
		if line == SubjectMark {
			subject = true
			continue
		}
		out = append(out, line)
	}
	return out, subject
}

// docLines renders a comment group as plain lines, with the markers
// and one leading space stripped.
//
// The schema documents every kind and most fields, and the
// generated models copy that documentation. Each contract is
// written once, in the schema that decides it.
func docLines(group *ast.CommentGroup) []string {
	if group == nil {
		return nil
	}
	out := make([]string, 0, len(group.List))
	for _, comment := range group.List {
		text := strings.TrimPrefix(comment.Text, "//")
		out = append(out, strings.TrimPrefix(text, " "))
	}
	return out
}

// at formats an error with its schema position after the package
// prefix.
func at(fset *token.FileSet, pos token.Pos, format string, args ...any) error {
	return fmt.Errorf("model: %s: %s", fset.Position(pos), fmt.Sprintf(format, args...))
}
