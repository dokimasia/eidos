// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/lang/treesitter"

// vocabulary is one grammar's node kinds and field names that the
// lowering compares, resolved once when the frontend is built, so a
// walk compares ids and a grammar that renames a kind fails at
// construction. The TypeScript and TSX grammars assign their ids
// apart, so each has its own vocabulary.
type vocabulary struct {
	grammar *treesitter.Grammar

	// The statements and the containers a module's declarations are in.
	program, comment, exportStatement, ambientDeclaration, expressionStatement,
	statementBlock, internalModule, module, nestedIdentifier treesitter.Kind

	// The declarations.
	classDeclaration, abstractClassDeclaration, class, classBody, classHeritage,
	extendsClause, implementsClause, interfaceDeclaration, interfaceBody,
	extendsTypeClause, enumDeclaration, enumBody, enumAssignment,
	typeAliasDeclaration, functionDeclaration, generatorFunctionDeclaration,
	functionSignature, functionExpression, generatorFunction, lexicalDeclaration,
	variableDeclaration, variableDeclarator treesitter.Kind

	// The members of a class, an interface and an object type.
	methodDefinition, methodSignature, abstractMethodSignature,
	publicFieldDefinition, propertySignature, indexSignature, constructSignature,
	callSignature, accessibilityModifier, overrideModifier treesitter.Kind

	// The names a declaration or a member spells.
	identifier, propertyIdentifier, privatePropertyIdentifier, typeIdentifier,
	stringNode, stringFragment, computedPropertyName treesitter.Kind

	// The module boundary.
	importStatement, importClause, namedImports, importSpecifier,
	namespaceImport, importRequireClause, importAlias, exportClause,
	exportSpecifier, namespaceExport treesitter.Kind

	// Decorators and the expressions they call.
	decorator, callExpression, memberExpression treesitter.Kind

	// Signatures.
	formalParameters, requiredParameter, optionalParameter, restPattern, this,
	typeAnnotation, typePredicateAnnotation, assertsAnnotation, typeParameters,
	typeParameter, constraint, defaultType, typeArguments treesitter.Kind

	// Types.
	predefinedType, genericType, nestedTypeIdentifier, arrayType, tupleType,
	optionalType, unionType, intersectionType, literalType, undefined, null,
	functionType, constructorType, readonlyType, objectType, parenthesizedType,
	mappedTypeClause treesitter.Kind

	// The fields the lowering reads children by.
	fieldName, fieldBody, fieldDeclaration, fieldValue, fieldSource, fieldAlias,
	fieldTypeParameters, fieldParameters, fieldReturnType, fieldType,
	fieldDecorator, fieldPattern, fieldIndexType, fieldConstraint,
	fieldFunction, fieldArguments, fieldTypeArguments, fieldObject,
	fieldProperty, fieldModule treesitter.Field
}

