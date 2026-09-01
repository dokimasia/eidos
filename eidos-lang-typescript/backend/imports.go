// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
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

// quote spells a specifier in single quotes, escaping the two
// characters that need it; everything else passes through, the
// way TypeScript source spells its own strings.
func quote(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(escaped, "'", `\'`) + "'"
}
