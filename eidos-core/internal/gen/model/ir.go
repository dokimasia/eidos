// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// Side says which model a field lands in.
//
// The zero value is [SideNode], so a field whose tag lowering
// refuses never silently reaches the emit model.
type Side uint8

const (
	// SideNode places the field on the node model only.
	SideNode Side = iota
	// SideEmit places the field on the emit model only.
	SideEmit
	// SideBoth places the field on both models.
	SideBoth
)

// OnNode reports whether the field lands on the node model.
func (s Side) OnNode() bool { return s == SideNode || s == SideBoth }

// OnEmit reports whether the field lands on the emit model.
func (s Side) OnEmit() bool { return s == SideEmit || s == SideBoth }

// KindSpec is one declaration kind, lowered from its schema struct.
//
// Kinds answer in schema declaration order: files sort by name and
// declarations keep their order within a file. That order fixes the
// generated Kind constants and every rendered switch, which is what
// makes the same schema bytes produce the same output bytes.
type KindSpec struct {
	// Name is the kind's Go type name, "Struct".
	Name string
	// Fields are the annotated fields, in declaration order.
	Fields []FieldSpec
}

// FieldSpec is one annotated field of a kind.
//
// Type is the spelling the generated model carries. A schema kind
// name stays bare, so a template can qualify it for the side it
// renders; a type from another package keeps its qualifier.
//
// Elem names the kind a pointer or slice field references and is
// empty for everything else, which is what the traversal, the
// rewiring and the slot accessors switch on. Slice says the field
// holds many of them, and IsSymbol says the field is typed by the
// [MarkerName] marker and so admits any kind.
type FieldSpec struct {
	Name     string
	Type     string
	Elem     string
	Side     Side
	Walk     bool
	Slot     string
	Owner    bool
	Slice    bool
	IsSymbol bool
}

// Lower parses and type-checks the schema in dir and answers its
// kinds.
//
// modRoot roots the importer that resolves the schema's imports; a
// schema importing nothing lowers with an empty modRoot. Type
// checking runs first, so an undefined type is reported as such
// rather than lowered into a plausible-looking spelling.
//
// Every violation of the annotation contract is an error naming the
// schema position. The package documentation lists them.
func Lower(dir, modRoot string) ([]KindSpec, error) {
	fset := token.NewFileSet()
	_, files, err := gosource.Load(fset, dir, SchemaPackage, modRoot)
	if err != nil {
		return nil, fmt.Errorf("model: load schema: %w", err)
	}

	declared, err := collect(fset, files)
	if err != nil {
		return nil, err
	}
	structs := make(map[string]*ast.StructType, len(declared))
	for _, d := range declared {
		structs[d.name] = d.typ
	}

	kinds := make([]KindSpec, 0, len(declared))
	for _, d := range declared {
		kind, err := lowerKind(fset, d, structs)
		if err != nil {
			return nil, err
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

// declaration is one schema struct, kept in declaration order.
type declaration struct {
	name string
	typ  *ast.StructType
	pos  token.Pos
}

// collect gathers the schema's struct declarations in order.
//
// Imports pass through, since a kind's fields are typed from other
// packages. Everything else the schema might declare is refused: a
// helper type would generate a kind nobody meant to add, and a
// constant or a function would not generate at all, so either would
// be a silent omission.
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
				if typeSpec.Name.Name == MarkerName {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || typeSpec.Assign.IsValid() {
					return nil, at(fset, typeSpec.Pos(),
						"%s is not a struct: a schema declares kinds and the %s marker only",
						typeSpec.Name.Name, MarkerName)
				}
				if !typeSpec.Name.IsExported() {
					return nil, at(fset, typeSpec.Pos(),
						"%s is not exported: every kind is public", typeSpec.Name.Name)
				}
				out = append(out, declaration{
					name: typeSpec.Name.Name,
					typ:  structType,
					pos:  typeSpec.Pos(),
				})
			}
		}
	}
	return out, nil
}

