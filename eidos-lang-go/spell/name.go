// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Name spells one declared name in Go's convention, which is where
// visibility becomes case: a public or unstated scope exports
// through Pascal case, a package scope stays unexported through
// camel case, and the initialisms the [naming.Default] caser
// recognises keep their shape, so httpRow exports as HTTPRow.
// Parameters and results spell camel whatever the scope, because
// their case carries no visibility, and a type parameter keeps its
// spelling, whose single-capital convention is near universal. A
// protected, private or internal scope refuses: no case carries
// it.
func Name(_, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
	switch kind {
	case symbol.KindTypeParam:
		return name, nil
	case symbol.KindParam, symbol.KindReturn:
		return naming.Camel(name), nil
	}
	switch v {
	case symbol.VisibilityUnknown, symbol.VisibilityPublic:
		return naming.Pascal(name), nil
	case symbol.VisibilityPackage:
		return naming.Camel(name), nil
	default:
		return "", fmt.Errorf(
			"go: visibility spells through the name's case, and %s states a "+
				"scope no case carries", name,
		)
	}
}
