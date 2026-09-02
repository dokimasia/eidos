// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// bindings is what a file's parse records for its own Resolve: the
// local spelling of each import against its path, the dot-imported
// packages whose exported names flood the file's scope, and every
// imported path in source order for the fallback probes.
type bindings struct {
	named map[string]string
	dots  []string
	all   []string
}

// newBindings returns an empty record.
func newBindings() *bindings {
	return &bindings{named: map[string]string{}}
}

// builtins are the predeclared identifiers no package owns: a
// reference to one keeps its spelling, which is degradation a
// reader can ask about rather than failure.
var builtins = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true,
	"complex64": true, "complex128": true, "error": true,
	"float32": true, "float64": true, "int": true, "int8": true,
	"int16": true, "int32": true, "int64": true, "rune": true,
	"string": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "uintptr": true,
}

// resolve answers what a spelling could mean in one file's
// bindings, candidates in probe order. A qualified spelling probes
// the import its qualifier binds; a qualifier no binding matches
// probes every imported path in source order, because a package
// whose clause differs from its path is still one of the file's
// imports and the graph decides which. A bare spelling probes the
// file's own package and then, exported, each dot-imported package
// in source order. Decoration strips first — pointer, slice,
// array, variadic, parentheses, a trailing instantiation — because
// the reference carries its source verbatim and the normalization
// is Go's own; a shape no single declaration owns answers nothing.
func resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	b, _ := scope.Bindings.(*bindings)
	if b == nil {
		b = newBindings()
	}
	core := normalize(spelling)
	if core == "" || builtins[core] {
		return nil
	}
	if qualifier, name, qualified := strings.Cut(core, "."); qualified {
		if imported, bound := b.named[qualifier]; bound {
			return []symbol.Identity{{Lang: Lang, Package: imported, Name: name}}
		}
		out := make([]symbol.Identity, 0, len(b.all))
		for _, imported := range b.all {
			out = append(out, symbol.Identity{Lang: Lang, Package: imported, Name: name})
		}
		return out
	}
	out := []symbol.Identity{{Lang: Lang, Package: scope.File.Package, Name: core}}
	if exported(core) {
		for _, dotted := range b.dots {
			out = append(out, symbol.Identity{Lang: Lang, Package: dotted, Name: core})
		}
	}
	return out
}

// exported reports Go's own visibility rule for a bare name.
func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// normalize strips the decoration a type expression wears over the
// named type: pointers, slices, arrays, variadic dots,
// parentheses, and a trailing instantiation's argument list, so
// `*List[T]` resolves the way `List[T]` does. What remains either
// names a declaration or is a composite — map, func, chan, an
// inline struct or interface — or a constraint term, which no
// single declaration owns, and answers empty.
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
