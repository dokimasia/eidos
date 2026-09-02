// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in Rust's convention: types and
// their variants take Pascal case, a constant takes screaming
// snake case, everything callable or bound takes snake case, and a
// type parameter keeps its spelling, so a const parameter's N
// stands. Visibility never changes a spelling, because Rust scopes
// through pub. A name outside the identifier shape refuses,
// because a convention must not respell what a consumer matches by
// string; a spelling landing on a keyword takes Rust's own raw
// form, r#name, except the few spellings the raw form cannot
// carry, which refuse.
func Name(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	if !naming.IsIdentifier(name) {
		return "", fmt.Errorf(
			"rust: %q is outside the identifier shape, and a convention "+
				"must not respell a wire name", name,
		)
	}
	var out string
	switch kind {
	case symbol.KindTypeParam:
		out = name
	case symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
		symbol.KindEnum, symbol.KindEnumVariant,
		symbol.KindSum, symbol.KindSumVariant:
		out = naming.Pascal(name)
	case symbol.KindConstant:
		out = naming.ScreamingSnake(name)
	default:
		out = naming.Snake(name)
	}
	if keywords[out] {
		if unraw[out] {
			return "", fmt.Errorf(
				"rust: %q lands on a keyword the raw form cannot carry", out,
			)
		}
		return "r#" + out, nil
	}
	return out, nil
}

// keywords holds Rust's reserved words, spelled only through the
// raw form.
var keywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true,
	"const": true, "continue": true, "crate": true, "dyn": true,
	"else": true, "enum": true, "extern": true, "false": true,
	"fn": true, "for": true, "if": true, "impl": true, "in": true,
	"let": true, "loop": true, "match": true, "mod": true,
	"move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true,
	"struct": true, "super": true, "trait": true, "true": true,
	"type": true, "unsafe": true, "use": true, "where": true,
	"while": true,
}

// unraw holds the keywords the raw form cannot spell, per Rust's
// own rule: r#self, r#Self, r#super, r#crate are rejected by
// rustc.
var unraw = map[string]bool{
	"self": true, "Self": true, "super": true, "crate": true,
}
