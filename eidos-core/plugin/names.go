// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// Names is the name table of the files that a warm run keeps without
// generating them again. [SettleWith] resolves a reference that its store
// does not declare against it, and the layout qualifies a reference into a
// kept file of another package through it. A plan reads the kept names of a
// collision scope through it.
//
// # Concurrency
//
// A run reads one Names from the goroutine of one plan, so an
// implementation needs no lock for that use.
type Names interface {
	// InPackage returns the entry that a file-level declaration of the
	// package declares under an emitted name, and false where none does.
	// A method is no such declaration, because no bare reference names
	// one. Where declarations of the package declare the emitted name
	// under different settled names, the entry reports Ambiguous.
	InPackage(pkg, emitted string) (NameEntry, bool)
	// OfOrigin returns the entry that a file-level declaration derived
	// from the origin declares under an emitted name, and false where
	// none does.
	OfOrigin(origin symbol.Identity, emitted string) (NameEntry, bool)
	// InScope returns the entries of one collision scope, sorted by
	// settled name: the methods of the package that attach to the
	// receiver where receiver is set, and every other file-level name of
	// the package where it is empty.
	InScope(pkg, receiver string) []NameEntry
}

// NameEntry is one file-level name of a plan's file: what the name
// table of a kept file lists.
//
// # Concurrency
//
// A NameEntry is a value, and any number of goroutines may read one.
type NameEntry struct {
	// Package is the package path of the unit that declares the name.
	Package string
	// Receiver is the emitted spelling of the type that a method
	// attaches to, and empty for every other declaration. A name
	// collides with the other names of its package and receiver.
	Receiver string
	// Origin is the declaration's origin, and the zero identity for a
	// declaration without one.
	Origin symbol.Identity
	// Emitted is the name the declaration was emitted under, and
	// Settled the name the settle left it with.
	Emitted string
	Settled string
	// Kind is the declaration's kind.
	Kind symbol.Kind
	// Ambiguous reports that more than one declaration of the package
	// declares the emitted name under different settled names. A bare
	// reference to such a name is left as written.
	Ambiguous bool
	// File is the path the layout routed the declaration to, and
	// FilePkg the package that the file declares.
	File    string
	FilePkg symbol.Identity
}

// NameKey names one entry that a reference looks up: a bare reference's
// package and emitted name, or a resolved reference's origin and emitted
// name. Exactly one of Package and Origin is set.
//
// # Concurrency
//
// A NameKey is a value, and any number of goroutines may read one.
type NameKey struct {
	Package string
	Origin  symbol.Identity
	Emitted string
}

// Compare orders two keys by package, then by origin, then by emitted
// name, and returns a negative number, zero or a positive number as k
// sorts before, with or after o. It allocates nothing.
func (k NameKey) Compare(o NameKey) int {
	return cmp.Or(
		strings.Compare(k.Package, o.Package),
		k.Origin.Compare(o.Origin),
		strings.Compare(k.Emitted, o.Emitted),
	)
}

// NameRead is one entry that a unit's references looked up during a
// settle, whether a file declares the entry or not: a lookup that found
// nothing is what makes the unit's file generate again when the name
// appears. Unit is the unit's place in [Emit.Units] order.
type NameRead struct {
	Unit int
	Key  NameKey
}

// FactRead is one fact that the settle read for a unit: the name
// override of the origin of one of the unit's declarations, present or
// not. Unit is the unit's place in [Emit.Units] order.
type FactRead struct {
	Unit int
	Fact meta.FactRef
}

// Settled is what one [SettleWith] call read for each unit of its store:
// what makes the unit's file generate again when another file's names or
// the overrides change. Each list is sorted by unit, then by key, and
// lists a unit's read once.
type Settled struct {
	// Read are the entries that the references of each unit looked up.
	Read []NameRead
	// Facts are the name overrides that the settle read for each unit.
	Facts []FactRead
}

