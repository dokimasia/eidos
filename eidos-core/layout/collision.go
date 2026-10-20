// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/internal/pathset"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The words that the finding of a collision of names joins its
// declarations with: the separator of two declarations, the word before
// the file of a declaration, and the dot between a source package and a
// name.
const (
	collisionSep = " and "
	collisionIn  = " in "
	qualifierSep = "."
)

// collide refuses what one tree cannot contain: a file whose units
// derive from two packages, and two files whose paths clash under the
// rule every output sink stages by. Each collision is one
// [PathCollision] at the second file's first declaration, with the
// first file's as related, and the declarations of both files are
// refused. The files are sorted by path, so the first is the earlier
// path.
func (r *router) collide() {
	var tree pathset.Set
	for fi := range r.files {
		f := &r.files[fi]
		r.mixed(f)
		clash, other := tree.Clash(f.file.Path)
		if clash == pathset.ClashNone {
			tree.Add(f.file.Path)
			continue
		}
		first := &r.files[r.byPath[other]]
		related := []position.Pos{r.fileAt(first)}
		switch clash {
		case pathset.ClashCase:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s and %s differ only in case, and a case-insensitive filesystem stores them as one file",
				first.file.Path, f.file.Path)
		case pathset.ClashDirectory:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s is routed as a file, and %s needs it as a directory", f.file.Path, first.file.Path)
		default:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s needs %s as a directory, and %s is routed as a file",
				f.file.Path, first.file.Path, first.file.Path)
		}
		r.refuse(first)
		r.refuse(f)
	}
}

// mixed refuses a file whose units derive from two packages, because a
// file declares one package. A unit without a package, such as a plan
// unit, derives from none and mixes with any.
func (r *router) mixed(f *building) {
	var first *plugin.Unit
	for i := range f.file.Units {
		u := &f.file.Units[i]
		switch {
		case u.Pkg.IsZero():
		case first == nil:
			first = u
		case u.Pkg != first.Pkg:
			r.errorf(PathCollision, r.firstAt(u), []position.Pos{r.firstAt(first)},
				"declarations of %s and of %s route to %s, and a file declares one package",
				first.Pkg, u.Pkg, f.file.Path)
			r.refuse(f)
			return
		}
	}
}

// fileAt positions a finding about a file at its first declaration's
// origin.
func (r *router) fileAt(f *building) position.Pos {
	return r.firstAt(&f.file.Units[0])
}

// scopedName is one name in the check of names: its index in the names of
// the routed files, and the index of its file.
type scopedName struct {
	name, file int
}

// collideNames reports [plugin.CollidingNames] for two file-level names of
// one routed scope with one settled name. The scope of a name is the
// namespace of its routed file, [plugin.File.Scope], and the receiver of a
// method or of a field that another type receives. A file without a
// namespace has no package that the target derives, so its names are in
// no scope. names are the names of files, in the order that
// [plugin.NamesOf] returns them. The names of a scope are in the order of
// the paths of their files, and the names of one file in the order of
// their declarations. Where Input.Others is set, the kept names of each
// scope take part in the same order. The first declaration of a settled
// name in a scope opens a group, and each later declaration joins it. A
// group is one Error at the origin of its second declaration, with the
// origins of the others as related. A group of functions of one emitted
// name is no collision where the target is a [plugin.Overloader] that
// overloads, because its declarations are the overloads of one function.
// The check refuses no declaration: the Error fails the plan, so no file of
// the group is written.
func (r *router) collideNames(files []plugin.File, names []plugin.NameEntry) {
	overloader, declares := r.in.Packager.(plugin.Overloader)
	overloading := declares && overloader.Overloads()
	// The names of each file follow the names of the files before it, so
	// one walk of both finds the file of each name.
	order := make([]scopedName, 0, len(names))
	at := 0
	for fi := range files {
		for at < len(names) && names[at].File == files[fi].Path {
			order = append(order, scopedName{name: at, file: fi})
			at++
		}
	}
	compare := func(a, b scopedName) int {
		return cmp.Or(files[a.file].Scope.Compare(files[b.file].Scope),
			strings.Compare(names[a.name].Receiver, names[b.name].Receiver),
			strings.Compare(names[a.name].Settled, names[b.name].Settled))
	}
	slices.SortStableFunc(order, compare)
	var group []*plugin.NameEntry
	for lo := 0; lo < len(order); {
		hi := lo + 1
		for hi < len(order) && compare(order[lo], order[hi]) == 0 {
			hi++
		}
		scope := files[order[lo].file].Scope
		if scope.IsZero() {
			lo = hi
			continue
		}
		group = group[:0]
		for _, s := range order[lo:hi] {
			group = append(group, &names[s.name])
		}
		if r.in.Others != nil {
			first := group[0]
			kept := r.in.Others.InFileScope(scope, first.Receiver)
			found, _ := slices.BinarySearchFunc(kept, first.Settled, func(e plugin.NameEntry, settled string) int {
				return strings.Compare(e.Settled, settled)
			})
			for ; found < len(kept) && kept[found].Settled == first.Settled; found++ {
				group = append(group, &kept[found])
			}
			slices.SortStableFunc(group, func(a, b *plugin.NameEntry) int { return strings.Compare(a.File, b.File) })
		}
		if len(group) > 1 && (!overloading || !overloads(group)) {
			r.reportCollision(group)
		}
		lo = hi
	}
}

// overloads reports whether every declaration of group is a function of the
// emitted name of the first: the overloads of one function.
func overloads(group []*plugin.NameEntry) bool {
	for _, e := range group {
		if e.Kind != symbol.KindFunction || e.Emitted != group[0].Emitted {
			return false
		}
	}
	return true
}

// reportCollision reports one group of names of one routed scope that
// settle to one name, at the origin of its second declaration, with the
// origins of the others as related. The message names each declaration
// by its source package and its emitted name, and the file that declares
// it.
func (r *router) reportCollision(group []*plugin.NameEntry) {
	var described strings.Builder
	related := make([]position.Pos, 0, len(group)-1)
	for i, e := range group {
		if i > 0 {
			described.WriteString(collisionSep)
		}
		if e.Package != "" {
			described.WriteString(e.Package)
			described.WriteString(qualifierSep)
		}
		described.WriteString(e.Emitted)
		described.WriteString(collisionIn)
		described.WriteString(e.File)
		if i != 1 {
			related = append(related, r.nameAt(e))
		}
	}
	if receiver := group[0].Receiver; receiver != "" {
		r.errorf(plugin.CollidingNames, r.nameAt(group[1]), related,
			"%s settle to %q on the receiver %s in one package", described.String(), group[0].Settled, receiver)
		return
	}
	r.errorf(plugin.CollidingNames, r.nameAt(group[1]), related,
		"%s settle to %q in one package", described.String(), group[0].Settled)
}

// nameAt positions a finding about a name at the position of its origin,
// and at its file where the origin has none.
func (r *router) nameAt(e *plugin.NameEntry) position.Pos {
	if !e.Origin.IsZero() {
		if s, found := r.in.Index.Lookup(e.Origin); found {
			if pos := s.Position(); !pos.IsZero() {
				return pos
			}
		}
	}
	return position.Pos{File: e.File}
}
