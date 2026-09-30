// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// resolve returns what a spelling could mean in one file: the
// candidates the file's [golang.Scope] probes, as one tier, because
// Go refuses to compile a file in which two of them declare the name
// and the resolution step reports that case as an ambiguity. A file
// whose parse recorded no scope probes its own package alone.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	s, _ := scope.Bindings.(golang.Scope)
	tier := s.Candidates(scope.File.Package, spelling)
	if len(tier) == 0 {
		return nil
	}
	return plugin.Candidates{tier}
}
