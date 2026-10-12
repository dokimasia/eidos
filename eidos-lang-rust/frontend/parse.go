// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"context"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// modStem is the stem of the file of a directory module, mod.rs.
const modStem = "mod"

// exclusion is a module that a cfg predicate keeps out: the file of the
// module, empty for an inline module and for a file that is not a member,
// the directory of its file modules, and the predicate as written.
type exclusion struct {
	file string
	dir  string
	pred string
}

// crate is the state of the parse of one unit: the unit, the vocabulary of
// the grammar, the options of the load, the target of the unit, the
// directory of the crate and its root file, the members of the unit and
// the members that the module walk lowered, the modules that a cfg
// predicate keeps out, the types of the crate by package and name, and the
// impl blocks that fold once every module is lowered.
type crate struct {
	ctx        context.Context
	u          *plugin.SourceUnit
	v          *vocabulary
	opts       *Options
	target     target
	dir        string
	root       string
	members    map[string]bool
	walked     map[string]bool
	exclusions []exclusion
	types      map[string]map[string]symbol.Symbol
	impls      []pendingImpl
	err        error
}

// targetOf returns the target of a unit and the directory of its crate:
// the target that the first member roots in the package of its manifest,
// in the directory of the manifest. A file without a manifest, and a file
// whose manifest names no package, is a crate of its own, named after its
// path, in its own directory. A manifest that does not read or parse
// reports under [BadManifest] and states no target.
func (w *crate) targetOf(files []plugin.SourceRef) (target, string) {
	loose := target{role: roleShared, name: strings.TrimSuffix(w.root, rust.Extension), root: w.root}
	looseDir := path.Dir(w.root)
	if len(files[0].Shared) == 0 {
		return loose, looseDir
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
		return loose, looseDir
	}
	if m.crateName() == "" {
		return loose, looseDir
	}
	dir := path.Dir(mpath)
	rel := make(map[string]bool, len(files))
	for _, ref := range files {
		rel[relative(dir, ref.Path)] = true
	}
	return owner(m.targets(rel), m.crateName(), relative(dir, w.root)), dir
}

// walk lowers one module file under a package path, and through its mod
// items every module file below it. The walk lowers a file once, and a
// read or a parse that fails stops the walk. A file whose inner cfg
// predicate is outside the set of the load lowers no item, and the members
// below it are left out. rust.test stamps every file of an integration
// test, and of a shared module under the tests directory. rust.generated
// stamps a file whose first comment contains [rust.GeneratedMark], with
// the comment as written. The sweep reports the carriers of the comments
// that no declaration takes, and every syntax error of the file reports
// under [UnparsedFile].
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
	l := &lowering{w: w, v: w.v, tree: tree, path: file, comments: w.v.comments}
	root := tree.Root()
	c := l.container(pkg, moduleDir(file, w.root), false, root.Pos())
	if w.target.tests() {
		w.stamp(c.file, rust.TestKey, true, c.file.Pos)
	}
	if first := root.FirstOf(w.v.lineComment, w.v.blockComment); first.PrevNamedSibling().IsZero() &&
		bytes.Contains(first.Bytes(), []byte(rust.GeneratedMark)) {

		w.stamp(c.file, rust.GeneratedKey, strings.TrimRight(first.Text(), lineEnd), c.file.Pos)
	}
	if pred := l.header(root, c); pred != "" {
		w.exclusions = append(w.exclusions, exclusion{dir: c.dir, pred: pred})
	} else {
		l.items(root, c)
		l.comments.Sweep(root, func(n treesitter.Node) {
			w.u.RefuseCarriers(UnaddressedCarrier, w.u.Comment(n.Text(), n.Pos()).Carriers, unattached)
		})
	}
	if problems := tree.Problems(); problems != nil {
		w.u.ReportSyntax(UnparsedFile, problems)
	}
	if len(l.excluded) > 0 {
		w.stamp(c.file, rust.CfgKey, l.excluded, c.file.Pos)
	}
}

