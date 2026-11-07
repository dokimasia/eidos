// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/token"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// These constants are the punctuation of a type expression. They are
// the marks that open a pointer and a variadic parameter, and the
// parentheses and brackets that enclose a type, an array length or an
// argument list.
const (
	pointerMark  = "*"
	variadicMark = "..."
	parenOpen    = "("
	parenClose   = ")"
	bracketOpen  = "["
	bracketClose = "]"
)

// qualifierMark separates a package qualifier from the name it
// qualifies.
const qualifierMark = "."

// shapeMarks are the characters that the spelling of a composite or a
// constraint term contains and a declared name never contains. They
// are brackets, parentheses and braces, the approximation and union
// marks, and the space and tab between the keyword of a composite and
// its elements.
const shapeMarks = "[({ \t~|"

// shapeKeywords are the keywords of the composite types. A stripped
// spelling that equals one of them refers to no single declaration.
var shapeKeywords = []string{"map", "func", "chan", "struct", "interface"}

// The tiers in which [Bindings.Tiers] returns the candidates of a spelling.
const (
	// AliasTier contains the candidate of the import whose alias is the
	// qualifier of a spelling, and the candidates of a bare name.
	AliasTier = 0
	// ImportTier contains a candidate in each import without an alias, in
	// source order. Such an import binds the name in the package clause of
	// the imported package, so [Qualifies] admits a candidate of the tier by
	// that name.
	ImportTier = 1
)

// Bindings contains what the imports of one Go file bind. It has the
// import path that each qualifier binds, the import path that each alias
// binds, the dot-imported paths whose exported names enter the scope of
// the file, and every unaliased import in source order. A qualifier that
// no import binds probes the unaliased imports. [NewBindings] derives the
// bindings from the import records of a file. The resolution step of the
// load, the rules and the reference lowering of the frontend resolve a
// spelling through them. The zero Bindings binds nothing.
//
// # Concurrency
//
// A Bindings value is immutable after [NewBindings] and safe for
// concurrent use.
//
// # Allocation contract
//
// [Bindings.Import] and [Bindings.ImportIn] allocate nothing, and
// [Bindings.Candidates] and [Bindings.Tiers] allocate the lists that they
// return.
type Bindings struct {
	named   map[string]string
	aliases []alias
	dots    []string
	all     []string
}

// alias is the qualifier that an aliased import states and the path that
// it binds the qualifier to.
type alias struct {
	qualifier, path string
}

// NewBindings derives the bindings of a file from its import records.
// An import binds the qualifier that [ImportName] returns, and an
// unaliased import also joins the fallback probe in source order. A dot
// import brings its exported names into the scope of the file, whether
// the source writes it as a wildcard or as [DotAlias]. A blank import
// binds nothing, and NewBindings skips a nil record. Where two imports
// bind one qualifier, the later one stays bound. Go refuses to compile
// such a file.
//
// # Allocation contract
//
// NewBindings sizes the map of qualifiers and the list of unaliased
// imports to the imports of the file, and grows the lists of dot imports
// and of aliases as it meets them. A file without a dot import costs two
// allocations for the map up to eight imports, one for the list of
// unaliased imports, and one for the list of aliases where it has one
// aliased import.
func NewBindings(imports []*node.Import) Bindings {
	b := Bindings{named: make(map[string]string, len(imports)), all: make([]string, 0, len(imports))}
	for _, imp := range imports {
		if imp == nil {
			continue
		}
		if imp.Wildcard || imp.Alias == DotAlias {
			b.dots = append(b.dots, imp.Path)
			continue
		}
		name, binds := ImportName(imp)
		if !binds {
			continue
		}
		b.named[name] = imp.Path
		if imp.Alias == "" {
			b.all = append(b.all, imp.Path)
			continue
		}
		b.aliases = append(b.aliases, alias{qualifier: imp.Alias, path: imp.Path})
	}
	return b
}

// Import returns the import path that a qualifier binds, and false for
// a qualifier that no import binds. It allocates nothing.
func (b Bindings) Import(qualifier string) (string, bool) {
	path, bound := b.named[qualifier]
	return path, bound
}

// ImportIn returns the import path that a qualifier binds where names
// returns the name that the package at an import path declares, and false
// for a qualifier that no import binds. An alias binds the qualifier that
// it states. An import without an alias binds the name that names returns
// for its path, and the name that [AssumedName] returns for a path that
// names does not know. Where two imports without an alias bind the
// qualifier, the later one binds it. A nil names knows no path. ImportIn
// allocates nothing.
func (b Bindings) ImportIn(qualifier string, names func(path string) (string, bool)) (string, bool) {
	if path, aliased := b.aliased(qualifier); aliased {
		return path, true
	}
	var found string
	for _, path := range b.all {
		if declaredName(path, names) == qualifier {
			found = path
		}
	}
	return found, found != ""
}

