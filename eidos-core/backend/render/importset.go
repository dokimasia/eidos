// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Entry is one collected import: the path a spelling qualified
// with, the name the import binds in the file, and, for a language
// that imports one declaration at a time, the declaration it binds
// under that name.
//
// A bare path leaves Name empty, which is the side-effect or
// whole-namespace form. A bare and a named entry under one path are
// two entries. TypeOnly marks a binding a language erases at run
// time, such as TypeScript's import type. The renderer joins a value
// binding and a type-only one under one path into one value import.
type Entry struct {
	Path string
	// Name is the local name the import binds: a Go package's name
	// in the file, or the name a TypeScript or Rust import binds one
	// declaration under.
	Name string
	// Item is the declaration an import of one declaration binds,
	// where Name renames it, as TypeScript's { Item as Name } and
	// Rust's Item as Name write it. It is empty where Name spells the
	// declaration unchanged, and for an import of a whole package.
	Item     string
	TypeOnly bool
}

// ImportSet is one file's collected imports: every entry the file's
// spellings qualified with, deduplicated, and the local name each
// binds. The set is per file by rule, filled as a side effect of
// spelling, and the language's Imports renderer turns it into the
// block its own formatter would leave. An entry under the file's
// own package records nothing, because a file never imports its
// own package.
//
// A spelling binds through [ImportSet.Bind], [ImportSet.BindItem] or
// [ImportSet.Claim], which assign local names deterministically: the
// first claimant of a name keeps it, a later one takes the name with
// the lowest free numeric suffix, and a name [ImportSet.Reserve]
// took for a declaration of the file is never bound. The render
// binds in canonical declaration order, so two runs bind alike.
//
// A set can have the names that the package clauses of a run declare.
// [ImportSet.SetNames] sets them, and [ImportSet.Declared] returns the
// name of one package and records its path. [ImportSet.Packages] returns
// the recorded paths, because the file depends on the names of those
// packages.
//
// A set can also have the packages of the files of the plan that the file
// refers into. [ImportSet.SetReferents] sets them for one file, and
// [ImportSet.Referent] returns the package of one such file.
//
// A set has the plan's lowering policy, which [ImportSet.SetPolicy] sets
// for a render and [ImportSet.Policy] returns, so a helper that a
// vocabulary binds to the set reads the policy. A set also has the order
// of the file-level declarations of its file: the order of their
// reservations. [ImportSet.Enter] names the declaration under render, and
// [ImportSet.Pending] reports a name that the declaration under render or
// a later declaration takes. While the pass spells a body,
// [ImportSet.Body] returns the callable of the body and the declaration
// whose template places it. [ImportSet.Has] reports whether the set
// records an entry, so a vocabulary keeps a fact of the file under render
// as an entry under a path that no import has.
//
// An ImportSet is not safe for concurrent use: it belongs to the
// one file under render.
type ImportSet struct {
	entries map[Entry]struct{}
	// journal lists the changes in the order they were made, so the
	// pass can withdraw what a skipped declaration recorded before
	// it failed.
	journal []record
	// home is the package path of the file under render, and pkg its
	// package, with Name set to the name of its package clause.
	home string
	pkg  symbol.Identity
	// locals maps each name bound in the file to its binding.
	locals map[string]binding
	// packages maps a package path to the name it is bound under.
	packages map[string]string
	// items maps one declaration of a path to the name it is bound
	// under.
	items map[binding]string
	// names returns the name that the package clause of a path declares,
	// and is nil for a set without names.
	names func(path string) (string, bool)
	// declared maps each path that Declared looked up in the file to its
	// declared name, and to the empty name where names has none. known
	// counts the paths with a name.
	declared map[string]string
	known    int
	// referents are the packages of the files that the file refers into,
	// sorted by package path, and nil for a file that refers into none.
	referents []symbol.Identity
	// policy is the plan's lowering policy, which a Reset keeps.
	policy plugin.Policy
	// reservations counts the names that Reserve took, and current is the
	// position of the reservation of the declaration under render.
	reservations int
	current      int
	// callable is the function or the method whose body the pass spells,
	// and host is the declaration whose template places the body. Both are
	// nil outside a body.
	callable, host symbol.Symbol
}

