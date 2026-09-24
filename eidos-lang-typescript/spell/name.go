// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in TypeScript's convention: types
// and their variants take Pascal case, a constant takes screaming
// snake case, everything callable or bound takes camel case, and a
// type parameter keeps its spelling, whose single-capital convention
// is near universal. Visibility never changes a spelling, because
// TypeScript scopes through keywords.
//
// A name that is not an identifier, such as the wire name
// content-type or a digit-led key, passes through unchanged on a
// field, a method, an enum variant and a parameter, so a convention
// cannot rename what a consumer matches by string. The backend
// quotes it on a property, a method member and an enum member, and
// refuses it on a parameter, which has no quoted form. Every other
// kind has no quoted form either, so Name refuses a non-identifier
// on it. A spelling that is a reserved word is refused, because the
// declaration it produces does not parse.
func Name(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	if !IsIdentifier(name) {
		if quotable(kind) {
			return name, nil
		}
		return "", fmt.Errorf(
			"typescript: %q is not an identifier, and a %s name has no quoted form", name, kind,
		)
	}

	spelt := name
	switch kind {
	case symbol.KindTypeParam:
	case symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
		symbol.KindEnum, symbol.KindEnumVariant,
		symbol.KindSum, symbol.KindSumVariant:
		spelt = naming.Pascal(name)
	case symbol.KindConstant:
		spelt = naming.ScreamingSnake(name)
	default:
		spelt = naming.Camel(name)
	}
	switch {
	case reserved[spelt]:
		return "", fmt.Errorf(
			"typescript: %s spells the reserved word %s, which no declaration may take", name, spelt,
		)
	case !member(kind) && bindingReserved[spelt]:
		return "", fmt.Errorf(
			"typescript: %s spells %s, which a module binds nowhere", name, spelt,
		)
	case typeDeclaration(kind) && predefinedTypes[spelt]:
		return "", fmt.Errorf(
			"typescript: %s spells the predefined type %s, which no type declaration may take",
			name, spelt,
		)
	}
	return spelt, nil
}

// IsIdentifier reports whether a name spells bare in TypeScript: an
// ASCII letter, an underscore or a dollar first, those and digits
// after.
func IsIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		head := r == '_' || r == '$' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if head || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// quotable reports whether a kind's name passes to the backend,
// which quotes it or refuses it itself: the members, whose keys
// TypeScript admits quoted, and a parameter.
func quotable(kind symbol.Kind) bool {
	return member(kind) || kind == symbol.KindParam
}

// member reports whether a kind's name is a member key, which
// TypeScript reads as a property name and not as a binding.
func member(kind symbol.Kind) bool {
	return kind == symbol.KindField || kind == symbol.KindMethod || kind == symbol.KindEnumVariant
}

// typeDeclaration reports whether a kind declares a type name.
func typeDeclaration(kind symbol.Kind) bool {
	switch kind {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
		symbol.KindEnum, symbol.KindSum, symbol.KindTypeParam:
		return true
	default:
		return false
	}
}

// reserved is the word set TypeScript refuses as a declaration name:
// the ECMAScript reserved words, and the words strict mode reserves,
// which a generated module always runs under.
var reserved = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true,
	"const": true, "continue": true, "debugger": true, "default": true,
	"delete": true, "do": true, "else": true, "enum": true,
	"export": true, "extends": true, "false": true, "finally": true,
	"for": true, "function": true, "if": true, "import": true,
	"in": true, "instanceof": true, "new": true, "null": true,
	"return": true, "super": true, "switch": true, "this": true,
	"throw": true, "true": true, "try": true, "typeof": true,
	"var": true, "void": true, "while": true, "with": true,
	"implements": true, "interface": true, "let": true,
	"package": true, "private": true, "protected": true,
	"public": true, "static": true, "yield": true,
}

// bindingReserved is the word set a module refuses as a binding name
// only: await, which module code reserves, and eval and arguments,
// which strict mode refuses to bind. A member key may take any of
// them.
var bindingReserved = map[string]bool{
	"await": true, "eval": true, "arguments": true,
}

// predefinedTypes is the word set TypeScript refuses as the name of a
// class, an interface, an enum, a type alias or a type parameter:
// the predefined type names.
var predefinedTypes = map[string]bool{
	"any": true, "bigint": true, "boolean": true, "never": true,
	"number": true, "object": true, "string": true, "symbol": true,
	"undefined": true, "unknown": true, "void": true,
}
