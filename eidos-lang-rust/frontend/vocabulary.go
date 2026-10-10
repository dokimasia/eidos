// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/lang/treesitter"

// vocabulary is the Rust grammar's node kinds and field names that the
// lowering compares, resolved once when the frontend is built, so a
// walk compares ids and a grammar that renames a kind fails at
// construction.
type vocabulary struct {
	grammar *treesitter.Grammar

	// The file, its comments and attributes, and the items that declare
	// nothing.
	lineComment, blockComment, attributeItem, innerAttributeItem, attribute, tokenTree,
	shebang, emptyStatement treesitter.Kind

	// The items.
	modItem, structItem, enumItem, unionItem, traitItem, implItem, functionItem,
	functionSignatureItem, constItem, staticItem, typeItem, associatedType, useDeclaration,
	externCrateDeclaration, foreignModItem treesitter.Kind

	// The members of a struct and an enum.
	fieldDeclaration, enumVariant treesitter.Kind

	// Names and modifiers.
	identifier, typeIdentifier, scopedIdentifier, scopedTypeIdentifier, self, visibilityModifier,
	functionModifiers, mutableSpecifier treesitter.Kind

	// Signatures and the function bodies the sweep passes over.
	parameter, selfParameter, variadicParameter, typeParameter, lifetimeParameter, constParameter,
	lifetime, whereClause, block treesitter.Kind

	// Types.
	genericType, referenceType, arrayType, tupleType, functionType, dynamicType, abstractType,
	boundedType, useBounds treesitter.Kind

	// Use trees.
	scopedUseList, useList, useAsClause, useWildcard treesitter.Kind

	// Literals.
	stringLiteral, rawStringLiteral, integerLiteral, floatLiteral, booleanLiteral treesitter.Kind

	// The fields the lowering reads children by.
	fieldName, fieldBody, fieldType, fieldTypeParameters, fieldParameters, fieldReturnType,
	fieldTrait, fieldBounds, fieldValue, fieldAlias, fieldArgument, fieldPath, fieldList,
	fieldPattern, fieldElement, fieldLength, fieldArguments, fieldDefaultType, fieldTypeArguments,
	fieldLeft, fieldInner, fieldOuter treesitter.Field
}

// newVocabulary resolves the vocabulary from the Rust grammar. It
// panics on a kind or field the grammar does not declare, through
// [treesitter.Grammar.Kind] and [treesitter.Grammar.Field].
func newVocabulary(g *treesitter.Grammar) *vocabulary {
	return &vocabulary{
		grammar: g,

		lineComment:        g.Kind("line_comment"),
		blockComment:       g.Kind("block_comment"),
		attributeItem:      g.Kind("attribute_item"),
		innerAttributeItem: g.Kind("inner_attribute_item"),
		attribute:          g.Kind("attribute"),
		tokenTree:          g.Kind("token_tree"),
		shebang:            g.Kind("shebang"),
		emptyStatement:     g.Kind("empty_statement"),

		modItem:                g.Kind("mod_item"),
		structItem:             g.Kind("struct_item"),
		enumItem:               g.Kind("enum_item"),
		unionItem:              g.Kind("union_item"),
		traitItem:              g.Kind("trait_item"),
		implItem:               g.Kind("impl_item"),
		functionItem:           g.Kind("function_item"),
		functionSignatureItem:  g.Kind("function_signature_item"),
		constItem:              g.Kind("const_item"),
		staticItem:             g.Kind("static_item"),
		typeItem:               g.Kind("type_item"),
		associatedType:         g.Kind("associated_type"),
		useDeclaration:         g.Kind("use_declaration"),
		externCrateDeclaration: g.Kind("extern_crate_declaration"),
		foreignModItem:         g.Kind("foreign_mod_item"),

		fieldDeclaration: g.Kind("field_declaration"),
		enumVariant:      g.Kind("enum_variant"),

		identifier:           g.Kind("identifier"),
		typeIdentifier:       g.Kind("type_identifier"),
		scopedIdentifier:     g.Kind("scoped_identifier"),
		scopedTypeIdentifier: g.Kind("scoped_type_identifier"),
		self:                 g.Kind("self"),
		visibilityModifier:   g.Kind("visibility_modifier"),
		functionModifiers:    g.Kind("function_modifiers"),
		mutableSpecifier:     g.Kind("mutable_specifier"),

		parameter:         g.Kind("parameter"),
		selfParameter:     g.Kind("self_parameter"),
		variadicParameter: g.Kind("variadic_parameter"),
		typeParameter:     g.Kind("type_parameter"),
		lifetimeParameter: g.Kind("lifetime_parameter"),
		constParameter:    g.Kind("const_parameter"),
		lifetime:          g.Kind("lifetime"),
		whereClause:       g.Kind("where_clause"),
		block:             g.Kind("block"),

		genericType:   g.Kind("generic_type"),
		referenceType: g.Kind("reference_type"),
		arrayType:     g.Kind("array_type"),
		tupleType:     g.Kind("tuple_type"),
		functionType:  g.Kind("function_type"),
		dynamicType:   g.Kind("dynamic_type"),
		abstractType:  g.Kind("abstract_type"),
		boundedType:   g.Kind("bounded_type"),
		useBounds:     g.Kind("use_bounds"),

		scopedUseList: g.Kind("scoped_use_list"),
		useList:       g.Kind("use_list"),
		useAsClause:   g.Kind("use_as_clause"),
		useWildcard:   g.Kind("use_wildcard"),

		stringLiteral:    g.Kind("string_literal"),
		rawStringLiteral: g.Kind("raw_string_literal"),
		integerLiteral:   g.Kind("integer_literal"),
		floatLiteral:     g.Kind("float_literal"),
		booleanLiteral:   g.Kind("boolean_literal"),

		fieldName:           g.Field("name"),
		fieldBody:           g.Field("body"),
		fieldType:           g.Field("type"),
		fieldTypeParameters: g.Field("type_parameters"),
		fieldParameters:     g.Field("parameters"),
		fieldReturnType:     g.Field("return_type"),
		fieldTrait:          g.Field("trait"),
		fieldBounds:         g.Field("bounds"),
		fieldValue:          g.Field("value"),
		fieldAlias:          g.Field("alias"),
		fieldArgument:       g.Field("argument"),
		fieldPath:           g.Field("path"),
		fieldList:           g.Field("list"),
		fieldPattern:        g.Field("pattern"),
		fieldElement:        g.Field("element"),
		fieldLength:         g.Field("length"),
		fieldArguments:      g.Field("arguments"),
		fieldDefaultType:    g.Field("default_type"),
		fieldTypeArguments:  g.Field("type_arguments"),
		fieldLeft:           g.Field("left"),
		fieldInner:          g.Field("inner"),
		fieldOuter:          g.Field("outer"),
	}
}
