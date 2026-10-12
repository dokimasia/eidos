// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// resolve returns the candidates of a spelling in one file as one tier.
// The candidates are the declarations that the [golang.Bindings] of the
// file probe. Go refuses to compile a file in which two of them declare
// the name, and the resolution step reports that case as an ambiguity.
// A file whose parse recorded no bindings probes its own package alone.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	b, _ := scope.Bindings.(golang.Bindings)
	tier := b.Candidates(scope.File.Package, spelling)
	if len(tier) == 0 {
		return nil
	}
	return plugin.Candidates{tier}
}
