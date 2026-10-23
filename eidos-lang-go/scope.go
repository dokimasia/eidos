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

// Bindings contains what the imports of one Go file bind. It has the
// import path that each qualifier binds, the dot-imported paths whose
// exported names enter the scope of the file, and every unaliased
// import in source order. A qualifier that no import binds probes the
// unaliased imports. [NewBindings] derives the bindings from the import
// records of a file. The resolution step of the load, the rules and the
// reference lowering of the frontend resolve a spelling through them.
// The zero Bindings binds nothing.
//
// # Concurrency
//
// A Bindings value is immutable after [NewBindings] and safe for
// concurrent use.
//
// # Allocation contract
//
// [Bindings.Import] allocates nothing, and [Bindings.Candidates]
// allocates the list that it returns.
type Bindings struct {
	named map[string]string
	dots  []string
	all   []string
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
// imports to the imports of the file, and grows the list of dot imports
// as it meets them. A file without a dot import costs two allocations
// for the map up to eight imports, and one for the list.
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
		}
	}
	return b
}

// Import returns the import path that a qualifier binds, and false for
// a qualifier that no import binds. It allocates nothing.
func (b Bindings) Import(qualifier string) (string, bool) {
	path, bound := b.named[qualifier]
	return path, bound
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
	dots := 0
	if token.IsExported(core) {
		dots = len(b.dots)
	}
	out := make([]symbol.Identity, 0, 1+dots)
	out = append(out, symbol.Identity{Lang: Lang, Package: pkg, Name: core})
	for _, path := range b.dots[:dots] {
		out = append(out, symbol.Identity{Lang: Lang, Package: path, Name: core})
	}
	return out
}

// PackageBindings is the scope of the imports of the files of one Go
// package, in file order. A file of the package resolves a qualifier
// through them where its own imports do not bind the qualifier, so a
// directive in one file can name a package that another file of the
// package imports. An import binds the qualifier that [ImportName]
// returns. A dot import and a blank import bind no qualifier, because a
// dot import brings its names into its own file alone. A qualifier that
// two files bind to different import paths binds each of those paths. A
// nil file and a nil import record bind nothing, and a nil
// PackageBindings binds nothing.
//
// # Concurrency
//
// A PackageBindings value is safe for concurrent use while no caller
// changes its files.
//
// # Allocation contract
//
// Each method reads every import of the files and keeps no index, so its
// cost grows with the imports of the package. [PackageBindings.Import]
// allocates nothing, and [PackageBindings.Candidates] allocates the list
// that it returns.
type PackageBindings []*node.File

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
	if _, bound := file.Import(qualifier); bound {
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
	for _, f := range b {
		if f == nil {
			continue
		}
		for _, imp := range f.Imports {
			if q, binds := ImportName(imp); !binds || q != qualifier {
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
	for _, f := range b {
		if f == nil {
			continue
		}
		for _, imp := range f.Imports {
			if name, binds := ImportName(imp); !binds || name != qualifier {
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