// Candidates returns the declarations that a spelling can refer to, in
// the probe order of Go, as identities in [Lang] with a package and a
// name and no kind. Every candidate competes with the others, because
// Go refuses to compile a file in which two of them declare the name.
//
// Candidates first strips the decoration of the spelling, because a
// reference keeps its source verbatim. The decoration is pointers,
// slices, arrays, variadic dots, parentheses and the argument list of a
// trailing instantiation, so *List[T] probes the same declarations as
// List. A qualified spelling then probes the import that its qualifier
// binds. Where no import binds the qualifier, it probes every unaliased
// import in source order, because the clause of a package can declare a
// name other than the one that its path assumes. A bare spelling probes
// pkg, the package of the file, and then each dot-imported package in
// source order for an exported name. A predeclared type, a composite
// such as a map, a func, a chan or an inline body, and a constraint
// term return no candidate.
//
// # Allocation contract
//
// Candidates allocates the list that it returns once, at its final
// size, and nothing for a spelling without a candidate.
func (b Bindings) Candidates(pkg, spelling string) []symbol.Identity {
	core := stripped(spelling)
	if core == "" || Predeclared(core) {
		return nil
	}
	if qualifier, name, qualified := strings.Cut(core, qualifierMark); qualified {
		if path, bound := b.named[qualifier]; bound {
			return []symbol.Identity{{Lang: Lang, Package: path, Name: name}}
		}
		if len(b.all) == 0 {
			return nil
		}
		out := make([]symbol.Identity, 0, len(b.all))
		for _, path := range b.all {
			out = append(out, symbol.Identity{Lang: Lang, Package: path, Name: name})
		}
		return out
	}
	return b.bare(pkg, core)
}

// Tiers returns the candidates of a spelling in the tiers of the
// resolution step, as identities in [Lang] with a package and a name and no
// kind. It strips the decoration of the spelling as [Bindings.Candidates]
// does. A bare name has the candidates of Candidates in [AliasTier]. A
// qualified spelling whose qualifier an alias binds has the name in the
// package of that alias in AliasTier, and no other tier. Any other qualified
// spelling has an empty AliasTier and the name in each import without an
// alias in [ImportTier], in source order, because the package clause of
// such an import decides the qualifier that it binds. A file without such an
// import gives it no candidate. A predeclared type, a composite and a
// constraint term have no candidate.
//
// The tiers do not depend on another package, so a record of them stays
// valid while the file is unchanged. [Qualifies] admits each declaration of
// a tier by the name that its package declares.
//
// # Allocation contract
//
// Tiers allocates the list of tiers and the one tier with candidates, two
// allocations, and nothing for a spelling without a candidate.
func (b Bindings) Tiers(pkg, spelling string) [][]symbol.Identity {
	core := stripped(spelling)
	if core == "" || Predeclared(core) {
		return nil
	}
	qualifier, name, qualified := strings.Cut(core, qualifierMark)
	if !qualified {
		return [][]symbol.Identity{b.bare(pkg, core)}
	}
	if path, aliased := b.aliased(qualifier); aliased {
		return [][]symbol.Identity{{{Lang: Lang, Package: path, Name: name}}}
	}
	if len(b.all) == 0 {
		return nil
	}
	tier := make([]symbol.Identity, 0, len(b.all))
	for _, path := range b.all {
		tier = append(tier, symbol.Identity{Lang: Lang, Package: path, Name: name})
	}
	return [][]symbol.Identity{AliasTier: nil, ImportTier: tier}
}

// aliased returns the path that the later aliased import with the
// qualifier binds, and false for a qualifier that no alias states.
func (b Bindings) aliased(qualifier string) (string, bool) {
	for _, a := range slices.Backward(b.aliases) {
		if a.qualifier == qualifier {
			return a.path, true
		}
	}
	return "", false
}

// bare returns the candidates of a bare name: the name in pkg, and an
// exported name in each dot import in source order.
func (b Bindings) bare(pkg, name string) []symbol.Identity {
	dots := 0
	if token.IsExported(name) {
		dots = len(b.dots)
	}
	out := make([]symbol.Identity, 0, 1+dots)
	out = append(out, symbol.Identity{Lang: Lang, Package: pkg, Name: name})
	for _, path := range b.dots[:dots] {
		out = append(out, symbol.Identity{Lang: Lang, Package: path, Name: name})
	}
	return out
}

// Qualifies reports whether a declaration of a package that declares name
// is a target of spelling, where the candidate that names the declaration
// is in the tier of that index among the tiers that [Bindings.Tiers]
// returns. A candidate of [AliasTier] is a target whatever its package
// declares. A candidate of [ImportTier] is a target where its package
// declares the qualifier of the spelling, because a Go import without an
// alias binds the name in the package clause of the imported package. An
// empty name admits the candidate, because no clause of its package was
// read. Qualifies allocates nothing.
func Qualifies(spelling string, tier int, name string) bool {
	if tier != ImportTier || name == "" {
		return true
	}
	qualifier, _, _ := strings.Cut(stripped(spelling), qualifierMark)
	return qualifier == name
}

