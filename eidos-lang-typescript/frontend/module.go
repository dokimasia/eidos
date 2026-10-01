// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	typescript "go.dokimi.dev/eidos/lang/typescript"
)

// indexModule is the file name a directory specifier resolves to.
const indexModule = "index"

// defaultExport is the name a module's default export is published
// under, and the name a default import binds.
const defaultExport = "default"

// relative reports whether a module specifier names a path relative to
// the importing file's directory.
func relative(specifier string) bool {
	return strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") ||
		specifier == "." || specifier == ".."
}

// binding is what an import binds one local name to: a module
// specifier and the name the module exports, "default" for a default
// import. A namespace import and an import-require bind the module
// itself, and their name is empty.
type binding struct {
	specifier string
	name      string
}

// reexport is one name an export clause publishes: the name the
// source module or the file binds, the name it is published under, and
// the source module, empty for a name of the file's own scope.
type reexport struct {
	name      string
	published string
	specifier string
}

// module is one file's record, shared by every scope the file records:
// its directory and package, what its imports bind, what it publishes
// and does not declare, and how its non-relative specifiers resolve.
// It is immutable once the parse returns, and safe for concurrent use.
type module struct {
	dir string
	pkg string

	// imports maps each local name an import binds onto its binding,
	// and aliases each local name an import alias binds onto the
	// dotted entity it names, as in import x = A.B.
	imports map[string]binding
	aliases map[string]string

	// reexports are the export clauses' names, and stars the source
	// modules of the export-star statements, in source order.
	reexports []reexport
	stars     []string

	// defaultName is the local name the default export publishes:
	// the default-exported declaration's, default for an anonymous
	// one, and the identifier export default and export = name.
	defaultName string

	// paths is the governing tsconfig's module resolution.
	paths resolution
}

// newModule returns the empty record of one file.
func newModule(file string, paths resolution) *module {
	return &module{
		dir:     path.Dir(file),
		pkg:     typescript.ModulePath(file),
		imports: map[string]binding{},
		aliases: map[string]string{},
		paths:   paths,
	}
}

// packages returns the candidate packages a module specifier names, in
// tiers, without a read. A relative specifier names the file before the
// directory's index, which is TypeScript's order. A specifier a paths
// pattern matches names each substitution's file and index the same
// way, and so does one the baseUrl places. Any other specifier names
// the package of its own spelling, which an ambient module declaration
// declares.
func (m *module) packages(specifier string) [][]string {
	if relative(specifier) {
		return fileAndIndex(path.Join(m.dir, specifier))
	}
	var out [][]string
	for _, target := range m.paths.substitutions(specifier) {
		out = append(out, fileAndIndex(target)...)
	}
	if m.paths.baseURL != "" {
		out = append(out, fileAndIndex(path.Join(m.paths.baseURL, specifier))...)
	}
	return append(out, []string{specifier})
}

// fileAndIndex returns the two tiers one module path names: the file
// without its extension, and the directory's index.
func fileAndIndex(p string) [][]string {
	base := typescript.ModulePath(p)
	return [][]string{{base}, {path.Join(base, indexModule)}}
}

// scope is what one File node of a file records for the resolution
// step: the file's module record, the package of the File, and the
// packages that enclose it, innermost first, ending with the file's own
// package. A namespace's File and the file's own File share one module.
type scope struct {
	module *module
	own    string
	outer  []string
}

// chain returns the packages a bare name probes, innermost first: the
// File's own, then each enclosing one.
func (s *scope) chain() []string {
	return append([]string{s.own}, s.outer...)
}
