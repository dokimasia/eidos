// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// link resolves every type reference of the parsed units' packages,
// nested type arguments included, and returns the linker, which selects
// the references of kept and restored units again afterwards.
//
// A bare name that spells a type parameter in scope targets that
// parameter: the parameters of every enclosing declaration are in
// scope, the innermost first, so a parameter takes its name from a
// package-level type of the same spelling the way every language
// with generics scopes it. The references among an inline body's
// members resolve the same way, and a parameter of the body's
// method, which has no identity, leaves a reference that spells it
// without a target. Every other reference resolves through
// the bindings of its file's language: the frontend's Resolve names
// the candidates in shadowing tiers, and the first tier with a
// candidate the graph contains decides. That candidate is the
// target, and several such candidates in the tier report under
// [AmbiguousReference]. A reference no tier resolves keeps its
// spelling alone: degradation a reader can ask about, not a
// failure. For a language whose frontend is a [plugin.Importer], a
// target declared in another file sets the reference's package to
// the import the frontend names for that file.
//
// A file without a recorded scope resolves nothing: there are no
// bindings to resolve through, and its references keep their
// spellings.
//
// A declaration resolves under the scope of the file its position
// names, which is not always the file whose tree contains it: a
// language that folds a method onto the declaration of its receiver
// type, as Go and Rust do, moves the method into another file's tree,
// and the method's spellings resolve through its own file's imports.
// Where two packages each contain a file of that path, the file of
// the declaration's own package is the one.
//
// A candidate that names no declaration is followed through the
// re-exports of its package when its language's frontend is a
// [plugin.Exporter]. The package's files name what they publish under
// the candidate's name, in path order, and the first file whose
// candidates resolve by the same rule decides. What that file's
// candidates resolve to counts as the candidate's own hits in its
// tier. A package and name already on the following path find
// nothing, which stops a cycle of re-exports.
//
// Every reference resolved through its frontend leaves a record on the
// unit whose file contains it: the tiers the frontend returned, the
// candidates whose re-exports the selection followed, the candidates
// the followed re-exports offered, and the ambiguity it reported. A run
// that keeps the unit selects the target again from the record.
func link(packages []*spliced, scopes []scopeEntry, ix *index, sink *diag.Sink) *linker {
	byFile := make(map[*node.File]scopeEntry, len(scopes))
	for _, s := range scopes {
		byFile[s.file] = s
	}
	byPath := make(map[fileKey]scopeEntry, len(scopes))
	byName := make(map[string]scopeEntry, len(scopes))
	exporters := map[pkgKey][]exporterEntry{}
	for _, sp := range packages {
		for _, f := range sp.pkg.Files {
			entry, held := byFile[f]
			if !held {
				continue
			}
			byPath[fileKey{pkg: sp.pkg, path: f.Path}] = entry
			if _, met := byName[f.Path]; !met {
				byName[f.Path] = entry
			}
			if exp, exports := entry.frontend.(plugin.Exporter); exports {
				key := pkgKey{lang: sp.lang, path: sp.pkg.ID.Package}
				exporters[key] = append(exporters[key], exporterEntry{scope: entry, exporter: exp})
			}
		}
	}
	for _, files := range exporters {
		slices.SortFunc(files, byExporterPath)
	}
	w := &linker{
		byPath: byPath, byName: byName, exporters: exporters, merged: map[pkgKey]bool{},
		following: map[followKey]bool{}, ix: ix, sink: sink,
	}
	for _, sp := range packages {
		w.pkg = sp.pkg
		for _, f := range sp.pkg.Files {
			entry, held := byFile[f]
			if !held {
				continue
			}
			w.unit = sp.units[f]
			w.under(f, symbol.Identity{}, nil, entry)
		}
	}
	return w
}

// fileKey names one file of one package, which is how a declaration's
// position finds the scope it resolves under: two packages can each
// contain a file of one path, as a TypeScript namespace's package and
// its file's own package do.
type fileKey struct {
	pkg  *node.Package
	path string
}

// pkgKey names one package of one language, which is how following
// a re-export finds the files of a candidate's package.
type pkgKey struct {
	lang symbol.Lang
	path string
}

// followKey is one package and name on the following path.
type followKey struct {
	pkg  pkgKey
	name string
}

// exporterEntry is one file that recorded a scope, in a language
// whose frontend is a [plugin.Exporter].
type exporterEntry struct {
	scope    scopeEntry
	exporter plugin.Exporter
}

