// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// bindings is what a file's parse records for its own Resolve: the
// local spelling of each import against the path it binds.
type bindings map[string]string

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
// bindings: a qualified spelling through the import it names, a
// bare one through the file's own package. Decoration strips
// first — pointer, slice, array, variadic — because the reference
// carries its source verbatim and the normalization is Go's own;
// a shape no single declaration owns answers nothing.
func resolve(scope plugin.ImportScope, spelling string) []symbol.Identity {
	b, _ := scope.Bindings.(bindings)
	core := normalize(spelling)
	if core == "" || builtins[core] {
		return nil
	}
	if qualifier, name, qualified := strings.Cut(core, "."); qualified {
		imported, bound := b[qualifier]
		if !bound {
			return nil
		}
		return []symbol.Identity{{Lang: Lang, Package: imported, Name: name}}
	}
	return []symbol.Identity{{Lang: Lang, Package: scope.File.Package, Name: core}}
}

// normalize strips the decoration a type expression wears over the
// named type: pointers, slices, arrays and variadic dots. What
// remains either names a declaration or is a composite — map,
// func, chan, an inline struct or interface — that no declaration
// owns, and answers empty.
func normalize(spelling string) string {
	s := strings.TrimSpace(spelling)
	for {
		switch {
		case strings.HasPrefix(s, "*"):
			s = s[1:]
		case strings.HasPrefix(s, "..."):
			s = s[3:]
		case strings.HasPrefix(s, "["):
			bracket := strings.IndexByte(s, ']')
			if bracket < 0 {
				return ""
			}
			s = s[bracket+1:]
		default:
			if s == "" || strings.ContainsAny(s, "[({ \t") {
				return ""
			}
			if slices.Contains([]string{"map", "func", "chan", "struct", "interface"}, s) {
				return ""
			}
			if rest, directed := strings.CutPrefix(s, "<-"); directed {
				s = rest
				continue
			}
			return s
		}
	}
}
