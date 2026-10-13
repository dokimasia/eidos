// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Package is a namespace holding declarations.
//
// Path holds the namespace as hierarchical segments, so Rust module
// nesting and TypeScript namespaces map into it the way Go import
// paths do. The original spelling stays in language metadata. Name
// is the declared name, which Go lets differ from the last path
// segment.
//
// Files is node-only: a generated package is a set of files the
// layout routes, never a parsed directory.
type Package struct {
	ID    symbol.Identity `eidos:"node"`
	Pos   position.Pos    `eidos:"node"`
	Doc   []string        `eidos:"both"`
	Path  []string        `eidos:"both"` // ["svc","store"]
	Name  string          `eidos:"both"` // may differ from the last segment
	Files []*File         `eidos:"node,walk"`
}

// File is one source file and the declarations it holds.
//
// Imports and Exports record the file's module boundary as written,
// which is what the resolution step reads to turn a spelling into
// an identity: a name means what the file's imports say it means.
// Decls holds any kind, because a file admits whatever its language
// admits at top level.
type File struct {
	ID          symbol.Identity    `eidos:"node"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Path        string             `eidos:"both"`                  // workspace-relative, slash-separated
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"` // file-level tool directives no declaration owns
	Imports     []*Import          `eidos:"node,walk"`             // the file's import scope, as written
	Exports     []*Export          `eidos:"node,walk"`             // re-exports; a declaration's own export is its Visibility
	Decls       []Symbol           `eidos:"node,walk"`
}

// Import is one import statement and everything it binds.
//
// One statement is one Import, however many names it binds. Path
// names the module. Alias is the module-level alias, which covers a
// Go renamed import, a Python "import numpy as np" and a TypeScript
// namespace import. Names has the per-symbol bindings, so
// "import {a as b, c} from 'x'" has two. Default is the local name
// bound to the module's default export, which TypeScript has and
// most languages do not.
//
// Wildcard means the statement pulls every exported name into
// scope: a Java "import java.util.*", a Rust glob use, a Python
// star import. Resolution treats a wildcard scope differently from
// a named one, so it is a field rather than a spelling.
//
// Modules are the packages that Path names, in the order in which the
// language tries them, where a configuration of the workspace maps
// the specifier, such as the paths option of a tsconfig. Modules is
// nil where the language resolves the specifier from its text alone.
// The graph does not record the configuration, so the rules resolve a
// mapped specifier through Modules.
//
// Import is node-only. A generated file's imports are a side effect
// of spelling types, collected per file as the backend renders,
// never a declaration a generator writes.
type Import struct {
	ID       symbol.Identity `eidos:"node"`
	Pos      position.Pos    `eidos:"node"`
	Doc      []string        `eidos:"node"`
	Comment  string          `eidos:"node"` // trailing line comment; "" when none
	Path     string          `eidos:"node"`
	Alias    string          `eidos:"node"`      // module-level alias; "" when unaliased
	Names    []*Binding      `eidos:"node,walk"` // per-symbol bindings
	Default  string          `eidos:"node"`      // local name for the module's default export
	Wildcard bool            `eidos:"node"`      // every exported name enters scope
	Modules  []string        `eidos:"node"`      // the packages that a mapped Path names; nil when unmapped
}

// Export is one re-export statement: a name this file publishes
// that it did not declare.
//
// A declaration's own export is its [symbol.Visibility], not an
// Export. This kind records the statement forms that publish
// something else: a TypeScript "export { a } from './x'" or
// "export * from './x'", a Rust "pub use", a Python __all__ entry.
// Path names the source module and is empty when the file
// re-exports its own local names. Modules are the packages that a
// mapped Path names, as [Import.Modules] are.
//
// The public surface of a package is its exported declarations plus
// these, which is why a binding generator has to read both.
type Export struct {
	ID       symbol.Identity `eidos:"node"`
	Pos      position.Pos    `eidos:"node"`
	Doc      []string        `eidos:"node"`
	Path     string          `eidos:"node"`      // source module; "" when re-exporting local names
	Names    []*Binding      `eidos:"node,walk"` // per-symbol bindings
	Default  string          `eidos:"node"`      // name published as the module's default export
	Wildcard bool            `eidos:"node"`      // every name of Path is republished
	Modules  []string        `eidos:"node"`      // the packages that a mapped Path names; nil when unmapped
}

// Binding is one name bound by an [Import] or published by an
// [Export].
//
// Name is the name as the source module spells it. Alias is the
// local spelling when the statement renames it, so "a as b" carries
// Name "a" and Alias "b", and an unrenamed binding leaves Alias
// empty. It carries its own position, so a diagnostic about one
// unused name in a ten-name import points at that name.
type Binding struct {
	ID    symbol.Identity `eidos:"node"`
	Pos   position.Pos    `eidos:"node"`
	Name  string          `eidos:"node"`
	Alias string          `eidos:"node"` // "" when unrenamed
	Host  symbol.Identity `eidos:"node"`
}
