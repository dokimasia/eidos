// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// parse loads one unit, a file, through its grammar into the unit's
// builder. A syntax error is the source's problem: every ERROR and
// MISSING node reports positioned, and every declaration the parser
// still recovered loads. The member's tsconfig chain, its shared
// inputs, states how its non-relative specifiers resolve, and a chain
// that does not read whole reports under [BadConfig] and resolves
// through what it read before the fault.
func (f *tsFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	for _, ref := range u.Files() {
		src, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		v := f.vocabularyOf(ref.Path)
		tree, err := v.grammar.Parse(ctx, ref.Path, src)
		if err != nil {
			return err
		}
		l := &lowering{
			u: u, v: v, tree: tree, path: ref.Path,
			module:   newModule(ref.Path, resolutionOf(u, ref)),
			files:    map[string]*node.File{},
			consumed: map[position.Pos]bool{},
			exported: map[string]bool{},
		}
		l.lower(tree.Root())
		tree.Close()
	}
	return nil
}

// resolutionOf reads a member's tsconfig chain through the unit's door
// and returns its module resolution, reporting a chain that does not
// read whole at its governing configuration.
func resolutionOf(u *plugin.SourceUnit, ref plugin.SourceRef) resolution {
	if len(ref.Shared) == 0 {
		return resolution{}
	}
	_, res, err := readChain(u.Read, ref.Shared[0])
	if err != nil {
		u.Warnf(BadConfig, position.Pos{File: ref.Shared[0], Line: 1, Col: 1},
			"%v. The files it governs resolve without its baseUrl and paths", err)
	}
	return res
}
