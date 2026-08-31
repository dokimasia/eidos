// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang/naming"
)

// Name spells one declared name in Rust's convention: types and
// their variants take Pascal case, a constant takes screaming
// snake case, everything callable or bound takes snake case, and a
// type parameter keeps its spelling, so a const parameter's N
// stands. Visibility never changes a spelling, because Rust scopes
// through pub, and no name refuses.
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
		return naming.Snake(name), nil
	}
}