// binding is what a local name refers to: a whole package, one
// declaration of a path, or a name the file declares itself. index is the
// position of a reservation among the reservations of the file, and zero
// for every other binding.
type binding struct {
	path, item string
	reserved   bool
	index      int
}

// record is one journaled change: the entry it added, where added
// reports one, and the local name it bound, where local names one. A
// rollback withdraws both.
type record struct {
	entry Entry
	added bool
	local string
	bound binding
}

// Home returns the package path of the file the set collects for,
// and empty where none is named.
func (s *ImportSet) Home() string { return s.home }

// SetHome names the package path of the file the set collects for.
// The render pass names it for every file through [ImportSet.SetPackage];
// an entry under it records nothing.
func (s *ImportSet) SetHome(path string) { s.home = path }

// SetPackage names the package of the file the set collects for, with Name
// set to the name of its package clause, and makes the path of the package
// the home of the set, as [ImportSet.SetHome] does. The render pass sets it
// for every file. A language that qualifies a reference by the name of a
// package clause compares the package of the file with the package of the
// file of the referent, which [ImportSet.Referent] returns.
func (s *ImportSet) SetPackage(pkg symbol.Identity) {
	s.pkg = pkg
	s.home = pkg.Package
}

// Package returns the package of the file the set collects for, and the
// zero identity where [ImportSet.SetPackage] named none. It allocates
// nothing.
func (s *ImportSet) Package() symbol.Identity { return s.pkg }

// SetNames sets the function that returns the name that the package
// clause of a path declares. The function reports false for a path that
// the run did not load. The render pass sets the names of its context on
// the set of each worker. A Reset keeps the names, and a nil function
// removes them.
func (s *ImportSet) SetNames(names func(path string) (string, bool)) { s.names = names }

// Declared returns the name that the package clause of path declares. It
// reports false where the set has no names, and where the names have no
// name for path. A path with a name joins the packages of the file, and
// [ImportSet.Packages] returns them. A language that binds a package under
// its declared name calls Declared before [ImportSet.Bind]. Declared calls
// the names once for each path of a file.
//
// # Allocation contract
//
// Declared allocates nothing for a set without names, and nothing for a
// path that the file looked up before. The first lookup of a set allocates
// the map of its lookups, and a Reset keeps the map.
func (s *ImportSet) Declared(path string) (string, bool) {
	if s.names == nil {
		return "", false
	}
	if name, looked := s.declared[path]; looked {
		return name, name != ""
	}
	var name string
	if declared, known := s.names(path); known {
		name = declared
	}
	if s.declared == nil {
		s.declared = map[string]string{}
	}
	s.declared[path] = name
	if name != "" {
		s.known++
	}
	return name, name != ""
}

// SetReferents sets the packages of the other files that the file refers
// into, which [plugin.File.Referents] lists sorted by package path. The
// render pass sets them for each file after the Reset of the set. The set
// keeps refs as it is, so the caller does not change it afterwards.
func (s *ImportSet) SetReferents(refs []symbol.Identity) { s.referents = refs }

// Referent returns the package of the file at the package path path, and
// reports false where the file under render does not refer into that
// file. A language whose package clause does not follow from the package
// path reads the clause of such a file from the Name of the package.
// Referent searches the sorted referents and allocates nothing.
func (s *ImportSet) Referent(path string) (symbol.Identity, bool) {
	i, held := slices.BinarySearchFunc(s.referents, path, func(id symbol.Identity, p string) int {
		return cmp.Compare(id.Package, p)
	})
	if !held {
		return symbol.Identity{}, false
	}
	return s.referents[i], true
}

// Reserve takes names for the declarations of the file itself, so
// no import binds one of them. The render pass reserves every
// file-level declaration's name before the first declaration
// renders, in the order in which the declarations render. An empty
// name reserves nothing, and a name already bound keeps its binding,
// so the first declaration of a name gives the name its position.
func (s *ImportSet) Reserve(names ...string) {
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, held := s.locals[name]; held {
			continue
		}
		if s.locals == nil {
			s.locals = map[string]binding{}
		}
		s.locals[name] = binding{reserved: true, index: s.reservations}
		s.reservations++
	}
}

