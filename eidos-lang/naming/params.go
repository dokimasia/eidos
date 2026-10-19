// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/sdk/emit"
)

// positionalPrefix opens the name that [Params] gives a parameter without
// a name, before the position of the parameter.
const positionalPrefix = "a"

// Params names each parameter of m that has no name, and returns m. The
// parameter at position i takes a followed by i, or that name followed by
// the smallest number from 0 that m does not have, as [Unique] chooses.
// The names of the receiver, the type parameters, the parameters and the
// results of m are taken first, so a named parameter keeps its name and no
// two names of m are equal. A generator that writes a body for a mirrored
// method calls Params, because the body passes each parameter on by its
// name. A nil m returns nil.
//
// # Allocation contract
//
// Params allocates nothing for a method whose parameters all have names.
// Otherwise it allocates the set of the names of m and the text of each
// name that it gives, and the set takes one allocation for a method of
// eight names or fewer.
func Params(m *emit.Method) *emit.Method {
	if m == nil {
		return nil
	}
	params := m.Params.Items()
	if !slices.ContainsFunc(params, func(p *emit.Param) bool { return p.Name == "" }) {
		return m
	}
	taken := make(map[string]bool, 1+len(m.TypeParams)+len(params)+m.Returns.Len())
	if m.Receiver != nil {
		taken[m.Receiver.Name] = true
	}
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
	}
	for _, p := range params {
		taken[p.Name] = true
	}
	for _, r := range m.Returns.Items() {
		taken[r.Name] = true
	}
	for i, p := range params {
		if p.Name == "" {
			p.Name = Unique(taken, positionalPrefix+strconv.Itoa(i))
			taken[p.Name] = true
		}
	}
	return m
}