// newVocabulary resolves a vocabulary from one grammar. It panics on a
// kind or field the grammar does not declare, through
// [treesitter.Grammar.Kind] and [treesitter.Grammar.Field].
func newVocabulary(g *treesitter.Grammar) *vocabulary {
	return &vocabulary{
		grammar: g,

		program:             g.Kind("program"),
		comment:             g.Kind("comment"),
		exportStatement:     g.Kind("export_statement"),
		ambientDeclaration:  g.Kind("ambient_declaration"),
		expressionStatement: g.Kind("expression_statement"),
		statementBlock:      g.Kind("statement_block"),
		internalModule:      g.Kind("internal_module"),
		module:              g.Kind("module"),
		nestedIdentifier:    g.Kind("nested_identifier"),

		classDeclaration:             g.Kind("class_declaration"),
		abstractClassDeclaration:     g.Kind("abstract_class_declaration"),
		class:                        g.Kind("class"),
		classBody:                    g.Kind("class_body"),
		classHeritage:                g.Kind("class_heritage"),
		extendsClause:                g.Kind("extends_clause"),
		implementsClause:             g.Kind("implements_clause"),
		interfaceDeclaration:         g.Kind("interface_declaration"),
		interfaceBody:                g.Kind("interface_body"),
		extendsTypeClause:            g.Kind("extends_type_clause"),
		enumDeclaration:              g.Kind("enum_declaration"),
		enumBody:                     g.Kind("enum_body"),
		enumAssignment:               g.Kind("enum_assignment"),
		typeAliasDeclaration:         g.Kind("type_alias_declaration"),
		functionDeclaration:          g.Kind("function_declaration"),
		generatorFunctionDeclaration: g.Kind("generator_function_declaration"),
		functionSignature:            g.Kind("function_signature"),
		functionExpression:           g.Kind("function_expression"),
		generatorFunction:            g.Kind("generator_function"),
		lexicalDeclaration:           g.Kind("lexical_declaration"),
		variableDeclaration:          g.Kind("variable_declaration"),
		variableDeclarator:           g.Kind("variable_declarator"),

		methodDefinition:        g.Kind("method_definition"),
		methodSignature:         g.Kind("method_signature"),
		abstractMethodSignature: g.Kind("abstract_method_signature"),
		publicFieldDefinition:   g.Kind("public_field_definition"),
		propertySignature:       g.Kind("property_signature"),
		indexSignature:          g.Kind("index_signature"),
		constructSignature:      g.Kind("construct_signature"),
		callSignature:           g.Kind("call_signature"),
		accessibilityModifier:   g.Kind("accessibility_modifier"),
		overrideModifier:        g.Kind("override_modifier"),

		identifier:                g.Kind("identifier"),
		propertyIdentifier:        g.Kind("property_identifier"),
		privatePropertyIdentifier: g.Kind("private_property_identifier"),
		typeIdentifier:            g.Kind("type_identifier"),
		stringNode:                g.Kind("string"),
		stringFragment:            g.Kind("string_fragment"),
		computedPropertyName:      g.Kind("computed_property_name"),

		importStatement:     g.Kind("import_statement"),
		importClause:        g.Kind("import_clause"),
		namedImports:        g.Kind("named_imports"),
		importSpecifier:     g.Kind("import_specifier"),
		namespaceImport:     g.Kind("namespace_import"),
		importRequireClause: g.Kind("import_require_clause"),
		importAlias:         g.Kind("import_alias"),
		exportClause:        g.Kind("export_clause"),
		exportSpecifier:     g.Kind("export_specifier"),
		namespaceExport:     g.Kind("namespace_export"),

		decorator:        g.Kind("decorator"),
		callExpression:   g.Kind("call_expression"),
		memberExpression: g.Kind("member_expression"),

		formalParameters:        g.Kind("formal_parameters"),
		requiredParameter:       g.Kind("required_parameter"),
		optionalParameter:       g.Kind("optional_parameter"),
		restPattern:             g.Kind("rest_pattern"),
		this:                    g.Kind("this"),
		typeAnnotation:          g.Kind("type_annotation"),
		typePredicateAnnotation: g.Kind("type_predicate_annotation"),
		assertsAnnotation:       g.Kind("asserts_annotation"),
		typeParameters:          g.Kind("type_parameters"),
		typeParameter:           g.Kind("type_parameter"),
		constraint:              g.Kind("constraint"),
		defaultType:             g.Kind("default_type"),
		typeArguments:           g.Kind("type_arguments"),

		predefinedType:       g.Kind("predefined_type"),
		genericType:          g.Kind("generic_type"),
		nestedTypeIdentifier: g.Kind("nested_type_identifier"),
		arrayType:            g.Kind("array_type"),
		tupleType:            g.Kind("tuple_type"),
		optionalType:         g.Kind("optional_type"),
		unionType:            g.Kind("union_type"),
		intersectionType:     g.Kind("intersection_type"),
		literalType:          g.Kind("literal_type"),
		undefined:            g.Kind("undefined"),
		null:                 g.Kind("null"),
		functionType:         g.Kind("function_type"),
		constructorType:      g.Kind("constructor_type"),
		readonlyType:         g.Kind("readonly_type"),
		objectType:           g.Kind("object_type"),
		parenthesizedType:    g.Kind("parenthesized_type"),
		mappedTypeClause:     g.Kind("mapped_type_clause"),

		fieldName:           g.Field("name"),
		fieldBody:           g.Field("body"),
		fieldDeclaration:    g.Field("declaration"),
		fieldValue:          g.Field("value"),
		fieldSource:         g.Field("source"),
		fieldAlias:          g.Field("alias"),
		fieldTypeParameters: g.Field("type_parameters"),
		fieldParameters:     g.Field("parameters"),
		fieldReturnType:     g.Field("return_type"),
		fieldType:           g.Field("type"),
		fieldDecorator:      g.Field("decorator"),
		fieldPattern:        g.Field("pattern"),
		fieldIndexType:      g.Field("index_type"),
		fieldConstraint:     g.Field("constraint"),
		fieldFunction:       g.Field("function"),
		fieldArguments:      g.Field("arguments"),
		fieldTypeArguments:  g.Field("type_arguments"),
		fieldObject:         g.Field("object"),
		fieldProperty:       g.Field("property"),
		fieldModule:         g.Field("module"),
	}
}