// Reserved reports whether a declaration of the file took name through
// [ImportSet.Reserve]. A language with global names checks it before it
// writes a global name without an import, because a declaration of the
// file with that name hides the global. Reserved allocates nothing.
func (s *ImportSet) Reserved(name string) bool {
	b, held := s.locals[name]
	return held && b.reserved
}

// Enter names the file-level declaration under render by the name that
// [ImportSet.Reserve] took for it. The render pass enters each declaration
// before its template runs, and a cluster by its first declaration. A
// name that no declaration of the file took enters nothing, and the
// declaration entered before stays under render. Enter allocates nothing.
func (s *ImportSet) Enter(name string) {
	if b, held := s.locals[name]; held && b.reserved {
		s.current = b.index
	}
}

// Pending reports whether name is the name of a file-level declaration of
// the file that has not finished rendering: the declaration under render,
// or a declaration that the render writes after it. Before the first
// [ImportSet.Enter], every declaration of the file is pending. A language
// that runs the declarations of a file in order, such as Python, writes a
// reference to a pending name in a form that it evaluates later. Pending
// reports false for a name that no declaration of the file takes, and
// allocates nothing.
func (s *ImportSet) Pending(name string) bool {
	b, held := s.locals[name]
	return held && b.reserved && b.index >= s.current
}

// Body returns the callable whose body the pass spells and the declaration
// whose template places the body. The host of a method is the type whose
// template places the method, and the host of a function is the function.
// A body that the template of a claimed file places has no host. Body
// returns nil for both outside a body. The pass names the two for the
// statements and the reference template of each body, so a language whose
// statements spell a name that the callable or its host declares, such as
// a type parameter, finds the declaration. Body allocates nothing.
func (s *ImportSet) Body() (callable, host symbol.Symbol) { return s.callable, s.host }

// SetPolicy sets the plan's lowering policy, which [ImportSet.Policy]
// returns. The render pass sets the policy of its context on the set of
// each worker before it binds the vocabulary to the set. A Reset keeps
// the policy, because the files of one render share it.
func (s *ImportSet) SetPolicy(p plugin.Policy) { s.policy = p }

// Policy returns the plan's lowering policy that [ImportSet.SetPolicy]
// set, and the zero Policy for a set without one. It allocates nothing.
func (s *ImportSet) Policy() plugin.Policy { return s.policy }

// Bind imports a whole package and returns the name the file refers
// to it by: name where no other binding and no declaration of the
// file takes it, and otherwise name with the lowest free numeric
// suffix. A path bound before returns its name again, and the file's
// own package returns empty, because a file never qualifies its own
// names. Go binds its imports through it.
func (s *ImportSet) Bind(path, name string) string {
	if path == "" || path == s.home {
		return ""
	}
	if local, held := s.packages[path]; held {
		return local
	}
	bound := binding{path: path}
	local := s.free(name, bound)
	if s.packages == nil {
		s.packages = map[string]string{}
	}
	s.packages[path] = local
	s.bind(Entry{Path: path, Name: local}, local, bound)
	return local
}

// BindItem imports one declaration of a path and returns the name
// the file refers to it by, assigned the way [ImportSet.Bind]
// assigns one, the declaration's own name first. A declaration of
// the file's own package returns its name unchanged and records
// nothing. A declaration bound before returns its name again, and a
// value binding beside a type-only one records the value entry too.
// TypeScript and Rust bind their imports through it.
func (s *ImportSet) BindItem(path, item string, typeOnly bool) string {
	if path == "" || path == s.home {
		return item
	}
	bound := binding{path: path, item: item}
	if local, held := s.items[bound]; held {
		s.add(itemEntry(path, item, local, typeOnly))
		return local
	}
	local := s.free(item, bound)
	if s.items == nil {
		s.items = map[binding]string{}
	}
	s.items[bound] = local
	s.bind(itemEntry(path, item, local, typeOnly), local, bound)
	return local
}

