// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/lang/treesitter"

// vocabulary is the node kinds and the field names of one grammar that the
// lowering compares. The frontend resolves them once when it is built, so
// a walk compares ids, and a grammar that renames a kind fails at
// construction. The TypeScript and TSX grammars assign their ids apart, so
// each has its own vocabulary.
type vocabulary struct {
	grammar *treesitter.Grammar
	// comments is the comment walk of the grammar, which the lowering of
	// each file copies.
	comments treesitter.Comments

	// The statements, and the containers of the declarations of a module.
	comment, exportStatement, ambientDeclaration, expressionStatement,
	statementBlock, internalModule, module, nestedIdentifier treesitter.Kind

	// The declarations.
	classDeclaration, abstractClassDeclaration, class, classBody, classHeritage,
	extendsClause, implementsClause, interfaceDeclaration,
	extendsTypeClause, enumDeclaration, enumAssignment,
	typeAliasDeclaration, functionDeclaration, generatorFunctionDeclaration,
	functionSignature, functionExpression, generatorFunction, lexicalDeclaration,
	variableDeclaration, variableDeclarator treesitter.Kind

	// The members of a class, an interface and an object type.
	methodDefinition, methodSignature, abstractMethodSignature,
	publicFieldDefinition, propertySignature, indexSignature, constructSignature,
	callSignature, accessibilityModifier, overrideModifier treesitter.Kind

	// The names of a declaration or a member.
	identifier, propertyIdentifier, privatePropertyIdentifier, computedPropertyName, typeIdentifier,
	stringNode, stringFragment treesitter.Kind

	// The module boundary.
	importStatement, importClause, namedImports, importSpecifier,
	namespaceImport, importRequireClause, importAlias, exportClause,
	exportSpecifier, namespaceExport treesitter.Kind

	// Decorators, the expressions they call, and the literals of their
	// arguments.
	decorator, callExpression, memberExpression, number, trueNode, falseNode, array, object, pair,
	templateString, unaryExpression treesitter.Kind

	// Signatures.
	requiredParameter, optionalParameter, restPattern, this,
	typeAnnotation, typePredicateAnnotation, assertsAnnotation,
	typeParameter treesitter.Kind

	// Types.
	genericType, nestedTypeIdentifier, arrayType, tupleType,
	optionalType, unionType, intersectionType, literalType, undefined, null,
	functionType, constructorType, readonlyType, objectType, parenthesizedType,
	mappedTypeClause treesitter.Kind

	// The fields by which the lowering reads children.
	fieldName, fieldBody, fieldDeclaration, fieldValue, fieldSource, fieldAlias,
	fieldTypeParameters, fieldParameters, fieldReturnType, fieldType,
	fieldPattern, fieldIndexType, fieldConstraint,
	fieldFunction, fieldArguments, fieldTypeArguments, fieldObject,
	fieldProperty, fieldKey, fieldArgument, fieldOperator treesitter.Field
}

// newVocabulary resolves a vocabulary from one grammar, and the comment
// walk over its kinds. It panics on a kind or a field that the grammar
// does not declare, through [treesitter.Grammar.Kind] and
// [treesitter.Grammar.Field].
func newVocabulary(g *treesitter.Grammar) *vocabulary {
	v := &vocabulary{
		grammar: g,

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
		extendsTypeClause:            g.Kind("extends_type_clause"),
		enumDeclaration:              g.Kind("enum_declaration"),
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
		computedPropertyName:      g.Kind("computed_property_name"),
		typeIdentifier:            g.Kind("type_identifier"),
		stringNode:                g.Kind("string"),
		stringFragment:            g.Kind("string_fragment"),

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
		number:           g.Kind("number"),
		trueNode:         g.Kind("true"),
		falseNode:        g.Kind("false"),
		array:            g.Kind("array"),
		object:           g.Kind("object"),
		pair:             g.Kind("pair"),
		templateString:   g.Kind("template_string"),
		unaryExpression:  g.Kind("unary_expression"),

		requiredParameter:       g.Kind("required_parameter"),
		optionalParameter:       g.Kind("optional_parameter"),
		restPattern:             g.Kind("rest_pattern"),
		this:                    g.Kind("this"),
		typeAnnotation:          g.Kind("type_annotation"),
		typePredicateAnnotation: g.Kind("type_predicate_annotation"),
		assertsAnnotation:       g.Kind("asserts_annotation"),
		typeParameter:           g.Kind("type_parameter"),

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
		fieldPattern:        g.Field("pattern"),
		fieldIndexType:      g.Field("index_type"),
		fieldConstraint:     g.Field("constraint"),
		fieldFunction:       g.Field("function"),
		fieldArguments:      g.Field("arguments"),
		fieldTypeArguments:  g.Field("type_arguments"),
		fieldObject:         g.Field("object"),
		fieldProperty:       g.Field("property"),
		fieldKey:            g.Field("key"),
		fieldArgument:       g.Field("argument"),
		fieldOperator:       g.Field("operator"),
	}
	v.comments = commentWalk(v)
	return v
}
