// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"go/token"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// blank is Go's blank identifier, which binds nothing.
const blank = "_"

// refusalPrefix opens every refusal a spelling returns: the
// language's identity, as every satellite's refusals open.
const refusalPrefix = string(golang.Lang) + ": "

// Name spells one declared name in Go's convention, which makes
// visibility a matter of case: a public or unstated scope exports
// through Pascal case, a package scope is unexported through camel
// case, and the initialisms the [naming.Default] caser recognises
// keep their shape, so httpRow exports as HTTPRow. Parameters and
// results spell camel whatever the scope, because their case states
// no visibility, and a type parameter keeps its spelling, whose
// single-capital convention is near universal. The blank identifier
// is kept as it is. A protected, private or internal scope refuses,
// because no case spells it, and so does a spelling that is no Go
// identifier: empty, led by a digit, or a keyword.
func Name(_, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
	if name == blank {
		return name, nil
	}
	var spelled string
	switch {
	case kind == symbol.KindTypeParam:
		spelled = name
	case kind == symbol.KindParam || kind == symbol.KindReturn:
		spelled = naming.Camel(name)
	case v == symbol.VisibilityUnknown || v == symbol.VisibilityPublic:
		spelled = naming.Pascal(name)
	case v == symbol.VisibilityPackage:
		spelled = naming.Camel(name)
	default:
		return "", fmt.Errorf(refusalPrefix+
			"visibility spells through the name's case, and %s states a scope no case spells", name)
	}
	if !token.IsIdentifier(spelled) {
		return "", fmt.Errorf(refusalPrefix+"%q spells as %q, which is no Go identifier", name, spelled)
	}
	return spelled, nil
}
