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
// type parameter keeps its spelling, whose single-capital
// convention is near universal. Visibility never changes a
// spelling, because TypeScript scopes through keywords.
//
// A name that is not an identifier — a wire name like
// content-type, a digit-led key — passes through unchanged, so a
// convention cannot silently rename what a consumer matches by
// string; the backend quotes it where TypeScript admits a quoted
// key, on properties and method members, and refuses it where no
// quoted form exists, on parameters. A spelling that comes out a
// reserved word refuses, because the declaration it would produce
// does not parse.
func Name(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	if !identifier(name) {
		return name, nil
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
	if reserved[spelt] {
		return "", fmt.Errorf(
			"typescript: %s spells the reserved word %s, which no "+
				"declaration may take", name, spelt,
		)
	}
	return spelt, nil
}

// identifier reports whether a name spells bare in TypeScript:
// a letter, underscore or dollar first, those and digits after.
func identifier(name string) bool {
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

// reserved is the word set TypeScript refuses as a declaration
// name: the ECMAScript keywords plus the strict-mode set, which
// generated modules always run under.
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
