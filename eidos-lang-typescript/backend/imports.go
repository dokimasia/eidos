// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries: package specifiers
// first, relative ones after, each group sorted, which is the
// order the ecosystem's formatters leave. The names bound under a
// path render as one named import — import type when every
// binding is type-only, because one value binding makes the whole
// import a value import — and a path binding no names renders as
// a side-effect import, which a named import already implies, so
// a path carrying both forms renders the named one alone.
// Specifiers quote single, TypeScript's own canon.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}

	var packages, relative []string
	for i := 0; i < len(entries); {
		path := entries[i].Path
		var names []string
		typeOnly := true
		for ; i < len(entries) && entries[i].Path == path; i++ {
			if n := entries[i].Name; n != "" {
				names = append(names, n)
				typeOnly = typeOnly && entries[i].TypeOnly
			}
		}

		var line strings.Builder
		line.WriteString("import ")
		if len(names) > 0 {
			if typeOnly {
				line.WriteString("type ")
			}
			line.WriteString("{ " + strings.Join(names, ", ") + " } from ")
		}
		line.WriteString(quote(path) + ";\n")

		if strings.HasPrefix(path, ".") {
			relative = append(relative, line.String())
			continue
		}
		packages = append(packages, line.String())
	}

	var b strings.Builder
	for _, line := range packages {
		b.WriteString(line)
	}
	for _, line := range relative {
		b.WriteString(line)
	}
	b.WriteString("\n")
	return b.String()
}

// The two Unicode line separators a string literal escapes, because
// an engine before ES2019 ends the literal at either.
const (
	lineSeparator      = 0x2028
	paragraphSeparator = 0x2029
)

// quote spells a string in single quotes, TypeScript's own canon,
// in TypeScript's escape grammar: the backslash, the quote and the
// named escapes \b \f \n \r \t \v, every other control character
// and the two line separators as a four-digit hex escape, and
// everything else as itself. A specifier, a quoted member key and
// a string value all spell through it.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\v':
			b.WriteString(`\v`)
		default:
			if r < ' ' || r == 0x7f || r == lineSeparator || r == paragraphSeparator {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