// linker walks the packages' declarations one package at a time and
// resolves their references. pkg is the package under the walk and unit
// the unit of the file under the walk, byName maps a path onto the
// first file of that path the splice recorded a scope for, exporters
// maps a package to its exporting files, merged marks a package whose
// exporting files of kept units joined the parsed ones, following is the
// package-and-name path of the re-exports it follows. followed contains
// the candidates whose re-exports the reference under selection
// followed, and reached the candidates those re-exports offered.
type linker struct {
	pkg       *node.Package
	unit      *unit
	byPath    map[fileKey]scopeEntry
	byName    map[string]scopeEntry
	exporters map[pkgKey][]exporterEntry
	merged    map[pkgKey]bool
	following map[followKey]bool
	followed  []symbol.Identity
	reached   []symbol.Identity
	// hits is the buffer the selection of one reference collects its
	// first tier's declarations in, which every selection reuses.
	hits []symbol.Identity
	ix   *index
	sink *diag.Sink
}

// under resolves the references directly inside one declaration,
// then descends into each nested declaration under its own identity.
//
// The owner is what a lexically scoped language resolves against:
// a name written inside a message resolves to that message's own
// nested types before it resolves outward. A declaration without an
// identity, which a dropped duplicate is, keeps its parent's owner,
// because its references are written inside the parent. The type
// parameters in scope map each parameter's name to its identity.
// A nested declaration whose position names another file of the
// package that recorded a scope resolves under that file's scope.
func (w *linker) under(
	s symbol.Symbol, owner symbol.Identity, params map[string]symbol.Identity, entry scopeEntry,
) {
	scope := plugin.ImportScope{File: entry.file.ID, Owner: owner, Bindings: entry.bindings}
	node.Walk(s, func(child symbol.Symbol) bool {
		if child == s {
			return true
		}
		// A reference resolves here, under this owner. A structural
		// one has no target of its own: the walk descends into its
		// children, and the named ones resolve.
		if ref, is := child.(*node.TypeRef); is {
			if ref.Form == symbol.FormNamed && ref.Spelling != "" && ref.Target.IsZero() {
				if id, spells := params[ref.Spelling]; spells && len(ref.Args) == 0 {
					ref.Target = id
				} else {
					w.resolve(ref, entry.frontend, scope)
				}
			}
			return true
		}
		next := w.scopeOf(child, entry)
		declared := typeParamsOf(child)
		if !scoped(child) && len(declared) == 0 && next.file == entry.file {
			return true
		}
		inner := owner
		if scoped(child) {
			if decl, names := child.(node.Declaration); names && !decl.Identity().IsZero() {
				inner = decl.Identity()
			}
		}
		w.under(child, inner, withParams(params, declared), next)
		return false // the recursion walks this subtree
	})
}

// scopeOf returns the scope a declaration resolves under: the one its
// position's file recorded when that is another file, and the
// enclosing scope when the position names no file or a file with no
// recorded scope, as a Go line directive can. A file of the package
// under the walk comes first, and otherwise the first file of that
// path the splice recorded, as for a Rust impl block written in
// another module of the crate.
func (w *linker) scopeOf(s symbol.Symbol, enclosing scopeEntry) scopeEntry {
	at := s.Position().File
	if at == "" || at == enclosing.file.Path {
		return enclosing
	}
	if entry, held := w.byPath[fileKey{pkg: w.pkg, path: at}]; held {
		return entry
	}
	if entry, held := w.byName[at]; held {
		return entry
	}
	return enclosing
}

// resolve settles one reference: the first tier with a candidate the
// graph contains decides, that candidate is the target, and several
// such candidates in the tier report as an ambiguity. A target an
// importer's language declares in another file records the import
// naming that file as the reference's package. The reference's record
// goes on the unit under the walk, and a reference the frontend offers
// no candidate for leaves none, because no graph can give it a target.
func (w *linker) resolve(ref *node.TypeRef, f plugin.Frontend, scope plugin.ImportScope) {
	tiers := f.Resolve(scope, ref.Spelling)
	if len(tiers) == 0 {
		return
	}
	w.followed, w.reached = nil, nil
	hits := w.selected(tiers)
	record := store.Link{Tiers: tiers, Followed: w.followed, Reached: w.reached}
	if len(hits) > 0 {
		ref.Target = hits[0]
		if imp, imports := f.(plugin.Importer); imports {
			if file := w.ix.file(ref.Target); file != "" && file != scope.File.Name {
				ref.Package = imp.ImportOf(scope, file)
			}
		}
	}
	record.Findings = ambiguity(ref, f, hits)
	for _, d := range record.Findings {
		w.sink.Report(d)
	}
	if w.unit.resolved == nil {
		w.unit.resolved = map[*node.TypeRef]store.Link{}
	}
	w.unit.resolved[ref] = record
}

// selected returns the declarations of the first tier that names any,
// collected in the linker's buffer, which the next selection overwrites.
// It allocates only where the buffer lacks the room for them, and keeps
// the grown buffer.
func (w *linker) selected(tiers plugin.Candidates) []symbol.Identity {
	hits := w.firstTier(w.hits[:0], tiers)
	if cap(hits) > cap(w.hits) {
		w.hits = hits[:0]
	}
	return hits
}