// Claim imports one declaration under its own simple name and
// reports whether the name is the file's: true where the name is
// free or already binds this declaration, false where another
// import or a declaration of the file takes it. Java claims its
// imports: a false result means the spelling is written fully
// qualified.
//
// A declaration of the file's own package takes its name and records
// no entry, because Java needs no import for it and an import of the
// same simple name would shadow it. An empty path reports true and
// records nothing.
func (s *ImportSet) Claim(path, name string) bool {
	if path == "" {
		return true
	}
	bound := binding{path: path, item: name}
	if cur, held := s.locals[name]; held {
		return cur == bound
	}
	if s.items == nil {
		s.items = map[binding]string{}
	}
	s.items[bound] = name
	if path == s.home {
		s.bindName(name, bound)
		return true
	}
	s.bind(Entry{Path: path, Name: name}, name, bound)
	return true
}

// Use records the import of path that a template states and binds the
// name local in the name assignment of the file. item is the declaration
// of path that local refers to, and empty where local refers to the
// whole package. A later [ImportSet.Bind] of path, or a later
// [ImportSet.BindItem] of the item, returns local. A later binding of
// another path that asks for local takes the name with the lowest free
// numeric suffix. A second Use of the same binding under the same name
// records nothing.
//
// Use returns an error where local binds another path, another item or
// a declaration of the file, and where the file binds the import under
// another name. The template refers to the import by local, so a suffix
// would break the reference. For the file's own package and for an
// empty path, Use does not record an entry and returns nil.
//
// # Allocation contract
//
// Use allocates what Bind allocates for a new binding, and nothing for a
// binding that it made before under local.
func (s *ImportSet) Use(path, local, item string) error {
	if path == "" || path == s.home {
		return nil
	}
	bound := binding{path: path, item: item}
	if cur, held := s.locals[local]; held {
		if cur != bound {
			return fmt.Errorf(
				"render: the template imports %q as %s, which the file binds to another import or declaration",
				path,
				local,
			)
		}
		return nil
	}
	if item == "" {
		if other, held := s.packages[path]; held {
			return fmt.Errorf(
				"render: the template imports %q as %s, and the file imports it as %s",
				path,
				local,
				other,
			)
		}
		if s.packages == nil {
			s.packages = map[string]string{}
		}
		s.packages[path] = local
		s.bind(Entry{Path: path, Name: local}, local, bound)
		return nil
	}
	if other, held := s.items[bound]; held {
		return fmt.Errorf("render: the template imports %s of %q as %s, and the file imports it as %s",
			item, path, local, other)
	}
	if s.items == nil {
		s.items = map[binding]string{}
	}
	s.items[bound] = local
	s.bind(itemEntry(path, item, local, false), local, bound)
	return nil
}

// Add records one bare import path; recording a path twice records
// one. The entry binds no name, so it is outside the name
// assignment: the template that recorded it is responsible for the
// names its import brings in.
func (s *ImportSet) Add(path string) {
	s.add(Entry{Path: path})
}

// AddNamed records the name an import of path binds, as a template
// writes it; recording a pair twice records one. Like [ImportSet.Add],
// it is outside the name assignment.
func (s *ImportSet) AddNamed(path, name string) {
	s.add(Entry{Path: path, Name: name})
}

// AddType records a type-only binding: the name an import of path
// binds for the type checker alone, erased at run time where the
// language erases one. Like [ImportSet.Add], it is outside the name
// assignment.
func (s *ImportSet) AddType(path, name string) {
	s.add(Entry{Path: path, Name: name, TypeOnly: true})
}

// Has reports whether the set records the entry of path and name that
// [ImportSet.Add] records for an empty name and [ImportSet.AddNamed]
// records otherwise, so a language reads back what it recorded for the
// file under render. Has allocates nothing.
func (s *ImportSet) Has(path, name string) bool {
	_, held := s.entries[Entry{Path: path, Name: name}]
	return held
}

// Paths returns every recorded path, distinct and sorted, so two
// renders spell one block. A renderer that binds no names reads
// nothing else. It allocates the list it returns, and nothing for an
// empty set.
func (s *ImportSet) Paths() []string {
	if len(s.entries) == 0 {
		return nil
	}
	paths := make([]string, 0, len(s.entries))
	for e := range s.entries {
		paths = append(paths, e.Path)
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// Entries returns every recorded entry, sorted by path, name, item,
// then value before type-only, so two renders spell one block. It
// allocates the list it returns, and nothing for an empty set.
func (s *ImportSet) Entries() []Entry {
	if len(s.entries) == 0 {
		return nil
	}
	entries := make([]Entry, 0, len(s.entries))
	for e := range s.entries {
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Item, b.Item),
			boolCmp(a.TypeOnly, b.TypeOnly),
		)
	})
	return entries
}

