// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/lang/treesitter"

// vocabulary is the node kinds and the field names of the grammar that the
// lowering compares. The frontend resolves them once when it is built, so a
// walk compares ids, and a grammar that renames a kind fails at
// construction.
type vocabulary struct {
	grammar *treesitter.Grammar
	// comments is the comment walk of the grammar, which the lowering of
	// each file copies.
	comments treesitter.Comments

	// The containers of statements and the statements that contain
	// declarations.
	block, comment, decoratedDefinition, decorator, classDefinition, functionDefinition,
	assignment, augmentedAssignment, typeAliasStatement, ifStatement, elifClause, elseClause,
	tryStatement, exceptClause, finallyClause, passStatement treesitter.Kind

	// The statements whose bodies the self assignments of a method and the
	// yields of a callable can be in.
	forStatement, whileStatement, withStatement, matchStatement, caseClause, lambda, yield treesitter.Kind

	// The module boundary.
	importStatement, importFromStatement, futureImportStatement, lazyImportStatement, aliasedImport,
	dottedName, relativeImport, importPrefix, wildcardImport treesitter.Kind

	// Names, literals and the expressions that a declaration reads.
	identifier, attribute, subscript, call, keywordArgument, listSplat, dictionarySplat,
	stringNode, stringStart, concatenatedString, integer, float, trueNode, falseNode, none, ellipsis, list, tuple,
	pair, unaryOperator, binaryOperator, comparisonOperator, booleanOperator, notOperator,
	parenthesizedExpression, slice, patternList treesitter.Kind

	// Signatures.
	typedParameter, typedDefaultParameter, defaultParameter, listSplatPattern,
	dictionarySplatPattern, positionalSeparator, keywordSeparator, tuplePattern treesitter.Kind

	// Types.
	typeNode, typeParameter, constrainedType, memberType, genericType, unionType,
	defaultType treesitter.Kind

	// The fields by which the lowering reads children.
	fieldName, fieldAlias, fieldModuleName, fieldBody, fieldSuperclasses, fieldTypeParameters,
	fieldParameters, fieldReturnType, fieldDefinition, fieldLeft, fieldRight, fieldType, fieldValue,
	fieldCondition, fieldConsequence, fieldAlternative, fieldObject, fieldAttribute, fieldFunction,
	fieldArguments, fieldSubscript, fieldOperators, fieldOperator, fieldArgument, fieldKey treesitter.Field
}