// firstTier returns the declarations of the first tier that names
// any, in candidate order and without repeats, and nothing when no
// tier does. It collects them in dst, which is empty, and allocates
// only where dst lacks the room for them.
func (w *linker) firstTier(dst []symbol.Identity, tiers plugin.Candidates) []symbol.Identity {
	for _, tier := range tiers {
		hits := dst
		for _, c := range tier {
			for _, full := range w.hitsOf(c) {
				if !slices.Contains(hits, full) {
					hits = append(hits, full)
				}
			}
		}
		if len(hits) > 0 {
			return hits
		}
	}
	return nil
}

// hitsOf returns the declarations one candidate names: the ones the
// index lists under its bare identity, or else what following the
// re-exports of its package finds. A candidate with an owner names a
// member, which no file re-exports, and a package and name already on
// the following path find nothing. hitsOf records each candidate a
// followed re-export offered in reached.
func (w *linker) hitsOf(c symbol.Identity) []symbol.Identity {
	if len(w.following) > 0 {
		w.reached = append(w.reached, bareOf(c))
	}
	if hits := w.ix.lookup(c); len(hits) > 0 || c.Owner != "" {
		return hits
	}
	pkg := pkgKey{lang: c.Lang, path: c.Package}
	files := w.exportersOf(pkg)
	key := followKey{pkg: pkg, name: c.Name}
	if len(files) == 0 || w.following[key] {
		return nil
	}
	w.following[key] = true
	defer delete(w.following, key)
	w.followed = append(w.followed, bareOf(c))
	for _, e := range files {
		scope := plugin.ImportScope{File: e.scope.file.ID, Bindings: e.scope.bindings}
		if hits := w.firstTier(nil, e.exporter.Exports(scope, c.Name)); len(hits) > 0 {
			return hits
		}
	}
	return nil
}

// exportersOf returns a package's exporting files in path order: the
// parsed units' and, the first time it is asked, the kept and restored
// units' too.
func (w *linker) exportersOf(pkg pkgKey) []exporterEntry {
	if w.merged[pkg] || w.ix.kept == nil {
		return w.exporters[pkg]
	}
	w.merged[pkg] = true
	files := slices.Concat(w.exporters[pkg], w.ix.kept.exportersOf(pkg))
	slices.SortFunc(files, byExporterPath)
	w.exporters[pkg] = files
	return files
}

// byExporterPath orders exporting files by path, the order a package's
// re-exports are asked in.
func byExporterPath(a, b exporterEntry) int {
	return strings.Compare(a.scope.file.Path, b.scope.file.Path)
}

// ambiguity returns the finding of a reference whose first tier with a
// match matched more than one declaration, and nothing for any other.
func ambiguity(ref *node.TypeRef, f plugin.Frontend, hits []symbol.Identity) []diag.Diag {
	if len(hits) < 2 {
		return nil
	}
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = h.String()
	}
	return []diag.Diag{{
		Code:     AmbiguousReference,
		Severity: diag.SeverityWarning,
		Pos:      ref.Pos,
		Msg: fmt.Sprintf("%q resolves to %s, and the first is the target",
			ref.Spelling, strings.Join(names, " and ")),
		Origin: f.Name(),
	}}
}

// scoped reports whether a declaration opens a lexical scope a
// reference inside it resolves against first: the kinds that nest
// types. A member nests none, so a field's own reference resolves
// under the type that declares the field and not under the field.
func scoped(s symbol.Symbol) bool {
	switch s.(type) {
	case *node.Struct, *node.Interface, *node.Enum, *node.Sum:
		return true
	default:
		return false
	}
}

// typeParamsOf returns the type parameters a declaration declares,
// and nil for a kind that declares none.
func typeParamsOf(s symbol.Symbol) []*node.TypeParam {
	switch d := s.(type) {
	case *node.Struct:
		return d.TypeParams
	case *node.Interface:
		return d.TypeParams
	case *node.Alias:
		return d.TypeParams
	case *node.Sum:
		return d.TypeParams
	case *node.Function:
		return d.TypeParams
	case *node.Method:
		return d.TypeParams
	default:
		return nil
	}
}

// withParams returns the type parameters in scope inside a
// declaration: the enclosing ones, shadowed by the declaration's
// own of the same name. A parameter without an identity, which a
// dropped duplicate's and an inline body's method's are, still
// shadows, and maps to the zero identity, so a reference that spells
// it targets nothing and keeps its spelling. The enclosing map is
// never written, so a sibling declaration sees what its parent saw.
func withParams(
	enclosing map[string]symbol.Identity, declared []*node.TypeParam,
) map[string]symbol.Identity {
	if len(declared) == 0 {
		return enclosing
	}
	out := make(map[string]symbol.Identity, len(enclosing)+len(declared))
	maps.Copy(out, enclosing)
	for _, tp := range declared {
		if tp != nil && tp.Name != "" {
			out[tp.Name] = tp.ID
		}
	}
	return out
}
