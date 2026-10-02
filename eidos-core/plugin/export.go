// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
)

// ExportKey identifies an exported declaration the way a dependent
// knows it before the producing plan runs: the source declaration it
// derives from, the plugin that emitted it, the family it was emitted
// into, and the names the plugin gave it and its host. Both names are
// the emitted ones, before the settle respelled them, so a dependent
// builds a key from the producing plugin's conventions and reads the
// target's spelling from the export.
type ExportKey struct {
	// Origin is the source declaration the generated one derives from,
	// and the zero identity for a declaration without one.
	Origin symbol.Identity
	// Plugin is the plugin whose unit declared it.
	Plugin ID
	// Tag is the family the unit was emitted into, empty for the
	// primary family.
	Tag string
	// Host is the emitted name of the declaration a member is declared
	// in, or of the type a method's receiver names. It is empty for
	// any other declaration at file level.
	Host string
	// Name is the emitted name.
	Name string
}

// compare orders keys by origin, then by plugin, tag, host and name:
// the order an export lists its declarations in.
func (k ExportKey) compare(o ExportKey) int {
	return cmp.Or(
		k.Origin.Compare(o.Origin),
		strings.Compare(string(k.Plugin), string(o.Plugin)),
		strings.Compare(k.Tag, o.Tag),
		strings.Compare(k.Host, o.Host),
		strings.Compare(k.Name, o.Name),
	)
}

// ExportedSymbol is one declaration of an export.
type ExportedSymbol struct {
	ExportKey
	// Kind is the declaration's kind.
	Kind symbol.Kind
	// Spelling is the name the producing plan's settle gave the
	// declaration: the name the plan's files declare it under.
	Spelling string
	// Package is the package the declaration's file declares, as the
	// producing target names it, with Name set to the name of its
	// package clause. It is the zero identity where the target derives
	// no package for the path.
	Package symbol.Identity
	// File is the workspace-relative, slash-separated path of the file
	// the declaration was rendered into.
	File string
}

// ExportDoc is one plan's export. It lists every declaration in the
// files the plan rendered, each with the name the plan's settle gave it
// and the file and package its layout routed it to. A dependent plan
// reads names and import paths from it instead of applying the
// producing target's naming rules a second time.
//
// The run builds the export of a plan that a dependent plan or a
// workspace check reads, after the plan renders, through [NewExport].
// Every dependent and every check of the run reads the same value, so a
// reader does not mutate it.
type ExportDoc struct {
	// Plan is the producing plan's name.
	Plan string
	// Symbols are the exported declarations, sorted by key, then by
	// file.
	Symbols []ExportedSymbol
}

