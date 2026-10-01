// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"
	"strings"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// modStem is the stem of a directory module's file, mod.rs.
const modStem = "mod"

// exclusion is a module a cfg predicate keeps out: the module's file,
// empty for an inline module and for one whose file is not a member,
// the directory its file modules are in, and the predicate as written.
type exclusion struct {
	file string
	dir  string
	pred string
}

// crate is one unit's parse state: the unit, the grammar's vocabulary,
// the load's options, the target the unit is and its root file, the
// members of the unit and the ones the module walk lowered, the modules
// a cfg predicate keeps out, the types the crate declares by package
// and name, and the impl blocks that fold once every module is lowered.
type crate struct {
	ctx        context.Context
	u          *plugin.SourceUnit
	v          *vocabulary
	opts       *Options
	target     target
	root       string
	members    map[string]bool
	walked     map[string]bool
	exclusions []exclusion
	types      map[string]map[string]symbol.Symbol
	impls      []pendingImpl
	err        error
}

// targetOf returns the target a unit is: the one its first member roots
// in the package its manifest states. A file without a manifest, and a
// file whose manifest names no package, is a crate of its own, named for
// its path. A manifest that does not read or parse reports under
// [BadManifest] and states no target.
func (w *crate) targetOf(files []plugin.SourceRef) target {
	loose := target{role: roleShared, name: strings.TrimSuffix(w.root, rustExtension), root: w.root}
	if len(files[0].Shared) == 0 {
		return loose
	}
	mpath := files[0].Shared[0]
	var m *manifest
	data, err := w.u.Read(mpath)
	if err == nil {
		m, err = parseManifest(data)
	}
	if err != nil {
		w.u.Warnf(BadManifest, position.Pos{File: mpath, Line: 1, Col: 1},
			"%v. Each file of the package loads as a crate of its own", err)
		return loose
	}
	if m.crateName() == "" {
		return loose
	}
	dir := path.Dir(mpath)
	rel := make(map[string]bool, len(files))
	for _, ref := range files {
		rel[relative(dir, ref.Path)] = true
	}
	return owner(m.targets(rel), m.crateName(), relative(dir, w.root))
}

// walk lowers one module file, and through its mod items every module
// file below it, under a package path. A file the walk lowered already
// is not lowered twice, and a read or a parse that fails stops the
// walk. A file whose inner cfg predicate is outside the load's set
// lowers no item, and the members below it are left out. Every file of
// an integration test, and of a shared module under the tests
// directory, is stamped rust.test.
func (w *crate) walk(file, pkg string) {
	if w.walked[file] || w.err != nil {
		return
	}
	w.walked[file] = true
	src, err := w.u.Read(file)
	if err != nil {
		w.err = err
		return
	}
	tree, err := w.v.grammar.Parse(w.ctx, file, src)
	if err != nil {
		w.err = err
		return
	}
	defer tree.Close()
	l := &lowering{w: w, v: w.v, tree: tree, path: file, taken: map[position.Pos]bool{}}
	root := tree.Root()
	c := l.container(pkg, moduleDir(file, w.root), false, root.Pos())
	if w.target.tests() {
		w.stamp(c.file, rust.TestKey, true, c.file.Pos)
	}
	if pred := l.header(root, c); pred != "" {
		w.exclusions = append(w.exclusions, exclusion{dir: c.dir, pred: pred})
	} else {
		l.items(root, c)
		l.sweep(root)
	}
	l.reportSyntax()
	if len(l.excluded) > 0 {
		w.stamp(c.file, rust.CfgKey, l.excluded, c.file.Pos)
	}
}

// excludedBy returns the predicate that keeps a member out, and reports
// whether one does: the member is the file of a module a cfg predicate
// keeps out, or below the directory of one.
func (w *crate) excludedBy(file string) (string, bool) {
	for _, e := range w.exclusions {
		if file == e.file || e.dir == rootDir || strings.HasPrefix(file, e.dir+"/") {
			return e.pred, true
		}
	}
	return "", false
}

// layoutPackage returns the module path a member's place spells: its
// path relative to the crate root's directory, without its extension
// and without a final mod, below the target's crate name.
func (w *crate) layoutPackage(file string) string {
	rel := strings.TrimSuffix(relative(path.Dir(w.root), file), rustExtension)
	if path.Base(rel) == modStem {
		rel = path.Dir(rel)
		if rel == rootDir {
			rel = ""
		}
	}
	return join(w.target.name, rel)
}