// lowerKind lowers one struct into its kind spec.
//
// Fields carrying no eidos tag are skipped rather than refused, so
// a schema may hold a field the models do not carry. Slot names are
// unique within a kind, because two slots under one name would give
// the generated accessors one target and two meanings.
func lowerKind(
	fset *token.FileSet,
	d declaration,
	structs map[string]*ast.StructType,
) (KindSpec, error) {
	kind := KindSpec{Name: d.name}
	slots := make(map[string]bool)

	for _, field := range d.typ.Fields.List {
		tag, ok := tagOf(field)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			spec, err := lowerField(fset, name.Name, tag, field.Type, structs)
			if err != nil {
				return KindSpec{}, err
			}
			if spec.Slot != "" {
				if slots[spec.Slot] {
					return KindSpec{}, at(fset, field.Pos(),
						"%s declares slot %q twice", d.name, spec.Slot)
				}
				slots[spec.Slot] = true
			}
			if spec.Owner {
				if err := requireHost(fset, field.Pos(), spec, structs); err != nil {
					return KindSpec{}, err
				}
			}
			kind.Fields = append(kind.Fields, spec)
		}
	}
	return kind, nil
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
	spec.Type, spec.Elem, spec.Slice, spec.IsSymbol = describe(expr, structs)

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
		case tok == OwnerToken:
			spec.Owner = true
		case strings.HasPrefix(tok, SlotPrefix):
			spec.Slot = strings.TrimPrefix(tok, SlotPrefix)
		default:
			return FieldSpec{}, at(fset, expr.Pos(),
				"%s carries unknown tag token %q: the vocabulary is %s, %s, %s and a side",
				name, tok, WalkToken, OwnerToken, SlotPrefix)
		}
	}
	return spec, validate(fset, expr.Pos(), spec)
}

// validate holds the three rules a lowered field has to satisfy.
func validate(fset *token.FileSet, pos token.Pos, spec FieldSpec) error {
	referencesKind := spec.Elem != ""

	if spec.Walk && !referencesKind && !spec.IsSymbol {
		return at(fset, pos,
			"%s is tagged %s but %s is neither a kind nor the %s marker",
			spec.Name, WalkToken, spec.Type, MarkerName)
	}
	sliceOfKinds := spec.Slice && (referencesKind || spec.IsSymbol)
	if (spec.Slot != "" || spec.Owner) && !sliceOfKinds {
		token := SlotPrefix
		if spec.Owner {
			token = OwnerToken
		}
		return at(fset, pos, "%s is tagged %s but %s is not a slice of kinds",
			spec.Name, token, spec.Type)
	}
	if spec.Name == HostField && spec.Walk {
		return at(fset, pos,
			"%s is tagged %s: the back-pointer stays out of the traversal, "+
				"which is what keeps it a tree over a cyclic graph", HostField, WalkToken)
	}
	return nil
}

// requireHost refuses an owner-tagged slice whose element kind
// declares no back-pointer for the rewiring pass to fill.
func requireHost(
	fset *token.FileSet,
	pos token.Pos,
	spec FieldSpec,
	structs map[string]*ast.StructType,
) error {
	element, ok := structs[spec.Elem]
	if !ok {
		return nil
	}
	for _, field := range element.Fields.List {
		for _, name := range field.Names {
			if name.Name == HostField {
				return nil
			}
		}
	}
	return at(fset, pos, "%s is tagged %s but %s declares no %s field",
		spec.Name, OwnerToken, spec.Elem, HostField)
}

// describe renders a field's type as the generated model spells it
// and reports what lowering needs to know about it.
//
// A schema kind name stays bare so a template can qualify it per
// side; a qualified type keeps its package. elem names the
// referenced kind for a pointer or a slice of kinds, slice says the
// field holds many, and marker says the field is typed by the
// heterogeneous marker.
func describe(
	expr ast.Expr,
	structs map[string]*ast.StructType,
) (spelling, elem string, slice, marker bool) {
	switch typed := expr.(type) {
	case *ast.Ident:
		if typed.Name == MarkerName {
			return typed.Name, "", false, true
		}
		if _, ok := structs[typed.Name]; ok {
			return typed.Name, typed.Name, false, false
		}
		return typed.Name, "", false, false
	case *ast.StarExpr:
		inner, innerElem, _, innerMarker := describe(typed.X, structs)
		return "*" + inner, innerElem, false, innerMarker
	case *ast.ArrayType:
		inner, innerElem, _, innerMarker := describe(typed.Elt, structs)
		return "[]" + inner, innerElem, true, innerMarker
	case *ast.SelectorExpr:
		pkg, ok := typed.X.(*ast.Ident)
		if !ok {
			return "", "", false, false
		}
		return pkg.Name + "." + typed.Sel.Name, "", false, false
	default:
		return "", "", false, false
	}
}

// tagOf reads a field's eidos tag, reporting false when it carries
// none.
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

// at formats an error carrying its schema position.
func at(fset *token.FileSet, pos token.Pos, format string, args ...any) error {
	return fmt.Errorf("model: %s: %s", fset.Position(pos), fmt.Sprintf(format, args...))
}