// NewExport returns a plan's export: every declaration of the units of
// files, and every name the settle visits inside them except
// parameters, results and type parameters, so a type's fields and
// methods and an enum's values are listed beside the type. Each is keyed
// by its origin, its unit's plugin and tag, and the names it and its
// host were emitted under, and it is spelled as the settle left it, at
// its file's path and package. files are the files the plan rendered.
// settled is the store the plan's settle ran over, whose record supplies
// the emitted name of each declaration a respell changed. A nil store
// reads every name as emitted.
//
// A method attached to a receiver is keyed under the emitted name of
// the type its receiver names: the settle rewrote the receiver's
// spelling with the type, and the type's record maps it back. A receiver
// that names a type no file declares keeps its spelling as the host.
//
// The walk visits names through [emit.RespellNames], with visitors that
// return every name unchanged, so it writes back the spellings it reads.
// It walks the names twice, once to count them and once to list them,
// and sorts the result once. A member's host is the nearest declaration
// the walk entered and has not left, which a stack of four entries
// tracks, and a method's receiver maps back through one binary search
// over the file-level declarations, sorted once. The symbols share their
// strings with the store.
//
// # Allocation contract
//
// NewExport allocates two slices: the result, sized from the count, and
// the sorted file-level declarations. A host nested more than four deep
// grows the stack onto the heap.
func NewExport(plan string, files []File, settled *Emit) ExportDoc {
	was := func(d symbol.Symbol, name string) string {
		if settled != nil {
			if emitted, respelled := settled.emitted[d]; respelled {
				return emitted
			}
		}
		return name
	}
	decls, names := 0, 0
	count := func(_, _ symbol.Symbol, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		if listed(kind) {
			names++
		}
		return name, nil
	}
	for _, f := range files {
		for _, u := range f.Units {
			decls += len(u.Decls)
			for _, d := range u.Decls {
				_ = emit.RespellNames(d, count)
			}
		}
	}
	declared := make([]declaredName, 0, decls)
	for _, f := range files {
		for _, u := range f.Units {
			for _, d := range u.Decls {
				if name := emit.DeclaredName(d); name != "" {
					declared = append(declared, declaredName{pkg: u.Pkg.Package, settled: name, emitted: was(d, name)})
				}
			}
		}
	}
	slices.SortFunc(declared, declaredName.compare)
	symbols := make([]ExportedSymbol, 0, names)
	var entered [hostDepth]hostName
	open := entered[:0]
	var file *File
	var unit *Unit
	visit := func(host, carrier symbol.Symbol, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		if !listed(kind) {
			return name, nil
		}
		key := ExportKey{Plugin: unit.Plugin, Tag: unit.Tag, Name: was(carrier, name)}
		key.Origin, _ = emit.OriginOf(carrier)
		for len(open) > 0 && open[len(open)-1].decl != host {
			open = open[:len(open)-1]
		}
		if len(open) > 0 {
			key.Host = open[len(open)-1].name
		} else if m, attached := carrier.(*emit.Method); attached && host == nil && m.Receives != nil {
			key.Host = emittedOf(declared, unit.Pkg.Package, m.Receives.Spelling)
		}
		open = append(open, hostName{decl: carrier, name: key.Name})
		symbols = append(symbols, ExportedSymbol{
			ExportKey: key, Kind: kind, Spelling: name, Package: file.Pkg, File: file.Path,
		})
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
	slices.SortFunc(symbols, func(a, b ExportedSymbol) int {
		return cmp.Or(a.compare(b.ExportKey), strings.Compare(a.File, b.File))
	})
	return ExportDoc{Plan: plan, Symbols: symbols}
}

// Find returns the exported declarations under one key, in file
// order: one for a key one file declares, more than one where the
// plugin emitted one name from one origin into one family more than
// once, and none where the export lists nothing under the key. The
// result is the export's own storage, capped at its last match, and the
// caller does not mutate it. Find costs one binary search and allocates
// nothing.
func (d ExportDoc) Find(k ExportKey) []ExportedSymbol {
	lo, _ := slices.BinarySearchFunc(d.Symbols, k, func(s ExportedSymbol, k ExportKey) int {
		return s.compare(k)
	})
	hi := lo
	for hi < len(d.Symbols) && d.Symbols[hi].ExportKey == k {
		hi++
	}
	return d.Symbols[lo:hi:hi]
}

// hostDepth is how many nested hosts the export's walk tracks before its
// stack grows onto the heap: a file-level declaration, a variant and a
// member, and one to spare.
const hostDepth = 4

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

// hostName is one declaration the export's walk entered, with the name
// it was emitted under: the host a member's key names.
type hostName struct {
	decl symbol.Symbol
	name string
}

// listed reports whether an export lists a name of one kind: every kind
// but a parameter, a result and a type parameter, which no dependent
// names from outside its callable.
func listed(k symbol.Kind) bool {
	return k != symbol.KindParam && k != symbol.KindReturn && k != symbol.KindTypeParam
}

// emittedOf returns the name a file-level declaration of a package was
// emitted under, found by the name the settle left it with in declared,
// which is sorted, and the settled name itself where no file of the
// export declares it.
func emittedOf(declared []declaredName, pkg, settled string) string {
	at, found := slices.BinarySearchFunc(declared, declaredName{pkg: pkg, settled: settled}, declaredName.compare)
	if !found {
		return settled
	}
	return declared[at].emitted
}
