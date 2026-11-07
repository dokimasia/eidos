// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// resolve returns the candidates of a spelling in one file in the tiers of
// [golang.Bindings.Tiers]. The resolution step admits each declaration of a
// tier through [golang.Qualifies], which the frontend declares as its
// qualifier, so a candidate of an import without an alias names a target
// only where its package declares the qualifier. Go refuses to compile a
// file in which two admitted candidates of one tier declare the name, and
// the resolution step reports that case as an ambiguity. A file whose parse
// recorded no bindings probes its own package alone.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	b, _ := scope.Bindings.(golang.Bindings)
	return b.Tiers(scope.File.Package, spelling)
}
