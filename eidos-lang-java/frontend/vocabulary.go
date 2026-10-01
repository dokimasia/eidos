// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/lang/treesitter"

// vocabulary is the Java grammar's node kinds and field names that the
// lowering compares, resolved once when the frontend is built, so a
// walk compares ids and a grammar that renames a kind fails at
// construction.
type vocabulary struct {
	grammar *treesitter.Grammar

	// The file, its comments, and its package, import and module
	// declarations.
	lineComment, blockComment, packageDeclaration, importDeclaration, asterisk,
	moduleDeclaration treesitter.Kind

	// The type declarations and their bodies.
	classDeclaration, interfaceDeclaration, enumDeclaration, recordDeclaration,
	annotationTypeDeclaration, enumBodyDeclarations treesitter.Kind

	// The members.
	fieldDeclaration, constantDeclaration, methodDeclaration, constructorDeclaration,
	compactConstructorDeclaration, annotationTypeElementDeclaration, enumConstant,
	variableDeclarator, localVariableDeclaration treesitter.Kind

	// Modifiers and annotations.
	modifiers, annotation, markerAnnotation treesitter.Kind

	// Parameters and the bodies the sweep passes over.
	formalParameter, spreadParameter, receiverParameter, typeParameter, typeBound, block,
	constructorBody treesitter.Kind

	// Names and types.
	identifier, scopedIdentifier, typeIdentifier, scopedTypeIdentifier, genericType,
	typeArguments, wildcard, arrayType, annotatedType, voidType, superKeyword treesitter.Kind

	// The clauses that list types.
	typeList, extendsInterfaces, throws treesitter.Kind

	// The fields the lowering reads children by.
	fieldName, fieldBody, fieldType, fieldTypeParameters, fieldParameters, fieldSuperclass,
	fieldInterfaces, fieldPermits, fieldValue, fieldDimensions, fieldArguments,
	fieldElement, fieldDeclarator treesitter.Field
}

// newVocabulary resolves the vocabulary from the Java grammar. It
// panics on a kind or field the grammar does not declare, through
// [treesitter.Grammar.Kind] and [treesitter.Grammar.Field].
func newVocabulary(g *treesitter.Grammar) *vocabulary {
	return &vocabulary{
		grammar: g,

		lineComment:        g.Kind("line_comment"),
		blockComment:       g.Kind("block_comment"),
		packageDeclaration: g.Kind("package_declaration"),
		importDeclaration:  g.Kind("import_declaration"),
		asterisk:           g.Kind("asterisk"),
		moduleDeclaration:  g.Kind("module_declaration"),

		classDeclaration:          g.Kind("class_declaration"),
		interfaceDeclaration:      g.Kind("interface_declaration"),
		enumDeclaration:           g.Kind("enum_declaration"),
		recordDeclaration:         g.Kind("record_declaration"),
		annotationTypeDeclaration: g.Kind("annotation_type_declaration"),
		enumBodyDeclarations:      g.Kind("enum_body_declarations"),

		fieldDeclaration:                 g.Kind("field_declaration"),
		constantDeclaration:              g.Kind("constant_declaration"),
		methodDeclaration:                g.Kind("method_declaration"),
		constructorDeclaration:           g.Kind("constructor_declaration"),
		compactConstructorDeclaration:    g.Kind("compact_constructor_declaration"),
		annotationTypeElementDeclaration: g.Kind("annotation_type_element_declaration"),
		enumConstant:                     g.Kind("enum_constant"),
		variableDeclarator:               g.Kind("variable_declarator"),
		localVariableDeclaration:         g.Kind("local_variable_declaration"),

		modifiers:        g.Kind("modifiers"),
		annotation:       g.Kind("annotation"),
		markerAnnotation: g.Kind("marker_annotation"),

		formalParameter:   g.Kind("formal_parameter"),
		spreadParameter:   g.Kind("spread_parameter"),
		receiverParameter: g.Kind("receiver_parameter"),
		typeParameter:     g.Kind("type_parameter"),
		typeBound:         g.Kind("type_bound"),
		block:             g.Kind("block"),
		constructorBody:   g.Kind("constructor_body"),

		identifier:           g.Kind("identifier"),
		scopedIdentifier:     g.Kind("scoped_identifier"),
		typeIdentifier:       g.Kind("type_identifier"),
		scopedTypeIdentifier: g.Kind("scoped_type_identifier"),
		genericType:          g.Kind("generic_type"),
		typeArguments:        g.Kind("type_arguments"),
		wildcard:             g.Kind("wildcard"),
		arrayType:            g.Kind("array_type"),
		annotatedType:        g.Kind("annotated_type"),
		voidType:             g.Kind("void_type"),
		superKeyword:         g.Kind("super"),

		typeList:          g.Kind("type_list"),
		extendsInterfaces: g.Kind("extends_interfaces"),
		throws:            g.Kind("throws"),

		fieldName:           g.Field("name"),
		fieldBody:           g.Field("body"),
		fieldType:           g.Field("type"),
		fieldTypeParameters: g.Field("type_parameters"),
		fieldParameters:     g.Field("parameters"),
		fieldSuperclass:     g.Field("superclass"),
		fieldInterfaces:     g.Field("interfaces"),
		fieldPermits:        g.Field("permits"),
		fieldValue:          g.Field("value"),
		fieldDimensions:     g.Field("dimensions"),
		fieldArguments:      g.Field("arguments"),
		fieldElement:        g.Field("element"),
		fieldDeclarator:     g.Field("declarator"),
	}
}
