// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The punctuation of a type expression: the marks that open a
// pointer and a variadic parameter, and the parentheses and brackets
// that enclose a type, an array length or an argument list.
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

// shapeMarks are the characters a composite's or a constraint term's
// spelling contains and no declared name does: brackets, parentheses
// and braces, the approximation and union marks, and the space and
// tab between a composite's keyword and its elements.
const shapeMarks = "[({ \t~|"

// shapeKeywords are the composite keywords a stripped spelling can
// be, each naming a shape no single declaration declares.
var shapeKeywords = []string{"map", "func", "chan", "struct", "interface"}

// Scope is what one Go file's imports bind: the import path each
// qualifier binds, the dot-imported paths whose exported names enter
// the file's scope, and every unaliased import in source order, which
// a qualifier no import binds probes. [NewScope] derives it from the
// file's import records, and the load's resolution step, the rules and
// the frontend's reference lowering resolve a spelling through it. The
// zero Scope binds nothing.
//
// A Scope is immutable after [NewScope] and safe for concurrent use.
type Scope struct {
	named map[string]string
	dots  []string
	all   []string
}

// NewScope derives a file's scope from its import records. An import
// binds the qualifier [ImportName] returns, and an unaliased one also
// joins the fallback probe in source order. A dot import, spelled as
// a wildcard or as [DotAlias], brings its exported names into the
// file's scope, and a blank import binds nothing. A nil record is
// skipped. Two imports that bind one qualifier leave the later one
// bound, a file Go refuses to compile.
func NewScope(imports []*node.Import) Scope {
	s := Scope{named: map[string]string{}}
	for _, imp := range imports {
		if imp == nil {
			continue
		}
		if imp.Wildcard || imp.Alias == DotAlias {
			s.dots = append(s.dots, imp.Path)
			continue
		}
		name, binds := ImportName(imp)
		if !binds {
			continue
		}
		s.named[name] = imp.Path
		if imp.Alias == "" {
			s.all = append(s.all, imp.Path)
		}
	}
	return s
}

// Import returns the import path a qualifier binds, and false for a
// qualifier no import binds.
func (s Scope) Import(qualifier string) (string, bool) {
	path, bound := s.named[qualifier]
	return path, bound
}

// Candidates returns the declarations a spelling may name, in Go's
// probe order, as identities in [Lang] with a package and a name and
// no kind. Every candidate competes with the others: Go refuses to
// compile a file in which two of them declare the name.
//
// Decoration strips first, because a reference keeps its source
// verbatim: pointers, slices, arrays, variadic dots, parentheses and
// a trailing instantiation's argument list, so *List[T] probes what
// List does. A qualified spelling then probes the import its
// qualifier binds, or, where no import binds the qualifier, every
// unaliased import in source order, because a package's clause can
// declare a name other than the one its path assumes. A bare spelling
// probes pkg, the file's own package, and then, for an exported name,
// each dot-imported package in source order. A predeclared type, a
// composite such as a map, a func, a chan or an inline body, and a
// constraint term return no candidate.
func (s Scope) Candidates(pkg, spelling string) []symbol.Identity {
	core := stripped(spelling)
	if core == "" || Predeclared(core) {
		return nil
	}
	if qualifier, name, qualified := strings.Cut(core, qualifierMark); qualified {
		if path, bound := s.named[qualifier]; bound {
			return []symbol.Identity{{Lang: Lang, Package: path, Name: name}}
		}
		var out []symbol.Identity
		for _, path := range s.all {
			out = append(out, symbol.Identity{Lang: Lang, Package: path, Name: name})
		}
		return out
	}
	out := []symbol.Identity{{Lang: Lang, Package: pkg, Name: core}}
	if exported(core) {
		for _, path := range s.dots {
			out = append(out, symbol.Identity{Lang: Lang, Package: path, Name: core})
		}
	}
	return out
}

// stripped returns the name inside a type expression's decoration,
// and empty for a spelling that names no single declaration: a
// composite, a constraint term, or an array whose bracket never
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

// exported reports Go's visibility rule for a name: an upper-case
// first letter.
func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
