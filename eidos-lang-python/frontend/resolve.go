// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/sdk/plugin"

// resolve returns what a spelling could mean in one module, in Python's
// order of scopes, through the record that the parse made for the File of
// the module. A file whose parse recorded no scope resolves nothing.
func resolve(s plugin.ImportScope, spelling string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	return sc.candidates(s.Owner, spelling)
}

// exports returns what a module publishes under a name and does not
// declare: the declaration that an import of the module binds the name to,
// and then the name in each module that it imports with a star. A file
// whose parse recorded no scope publishes nothing.
func exports(s plugin.ImportScope, name string) plugin.Candidates {
	sc, _ := s.Bindings.(*scope)
	if sc == nil {
		return nil
	}
	return sc.published(name)
}
