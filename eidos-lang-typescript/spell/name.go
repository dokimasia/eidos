// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang/naming"
)

// Name spells one declared name in TypeScript's convention: types
// and their variants take Pascal case, a constant takes screaming
// snake case, everything callable or bound takes camel case, and a
// type parameter keeps its spelling, whose single-capital
// convention is near universal. Visibility never changes a
// spelling, because TypeScript scopes through keywords, and no
// name refuses.
func Name(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	switch kind {
	case symbol.KindTypeParam:
		return name, nil
	case symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
		symbol.KindEnum, symbol.KindEnumVariant,
		symbol.KindSum, symbol.KindSumVariant:
		return naming.Pascal(name), nil
	case symbol.KindConstant:
		return naming.ScreamingSnake(name), nil
	default:
		return naming.Camel(name), nil
	}
}