// NamesOf returns the file-level names of a plan's routed files, each
// with the file that declares it, in file order and then in the order of
// each file's declarations. settled is the store that the plan's settle
// ran over, whose record supplies the name that each respelled
// declaration was emitted under. A nil store reads every name as
// emitted. A method's receiver maps back to the name that its type was
// emitted under. The files' own declarations map it first, and others
// map it second. A receiver whose type neither declares keeps its
// spelling. Ambiguous is false on every entry, because the ambiguity of
// a name depends on every file of the plan.
//
// # Allocation contract
//
// NamesOf allocates the result, sized from a count of the names, and the
// sorted list of the files' file-level declarations. A receiver that maps
// back through others allocates what others.InScope returns.
func NamesOf(files []File, settled *Emit, others Names) []NameEntry {
	n := 0
	for _, f := range files {
		for _, u := range f.Units {
			n += len(u.Decls)
		}
	}
	declared := declaredNames(files, settled, n)
	out := make([]NameEntry, 0, n)
	var file *File
	var unit *Unit
	// The walk visits a declaration's own name with no host, and its
	// members' names under it, which no entry lists.
	visit := func(host, carrier symbol.Symbol, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		if host != nil {
			return name, nil
		}
		entry := NameEntry{
			Package: unit.Pkg.Package, Kind: kind, Emitted: emittedName(settled, carrier, name), Settled: name,
			File: file.Path, FilePkg: file.Pkg,
		}
		entry.Origin, _ = emit.OriginOf(carrier)
		if m, attached := carrier.(*emit.Method); attached && m.Receives != nil {
			entry.Receiver = hostOf(declared, others, unit.Pkg.Package, m.Receives.Spelling)
		}
		out = append(out, entry)
		return name, nil
	}
	for i := range files {
		file = &files[i]
		for j := range file.Units {
			unit = &file.Units[j]
			for _, d := range unit.Decls {
				_ = emit.RespellNames(d, visit)
			}
		}
	}
	return out
}

// declaredName is one file-level declaration under the package its unit
// declares it in, with the name the settle left it with and the name it
// was emitted under: how a method's receiver spelling finds the emitted
// name of the type it names.
type declaredName struct {
	pkg     string
	settled string
	emitted string
}

// compare orders declarations by package, then by settled name.
func (d declaredName) compare(o declaredName) int {
	return cmp.Or(strings.Compare(d.pkg, o.pkg), strings.Compare(d.settled, o.settled))
}

// declaredNames returns the file-level declarations of files that have a
// declared name, each under the package of its unit, sorted by package
// and then by settled name: how a method's receiver finds the name that
// its type was emitted under. n sizes the list.
func declaredNames(files []File, settled *Emit, n int) []declaredName {
	declared := make([]declaredName, 0, n)
	for _, f := range files {
		for _, u := range f.Units {
			for _, d := range u.Decls {
				if name := emit.DeclaredName(d); name != "" {
					declared = append(declared, declaredName{
						pkg: u.Pkg.Package, settled: name, emitted: emittedName(settled, d, name),
					})
				}
			}
		}
	}
	slices.SortFunc(declared, declaredName.compare)
	return declared
}

// emittedName returns the name that a declaration was emitted under: the
// name the store recorded where the settle respelled it, and name
// otherwise. A nil store records nothing.
func emittedName(settled *Emit, d symbol.Symbol, name string) string {
	if settled != nil {
		if emitted, respelled := settled.emitted[d]; respelled {
			return emitted
		}
	}
	return name
}

// hostOf returns the name that the type a receiver spells was emitted
// under: through declared, which is sorted by package and settled name,
// then through the kept names of the package's scope in others, and the
// spelling itself where neither declares the type.
func hostOf(declared []declaredName, others Names, pkg, spelling string) string {
	key := declaredName{pkg: pkg, settled: spelling}
	if at, found := slices.BinarySearchFunc(declared, key, declaredName.compare); found {
		return declared[at].emitted
	}
	if others == nil {
		return spelling
	}
	kept := others.InScope(pkg, "")
	at, found := slices.BinarySearchFunc(kept, spelling, func(e NameEntry, settled string) int {
		return strings.Compare(e.Settled, settled)
	})
	if !found {
		return spelling
	}
	return kept[at].Emitted
}