// PackageBindings is the scope of the imports of the files of one Go
// package, in file order. A file of the package resolves a qualifier
// through them where its own imports do not bind the qualifier, so a
// directive in one file can name a package that another file of the
// package imports. An import binds the qualifier that [ImportNameIn]
// returns for Names. A dot import and a blank import bind no qualifier,
// because a dot import brings its names into its own file alone. A
// qualifier that two files bind to different import paths binds each of
// those paths. A nil file and a nil import record bind nothing, and the
// zero PackageBindings binds nothing.
//
// # Concurrency
//
// A PackageBindings value is safe for concurrent use while no caller
// changes its files, and while Names is safe for concurrent use.
//
// # Allocation contract
//
// Each method reads every import of the files and keeps no index, so its
// cost grows with the imports of the package. [PackageBindings.Import]
// allocates nothing, and [PackageBindings.Candidates] allocates the list
// that it returns.
type PackageBindings struct {
	// Files are the files of the package, in file order.
	Files []*node.File
	// Names returns the name that the package at an import path declares,
	// and false for a package whose clause the caller has not read. A nil
	// Names knows no package, so an import without an alias binds the name
	// that its path assumes.
	Names func(path string) (string, bool)
}

// Import returns the import path that the files bind to a qualifier. It
// reports false for a qualifier that no file binds, and for a qualifier
// that two files bind to different paths. It allocates nothing.
func (b PackageBindings) Import(qualifier string) (string, bool) {
	path, n, conflict := b.bound(qualifier)
	if conflict {
		return "", false
	}
	return path, n > 0
}

// Candidates returns the declarations that a qualified spelling can refer
// to through the imports of the files, as identities in [Lang] with a
// package and a name and no kind. It strips the decoration of the spelling
// as [Bindings.Candidates] does. The imports of a file take precedence, so
// Candidates returns nothing for a spelling whose qualifier the imports of
// file bind. Otherwise the one candidate is the import path that the files
// bind to the qualifier. For a qualifier that two files bind to different
// paths, Candidates returns one candidate for each path in file order and
// reports ambiguous. A qualifier that no file binds has no candidate,
// because the scope of a package reads the names of the imports alone and
// does not probe the unaliased imports as the scope of a file does. A bare
// spelling has no candidate, because a file resolves a bare name in its
// package and in its own dot imports.
//
// # Allocation contract
//
// Candidates allocates the list that it returns once, and nothing for a
// spelling without a candidate.
func (b PackageBindings) Candidates(file Bindings, spelling string) (candidates []symbol.Identity, ambiguous bool) {
	qualifier, name, qualified := strings.Cut(stripped(spelling), qualifierMark)
	if !qualified {
		return nil, false
	}
	if _, bound := file.ImportIn(qualifier, b.Names); bound {
		return nil, false
	}
	path, n, conflict := b.bound(qualifier)
	switch {
	case n == 0:
		return nil, false
	case !conflict:
		return []symbol.Identity{{Lang: Lang, Package: path, Name: name}}, false
	}
	out := make([]symbol.Identity, 0, n)
	for _, f := range b.Files {
		if f == nil {
			continue
		}
		for _, imp := range f.Imports {
			if !b.binds(imp, qualifier) {
				continue
			}
			if id := (symbol.Identity{Lang: Lang, Package: imp.Path, Name: name}); !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out, true
}

// bound returns the import path of the first import of the files that
// binds qualifier, the number of the imports that bind it, and whether two
// of them bind different paths. It allocates nothing.
func (b PackageBindings) bound(qualifier string) (first string, n int, conflict bool) {
	for _, f := range b.Files {
		if f == nil {
			continue
		}
		for _, imp := range f.Imports {
			if !b.binds(imp, qualifier) {
				continue
			}
			if n == 0 {
				first = imp.Path
			}
			conflict = conflict || imp.Path != first
			n++
		}
	}
	return first, n, conflict
}

// binds reports whether an import binds qualifier: its alias, or the name
// that Names returns for its path, as [ImportNameIn] returns it. A blank and
// a dot import bind no qualifier. It allocates nothing.
func (b PackageBindings) binds(imp *node.Import, qualifier string) bool {
	name, binds := ImportNameIn(imp, b.Names)
	return binds && name == qualifier
}

// stripped returns the name inside the decoration of a type expression.
// It returns empty for a spelling that refers to no single declaration,
// such as a composite, a constraint term or an array whose bracket never
// closes.
func stripped(spelling string) string {
	s := strings.TrimSpace(spelling)
	for {
		switch {
		case strings.HasPrefix(s, pointerMark):
			s = s[len(pointerMark):]
		case strings.HasPrefix(s, variadicMark):
			s = s[len(variadicMark):]
		case strings.HasPrefix(s, parenOpen) && strings.HasSuffix(s, parenClose):
			s = strings.TrimSpace(s[len(parenOpen) : len(s)-len(parenClose)])
		case strings.HasPrefix(s, bracketOpen):
			closing := strings.Index(s, bracketClose)
			if closing < 0 {
				return ""
			}
			s = s[closing+len(bracketClose):]
		default:
			if open := strings.Index(s, bracketOpen); open > 0 && strings.HasSuffix(s, bracketClose) {
				s = s[:open]
				continue
			}
			if strings.ContainsAny(s, shapeMarks) || slices.Contains(shapeKeywords, s) {
				return ""
			}
			return s
		}
	}
}
