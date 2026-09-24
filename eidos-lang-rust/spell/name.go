// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in Rust's convention: types and
// their variants take Pascal case, a constant takes screaming snake
// case, everything callable or bound takes snake case, and a type
// parameter keeps its spelling, so a const parameter's N keeps its
// case. Visibility never changes a spelling, because Rust scopes
// through pub.
//
// A name outside the identifier shape is refused, because a
// convention must not respell what a consumer matches by string, and
// so is a name whose spelling comes out empty, such as "_" in snake
// case. A spelling that is a strict or reserved keyword takes Rust's
// raw form, r#name, except the keywords the raw form cannot express,
// which are refused.
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
	switch {
	case out == "":
		return "", fmt.Errorf("rust: %q spells nothing in Rust's convention", name)
	case unraw[out]:
		return "", fmt.Errorf(
			"rust: %q is a keyword the raw form cannot express", out,
		)
	case keywords[out]:
		return "r#" + out, nil
	}
	return out, nil
}

// keywords is the set of Rust's strict and reserved keywords, which
// a name spells only through the raw form: the strict keywords of
// every edition, the reserved keywords, try from the 2018 edition
// and gen from the 2024 edition.
var keywords = map[string]bool{
	"_": true, "as": true, "async": true, "await": true, "break": true,
	"const": true, "continue": true, "crate": true, "dyn": true,
	"else": true, "enum": true, "extern": true, "false": true,
	"fn": true, "for": true, "if": true, "impl": true, "in": true,
	"let": true, "loop": true, "match": true, "mod": true,
	"move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true,
	"struct": true, "super": true, "trait": true, "true": true,
	"type": true, "unsafe": true, "use": true, "where": true, "while": true,
	"abstract": true, "become": true, "box": true, "do": true,
	"final": true, "macro": true, "override": true, "priv": true,
	"typeof": true, "unsized": true, "virtual": true, "yield": true,
	"try": true, "gen": true,
}

// unraw is the set of keywords the raw form cannot spell: rustc
// rejects r#_, r#crate, r#self, r#Self and r#super.
var unraw = map[string]bool{
	"_": true, "crate": true, "self": true, "Self": true, "super": true,
}
