// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// moduleOf returns the module of the members of a unit: the coordinates
// that their shared pom.xml states, and the directory of that file. A unit
// whose members share no pom.xml is in no module, and so are the members
// of a dependency unit. moduleOf reports a pom.xml that does not read or
// parse under [BadPOM].
func moduleOf(u *plugin.SourceUnit) (name, root string) {
	shared := u.Files()[0].Shared
	if len(shared) == 0 || path.Base(shared[0]) != pomName {
		return "", ""
	}
	data, err := u.Read(shared[0])
	var p *pom
	if err == nil {
		p, err = parsePOM(data)
	}
	if err != nil {
		u.Warnf(BadPOM, position.Pos{File: shared[0], Line: 1, Col: 1},
			"%v. The packages it governs load without a module", err)
		return "", ""
	}
	return p.module(), path.Dir(shared[0])
}

// parse loads one unit into the builder of the unit. A workspace unit is a
// directory of Java source. Each member lowers into the package that its
// package clause names, and every package of the unit has the module of
// the governing pom.xml. A syntax error is a problem of the source: every
// ERROR and MISSING node reports at its position, and every declaration
// that the parser recovered loads. The members of a dependency unit are
// class files, ct.sym entries and JARs, whose classes lower together, so a
// member class lowers into the class that declares it. A member that does
// not read and a context that is done fail the unit.
func (f *javaFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	name, root := moduleOf(u)
	var classes []classFile
	for _, ref := range u.Files() {
		src, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		switch path.Ext(ref.Path) {
		case sigExt, classExt:
			if err := ctx.Err(); err != nil {
				return err
			}
			if cf, ok := decodeClass(u, ref.Path, path.Base(ref.Path), src); ok {
				classes = append(classes, cf)
			}
		case jarExt:
			if err := ctx.Err(); err != nil {
				return err
			}
			classes = append(classes, jarClasses(u, ref.Path, src, f.opts.Release)...)
		default:
			tree, err := f.v.grammar.Parse(ctx, ref.Path, src)
			if err != nil {
				return err
			}
			l := &lowering{u: u, v: f.v, tree: tree, path: ref.Path, comments: f.v.comments}
			l.lower(tree.Root())
			tree.Close()
		}
	}
	lowerClasses(u, classes)
	gb := u.Graph()
	for _, p := range gb.Packages() {
		gb.StampModule(p, name, root)
	}
	return nil
}
