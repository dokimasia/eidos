// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in Java's convention: types take
// Pascal case, enum constants and file-level constants take
// screaming snake case, and so does a field inside an interface,
// because Java reads it as a constant; everything else callable or
// bound takes camel case, and a type parameter keeps its spelling,
// whose single-capital convention is near universal. Visibility
// never changes a spelling, because Java scopes through keywords.
// Two shapes refuse: a name outside the identifier shape, because
// a convention must not respell what a consumer matches by string,
// and a spelling landing on a reserved word, which Java cannot
// escape.
func Name(host, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	if !naming.IsIdentifier(name) {
		return "", fmt.Errorf(
			"java: %q is outside the identifier shape, and a convention "+
				"must not respell a wire name", name,
		)
	}
	var out string
	switch {
	case kind == symbol.KindTypeParam:
		out = name
	case kind == symbol.KindField && host == symbol.KindInterface:
		out = naming.ScreamingSnake(name)
	case kind == symbol.KindConstant || kind == symbol.KindEnumVariant:
		out = naming.ScreamingSnake(name)
	case kind == symbol.KindStruct || kind == symbol.KindInterface ||
		kind == symbol.KindAlias || kind == symbol.KindEnum ||
		kind == symbol.KindSum || kind == symbol.KindSumVariant:
		out = naming.Pascal(name)
	default:
		out = naming.Camel(name)
	}
	if reserved[out] {
		return "", fmt.Errorf(
			"java: %q lands on a reserved word, which Java cannot escape", out,
		)
	}
	return out, nil
}

// reserved holds Java's reserved words and literals, which no
// declaration may spell: the language grants no escape the way
// Rust's raw form does.
var reserved = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true,
	"class": true, "const": true, "continue": true, "default": true,
	"do": true, "double": true, "else": true, "enum": true,
	"extends": true, "final": true, "finally": true, "float": true,
	"for": true, "goto": true, "if": true, "implements": true,
	"import": true, "instanceof": true, "int": true, "interface": true,
	"long": true, "native": true, "new": true, "package": true,
	"private": true, "protected": true, "public": true, "return": true,
	"short": true, "static": true, "strictfp": true, "super": true,
	"switch": true, "synchronized": true, "this": true, "throw": true,
	"throws": true, "transient": true, "try": true, "void": true,
	"volatile": true, "while": true,
	"true": true, "false": true, "null": true, "_": true,
}