// packageAt returns the unit's package of a path, named for the path's
// last segment.
func (w *crate) packageAt(pkg string) *node.Package {
	p := w.u.Graph().Package(pkg)
	if p.Name == "" {
		p.Name = path.Base(pkg)
	}
	return p
}

// fileNode creates the File node one file contributes to a package,
// and records the scope its declarations resolve through.
func (w *crate) fileNode(pkg, file string, at position.Pos, sc *scope) *node.File {
	p := w.packageAt(pkg)
	f := &node.File{Path: file, Pos: at}
	p.Files = append(p.Files, f)
	w.u.Graph().Scope(f, sc)
	return f
}

// addType records a type the crate declares, under its package and name,
// for the impl blocks to fold onto. The first declaration of a name is
// the one recorded.
func (w *crate) addType(pkg, name string, decl symbol.Symbol) {
	if w.types[pkg] == nil {
		w.types[pkg] = map[string]symbol.Symbol{}
	}
	if _, met := w.types[pkg][name]; !met {
		w.types[pkg][name] = decl
	}
}

// mark attaches a declaration's carriers, and stamps its restricted
// visibility's spelling and its test mark.
func (w *crate) mark(decl symbol.Symbol, parts plugin.CommentParts, spelled string, a attributes) {
	w.u.AttachCarriers(decl, parts.Carriers, BadCarrier)
	if spelled != "" {
		w.stamp(decl, rust.VisibilityKey, spelled, decl.Position())
	}
	if a.test {
		w.stamp(decl, rust.TestKey, true, decl.Position())
	}
}

// stamp records one stamp on a subject.
func (w *crate) stamp(subject symbol.Symbol, key meta.KeyName, value any, at position.Pos) {
	w.u.Graph().Stamp(subject, meta.RawStamp{Key: key, Value: value, Pos: at})
}

// moduleDir returns the directory the file modules of a module file are
// in: the file's own directory for a crate root and a mod.rs, and a
// directory named after the file's stem beside it for any other file.
func moduleDir(file, root string) string {
	if file == root || path.Base(file) == modFile {
		return path.Dir(file)
	}
	return strings.TrimSuffix(file, rustExtension)
}

// join appends a module path to a package path, which a Rust crate never
// leaves empty.
func join(pkg, rel string) string {
	if rel == "" {
		return pkg
	}
	return pkg + "/" + rel
}

// parse loads one unit, a crate target, into the unit's builder. It
// walks the module tree from the crate root, the unit's first member,
// through every mod item, and lowers each member the tree names under
// the module path the tree spells, below the target's crate name. A
// member of a module a cfg predicate keeps out loads nothing and reports
// under [ExcludedFile]. Any other member no mod item names loads under
// the module path its place in the crate's directories spells, and
// reports under [UnlinkedFile]. The inherent impl blocks fold onto the
// types they name once every module is lowered, because a block
// anywhere in the crate adds methods to a type the crate declares. A
// syntax error is the source's problem: every ERROR and MISSING node
// reports positioned, and every item the parser still recovered loads. A
// member that does not read and a context that is done fail the unit.
func (f *rustFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	w := &crate{
		ctx: ctx, u: u, v: f.v, opts: f.opts, root: u.Files()[0].Path,
		members: map[string]bool{}, walked: map[string]bool{},
		types: map[string]map[string]symbol.Symbol{},
	}
	for _, ref := range u.Files() {
		w.members[ref.Path] = true
	}
	w.target = w.targetOf(u.Files())
	w.walk(w.root, w.target.name)
	for _, ref := range u.Files() {
		if w.err != nil || w.walked[ref.Path] {
			continue
		}
		at := position.Pos{File: ref.Path, Line: 1, Col: 1}
		if pred, out := w.excludedBy(ref.Path); out {
			u.Infof(ExcludedFile, at, "the cfg predicate %s keeps the module of %s out of the load", pred, ref.Path)
			continue
		}
		pkg := w.layoutPackage(ref.Path)
		u.Infof(UnlinkedFile, at, "no mod item names %s, so it loads as the module %s", ref.Path, pkg)
		w.walk(ref.Path, pkg)
	}
	if w.err != nil {
		return w.err
	}
	w.fold()
	return nil
}