// newVocabulary resolves the vocabulary of one grammar, and the comment
// walk over its kinds. It panics on a kind or a field that the grammar does
// not declare, through [treesitter.Grammar.Kind] and
// [treesitter.Grammar.Field].
func newVocabulary(g *treesitter.Grammar) *vocabulary {
	v := &vocabulary{
		grammar: g,

		block:               g.Kind("block"),
		comment:             g.Kind("comment"),
		decoratedDefinition: g.Kind("decorated_definition"),
		decorator:           g.Kind("decorator"),
		classDefinition:     g.Kind("class_definition"),
		functionDefinition:  g.Kind("function_definition"),
		assignment:          g.Kind("assignment"),
		augmentedAssignment: g.Kind("augmented_assignment"),
		typeAliasStatement:  g.Kind("type_alias_statement"),
		ifStatement:         g.Kind("if_statement"),
		elifClause:          g.Kind("elif_clause"),
		elseClause:          g.Kind("else_clause"),
		tryStatement:        g.Kind("try_statement"),
		exceptClause:        g.Kind("except_clause"),
		finallyClause:       g.Kind("finally_clause"),
		passStatement:       g.Kind("pass_statement"),

		forStatement:   g.Kind("for_statement"),
		whileStatement: g.Kind("while_statement"),
		withStatement:  g.Kind("with_statement"),
		matchStatement: g.Kind("match_statement"),
		caseClause:     g.Kind("case_clause"),
		lambda:         g.Kind("lambda"),
		yield:          g.Kind("yield"),

		importStatement:       g.Kind("import_statement"),
		importFromStatement:   g.Kind("import_from_statement"),
		futureImportStatement: g.Kind("future_import_statement"),
		lazyImportStatement:   g.Kind("lazy_import_statement"),
		aliasedImport:         g.Kind("aliased_import"),
		dottedName:            g.Kind("dotted_name"),
		relativeImport:        g.Kind("relative_import"),
		importPrefix:          g.Kind("import_prefix"),
		wildcardImport:        g.Kind("wildcard_import"),

		identifier:              g.Kind("identifier"),
		attribute:               g.Kind("attribute"),
		subscript:               g.Kind("subscript"),
		call:                    g.Kind("call"),
		keywordArgument:         g.Kind("keyword_argument"),
		listSplat:               g.Kind("list_splat"),
		dictionarySplat:         g.Kind("dictionary_splat"),
		stringNode:              g.Kind("string"),
		stringStart:             g.Kind("string_start"),
		concatenatedString:      g.Kind("concatenated_string"),
		integer:                 g.Kind("integer"),
		float:                   g.Kind("float"),
		trueNode:                g.Kind("true"),
		falseNode:               g.Kind("false"),
		none:                    g.Kind("none"),
		ellipsis:                g.Kind("ellipsis"),
		list:                    g.Kind("list"),
		tuple:                   g.Kind("tuple"),
		pair:                    g.Kind("pair"),
		unaryOperator:           g.Kind("unary_operator"),
		binaryOperator:          g.Kind("binary_operator"),
		comparisonOperator:      g.Kind("comparison_operator"),
		booleanOperator:         g.Kind("boolean_operator"),
		notOperator:             g.Kind("not_operator"),
		parenthesizedExpression: g.Kind("parenthesized_expression"),
		slice:                   g.Kind("slice"),
		patternList:             g.Kind("pattern_list"),

		typedParameter:         g.Kind("typed_parameter"),
		typedDefaultParameter:  g.Kind("typed_default_parameter"),
		defaultParameter:       g.Kind("default_parameter"),
		listSplatPattern:       g.Kind("list_splat_pattern"),
		dictionarySplatPattern: g.Kind("dictionary_splat_pattern"),
		positionalSeparator:    g.Kind("positional_separator"),
		keywordSeparator:       g.Kind("keyword_separator"),
		tuplePattern:           g.Kind("tuple_pattern"),

		typeNode:        g.Kind("type"),
		typeParameter:   g.Kind("type_parameter"),
		constrainedType: g.Kind("constrained_type"),
		memberType:      g.Kind("member_type"),
		genericType:     g.Kind("generic_type"),
		unionType:       g.Kind("union_type"),
		defaultType:     g.Kind("default_type"),

		fieldName:           g.Field("name"),
		fieldAlias:          g.Field("alias"),
		fieldModuleName:     g.Field("module_name"),
		fieldBody:           g.Field("body"),
		fieldSuperclasses:   g.Field("superclasses"),
		fieldTypeParameters: g.Field("type_parameters"),
		fieldParameters:     g.Field("parameters"),
		fieldReturnType:     g.Field("return_type"),
		fieldDefinition:     g.Field("definition"),
		fieldLeft:           g.Field("left"),
		fieldRight:          g.Field("right"),
		fieldType:           g.Field("type"),
		fieldValue:          g.Field("value"),
		fieldCondition:      g.Field("condition"),
		fieldConsequence:    g.Field("consequence"),
		fieldAlternative:    g.Field("alternative"),
		fieldObject:         g.Field("object"),
		fieldAttribute:      g.Field("attribute"),
		fieldFunction:       g.Field("function"),
		fieldArguments:      g.Field("arguments"),
		fieldSubscript:      g.Field("subscript"),
		fieldOperators:      g.Field("operators"),
		fieldOperator:       g.Field("operator"),
		fieldArgument:       g.Field("argument"),
		fieldKey:            g.Field("key"),
	}
	v.comments = commentWalk(v)
	return v
}
