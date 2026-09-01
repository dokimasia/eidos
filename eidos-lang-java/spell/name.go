// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in Java's convention: types take
// Pascal case, enum constants and file-level constants take
// screaming snake case, and so does a field inside an interface,
// because Java reads it as a constant; everything else callable or
// bound takes camel case, and a type parameter keeps its spelling,
// whose single-capital convention is near universal. Visibility
// never changes a spelling, because Java scopes through keywords,
// and no name refuses.
func Name(host, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	switch {
	case kind == symbol.KindTypeParam:
		return name, nil
	case kind == symbol.KindField && host == symbol.KindInterface:
		return naming.ScreamingSnake(name), nil
	case kind == symbol.KindConstant || kind == symbol.KindEnumVariant:
		return naming.ScreamingSnake(name), nil
	case kind == symbol.KindStruct || kind == symbol.KindInterface ||
		kind == symbol.KindAlias || kind == symbol.KindEnum ||
		kind == symbol.KindSum || kind == symbol.KindSumVariant:
		return naming.Pascal(name), nil
	default:
		return naming.Camel(name), nil
	}
}
