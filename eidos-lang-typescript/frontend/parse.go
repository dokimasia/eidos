// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"encoding/json"
	"path"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// manifest is the part of a package.json that the frontend reads: the name
// of the package.
type manifest struct {
	Name string `json:"name"`
}

// parse loads one unit, a file, through its grammar into the builder of
// the unit. A syntax error is a problem of the source: every ERROR and
// MISSING node reports at its position, and every declaration that the
// parser recovered loads. The tsconfig chain of the member, among its
// shared inputs, states how its non-relative specifiers resolve. A chain
// that does not read whole reports under [BadConfig], and the files that
// it governs resolve through neither a baseUrl nor paths. Every package of
// the unit has the module of the package.json of the member.
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
			comments: v.comments,
			exported: map[string]bool{},
		}
		l.lower(tree.Root())
		tree.Close()
	}
	name, root := moduleOf(u)
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		gb.StampModule(pkg, name, root)
	}
	return nil
}

// resolutionOf reads the tsconfig chain of a member through the door of
// the unit and returns its module resolution. It reports a chain that does
// not read whole at its governing configuration, and returns the empty
// resolution for it. A member without a tsconfig.json among its shared
// inputs resolves through neither a baseUrl nor paths.
func resolutionOf(u *plugin.SourceUnit, ref plugin.SourceRef) resolution {
	if len(ref.Shared) == 0 || path.Base(ref.Shared[0]) != tsconfigName {
		return resolution{}
	}
	_, res, err := readChain(u.Read, ref.Shared[0])
	if err != nil {
		u.Warnf(BadConfig, position.Pos{File: ref.Shared[0], Line: 1, Col: 1},
			"%v. The files it governs resolve without its baseUrl and paths", err)
		return resolution{}
	}
	return res
}

// moduleOf returns the module of the member of a unit: the name of the
// package.json among its shared inputs, and the directory of that file. A
// member without a package.json is in no module, and so is a member whose
// package.json has no name. moduleOf reports a package.json that does not
// read or parse under [BadPackage].
func moduleOf(u *plugin.SourceUnit) (name, root string) {
	shared := u.Files()[0].Shared
	if len(shared) == 0 || path.Base(shared[len(shared)-1]) != packageJSON {
		return "", ""
	}
	file := shared[len(shared)-1]
	data, err := u.Read(file)
	var m manifest
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil {
		u.Warnf(BadPackage, position.Pos{File: file, Line: 1, Col: 1},
			"%v. The files it governs load without a module", err)
		return "", ""
	}
	return m.Name, path.Dir(file)
}
