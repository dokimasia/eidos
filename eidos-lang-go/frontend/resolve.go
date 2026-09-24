// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// bindings is what a file's parse records for its own Resolve: the
// qualifier each import binds against its path, the dot-imported
// packages whose exported names enter the file's scope, and every
// unaliased import in source order for the fallback probes.
type bindings struct {
	named map[string]string
	dots  []string
	all   []string
}

// newBindings returns an empty record.
func newBindings() *bindings {
	return &bindings{named: map[string]string{}}
}

// resolve returns what a spelling could mean in one file's
// bindings: one tier of candidates in probe order, because Go
// refuses to compile a file in which two of them declare the name,
// and the resolution step reports that case as an ambiguity. A
// qualified spelling probes the import its qualifier binds. A
// qualifier no binding matches probes every unaliased import in
// source order, because a package whose clause differs from its
// assumed name is still one of the file's imports and the graph
// decides which; an aliased, a blank and a dot import bind no
// qualifier but their own. A bare spelling probes the file's own
// package and then, exported, each dot-imported package in source
// order. Decoration strips first: pointer, slice, array, variadic,
// parentheses and a trailing instantiation, because the reference
// keeps its source verbatim and the normalization is Go's own. A
// predeclared type and a shape no single declaration declares return
// no candidate.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	b, _ := scope.Bindings.(*bindings)
	if b == nil {
		b = newBindings()
	}
	core := normalize(spelling)
	if core == "" || golang.Predeclared(core) {
		return nil
	}
	if qualifier, name, qualified := strings.Cut(core, "."); qualified {
		if imported, bound := b.named[qualifier]; bound {
			return plugin.Candidates{{{Lang: Lang, Package: imported, Name: name}}}
		}
		if len(b.all) == 0 {
			return nil
		}
		tier := make([]symbol.Identity, 0, len(b.all))
		for _, imported := range b.all {
			tier = append(tier, symbol.Identity{Lang: Lang, Package: imported, Name: name})
		}
		return plugin.Candidates{tier}
	}
	tier := []symbol.Identity{{Lang: Lang, Package: scope.File.Package, Name: core}}
	if exported(core) {
		for _, dotted := range b.dots {
			tier = append(tier, symbol.Identity{Lang: Lang, Package: dotted, Name: core})
		}
	}
	return plugin.Candidates{tier}
}

// exported reports Go's own visibility rule for a bare name.
func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// normalize strips the decoration around a type expression's named
// type: pointers, slices, arrays, variadic dots,
// parentheses, and a trailing instantiation's argument list, so
// `*List[T]` resolves the way `List[T]` does. What remains either
// names a declaration or is a composite (a map, a func, a chan, an
// inline struct or interface) or a constraint term, which no single
// declaration declares, and returns empty.
func normalize(spelling string) string {
	s := strings.TrimSpace(spelling)
	for {
		switch {
		case strings.HasPrefix(s, "*"):
			s = s[1:]
		case strings.HasPrefix(s, "..."):
			s = s[3:]
		case strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")"):
			s = strings.TrimSpace(s[1 : len(s)-1])
		case strings.HasPrefix(s, "["):
			bracket := strings.IndexByte(s, ']')
			if bracket < 0 {
				return ""
			}
			s = s[bracket+1:]
		default:
			if open := strings.IndexByte(s, '['); open > 0 && strings.HasSuffix(s, "]") {
				s = s[:open]
				continue
			}
			if s == "" || strings.ContainsAny(s, "[({ \t~|") {
				return ""
			}
			if slices.Contains([]string{"map", "func", "chan", "struct", "interface"}, s) {
				return ""
			}
			return s
		}
	}
}