// Packages returns every path whose declared name the file read through
// [ImportSet.Declared], distinct and sorted, and nil where the file read
// none. It allocates the list it returns, and nothing for nil.
func (s *ImportSet) Packages() []string {
	if s.known == 0 {
		return nil
	}
	paths := make([]string, 0, s.known)
	for p, name := range s.declared {
		if name != "" {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	return paths
}

// boolCmp orders false before true.
func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}

// Len returns how many distinct paths the set records, which
// decides whether a block renders at all.
func (s *ImportSet) Len() int {
	paths := make(map[string]struct{}, len(s.entries))
	for e := range s.entries {
		paths[e.Path] = struct{}{}
	}
	return len(paths)
}

// Reset drops every entry, binding, reservation, declared name and
// referent of the file, and the declaration under render. It keeps the
// storage, the names and the policy, so the pass reuses one set across a
// call's files.
func (s *ImportSet) Reset() {
	clear(s.entries)
	clear(s.locals)
	clear(s.packages)
	clear(s.items)
	clear(s.declared)
	s.known = 0
	s.referents = nil
	s.reservations = 0
	s.current = 0
	s.journal = s.journal[:0]
}

// free returns the name a binding takes: name where it is free or
// already refers to the binding, and otherwise name with the
// lowest numeric suffix, from 2, that is.
func (s *ImportSet) free(name string, b binding) string {
	candidate := name
	for n := 2; ; n++ {
		if cur, held := s.locals[candidate]; !held || cur == b {
			return candidate
		}
		candidate = name + strconv.Itoa(n)
	}
}

// bind records an entry that binds a local name, journaling both, so
// a rollback withdraws the name with the entry. An entry recorded
// before is kept, and a rollback leaves it.
func (s *ImportSet) bind(e Entry, local string, b binding) {
	if s.locals == nil {
		s.locals = map[string]binding{}
	}
	s.locals[local] = b
	r := record{local: local, bound: b}
	if s.entries == nil {
		s.entries = map[Entry]struct{}{}
	}
	if _, held := s.entries[e]; !held {
		s.entries[e] = struct{}{}
		r.entry, r.added = e, true
	}
	s.journal = append(s.journal, r)
}

// bindName binds a local name and records no entry, journaled, for a
// declaration of the file's own package: the name is taken and
// nothing is imported.
func (s *ImportSet) bindName(local string, b binding) {
	if s.locals == nil {
		s.locals = map[string]binding{}
	}
	s.locals[local] = b
	s.journal = append(s.journal, record{local: local, bound: b})
}

// add records one entry and journals it when it is new. An entry
// under the file's own package records nothing.
func (s *ImportSet) add(e Entry) {
	if s.home != "" && e.Path == s.home {
		return
	}
	if s.entries == nil {
		s.entries = map[Entry]struct{}{}
	}
	if _, held := s.entries[e]; held {
		return
	}
	s.entries[e] = struct{}{}
	s.journal = append(s.journal, record{entry: e, added: true})
}

// mark returns the journal position a later rollback returns to.
func (s *ImportSet) mark() int { return len(s.journal) }

// rollback withdraws every change made after the mark: each entry
// first recorded there, and each name bound there.
func (s *ImportSet) rollback(mark int) {
	for _, r := range slices.Backward(s.journal[mark:]) {
		if r.added {
			delete(s.entries, r.entry)
		}
		if r.local == "" {
			continue
		}
		delete(s.locals, r.local)
		if r.bound.item == "" {
			delete(s.packages, r.bound.path)
			continue
		}
		delete(s.items, r.bound)
	}
	s.journal = s.journal[:mark]
}

// itemEntry spells the entry an import of one declaration records:
// the item named apart where the local name renames it.
func itemEntry(path, item, local string, typeOnly bool) Entry {
	e := Entry{Path: path, Name: local, TypeOnly: typeOnly}
	if local != item {
		e.Item = item
	}
	return e
}
