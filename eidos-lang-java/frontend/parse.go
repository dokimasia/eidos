// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// moduleID is the module a unit's packages are in: the coordinates the
// governing pom.xml states and the directory it is in, both empty for a
// unit no pom.xml governs.
type moduleID struct {
	name string
	root string
}

// moduleOf reads the module the pom.xml a unit's members share states,
// and reports a pom.xml that does not read or parse under [BadPOM]. A
// unit whose members share no pom.xml, as a dependency unit's do not,
// is in no module.
func moduleOf(u *plugin.SourceUnit) moduleID {
	shared := u.Files()[0].Shared
	if len(shared) == 0 || path.Base(shared[0]) != pomName {
		return moduleID{}
	}
	data, err := u.Read(shared[0])
	var p *pom
	if err == nil {
		p, err = parsePOM(data)
	}
	if err != nil {
		u.Warnf(BadPOM, position.Pos{File: shared[0], Line: 1, Col: 1},
			"%v. The packages it governs load without a module", err)
		return moduleID{}
	}
	return moduleID{name: p.module(), root: path.Dir(shared[0])}
}

// stampModule stamps a package with its module's coordinates and the
// directory its pom.xml is in, once per unit.
func stampModule(gb *plugin.GraphBuilder, p *node.Package, m moduleID, stamped map[*node.Package]bool) {
	if m.name == "" || stamped[p] {
		return
	}
	stamped[p] = true
	at := p.Files[0].Pos
	gb.Stamp(p, meta.RawStamp{Key: meta.ModuleKey, Value: m.name, Pos: at})
	gb.Stamp(p, meta.RawStamp{Key: meta.ModuleRootKey, Value: m.root, Pos: at})
}

// parse loads one unit into the unit's builder. A workspace unit is a
// directory of Java source: each member lowers into the package its
// package clause names, and the module the governing pom.xml states
// stamps every package the unit declares into. A syntax error is the
// source's problem: every ERROR and MISSING node reports positioned, and
// every declaration the parser still recovered loads. A dependency
// unit's members are class files, ct.sym entries and JARs, whose classes
// lower together, so a member class lowers into the class that declares
// it. A member that does not read and a context that is done fail the
// unit.
func (f *javaFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	module := moduleOf(u)
	stamped := map[*node.Package]bool{}
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
			l := &lowering{u: u, v: f.v, tree: tree, path: ref.Path, taken: map[position.Pos]bool{}}
			stampModule(u.Graph(), l.lower(tree.Root()), module, stamped)
			tree.Close()
		}
	}
	lowerClasses(u, classes)
	return nil
}
