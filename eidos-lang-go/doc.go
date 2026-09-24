// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package golang is the root of the Go satellite: the language's
// identity and comment forms, the golang.* fact keys, and the Go
// names the satellite's packages share.
//
// [Lang] is the language of every loaded Go declaration.
// [Target] and [Name] are the spellings a plan resolves to reach the
// backend, and [Version] is the backend's behavior version. [Syntax]
// returns Go's comment forms: the frontend strips comments with
// them, and the output contract writes the generated-file header
// through them. [Keys] registers the fact keys the frontend and the
// annotator stamp.
//
// [Predeclared] reports Go's predeclared type names, and [Basic] and
// [Ordered] the basic types among them, read off the universe scope
// of go/types. [ImportName] and [AssumedName] return the qualifier an
// import binds, so the frontend and the rules resolve a qualified
// spelling by one rule.
//
// # Packages
//
// The module's packages cover Go as a source and as a target:
//
//   - frontend loads Go source into the node graph.
//   - rules projects the graph through Go's decisions.
//   - annotate stamps what the sealed graph proves.
//   - spell spells filenames and declared names.
//   - backend renders emit values as Go source.
//   - testing runs the Go toolchain over generated output.
//
// # Projection facts
//
// The language fixes these facts, and no configuration changes them:
//
//   - Callables are Sync always, because Go's concurrency is
//     caller-side and never appears in a signature.
//   - The error model is LastReturn. The error-value rules read the
//     sentinel convention as a name and predicate pair.
//   - Composition is Embeds, never Extends. The members projection
//     and the promotion rules read promotion.
//   - Optionality projects from pointers. The equality rules read
//     comparability and name the members that break it: slices,
//     maps and funcs.
//   - The tag rules read struct tags, and the enum rules read
//     const-group enums, iota arithmetic included.
//
// # Parsing
//
// The frontend parses with the standard library's go/parser, a
// pure-Go library pinned by the module's toolchain version, never a
// toolchain installed on the machine. Positions and comment
// attachment are exact, and one workspace parses alike on every
// machine.
//
// # Dependency position
//
// The root package imports the sdk's meta, node, plugin and symbol
// facades and the Go stdlib, go/types among it. The module's other
// packages import the kernel's SPI through the sdk facade, the
// shared helpers of eidos-lang, and the Go stdlib. None imports
// eidos-lang's grammar packages: the frontend parses with the
// standard library, and the backend renders through the kernel's
// own pass.
package golang