// excludedBy returns the predicate that keeps a member out, and reports
// whether one does. The member is the file of a module that a cfg
// predicate keeps out, or a file below the directory of such a module.
func (w *crate) excludedBy(file string) (string, bool) {
	for _, e := range w.exclusions {
		if file == e.file || e.dir == rootDir || strings.HasPrefix(file, e.dir+"/") {
			return e.pred, true
		}
	}
	return "", false
}

// layoutPackage returns the module path of the place of a member: its
// path relative to the directory of the crate root, without its extension
// and without a final mod, below the crate name of the target.
func (w *crate) layoutPackage(file string) string {
	rel := strings.TrimSuffix(relative(path.Dir(w.root), file), rust.Extension)
	if path.Base(rel) == modStem {
		rel = path.Dir(rel)
		if rel == rootDir {
			rel = ""
		}
	}
	return join(w.target.name, rel)
}

// packageAt returns the package of a path in the unit, named after the
// last segment of the path.
func (w *crate) packageAt(pkg string) *node.Package {
	p := w.u.Graph().Package(pkg)
	if p.Name == "" {
		p.Name = path.Base(pkg)
	}
	return p
}

// fileNode creates the File node of one file in a package, and records
// the scope through which its declarations resolve.
func (w *crate) fileNode(pkg, file string, at position.Pos, sc *scope) *node.File {
	p := w.packageAt(pkg)
	f := &node.File{Path: file, Pos: at}
	p.Files = append(p.Files, f)
	w.u.Graph().Scope(f, sc)
	return f
}

// addType records a type of the crate under its package and name, onto
// which the impl blocks fold. The crate records the first declaration of a
// name.
func (w *crate) addType(pkg, name string, decl symbol.Symbol) {
	if w.types[pkg] == nil {
		w.types[pkg] = map[string]symbol.Symbol{}
	}
	if _, met := w.types[pkg][name]; !met {
		w.types[pkg][name] = decl
	}
}

// mark attaches the carriers of a declaration and the directives of its
// markers, and stamps the spelling of its restricted visibility and its
// test mark.
func (w *crate) mark(decl symbol.Symbol, parts plugin.CommentParts, spelled string, a attributes) {
	w.u.AttachCarriers(decl, parts.Carriers, BadCarrier)
	for _, s := range a.sugars {
		w.u.AttachSugar(decl, s, BadMarker)
	}
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

// moduleDir returns the directory of the file modules of a module file:
// the directory of the file for a crate root and a mod.rs, and a directory
// named after the stem of the file beside it for any other file.
func moduleDir(file, root string) string {
	if file == root || path.Base(file) == modFile {
		return path.Dir(file)
	}
	return strings.TrimSuffix(file, rust.Extension)
}

// join appends a module path to a package path, which a Rust crate never
// leaves empty.
func join(pkg, rel string) string {
	if rel == "" {
		return pkg
	}
	return pkg + "/" + rel
}

// parse loads one unit, a crate target, into the builder of the unit. It
// walks the module tree from the crate root, the first member of the unit,
// through every mod item. Each member that the tree names lowers under the
// module path of the tree, below the crate name of the target. A member of
// a module that a cfg predicate keeps out loads nothing and reports under
// [ExcludedFile]. Any other member that no mod item names loads under the
// module path of its place in the directories of the crate, and reports
// under [UnlinkedFile]. The inherent impl blocks fold onto their types
// once every module is lowered, because a block anywhere in the crate adds
// methods to a type of the crate. Every package of the unit then has the
// crate name as its module and the directory of the crate as its module
// root. A syntax error is a problem of the source: every ERROR and MISSING
// node reports at its position, and every item that the parser recovered
// loads. A member that does not read and a context that is done fail the
// unit.
func (f *rustFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	w := &crate{
		ctx: ctx, u: u, v: f.v, opts: f.opts, root: u.Files()[0].Path,
		members: map[string]bool{}, walked: map[string]bool{},
		types: map[string]map[string]symbol.Symbol{},
	}
	for _, ref := range u.Files() {
		w.members[ref.Path] = true
	}
	w.target, w.dir = w.targetOf(u.Files())
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
	gb := u.Graph()
	for _, pkg := range gb.Packages() {
		gb.StampModule(pkg, w.target.name, w.dir)
	}
	return nil
}
