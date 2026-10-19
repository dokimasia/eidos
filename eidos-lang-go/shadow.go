// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/types"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
)

// RenamePredeclared renames each parameter and each result of m whose name
// is declared in the universe scope of Go, such as append, len, error or
// true, and returns m. The new name is the name followed by the smallest
// number from 0 that m does not have, as [naming.Unique] chooses. The
// names of the receiver, the type parameters, the parameters and the
// results of m are taken first. The body of a generated method can then
// call a predeclared function, such as append in a stub, which a parameter
// of the same name would hide. The universe scope of go/types decides, so
// the set follows the toolchain that the module builds with. A nil m
// returns nil.
//
// # Allocation contract
//
// RenamePredeclared allocates nothing where no name of m is predeclared.
// Otherwise it allocates the set of the names of m and the text of each
// new name.
func RenamePredeclared(m *emit.Method) *emit.Method {
	if m == nil {
		return nil
	}
	params, returns := m.Params.Items(), m.Returns.Items()
	shadows := false
	for _, p := range params {
		shadows = shadows || types.Universe.Lookup(p.Name) != nil
	}
	for _, r := range returns {
		shadows = shadows || types.Universe.Lookup(r.Name) != nil
	}
	if !shadows {
		return m
	}
	taken := make(map[string]bool, 1+len(m.TypeParams)+len(params)+len(returns))
	if m.Receiver != nil {
		taken[m.Receiver.Name] = true
	}
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
	}
	for _, p := range params {
		taken[p.Name] = true
	}
	for _, r := range returns {
		taken[r.Name] = true
	}
	for _, p := range params {
		if types.Universe.Lookup(p.Name) != nil {
			p.Name = naming.Unique(taken, p.Name)
			taken[p.Name] = true
		}
	}
	for _, r := range returns {
		if types.Universe.Lookup(r.Name) != nil {
			r.Name = naming.Unique(taken, r.Name)
			taken[r.Name] = true
		}
	}
	return m
}
