// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix opens every refusal of the package. It is the identity of
// the language, as the refusals of every satellite open.
const refusalPrefix = string(graphql.Lang) + ": "

// reservedPrefix opens the names that the specification of GraphQL
// reserves for its introspection, such as __typename.
const reservedPrefix = "__"

// caser is the case conversion of GraphQL. The guide to naming of
// graphql.org writes an abbreviation as one word, as userId, so the
// conversion recognises no initialism. A name that the source declares in
// its style, such as imageUrl, keeps its spelling.
var caser = naming.New()

// Name spells one declared name in the convention of the guide to naming
// of graphql.org. The kinds take these spellings:
//
//   - an object type, an interface, an input object, an enum, a union, a
//     scalar, a sum and the function of an operation or a fragment take
//     Pascal case;
//   - a field, a field with arguments, an argument, a named result and a
//     sum variant take camel case;
//   - an enum value takes screaming snake case;
//   - a type parameter keeps its spelling.
//
// The conversion raises no initialism, so user_id becomes userId, and a
// name in its style, such as imageUrl, keeps its spelling.
//
// The lowering of the backend gives the interface of a directive definition
// a name that starts with [graphql.DirectiveSigil], and such a name takes
// camel case after its sigil. The kind of the host does not change a
// spelling.
//
// A name at a visibility other than public and the unstated one is refused,
// because a schema exposes every name that it declares. A name that is not
// a GraphQL name is refused, and so is a name that starts with two
// underscores, which GraphQL reserves for its introspection. A name whose
// spelling comes out empty, such as "_", is refused too. A type whose
// spelling is the name of a built-in scalar, such as String for string, is
// refused, because a schema does not declare a built-in scalar.
//
// # Allocation contract
//
// Name allocates a converted spelling, one allocation, and does not
// allocate for a name that the convention spells as it is. The name of a
// directive allocates the join with its sigil as well. A refusal allocates
// its error.
func Name(_, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
	if v != symbol.VisibilityUnknown && v != symbol.VisibilityPublic {
		return "", fmt.Errorf(refusalPrefix+"%s is not public, and a schema exposes every name that it declares", name)
	}
	bare, directive := name, false
	if kind == symbol.KindInterface {
		bare, directive = strings.CutPrefix(name, graphql.DirectiveSigil)
	}
	if !naming.IsIdentifier(bare) {
		return "", fmt.Errorf(refusalPrefix+"%q is not a GraphQL name", name)
	}
	if strings.HasPrefix(bare, reservedPrefix) {
		return "", fmt.Errorf(refusalPrefix+"%q starts with two underscores, which GraphQL reserves for "+
			"its introspection", name)
	}
	var spelled string
	switch {
	case directive:
		spelled = caser.Camel(bare)
	case kind == symbol.KindTypeParam:
		spelled = bare
	case kind == symbol.KindStruct || kind == symbol.KindInterface || kind == symbol.KindEnum ||
		kind == symbol.KindAlias || kind == symbol.KindSum:
		spelled = caser.Pascal(bare)
		if graphql.IsBuiltin(spelled) {
			return "", fmt.Errorf(refusalPrefix+"%q spells %s, a built-in scalar of GraphQL, "+
				"which a schema does not declare", name, spelled)
		}
	case kind == symbol.KindFunction:
		spelled = caser.Pascal(bare)
	case kind == symbol.KindEnumVariant:
		spelled = caser.ScreamingSnake(bare)
	default:
		spelled = caser.Camel(bare)
	}
	if spelled == "" {
		return "", fmt.Errorf(refusalPrefix+"%q spells nothing in the convention of GraphQL", name)
	}
	if directive {
		return graphql.DirectiveSigil + spelled, nil
	}
	return spelled, nil
}
